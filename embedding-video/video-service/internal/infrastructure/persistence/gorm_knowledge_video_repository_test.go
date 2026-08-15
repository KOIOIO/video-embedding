package persistence

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"video-service/internal/application/knowledgevideo"
	"video-service/internal/model"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestKnowledgeVideoEnsureSchemaCreatesTablesAndNonUniqueActiveKnowledgePointIndex(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}

	if err := EnsureSchema(db); err != nil {
		t.Fatalf("EnsureSchema() error = %v", err)
	}

	for _, table := range []string{
		"edu_knowledge_video_batch",
		"edu_knowledge_video",
		"edu_knowledge_video_play_record",
	} {
		if !db.Migrator().HasTable(table) {
			t.Fatalf("table %q does not exist", table)
		}
	}

	var sql string
	if err := db.Raw(`SELECT sql FROM sqlite_master WHERE type = 'index' AND name = ?`, "idx_knowledge_video_knowledge_point_active").Scan(&sql).Error; err != nil {
		t.Fatalf("query active knowledge point index: %v", err)
	}
	if strings.Contains(strings.ToLower(sql), "unique index") || !strings.Contains(strings.ToLower(sql), "where deleted = 0") {
		t.Fatalf("active knowledge point index SQL = %q, want non-unique partial index", sql)
	}
	var oldIndexCount int64
	if err := db.Raw(`SELECT COUNT(*) FROM sqlite_master WHERE type = 'index' AND name = ?`, "uk_knowledge_video_knowledge_point_active").Scan(&oldIndexCount).Error; err != nil {
		t.Fatal(err)
	}
	if oldIndexCount != 0 {
		t.Fatal("old unique active knowledge point index still exists")
	}
}

func TestKnowledgeVideoEnsureSchemaCreatesPartialWatchSessionIndex(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := EnsureSchema(db); err != nil {
		t.Fatalf("EnsureSchema() error = %v", err)
	}

	var sql string
	if err := db.Raw(`SELECT sql FROM sqlite_master WHERE type = 'index' AND name = ?`, "uk_knowledge_video_play_session").Scan(&sql).Error; err != nil {
		t.Fatal(err)
	}
	lower := strings.ToLower(sql)
	if !strings.Contains(lower, "unique index") || !strings.Contains(lower, "session_id is not null") || !strings.Contains(lower, "session_id <> ''") {
		t.Fatalf("watch session index SQL = %q", sql)
	}

	legacy := []model.EduKnowledgeVideoPlayRecord{
		{UserID: 7, KnowledgePointID: 9, KnowledgeVideoID: 88},
		{UserID: 7, KnowledgePointID: 9, KnowledgeVideoID: 88},
	}
	if err := db.Create(&legacy).Error; err != nil {
		t.Fatalf("legacy rows should not conflict: %v", err)
	}
}

func TestKnowledgeVideoRepositoryLookupDictionaryKnowledgePointsUsesBatchAndOptionalDeleted(t *testing.T) {
	repo, db := newKnowledgeVideoTestRepository(t)
	if err := db.Exec(`CREATE TABLE dict_knowledge_point (id integer primary key, name text, deleted integer)`).Error; err != nil {
		t.Fatalf("create dictionary table: %v", err)
	}
	if err := db.Exec(`INSERT INTO dict_knowledge_point (id, name, deleted) VALUES (1, 'one', 0), (2, 'two', 1), (3, 'three', NULL)`).Error; err != nil {
		t.Fatalf("seed dictionary: %v", err)
	}

	got, err := repo.LookupKnowledgePoints(context.Background(), []uint64{3, 1, 2, 1})
	if err != nil {
		t.Fatalf("LookupKnowledgePoints() error = %v", err)
	}
	if len(got) != 2 || got[0] != (knowledgevideo.DictionaryKnowledgePoint{ID: 1, Name: "one"}) || got[1] != (knowledgevideo.DictionaryKnowledgePoint{ID: 3, Name: "three"}) {
		t.Fatalf("LookupKnowledgePoints() = %+v", got)
	}
}

func TestKnowledgeVideoRepositoryLookupDictionaryKnowledgePointsSupportsDictionaryWithoutDeleted(t *testing.T) {
	repo, db := newKnowledgeVideoTestRepository(t)
	if err := db.Exec(`CREATE TABLE dict_knowledge_point (id integer primary key, name text)`).Error; err != nil {
		t.Fatalf("create dictionary table: %v", err)
	}
	if err := db.Exec(`INSERT INTO dict_knowledge_point (id, name) VALUES (5, 'five')`).Error; err != nil {
		t.Fatalf("seed dictionary: %v", err)
	}

	got, err := repo.LookupKnowledgePoints(context.Background(), []uint64{5})
	if err != nil || len(got) != 1 || got[0].Name != "five" {
		t.Fatalf("LookupKnowledgePoints() = %+v, %v", got, err)
	}
}

