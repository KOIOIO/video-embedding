package videoapp

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

type commentTestRepo struct {
	inserted        []Comment
	segmentExists   map[uint64]bool
	commentByID     map[uint64]Comment
	topComments     []Comment
	topTotal        int64
	replies         []Comment
	replyTotal      int64
	reactionCounts  map[uint64]VideoReactionCounts
	reactionTypes   map[uint64]VideoReactionType
	names           map[uint64]string
	appliedReactions []CommentReactionApply
	listCalls       int
}

type CommentReactionApply struct {
	CommentID    uint64
	UserID       uint64
	ReactionType VideoReactionType
	Active       bool
}

func (r *commentTestRepo) InsertComment(_ context.Context, comment *Comment) (uint64, error) {
	comment.ID = uint64(len(r.inserted) + 1)
	comment.CreatedAt = time.Unix(1700000000+int64(comment.ID)*10, 0)
	r.inserted = append(r.inserted, *comment)
	return comment.ID, nil
}

func (r *commentTestRepo) GetCommentByID(_ context.Context, id uint64) (Comment, bool, error) {
	comment, ok := r.commentByID[id]
	return comment, ok, nil
}

func (r *commentTestRepo) SegmentExists(_ context.Context, segmentID uint64) (bool, error) {
	return r.segmentExists[segmentID], nil
}

func (r *commentTestRepo) ListTopComments(_ context.Context, segmentID uint64, page int, pageSize int) ([]Comment, int64, error) {
	return r.topComments, r.topTotal, nil
}

func (r *commentTestRepo) ListReplies(_ context.Context, rootID uint64, page int, pageSize int) ([]Comment, int64, error) {
	return r.replies, r.replyTotal, nil
}

func (r *commentTestRepo) CountComments(_ context.Context, segmentID uint64) (int64, error) {
	return r.topTotal + r.replyTotal, nil
}

func (r *commentTestRepo) GetCommentReactionCounts(_ context.Context, commentIDs []uint64) (map[uint64]VideoReactionCounts, error) {
	return r.reactionCounts, nil
}

func (r *commentTestRepo) GetUserCommentReactionTypes(_ context.Context, commentIDs []uint64, userID uint64) (map[uint64]VideoReactionType, error) {
	return r.reactionTypes, nil
}

func (r *commentTestRepo) ApplyCommentReactionState(_ context.Context, commentID uint64, userID uint64, reactionType VideoReactionType, active bool) (bool, error) {
	r.appliedReactions = append(r.appliedReactions, CommentReactionApply{CommentID: commentID, UserID: userID, ReactionType: reactionType, Active: active})
	return true, nil
}

func (r *commentTestRepo) GetUserNamesByIDs(_ context.Context, userIDs []uint64) (map[uint64]string, error) {
	return r.names, nil
}

type commentLikeStoreStub struct {
	counts         map[uint64]VideoReactionCounts
	userReactions  map[uint64]VideoReactionType
	hasCounts      map[uint64]bool
	hasUserReacts  map[uint64]bool
	submitted      []uint64
}

func (s *commentLikeStoreStub) HasCounts(_ context.Context, commentID uint64) (bool, error) {
	return s.hasCounts[commentID], nil
}

func (s *commentLikeStoreStub) HasUserReaction(_ context.Context, commentID uint64, userID uint64) (bool, error) {
	return s.hasUserReacts[commentID], nil
}

func (s *commentLikeStoreStub) GetUserReaction(_ context.Context, commentID uint64, userID uint64) (VideoReactionType, bool, bool, error) {
	reactionType, ok := s.userReactions[commentID]
	return reactionType, ok, ok, nil
}

func (s *commentLikeStoreStub) SeedUserReaction(_ context.Context, commentID uint64, userID uint64, reactionType VideoReactionType, active bool) error {
	if active {
		s.userReactions[commentID] = reactionType
	} else {
		delete(s.userReactions, commentID)
	}
	return nil
}

