package redis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	goredis "github.com/go-redis/redis/v8"

	"video-service/internal/application/videoapp"
)

const defaultCommentReactionPendingMinIdle = 30 * time.Second

// CommentLikeBuffer 为评论互动（like/double_like/dislike）提供 Redis 原子切换、
// 计数缓存和事件流缓冲，写路径与视频/片段 reaction 缓冲保持一致。
type CommentLikeBuffer struct {
	rdb            *goredis.Client
	streamKey      string
	group          string
	consumer       string
	countsPrefix   string
	userPrefix     string
	initializedAt  time.Duration
	pendingMinIdle time.Duration
}

type CommentLikeBufferOptions struct {
	StreamKey    string
	CountsPrefix string
	UserPrefix   string
}

func NewCommentLikeBuffer(rdb *goredis.Client, streamKey string) *CommentLikeBuffer {
	return NewCommentLikeBufferWithOptions(rdb, CommentLikeBufferOptions{StreamKey: streamKey})
}

func NewCommentLikeBufferWithOptions(rdb *goredis.Client, opts CommentLikeBufferOptions) *CommentLikeBuffer {
	countsPrefix := opts.CountsPrefix
	if countsPrefix == "" {
		countsPrefix = "comment:like:counts:"
	}
	userPrefix := opts.UserPrefix
	if userPrefix == "" {
		userPrefix = "comment:like:user:"
	}
	return &CommentLikeBuffer{
		rdb:            rdb,
		streamKey:      opts.StreamKey,
		group:          streamGroupName(opts.StreamKey),
		consumer:       streamConsumerName("comment_reaction"),
		countsPrefix:   countsPrefix,
		userPrefix:     userPrefix,
		pendingMinIdle: defaultCommentReactionPendingMinIdle,
	}
}

func (b *CommentLikeBuffer) countsKey(commentID uint64) string {
	return b.countsPrefix + strconv.FormatUint(commentID, 10)
}

func (b *CommentLikeBuffer) userKey(commentID uint64, userID uint64) string {
	return b.userPrefix + strconv.FormatUint(commentID, 10) + ":" + strconv.FormatUint(userID, 10)
}

func (b *CommentLikeBuffer) HasCounts(ctx context.Context, commentID uint64) (bool, error) {
	exists, err := b.rdb.Exists(ctx, b.countsKey(commentID)).Result()
	if err != nil {
		return false, err
	}
	return exists > 0, nil
}

func (b *CommentLikeBuffer) HasUserReaction(ctx context.Context, commentID uint64, userID uint64) (bool, error) {
	exists, err := b.rdb.Exists(ctx, b.userKey(commentID, userID)).Result()
	if err != nil {
		return false, err
	}
	return exists > 0, nil
}

func (b *CommentLikeBuffer) GetUserReaction(ctx context.Context, commentID uint64, userID uint64) (videoapp.VideoReactionType, bool, bool, error) {
	value, err := b.rdb.Get(ctx, b.userKey(commentID, userID)).Result()
	if err == goredis.Nil {
		return "", false, false, nil
	}
	if err != nil {
		return "", false, false, err
	}
	if value == "" || value == noReactionValue {
		return "", false, true, nil
	}
	reactionType := videoapp.VideoReactionType(value)
	if !reactionType.IsValid() {
		return "", false, true, fmt.Errorf("invalid cached comment reaction type: %s", value)
	}
	return reactionType, true, true, nil
}

// SeedUserReaction 回填用户对评论的互动状态缓存，仅在缓存缺失时写入。
func (b *CommentLikeBuffer) SeedUserReaction(ctx context.Context, commentID uint64, userID uint64, reactionType videoapp.VideoReactionType, active bool) error {
	value := noReactionValue
	if active && reactionType.IsValid() {
		value = string(reactionType)
	}
	return b.rdb.Set(ctx, b.userKey(commentID, userID), value, 0).Err()
}

func (b *CommentLikeBuffer) GetCounts(ctx context.Context, commentID uint64, seed videoapp.VideoReactionCounts) (videoapp.VideoReactionCounts, error) {
	key := b.countsKey(commentID)
	exists, err := b.rdb.Exists(ctx, key).Result()
	if err != nil {
		return videoapp.VideoReactionCounts{}, err
	}
	if exists == 0 {
		if err := b.seedCounts(ctx, commentID, seed); err != nil {
			return videoapp.VideoReactionCounts{}, err
		}
	}
	values, err := b.rdb.HMGet(ctx, key, "like", "double_like").Result()
	if err != nil {
		return videoapp.VideoReactionCounts{}, err
	}
	likeCount, err := parseRedisInt64(values[0])
	if err != nil {
		return videoapp.VideoReactionCounts{}, err
	}
	doubleLikeCount, err := parseRedisInt64(values[1])
	if err != nil {
		return videoapp.VideoReactionCounts{}, err
	}
	if likeCount < 0 {
		likeCount = 0
	}
	if doubleLikeCount < 0 {
		doubleLikeCount = 0
	}
	return videoapp.VideoReactionCounts{LikeCount: likeCount, DoubleLikeCount: doubleLikeCount}, nil
}