func TestKnowledgeVideoRepositoryListTreeUsesOptionalParentID(t *testing.T) {
	repo, db := newKnowledgeVideoTestRepository(t)
	if err := db.Exec(`CREATE TABLE dict_knowledge_point (id integer primary key, parent_id integer, name text, deleted integer)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO dict_knowledge_point VALUES (1, 0, '函数', 0), (10, 1, '二次函数', 0), (9, 1, '一次函数', 0), (20, 99, '孤立知识点', 0), (30, 0, '已删除', 1)`).Error; err != nil {
		t.Fatal(err)
	}
	seedKnowledgeVideo(t, db, 41, 9, knowledgevideo.VideoReady)
	seedKnowledgeVideo(t, db, 42, 10, knowledgevideo.VideoTranscoding)
	seedKnowledgeVideo(t, db, 43, 9, knowledgevideo.VideoPending)

	got, err := repo.ListKnowledgeTree(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].ID != 1 || len(got[0].Children) != 2 || got[0].Children[0].ID != 9 || got[1].ID != 20 {
		t.Fatalf("tree = %+v", got)
	}
	if got[0].Children[0].Video == nil || got[0].Children[0].Video.ID != 41 || got[0].Children[0].Video.Status != knowledgevideo.VideoReady {
		t.Fatalf("ready video = %+v", got[0].Children[0].Video)
	}
	if len(got[0].Children[0].Videos) != 2 || got[0].Children[0].Videos[0].ID != 41 || got[0].Children[0].Videos[1].ID != 43 {
		t.Fatalf("videos = %+v", got[0].Children[0].Videos)
	}
}

func TestKnowledgeVideoRepositoryListTreeFallsBackWhenParentIDIsAbsent(t *testing.T) {
	repo, db := newKnowledgeVideoTestRepository(t)
	if err := db.Exec(`CREATE TABLE dict_knowledge_point (id integer primary key, name text)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO dict_knowledge_point VALUES (10, '二次函数'), (9, '一次函数')`).Error; err != nil {
		t.Fatal(err)
	}

	got, err := repo.ListKnowledgeTree(context.Background())
	if err != nil || len(got) != 1 || got[0].ID != 0 || got[0].Name != "全部知识点" || len(got[0].Children) != 2 || got[0].Children[0].ID != 9 {
		t.Fatalf("tree = %+v, err = %v", got, err)
	}
}

func TestKnowledgeVideoRepositoryReservesIDsWithInjectedAllocator(t *testing.T) {
	repo, _ := newKnowledgeVideoTestRepository(t)

	batchID, err := repo.ReserveBatchID(context.Background())
	if err != nil || batchID != 101 {
		t.Fatalf("ReserveBatchID() = %d, %v", batchID, err)
	}
	videoIDs, err := repo.ReserveVideoIDs(context.Background(), 3)
	if err != nil || len(videoIDs) != 3 || videoIDs[0] != 201 || videoIDs[2] != 203 {
		t.Fatalf("ReserveVideoIDs() = %v, %v", videoIDs, err)
	}
}

func TestKnowledgeVideoRepositoryCreateBatchCreatesAllRowsTransactionally(t *testing.T) {
	repo, db := newKnowledgeVideoTestRepository(t)
	batch := knowledgevideo.Batch{ID: 101, UploadUserID: 7, ZipFileName: "videos.zip", XLSXFileName: "mapping.xlsx", XLSXObjectKey: "manifests/101/mapping.xlsx", TotalCount: 2, Status: knowledgevideo.BatchProcessing}
	videos := []knowledgevideo.Video{
		{ID: 201, BatchID: 101, KnowledgePointID: 9, KnowledgePointName: "one", SourceFileName: "one.mp4", SourceObjectKey: "raw/201/source.mp4", HLSObjectPrefix: "hls/201", Status: knowledgevideo.VideoPending},
		{ID: 202, BatchID: 101, KnowledgePointID: 10, KnowledgePointName: "two", SourceFileName: "two.mp4", SourceObjectKey: "raw/202/source.mp4", HLSObjectPrefix: "hls/202", Status: knowledgevideo.VideoPending},
	}
	if err := repo.CreateBatch(context.Background(), batch, videos); err != nil {
		t.Fatalf("CreateBatch() error = %v", err)
	}
	var batchCount, videoCount int64
	if err := db.Model(&model.EduKnowledgeVideoBatch{}).Count(&batchCount).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&model.EduKnowledgeVideo{}).Count(&videoCount).Error; err != nil {
		t.Fatal(err)
	}
	if batchCount != 1 || videoCount != 2 {
		t.Fatalf("created batch/video rows = %d/%d, want 1/2", batchCount, videoCount)
	}
}