func (s *commentLikeStoreStub) Submit(_ context.Context, commentID uint64, userID uint64, reactionType VideoReactionType, seed VideoReactionCounts, seedUserReaction VideoReactionType, seedUserActive bool) (VideoReactionResult, error) {
	s.submitted = append(s.submitted, commentID)
	s.hasCounts[commentID] = true
	s.hasUserReacts[commentID] = true
	counts, hasCounts := s.counts[commentID]
	if !hasCounts {
		counts = seed
	}
	oldType, hasOld := s.userReactions[commentID]
	if !hasOld {
		if seedUserActive {
			oldType = seedUserReaction
		} else {
			oldType = ""
		}
	}
	active := oldType != reactionType
	switch oldType {
	case VideoReactionLike:
		counts.LikeCount--
	case VideoReactionDoubleLike:
		counts.DoubleLikeCount--
	}
	if active {
		switch reactionType {
		case VideoReactionLike:
			counts.LikeCount++
		case VideoReactionDoubleLike:
			counts.DoubleLikeCount++
		}
		s.userReactions[commentID] = reactionType
	} else {
		delete(s.userReactions, commentID)
	}
	if counts.LikeCount < 0 {
		counts.LikeCount = 0
	}
	if counts.DoubleLikeCount < 0 {
		counts.DoubleLikeCount = 0
	}
	s.counts[commentID] = counts
	return VideoReactionResult{Active: active, ReactionType: reactionType, Counts: counts}, nil
}

func (s *commentLikeStoreStub) GetCounts(_ context.Context, commentID uint64, seed VideoReactionCounts) (VideoReactionCounts, error) {
	if _, ok := s.counts[commentID]; !ok {
		s.counts[commentID] = seed
	}
	return s.counts[commentID], nil
}

type commentCountStoreStub struct {
	values map[uint64]int64
	seeded map[uint64]int64
}

func (s *commentCountStoreStub) Get(_ context.Context, segmentID uint64) (int64, bool, error) {
	value, ok := s.values[segmentID]
	return value, ok, nil
}

func (s *commentCountStoreStub) Seed(_ context.Context, segmentID uint64, count int64, ttl time.Duration) error {
	s.values[segmentID] = count
	s.seeded[segmentID] = count
	return nil
}

func (s *commentCountStoreStub) Incr(_ context.Context, segmentID uint64) error {
	if _, ok := s.values[segmentID]; ok {
		s.values[segmentID]++
	}
	return nil
}

func newCommentService(repo *commentTestRepo) *Service {
	svc := &Service{CommentRepo: repo, Now: time.Now, CommentCountTTL: time.Minute}
	return svc
}

func TestCreateCommentValidatesAndInserts(t *testing.T) {
	ctx := context.Background()
	repo := &commentTestRepo{segmentExists: map[uint64]bool{10: true}, names: map[uint64]string{7: "管理员"}}
	countStore := &commentCountStoreStub{values: map[uint64]int64{10: 5}}
	svc := newCommentService(repo)
	svc.CommentCountStore = countStore

	comment, err := svc.CreateComment(ctx, 10, 7, "  讲得不错  ")
	if err != nil {
		t.Fatalf("create comment: %v", err)
	}
	if comment.Content != "讲得不错" || comment.ID == 0 || comment.RootID != 0 || comment.ParentID != 0 {
		t.Fatalf("unexpected comment: %+v", comment)
	}
	if comment.Username != "管理员" {
		t.Fatalf("comment username = %q, want 管理员", comment.Username)
	}
	if countStore.values[10] != 6 {
		t.Fatalf("segment comment count = %d, want 6", countStore.values[10])
	}

	if _, err := svc.CreateComment(ctx, 10, 7, "   "); !isValidationError(err) {
		t.Fatalf("empty content error = %v, want validation error", err)
	}
	if _, err := svc.CreateComment(ctx, 10, 7, strings.Repeat("长", 501)); !isValidationError(err) {
		t.Fatalf("oversize content error = %v, want validation error", err)
	}
	if _, err := svc.CreateComment(ctx, 404, 7, "hi"); !errors.Is(err, ErrSegmentNotFound) {
		t.Fatalf("missing segment error = %v, want ErrSegmentNotFound", err)
	}
	if _, err := svc.CreateComment(ctx, 10, 0, "hi"); !isValidationError(err) {
		t.Fatalf("missing user error = %v, want validation error", err)
	}
}

