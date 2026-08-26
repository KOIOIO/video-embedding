package videoapp

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	// MaxCommentContentLength 限制单条评论内容的 UTF-8 字符数。
	MaxCommentContentLength = 500
	// DefaultCommentPageSize 一级评论默认分页大小。
	DefaultCommentPageSize = 10
	// MaxCommentPageSize 一级评论分页上限。
	MaxCommentPageSize = 50
	// DefaultReplyPageSize 二级回复默认分页大小。
	DefaultReplyPageSize = 3
	// MaxReplyPageSize 二级回复分页上限。
	MaxReplyPageSize = 20
	// defaultCommentCountTTL 片段评论总数缓存的回填有效期。
	defaultCommentCountTTL = 5 * time.Minute
)

var (
	ErrSegmentNotFound = errors.New("video segment not found")
	ErrCommentNotFound = errors.New("comment not found")
)

// CommentView 表示带展示信息的评论视图，供服务层组装后返回。
type CommentView struct {
	Comment
	Username         string
	ReplyToUsername  string
	UserReactionType VideoReactionType
	ReplyCount       int64
	HasMoreReplies   bool
	Replies          []CommentView
}

// CommentListView 表示评论列表及其分页信息。
type CommentListView struct {
	Total    int64
	Comments []CommentView
}

// CreateComment 为视频片段创建一条一级评论。ok=false 表示片段不存在。
func (s *Service) CreateComment(ctx context.Context, segmentID uint64, userID uint64, content string) (CommentView, error) {
	content = strings.TrimSpace(content)
	if err := validateCommentContent(content); err != nil {
		return CommentView{}, err
	}
	if userID == 0 {
		return CommentView{}, InvalidArgumentError("user_id is required")
	}
	if s.CommentRepo == nil {
		return CommentView{}, InvalidArgumentError("comment repository is required")
	}
	exists, err := s.CommentRepo.SegmentExists(ctx, segmentID)
	if err != nil {
		return CommentView{}, err
	}
	if !exists {
		return CommentView{}, ErrSegmentNotFound
	}
	comment := Comment{
		UserID:         userID,
		VideoSegmentID: segmentID,
		Content:        content,
	}
	id, err := s.CommentRepo.InsertComment(ctx, &comment)
	if err != nil {
		return CommentView{}, err
	}
	comment.ID = id
	comment.CreatedAt = s.Now()
	s.bumpSegmentCommentCount(ctx, segmentID)
	return CommentView{Comment: comment, Username: s.usernameOf(ctx, userID)}, nil
}

// CreateReply 对一条评论追加二级回复；无论被回复的是一级还是二级评论，
// 回复都挂到一级评论下，总层级保持两级。
func (s *Service) CreateReply(ctx context.Context, commentID uint64, userID uint64, content string) (CommentView, error) {
	content = strings.TrimSpace(content)
	if err := validateCommentContent(content); err != nil {
		return CommentView{}, err
	}
	if userID == 0 {
		return CommentView{}, InvalidArgumentError("user_id is required")
	}
	if s.CommentRepo == nil {
		return CommentView{}, InvalidArgumentError("comment repository is required")
	}
	parent, found, err := s.CommentRepo.GetCommentByID(ctx, commentID)
	if err != nil {
		return CommentView{}, err
	}
	if !found {
		return CommentView{}, ErrCommentNotFound
	}
	rootID := parent.RootID
	if rootID == 0 {
		rootID = parent.ID
	}
	reply := Comment{
		UserID:         userID,
		VideoSegmentID: parent.VideoSegmentID,
		RootID:         rootID,
		ParentID:       parent.ID,
		ReplyToUserID:  parent.UserID,
		Content:        content,
	}
	id, err := s.CommentRepo.InsertComment(ctx, &reply)
	if err != nil {
		return CommentView{}, err
	}
	reply.ID = id
	reply.CreatedAt = s.Now()
	s.bumpSegmentCommentCount(ctx, parent.VideoSegmentID)
	return CommentView{Comment: reply, Username: s.usernameOf(ctx, userID)}, nil
}

func (s *Service) usernameOf(ctx context.Context, userID uint64) string {
	names, err := s.CommentRepo.GetUserNamesByIDs(ctx, []uint64{userID})
	if err != nil {
		return ""
	}
	return names[userID]
}

// ListSegmentComments 分页返回片段的一级评论，并附带每条的二级回复首页。
func (s *Service) ListSegmentComments(ctx context.Context, segmentID uint64, viewerID uint64, page int, pageSize int) (CommentListView, error) {
	if s.CommentRepo == nil {
		return CommentListView{}, InvalidArgumentError("comment repository is required")
	}
	topComments, total, err := s.CommentRepo.ListTopComments(ctx, segmentID, page, pageSize)
	if err != nil {
		return CommentListView{}, err
	}
	views := make([]CommentView, 0, len(topComments))
	for _, comment := range topComments {
		view := CommentView{Comment: comment}
		replies, replyTotal, err := s.CommentRepo.ListReplies(ctx, comment.ID, 1, DefaultReplyPageSize)
		if err != nil {
			return CommentListView{}, err
		}
		view.Replies = make([]CommentView, 0, len(replies))
		for _, reply := range replies {
			view.Replies = append(view.Replies, CommentView{Comment: reply})
		}
		view.ReplyCount = replyTotal
		view.HasMoreReplies = replyTotal > int64(len(replies))
		views = append(views, view)
	}
	if err := s.decorateCommentViews(ctx, viewerID, views); err != nil {
		return CommentListView{}, err
	}
	return CommentListView{Total: total, Comments: views}, nil
}