func TestKnowledgeVideoRepositoryCreateBatchAllowsMultipleVideosForKnowledgePoint(t *testing.T) {
	repo, db := newKnowledgeVideoTestRepository(t)
	batch := knowledgevideo.Batch{ID: 101, UploadUserID: 7, ZipFileName: "videos.zip", XLSXFileName: "mapping.xlsx", XLSXObjectKey: "manifests/101/mapping.xlsx", TotalCount: 2, Status: knowledgevideo.BatchProcessing}
	videos := []knowledgevideo.Video{
		{ID: 201, BatchID: 101, KnowledgePointID: 9, KnowledgePointName: "one", SourceFileName: "one.mp4", SourceObjectKey: "raw/201/source.mp4", HLSObjectPrefix: "hls/201", Status: knowledgevideo.VideoPending},
		{ID: 202, BatchID: 101, KnowledgePointID: 9, KnowledgePointName: "one duplicate", SourceFileName: "two.mp4", SourceObjectKey: "raw/202/source.mp4", HLSObjectPrefix: "hls/202", Status: knowledgevideo.VideoPending},
	}
	if err := repo.CreateBatch(context.Background(), batch, videos); err != nil {
		t.Fatalf("CreateBatch() error = %v", err)
	}
	var batchCount, videoCount int64
	if err := db.Model(&model.EduKnowledgeVideoBatch{}).Count(&batchCount).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&model.EduKnowledgeVideo{}).Count(&videoCount).Error; err != nil {
		t.Fatal(err)
	}
	if batchCount != 1 || videoCount != 2 {
		t.Fatalf("created batch/video rows = %d/%d, want 1/2", batchCount, videoCount)
	}
}

func TestKnowledgeVideoRepositoryListsAllReadyVideosInIDOrderAndRecordsPlayback(t *testing.T) {
	repo, db := newKnowledgeVideoTestRepository(t)
	seedKnowledgeVideo(t, db, 43, 9, knowledgevideo.VideoReady)
	seedKnowledgeVideo(t, db, 41, 9, knowledgevideo.VideoReady)
	seedKnowledgeVideo(t, db, 42, 9, knowledgevideo.VideoPending)
	seedKnowledgeVideo(t, db, 44, 9, knowledgevideo.VideoFailed)
	seedKnowledgeVideo(t, db, 45, 10, knowledgevideo.VideoReady)
	if err := db.Create(&model.EduKnowledgeVideo{ID: 46, BatchID: 1, KnowledgePointID: 9, KnowledgePointName: "deleted", SourceFileName: "46.mp4", SourceObjectKey: "raw/46.mp4", HLSObjectPrefix: "hls/46", Status: int16(knowledgevideo.VideoReady), Deleted: 1}).Error; err != nil {
		t.Fatal(err)
	}
	got, err := repo.ListReadyByKnowledgePoint(context.Background(), 9)
	if err != nil || len(got) != 2 || got[0].ID != 41 || got[1].ID != 43 {
		t.Fatalf("got=%+v err=%v", got, err)
	}
	if err := repo.RecordPlayback(context.Background(), knowledgevideo.PlayRecord{UserID: 7, KnowledgePointID: 9, KnowledgeVideoID: 41}); err != nil {
		t.Fatal(err)
	}
	var record model.EduKnowledgeVideoPlayRecord
	if err := db.First(&record).Error; err != nil {
		t.Fatal(err)
	}
	if record.UserID != 7 || record.KnowledgePointID != 9 || record.KnowledgeVideoID != 41 {
		t.Fatalf("record = %+v", record)
	}
}

