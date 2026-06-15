package videoapp

import (
	"io"
	"time"

	domainvideo "nlp-video-analysis/internal/domain/video"
)

// UploadMeta 是上传接口额外传入的业务元信息。
type UploadMeta struct {
	Title       string
	Description string
}

// UploadVideoInput 描述协议层直接上传视频时传给应用层的最小输入集合。
type UploadVideoInput struct {
	FileName    string
	ContentType string
	Title       string
	Description string
	Reader      io.Reader
}

type UploadVideoArchiveInput struct {
	FileName    string
	ContentType string
	Description string
	Reader      io.Reader
}

type InitiateChunkedUploadInput struct {
	FileName    string
	ContentType string
	Title       string
	Description string
	FileSize    int64
	ChunkSize   int64
	TotalChunks int
}

type UploadVideoChunkInput struct {
	UploadID   string
	ChunkIndex int
	Reader     io.Reader
}

type CompleteChunkedUploadInput struct {
	UploadID string
}

type ChunkedUploadStatus struct {
	UploadID       string
	FileName       string
	FileSize       int64
	ChunkSize      int64
	TotalChunks    int
	UploadedChunks []int
	Completed      bool
}

// UploadCoverInput 描述封面直传时传给应用层的最小输入集合。
type UploadCoverInput struct {
	FileName    string
	ContentType string
	Size        int64
	Reader      io.Reader
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
	Status         int16
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
	StartTimeSec   int
	EndTimeSec     int
	Title          string
	VideoURL       string
	CoverURL       string
	Status         int16
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
