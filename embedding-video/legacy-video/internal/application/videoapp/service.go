package videoapp

import (
	"time"
)

// Service 是视频领域的应用服务入口。
// 它负责编排仓储、队列、对象存储与向量能力，而不直接关心 HTTP 或 gRPC 协议细节。
type Service struct {
	Repo        VideoRepository
	Queue       TranscodeQueue
	VectorQueue VectorizeQueue
	StatusStore TranscodeStatusStore
	Store       ObjectStore
	FS          FileStorage
	Embedder    TextEmbedder
	Paths       Paths
	Now         func() time.Time
	StatusTTL   time.Duration
	DeleteLocal bool
}

// NewService 创建应用服务，并注入运行期所需的基础设施依赖。
func NewService(repo VideoRepository, queue TranscodeQueue, vectorQueue VectorizeQueue, statusStore TranscodeStatusStore, store ObjectStore, fs FileStorage, embedder TextEmbedder, paths Paths) *Service {
	return &Service{
		Repo:        repo,
		Queue:       queue,
		VectorQueue: vectorQueue,
		StatusStore: statusStore,
		Store:       store,
		FS:          fs,
		Embedder:    embedder,
		Paths:       paths,
		Now:         time.Now,
		StatusTTL:   24 * time.Hour,
		DeleteLocal: true,
	}
}