func TestCreateReplyKeepsTwoLevels(t *testing.T) {
	ctx := context.Background()
	repo := &commentTestRepo{
		segmentExists: map[uint64]bool{},
		commentByID: map[uint64]Comment{
			1: {ID: 1, UserID: 1, VideoSegmentID: 10, RootID: 0, ParentID: 0, Content: "top"},
			2: {ID: 2, UserID: 2, VideoSegmentID: 10, RootID: 1, ParentID: 1, ReplyToUserID: 1, Content: "reply"},
		},
	}
	svc := newCommentService(repo)

	reply, err := svc.CreateReply(ctx, 2, 3, "回复二级评论")
	if err != nil {
		t.Fatalf("create reply: %v", err)
	}
	if reply.RootID != 1 {
		t.Fatalf("reply root = %d, want 1 (stay under top-level)", reply.RootID)
	}
	if reply.ParentID != 2 {
		t.Fatalf("reply parent = %d, want 2", reply.ParentID)
	}
	if reply.ReplyToUserID != 2 {
		t.Fatalf("reply_to_user = %d, want 2", reply.ReplyToUserID)
	}

	if _, err := svc.CreateReply(ctx, 404, 3, "hi"); !errors.Is(err, ErrCommentNotFound) {
		t.Fatalf("missing comment error = %v, want ErrCommentNotFound", err)
	}
}

func TestListSegmentCommentsDecoratesViews(t *testing.T) {
	ctx := context.Background()
	top := Comment{ID: 1, UserID: 1, VideoSegmentID: 10, Content: "top", CreatedAt: time.Unix(1700000000, 0)}
	reply := Comment{ID: 2, UserID: 2, VideoSegmentID: 10, RootID: 1, ParentID: 1, ReplyToUserID: 1, Content: "reply", CreatedAt: time.Unix(1700000010, 0)}
	repo := &commentTestRepo{
		topComments:    []Comment{top},
		topTotal:       1,
		replies:        []Comment{reply},
		replyTotal:     2,
		reactionCounts: map[uint64]VideoReactionCounts{1: {LikeCount: 4, DoubleLikeCount: 2}, 2: {LikeCount: 1}},
		reactionTypes:  map[uint64]VideoReactionType{1: VideoReactionLike},
		names:          map[uint64]string{1: "管理员", 2: "学生"},
	}
	svc := newCommentService(repo)

	list, err := svc.ListSegmentComments(ctx, 10, 7, 1, 10)
	if err != nil {
		t.Fatalf("list comments: %v", err)
	}
	if list.Total != 1 || len(list.Comments) != 1 {
		t.Fatalf("unexpected list: %+v", list)
	}
	view := list.Comments[0]
	if view.Username != "管理员" || view.LikeCount != 4 || view.DoubleLikeCount != 2 || view.UserReactionType != VideoReactionLike {
		t.Fatalf("unexpected top view: %+v", view)
	}
	if view.ReplyCount != 2 || !view.HasMoreReplies || len(view.Replies) != 1 {
		t.Fatalf("unexpected reply info: %+v", view)
	}
	replyView := view.Replies[0]
	if replyView.Username != "学生" || replyView.ReplyToUsername != "管理员" || replyView.LikeCount != 1 {
		t.Fatalf("unexpected reply view: %+v", replyView)
	}
}

