package redis

import (
	"context"
	"os"
	"strconv"
	"strings"
	"time"

	goredis "github.com/go-redis/redis/v8"
)

const runtimeCounterTTL = 2 * time.Minute

type RuntimeCounterStore struct {
	rdb    *goredis.Client
	prefix string
	id     string
}

func NewRuntimeCounterStore(rdb *goredis.Client, prefix string) *RuntimeCounterStore {
	return &RuntimeCounterStore{
		rdb:    rdb,
		prefix: prefix,
		id:     runtimeCounterInstanceID(),
	}
}

func (s *RuntimeCounterStore) key(name string) string {
	return s.prefix + name
}

func (s *RuntimeCounterStore) instanceKey(name string) string {
	return s.key(name) + ":" + s.id
}

func (s *RuntimeCounterStore) Inc(name string) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	pipe := s.rdb.TxPipeline()
	pipe.Incr(ctx, s.instanceKey(name))
	pipe.Expire(ctx, s.instanceKey(name), runtimeCounterTTL)
	_, _ = pipe.Exec(ctx)
}

func (s *RuntimeCounterStore) Dec(name string) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	val, err := s.rdb.Decr(ctx, s.instanceKey(name)).Result()
	if err != nil {
		return
	}
	if val < 0 {
		_ = s.rdb.Set(ctx, s.instanceKey(name), 0, runtimeCounterTTL).Err()
		return
	}
	_ = s.rdb.Expire(ctx, s.instanceKey(name), runtimeCounterTTL).Err()
}

func (s *RuntimeCounterStore) Snapshot() map[string]int {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	out := make(map[string]int, len(videoappCounterNames()))
	for _, name := range videoappCounterNames() {
		keys, err := s.rdb.Keys(ctx, s.key(name)+":*").Result()
		if err != nil {
			out[name] = 0
			continue
		}
		total := 0
		if len(keys) > 0 {
			vals, err := s.rdb.MGet(ctx, keys...).Result()
			if err != nil {
				out[name] = 0
				continue
			}
			for _, val := range vals {
				if val == nil {
					continue
				}
				n, err := strconv.Atoi(strings.TrimSpace(val.(string)))
				if err != nil {
					continue
				}
				if n > 0 {
					total += n
				}
			}
		}
		out[name] = total
	}
	return out
}

func (s *RuntimeCounterStore) Reset() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	for _, name := range videoappCounterNames() {
		keys, err := s.rdb.Keys(ctx, s.key(name)+":*").Result()
		if err != nil || len(keys) == 0 {
			continue
		}
		_ = s.rdb.Del(ctx, keys...).Err()
	}
}

func runtimeCounterInstanceID() string {
	host, _ := os.Hostname()
	host = strings.TrimSpace(host)
	if host == "" {
		host = "worker"
	}
	return host + "-" + strconv.FormatInt(time.Now().UnixNano(), 10)
}

func videoappCounterNames() []string {
	return []string{
		"transcode_tasks_active",
		"vector_tasks_active",
		"vector_coarse_clip_active",
		"vector_coarse_upload_active",
		"vector_coarse_asr_active",
		"vector_refine_asr_active",
	}
}
