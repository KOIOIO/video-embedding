package main

import (
	"context"
	"path"
	"path/filepath"
	"testing"
	"time"

	"github.com/xuri/excelize/v2"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"video-service/internal/infrastructure/objectstorage"
	"video-service/internal/model"
)

func TestNormalizeKnowledgePointName(t *testing.T) {
	if got := normalizeKnowledgePointName("4 古代诗歌四首 / 观沧海"); got != "观沧海" {
		t.Fatalf("normalizeKnowledgePointName() = %q", got)
	}
}

func TestDiscoverBulkBatchesPairsXLSXAndZIP(t *testing.T) {
	objects := []objectstorage.ObjectInfo{
		{Key: "root/batch-1/batch-1.xlsx", Size: 10},
		{Key: "root/batch-1/batch-1.zip", Size: 20},
		{Key: "root/batch-2/batch-2.xlsx", Size: 11},
	}
	batches := discoverBulkBatches(objects)
	if len(batches) != 2 || batches[0].name != "batch-1" || batches[0].xlsxKey == "" || batches[0].zipKey == "" {
		t.Fatalf("discoverBulkBatches() = %+v", batches)
	}
	if batches[1].name != "batch-2" || batches[1].zipKey != "" {
		t.Fatalf("second batch = %+v", batches[1])
	}
}

func TestReadMappingAllowsRepeatedVideoName(t *testing.T) {
	filePath := writeMapping(t, [][]string{
		{"id", "name", "video_name"},
		{"5316", "观沧海", "古代诗歌.mp4"},
		{"5317", "次北固山下", "古代诗歌.mp4"},
	})
	rows, issues, err := readMapping(filePath)
	if err != nil || len(issues) != 0 || len(rows) != 2 {
		t.Fatalf("readMapping() rows=%+v issues=%+v err=%v", rows, issues, err)
	}
}

func TestBulkMappingSkipsEmptyVideoRows(t *testing.T) {
	filePath := writeMapping(t, [][]string{{"ID", "完整路径", "视频文件名称"}, {"1", "知识点一", "lesson.mp4"}, {"2", "知识点二", ""}})
	rows, issues, err := readMappingWithOptions(filePath, true)
	if err != nil || len(issues) != 0 || len(rows) != 1 || rows[0].pointID != 1 {
		t.Fatalf("rows=%+v issues=%+v err=%v", rows, issues, err)
	}
}

func TestReadMappingAcceptsLegacyDeliveryHeaders(t *testing.T) {
	filePath := writeMapping(t, [][]string{{"ID", "完整路径", "视频文件名称"}, {"1", "name", "video.mp4"}})
	rows, issues, err := readMapping(filePath)
	if err != nil || len(issues) != 0 || len(rows) != 1 {
		t.Fatalf("readMapping() issues=%+v err=%v", issues, err)
	}
}

func TestFilterRowsByExistingSkipsSamePointAndFile(t *testing.T) {
	existing := map[string]struct{}{existingVideoKey(9, "lesson.mp4"): {}}
	rows := []mappingRow{
		{row: 2, pointID: 9, objectKey: "prefix/a/lesson.mp4"},
		{row: 3, pointID: 10, objectKey: "prefix/lesson.mp4"},
		{row: 4, pointID: 9, objectKey: "prefix/new.mp4"},
	}
	kept, skipped := filterRowsByExisting(rows, existing)
	if skipped != 1 || len(kept) != 2 {
		t.Fatalf("kept=%+v skipped=%d", kept, skipped)
	}
	for _, row := range kept {
		if row.pointID == 9 && path.Base(row.objectKey) == "lesson.mp4" {
			t.Fatal("already imported video was not skipped")
		}
	}
}

func TestFilterExistingVideosSkipsOnlyActiveDuplicates(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.EduKnowledgeVideo{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	now := time.Now()
	videos := []model.EduKnowledgeVideo{
		{ID: 1, BatchID: 1, KnowledgePointID: 9, KnowledgePointName: "p", SourceFileName: "lesson.mp4", SourceObjectKey: "k1", HLSObjectPrefix: "hls/1", Deleted: 0, CreateTime: now, UpdateTime: now},
		{ID: 2, BatchID: 1, KnowledgePointID: 9, KnowledgePointName: "p", SourceFileName: "old.mp4", SourceObjectKey: "k2", HLSObjectPrefix: "hls/2", Deleted: 1, CreateTime: now, UpdateTime: now},
		{ID: 3, BatchID: 2, KnowledgePointID: 10, KnowledgePointName: "q", SourceFileName: "lesson.mp4", SourceObjectKey: "k3", HLSObjectPrefix: "hls/3", Deleted: 0, CreateTime: now, UpdateTime: now},
	}
	if err := db.Create(&videos).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}
	rows := []mappingRow{
		{row: 2, pointID: 9, objectKey: "prefix/lesson.mp4"},
		{row: 3, pointID: 9, objectKey: "prefix/old.mp4"},
		{row: 4, pointID: 10, objectKey: "prefix/lesson.mp4"},
		{row: 5, pointID: 11, objectKey: "prefix/other.mp4"},
	}
	kept, skipped, err := filterExistingVideos(context.Background(), db, rows)
	if err != nil {
		t.Fatalf("filterExistingVideos() error = %v", err)
	}
	if skipped != 2 || len(kept) != 2 {
		t.Fatalf("kept=%+v skipped=%d", kept, skipped)
	}
	keptKeys := make(map[string]bool, len(kept))
	for _, row := range kept {
		keptKeys[existingVideoKey(row.pointID, path.Base(row.objectKey))] = true
	}
	if !keptKeys[existingVideoKey(9, "old.mp4")] || !keptKeys[existingVideoKey(11, "other.mp4")] {
		t.Fatalf("soft-deleted or new videos were dropped: %+v", kept)
	}
}

func writeMapping(t *testing.T, rows [][]string) string {
	t.Helper()
	book := excelize.NewFile()
	sheet := book.GetSheetName(0)
	for rowIndex, row := range rows {
		for columnIndex, value := range row {
			cell, err := excelize.CoordinatesToCellName(columnIndex+1, rowIndex+1)
			if err != nil {
				t.Fatal(err)
			}
			if err := book.SetCellValue(sheet, cell, value); err != nil {
				t.Fatal(err)
			}
		}
	}
	filePath := filepath.Join(t.TempDir(), "mapping.xlsx")
	if err := book.SaveAs(filePath); err != nil {
		t.Fatal(err)
	}
	return filePath
}