func (b *CommentLikeBuffer) seedCounts(ctx context.Context, commentID uint64, seed videoapp.VideoReactionCounts) error {
	_, err := b.rdb.Eval(ctx, seedCommentReactionCountsScript, []string{b.countsKey(commentID)}, int64(seed.LikeCount), int64(seed.DoubleLikeCount), int64(b.initializedAt.Seconds())).Result()
	return err
}

// Submit 原子地切换用户对评论的互动状态，返回切换后的状态与计数。
func (b *CommentLikeBuffer) Submit(ctx context.Context, commentID uint64, userID uint64, reactionType videoapp.VideoReactionType, seed videoapp.VideoReactionCounts, seedUserReaction videoapp.VideoReactionType, seedUserActive bool) (videoapp.VideoReactionResult, error) {
	if err := b.ensureGroup(ctx); err != nil {
		return videoapp.VideoReactionResult{}, err
	}
	seedUserValue := noReactionValue
	if seedUserActive && seedUserReaction.IsValid() {
		seedUserValue = string(seedUserReaction)
	}
	keys := []string{b.countsKey(commentID), b.userKey(commentID, userID), b.streamKey}
	args := []interface{}{
		int64(seed.LikeCount),
		int64(seed.DoubleLikeCount),
		string(reactionType),
		int64(b.initializedAt.Seconds()),
		seedUserValue,
		strconv.FormatUint(commentID, 10),
		strconv.FormatUint(userID, 10),
	}
	values, err := b.rdb.Eval(ctx, submitCommentReactionScript, keys, args...).Slice()
	if err != nil {
		return videoapp.VideoReactionResult{}, err
	}
	if len(values) < 3 {
		return videoapp.VideoReactionResult{}, errors.New("redis comment reaction script returned incomplete result")
	}
	active, err := parseRedisBool(values[0])
	if err != nil {
		return videoapp.VideoReactionResult{}, err
	}
	likeCount, err := parseRedisInt64(values[1])
	if err != nil {
		return videoapp.VideoReactionResult{}, err
	}
	doubleLikeCount, err := parseRedisInt64(values[2])
	if err != nil {
		return videoapp.VideoReactionResult{}, err
	}
	if likeCount < 0 {
		likeCount = 0
	}
	if doubleLikeCount < 0 {
		doubleLikeCount = 0
	}
	return videoapp.VideoReactionResult{
		Active:       active,
		ReactionType: reactionType,
		Counts: videoapp.VideoReactionCounts{
			LikeCount:       likeCount,
			DoubleLikeCount: doubleLikeCount,
		},
	}, nil
}

func (b *CommentLikeBuffer) ensureGroup(ctx context.Context) error {
	_, err := b.rdb.XGroupCreateMkStream(ctx, b.streamKey, b.group, "$").Result()
	if err != nil && !isBusyGroup(err) {
		return err
	}
	return nil
}

func (b *CommentLikeBuffer) Dequeue(ctx context.Context) (videoapp.CommentLikeQueueMessage, error) {
	if err := b.ensureGroup(ctx); err != nil {
		return videoapp.CommentLikeQueueMessage{}, err
	}
	if msg, ok, err := b.claimPending(ctx); err != nil {
		return videoapp.CommentLikeQueueMessage{}, err
	} else if ok {
		return msg, nil
	}
	streams, err := b.rdb.XReadGroup(ctx, &goredis.XReadGroupArgs{
		Group:    b.group,
		Consumer: b.consumer,
		Streams:  []string{b.streamKey, ">"},
		Count:    1,
		Block:    0,
	}).Result()
	if err != nil {
		return videoapp.CommentLikeQueueMessage{}, err
	}
	if len(streams) == 0 || len(streams[0].Messages) == 0 {
		return videoapp.CommentLikeQueueMessage{}, errors.New("empty stream message")
	}
	raw := streams[0].Messages[0]
	event, err := b.decodeEvent(ctx, raw)
	if err != nil {
		return videoapp.CommentLikeQueueMessage{}, err
	}
	return videoapp.CommentLikeQueueMessage{MessageID: raw.ID, Event: event}, nil
}

