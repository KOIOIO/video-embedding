package redis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	goredis "github.com/go-redis/redis/v8"

	"nlp-video-analysis/internal/application/videoapp"
)

const noReactionValue = "none"
const defaultReactionPendingMinIdle = 30 * time.Second

type VideoReactionBuffer struct {
	rdb            *goredis.Client
	streamKey      string
	group          string
	consumer       string
	countsPrefix   string
	userPrefix     string
	initializedAt  time.Duration
	pendingMinIdle time.Duration
}

type VideoReactionBufferOptions struct {
	StreamKey    string
	CountsPrefix string
	UserPrefix   string
}

func NewVideoReactionBuffer(rdb *goredis.Client, streamKey string) *VideoReactionBuffer {
	return NewVideoReactionBufferWithOptions(rdb, VideoReactionBufferOptions{StreamKey: streamKey})
}

func NewVideoReactionBufferWithOptions(rdb *goredis.Client, opts VideoReactionBufferOptions) *VideoReactionBuffer {
	streamKey := opts.StreamKey
	countsPrefix := opts.CountsPrefix
	if countsPrefix == "" {
		countsPrefix = "video:reaction:counts:"
	}
	userPrefix := opts.UserPrefix
	if userPrefix == "" {
		userPrefix = "video:reaction:user:"
	}
	return &VideoReactionBuffer{
		rdb:            rdb,
		streamKey:      streamKey,
		group:          streamGroupName(streamKey),
		consumer:       streamConsumerName("reaction"),
		countsPrefix:   countsPrefix,
		userPrefix:     userPrefix,
		pendingMinIdle: defaultReactionPendingMinIdle,
	}
}

func (b *VideoReactionBuffer) HasCounts(ctx context.Context, videoID uint64) (bool, error) {
	exists, err := b.rdb.Exists(ctx, b.countsKey(videoID)).Result()
	if err != nil {
		return false, err
	}
	return exists > 0, nil
}

func (b *VideoReactionBuffer) HasUserReaction(ctx context.Context, videoID uint64, userID uint64) (bool, error) {
	exists, err := b.rdb.Exists(ctx, b.userKey(videoID, userID)).Result()
	if err != nil {
		return false, err
	}
	return exists > 0, nil
}

