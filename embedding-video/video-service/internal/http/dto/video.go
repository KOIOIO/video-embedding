package dto

type VideoItem struct {
	VideoID       uint64 `json:"video_id"`
	Title         string `json:"title"`
	Description   string `json:"description"`
	RawURL        string `json:"raw_url"`
	HLSURL        string `json:"hls_url"`
	IsPublished   bool   `json:"is_published"`
	IsRecommend   bool   `json:"is_recommend"`
	ViewCount     int64  `json:"view_count"`
	CoverURL      string `json:"cover_url"`
	CreatedAtUnix int64  `json:"created_at_unix"`
	UpdatedAtUnix int64  `json:"updated_at_unix"`
}

type UpdateVideoRequest struct {
	Title       string `json:"title" binding:"required,max=200"`
	Description string `json:"description" binding:"max=5000"`
}

type PublishVideoRequest struct {
	IsPublished *bool `json:"is_published"`
}

type RecommendVideoRequest struct {
	IsRecommend    *bool   `json:"is_recommend"`
	UserID         uint64  `json:"user_id"`
	RecommendLevel int16   `json:"recommend_level"`
	RecommendScore float64 `json:"recommend_score"`
}

type VideoReactionRequest struct {
	UserID       uint64 `json:"user_id"`
	ReactionType string `json:"reaction_type"`
}

type VideoListData struct {
	Videos []VideoItem `json:"videos"`
	Total  int         `json:"total"`
	Type   string      `json:"type"`
}

type UpdateVideoData struct {
	VideoID     uint64 `json:"video_id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Updated     bool   `json:"updated"`
}

type DeleteVideoData struct {
	VideoID uint64 `json:"video_id"`
	Deleted bool   `json:"deleted"`
}

type PlayVideoData struct {
	PlayURL string    `json:"play_url"`
	Video   VideoItem `json:"video"`
}

type SimilarVideosData struct {
	Videos []VideoItem `json:"videos"`
	Total  int         `json:"total"`
}

type ViewCountData struct {
	VideoID   uint64 `json:"video_id"`
	ViewCount int64  `json:"view_count"`
}

type PublishVideoData struct {
	VideoID     uint64 `json:"video_id"`
	IsPublished bool   `json:"is_published"`
	Updated     bool   `json:"updated"`
}

type RecommendVideoData struct {
	VideoID     uint64 `json:"video_id"`
	IsRecommend bool   `json:"is_recommend"`
	Updated     bool   `json:"updated"`
}

type VideoReactionData struct {
	VideoID         uint64 `json:"video_id"`
	UserID          uint64 `json:"user_id"`
	ReactionType    string `json:"reaction_type"`
	Active          bool   `json:"active"`
	LikeCount       int64  `json:"like_count"`
	DoubleLikeCount int64  `json:"double_like_count"`
	Updated         bool   `json:"updated"`
}

type VideoReactionCountsData struct {
	VideoID         uint64 `json:"video_id"`
	LikeCount       int64  `json:"like_count"`
	DoubleLikeCount int64  `json:"double_like_count"`
}

type SegmentReactionData struct {
	SegmentID       uint64 `json:"segment_id"`
	UserID          uint64 `json:"user_id"`
	ReactionType    string `json:"reaction_type"`
	Active          bool   `json:"active"`
	LikeCount       int64  `json:"like_count"`
	DoubleLikeCount int64  `json:"double_like_count"`
	Updated         bool   `json:"updated"`
}

type SegmentReactionCountsData struct {
	SegmentID       uint64 `json:"segment_id"`
	LikeCount       int64  `json:"like_count"`
	DoubleLikeCount int64  `json:"double_like_count"`
}

type RandomVideoSegmentData struct {
	VideoID          uint64 `json:"video_id"`
	VideoSegmentID   uint64 `json:"video_segment_id"`
	StartTimeSec     int    `json:"start_time_sec"`
	EndTimeSec       int    `json:"end_time_sec"`
	Title            string `json:"title"`
	CoverURL         string `json:"cover_url"`
	PlayURL          string `json:"play_url"`
	UserReacted      bool   `json:"user_reacted"`
	UserReactionType string `json:"user_reaction_type"`
}

type TranscodeStatusData struct {
	TaskID string `json:"task_id"`
	Status string `json:"status"`
	HLSURL string `json:"hls_url"`
}

type VideoListResponse struct {
	Success bool          `json:"success"`
	Data    VideoListData `json:"data"`
}

type UpdateVideoResponse struct {
	Success bool            `json:"success"`
	Data    UpdateVideoData `json:"data"`
}

type DeleteVideoResponse struct {
	Success bool            `json:"success"`
	Data    DeleteVideoData `json:"data"`
}

type PlayVideoResponse struct {
	Success bool          `json:"success"`
	Data    PlayVideoData `json:"data"`
}

type SimilarVideosResponse struct {
	Success bool              `json:"success"`
	Data    SimilarVideosData `json:"data"`
}

type ViewCountResponse struct {
	Success bool          `json:"success"`
	Data    ViewCountData `json:"data"`
}

type PublishVideoResponse struct {
	Success bool             `json:"success"`
	Data    PublishVideoData `json:"data"`
}

type RecommendVideoResponse struct {
	Success bool               `json:"success"`
	Data    RecommendVideoData `json:"data"`
}

type VideoReactionResponse struct {
	Success bool              `json:"success"`
	Data    VideoReactionData `json:"data"`
}

type VideoReactionCountsResponse struct {
	Success bool                    `json:"success"`
	Data    VideoReactionCountsData `json:"data"`
}

type SegmentReactionResponse struct {
	Success bool                `json:"success"`
	Data    SegmentReactionData `json:"data"`
}

type SegmentReactionCountsResponse struct {
	Success bool                      `json:"success"`
	Data    SegmentReactionCountsData `json:"data"`
}

type RandomVideoSegmentResponse struct {
	Success bool                   `json:"success"`
	Data    RandomVideoSegmentData `json:"data"`
}

type TranscodeStatusResponse struct {
	Success bool                `json:"success"`
	Data    TranscodeStatusData `json:"data"`
}