func TestKnowledgeVideoRepositoryUpsertWatchSessionIsMonotonicAndCapsAggregate(t *testing.T) {
	repo, db := newKnowledgeVideoTestRepository(t)
	if err := db.Exec(`CREATE TABLE sys_user (id INTEGER PRIMARY KEY)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO sys_user (id) VALUES (7)`).Error; err != nil {
		t.Fatal(err)
	}
	seedKnowledgeVideo(t, db, 88, 9, knowledgevideo.VideoReady)
	if err := db.Model(&model.EduKnowledgeVideo{}).Where("id = ?", 88).Update("duration", 100).Error; err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 3, 10, 0, 0, 0, time.UTC)
	reports := []knowledgevideo.WatchSessionReport{
		{UserID: 7, KnowledgePointID: 9, KnowledgeVideoID: 88, SessionID: "session-00000001", WatchedSeconds: 37, UpdatedAt: now},
		{UserID: 7, KnowledgePointID: 9, KnowledgeVideoID: 88, SessionID: "session-00000001", WatchedSeconds: 20, UpdatedAt: now.Add(time.Minute)},
		{UserID: 7, KnowledgePointID: 9, KnowledgeVideoID: 88, SessionID: "session-00000002", WatchedSeconds: 90, UpdatedAt: now.Add(2 * time.Minute)},
	}
	var got knowledgevideo.WatchSessionAggregate
	for _, report := range reports {
		var err error
		got, err = repo.UpsertWatchSession(context.Background(), report, 100)
		if err != nil {
			t.Fatal(err)
		}
	}
	if got.SessionWatchedSeconds != 90 || got.TotalWatchedSeconds != 100 {
		t.Fatalf("aggregate = %+v", got)
	}
	var first model.EduKnowledgeVideoPlayRecord
	if err := db.Where("session_id = ?", "session-00000001").First(&first).Error; err != nil {
		t.Fatal(err)
	}
	if first.WatchDuration != 37 || first.UpdateTime == nil || !first.UpdateTime.Equal(now.Add(time.Minute)) {
		t.Fatalf("first session = %+v", first)
	}
}

func TestKnowledgeVideoRepositoryUserExists(t *testing.T) {
	repo, db := newKnowledgeVideoTestRepository(t)
	if err := db.Exec(`CREATE TABLE sys_user (id INTEGER PRIMARY KEY, deleted INTEGER)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO sys_user (id, deleted) VALUES (7, 0), (8, 1)`).Error; err != nil {
		t.Fatal(err)
	}
	for id, want := range map[uint64]bool{7: true, 8: false, 9: false} {
		got, err := repo.UserExists(context.Background(), id)
		if err != nil || got != want {
			t.Fatalf("UserExists(%d) = %v, %v", id, got, err)
		}
	}
}

func TestKnowledgeVideoRepositoryStateTransitionsUpdateOnlyKnowledgeVideos(t *testing.T) {
	repo, db := newKnowledgeVideoTestRepository(t)
	seedKnowledgeVideo(t, db, 41, 9, knowledgevideo.VideoPending)
	enqueued := time.Date(2026, 7, 23, 10, 0, 0, 0, time.UTC)
	if ok, err := repo.MarkTranscoding(context.Background(), 41); err != nil || !ok {
		t.Fatalf("MarkTranscoding() = %v, %v", ok, err)
	}
	if ok, err := repo.SetEnqueueTime(context.Background(), 41, enqueued); err != nil || !ok {
		t.Fatalf("SetEnqueueTime() = %v, %v", ok, err)
	}
	if ok, err := repo.MarkReady(context.Background(), 41, "hls/41/master.m3u8", 321); err != nil || !ok {
		t.Fatalf("MarkReady() = %v, %v", ok, err)
	}
	var row model.EduKnowledgeVideo
	if err := db.First(&row, 41).Error; err != nil {
		t.Fatal(err)
	}
	if row.Status != int16(knowledgevideo.VideoReady) || row.HLSMasterObjectKey != "hls/41/master.m3u8" || row.Duration != 321 || row.EnqueueTime == nil || !row.EnqueueTime.Equal(enqueued) || row.ErrorMessage != "" {
		t.Fatalf("row after ready = %+v", row)
	}
	if ok, err := repo.MarkFailed(context.Background(), 41, "ffmpeg failed"); err != nil || !ok {
		t.Fatalf("MarkFailed() = %v, %v", ok, err)
	}
	if err := db.First(&row, 41).Error; err != nil {
		t.Fatal(err)
	}
	if row.Status != int16(knowledgevideo.VideoFailed) || row.ErrorMessage != "ffmpeg failed" {
		t.Fatalf("row after failure = %+v", row)
	}
}

