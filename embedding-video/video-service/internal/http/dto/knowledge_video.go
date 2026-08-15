package dto

import "video-service/internal/application/knowledgevideo"

type KnowledgeVideoImportData struct {
	BatchID     uint64 `json:"batch_id"`
	TotalCount  int    `json:"total_count"`
	Status      string `json:"status"`
	ProgressURL string `json:"progress_url"`
}

type KnowledgeVideoItem struct {
	ID                 uint64 `json:"id"`
	KnowledgePointID   uint64 `json:"knowledge_point_id"`
	KnowledgePointName string `json:"knowledge_point_name"`
	VideoName          string `json:"video_name"`
	Duration           int    `json:"duration"`
	Status             string `json:"status"`
	ErrorMessage       string `json:"error_message,omitempty"`
}

type KnowledgeVideoBatchData struct {
	BatchID     uint64               `json:"batch_id"`
	TotalCount  int                  `json:"total_count"`
	ReadyCount  int                  `json:"ready_count"`
	FailedCount int                  `json:"failed_count"`
	Status      string               `json:"status"`
	Videos      []KnowledgeVideoItem `json:"videos"`
}

type KnowledgeVideoPlaybackItem struct {
	KnowledgeVideoID uint64 `json:"knowledge_video_id"`
	SourceFileName   string `json:"source_file_name"`
	DisplayName      string `json:"display_name"`
	Duration         int    `json:"duration"`
	PlaybackURL      string `json:"playback_url"`
}

type KnowledgeVideoPlaybackData struct {
	KnowledgePointID   uint64                       `json:"knowledge_point_id"`
	KnowledgePointName string                       `json:"knowledge_point_name"`
	Videos             []KnowledgeVideoPlaybackItem `json:"videos"`
	VideoID            *uint64                      `json:"video_id,omitempty"`
	VideoName          *string                      `json:"video_name,omitempty"`
	Duration           *int                         `json:"duration,omitempty"`
	PlaybackURL        *string                      `json:"playback_url,omitempty"`
}

type KnowledgeVideoPlaybackRecordRequest struct {
	UserID uint64 `json:"user_id"`
}

type KnowledgeVideoPlaybackRecordData struct{}

type KnowledgeVideoPlaybackRecordResponse struct {
	Success bool                             `json:"success"`
	Data    KnowledgeVideoPlaybackRecordData `json:"data"`
}

type KnowledgeVideoWatchSessionRequest struct {
	UserID         uint64 `json:"user_id"`
	WatchedSeconds *int   `json:"watched_seconds"`
}

type KnowledgeVideoWatchSessionData struct {
	SessionID             string  `json:"session_id"`
	SessionWatchedSeconds int     `json:"session_watched_seconds"`
	TotalWatchedSeconds   int     `json:"total_watched_seconds"`
	DurationSeconds       int     `json:"duration_seconds"`
	ProgressRatio         float64 `json:"progress_ratio"`
	EffectiveWatch        bool    `json:"effective_watch"`
}

type KnowledgeVideoWatchSessionResponse struct {
	Success bool                           `json:"success"`
	Data    KnowledgeVideoWatchSessionData `json:"data"`
}

type KnowledgeVideoTreeVideo struct {
	ID           uint64 `json:"id"`
	BatchID      uint64 `json:"batch_id"`
	VideoName    string `json:"video_name"`
	Duration     int    `json:"duration"`
	Status       string `json:"status"`
	ErrorMessage string `json:"error_message,omitempty"`
}

type KnowledgeVideoTreeNode struct {
	ID       uint64                    `json:"id"`
	ParentID uint64                    `json:"parent_id"`
	Name     string                    `json:"name"`
	Children []KnowledgeVideoTreeNode  `json:"children"`
	Videos   []KnowledgeVideoTreeVideo `json:"videos"`
	Video    *KnowledgeVideoTreeVideo  `json:"video,omitempty"`
}

type KnowledgeVideoTreeData struct {
	Nodes []KnowledgeVideoTreeNode `json:"nodes"`
}

type KnowledgeVideoTreeResponse struct {
	Success bool                   `json:"success"`
	Data    KnowledgeVideoTreeData `json:"data"`
}

type KnowledgeVideoImportResponse struct {
	Success bool                     `json:"success"`
	Data    KnowledgeVideoImportData `json:"data"`
}

type KnowledgeVideoBatchResponse struct {
	Success bool                    `json:"success"`
	Data    KnowledgeVideoBatchData `json:"data"`
}

type KnowledgeVideoPlaybackResponse struct {
	Success bool                       `json:"success"`
	Data    KnowledgeVideoPlaybackData `json:"data"`
}

type KnowledgeVideoValidationError struct {
	Success bool `json:"success"`
	Error   struct {
		Code    string                           `json:"code"`
		Message string                           `json:"message"`
		Issues  []knowledgevideo.ValidationIssue `json:"issues"`
	} `json:"error"`
}