func (b *CommentLikeBuffer) claimPending(ctx context.Context) (videoapp.CommentLikeQueueMessage, bool, error) {
	minIdle := b.pendingMinIdle
	if minIdle < 0 {
		minIdle = 0
	}
	pending, err := b.rdb.XPendingExt(ctx, &goredis.XPendingExtArgs{
		Stream: b.streamKey,
		Group:  b.group,
		Idle:   minIdle,
		Start:  "-",
		End:    "+",
		Count:  1,
	}).Result()
	if err != nil {
		return videoapp.CommentLikeQueueMessage{}, false, err
	}
	if len(pending) == 0 {
		return videoapp.CommentLikeQueueMessage{}, false, nil
	}
	messages, err := b.rdb.XClaim(ctx, &goredis.XClaimArgs{
		Stream:   b.streamKey,
		Group:    b.group,
		Consumer: b.consumer,
		MinIdle:  minIdle,
		Messages: []string{pending[0].ID},
	}).Result()
	if err != nil {
		return videoapp.CommentLikeQueueMessage{}, false, err
	}
	if len(messages) == 0 {
		return videoapp.CommentLikeQueueMessage{}, false, nil
	}
	event, err := b.decodeEvent(ctx, messages[0])
	if err != nil {
		return videoapp.CommentLikeQueueMessage{}, false, err
	}
	return videoapp.CommentLikeQueueMessage{MessageID: messages[0].ID, Event: event}, true, nil
}

func (b *CommentLikeBuffer) Ack(ctx context.Context, id string) error {
	return b.ackAndDelete(ctx, id)
}

func (b *CommentLikeBuffer) Requeue(ctx context.Context, msg videoapp.CommentLikeQueueMessage, delay time.Duration, reason string) error {
	if err := b.ensureGroup(ctx); err != nil {
		return err
	}
	values := map[string]interface{}{
		"comment_id":    strconv.FormatUint(msg.Event.CommentID, 10),
		"user_id":       strconv.FormatUint(msg.Event.UserID, 10),
		"reaction_type": string(msg.Event.ReactionType),
		"active":        boolToRedisInt(msg.Event.Active),
		"retry":         msg.Event.Retry,
		"retry_reason":  reason,
	}
	if delay > 0 {
		values["visible_at"] = time.Now().Add(delay).Unix()
	}
	if err := withRetry(ctx, func() error {
		_, err := b.rdb.XAdd(ctx, &goredis.XAddArgs{Stream: b.streamKey, Values: values}).Result()
		return err
	}); err != nil {
		return err
	}
	return b.ackAndDelete(ctx, msg.MessageID)
}

func (b *CommentLikeBuffer) MoveToDeadLetter(ctx context.Context, msg videoapp.CommentLikeQueueMessage, reason string) error {
	payload, err := json.Marshal(msg.Event)
	if err != nil {
		return err
	}
	if err := withRetry(ctx, func() error {
		_, err := b.rdb.XAdd(ctx, &goredis.XAddArgs{
			Stream: b.streamKey + ":dlq",
			Values: map[string]interface{}{
				"payload": string(payload),
				"reason":  reason,
			},
		}).Result()
		return err
	}); err != nil {
		return err
	}
	return b.ackAndDelete(ctx, msg.MessageID)
}

func (b *CommentLikeBuffer) ackAndDelete(ctx context.Context, id string) error {
	if id == "" {
		return nil
	}
	if err := withRetry(ctx, func() error {
		return b.rdb.XAck(ctx, b.streamKey, b.group, id).Err()
	}); err != nil {
		return err
	}
	_ = b.rdb.XDel(ctx, b.streamKey, id).Err()
	return nil
}

