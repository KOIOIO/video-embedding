package redis

import (
	"context"
	"errors"
	"fmt"

	goredis "github.com/go-redis/redis/v8"
)

type DeadLetterEntry struct {
	ID      string
	Payload string
	Reason  string
}

type ReplayDeadLetterOptions struct {
	KeepDeadLetter bool
}

func DeadLetterStreamKey(streamKey string) string {
	return streamKey + ":dlq"
}

func ListDeadLetters(ctx context.Context, rdb *goredis.Client, streamKey string, limit int64) ([]DeadLetterEntry, error) {
	if limit <= 0 {
		limit = 20
	}
	messages, err := rdb.XRevRangeN(ctx, DeadLetterStreamKey(streamKey), "+", "-", limit).Result()
	if err != nil {
		return nil, err
	}
	entries := make([]DeadLetterEntry, 0, len(messages))
	for _, msg := range messages {
		entry, err := deadLetterEntryFromMessage(msg)
		if err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

func ReplayDeadLetter(ctx context.Context, rdb *goredis.Client, streamKey string, id string, opts ReplayDeadLetterOptions) (bool, error) {
	entry, found, err := GetDeadLetter(ctx, rdb, streamKey, id)
	if err != nil || !found {
		return found, err
	}
	if err := withRetry(ctx, func() error {
		_, err := rdb.XAdd(ctx, &goredis.XAddArgs{
			Stream: streamKey,
			Values: map[string]interface{}{
				"payload":       entry.Payload,
				"replay_from":   id,
				"replay_reason": entry.Reason,
			},
		}).Result()
		return err
	}); err != nil {
		return false, err
	}
	if opts.KeepDeadLetter {
		return true, nil
	}
	if err := withRetry(ctx, func() error {
		return rdb.XDel(ctx, DeadLetterStreamKey(streamKey), id).Err()
	}); err != nil {
		return false, err
	}
	return true, nil
}

func GetDeadLetter(ctx context.Context, rdb *goredis.Client, streamKey string, id string) (DeadLetterEntry, bool, error) {
	if id == "" {
		return DeadLetterEntry{}, false, errors.New("dead letter id is required")
	}
	messages, err := rdb.XRange(ctx, DeadLetterStreamKey(streamKey), id, id).Result()
	if err != nil {
		return DeadLetterEntry{}, false, err
	}
	if len(messages) == 0 {
		return DeadLetterEntry{}, false, nil
	}
	entry, err := deadLetterEntryFromMessage(messages[0])
	if err != nil {
		return DeadLetterEntry{}, false, err
	}
	return entry, true, nil
}

func deadLetterEntryFromMessage(msg goredis.XMessage) (DeadLetterEntry, error) {
	payload := fmt.Sprint(msg.Values["payload"])
	if payload == "" || payload == "<nil>" {
		return DeadLetterEntry{}, errors.New("dead letter payload missing")
	}
	reason := ""
	if rawReason, ok := msg.Values["reason"]; ok {
		reason = fmt.Sprint(rawReason)
	}
	return DeadLetterEntry{
		ID:      msg.ID,
		Payload: payload,
		Reason:  reason,
	}, nil
}
