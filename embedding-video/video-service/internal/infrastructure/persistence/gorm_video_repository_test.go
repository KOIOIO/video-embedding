package persistence

import (
	"context"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"nlp-video-analysis/internal/application/videoapp"
	domainvideo "nlp-video-analysis/internal/domain/video"
	"nlp-video-analysis/internal/model"
)

func newVideoRepoTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.EduVideoResource{}, &model.EduVideoUserReaction{}, &model.EduUserReaction{}, &model.EduVideoSegment{}, &model.EduUserVideoRecommend{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func seedVideoResource(t *testing.T, db *gorm.DB, id uint64) {
	t.Helper()
	if err := db.Create(&model.EduVideoResource{
		ID:       id,
		Title:    "video",
		VideoURL: "/videos/raw/2026/06/02/demo.mp4",
		Status:   1,
	}).Error; err != nil {
		t.Fatalf("seed video: %v", err)
	}
}

func seedVideoSegment(t *testing.T, db *gorm.DB, id uint64, videoID uint64) {
	t.Helper()
	seedVideoResource(t, db, videoID)
	if err := db.Create(&model.EduVideoSegment{
		ID:           id,
		VideoID:      videoID,
		SegmentIndex: 1,
		StartTimeSec: 10,
		EndTimeSec:   40,
		Status:       1,
		Deleted:      0,
	}).Error; err != nil {
		t.Fatalf("seed segment: %v", err)
	}
}

func TestFindRandomPlayableSegmentFiltersDeletedUnpublishedAndNotDoneRows(t *testing.T) {
	ctx := context.Background()
	db := newVideoRepoTestDB(t)
	repo := NewGormVideoRepository(db)

	videos := []model.EduVideoResource{
		{ID: 11, Title: "playable video", VideoURL: "/videos/raw/2026/06/09/playable.mp4", CoverURL: "/covers/11.jpg", Status: int16(domainvideo.StatusDone), IsPublish: true, Deleted: 0},
		{ID: 12, Title: "deleted video", VideoURL: "/videos/raw/2026/06/09/deleted.mp4", Status: int16(domainvideo.StatusDone), IsPublish: true, Deleted: 1},
		{ID: 13, Title: "unpublished video", VideoURL: "/videos/raw/2026/06/09/unpublished.mp4", Status: int16(domainvideo.StatusDone), IsPublish: false, Deleted: 0},
		{ID: 14, Title: "processing video", VideoURL: "/videos/raw/2026/06/09/processing.mp4", Status: int16(domainvideo.StatusProcessing), IsPublish: true, Deleted: 0},
	}
	if err := db.Create(&videos).Error; err != nil {
		t.Fatalf("seed videos: %v", err)
	}
	if err := db.Model(&model.EduVideoResource{}).Where("id = ?", 13).Update("is_published", false).Error; err != nil {
		t.Fatalf("mark video unpublished: %v", err)
	}
	segments := []model.EduVideoSegment{
		{ID: 101, VideoID: 11, SegmentIndex: 1, StartTimeSec: 10, EndTimeSec: 40, ContentSummary: "playable segment", Status: 1, Deleted: 0},
		{ID: 102, VideoID: 11, SegmentIndex: 2, StartTimeSec: 40, EndTimeSec: 80, ContentSummary: "deleted segment", Status: 1, Deleted: 1},
		{ID: 103, VideoID: 12, SegmentIndex: 1, StartTimeSec: 10, EndTimeSec: 40, ContentSummary: "video deleted", Status: 1, Deleted: 0},
		{ID: 104, VideoID: 13, SegmentIndex: 1, StartTimeSec: 10, EndTimeSec: 40, ContentSummary: "video unpublished", Status: 1, Deleted: 0},
		{ID: 105, VideoID: 14, SegmentIndex: 1, StartTimeSec: 10, EndTimeSec: 40, ContentSummary: "video not done", Status: 1, Deleted: 0},
	}
	if err := db.Create(&segments).Error; err != nil {
		t.Fatalf("seed segments: %v", err)
	}

	result, found, err := repo.FindRandomPlayableSegment(ctx)
	if err != nil {
		t.Fatalf("FindRandomPlayableSegment returned error: %v", err)
	}
	if !found {
		t.Fatal("expected a playable segment")
	}
	if result.VideoSegmentID != 101 || result.VideoID != 11 {
		t.Fatalf("unexpected segment result: %+v", result)
	}
	if result.StartTimeSec != 10 || result.EndTimeSec != 40 {
		t.Fatalf("unexpected segment times: %+v", result)
	}
	if result.TitleOverride != "playable segment" {
		t.Fatalf("expected segment title override, got %q", result.TitleOverride)
	}
	if result.Video.Title != "playable video" || result.Video.CoverURL != "/covers/11.jpg" {
		t.Fatalf("unexpected video data: %+v", result.Video)
	}
}

func TestFindRandomPlayableSegmentFallsBackToVideoTitleAndReportsMissing(t *testing.T) {
	ctx := context.Background()
	db := newVideoRepoTestDB(t)
	repo := NewGormVideoRepository(db)

	_, found, err := repo.FindRandomPlayableSegment(ctx)
	if err != nil {
		t.Fatalf("FindRandomPlayableSegment empty returned error: %v", err)
	}
	if found {
		t.Fatal("expected no segment in empty database")
	}

	if err := db.Create(&model.EduVideoResource{
		ID:        21,
		Title:     "fallback title",
		VideoURL:  "/videos/raw/2026/06/09/fallback.mp4",
		Status:    int16(domainvideo.StatusDone),
		IsPublish: true,
		Deleted:   0,
	}).Error; err != nil {
		t.Fatalf("seed video: %v", err)
	}
	if err := db.Create(&model.EduVideoSegment{
		ID:           201,
		VideoID:      21,
		SegmentIndex: 1,
		StartTimeSec: 5,
		EndTimeSec:   35,
		Status:       1,
		Deleted:      0,
	}).Error; err != nil {
		t.Fatalf("seed segment: %v", err)
	}

	result, found, err := repo.FindRandomPlayableSegment(ctx)
	if err != nil {
		t.Fatalf("FindRandomPlayableSegment returned error: %v", err)
	}
	if !found {
		t.Fatal("expected a segment after seeding")
	}
	if result.TitleOverride != "fallback title" {
		t.Fatalf("expected video title fallback, got %q", result.TitleOverride)
	}
}

func TestSubmitVideoReactionCreatesFirstLike(t *testing.T) {
	ctx := context.Background()
	db := newVideoRepoTestDB(t)
	seedVideoResource(t, db, 1)
	repo := NewGormVideoRepository(db)

	active, found, err := repo.SubmitVideoReaction(ctx, 1, 7, videoapp.VideoReactionLike)
	if err != nil {
		t.Fatalf("SubmitVideoReaction returned error: %v", err)
	}
	if !active || !found {
		t.Fatalf("unexpected result: active=%v found=%v", active, found)
	}

	var resource model.EduVideoResource
	if err := db.First(&resource, 1).Error; err != nil {
		t.Fatalf("load resource: %v", err)
	}
	if resource.LikeCount != 1 || resource.DoubleLikeCount != 0 || resource.DislikeCount != 0 {
		t.Fatalf("unexpected counters: %+v", resource)
	}

	var reaction model.EduVideoUserReaction
	if err := db.Where("user_id = ? AND video_id = ?", 7, 1).First(&reaction).Error; err != nil {
		t.Fatalf("load reaction: %v", err)
	}
	if reaction.ReactionType != string(videoapp.VideoReactionLike) || reaction.Deleted != 0 {
		t.Fatalf("unexpected reaction: %+v", reaction)
	}
}

func TestSubmitVideoReactionSwitchesDoubleLikeToLike(t *testing.T) {
	ctx := context.Background()
	db := newVideoRepoTestDB(t)
	seedVideoResource(t, db, 2)
	repo := NewGormVideoRepository(db)

	if _, _, err := repo.SubmitVideoReaction(ctx, 2, 7, videoapp.VideoReactionDoubleLike); err != nil {
		t.Fatalf("first reaction: %v", err)
	}
	active, found, err := repo.SubmitVideoReaction(ctx, 2, 7, videoapp.VideoReactionLike)
	if err != nil {
		t.Fatalf("switch reaction: %v", err)
	}
	if !active || !found {
		t.Fatalf("unexpected result: active=%v found=%v", active, found)
	}

	var resource model.EduVideoResource
	if err := db.First(&resource, 2).Error; err != nil {
		t.Fatalf("load resource: %v", err)
	}
	if resource.LikeCount != 1 || resource.DoubleLikeCount != 0 || resource.DislikeCount != 0 {
		t.Fatalf("unexpected counters: %+v", resource)
	}
}

func TestSubmitVideoReactionRepeatingSameReactionCancelsIt(t *testing.T) {
	ctx := context.Background()
	db := newVideoRepoTestDB(t)
	seedVideoResource(t, db, 3)
	repo := NewGormVideoRepository(db)

	if _, _, err := repo.SubmitVideoReaction(ctx, 3, 7, videoapp.VideoReactionLike); err != nil {
		t.Fatalf("first reaction: %v", err)
	}
	active, found, err := repo.SubmitVideoReaction(ctx, 3, 7, videoapp.VideoReactionLike)
	if err != nil {
		t.Fatalf("cancel reaction: %v", err)
	}
	if active || !found {
		t.Fatalf("unexpected cancel result: active=%v found=%v", active, found)
	}

	var resource model.EduVideoResource
	if err := db.First(&resource, 3).Error; err != nil {
		t.Fatalf("load resource: %v", err)
	}
	if resource.LikeCount != 0 || resource.DoubleLikeCount != 0 || resource.DislikeCount != 0 {
		t.Fatalf("unexpected counters: %+v", resource)
	}

	var reaction model.EduVideoUserReaction
	if err := db.Where("user_id = ? AND video_id = ?", 7, 3).First(&reaction).Error; err != nil {
		t.Fatalf("load reaction: %v", err)
	}
	if reaction.Deleted != 1 {
		t.Fatalf("expected soft-deleted reaction, got %+v", reaction)
	}
}

func TestSubmitVideoReactionRevivesSoftDeletedReactionRow(t *testing.T) {
	ctx := context.Background()
	db := newVideoRepoTestDB(t)
	seedVideoResource(t, db, 4)
	repo := NewGormVideoRepository(db)

	if _, _, err := repo.SubmitVideoReaction(ctx, 4, 7, videoapp.VideoReactionLike); err != nil {
		t.Fatalf("first reaction: %v", err)
	}
	if _, _, err := repo.SubmitVideoReaction(ctx, 4, 7, videoapp.VideoReactionLike); err != nil {
		t.Fatalf("cancel reaction: %v", err)
	}
	active, found, err := repo.SubmitVideoReaction(ctx, 4, 7, videoapp.VideoReactionDoubleLike)
	if err != nil {
		t.Fatalf("revive reaction: %v", err)
	}
	if !active || !found {
		t.Fatalf("unexpected result: active=%v found=%v", active, found)
	}

	var reactions []model.EduVideoUserReaction
	if err := db.Where("user_id = ? AND video_id = ?", 7, 4).Find(&reactions).Error; err != nil {
		t.Fatalf("load reactions: %v", err)
	}
	if len(reactions) != 1 {
		t.Fatalf("reaction rows = %d, want 1", len(reactions))
	}
	if reactions[0].ReactionType != string(videoapp.VideoReactionDoubleLike) || reactions[0].Deleted != 0 {
		t.Fatalf("unexpected reaction: %+v", reactions[0])
	}

	var resource model.EduVideoResource
	if err := db.First(&resource, 4).Error; err != nil {
		t.Fatalf("load resource: %v", err)
	}
	if resource.LikeCount != 0 || resource.DoubleLikeCount != 1 || resource.DislikeCount != 0 {
		t.Fatalf("unexpected counters: %+v", resource)
	}
}

func TestGetVideoReactionCountsReadsOnlyResourceCounters(t *testing.T) {
	ctx := context.Background()
	db := newVideoRepoTestDB(t)
	if err := db.Create(&model.EduVideoResource{
		ID:              5,
		Title:           "video",
		VideoURL:        "/videos/raw/2026/06/02/demo.mp4",
		Status:          1,
		LikeCount:       5,
		DoubleLikeCount: 2,
		DislikeCount:    9,
	}).Error; err != nil {
		t.Fatalf("seed video: %v", err)
	}
	repo := NewGormVideoRepository(db)

	counts, found, err := repo.GetVideoReactionCounts(ctx, 5)
	if err != nil {
		t.Fatalf("GetVideoReactionCounts returned error: %v", err)
	}
	if !found {
		t.Fatal("expected found=true")
	}
	if counts.LikeCount != 5 || counts.DoubleLikeCount != 2 {
		t.Fatalf("unexpected counts: %+v", counts)
	}
}

func TestSubmitVideoReactionReturnsNotFoundWhenVideoMissing(t *testing.T) {
	ctx := context.Background()
	db := newVideoRepoTestDB(t)
	repo := NewGormVideoRepository(db)

	active, found, err := repo.SubmitVideoReaction(ctx, 404, 7, videoapp.VideoReactionLike)
	if err != nil {
		t.Fatalf("SubmitVideoReaction returned error: %v", err)
	}
	if active || found {
		t.Fatalf("unexpected result: active=%v found=%v", active, found)
	}
}

func TestApplyVideoReactionStateIsIdempotentForActiveEvent(t *testing.T) {
	ctx := context.Background()
	db := newVideoRepoTestDB(t)
	seedVideoResource(t, db, 6)
	repo := NewGormVideoRepository(db)

	for i := 0; i < 2; i++ {
		found, err := repo.ApplyVideoReactionState(ctx, 6, 7, videoapp.VideoReactionLike, true)
		if err != nil {
			t.Fatalf("apply active %d: %v", i, err)
		}
		if !found {
			t.Fatalf("apply active %d returned found=false", i)
		}
	}

	var resource model.EduVideoResource
	if err := db.First(&resource, 6).Error; err != nil {
		t.Fatalf("load resource: %v", err)
	}
	if resource.LikeCount != 1 || resource.DoubleLikeCount != 0 || resource.DislikeCount != 0 {
		t.Fatalf("unexpected counters after duplicate active event: %+v", resource)
	}
}

func TestApplyVideoReactionStateIsIdempotentForCancelEvent(t *testing.T) {
	ctx := context.Background()
	db := newVideoRepoTestDB(t)
	seedVideoResource(t, db, 7)
	repo := NewGormVideoRepository(db)

	if found, err := repo.ApplyVideoReactionState(ctx, 7, 7, videoapp.VideoReactionLike, true); err != nil || !found {
		t.Fatalf("apply active: found=%v err=%v", found, err)
	}
	for i := 0; i < 2; i++ {
		found, err := repo.ApplyVideoReactionState(ctx, 7, 7, videoapp.VideoReactionLike, false)
		if err != nil {
			t.Fatalf("apply cancel %d: %v", i, err)
		}
		if !found {
			t.Fatalf("apply cancel %d returned found=false", i)
		}
	}

	var resource model.EduVideoResource
	if err := db.First(&resource, 7).Error; err != nil {
		t.Fatalf("load resource: %v", err)
	}
	if resource.LikeCount != 0 || resource.DoubleLikeCount != 0 || resource.DislikeCount != 0 {
		t.Fatalf("unexpected counters after duplicate cancel event: %+v", resource)
	}
}

func TestGetVideoUserReactionReturnsInactiveSeedForSoftDeletedReaction(t *testing.T) {
	ctx := context.Background()
	db := newVideoRepoTestDB(t)
	seedVideoResource(t, db, 8)
	repo := NewGormVideoRepository(db)

	if _, _, err := repo.SubmitVideoReaction(ctx, 8, 7, videoapp.VideoReactionLike); err != nil {
		t.Fatalf("first reaction: %v", err)
	}
	if _, _, err := repo.SubmitVideoReaction(ctx, 8, 7, videoapp.VideoReactionLike); err != nil {
		t.Fatalf("cancel reaction: %v", err)
	}
	reactionType, active, found, err := repo.GetVideoUserReaction(ctx, 8, 7)
	if err != nil {
		t.Fatalf("GetVideoUserReaction returned error: %v", err)
	}
	if !found {
		t.Fatal("expected found=true")
	}
	if reactionType != videoapp.VideoReactionLike || active {
		t.Fatalf("unexpected user reaction seed: type=%q active=%v", reactionType, active)
	}
}

func TestHasWatchedVideoForQuestionScansExistingRecord(t *testing.T) {
	ctx := context.Background()
	db := newVideoRepoTestDB(t)
	repo := NewGormVideoRepository(db)

	if err := db.Create(&model.EduUserVideoRecommend{
		UserID:     1,
		QuestionID: 1756,
		VideoID:    14,
		Deleted:    0,
	}).Error; err != nil {
		t.Fatalf("seed recommendation: %v", err)
	}

	ok, err := repo.HasWatchedVideoForQuestion(ctx, 1, 1756, 14)
	if err != nil {
		t.Fatalf("HasWatchedVideoForQuestion returned error: %v", err)
	}
	if !ok {
		t.Fatal("expected existing record to return true")
	}
}

func TestSubmitSegmentReactionCreatesFirstLike(t *testing.T) {
	ctx := context.Background()
	db := newVideoRepoTestDB(t)
	seedVideoSegment(t, db, 101, 201)
	repo := NewGormVideoRepository(db)

	active, found, err := repo.SubmitSegmentReaction(ctx, 101, 7, videoapp.VideoReactionLike)
	if err != nil {
		t.Fatalf("SubmitSegmentReaction returned error: %v", err)
	}
	if !active || !found {
		t.Fatalf("unexpected result: active=%v found=%v", active, found)
	}

	var segment model.EduVideoSegment
	if err := db.Model(&model.EduVideoSegment{}).
		Select("id", "like_count", "double_like_count", "dislike_count").
		First(&segment, 101).Error; err != nil {
		t.Fatalf("load segment: %v", err)
	}
	if segment.LikeCount != 1 || segment.DoubleLikeCount != 0 || segment.DislikeCount != 0 {
		t.Fatalf("unexpected segment counters: %+v", segment)
	}

	var reaction model.EduUserReaction
	if err := db.Where("user_id = ? AND video_segment_id = ?", 7, 101).First(&reaction).Error; err != nil {
		t.Fatalf("load reaction: %v", err)
	}
	if reaction.VideoID != 201 || reaction.ReactionType != string(videoapp.VideoReactionLike) || reaction.Deleted != 0 {
		t.Fatalf("unexpected reaction: %+v", reaction)
	}
}

func TestSubmitSegmentReactionSwitchesAndCancels(t *testing.T) {
	ctx := context.Background()
	db := newVideoRepoTestDB(t)
	seedVideoSegment(t, db, 102, 202)
	repo := NewGormVideoRepository(db)

	if _, _, err := repo.SubmitSegmentReaction(ctx, 102, 7, videoapp.VideoReactionDoubleLike); err != nil {
		t.Fatalf("first reaction: %v", err)
	}
	active, found, err := repo.SubmitSegmentReaction(ctx, 102, 7, videoapp.VideoReactionLike)
	if err != nil {
		t.Fatalf("switch reaction: %v", err)
	}
	if !active || !found {
		t.Fatalf("unexpected switch result: active=%v found=%v", active, found)
	}
	active, found, err = repo.SubmitSegmentReaction(ctx, 102, 7, videoapp.VideoReactionLike)
	if err != nil {
		t.Fatalf("cancel reaction: %v", err)
	}
	if active || !found {
		t.Fatalf("unexpected cancel result: active=%v found=%v", active, found)
	}

	var segment model.EduVideoSegment
	if err := db.Model(&model.EduVideoSegment{}).
		Select("id", "like_count", "double_like_count", "dislike_count").
		First(&segment, 102).Error; err != nil {
		t.Fatalf("load segment: %v", err)
	}
	if segment.LikeCount != 0 || segment.DoubleLikeCount != 0 || segment.DislikeCount != 0 {
		t.Fatalf("unexpected segment counters: %+v", segment)
	}

	var reactions []model.EduUserReaction
	if err := db.Where("user_id = ? AND video_segment_id = ?", 7, 102).Find(&reactions).Error; err != nil {
		t.Fatalf("load reactions: %v", err)
	}
	if len(reactions) != 1 || reactions[0].Deleted != 1 {
		t.Fatalf("unexpected reactions: %+v", reactions)
	}
}

func TestApplySegmentReactionStateIsIdempotent(t *testing.T) {
	ctx := context.Background()
	db := newVideoRepoTestDB(t)
	seedVideoSegment(t, db, 103, 203)
	repo := NewGormVideoRepository(db)

	for i := 0; i < 2; i++ {
		found, err := repo.ApplySegmentReactionState(ctx, 103, 7, videoapp.VideoReactionLike, true)
		if err != nil {
			t.Fatalf("apply active %d: %v", i, err)
		}
		if !found {
			t.Fatalf("apply active %d returned found=false", i)
		}
	}
	for i := 0; i < 2; i++ {
		found, err := repo.ApplySegmentReactionState(ctx, 103, 7, videoapp.VideoReactionLike, false)
		if err != nil {
			t.Fatalf("apply cancel %d: %v", i, err)
		}
		if !found {
			t.Fatalf("apply cancel %d returned found=false", i)
		}
	}

	var segment model.EduVideoSegment
	if err := db.Model(&model.EduVideoSegment{}).
		Select("id", "like_count", "double_like_count", "dislike_count").
		First(&segment, 103).Error; err != nil {
		t.Fatalf("load segment: %v", err)
	}
	if segment.LikeCount != 0 || segment.DoubleLikeCount != 0 || segment.DislikeCount != 0 {
		t.Fatalf("unexpected counters after duplicate events: %+v", segment)
	}
}

func TestGetSegmentReactionCountsReadsSegmentCounters(t *testing.T) {
	ctx := context.Background()
	db := newVideoRepoTestDB(t)
	seedVideoSegment(t, db, 104, 204)
	if err := db.Model(&model.EduVideoSegment{}).
		Where("id = ?", 104).
		Updates(map[string]any{"like_count": 5, "double_like_count": 2, "dislike_count": 9}).Error; err != nil {
		t.Fatalf("seed counters: %v", err)
	}
	repo := NewGormVideoRepository(db)

	counts, found, err := repo.GetSegmentReactionCounts(ctx, 104)
	if err != nil {
		t.Fatalf("GetSegmentReactionCounts returned error: %v", err)
	}
	if !found {
		t.Fatal("expected found=true")
	}
	if counts.LikeCount != 5 || counts.DoubleLikeCount != 2 {
		t.Fatalf("unexpected counts: %+v", counts)
	}
}
