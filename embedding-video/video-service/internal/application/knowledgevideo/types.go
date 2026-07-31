package knowledgevideo

import "time"

type VideoStatus int16

const (
	VideoPending VideoStatus = iota
	VideoTranscoding
	VideoReady
	VideoFailed
)

type BatchStatus int16

const (
	BatchProcessing BatchStatus = iota + 1
	BatchCompleted
	BatchPartialFailed
	BatchFailed
)

type DictionaryKnowledgePoint struct {
	ID   uint64
	Name string
}

type KnowledgeTreeVideo struct {
	ID           uint64
	BatchID      uint64
	VideoName    string
	Duration     int
	Status       VideoStatus
	ErrorMessage string
}

type KnowledgeTreeNode struct {
	ID       uint64
	ParentID uint64
	Name     string
	Children []KnowledgeTreeNode
	Videos   []KnowledgeTreeVideo
	Video    *KnowledgeTreeVideo
}

type Batch struct {
	ID            uint64
	UploadUserID  uint64
	ZipFileName   string
	XLSXFileName  string
	XLSXObjectKey string
	TotalCount    int
	ReadyCount    int
	FailedCount   int
	Status        BatchStatus
	ErrorMessage  string
	CreateTime    time.Time
	UpdateTime    time.Time
}

type Video struct {
	ID                 uint64
	BatchID            uint64
	KnowledgePointID   uint64
	KnowledgePointName string
	SourceFileName     string
	SourceObjectKey    string
	HLSObjectPrefix    string
	HLSMasterObjectKey string
	Duration           int
	Status             VideoStatus
	ErrorMessage       string
	EnqueueTime        *time.Time
	CreateTime         time.Time
	UpdateTime         time.Time
}

type ImportRow struct {
	KnowledgePointID   uint64
	KnowledgePointName string
	SourceFileName     string
}

type ImportResult struct {
	BatchID     uint64
	TotalCount  int
	ReadyCount  int
	FailedCount int
	Status      BatchStatus
}

type PlaybackVideo struct {
	KnowledgeVideoID uint64
	SourceFileName   string
	DisplayName      string
	Duration         int
	PlaybackURL      string
}

type PlaybackResolution struct {
	KnowledgePointID   uint64
	KnowledgePointName string
	Videos             []PlaybackVideo
}

type InvalidArgument struct {
	Field string
}

func (e *InvalidArgument) Error() string { return e.Field + " must be a positive integer" }

type NotFound struct {
	Resource string
}

func (e *NotFound) Error() string { return e.Resource + " not found" }

type NotReady struct {
	Status VideoStatus
}

func (e *NotReady) Error() string { return "knowledge video is not ready" }

type TranscodeFailed struct {
	Message string
}

func (e *TranscodeFailed) Error() string {
	if e.Message == "" {
		return "knowledge video transcode failed"
	}
	return e.Message
}

type PlayRecord struct {
	UserID           uint64
	KnowledgePointID uint64
	KnowledgeVideoID uint64
	CreateTime       time.Time
}

type TranscodeTask struct {
	KnowledgeVideoID uint64 `json:"knowledge_video_id"`
	SourceObjectKey  string `json:"source_object_key"`
	HLSObjectPrefix  string `json:"hls_object_prefix"`
	TaskID           string `json:"task_id"`
	RetryCount       int    `json:"retry_count"`
}

type QueueMessage struct {
	MessageID string
	Task      TranscodeTask
}