func TestKnowledgeVideoRepositoryRefreshBatchStatus(t *testing.T) {
	tests := []struct {
		name       string
		statuses   []knowledgevideo.VideoStatus
		wantStatus knowledgevideo.BatchStatus
		wantReady  int
		wantFailed int
	}{
		{name: "completed", statuses: []knowledgevideo.VideoStatus{knowledgevideo.VideoReady, knowledgevideo.VideoReady}, wantStatus: knowledgevideo.BatchCompleted, wantReady: 2},
		{name: "partial failed terminal", statuses: []knowledgevideo.VideoStatus{knowledgevideo.VideoReady, knowledgevideo.VideoFailed}, wantStatus: knowledgevideo.BatchPartialFailed, wantReady: 1, wantFailed: 1},
		{name: "failed", statuses: []knowledgevideo.VideoStatus{knowledgevideo.VideoFailed, knowledgevideo.VideoFailed}, wantStatus: knowledgevideo.BatchFailed, wantFailed: 2},
		{name: "still processing until terminal", statuses: []knowledgevideo.VideoStatus{knowledgevideo.VideoFailed, knowledgevideo.VideoPending}, wantStatus: knowledgevideo.BatchProcessing, wantFailed: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, db := newKnowledgeVideoTestRepository(t)
			if err := db.Create(&model.EduKnowledgeVideoBatch{ID: 1, UploadUserID: 7, ZipFileName: "v.zip", XLSXFileName: "m.xlsx", XLSXObjectKey: "m", TotalCount: len(tt.statuses), Status: int16(knowledgevideo.BatchProcessing)}).Error; err != nil {
				t.Fatal(err)
			}
			for i, status := range tt.statuses {
				seedKnowledgeVideoInBatch(t, db, uint64(i+1), uint64(i+1), 1, status)
			}
			got, ok, err := repo.RefreshBatchStatus(context.Background(), 1)
			if err != nil || !ok {
				t.Fatalf("RefreshBatchStatus() = %+v, %v, %v", got, ok, err)
			}
			if got.Status != tt.wantStatus || got.ReadyCount != tt.wantReady || got.FailedCount != tt.wantFailed {
				t.Fatalf("batch = %+v", got)
			}
		})
	}
}

func TestKnowledgeVideoRepositoryListStalePending(t *testing.T) {
	repo, db := newKnowledgeVideoTestRepository(t)
	cutoff := time.Date(2026, 7, 23, 10, 0, 0, 0, time.UTC)
	seedKnowledgeVideo(t, db, 1, 1, knowledgevideo.VideoPending)
	seedKnowledgeVideo(t, db, 2, 2, knowledgevideo.VideoPending)
	seedKnowledgeVideo(t, db, 3, 3, knowledgevideo.VideoPending)
	seedKnowledgeVideo(t, db, 4, 4, knowledgevideo.VideoReady)
	if err := db.Model(&model.EduKnowledgeVideo{}).Where("id = ?", 2).Update("enqueue_time", cutoff.Add(-time.Minute)).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&model.EduKnowledgeVideo{}).Where("id = ?", 3).Update("enqueue_time", cutoff).Error; err != nil {
		t.Fatal(err)
	}

	got, err := repo.ListStalePending(context.Background(), cutoff, 2)
	if err != nil || len(got) != 2 || got[0].ID != 1 || got[1].ID != 2 {
		t.Fatalf("ListStalePending() = %+v, %v", got, err)
	}
}

func newKnowledgeVideoTestRepository(t *testing.T) (*GormKnowledgeVideoRepository, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := EnsureSchema(db); err != nil {
		t.Fatalf("EnsureSchema(): %v", err)
	}
	return NewGormKnowledgeVideoRepositoryWithAllocator(db, testKnowledgeVideoIDAllocator{}), db
}

type testKnowledgeVideoIDAllocator struct{}

func (testKnowledgeVideoIDAllocator) ReserveBatchID(context.Context) (uint64, error) { return 101, nil }

func (testKnowledgeVideoIDAllocator) ReserveVideoIDs(_ context.Context, count int) ([]uint64, error) {
	ids := make([]uint64, count)
	for i := range ids {
		ids[i] = uint64(201 + i)
	}
	return ids, nil
}

func seedKnowledgeVideo(t *testing.T, db *gorm.DB, id uint64, knowledgePointID uint64, status knowledgevideo.VideoStatus) {
	t.Helper()
	seedKnowledgeVideoInBatch(t, db, id, knowledgePointID, 1, status)
}

func seedKnowledgeVideoInBatch(t *testing.T, db *gorm.DB, id uint64, knowledgePointID uint64, batchID uint64, status knowledgevideo.VideoStatus) {
	t.Helper()
	fileName := fmt.Sprintf("source-%d.mp4", id)
	if err := db.Create(&model.EduKnowledgeVideo{ID: id, BatchID: batchID, KnowledgePointID: knowledgePointID, KnowledgePointName: "point", SourceFileName: fileName, SourceObjectKey: "raw/" + fileName, HLSObjectPrefix: fmt.Sprintf("hls/%d", id), Status: int16(status), Deleted: 0}).Error; err != nil {
		t.Fatalf("seed knowledge video: %v", err)
	}
}