// ListCommentReplies 分页返回一级评论下的二级回复。
func (s *Service) ListCommentReplies(ctx context.Context, commentID uint64, viewerID uint64, page int, pageSize int) (CommentListView, error) {
	if s.CommentRepo == nil {
		return CommentListView{}, InvalidArgumentError("comment repository is required")
	}
	target, found, err := s.CommentRepo.GetCommentByID(ctx, commentID)
	if err != nil {
		return CommentListView{}, err
	}
	if !found {
		return CommentListView{}, ErrCommentNotFound
	}
	rootID := target.RootID
	if rootID == 0 {
		rootID = target.ID
	}
	replies, total, err := s.CommentRepo.ListReplies(ctx, rootID, page, pageSize)
	if err != nil {
		return CommentListView{}, err
	}
	views := make([]CommentView, 0, len(replies))
	for _, reply := range replies {
		views = append(views, CommentView{Comment: reply})
	}
	if err := s.decorateCommentViews(ctx, viewerID, views); err != nil {
		return CommentListView{}, err
	}
	return CommentListView{Total: total, Comments: views}, nil
}

// ToggleCommentLike 切换用户对评论的互动（like/double_like/dislike），
// 语义与视频互动一致：同一评论同一用户只保留一种互动，再点同类型取消，点不同类型切换。
// 写路径走 Redis 缓冲：Lua 原子切换 + 计数更新，事件由 worker 异步落库。
func (s *Service) ToggleCommentLike(ctx context.Context, commentID uint64, userID uint64, reactionType VideoReactionType) (VideoReactionResult, error) {
	if commentID == 0 {
		return VideoReactionResult{}, InvalidArgumentError("comment_id is required")
	}
	if userID == 0 {
		return VideoReactionResult{}, InvalidArgumentError("user_id is required")
	}
	if !reactionType.IsValid() {
		return VideoReactionResult{}, InvalidArgumentError("reaction_type must be one of like, double_like, dislike")
	}
	if s.CommentRepo == nil {
		return VideoReactionResult{}, InvalidArgumentError("comment repository is required")
	}
	_, found, err := s.CommentRepo.GetCommentByID(ctx, commentID)
	if err != nil {
		return VideoReactionResult{}, err
	}
	if !found {
		return VideoReactionResult{}, ErrCommentNotFound
	}

	if s.CommentLikeStore != nil {
		seedCounts := VideoReactionCounts{}
		hasCounts, err := s.CommentLikeStore.HasCounts(ctx, commentID)
		if err != nil {
			return VideoReactionResult{}, err
		}
		if !hasCounts {
			counts, err := s.CommentRepo.GetCommentReactionCounts(ctx, []uint64{commentID})
			if err != nil {
				return VideoReactionResult{}, err
			}
			seedCounts = counts[commentID]
		}

		seedUserReaction := VideoReactionType("")
		seedUserActive := false
		hasUserReaction, err := s.CommentLikeStore.HasUserReaction(ctx, commentID, userID)
		if err != nil {
			return VideoReactionResult{}, err
		}
		if !hasUserReaction {
			reactions, err := s.CommentRepo.GetUserCommentReactionTypes(ctx, []uint64{commentID}, userID)
			if err != nil {
				return VideoReactionResult{}, err
			}
			seedUserReaction = reactions[commentID]
			seedUserActive = seedUserReaction != ""
		}

		return s.CommentLikeStore.Submit(ctx, commentID, userID, reactionType, seedCounts, seedUserReaction, seedUserActive)
	}

	reactions, err := s.CommentRepo.GetUserCommentReactionTypes(ctx, []uint64{commentID}, userID)
	if err != nil {
		return VideoReactionResult{}, err
	}
	active := reactions[commentID] != reactionType
	applied, err := s.CommentRepo.ApplyCommentReactionState(ctx, commentID, userID, reactionType, active)
	if err != nil {
		return VideoReactionResult{}, err
	}
	if !applied {
		return VideoReactionResult{}, ErrCommentNotFound
	}
	counts, err := s.CommentRepo.GetCommentReactionCounts(ctx, []uint64{commentID})
	if err != nil {
		return VideoReactionResult{}, err
	}
	return VideoReactionResult{Active: active, ReactionType: reactionType, Counts: counts[commentID]}, nil
}

