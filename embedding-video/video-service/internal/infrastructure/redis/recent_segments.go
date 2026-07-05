package redis

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	goredis "github.com/go-redis/redis/v8"
)

type RecentSegmentStore struct {
	rdb    *goredis.Client
	prefix string
}

func NewRecentSegmentStore(rdb *goredis.Client, prefix string) *RecentSegmentStore {
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		prefix = "video:random_play:recent:"
	}
	return &RecentSegmentStore{rdb: rdb, prefix: prefix}
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
	cmds := make([]*goredis.BoolCmd, 0, len(validIDs))
	key := s.key(userID)
	for _, segmentID := range validIDs {
		cmds = append(cmds, pipe.SIsMember(ctx, key, strconv.FormatUint(segmentID, 10)))
	}
	if _, err := pipe.Exec(ctx); err != nil {
		return nil, err
	}
	for i, cmd := range cmds {
		if cmd.Val() {
			out[validIDs[i]] = true
		}
	}
	return out, nil
}

func (s *RecentSegmentStore) ListRecent(ctx context.Context, userID uint64) ([]uint64, error) {
	if s == nil || s.rdb == nil || userID == 0 {
		return nil, nil
	}
	values, err := s.rdb.SMembers(ctx, s.key(userID)).Result()
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
	pipe := s.rdb.TxPipeline()
	pipe.SAdd(ctx, key, strconv.FormatUint(segmentID, 10))
	pipe.Expire(ctx, key, ttl)
	_, err := pipe.Exec(ctx)
	return err
}

func (s *RecentSegmentStore) key(userID uint64) string {
	return fmt.Sprintf("%s%d", s.prefix, userID)
}
