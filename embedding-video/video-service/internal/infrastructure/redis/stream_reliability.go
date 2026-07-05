package redis

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"time"

	goredis "github.com/go-redis/redis/v8"
)

const (
	defaultStreamPendingMinIdle = 3 * time.Hour
	defaultTaskPendingMinIdle   = 6 * time.Hour
	blockingQueuePollInterval   = time.Second
)

type delayedStreamPayload struct {
	ID          string `json:"id"`
	Payload     string `json:"payload"`
	RetryReason string `json:"retry_reason,omitempty"`
}

func delayedStreamKey(streamKey string) string {
	return streamKey + ":delayed"
}

func enqueueStreamPayload(ctx context.Context, rdb *goredis.Client, streamKey string, payload string, retryReason string, delay time.Duration) error {
	if delay <= 0 {
		values := map[string]interface{}{"payload": payload}
		if retryReason != "" {
			values["retry_reason"] = retryReason
		}
		return withRetry(ctx, func() error {
			_, err := rdb.XAdd(ctx, &goredis.XAddArgs{
				Stream: streamKey,
				Values: values,
			}).Result()
			return err
		})
	}

	retryReason = strings.ReplaceAll(retryReason, "\n", " ")
	member := fmt.Sprintf("%d_%d\n%s\n%s", time.Now().UnixNano(), rand.Int63(), retryReason, payload)
	return withRetry(ctx, func() error {
		return rdb.ZAdd(ctx, delayedStreamKey(streamKey), &goredis.Z{
			Score:  float64(time.Now().Add(delay).UnixMilli()),
			Member: member,
		}).Err()
	})
}

func promoteDueDelayed(ctx context.Context, rdb *goredis.Client, streamKey string) error {
	return withRetry(ctx, func() error {
		_, err := rdb.Eval(ctx, promoteDueDelayedScript, []string{delayedStreamKey(streamKey), streamKey}, time.Now().UnixMilli()).Result()
		if err == goredis.Nil {
			return nil
		}
		return err
	})
}

func claimPendingStreamMessage(ctx context.Context, rdb *goredis.Client, streamKey string, group string, consumer string, minIdle time.Duration) (goredis.XMessage, bool, error) {
	if minIdle < 0 {
		minIdle = 0
	}
	pending, err := rdb.XPendingExt(ctx, &goredis.XPendingExtArgs{
		Stream: streamKey,
		Group:  group,
		Idle:   minIdle,
		Start:  "-",
		End:    "+",
		Count:  1,
	}).Result()
	if err != nil {
		return goredis.XMessage{}, false, err
	}
	if len(pending) == 0 {
		return goredis.XMessage{}, false, nil
	}
	messages, err := rdb.XClaim(ctx, &goredis.XClaimArgs{
		Stream:   streamKey,
		Group:    group,
		Consumer: consumer,
		MinIdle:  minIdle,
		Messages: []string{pending[0].ID},
	}).Result()
	if err != nil {
		return goredis.XMessage{}, false, err
	}
	if len(messages) == 0 {
		return goredis.XMessage{}, false, nil
	}
	return messages[0], true, nil
}

const promoteDueDelayedScript = `
local delayedKey = KEYS[1]
local streamKey = KEYS[2]
local nowMillis = tonumber(ARGV[1])

local entries = redis.call('ZRANGEBYSCORE', delayedKey, '-inf', nowMillis, 'LIMIT', 0, 1)
if #entries == 0 then
  return nil
end

if redis.call('ZREM', delayedKey, entries[1]) == 0 then
  return nil
end

local firstBreak = string.find(entries[1], '\n')
local secondBreak = nil
if firstBreak ~= nil then
  secondBreak = string.find(entries[1], '\n', firstBreak + 1)
end
if firstBreak == nil or secondBreak == nil then
  return redis.call('XADD', streamKey, '*', 'payload', entries[1])
end

local reason = string.sub(entries[1], firstBreak + 1, secondBreak - 1)
local payload = string.sub(entries[1], secondBreak + 1)
if reason ~= nil and reason ~= '' then
  return redis.call('XADD', streamKey, '*', 'payload', payload, 'retry_reason', reason)
end
return redis.call('XADD', streamKey, '*', 'payload', payload)
`
