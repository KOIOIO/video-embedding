package redis

import (
	"context"
	"strconv"
	"time"

	goredis "github.com/go-redis/redis/v8"
)

// CommentSegmentCountCache 缓存视频片段的评论总数。
// 创建评论时 INCR，缓存未命中时从数据库回填；回填带 TTL，
// 过期后自动重新与数据库对齐，避免长驻缓存产生计数漂移。
type CommentSegmentCountCache struct {
	rdb    *goredis.Client
	prefix string
}

func NewCommentSegmentCountCache(rdb *goredis.Client, prefix string) *CommentSegmentCountCache {
	return &CommentSegmentCountCache{rdb: rdb, prefix: prefix}
}

func (c *CommentSegmentCountCache) key(segmentID uint64) string {
	return c.prefix + strconv.FormatUint(segmentID, 10)
}

func (c *CommentSegmentCountCache) Get(ctx context.Context, segmentID uint64) (int64, bool, error) {
	value, err := c.rdb.Get(ctx, c.key(segmentID)).Result()
	if err == goredis.Nil {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	count, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, false, err
	}
	return count, true, nil
}

func (c *CommentSegmentCountCache) Seed(ctx context.Context, segmentID uint64, count int64, ttl time.Duration) error {
	return c.rdb.Set(ctx, c.key(segmentID), count, ttl).Err()
}

func (c *CommentSegmentCountCache) Incr(ctx context.Context, segmentID uint64) error {
	return c.rdb.Incr(ctx, c.key(segmentID)).Err()
}
