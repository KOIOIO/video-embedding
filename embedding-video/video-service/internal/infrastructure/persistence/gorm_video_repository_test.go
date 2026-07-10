package persistence

import (
	"context"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"nlp-video-analysis/internal/application/videoapp"
	domainvideo "nlp-video-analysis/internal/domain/video"
	"nlp-video-analysis/internal/model"
)

type sqlCaptureLogger struct {
	sql []string
}

func (l *sqlCaptureLogger) LogMode(logger.LogLevel) logger.Interface      { return l }
func (l *sqlCaptureLogger) Info(context.Context, string, ...interface{})  {}
func (l *sqlCaptureLogger) Warn(context.Context, string, ...interface{})  {}
func (l *sqlCaptureLogger) Error(context.Context, string, ...interface{}) {}
func (l *sqlCaptureLogger) Trace(_ context.Context, _ time.Time, fc func() (string, int64), _ error) {
	sql, _ := fc()
	l.sql = append(l.sql, sql)
}

func newVideoRepoTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.EduVideoResource{}, &model.EduVideoUserReaction{}, &model.EduUserReaction{}, &model.EduVideoSegment{}, &model.EduVideoVectorStage{}, &model.EduUserVideoRecommend{}, &model.EduUserVideoProfile{}, &model.EduRecommendExposure{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func TestCanUploadVideoAllowsUserTypesTwoAndThreeOnly(t *testing.T) {
	ctx := context.Background()
	db := newVideoRepoTestDB(t)
	if err := db.Exec(`CREATE TABLE sys_user (id INTEGER PRIMARY KEY, user_type INTEGER NOT NULL)`).Error; err != nil {
		t.Fatalf("create sys_user: %v", err)
	}
	if err := db.Exec(`INSERT INTO sys_user (id, user_type) VALUES (1, 1), (2, 2), (3, 3)`).Error; err != nil {
		t.Fatalf("seed sys_user: %v", err)
	}
	repo := NewGormVideoRepository(db)

	for _, userID := range []uint64{2, 3} {
		allowed, err := repo.CanUploadVideo(ctx, userID)
		if err != nil {
			t.Fatalf("CanUploadVideo(%d) returned error: %v", userID, err)
		}
		if !allowed {
			t.Fatalf("CanUploadVideo(%d) = false, want true", userID)
		}
	}

	for _, userID := range []uint64{1, 404} {
		allowed, err := repo.CanUploadVideo(ctx, userID)
		if err != nil {
			t.Fatalf("CanUploadVideo(%d) returned error: %v", userID, err)
		}
		if allowed {
			t.Fatalf("CanUploadVideo(%d) = true, want false", userID)
		}
	}
}

func TestSaveRecommendationExposuresPersistsRankAndStrategy(t *testing.T) {
	ctx := context.Background()
	db := newVideoRepoTestDB(t)
	repo := NewGormVideoRepository(db)
	now := time.Date(2026, 6, 22, 10, 0, 0, 0, time.UTC)

	err := repo.SaveRecommendationExposures(ctx, []videoapp.RecommendationExposure{
		{
			RequestID:      "req-1",
			UserID:         7,
			QuestionID:     99,
			VideoID:        11,
			VideoSegmentID: 22,
			Rank:           1,
			Score:          0.8,
			Strategy:       videoapp.RecommendStrategyQuestionVector,
			Now:            now,
		},
	})
	if err != nil {
		t.Fatalf("SaveRecommendationExposures returned error: %v", err)
	}

	var row model.EduRecommendExposure
	if err := db.First(&row).Error; err != nil {
		t.Fatalf("query exposure: %v", err)
	}
	if row.RequestID != "req-1" || row.UserID != 7 || row.QuestionID != 99 || row.VideoID != 11 || row.VideoSegmentID != 22 {
		t.Fatalf("unexpected exposure row: %+v", row)
	}
	if row.Rank != 1 || row.Score != 0.8 || row.Strategy != videoapp.RecommendStrategyQuestionVector || row.Clicked || row.Watched {
		t.Fatalf("unexpected exposure metadata: %+v", row)
	}
}

func TestMarkRecommendationExposureWatchedUpdatesLatestMatchingExposure(t *testing.T) {
	ctx := context.Background()
	db := newVideoRepoTestDB(t)
	repo := NewGormVideoRepository(db)
	first := time.Date(2026, 6, 22, 10, 0, 0, 0, time.UTC)
	second := first.Add(time.Minute)

	if err := db.Create(&[]model.EduRecommendExposure{
		{RequestID: "req-old", UserID: 7, QuestionID: 99, VideoID: 11, VideoSegmentID: 22, Rank: 1, Score: 0.8, Strategy: videoapp.RecommendStrategyQuestionVector, CreateTime: first, UpdateTime: first, Deleted: 0},
		{RequestID: "req-new", UserID: 7, QuestionID: 99, VideoID: 11, VideoSegmentID: 22, Rank: 1, Score: 0.9, Strategy: videoapp.RecommendStrategyProfileRerank, CreateTime: second, UpdateTime: second, Deleted: 0},
	}).Error; err != nil {
		t.Fatalf("seed exposures: %v", err)
	}

	markTime := second.Add(time.Minute)
	if err := repo.MarkRecommendationExposureWatched(ctx, 7, 99, 22, markTime); err != nil {
		t.Fatalf("MarkRecommendationExposureWatched returned error: %v", err)
	}

	var oldRow model.EduRecommendExposure
	if err := db.Where("request_id = ?", "req-old").First(&oldRow).Error; err != nil {
		t.Fatalf("query old exposure: %v", err)
	}
	if oldRow.Clicked || oldRow.Watched {
		t.Fatalf("old exposure should remain unmarked: %+v", oldRow)
	}
	var newRow model.EduRecommendExposure
	if err := db.Where("request_id = ?", "req-new").First(&newRow).Error; err != nil {
		t.Fatalf("query new exposure: %v", err)
	}
	if !newRow.Clicked || !newRow.Watched || !newRow.ClickedTime.Equal(markTime) || !newRow.WatchedTime.Equal(markTime) {
		t.Fatalf("new exposure should be marked watched: %+v", newRow)
	}
}

func TestRecommendationDatasourceStatsCountsRecommendationInputs(t *testing.T) {
	ctx := context.Background()
	db := newVideoRepoTestDB(t)
	repo := NewGormVideoRepository(db)
	now := time.Date(2026, 7, 9, 10, 0, 0, 0, time.UTC)

	if err := db.Create(&[]model.EduVideoResource{
		{ID: 1, Title: "published recommended", VideoURL: "/v/1.mp4", Status: int16(domainvideo.StatusDone), IsPublish: true, IsRec: true, Deleted: 0},
		{ID: 2, Title: "published only", VideoURL: "/v/2.mp4", Status: int16(domainvideo.StatusDone), IsPublish: true, Deleted: 0},
		{ID: 3, Title: "deleted", VideoURL: "/v/3.mp4", Status: int16(domainvideo.StatusDone), IsPublish: true, IsRec: true, Deleted: 1},
	}).Error; err != nil {
		t.Fatalf("seed videos: %v", err)
	}
	if err := db.Create(&[]model.EduVideoSegment{
		{ID: 11, VideoID: 1, SegmentIndex: 1, StartTimeSec: 0, EndTimeSec: 10, Status: 1, Deleted: 0},
		{ID: 12, VideoID: 1, SegmentIndex: 2, StartTimeSec: 10, EndTimeSec: 20, Status: 1, Deleted: 0},
		{ID: 13, VideoID: 2, SegmentIndex: 1, StartTimeSec: 0, EndTimeSec: 10, Status: 2, Deleted: 0},
		{ID: 14, VideoID: 3, SegmentIndex: 1, StartTimeSec: 0, EndTimeSec: 10, Status: 1, Deleted: 1},
	}).Error; err != nil {
		t.Fatalf("seed segments: %v", err)
	}
	if err := db.Model(&model.EduVideoSegment{}).Where("id = ?", 11).Update("embedding", "[1,2,3]").Error; err != nil {
		t.Fatalf("seed segment embedding: %v", err)
	}
	if err := db.Model(&model.EduVideoSegment{}).Where("id IN ?", []uint64{12, 13}).Update("embedding", nil).Error; err != nil {
		t.Fatalf("clear segment embeddings: %v", err)
	}
	if err := db.Create(&[]model.EduRecommendExposure{
		{RequestID: "req-1", UserID: 7, VideoID: 1, VideoSegmentID: 11, Rank: 1, Score: 1, Strategy: videoapp.RecommendStrategyGorse, Watched: true, CreateTime: now, Deleted: 0},
		{RequestID: "req-2", UserID: 8, VideoID: 1, VideoSegmentID: 12, Rank: 2, Score: 0.5, Strategy: videoapp.RecommendStrategyRecBole, CreateTime: now, Deleted: 0},
	}).Error; err != nil {
		t.Fatalf("seed exposures: %v", err)
	}
	if err := db.Create(&[]model.EduUserVideoRecommend{
		{UserID: 7, VideoID: 1, VideoSegmentID: 11, IsWatched: true, Deleted: 0},
		{UserID: 8, VideoID: 1, VideoSegmentID: 12, Deleted: 0},
	}).Error; err != nil {
		t.Fatalf("seed recommendations: %v", err)
	}
	if err := db.Create(&model.EduUserReaction{UserID: 7, VideoID: 1, VideoSegmentID: 11, ReactionType: string(videoapp.VideoReactionLike), Deleted: 0}).Error; err != nil {
		t.Fatalf("seed reaction: %v", err)
	}
	if err := db.Exec(`ATTACH DATABASE ':memory:' AS recsys`).Error; err != nil {
		t.Fatalf("attach recsys: %v", err)
	}
	if err := db.Exec(`CREATE TABLE recsys.recommend_user_embedding (user_id INTEGER, status INTEGER, deleted INTEGER)`).Error; err != nil {
		t.Fatalf("create recsys user embedding: %v", err)
	}
	if err := db.Exec(`CREATE TABLE recsys.recommend_item_embedding (video_segment_id INTEGER, status INTEGER, deleted INTEGER)`).Error; err != nil {
		t.Fatalf("create recsys item embedding: %v", err)
	}
	if err := db.Exec(`INSERT INTO recsys.recommend_user_embedding (user_id, status, deleted) VALUES (7, 1, 0), (7, 1, 0), (8, 0, 0), (9, 1, 1)`).Error; err != nil {
		t.Fatalf("seed recsys user embedding: %v", err)
	}
	if err := db.Exec(`INSERT INTO recsys.recommend_item_embedding (video_segment_id, status, deleted) VALUES (11, 1, 0), (11, 1, 0), (12, 0, 0), (13, 1, 1)`).Error; err != nil {
		t.Fatalf("seed recsys item embedding: %v", err)
	}

	stats, err := repo.GetRecommendationDatasourceStats(ctx)
	if err != nil {
		t.Fatalf("GetRecommendationDatasourceStats returned error: %v", err)
	}

	if stats.VideoTotal != 2 || stats.PublishedVideos != 2 || stats.RecommendVideos != 1 {
		t.Fatalf("video stats = %+v", stats)
	}
	if stats.SegmentTotal != 3 || stats.PlayableSegments != 2 || stats.EmbeddedSegments != 1 {
		t.Fatalf("segment stats = %+v", stats)
	}
	if stats.ExposureTotal != 2 || stats.WatchedExposures != 1 || stats.RecommendationRows != 2 || stats.WatchedRecommendations != 1 {
		t.Fatalf("recommendation stats = %+v", stats)
	}
	if stats.RecBoleUsers != 1 || stats.RecBoleItems != 1 || stats.ReactionRows != 1 {
		t.Fatalf("recbole/reaction stats = %+v", stats)
	}
}

func TestRecommendationDiagnosticsReadsRecentRequestsAndFreshness(t *testing.T) {
	ctx := context.Background()
	db := newVideoRepoTestDB(t)
	repo := NewGormVideoRepository(db)
	now := time.Date(2026, 7, 10, 9, 0, 0, 0, time.UTC)
	if err := db.Create(&[]model.EduRecommendExposure{
		{RequestID: "req-new", UserID: 7, QuestionID: 3, VideoID: 11, VideoSegmentID: 101, Rank: 1, Score: 0.9, Strategy: videoapp.RecommendStrategyRecBole, ModelVersion: "recbole_v2", Watched: true, CreateTime: now},
		{RequestID: "req-new", UserID: 7, QuestionID: 3, VideoID: 12, VideoSegmentID: 102, Rank: 2, Score: 0.8, Strategy: videoapp.RecommendStrategyRecBole, ModelVersion: "recbole_v2", Watched: false, CreateTime: now.Add(-time.Minute)},
		{RequestID: "req-old", UserID: 8, QuestionID: 4, VideoID: 13, VideoSegmentID: 103, Rank: 1, Score: 0.7, Strategy: videoapp.RecommendStrategyGorse, ModelVersion: "gorse", Watched: false, CreateTime: now.Add(-time.Hour)},
	}).Error; err != nil {
		t.Fatalf("seed exposures: %v", err)
	}
	if err := db.Exec(`ATTACH DATABASE ':memory:' AS recsys`).Error; err != nil {
		t.Fatalf("attach recsys: %v", err)
	}
	if err := db.Exec(`CREATE TABLE recsys.recommend_user_embedding (user_id INTEGER, status INTEGER, deleted INTEGER, update_time TIMESTAMP)`).Error; err != nil {
		t.Fatalf("create recsys user embedding: %v", err)
	}
	if err := db.Exec(`CREATE TABLE recsys.recommend_item_embedding (video_segment_id INTEGER, status INTEGER, deleted INTEGER, update_time TIMESTAMP)`).Error; err != nil {
		t.Fatalf("create recsys item embedding: %v", err)
	}
	if err := db.Exec(`INSERT INTO recsys.recommend_user_embedding (user_id, status, deleted, update_time) VALUES (?, 1, 0, ?)`, 7, now.Add(-2*time.Hour)).Error; err != nil {
		t.Fatalf("seed recsys user embedding: %v", err)
	}
	if err := db.Exec(`INSERT INTO recsys.recommend_item_embedding (video_segment_id, status, deleted, update_time) VALUES (?, 1, 0, ?)`, 101, now.Add(-time.Hour)).Error; err != nil {
		t.Fatalf("seed recsys item embedding: %v", err)
	}

	requests, err := repo.ListRecommendationRecentRequests(ctx, 5)
	if err != nil {
		t.Fatalf("ListRecommendationRecentRequests returned error: %v", err)
	}
	if len(requests) < 2 || requests[0].RequestID != "req-new" {
		t.Fatalf("requests = %+v, want req-new first", requests)
	}
	if requests[0].Exposures != 2 || requests[0].Watched != 1 || requests[0].WatchRate != 0.5 {
		t.Fatalf("req-new aggregate = %+v", requests[0])
	}

	freshness, err := repo.ListRecommendationDataFreshness(ctx)
	if err != nil {
		t.Fatalf("ListRecommendationDataFreshness returned error: %v", err)
	}
	if !hasFreshnessSource(freshness, "recbole_user_embedding") || !hasFreshnessSource(freshness, "recbole_item_embedding") {
		t.Fatalf("freshness = %+v, want recbole user and item sources", freshness)
	}
}

func TestRecommendationEffectMetricsAggregatesByDayAndStrategy(t *testing.T) {
	ctx := context.Background()
	db := newVideoRepoTestDB(t)
	repo := NewGormVideoRepository(db)
	now := time.Date(2026, 7, 9, 10, 0, 0, 0, time.UTC)
	yesterday := now.AddDate(0, 0, -1)

	if err := db.Create(&[]model.EduRecommendExposure{
		{RequestID: "req-gorse-1", UserID: 7, VideoID: 1, VideoSegmentID: 11, Rank: 1, Score: 1, Strategy: videoapp.RecommendStrategyGorse, ModelVersion: "gorse", Watched: true, CreateTime: now, Deleted: 0},
		{RequestID: "req-gorse-2", UserID: 8, VideoID: 1, VideoSegmentID: 12, Rank: 2, Score: 0.5, Strategy: videoapp.RecommendStrategyGorse, ModelVersion: "gorse", CreateTime: now, Deleted: 0},
		{RequestID: "req-recbole-1", UserID: 9, VideoID: 2, VideoSegmentID: 21, Rank: 1, Score: 0.8, Strategy: videoapp.RecommendStrategyRecBole, ModelVersion: "recbole_v2", Watched: true, CreateTime: yesterday, Deleted: 0},
	}).Error; err != nil {
		t.Fatalf("seed exposures: %v", err)
	}

	metrics, err := repo.ListRecommendationEffectMetrics(ctx, 3)
	if err != nil {
		t.Fatalf("ListRecommendationEffectMetrics returned error: %v", err)
	}

	if len(metrics.Daily) != 2 {
		t.Fatalf("daily rows = %+v, want 2", metrics.Daily)
	}
	if metrics.Daily[0].Day != "2026-07-08" || metrics.Daily[0].Exposures != 1 || metrics.Daily[0].Watched != 1 || metrics.Daily[0].WatchRate != 1 {
		t.Fatalf("first daily = %+v", metrics.Daily[0])
	}
	if metrics.Daily[1].Day != "2026-07-09" || metrics.Daily[1].Exposures != 2 || metrics.Daily[1].Watched != 1 || metrics.Daily[1].WatchRate != 0.5 {
		t.Fatalf("second daily = %+v", metrics.Daily[1])
	}
	if len(metrics.Strategies) != 2 {
		t.Fatalf("strategy rows = %+v, want 2", metrics.Strategies)
	}
	if metrics.Strategies[0].Strategy != videoapp.RecommendStrategyGorse || metrics.Strategies[0].ModelVersion != "gorse" || metrics.Strategies[0].Exposures != 2 || metrics.Strategies[0].Watched != 1 || metrics.Strategies[0].WatchRate != 0.5 {
		t.Fatalf("gorse row = %+v", metrics.Strategies[0])
	}
	if metrics.Strategies[1].Strategy != videoapp.RecommendStrategyRecBole || metrics.Strategies[1].ModelVersion != "recbole_v2" || metrics.Strategies[1].Exposures != 1 || metrics.Strategies[1].Watched != 1 || metrics.Strategies[1].WatchRate != 1 {
		t.Fatalf("recbole row = %+v", metrics.Strategies[1])
	}
}

func hasFreshnessSource(rows []videoapp.RecommendationDataFreshness, source string) bool {
	for _, row := range rows {
		if row.Source == source && row.HasData {
			return true
		}
	}
	return false
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

func TestFindRandomPlayableSegmentExcludingSkipsRecentSegments(t *testing.T) {
	ctx := context.Background()
	db := newVideoRepoTestDB(t)
	repo := NewGormVideoRepository(db)

	videos := []model.EduVideoResource{
		{ID: 31, Title: "first video", VideoURL: "/raw/31.mp4", Status: int16(domainvideo.StatusDone), IsPublish: true, Deleted: 0},
		{ID: 32, Title: "second video", VideoURL: "/raw/32.mp4", Status: int16(domainvideo.StatusDone), IsPublish: true, Deleted: 0},
	}
	if err := db.Create(&videos).Error; err != nil {
		t.Fatalf("seed videos: %v", err)
	}
	segments := []model.EduVideoSegment{
		{ID: 301, VideoID: 31, SegmentIndex: 1, StartTimeSec: 0, EndTimeSec: 30, ContentSummary: "recent segment", Status: 1, Deleted: 0},
		{ID: 302, VideoID: 32, SegmentIndex: 1, StartTimeSec: 0, EndTimeSec: 30, ContentSummary: "fresh segment", Status: 1, Deleted: 0},
	}
	if err := db.Create(&segments).Error; err != nil {
		t.Fatalf("seed segments: %v", err)
	}

	result, found, err := repo.FindRandomPlayableSegmentExcluding(ctx, []uint64{301})
	if err != nil {
		t.Fatalf("FindRandomPlayableSegmentExcluding returned error: %v", err)
	}
	if !found {
		t.Fatal("expected a playable segment")
	}
	if result.VideoSegmentID != 302 {
		t.Fatalf("VideoSegmentID = %d, want 302", result.VideoSegmentID)
	}
}

func TestFindRandomPlayableSegmentDoesNotUseDatabaseRandomSort(t *testing.T) {
	ctx := context.Background()
	db := newVideoRepoTestDB(t)
	capture := &sqlCaptureLogger{}
	repo := NewGormVideoRepository(db.Session(&gorm.Session{Logger: capture}))

	if err := db.Create(&model.EduVideoResource{
		ID:        41,
		Title:     "playable video",
		VideoURL:  "/raw/41.mp4",
		Status:    int16(domainvideo.StatusDone),
		IsPublish: true,
		Deleted:   0,
	}).Error; err != nil {
		t.Fatalf("seed video: %v", err)
	}
	if err := db.Create(&model.EduVideoSegment{
		ID:             401,
		VideoID:        41,
		SegmentIndex:   1,
		StartTimeSec:   0,
		EndTimeSec:     30,
		ContentSummary: "playable segment",
		Status:         1,
		Deleted:        0,
	}).Error; err != nil {
		t.Fatalf("seed segment: %v", err)
	}

	_, found, err := repo.FindRandomPlayableSegment(ctx)
	if err != nil {
		t.Fatalf("FindRandomPlayableSegment returned error: %v", err)
	}
	if !found {
		t.Fatal("expected a playable segment")
	}
	if sql := strings.ToUpper(strings.Join(capture.sql, "\n")); strings.Contains(sql, "RANDOM()") {
		t.Fatalf("random playable fallback should not use database random sort, got SQL:\n%s", sql)
	}
}

func TestHydrateRecommendedSegmentsByIDFiltersAndPreservesGorseOrder(t *testing.T) {
	ctx := context.Background()
	db := newVideoRepoTestDB(t)
	repo := NewGormVideoRepository(db)

	videos := []model.EduVideoResource{
		{ID: 11, Title: "video 11", VideoURL: "/raw/11.mp4", Status: int16(domainvideo.StatusDone), IsPublish: true, IsRec: true, Deleted: 0},
		{ID: 12, Title: "video 12", VideoURL: "/raw/12.mp4", Status: int16(domainvideo.StatusDone), IsPublish: true, IsRec: true, Deleted: 0},
		{ID: 13, Title: "video 13", VideoURL: "/raw/13.mp4", Status: int16(domainvideo.StatusDone), IsPublish: true, IsRec: true, Deleted: 0},
		{ID: 14, Title: "video 14", VideoURL: "/raw/14.mp4", Status: int16(domainvideo.StatusDone), IsPublish: true, IsRec: true, Deleted: 0},
		{ID: 15, Title: "video 15", VideoURL: "/raw/15.mp4", Status: int16(domainvideo.StatusProcessing), IsPublish: true, IsRec: true, Deleted: 0},
		{ID: 16, Title: "video 16", VideoURL: "/raw/16.mp4", Status: int16(domainvideo.StatusDone), IsPublish: false, IsRec: true, Deleted: 0},
	}
	if err := db.Create(&videos).Error; err != nil {
		t.Fatalf("seed videos: %v", err)
	}
	if err := db.Model(&model.EduVideoResource{}).Where("id = ?", 16).Update("is_published", false).Error; err != nil {
		t.Fatalf("mark unpublished: %v", err)
	}
	segments := []model.EduVideoSegment{
		{ID: 101, VideoID: 11, SegmentIndex: 1, StartTimeSec: 10, EndTimeSec: 40, ContentSummary: "segment 101", Status: 1, Deleted: 0},
		{ID: 102, VideoID: 12, SegmentIndex: 1, StartTimeSec: 10, EndTimeSec: 40, ContentSummary: "segment 102", Status: 1, Deleted: 0},
		{ID: 103, VideoID: 13, SegmentIndex: 1, StartTimeSec: 10, EndTimeSec: 40, ContentSummary: "segment 103", Status: 1, Deleted: 0},
		{ID: 104, VideoID: 14, SegmentIndex: 1, StartTimeSec: 10, EndTimeSec: 40, ContentSummary: "segment 104", Status: 1, Deleted: 0},
		{ID: 105, VideoID: 15, SegmentIndex: 1, StartTimeSec: 10, EndTimeSec: 40, ContentSummary: "segment 105", Status: 1, Deleted: 0},
		{ID: 106, VideoID: 16, SegmentIndex: 1, StartTimeSec: 10, EndTimeSec: 40, ContentSummary: "segment 106", Status: 1, Deleted: 0},
	}
	if err := db.Create(&segments).Error; err != nil {
		t.Fatalf("seed segments: %v", err)
	}
	if err := db.Create(&model.EduUserReaction{UserID: 7, VideoID: 12, VideoSegmentID: 102, ReactionType: "dislike", Deleted: 0}).Error; err != nil {
		t.Fatalf("seed dislike: %v", err)
	}
	if err := db.Create(&model.EduUserVideoRecommend{UserID: 7, VideoID: 13, QuestionID: 0, VideoSegmentID: 103, IsWatched: true, Deleted: 0}).Error; err != nil {
		t.Fatalf("seed watched: %v", err)
	}

	got, err := repo.HydrateRecommendedSegmentsByID(ctx, 7, []uint64{104, 102, 101, 106, 103, 105, 101})
	if err != nil {
		t.Fatalf("HydrateRecommendedSegmentsByID returned error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d candidates: %+v", len(got), got)
	}
	if got[0].VideoSegmentID != 104 || got[1].VideoSegmentID != 101 {
		t.Fatalf("got order = [%d %d], want [104 101]", got[0].VideoSegmentID, got[1].VideoSegmentID)
	}
	if got[0].SegmentTitle != "segment 104" || got[1].VideoID != 11 {
		t.Fatalf("unexpected hydrated data: %+v", got)
	}
}

func TestFindRecommendedSegmentsByWeakKnowledgeMatchesLowestMasteryDescriptions(t *testing.T) {
	ctx := context.Background()
	db := newVideoRepoTestDB(t)
	repo := NewGormVideoRepository(db)

	if err := db.Exec(`CREATE TABLE edu_user_knowledge_mastery (
		user_id INTEGER NOT NULL,
		knowledge_point_id INTEGER NOT NULL,
		mastery REAL NOT NULL,
		deleted INTEGER DEFAULT 0
	)`).Error; err != nil {
		t.Fatalf("create mastery table: %v", err)
	}
	if err := db.Exec(`CREATE TABLE dict_knowledge_point (
		id INTEGER PRIMARY KEY,
		name TEXT,
		discription TEXT,
		description TEXT,
		deleted INTEGER DEFAULT 0
	)`).Error; err != nil {
		t.Fatalf("create knowledge dict: %v", err)
	}

	videos := []model.EduVideoResource{
		{ID: 11, Title: "一次函数专题", VideoURL: "/raw/11.mp4", Status: int16(domainvideo.StatusDone), IsPublish: true, IsRec: true, Deleted: 0},
		{ID: 12, Title: "几何专题", VideoURL: "/raw/12.mp4", Status: int16(domainvideo.StatusDone), IsPublish: true, IsRec: true, Deleted: 0},
		{ID: 13, Title: "已看专题", VideoURL: "/raw/13.mp4", Status: int16(domainvideo.StatusDone), IsPublish: true, IsRec: true, Deleted: 0},
		{ID: 14, Title: "第十一弱专题", VideoURL: "/raw/14.mp4", Status: int16(domainvideo.StatusDone), IsPublish: true, IsRec: true, Deleted: 0},
		{ID: 15, Title: "无摘要斜率专题", Description: "一次函数 斜率", VideoURL: "/raw/15.mp4", Status: int16(domainvideo.StatusDone), IsPublish: true, IsRec: true, Deleted: 0},
	}
	if err := db.Create(&videos).Error; err != nil {
		t.Fatalf("seed videos: %v", err)
	}
	segments := []model.EduVideoSegment{
		{ID: 101, VideoID: 11, SegmentIndex: 1, StartTimeSec: 0, EndTimeSec: 30, ContentSummary: "一次函数图像", KnowledgeTags: model.TextArray{"一次函数"}, Status: 1, Deleted: 0},
		{ID: 102, VideoID: 12, SegmentIndex: 1, StartTimeSec: 0, EndTimeSec: 30, ContentSummary: "三角形面积", KnowledgeTags: model.TextArray{"几何"}, Status: 1, Deleted: 0},
		{ID: 103, VideoID: 13, SegmentIndex: 1, StartTimeSec: 0, EndTimeSec: 30, ContentSummary: "一次函数练习", KnowledgeTags: model.TextArray{"一次函数"}, Status: 1, Deleted: 0},
		{ID: 104, VideoID: 14, SegmentIndex: 1, StartTimeSec: 0, EndTimeSec: 30, ContentSummary: "第十一弱知识点", KnowledgeTags: model.TextArray{"第十一弱"}, Status: 1, Deleted: 0},
		{ID: 105, VideoID: 15, SegmentIndex: 1, StartTimeSec: 0, EndTimeSec: 30, ContentSummary: "综合复习", Status: 1, Deleted: 0},
	}
	if err := db.Create(&segments).Error; err != nil {
		t.Fatalf("seed segments: %v", err)
	}
	if err := db.Create(&model.EduUserVideoRecommend{UserID: 7, VideoID: 13, QuestionID: 0, VideoSegmentID: 103, IsWatched: true, Deleted: 0}).Error; err != nil {
		t.Fatalf("seed watched: %v", err)
	}
	if err := db.Exec(`INSERT INTO edu_user_knowledge_mastery (user_id, knowledge_point_id, mastery, deleted) VALUES
		(7, 1, 0.10, 0),
		(7, 2, 0.20, 0),
		(7, 3, 0.30, 0),
		(7, 4, 0.40, 0),
		(7, 5, 0.50, 0),
		(7, 6, 0.60, 0),
		(7, 7, 0.70, 0),
		(7, 8, 0.80, 0),
		(7, 9, 0.90, 0),
		(7, 10, 1.00, 0),
		(7, 11, 1.10, 0),
		(8, 2, 0.05, 0)`).Error; err != nil {
		t.Fatalf("seed mastery: %v", err)
	}
	if err := db.Exec(`INSERT INTO dict_knowledge_point (id, name, discription, description, deleted) VALUES
		(1, '一次函数', '一次函数 图像 斜率', '', 0),
		(2, '三角形面积', '三角形 面积', '', 0),
		(3, '无匹配三', '无匹配三', '', 0),
		(4, '无匹配四', '无匹配四', '', 0),
		(5, '无匹配五', '无匹配五', '', 0),
		(6, '无匹配六', '无匹配六', '', 0),
		(7, '无匹配七', '无匹配七', '', 0),
		(8, '无匹配八', '无匹配八', '', 0),
		(9, '无匹配九', '无匹配九', '', 0),
		(10, '无匹配十', '无匹配十', '', 0),
		(11, '第十一弱', '第十一弱 知识点', '', 0)`).Error; err != nil {
		t.Fatalf("seed knowledge dict: %v", err)
	}

	got, err := repo.FindRecommendedSegmentsByWeakKnowledge(ctx, 7, 5, 10)
	if err != nil {
		t.Fatalf("FindRecommendedSegmentsByWeakKnowledge returned error: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("got %d candidates: %+v", len(got), got)
	}
	if got[0].VideoSegmentID != 101 || got[0].VideoID != 11 || got[0].SegmentTitle != "一次函数图像" {
		t.Fatalf("candidate = %+v, want segment 101", got[0])
	}
	if got[1].VideoSegmentID != 105 || got[1].VideoID != 15 || got[1].SegmentTitle != "综合复习" {
		t.Fatalf("candidate = %+v, want segment 105 from resource title/description match", got[1])
	}
	if got[2].VideoSegmentID != 102 || got[2].VideoID != 12 || got[2].SegmentTitle != "三角形面积" {
		t.Fatalf("candidate = %+v, want segment 102", got[2])
	}
}

func TestFindRecommendedSegmentsByWeakKnowledgeSupportsLegacyKnowledgePointColumnCase(t *testing.T) {
	ctx := context.Background()
	db := newVideoRepoTestDB(t)
	repo := NewGormVideoRepository(db)

	if err := db.Exec(`CREATE TABLE edu_user_knowledge_mastery (
		user_id INTEGER NOT NULL,
		Knowledge_point_id INTEGER NOT NULL,
		mastery REAL NOT NULL
	)`).Error; err != nil {
		t.Fatalf("create mastery table: %v", err)
	}
	if err := db.Exec(`CREATE TABLE dict_knowledge_point (
		id INTEGER PRIMARY KEY,
		name TEXT,
		discription TEXT
	)`).Error; err != nil {
		t.Fatalf("create knowledge dict: %v", err)
	}
	if err := db.Create(&model.EduVideoResource{ID: 21, Title: "一次函数专题", VideoURL: "/raw/21.mp4", Status: int16(domainvideo.StatusDone), IsPublish: true, IsRec: true, Deleted: 0}).Error; err != nil {
		t.Fatalf("seed video: %v", err)
	}
	if err := db.Create(&model.EduVideoSegment{ID: 201, VideoID: 21, SegmentIndex: 1, StartTimeSec: 0, EndTimeSec: 30, ContentSummary: "一次函数图像", Status: 1, Deleted: 0}).Error; err != nil {
		t.Fatalf("seed segment: %v", err)
	}
	if err := db.Exec(`INSERT INTO edu_user_knowledge_mastery (user_id, Knowledge_point_id, mastery) VALUES (7, 1, 0.10)`).Error; err != nil {
		t.Fatalf("seed mastery: %v", err)
	}
	if err := db.Exec(`INSERT INTO dict_knowledge_point (id, name, discription) VALUES (1, '一次函数', '一次函数 图像')`).Error; err != nil {
		t.Fatalf("seed knowledge dict: %v", err)
	}

	got, err := repo.FindRecommendedSegmentsByWeakKnowledge(ctx, 7, 5, 10)
	if err != nil {
		t.Fatalf("FindRecommendedSegmentsByWeakKnowledge returned error: %v", err)
	}
	if len(got) != 1 || got[0].VideoSegmentID != 201 {
		t.Fatalf("got candidates = %+v, want segment 201", got)
	}
}

func TestListWeakKnowledgeReturnsLowestMasteryDescriptions(t *testing.T) {
	ctx := context.Background()
	db := newVideoRepoTestDB(t)
	repo := NewGormVideoRepository(db)

	if err := db.Exec(`CREATE TABLE edu_user_knowledge_mastery (
		user_id INTEGER NOT NULL,
		knowledge_point_id INTEGER NOT NULL,
		mastery REAL NOT NULL
	)`).Error; err != nil {
		t.Fatalf("create mastery table: %v", err)
	}
	if err := db.Exec(`CREATE TABLE dict_knowledge_point (
		id INTEGER PRIMARY KEY,
		name TEXT,
		discription TEXT
	)`).Error; err != nil {
		t.Fatalf("create knowledge dict: %v", err)
	}
	if err := db.Exec(`INSERT INTO edu_user_knowledge_mastery (user_id, knowledge_point_id, mastery) VALUES
		(7, 1, 0.30),
		(7, 2, 0.10),
		(7, 3, 0.20),
		(8, 4, 0.01)`).Error; err != nil {
		t.Fatalf("seed mastery: %v", err)
	}
	if err := db.Exec(`INSERT INTO dict_knowledge_point (id, name, discription) VALUES
		(1, '一次函数', '图像与斜率'),
		(2, '三角形面积', '底高关系'),
		(3, '方程', '一元一次方程'),
		(4, '其他用户', '不应出现')`).Error; err != nil {
		t.Fatalf("seed knowledge dict: %v", err)
	}

	got, err := repo.ListWeakKnowledge(ctx, 7, 2)
	if err != nil {
		t.Fatalf("ListWeakKnowledge returned error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d rows: %+v", len(got), got)
	}
	if got[0].KnowledgePointID != 2 || got[0].Name != "三角形面积" || got[0].Description != "底高关系" {
		t.Fatalf("first weak knowledge = %+v, want id=2", got[0])
	}
	if got[1].KnowledgePointID != 3 || got[1].Name != "方程" || got[1].Description != "一元一次方程" {
		t.Fatalf("second weak knowledge = %+v, want id=3", got[1])
	}
}

func TestGetArchiveProcessingProgressCountsTranscodeAndVectorizedVideos(t *testing.T) {
	ctx := context.Background()
	db := newVideoRepoTestDB(t)
	repo := NewGormVideoRepository(db)

	videos := []model.EduVideoResource{
		{ID: 11, Title: "done and vectorized", VideoURL: "/raw/11.mp4", Status: int16(domainvideo.StatusDone), IsPublish: true, Deleted: 0},
		{ID: 12, Title: "done only", VideoURL: "/raw/12.mp4", Status: int16(domainvideo.StatusDone), IsPublish: true, Deleted: 0},
		{ID: 13, Title: "processing", VideoURL: "/raw/13.mp4", Status: int16(domainvideo.StatusProcessing), IsPublish: true, Deleted: 0},
		{ID: 14, Title: "deleted", VideoURL: "/raw/14.mp4", Status: int16(domainvideo.StatusDone), IsPublish: true, Deleted: 1},
	}
	if err := db.Create(&videos).Error; err != nil {
		t.Fatalf("seed videos: %v", err)
	}
	segments := []model.EduVideoSegment{
		{ID: 101, VideoID: 11, SegmentIndex: 1, StartTimeSec: 0, EndTimeSec: 30, Status: 1, Deleted: 0},
		{ID: 102, VideoID: 12, SegmentIndex: 1, StartTimeSec: 0, EndTimeSec: 30, Status: 1, Deleted: 0},
		{ID: 103, VideoID: 13, SegmentIndex: 1, StartTimeSec: 0, EndTimeSec: 30, Status: 1, Deleted: 0},
		{ID: 104, VideoID: 14, SegmentIndex: 1, StartTimeSec: 0, EndTimeSec: 30, Status: 1, Deleted: 0},
	}
	if err := db.Create(&segments).Error; err != nil {
		t.Fatalf("seed segments: %v", err)
	}
	if err := db.Create(&model.EduVideoVectorStage{
		TaskID:       "11",
		VideoID:      11,
		Stage:        "vector.finalize",
		SegmentIndex: 0,
		SegmentID:    0,
		Status:       2,
	}).Error; err != nil {
		t.Fatalf("seed vector stage: %v", err)
	}

	progress, err := repo.GetArchiveProcessingProgress(ctx, []uint64{11, 12, 13, 14})
	if err != nil {
		t.Fatalf("GetArchiveProcessingProgress returned error: %v", err)
	}
	if progress.Total != 4 {
		t.Fatalf("Total = %d, want 4", progress.Total)
	}
	if progress.Transcoded != 2 {
		t.Fatalf("Transcoded = %d, want 2", progress.Transcoded)
	}
	if progress.Vectorized != 1 {
		t.Fatalf("Vectorized = %d, want 1", progress.Vectorized)
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
