package videoapp

import (
	"strings"
	"time"
)

// Service 是视频领域的应用服务入口。
// 它负责编排仓储、队列、对象存储与向量能力，而不直接关心 HTTP 或 gRPC 协议细节。
type Service struct {
	Repo                 VideoRepository
	SegmentReactionRepo  SegmentReactionRepository
	Queue                TranscodeQueue
	VectorQueue          VectorizeQueue
	StatusStore          TranscodeStatusStore
	ReactionStore        VideoReactionStore
	SegmentReactionStore VideoReactionStore
	Store                ObjectStore
	FS                   FileStorage
	Embedder             TextEmbedder
	Paths                Paths
	Now                  func() time.Time
	StatusTTL            time.Duration
	DeleteLocal          bool
}

// NewService 创建应用服务，并注入运行期所需的基础设施依赖。
func NewService(repo VideoRepository, queue TranscodeQueue, vectorQueue VectorizeQueue, statusStore TranscodeStatusStore, store ObjectStore, fs FileStorage, embedder TextEmbedder, paths Paths) *Service {
	svc := &Service{
		Repo:        repo,
		Queue:       queue,
		VectorQueue: vectorQueue,
		StatusStore: statusStore,
		Store:       store,
		FS:          fs,
		Embedder:    embedder,
		Paths:       normalizePaths(paths),
		Now:         time.Now,
		StatusTTL:   24 * time.Hour,
		DeleteLocal: true,
	}
	if segmentRepo, ok := repo.(SegmentReactionRepository); ok {
		svc.SegmentReactionRepo = segmentRepo
	}
	return svc
}

func normalizePaths(paths Paths) Paths {
	if strings.TrimSpace(paths.RawObjectPrefix) == "" {
		paths.RawObjectPrefix = "raw"
	}
	if strings.TrimSpace(paths.HLSObjectPrefix) == "" {
		paths.HLSObjectPrefix = "hls"
	}
	if strings.TrimSpace(paths.RawURLPrefix) == "" {
		paths.RawURLPrefix = "/videos/raw"
	}
	if strings.TrimSpace(paths.HLSURLPrefix) == "" {
		paths.HLSURLPrefix = "/videos/hls"
	}
	if strings.TrimSpace(paths.CoverURLPrefix) == "" {
		paths.CoverURLPrefix = "/videos"
	}
	if strings.TrimSpace(paths.HLSMasterName) == "" {
		paths.HLSMasterName = "master.m3u8"
	}
	return paths
}
