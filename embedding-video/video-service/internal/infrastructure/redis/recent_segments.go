package redis

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	goredis "github.com/go-redis/redis/v8"
)

const defaultRecentSegmentMaxSize = 200

type RecentSegmentStore struct {
	rdb     *goredis.Client
	prefix  string
	maxSize int
}

type RecentSegmentStoreOptions struct {
	Prefix  string
	MaxSize int
}

func NewRecentSegmentStore(rdb *goredis.Client, prefix string) *RecentSegmentStore {
	return NewRecentSegmentStoreWithOptions(rdb, RecentSegmentStoreOptions{
		Prefix:  prefix,
		MaxSize: defaultRecentSegmentMaxSize,
	})
}

func NewRecentSegmentStoreWithOptions(rdb *goredis.Client, opts RecentSegmentStoreOptions) *RecentSegmentStore {
	prefix := strings.TrimSpace(opts.Prefix)
	if prefix == "" {
		prefix = "video:random_play:recent:"
	}
	maxSize := opts.MaxSize
	if maxSize <= 0 {
		maxSize = defaultRecentSegmentMaxSize
	}
	return &RecentSegmentStore{rdb: rdb, prefix: prefix, maxSize: maxSize}
}

func (s *RecentSegmentStore) FilterRecent(ctx context.Context, userID uint64, segmentIDs []uint64) (map[uint64]bool, error) {
	out := make(map[uint64]bool)
	if s == nil || s.rdb == nil || userID == 0 || len(segmentIDs) == 0 {
		return out, nil
	}
	validIDs := make([]uint64, 0, len(segmentIDs))
	for _, segmentID := range segmentIDs {
		if segmentID == 0 {
			continue
		}
		validIDs = append(validIDs, segmentID)
	}
	if len(validIDs) == 0 {
		return out, nil
	}
	pipe := s.rdb.Pipeline()
	cmds := make([]*goredis.FloatCmd, 0, len(validIDs))
	key := s.key(userID)
	for _, segmentID := range validIDs {
		cmds = append(cmds, pipe.ZScore(ctx, key, strconv.FormatUint(segmentID, 10)))
	}
	if _, err := pipe.Exec(ctx); err != nil && err != goredis.Nil {
		return nil, err
	}
	for i, cmd := range cmds {
		if cmd.Err() == nil {
			out[validIDs[i]] = true
			continue
		}
		if cmd.Err() != goredis.Nil {
			return nil, cmd.Err()
		}
	}
	return out, nil
}

func (s *RecentSegmentStore) ListRecent(ctx context.Context, userID uint64) ([]uint64, error) {
	if s == nil || s.rdb == nil || userID == 0 {
		return nil, nil
	}
	values, err := s.rdb.ZRange(ctx, s.key(userID), 0, -1).Result()
	if err != nil {
		return nil, err
	}
	out := make([]uint64, 0, len(values))
	for _, value := range values {
		segmentID, err := strconv.ParseUint(value, 10, 64)
		if err != nil || segmentID == 0 {
			continue
		}
		out = append(out, segmentID)
	}
	return out, nil
}

func (s *RecentSegmentStore) MarkReturned(ctx context.Context, userID uint64, segmentID uint64, ttl time.Duration) error {
	if s == nil || s.rdb == nil || userID == 0 || segmentID == 0 || ttl <= 0 {
		return nil
	}
	key := s.key(userID)
	if err := s.markReturned(ctx, key, segmentID, ttl); err != nil {
		if !isRedisWrongType(err) {
			return err
		}
		if delErr := s.rdb.Del(ctx, key).Err(); delErr != nil {
			return delErr
		}
		return s.markReturned(ctx, key, segmentID, ttl)
	}
	return nil
}

func (s *RecentSegmentStore) markReturned(ctx context.Context, key string, segmentID uint64, ttl time.Duration) error {
	nowMillis := time.Now().UnixMilli()
	cutoffMillis := nowMillis - ttl.Milliseconds()
	pipe := s.rdb.TxPipeline()
	pipe.ZAdd(ctx, key, &goredis.Z{
		Score:  float64(nowMillis),
		Member: strconv.FormatUint(segmentID, 10),
	})
	pipe.ZRemRangeByScore(ctx, key, "-inf", strconv.FormatInt(cutoffMillis, 10))
	pipe.ZRemRangeByRank(ctx, key, 0, int64(-s.maxSize-1))
	pipe.Expire(ctx, key, ttl)
	_, err := pipe.Exec(ctx)
	return err
}

func (s *RecentSegmentStore) TTL(ctx context.Context, userID uint64) (time.Duration, error) {
	if s == nil || s.rdb == nil || userID == 0 {
		return 0, nil
	}
	return s.rdb.TTL(ctx, s.key(userID)).Result()
}

func (s *RecentSegmentStore) MaxSize() int {
	if s == nil {
		return 0
	}
	return s.maxSize
}

func (s *RecentSegmentStore) key(userID uint64) string {
	return fmt.Sprintf("%s%d", s.prefix, userID)
}

func isRedisWrongType(err error) bool {
	return err != nil && strings.Contains(strings.ToUpper(err.Error()), "WRONGTYPE")
}
