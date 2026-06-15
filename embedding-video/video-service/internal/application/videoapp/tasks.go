package videoapp

import domainvideo "nlp-video-analysis/internal/domain/video"

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

type VectorizeQueueMessage struct {
	MessageID string
	Task      VectorizeTask
}