func (b *VideoReactionBuffer) Submit(ctx context.Context, videoID uint64, userID uint64, reactionType videoapp.VideoReactionType, seed videoapp.VideoReactionCounts, seedUserReaction videoapp.VideoReactionType, seedUserActive bool) (videoapp.VideoReactionResult, error) {
	if err := b.ensureGroup(ctx); err != nil {
		return videoapp.VideoReactionResult{}, err
	}
	keys := []string{b.countsKey(videoID), b.userKey(videoID, userID), b.streamKey}
	seedUserValue := noReactionValue
	if seedUserActive && seedUserReaction.IsValid() {
		seedUserValue = string(seedUserReaction)
	}
	args := []interface{}{
		int64(seed.LikeCount),
		int64(seed.DoubleLikeCount),
		string(reactionType),
		int64(b.initializedAt.Seconds()),
		seedUserValue,
		strconv.FormatUint(videoID, 10),
		strconv.FormatUint(userID, 10),
	}
	values, err := b.rdb.Eval(ctx, submitVideoReactionScript, keys, args...).Slice()
	if err != nil {
		return videoapp.VideoReactionResult{}, err
	}
	if len(values) < 3 {
		return videoapp.VideoReactionResult{}, errors.New("redis reaction script returned incomplete result")
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
	if len(values) >= 4 {
		active, err = parseRedisBool(values[3])
		if err != nil {
			return videoapp.VideoReactionResult{}, err
		}
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

func (b *VideoReactionBuffer) GetCounts(ctx context.Context, videoID uint64, seed videoapp.VideoReactionCounts) (videoapp.VideoReactionCounts, error) {
	key := b.countsKey(videoID)
	exists, err := b.rdb.Exists(ctx, key).Result()
	if err != nil {
		return videoapp.VideoReactionCounts{}, err
	}
	if exists == 0 {
		if err := b.seedCounts(ctx, videoID, seed); err != nil {
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
	return videoapp.VideoReactionCounts{
		LikeCount:       likeCount,
		DoubleLikeCount: doubleLikeCount,
	}, nil
}

func (b *VideoReactionBuffer) Enqueue(ctx context.Context, event videoapp.VideoReactionEvent) error {
	if err := b.ensureGroup(ctx); err != nil {
		return err
	}
	return withRetry(ctx, func() error {
		_, err := b.rdb.XAdd(ctx, &goredis.XAddArgs{
			Stream: b.streamKey,
			Values: map[string]interface{}{
				"video_id":      strconv.FormatUint(event.VideoID, 10),
				"user_id":       strconv.FormatUint(event.UserID, 10),
				"reaction_type": string(event.ReactionType),
				"active":        boolToRedisInt(event.Active),
				"retry":         event.Retry,
			},
		}).Result()
		return err
	})
}

func (b *VideoReactionBuffer) Dequeue(ctx context.Context) (videoapp.VideoReactionQueueMessage, error) {
	if err := b.ensureGroup(ctx); err != nil {
		return videoapp.VideoReactionQueueMessage{}, err
	}
	if msg, ok, err := b.claimPending(ctx); err != nil {
		return videoapp.VideoReactionQueueMessage{}, err
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
		return videoapp.VideoReactionQueueMessage{}, err
	}
	if len(streams) == 0 || len(streams[0].Messages) == 0 {
		return videoapp.VideoReactionQueueMessage{}, errors.New("empty stream message")
	}
	raw := streams[0].Messages[0]
	event, err := b.decodeEvent(ctx, raw)
	if err != nil {
		return videoapp.VideoReactionQueueMessage{}, err
	}
	return videoapp.VideoReactionQueueMessage{MessageID: raw.ID, Event: event}, nil
}

func (b *VideoReactionBuffer) claimPending(ctx context.Context) (videoapp.VideoReactionQueueMessage, bool, error) {
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
		return videoapp.VideoReactionQueueMessage{}, false, err
	}
	if len(pending) == 0 {
		return videoapp.VideoReactionQueueMessage{}, false, nil
	}
	messages, err := b.rdb.XClaim(ctx, &goredis.XClaimArgs{
		Stream:   b.streamKey,
		Group:    b.group,
		Consumer: b.consumer,
		MinIdle:  minIdle,
		Messages: []string{pending[0].ID},
	}).Result()
	if err != nil {
		return videoapp.VideoReactionQueueMessage{}, false, err
	}
	if len(messages) == 0 {
		return videoapp.VideoReactionQueueMessage{}, false, nil
	}
	event, err := b.decodeEvent(ctx, messages[0])
	if err != nil {
		return videoapp.VideoReactionQueueMessage{}, false, err
	}
	return videoapp.VideoReactionQueueMessage{MessageID: messages[0].ID, Event: event}, true, nil
}

func (b *VideoReactionBuffer) Ack(ctx context.Context, id string) error {
	return b.ackAndDelete(ctx, id)
}

func (b *VideoReactionBuffer) Requeue(ctx context.Context, msg videoapp.VideoReactionQueueMessage, delay time.Duration, reason string) error {
	if err := b.ensureGroup(ctx); err != nil {
		return err
	}
	values := map[string]interface{}{
		"video_id":      strconv.FormatUint(msg.Event.VideoID, 10),
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

func (b *VideoReactionBuffer) MoveToDeadLetter(ctx context.Context, msg videoapp.VideoReactionQueueMessage, reason string) error {
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

func (b *VideoReactionBuffer) seedCounts(ctx context.Context, videoID uint64, seed videoapp.VideoReactionCounts) error {
	_, err := b.rdb.Eval(ctx, seedVideoReactionCountsScript, []string{b.countsKey(videoID)}, int64(seed.LikeCount), int64(seed.DoubleLikeCount), int64(b.initializedAt.Seconds())).Result()
	return err
}

func (b *VideoReactionBuffer) countsKey(videoID uint64) string {
	return b.countsPrefix + strconv.FormatUint(videoID, 10)
}

func (b *VideoReactionBuffer) userKey(videoID uint64, userID uint64) string {
	return b.userPrefix + strconv.FormatUint(videoID, 10) + ":" + strconv.FormatUint(userID, 10)
}

func (b *VideoReactionBuffer) ensureGroup(ctx context.Context) error {
	_, err := b.rdb.XGroupCreateMkStream(ctx, b.streamKey, b.group, "$").Result()
	if err != nil && !isBusyGroup(err) {
		return err
	}
	return nil
}

func (b *VideoReactionBuffer) ackAndDelete(ctx context.Context, id string) error {
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

func (b *VideoReactionBuffer) decodeEvent(ctx context.Context, msg goredis.XMessage) (videoapp.VideoReactionEvent, error) {
	if rawVideoID, hasVideoID := msg.Values["video_id"]; hasVideoID {
		videoID, err := parseRedisUint64(rawVideoID)
		if err != nil {
			_ = b.ackAndDelete(ctx, msg.ID)
			return videoapp.VideoReactionEvent{}, err
		}
		userID, err := parseRedisUint64(msg.Values["user_id"])
		if err != nil {
			_ = b.ackAndDelete(ctx, msg.ID)
			return videoapp.VideoReactionEvent{}, err
		}
		reactionType := videoapp.VideoReactionType(fmt.Sprint(msg.Values["reaction_type"]))
		active, err := parseRedisBool(msg.Values["active"])
		if err != nil {
			_ = b.ackAndDelete(ctx, msg.ID)
			return videoapp.VideoReactionEvent{}, err
		}
		event := videoapp.VideoReactionEvent{
			VideoID:      videoID,
			UserID:       userID,
			ReactionType: reactionType,
			Active:       active,
		}
		if rawRetry, hasRetry := msg.Values["retry"]; hasRetry {
			retry, err := parseRedisInt64(rawRetry)
			if err != nil {
				_ = b.ackAndDelete(ctx, msg.ID)
				return videoapp.VideoReactionEvent{}, err
			}
			event.Retry = int(retry)
		}
		if event.VideoID == 0 || event.UserID == 0 || !event.ReactionType.IsValid() {
			_ = b.ackAndDelete(ctx, msg.ID)
			return videoapp.VideoReactionEvent{}, errors.New("invalid reaction stream payload")
		}
		return event, nil
	}

	payload, _ := msg.Values["payload"].(string)
	if payload == "" {
		_ = b.ackAndDelete(ctx, msg.ID)
		return videoapp.VideoReactionEvent{}, errors.New("reaction stream payload missing")
	}
	var event videoapp.VideoReactionEvent
	if err := json.Unmarshal([]byte(payload), &event); err != nil {
		_ = b.ackAndDelete(ctx, msg.ID)
		return videoapp.VideoReactionEvent{}, err
	}
	if event.VideoID == 0 || event.UserID == 0 || !event.ReactionType.IsValid() {
		_ = b.ackAndDelete(ctx, msg.ID)
		return videoapp.VideoReactionEvent{}, errors.New("invalid reaction stream payload")
	}
	return event, nil
}

func boolToRedisInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func parseRedisBool(value any) (bool, error) {
	n, err := parseRedisInt64(value)
	if err != nil {
		return false, err
	}
	return n != 0, nil
}

func parseRedisInt64(value any) (int64, error) {
	switch v := value.(type) {
	case nil:
		return 0, nil
	case int:
		return int64(v), nil
	case int64:
		return v, nil
	case string:
		if v == "" {
			return 0, nil
		}
		return strconv.ParseInt(v, 10, 64)
	case []byte:
		if len(v) == 0 {
			return 0, nil
		}
		return strconv.ParseInt(string(v), 10, 64)
	default:
		return 0, fmt.Errorf("unsupported redis integer type %T", value)
	}
}

func parseRedisUint64(value any) (uint64, error) {
	switch v := value.(type) {
	case nil:
		return 0, nil
	case uint64:
		return v, nil
	case int:
		if v < 0 {
			return 0, fmt.Errorf("negative redis integer %d", v)
		}
		return uint64(v), nil
	case int64:
		if v < 0 {
			return 0, fmt.Errorf("negative redis integer %d", v)
		}
		return uint64(v), nil
	case string:
		if v == "" {
			return 0, nil
		}
		return strconv.ParseUint(v, 10, 64)
	case []byte:
		if len(v) == 0 {
			return 0, nil
		}
		return strconv.ParseUint(string(v), 10, 64)
	default:
		return 0, fmt.Errorf("unsupported redis uint type %T", value)
	}
}

const submitVideoReactionScript = `
local countsKey = KEYS[1]
local userKey = KEYS[2]
local streamKey = KEYS[3]

local seedLike = tonumber(ARGV[1]) or 0
local seedDoubleLike = tonumber(ARGV[2]) or 0
local newType = ARGV[3]
local ttlSeconds = tonumber(ARGV[4]) or 86400
local seedUserValue = ARGV[5]
local videoID = ARGV[6]
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
  if oldType and oldType ~= '' and oldType ~= '` + noReactionValue + `' then
    if ttlSeconds > 0 then
      redis.call('SET', userKey, oldType, 'EX', ttlSeconds)
    else
      redis.call('SET', userKey, oldType)
    end
  end
end
if not oldType or oldType == '' then
  oldType = '` + noReactionValue + `'
end

local active = 1
if oldType == newType then
  active = 0
  redis.call('DEL', userKey)
  redis.call('HINCRBY', countsKey, oldType, -1)
else
  if oldType ~= '` + noReactionValue + `' then
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
redis.call('XADD', streamKey, '*', 'video_id', videoID, 'user_id', userID, 'reaction_type', newType, 'active', active)

return { active, likeCount, doubleLikeCount, active }
`

const seedVideoReactionCountsScript = `
local countsKey = KEYS[1]
local seedLike = tonumber(ARGV[1]) or 0
local seedDoubleLike = tonumber(ARGV[2]) or 0
local ttlSeconds = tonumber(ARGV[3]) or 86400

if redis.call('EXISTS', countsKey) == 0 then
  redis.call('HSET', countsKey, 'like', seedLike, 'double_like', seedDoubleLike, 'dislike', 0, 'initialized', 1)
  if ttlSeconds > 0 then
    redis.call('EXPIRE', countsKey, ttlSeconds)
  end
end
return 1
`
