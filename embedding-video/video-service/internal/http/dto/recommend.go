package dto

type RecommendationItem struct {
	QuestionID     uint64  `json:"question_id"`
	VideoID        uint64  `json:"video_id"`
	VideoSegmentID uint64  `json:"video_segment_id"`
	RecommendScore float64 `json:"recommend_score"`
	IsWatched      bool    `json:"is_watched"`
	WatchDuration  int     `json:"watch_duration"`
	StartTimeSec   int     `json:"start_time_sec"`
	EndTimeSec     int     `json:"end_time_sec"`
	Title          string  `json:"title"`
	CoverURL       string  `json:"cover_url"`
	PlayURL        string  `json:"play_url"`
}

type RecommendByQuestionRequest struct {
	QuestionID   uint64 `json:"question_id"`
	QuestionText string `json:"question_text"`
	UserID       uint64 `json:"user_id"`
	Limit        int    `json:"limit"`
}

type ReportWatchRequest struct {
	QuestionID     uint64 `json:"question_id"`
	UserID         uint64 `json:"user_id"`
	VideoSegmentID uint64 `json:"video_segment_id"`
	IsWatched      bool   `json:"is_watched"`
	WatchDuration  int    `json:"watch_duration"`
}

type RecommendationListData struct {
	Items    []RecommendationItem `json:"items"`
	Total    int                  `json:"total"`
	Degraded bool                 `json:"degraded,omitempty"`
	Message  string               `json:"message,omitempty"`
}

type WatchRecordData struct {
	Recorded bool `json:"recorded"`
}

type QuestionItem struct {
	ID               uint64 `json:"id"`
	Source           string `json:"source"`
	SourceQuestionID string `json:"source_question_id"`
	Content          string `json:"content"`
	Answer           string `json:"answer"`
	Analysis         string `json:"analysis"`
	Knowledge        string `json:"knowledge"`
	Subject          string `json:"subject"`
	Type             string `json:"type"`
	Status           int16  `json:"status"`
	CreateTime       int64  `json:"create_time"`
	UpdateTime       int64  `json:"update_time"`
}

type QuestionListData struct {
	Total    int64          `json:"total"`
	Page     int            `json:"page"`
	PageSize int            `json:"page_size"`
	List     []QuestionItem `json:"list"`
}

type QuestionDetailData struct {
	Question QuestionItem `json:"question"`
}

type RecommendationListResponse struct {
	Success bool                   `json:"success"`
	Data    RecommendationListData `json:"data"`
}

type WatchRecordResponse struct {
	Success bool            `json:"success"`
	Data    WatchRecordData `json:"data"`
}

type QuestionListResponse struct {
	Success bool             `json:"success"`
	Data    QuestionListData `json:"data"`
}

type QuestionDetailResponse struct {
	Success bool               `json:"success"`
	Data    QuestionDetailData `json:"data"`
}
