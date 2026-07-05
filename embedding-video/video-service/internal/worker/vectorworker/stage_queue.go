package vectorworker

import (
	"time"

	"nlp-video-analysis/internal/config"
	infraredis "nlp-video-analysis/internal/infrastructure/redis"

	goredis "github.com/go-redis/redis/v8"
)

type vectorStageQueues map[string]*infraredis.StreamQueue[VectorStageTask]

func newVectorStageQueues(rdb *goredis.Client, cfg config.Config) vectorStageQueues {
	queues := make(vectorStageQueues, len(vectorStageOrder))
	pendingMinIdle := time.Duration(cfg.VectorWorker.TaskTimeoutMinutes) * time.Minute
	if pendingMinIdle <= 0 {
		pendingMinIdle = 3 * time.Hour
	}
	pendingMinIdle += time.Minute
	for _, stage := range vectorStageOrder {
		key := vectorStageQueueKeyFromConfig(cfg, stage)
		queues[stage] = infraredis.NewStreamQueue[VectorStageTask](rdb, infraredis.StreamQueueOptions{
			Key:            key,
			Group:          key + ":group",
			PendingMinIdle: pendingMinIdle,
		})
	}
	return queues
}

func (qs vectorStageQueues) queue(stage string) *infraredis.StreamQueue[VectorStageTask] {
	if qs == nil {
		return nil
	}
	return qs[stage]
}