func TestToggleCommentLikeUsesStore(t *testing.T) {
	ctx := context.Background()
	repo := &commentTestRepo{
		commentByID:    map[uint64]Comment{1: {ID: 1, UserID: 1, VideoSegmentID: 10, Content: "top"}},
		reactionCounts: map[uint64]VideoReactionCounts{1: {LikeCount: 3}},
		reactionTypes:  map[uint64]VideoReactionType{},
	}
	store := &commentLikeStoreStub{
		counts:        map[uint64]VideoReactionCounts{},
		userReactions: map[uint64]VideoReactionType{},
		hasCounts:     map[uint64]bool{},
		hasUserReacts: map[uint64]bool{},
	}
	svc := newCommentService(repo)
	svc.CommentLikeStore = store

	result, err := svc.ToggleCommentLike(ctx, 1, 7, VideoReactionLike)
	if err != nil {
		t.Fatalf("toggle like: %v", err)
	}
	if !result.Active || result.Counts.LikeCount != 4 {
		t.Fatalf("toggle like result %+v, want active like_count=4", result)
	}

	result, err = svc.ToggleCommentLike(ctx, 1, 7, VideoReactionDoubleLike)
	if err != nil {
		t.Fatalf("switch double like: %v", err)
	}
	if !result.Active || result.ReactionType != VideoReactionDoubleLike || result.Counts.LikeCount != 3 || result.Counts.DoubleLikeCount != 1 {
		t.Fatalf("switch result %+v, want double_like active like=3 double=1", result)
	}

	result, err = svc.ToggleCommentLike(ctx, 1, 7, VideoReactionDoubleLike)
	if err != nil {
		t.Fatalf("cancel double like: %v", err)
	}
	if result.Active || result.Counts.DoubleLikeCount != 0 {
		t.Fatalf("cancel result %+v, want inactive double=0", result)
	}

	result, err = svc.ToggleCommentLike(ctx, 1, 7, VideoReactionDislike)
	if err != nil {
		t.Fatalf("toggle dislike: %v", err)
	}
	if !result.Active || result.ReactionType != VideoReactionDislike || result.Counts.LikeCount != 3 {
		t.Fatalf("dislike result %+v, want active dislike like=3", result)
	}

	if len(store.submitted) != 4 {
		t.Fatalf("submitted count = %d, want 4", len(store.submitted))
	}

	if _, err := svc.ToggleCommentLike(ctx, 404, 7, VideoReactionLike); !errors.Is(err, ErrCommentNotFound) {
		t.Fatalf("missing comment error = %v, want ErrCommentNotFound", err)
	}
	if _, err := svc.ToggleCommentLike(ctx, 1, 0, VideoReactionLike); !isValidationError(err) {
		t.Fatalf("missing user error = %v, want validation error", err)
	}
	if _, err := svc.ToggleCommentLike(ctx, 1, 7, VideoReactionType("bad")); !isValidationError(err) {
		t.Fatalf("bad type error = %v, want validation error", err)
	}
}

func TestToggleCommentLikeFallsBackToRepoWithoutStore(t *testing.T) {
	ctx := context.Background()
	repo := &commentTestRepo{
		commentByID:    map[uint64]Comment{1: {ID: 1, UserID: 1, VideoSegmentID: 10, Content: "top"}},
		reactionCounts: map[uint64]VideoReactionCounts{1: {LikeCount: 2}},
		reactionTypes:  map[uint64]VideoReactionType{},
	}
	svc := newCommentService(repo)

	result, err := svc.ToggleCommentLike(ctx, 1, 7, VideoReactionLike)
	if err != nil {
		t.Fatalf("toggle like without store: %v", err)
	}
	if !result.Active || result.Counts.LikeCount != 2 {
		t.Fatalf("fallback result %+v, want active like=2", result)
	}
	if len(repo.appliedReactions) != 1 || !repo.appliedReactions[0].Active || repo.appliedReactions[0].ReactionType != VideoReactionLike {
		t.Fatalf("unexpected applied reactions: %+v", repo.appliedReactions)
	}
}

func TestGetSegmentCommentCountReadsStoreThenSeeds(t *testing.T) {
	ctx := context.Background()
	repo := &commentTestRepo{topTotal: 4, replyTotal: 2}
	countStore := &commentCountStoreStub{values: map[uint64]int64{1: 9}, seeded: map[uint64]int64{}}
	svc := newCommentService(repo)
	svc.CommentCountStore = countStore

	count, err := svc.GetSegmentCommentCount(ctx, 1)
	if err != nil {
		t.Fatalf("get cached count: %v", err)
	}
	if count != 9 {
		t.Fatalf("cached count = %d, want 9", count)
	}

	count, err = svc.GetSegmentCommentCount(ctx, 2)
	if err != nil {
		t.Fatalf("get seeded count: %v", err)
	}
	if count != 6 {
		t.Fatalf("seeded count = %d, want 6", count)
	}
	if countStore.values[2] != 6 {
		t.Fatalf("seeded store value = %d, want 6", countStore.values[2])
	}
}

func isValidationError(err error) bool {
	var validationErr ValidationError
	return errors.As(err, &validationErr)
}
