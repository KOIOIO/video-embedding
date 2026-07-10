package videoapp

import (
	"context"
	"io"
	"time"

	"github.com/pgvector/pgvector-go"

	domainvideo "nlp-video-analysis/internal/domain/video"
)

// VideoRepository 视频仓库接口
type VideoRepository interface {
	Create(ctx context.Context, v *domainvideo.Video) error
	List(ctx context.Context, filter ListFilter) ([]domainvideo.Video, error)
	ListRecommendPool(ctx context.Context) ([]domainvideo.Video, error)
	GetByID(ctx context.Context, id uint64) (domainvideo.Video, bool, error)
	DeleteByID(ctx context.Context, id uint64) (bool, error)
	UpdateMetadata(ctx context.Context, id uint64, title string, description string) (bool, error)
	UpdatePublished(ctx context.Context, id uint64, isPublished bool) (bool, error)
	UpdateRecommend(ctx context.Context, id uint64, isRecommend bool, userID uint64, recommendLevel int16, recommendScore float64) (bool, error)
	IncrementViewCount(ctx context.Context, id uint64) (int, bool, error)
	GetViewCount(ctx context.Context, id uint64) (int, bool, error)
	SubmitVideoReaction(ctx context.Context, videoID uint64, userID uint64, reactionType VideoReactionType) (bool, bool, error)
	ApplyVideoReactionState(ctx context.Context, videoID uint64, userID uint64, reactionType VideoReactionType, active bool) (bool, error)
	GetVideoUserReaction(ctx context.Context, videoID uint64, userID uint64) (VideoReactionType, bool, bool, error)
	GetVideoReactionCounts(ctx context.Context, videoID uint64) (VideoReactionCounts, bool, error)
	FindSimilar(ctx context.Context, id uint64, limit int) ([]domainvideo.Video, error)
	UpdateCoverByID(ctx context.Context, id uint64, coverURL string) (bool, error)
	UpdateStatusByID(ctx context.Context, id uint64, status domainvideo.Status, errMsg string) error
	GetSegmentEmbeddingDim(ctx context.Context) (int, error)
	GetQuestionEmbeddingTextByID(ctx context.Context, questionID uint64) (string, error)
	ListQuestions(ctx context.Context, page int, pageSize int) (QuestionPage, error)
	GetQuestionByID(ctx context.Context, id uint64) (QuestionItem, bool, error)
	FindRecommendedSegments(ctx context.Context, query pgvector.Vector, limit int) ([]RecommendCandidate, error)
	FindRecommendedSegmentsByWeakKnowledge(ctx context.Context, userID uint64, limit int, weakLimit int) ([]RecommendCandidate, error)
	SaveUserVideoRecommendation(ctx context.Context, userID uint64, questionID uint64, videoID uint64, segmentID uint64, score float64, now time.Time) error
	ListRecommendations(ctx context.Context, userID uint64, questionID uint64, limit int) ([]RecommendationRecord, error)
	GetVideoIDBySegmentID(ctx context.Context, segmentID uint64) (uint64, error)
	HasWatchedVideoForQuestion(ctx context.Context, userID uint64, questionID uint64, videoID uint64) (bool, error)
	SaveWatchRecord(ctx context.Context, userID uint64, videoID uint64, questionID uint64, segmentID uint64, isWatched bool, watchDuration int, now time.Time) (bool, error)
}

type VideoUploadPermissionRepository interface {
	CanUploadVideo(ctx context.Context, userID uint64) (bool, error)
}

type VideoReactionStateRepository interface {
	ApplyVideoReactionState(ctx context.Context, videoID uint64, userID uint64, reactionType VideoReactionType, active bool) (bool, error)
}

type VideoProfileRepository interface {
	GetUserVideoProfile(ctx context.Context, userID uint64, modelVersion string) (UserVideoProfile, bool, error)
	FindRecommendedSegmentsForProfileRerank(ctx context.Context, input ProfileRerankQuery) ([]ProfileRerankCandidate, error)
}

type RecBoleRepository interface {
	GetUserRecBoleEmbedding(ctx context.Context, userID uint64, modelVersion string) (UserRecBoleEmbedding, bool, error)
	FindRecommendedSegmentsForRecBole(ctx context.Context, input RecBoleQuery) ([]RecBoleCandidate, error)
}

type GorseHydrationRepository interface {
	HydrateRecommendedSegmentsByID(ctx context.Context, userID uint64, ids []uint64) ([]RecommendCandidate, error)
}

type WeakKnowledgeVectorRepository interface {
	ListWeakKnowledge(ctx context.Context, userID uint64, limit int) ([]WeakKnowledge, error)
	FindRecommendedSegmentsByWeakKnowledgeVector(ctx context.Context, input WeakKnowledgeVectorQuery) ([]RecommendCandidate, error)
}

type RecBoleModelVersionRepository interface {
	GetActiveRecBoleModelVersion(ctx context.Context) (string, bool, error)
}

