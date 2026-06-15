package persistence

import (
	"context"
	"testing"

	"nlp-video-analysis/internal/model"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newVectorStageTestRepo(t *testing.T) (*VectorStageRepository, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.EduVideoVectorStage{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return NewVectorStageRepository(db), db
}

func TestVectorStageRepositoryUpsertIsIdempotent(t *testing.T) {
	ctx := context.Background()
	repo, db := newVectorStageTestRepo(t)

	in := VectorStageRecord{
		TaskID:       "42",
		VideoID:      42,
		Stage:        "vector.coarse.asr",
		SegmentIndex: 3,
		StartSec:     120,
		EndSec:       160,
		ObjectKey:    "obj-a",
	}
	if err := repo.UpsertPending(ctx, in); err != nil {
		t.Fatalf("first upsert: %v", err)
	}
	in.ObjectKey = "obj-b"
	if err := repo.UpsertPending(ctx, in); err != nil {
		t.Fatalf("second upsert: %v", err)
	}

	var rows []model.EduVideoVectorStage
	if err := db.Find(&rows).Error; err != nil {
		t.Fatalf("find: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(rows))
	}
	if rows[0].ObjectKey != "obj-b" {
		t.Fatalf("object key = %q, want obj-b", rows[0].ObjectKey)
	}
}

func TestVectorStageRepositoryMarkCompleteAndCount(t *testing.T) {
	ctx := context.Background()
	repo, _ := newVectorStageTestRepo(t)

	for i := 0; i < 3; i++ {
		if err := repo.UpsertPending(ctx, VectorStageRecord{
			TaskID:       "42",
			VideoID:      42,
			Stage:        "vector.coarse.asr",
			SegmentIndex: i,
		}); err != nil {
			t.Fatalf("upsert %d: %v", i, err)
		}
	}
	if err := repo.MarkComplete(ctx, VectorStageRecord{
		TaskID:       "42",
		Stage:        "vector.coarse.asr",
		SegmentIndex: 0,
		Text:         "first",
	}); err != nil {
		t.Fatalf("complete: %v", err)
	}

	total, complete, err := repo.CountStage(ctx, "42", "vector.coarse.asr")
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if total != 3 || complete != 1 {
		t.Fatalf("count = total %d complete %d, want 3/1", total, complete)
	}
}

func TestVectorStageRepositoryMarkCompleteCreatesMissingStage(t *testing.T) {
	ctx := context.Background()
	repo, _ := newVectorStageTestRepo(t)

	if err := repo.MarkComplete(ctx, VectorStageRecord{
		TaskID:  "task-1",
		VideoID: 42,
		Stage:   "vector.coarse",
		EndSec:  120,
		Text:    "done",
	}); err != nil {
		t.Fatalf("complete: %v", err)
	}

	rec, found, err := repo.FindStage(ctx, "task-1", "vector.coarse", 0, 0)
	if err != nil {
		t.Fatalf("FindStage: %v", err)
	}
	if !found {
		t.Fatal("expected completed stage to be created")
	}
	if rec.Status != VectorStageStatusComplete || rec.VideoID != 42 || rec.EndSec != 120 || rec.Text != "done" {
		t.Fatalf("unexpected record: %+v", rec)
	}
}

func TestVectorStageRepositoryMarkCompletePreservesExistingVideoID(t *testing.T) {
	ctx := context.Background()
	repo, _ := newVectorStageTestRepo(t)

	if err := repo.UpsertPending(ctx, VectorStageRecord{
		TaskID:  "task-1",
		VideoID: 42,
		Stage:   "vector.finalize",
	}); err != nil {
		t.Fatalf("UpsertPending: %v", err)
	}
	if err := repo.MarkComplete(ctx, VectorStageRecord{
		TaskID: "task-1",
		Stage:  "vector.finalize",
		EndSec: 120,
	}); err != nil {
		t.Fatalf("complete: %v", err)
	}

	rec, found, err := repo.FindStage(ctx, "task-1", "vector.finalize", 0, 0)
	if err != nil {
		t.Fatalf("FindStage: %v", err)
	}
	if !found {
		t.Fatal("expected stage to be found")
	}
	if rec.VideoID != 42 || rec.Status != VectorStageStatusComplete || rec.EndSec != 120 {
		t.Fatalf("unexpected record: %+v", rec)
	}
}

func TestVectorStageRepositoryMarkFailed(t *testing.T) {
	ctx := context.Background()
	repo, db := newVectorStageTestRepo(t)

	if err := repo.UpsertPending(ctx, VectorStageRecord{
		TaskID:       "42",
		VideoID:      42,
		Stage:        "vector.coarse.asr",
		SegmentIndex: 1,
	}); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if err := repo.MarkFailed(ctx, VectorStageRecord{
		TaskID:       "42",
		Stage:        "vector.coarse.asr",
		SegmentIndex: 1,
		ErrorMessage: "asr failed",
		RetryCount:   2,
	}); err != nil {
		t.Fatalf("failed: %v", err)
	}

	var row model.EduVideoVectorStage
	if err := db.First(&row).Error; err != nil {
		t.Fatalf("first: %v", err)
	}
	if row.Status != VectorStageStatusFailed || row.ErrorMessage != "asr failed" || row.RetryCount != 2 {
		t.Fatalf("unexpected row: %+v", row)
	}
}

func TestVectorStageRepositoryFindStage(t *testing.T) {
	ctx := context.Background()
	repo, _ := newVectorStageTestRepo(t)

	if err := repo.UpsertPending(ctx, VectorStageRecord{
		TaskID:       "task-1",
		VideoID:      10,
		Stage:        "vector.coarse.segment",
		SegmentIndex: 2,
		ObjectKey:    "segments/coarse/video_10/task-1/seg_002.mp4",
		StartSec:     40,
		EndSec:       80,
	}); err != nil {
		t.Fatalf("UpsertPending: %v", err)
	}

	rec, found, err := repo.FindStage(ctx, "task-1", "vector.coarse.segment", 2, 0)
	if err != nil {
		t.Fatalf("FindStage: %v", err)
	}
	if !found {
		t.Fatal("expected stage to be found")
	}
	if rec.ObjectKey != "segments/coarse/video_10/task-1/seg_002.mp4" || rec.StartSec != 40 || rec.EndSec != 80 {
		t.Fatalf("unexpected record: %+v", rec)
	}
}

func TestVectorStageRepositoryListStageOrdered(t *testing.T) {
	ctx := context.Background()
	repo, _ := newVectorStageTestRepo(t)

	for _, idx := range []int{2, 0, 1} {
		if err := repo.UpsertPending(ctx, VectorStageRecord{
			TaskID:       "task-1",
			VideoID:      10,
			Stage:        "vector.coarse.segment",
			SegmentIndex: idx,
		}); err != nil {
			t.Fatalf("UpsertPending: %v", err)
		}
	}

	recs, err := repo.ListStage(ctx, "task-1", "vector.coarse.segment")
	if err != nil {
		t.Fatalf("ListStage: %v", err)
	}
	if len(recs) != 3 {
		t.Fatalf("len = %d, want 3", len(recs))
	}
	for i, rec := range recs {
		if rec.SegmentIndex != i {
			t.Fatalf("record %d SegmentIndex = %d, want %d", i, rec.SegmentIndex, i)
		}
	}
}