func (b *CommentLikeBuffer) decodeEvent(ctx context.Context, msg goredis.XMessage) (videoapp.CommentLikeEvent, error) {
	if rawCommentID, hasCommentID := msg.Values["comment_id"]; hasCommentID {
		commentID, err := parseRedisUint64(rawCommentID)
		if err != nil {
			_ = b.ackAndDelete(ctx, msg.ID)
			return videoapp.CommentLikeEvent{}, err
		}
		userID, err := parseRedisUint64(msg.Values["user_id"])
		if err != nil {
			_ = b.ackAndDelete(ctx, msg.ID)
			return videoapp.CommentLikeEvent{}, err
		}
		reactionType := videoapp.VideoReactionType(fmt.Sprint(msg.Values["reaction_type"]))
		active, err := parseRedisBool(msg.Values["active"])
		if err != nil {
			_ = b.ackAndDelete(ctx, msg.ID)
			return videoapp.CommentLikeEvent{}, err
		}
		event := videoapp.CommentLikeEvent{
			CommentID:    commentID,
			UserID:       userID,
			ReactionType: reactionType,
			Active:       active,
		}
		if rawRetry, hasRetry := msg.Values["retry"]; hasRetry {
			retry, err := parseRedisInt64(rawRetry)
			if err != nil {
				_ = b.ackAndDelete(ctx, msg.ID)
				return videoapp.CommentLikeEvent{}, err
			}
			event.Retry = int(retry)
		}
		if event.CommentID == 0 || event.UserID == 0 || !event.ReactionType.IsValid() {
			_ = b.ackAndDelete(ctx, msg.ID)
			return videoapp.CommentLikeEvent{}, errors.New("invalid comment reaction stream payload")
		}
		return event, nil
	}

	payload, _ := msg.Values["payload"].(string)
	if payload == "" {
		_ = b.ackAndDelete(ctx, msg.ID)
		return videoapp.CommentLikeEvent{}, errors.New("comment reaction stream payload missing")
	}
	var event videoapp.CommentLikeEvent
	if err := json.Unmarshal([]byte(payload), &event); err != nil {
		_ = b.ackAndDelete(ctx, msg.ID)
		return videoapp.CommentLikeEvent{}, err
	}
	if event.CommentID == 0 || event.UserID == 0 || !event.ReactionType.IsValid() {
		_ = b.ackAndDelete(ctx, msg.ID)
		return videoapp.CommentLikeEvent{}, errors.New("invalid comment reaction stream payload")
	}
	return event, nil
}

const submitCommentReactionScript = `
local countsKey = KEYS[1]
local userKey = KEYS[2]
local streamKey = KEYS[3]

local seedLike = tonumber(ARGV[1]) or 0
local seedDoubleLike = tonumber(ARGV[2]) or 0
local newType = ARGV[3]
local ttlSeconds = tonumber(ARGV[4]) or 0
local seedUserValue = ARGV[5]
local commentID = ARGV[6]
local userID = ARGV[7]

if redis.call('EXISTS', countsKey) == 0 then
  redis.call('HSET', countsKey, 'like', seedLike, 'double_like', seedDoubleLike, 'dislike', 0, 'initialized', 1)
  if ttlSeconds > 0 then
    redis.call('EXPIRE', countsKey, ttlSeconds)
  end
end

local oldType = redis.call('GET', userKey)
if not oldType or oldType == '' then
  oldType = seedUserValue
  if oldType and oldType ~= '' and oldType ~= 'none' then
    if ttlSeconds > 0 then
      redis.call('SET', userKey, oldType, 'EX', ttlSeconds)
    else
      redis.call('SET', userKey, oldType)
    end
  end
end
if not oldType or oldType == '' then
  oldType = 'none'
end

local active = 1
if oldType == newType then
  active = 0
  if ttlSeconds > 0 then
    redis.call('SET', userKey, 'none', 'EX', ttlSeconds)
  else
    redis.call('SET', userKey, 'none')
  end
  redis.call('HINCRBY', countsKey, oldType, -1)
else
  if oldType ~= 'none' then
    redis.call('HINCRBY', countsKey, oldType, -1)
  end
  if ttlSeconds > 0 then
    redis.call('SET', userKey, newType, 'EX', ttlSeconds)
  else
    redis.call('SET', userKey, newType)
  end
  redis.call('HINCRBY', countsKey, newType, 1)
end

local likeCount = tonumber(redis.call('HGET', countsKey, 'like')) or 0
local doubleLikeCount = tonumber(redis.call('HGET', countsKey, 'double_like')) or 0
if likeCount < 0 then
  likeCount = 0
  redis.call('HSET', countsKey, 'like', 0)
end
if doubleLikeCount < 0 then
  doubleLikeCount = 0
  redis.call('HSET', countsKey, 'double_like', 0)
end
if ttlSeconds > 0 then
  redis.call('EXPIRE', countsKey, ttlSeconds)
end
redis.call('XADD', streamKey, '*', 'comment_id', commentID, 'user_id', userID, 'reaction_type', newType, 'active', active)
return { active, likeCount, doubleLikeCount, active }
`

const seedCommentReactionCountsScript = `
local countsKey = KEYS[1]
local seedLike = tonumber(ARGV[1]) or 0
local seedDoubleLike = tonumber(ARGV[2]) or 0
local ttlSeconds = tonumber(ARGV[3]) or 0

if redis.call('EXISTS', countsKey) == 0 then
  redis.call('HSET', countsKey, 'like', seedLike, 'double_like', seedDoubleLike, 'dislike', 0, 'initialized', 1)
  if ttlSeconds > 0 then
    redis.call('EXPIRE', countsKey, ttlSeconds)
  end
end
return 1
`