// GetSegmentCommentCount 返回片段的评论总数，优先读 Redis 缓存，未命中时回填。
func (s *Service) GetSegmentCommentCount(ctx context.Context, segmentID uint64) (int64, error) {
	if s.CommentRepo == nil {
		return 0, InvalidArgumentError("comment repository is required")
	}
	if s.CommentCountStore != nil {
		count, found, err := s.CommentCountStore.Get(ctx, segmentID)
		if err != nil {
			return 0, err
		}
		if found {
			return count, nil
		}
	}
	count, err := s.CommentRepo.CountComments(ctx, segmentID)
	if err != nil {
		return 0, err
	}
	if s.CommentCountStore != nil {
		_ = s.CommentCountStore.Seed(ctx, segmentID, count, s.CommentCountTTL)
	}
	return count, nil
}

func (s *Service) bumpSegmentCommentCount(ctx context.Context, segmentID uint64) {
	if s.CommentCountStore == nil || segmentID == 0 {
		return
	}
	_ = s.CommentCountStore.Incr(ctx, segmentID)
}

// decorateCommentViews 批量补充用户名、回复对象、点赞数与当前用户的点赞状态。
func (s *Service) decorateCommentViews(ctx context.Context, viewerID uint64, views []CommentView) error {
	if len(views) == 0 {
		return nil
	}
	var commentIDs []uint64
	userIDSet := map[uint64]struct{}{}
	for i := range views {
		commentIDs = append(commentIDs, views[i].ID)
		userIDSet[views[i].UserID] = struct{}{}
		if views[i].ReplyToUserID != 0 {
			userIDSet[views[i].ReplyToUserID] = struct{}{}
		}
		for j := range views[i].Replies {
			commentIDs = append(commentIDs, views[i].Replies[j].ID)
			userIDSet[views[i].Replies[j].UserID] = struct{}{}
			if views[i].Replies[j].ReplyToUserID != 0 {
				userIDSet[views[i].Replies[j].ReplyToUserID] = struct{}{}
			}
		}
	}

	userIDs := make([]uint64, 0, len(userIDSet))
	for id := range userIDSet {
		userIDs = append(userIDs, id)
	}
	names, err := s.CommentRepo.GetUserNamesByIDs(ctx, userIDs)
	if err != nil {
		return err
	}

	dbCounts, err := s.CommentRepo.GetCommentReactionCounts(ctx, commentIDs)
	if err != nil {
		return err
	}
	var reactionTypes map[uint64]VideoReactionType
	if viewerID > 0 {
		reactionTypes, err = s.CommentRepo.GetUserCommentReactionTypes(ctx, commentIDs, viewerID)
		if err != nil {
			return err
		}
	}

	attach := func(views []CommentView) error {
		for i := range views {
			views[i].Username = names[views[i].UserID]
			if views[i].ReplyToUserID != 0 {
				views[i].ReplyToUsername = names[views[i].ReplyToUserID]
			}
			seedUserReaction := reactionTypes[views[i].ID]
			seedUserActive := seedUserReaction != ""
			if s.CommentLikeStore != nil {
				if err := s.seedUserReactionCache(ctx, views[i].ID, viewerID, seedUserReaction, seedUserActive); err != nil {
					return err
				}
				counts, err := s.CommentLikeStore.GetCounts(ctx, views[i].ID, dbCounts[views[i].ID])
				if err != nil {
					return err
				}
				views[i].LikeCount = counts.LikeCount
				views[i].DoubleLikeCount = counts.DoubleLikeCount
				if viewerID > 0 {
					reactionType, active, found, err := s.CommentLikeStore.GetUserReaction(ctx, views[i].ID, viewerID)
					if err != nil {
						return err
					}
					if found {
						if active {
							views[i].UserReactionType = reactionType
						}
						continue
					}
				}
			} else {
				views[i].LikeCount = dbCounts[views[i].ID].LikeCount
				views[i].DoubleLikeCount = dbCounts[views[i].ID].DoubleLikeCount
			}
			if viewerID > 0 && seedUserActive {
				views[i].UserReactionType = seedUserReaction
			}
		}
		return nil
	}

	for i := range views {
		if err := attach(views[i].Replies); err != nil {
			return err
		}
	}
	return attach(views)
}

func (s *Service) seedUserReactionCache(ctx context.Context, commentID uint64, viewerID uint64, reactionType VideoReactionType, active bool) error {
	if s.CommentLikeStore == nil || viewerID == 0 {
		return nil
	}
	has, err := s.CommentLikeStore.HasUserReaction(ctx, commentID, viewerID)
	if err != nil {
		return err
	}
	if has {
		return nil
	}
	return s.CommentLikeStore.SeedUserReaction(ctx, commentID, viewerID, reactionType, active)
}

func validateCommentContent(content string) error {
	if content == "" {
		return InvalidArgumentError("content is required")
	}
	if utf8.RuneCountInString(content) > MaxCommentContentLength {
		return InvalidArgumentError("content exceeds max length")
	}
	return nil
}
