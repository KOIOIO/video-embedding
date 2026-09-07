package dto

// MentionData 评论中 @提及 的用户信息。
type MentionData struct {
	Nickname string `json:"nickname"`
	UserID   uint64 `json:"user_id"`
}

type CommentData struct {
	ID              uint64        `json:"id"`
	UserID          uint64        `json:"user_id"`
	Username        string        `json:"username"`
	Nickname        string        `json:"nickname,omitempty"`
	AvatarURL       string        `json:"avatar_url,omitempty"`
	ReplyToUsername string        `json:"reply_to_username,omitempty"`
	Content         string        `json:"content"`
	LikeCount       int64         `json:"like_count"`
	DoubleLikeCount int64         `json:"double_like_count"`
	UserReactionType string       `json:"user_reaction_type"`
	CreatedAtUnix   int64         `json:"created_at_unix"`
	ReplyCount      int64         `json:"reply_count"`
	HasMoreReplies  bool          `json:"has_more_replies,omitempty"`
	Mentions        []MentionData `json:"mentions,omitempty"`
	Replies         []CommentData `json:"replies,omitempty"`
}

type CommentListData struct {
	Total    int64         `json:"total"`
	Page     int           `json:"page"`
	PageSize int           `json:"page_size"`
	Comments []CommentData `json:"comments"`
}

type CreateCommentRequest struct {
	Content string `json:"content" binding:"required"`
}

type CommentReactionRequest struct {
	ReactionType string `json:"reaction_type" binding:"required"`
}

type CommentLikeData struct {
	CommentID      uint64 `json:"comment_id"`
	Active         bool   `json:"active"`
	ReactionType   string `json:"reaction_type"`
	LikeCount      int64  `json:"like_count"`
	DoubleLikeCount int64  `json:"double_like_count"`
}

type SegmentCommentCountData struct {
	VideoSegmentID uint64 `json:"video_segment_id"`
	Total          int64  `json:"total"`
}

type CommentListResponse struct {
	Success bool           `json:"success"`
	Data    CommentListData `json:"data"`
}

type CommentCreateResponse struct {
	Success bool        `json:"success"`
	Data    CommentData `json:"data"`
}

type CommentLikeResponse struct {
	Success bool            `json:"success"`
	Data    CommentLikeData `json:"data"`
}

type SegmentCommentCountResponse struct {
	Success bool                    `json:"success"`
	Data    SegmentCommentCountData `json:"data"`
}
