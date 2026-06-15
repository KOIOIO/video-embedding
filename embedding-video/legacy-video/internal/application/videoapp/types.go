package videoapp

import (
	"context"
	"io"
	"time"

	"github.com/pgvector/pgvector-go"

	domainvideo "legacy-video/internal/domain/video"
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
	FindSimilar(ctx context.Context, id uint64, limit int) ([]domainvideo.Video, error)
	UpdateCoverByID(ctx context.Context, id uint64, coverURL string) (bool, error)
	UpdateStatusByID(ctx context.Context, id uint64, status domainvideo.Status, errMsg string) error
	GetSegmentEmbeddingDim(ctx context.Context) (int, error)
	GetQuestionEmbeddingTextByID(ctx context.Context, questionID uint64) (string, error)
	FindRecommendedSegments(ctx context.Context, query pgvector.Vector, limit int) ([]RecommendCandidate, error)
	SaveUserVideoRecommendation(ctx context.Context, userID uint64, questionID uint64, videoID uint64, segmentID uint64, score float64, now time.Time) error
	ListRecommendations(ctx context.Context, userID uint64, questionID uint64, limit int) ([]RecommendationRecord, error)
	GetVideoIDBySegmentID(ctx context.Context, segmentID uint64) (uint64, error)
	SaveWatchRecord(ctx context.Context, userID uint64, videoID uint64, questionID uint64, segmentID uint64, isWatched bool, watchDuration int, now time.Time) error
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

// Paths 保存本地路径与对外 URL 前缀之间的映射关系。
type Paths struct {
	RawDir       string
	HLSDir       string
	RawURLPrefix string
	HLSURLPrefix string
}

// UploadPlan 描述上传开始时计算出的所有路径、对象键与回传 URL。
type UploadPlan struct {
	OriginalFileName string
	StoredFileName   string
	DatePath         string
	RawAbsPath       string
	RawObjectKey     string
	RawURL           string
	HLSAbsDir        string
	HLSObjectPrefix  string
	HLSURL           string
	RawUploaded      bool
}

// UploadResult 是上传流程完成后返回给协议层的摘要信息。
type UploadResult struct {
	VideoID uint64
	TaskID  string
	RawURL  string
	HLSURL  string
	Name    string
}

// TranscodeTask 是发往转码队列的任务载荷。
type TranscodeTask struct {
	VideoID         uint64 `json:"video_id"`
	RawKey          string `json:"raw_key"`
	HLSObjectPrefix string `json:"hls_prefix"`
	TaskID          string `json:"task_id"`
	HLSURL          string `json:"hls_url"`
	RetryCount      int    `json:"retry_count,omitempty"`
}

// TranscodeStatus 表示缓存中的转码状态快照。
type TranscodeStatus struct {
	Status domainvideo.Status `json:"status"`
	HLSURL string             `json:"hls_url"`
}

// VectorizeTask 是发往向量化队列的任务载荷。
type VectorizeTask struct {
	VideoID uint64 `json:"video_id"`
	RawKey  string `json:"raw_key"`
	TaskID  string `json:"task_id"`
}

// UploadMeta 是上传接口额外传入的业务元信息。
type UploadMeta struct {
	Title       string
	Description string
}

// RecommendCandidate 表示数据库召回出的候选视频片段。
type RecommendCandidate struct {
	VideoSegmentID uint64
	VideoID        uint64
	StartTimeSec   int
	EndTimeSec     int
	Distance       float64
	SegmentTitle   string
	VideoURL       string
	CoverURL       string
	IsPublished    bool
	IsRecommend    bool
	ViewCount      int
	CreateTime     time.Time
	UpdateTime     time.Time
}

// RecommendResultItem 表示推荐接口对外返回的单条结果。
type RecommendResultItem struct {
	QuestionID     uint64
	VideoID        uint64
	VideoSegmentID uint64
	RecommendScore float64
	IsWatched      bool
	WatchDuration  int
	StartTimeSec   int
	EndTimeSec     int
	Video          domainvideo.Video
	TitleOverride  string
}

// RecommendationRecord 表示已保存的推荐记录及其展示信息。
type RecommendationRecord struct {
	QuestionID     uint64
	VideoID        uint64
	VideoSegmentID uint64
	RecommendScore float64
	IsWatched      bool
	WatchDuration  int
	Title          string
	VideoURL       string
	CoverURL       string
	IsPublished    bool
	IsRecommend    bool
	ViewCount      int
	CreateTime     time.Time
	UpdateTime     time.Time
}

// RecommendByQuestionInput 描述按题目做召回时的输入参数。
type RecommendByQuestionInput struct {
	QuestionID   uint64
	QuestionText string
	UserID       uint64
	Limit        int
}

// ListRecommendationsInput 描述推荐列表查询接口的输入参数。
type ListRecommendationsInput struct {
	QuestionID uint64
	UserID     uint64
	Limit      int
}

// ReportWatchInput 描述观看记录上报的输入参数。
type ReportWatchInput struct {
	QuestionID     uint64
	UserID         uint64
	VideoSegmentID uint64
	IsWatched      bool
	WatchDuration  int
}
