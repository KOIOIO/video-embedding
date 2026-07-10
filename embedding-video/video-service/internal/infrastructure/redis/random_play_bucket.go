package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	goredis "github.com/go-redis/redis/v8"

	"nlp-video-analysis/internal/application/videoapp"
)

type RandomPlayBucketStore struct {
	rdb    *goredis.Client
	prefix string
}

func NewRandomPlayBucketStore(rdb *goredis.Client, prefix string) *RandomPlayBucketStore {
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		prefix = "video:random_play:bucket:"
	}
	return &RandomPlayBucketStore{rdb: rdb, prefix: prefix}
}

func (s *RandomPlayBucketStore) Pop(ctx context.Context, userID uint64) (videoapp.RecommendResultItem, bool, error) {
	if s == nil || s.rdb == nil || userID == 0 {
		return videoapp.RecommendResultItem{}, false, nil
	}
	value, err := s.rdb.LPop(ctx, s.key(userID)).Result()
	if err == goredis.Nil {
		return videoapp.RecommendResultItem{}, false, nil
	}
	if err != nil {
		return videoapp.RecommendResultItem{}, false, err
	}
	var item videoapp.RecommendResultItem
	if err := json.Unmarshal([]byte(value), &item); err != nil || !isValidRandomPlayBucketItem(item) {
		return videoapp.RecommendResultItem{}, false, nil
	}
	return item, true, nil
}

func (s *RandomPlayBucketStore) Fill(ctx context.Context, userID uint64, items []videoapp.RecommendResultItem, maxSize int, ttl time.Duration) error {
	if s == nil || s.rdb == nil || userID == 0 || maxSize <= 0 || ttl <= 0 {
		return nil
	}
	key := s.key(userID)
	existing, err := s.List(ctx, userID)
	if err != nil {
		return err
	}
	seen := make(map[uint64]bool, len(existing)+len(items))
	for _, item := range existing {
		if item.VideoSegmentID == 0 {
			continue
		}
		seen[item.VideoSegmentID] = true
	}
	toPush := make([]any, 0, len(items))
	remaining := maxSize - len(existing)
	for _, item := range items {
		if remaining <= 0 {
			break
		}
		if !isValidRandomPlayBucketItem(item) || seen[item.VideoSegmentID] {
			continue
		}
		raw, err := json.Marshal(item)
		if err != nil {
			return err
		}
		seen[item.VideoSegmentID] = true
		toPush = append(toPush, string(raw))
		remaining--
	}
	pipe := s.rdb.TxPipeline()
	if len(toPush) > 0 {
		pipe.RPush(ctx, key, toPush...)
	}
	pipe.LTrim(ctx, key, 0, int64(maxSize-1))
	pipe.Expire(ctx, key, ttl)
	_, err = pipe.Exec(ctx)
	return err
}

func (s *RandomPlayBucketStore) Len(ctx context.Context, userID uint64) (int64, error) {
	if s == nil || s.rdb == nil || userID == 0 {
		return 0, nil
	}
	return s.rdb.LLen(ctx, s.key(userID)).Result()
}

func (s *RandomPlayBucketStore) List(ctx context.Context, userID uint64) ([]videoapp.RecommendResultItem, error) {
	if s == nil || s.rdb == nil || userID == 0 {
		return nil, nil
	}
	values, err := s.rdb.LRange(ctx, s.key(userID), 0, -1).Result()
	if err != nil {
		return nil, err
	}
	out := make([]videoapp.RecommendResultItem, 0, len(values))
	for _, value := range values {
		var item videoapp.RecommendResultItem
		if err := json.Unmarshal([]byte(value), &item); err != nil || !isValidRandomPlayBucketItem(item) {
			continue
		}
		out = append(out, item)
	}
	return out, nil
}

func (s *RandomPlayBucketStore) TTL(ctx context.Context, userID uint64) (time.Duration, error) {
	if s == nil || s.rdb == nil || userID == 0 {
		return 0, nil
	}
	return s.rdb.TTL(ctx, s.key(userID)).Result()
}

func (s *RandomPlayBucketStore) key(userID uint64) string {
	return fmt.Sprintf("%s%d", s.prefix, userID)
}

func isValidRandomPlayBucketItem(item videoapp.RecommendResultItem) bool {
	return item.VideoSegmentID != 0 && item.VideoID != 0 && strings.TrimSpace(item.Video.VideoURL) != ""
}
