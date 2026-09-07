package persistence

import (
	"context"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"video-service/internal/application/videoapp"
	"video-service/internal/model"
)

func newCommentRepoTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.EduVideoResource{}, &model.EduVideoSegment{}, &model.EduVideoComment{}, &model.EduCommentLike{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if err := db.Exec(`CREATE TABLE sys_user (id INTEGER PRIMARY KEY, username TEXT NOT NULL DEFAULT '', real_name TEXT NOT NULL DEFAULT '', deleted INTEGER NOT NULL DEFAULT 0)`).Error; err != nil {
		t.Fatalf("create sys_user: %v", err)
	}
	if err := db.Exec(`INSERT INTO sys_user (id, username, real_name, deleted) VALUES (1, 'admin', '系统管理员', 0), (2, 'student1', '', 0), (3, 'deleted_user', '已删除', 1)`).Error; err != nil {
		t.Fatalf("seed sys_user: %v", err)
	}
	return db
}

func TestCommentRepositoryCreateAndList(t *testing.T) {
	ctx := context.Background()
	db := newCommentRepoTestDB(t)
	if err := db.Create(&model.EduVideoSegment{ID: 10, VideoID: 1, SegmentIndex: 0, StartTimeSec: 0, EndTimeSec: 10, Status: 1}).Error; err != nil {
		t.Fatalf("seed segment: %v", err)
	}
	repo := NewGormVideoRepository(db)

	exists, err := repo.SegmentExists(ctx, 10)
	if err != nil || !exists {
		t.Fatalf("SegmentExists(10) = %v, %v; want true", exists, err)
	}
	exists, err = repo.SegmentExists(ctx, 404)
	if err != nil || exists {
		t.Fatalf("SegmentExists(404) = %v, %v; want false", exists, err)
	}

	var replyRootID uint64
	for i := 0; i < 3; i++ {
		id, err := repo.InsertComment(ctx, &videoapp.Comment{UserID: uint64(i + 1), VideoSegmentID: 10, Content: "top"})
		if err != nil {
			t.Fatalf("insert comment %d: %v", i, err)
		}
		if i == 0 {
			replyRootID = id
			if _, err := repo.InsertComment(ctx, &videoapp.Comment{UserID: 2, VideoSegmentID: 10, RootID: id, ParentID: id, ReplyToUserID: 1, Content: "reply"}); err != nil {
				t.Fatalf("insert reply: %v", err)
			}
		}
	}

	comments, total, err := repo.ListTopComments(ctx, 10, 1, 2)
	if err != nil {
		t.Fatalf("list top comments: %v", err)
	}
	if total != 3 || len(comments) != 2 {
		t.Fatalf("top comments total=%d len=%d, want 3/2", total, len(comments))
	}

	replies, replyTotal, err := repo.ListReplies(ctx, replyRootID, 1, 10)
	if err != nil {
		t.Fatalf("list replies: %v", err)
	}
	if replyTotal != 1 || len(replies) != 1 {
		t.Fatalf("replies total=%d len=%d, want 1/1", replyTotal, len(replies))
	}
	if replies[0].ReplyToUserID != 1 {
		t.Fatalf("reply_to_user_id = %d, want 1", replies[0].ReplyToUserID)
	}

	count, err := repo.CountComments(ctx, 10)
	if err != nil {
		t.Fatalf("count comments: %v", err)
	}
	if count != 4 {
		t.Fatalf("comment count = %d, want 4", count)
	}
}

func TestCommentRepositoryApplyReactionStateSwitchesAndCancels(t *testing.T) {
	ctx := context.Background()
	db := newCommentRepoTestDB(t)
	if err := db.Create(&model.EduVideoComment{ID: 20, UserID: 1, VideoSegmentID: 10, Content: "c"}).Error; err != nil {
		t.Fatalf("seed comment: %v", err)
	}
	repo := NewGormVideoRepository(db)

	apply := func(reactionType videoapp.VideoReactionType, active bool) bool {
		t.Helper()
		found, err := repo.ApplyCommentReactionState(ctx, 20, 7, reactionType, active)
		if err != nil {
			t.Fatalf("apply reaction %s active=%v: %v", reactionType, active, err)
		}
		return found
	}
	counts := func() videoapp.VideoReactionCounts {
		t.Helper()
		result, err := repo.GetCommentReactionCounts(ctx, []uint64{20})
		if err != nil {
			t.Fatalf("get counts: %v", err)
		}
		return result[20]
	}
	reactionOf := func() videoapp.VideoReactionType {
		t.Helper()
		result, err := repo.GetUserCommentReactionTypes(ctx, []uint64{20}, 7)
		if err != nil {
			t.Fatalf("get reaction: %v", err)
		}
		return result[20]
	}

	if !apply(videoapp.VideoReactionLike, true) {
		t.Fatal("apply like: want found")
	}
	if !apply(videoapp.VideoReactionLike, true) {
		t.Fatal("re-apply like: want found")
	}
	if got := counts(); got.LikeCount != 1 || got.DoubleLikeCount != 0 {
		t.Fatalf("counts after like = %+v, want like=1", got)
	}
	if got := reactionOf(); got != videoapp.VideoReactionLike {
		t.Fatalf("reaction after like = %q, want like", got)
	}

	if !apply(videoapp.VideoReactionDoubleLike, true) {
		t.Fatal("switch to double like: want found")
	}
	if got := counts(); got.LikeCount != 0 || got.DoubleLikeCount != 1 {
		t.Fatalf("counts after switch = %+v, want double=1", got)
	}
	if got := reactionOf(); got != videoapp.VideoReactionDoubleLike {
		t.Fatalf("reaction after switch = %q, want double_like", got)
	}

	if !apply(videoapp.VideoReactionDislike, true) {
		t.Fatal("switch to dislike: want found")
	}
	if got := counts(); got.LikeCount != 0 || got.DoubleLikeCount != 0 {
		t.Fatalf("counts after dislike = %+v, want all zero", got)
	}
	if got := reactionOf(); got != videoapp.VideoReactionDislike {
		t.Fatalf("reaction after dislike = %q, want dislike", got)
	}

	if !apply(videoapp.VideoReactionDislike, false) {
		t.Fatal("cancel dislike: want found")
	}
	if !apply(videoapp.VideoReactionDislike, false) {
		t.Fatal("re-cancel dislike: want found")
	}
	if got := reactionOf(); got != "" {
		t.Fatalf("reaction after cancel = %q, want none", got)
	}

	found, err := repo.ApplyCommentReactionState(ctx, 404, 7, videoapp.VideoReactionLike, true)
	if err != nil {
		t.Fatalf("apply reaction on missing comment: %v", err)
	}
	if found {
		t.Fatal("missing comment should report not found")
	}
}

func TestCommentRepositoryGetUserNames(t *testing.T) {
	ctx := context.Background()
	db := newCommentRepoTestDB(t)
	repo := NewGormVideoRepository(db)

	names, err := repo.GetUserNamesByIDs(ctx, []uint64{1, 2, 3, 404})
	if err != nil {
		t.Fatalf("get user names: %v", err)
	}
	if names[1] != "系统管理员" {
		t.Fatalf("names[1] = %q, want 系统管理员", names[1])
	}
	if names[2] != "student1" {
		t.Fatalf("names[2] = %q, want student1 (fallback to username)", names[2])
	}
	if _, ok := names[3]; ok {
		t.Fatal("deleted user should be excluded")
	}
	if _, ok := names[404]; ok {
		t.Fatal("missing user should be excluded")
	}
}