type ArchiveProcessingProgress struct {
	Total      int
	Transcoded int
	Vectorized int
}

type ArchiveProcessingProgressRepository interface {
	GetArchiveProcessingProgress(ctx context.Context, videoIDs []uint64) (ArchiveProcessingProgress, error)
}

type RecommendationExposureRepository interface {
	SaveRecommendationExposures(ctx context.Context, exposures []RecommendationExposure) error
	MarkRecommendationExposureWatched(ctx context.Context, userID uint64, questionID uint64, segmentID uint64, now time.Time) error
}

type SegmentReactionRepository interface {
	SubmitSegmentReaction(ctx context.Context, segmentID uint64, userID uint64, reactionType VideoReactionType) (bool, bool, error)
	GetSegmentUserReaction(ctx context.Context, segmentID uint64, userID uint64) (VideoReactionType, bool, bool, error)
	GetSegmentReactionCounts(ctx context.Context, segmentID uint64) (VideoReactionCounts, bool, error)
}

type SegmentReactionStateRepository interface {
	ApplySegmentReactionState(ctx context.Context, segmentID uint64, userID uint64, reactionType VideoReactionType, active bool) (bool, error)
}

type VideoReactionType string

const (
	VideoReactionLike       VideoReactionType = "like"
	VideoReactionDoubleLike VideoReactionType = "double_like"
	VideoReactionDislike    VideoReactionType = "dislike"
)

func (t VideoReactionType) IsValid() bool {
	switch t {
	case VideoReactionLike, VideoReactionDoubleLike, VideoReactionDislike:
		return true
	default:
		return false
	}
}

type VideoReactionCounts struct {
	LikeCount       int64
	DoubleLikeCount int64
}

type VideoReactionResult struct {
	Active       bool
	Counts       VideoReactionCounts
	ReactionType VideoReactionType
}

type VideoReactionEvent struct {
	VideoID      uint64            `json:"video_id"`
	UserID       uint64            `json:"user_id"`
	ReactionType VideoReactionType `json:"reaction_type"`
	Active       bool              `json:"active"`
	Retry        int               `json:"retry,omitempty"`
}

type VideoReactionQueueMessage struct {
	MessageID string
	Event     VideoReactionEvent
}

type VideoReactionStore interface {
	HasCounts(ctx context.Context, videoID uint64) (bool, error)
	HasUserReaction(ctx context.Context, videoID uint64, userID uint64) (bool, error)
	GetUserReaction(ctx context.Context, videoID uint64, userID uint64) (VideoReactionType, bool, bool, error)
	Submit(ctx context.Context, videoID uint64, userID uint64, reactionType VideoReactionType, seed VideoReactionCounts, seedUserReaction VideoReactionType, seedUserActive bool) (VideoReactionResult, error)
	GetCounts(ctx context.Context, videoID uint64, seed VideoReactionCounts) (VideoReactionCounts, error)
}

type VideoReactionQueue interface {
	Enqueue(ctx context.Context, event VideoReactionEvent) error
	Dequeue(ctx context.Context) (VideoReactionQueueMessage, error)
	Ack(ctx context.Context, id string) error
	Requeue(ctx context.Context, msg VideoReactionQueueMessage, delay time.Duration, reason string) error
	MoveToDeadLetter(ctx context.Context, msg VideoReactionQueueMessage, reason string) error
}

// TextEmbedder 抽象文本向量化能力，供推荐场景按题目文本生成查询向量。
type TextEmbedder interface {
	Embed(ctx context.Context, text string) ([]float32, error)
}

// ListFilter 定义视频列表查询时的过滤模式。
type ListFilter int

// 列表过滤常量。
const (
	ListAll ListFilter = iota
	ListRawOnly
	ListHLSOnly
)

// TranscodeQueue 抽象转码任务投递能力。
type TranscodeQueue interface {
	Enqueue(ctx context.Context, task TranscodeTask) error
}

// VectorizeQueue 抽象向量化任务投递能力。
type VectorizeQueue interface {
	Enqueue(ctx context.Context, task VectorizeTask) error
}

// ObjectStore 抽象对象存储最小能力，供上传、封面和转码产物落库使用。
type ObjectStore interface {
	PutFile(ctx context.Context, objectKey string, filePath string, contentType string) error
	Put(ctx context.Context, objectKey string, r io.Reader, size int64, contentType string) error
	Delete(ctx context.Context, objectKey string) error
}

// TranscodeStatusStore 抽象转码状态读写能力。
type TranscodeStatusStore interface {
	Set(ctx context.Context, taskID string, status domainvideo.Status, hlsURL string, ttl time.Duration) error
	Get(ctx context.Context, taskID string) (TranscodeStatus, bool, error)
}

// FileStorage 抽象本地文件系统操作，便于上传与转码过程管理临时文件。
type FileStorage interface {
	MkdirAll(path string) error
	Create(path string) (io.WriteCloser, error)
	RemoveAll(path string) error
	Remove(path string) error
}
