package main

import (
	"archive/zip"
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/minio/minio-go/v7"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"video-service/internal/application/knowledgevideo"
	"video-service/internal/config"
	"video-service/internal/infrastructure/objectstorage"
	"video-service/internal/infrastructure/persistence"
	infraredis "video-service/internal/infrastructure/redis"
	"video-service/internal/model"
)

type bulkBatch struct{ name, xlsxKey, zipKey string }
type bulkResult struct {
	Batch           string `json:"batch"`
	Status          string `json:"status"`
	Rows            int    `json:"rows"`
	UniqueObjects   int    `json:"unique_objects"`
	SkippedEmpty    int    `json:"skipped_empty_rows"`
	SkippedExisting int    `json:"skipped_existing_rows"`
	Error           string `json:"error,omitempty"`
}

func discoverBulkBatches(objects []objectstorage.ObjectInfo) []bulkBatch {
	byName := map[string]*bulkBatch{}
	for _, object := range objects {
		base := path.Base(object.Key)
		dir := path.Dir(object.Key)
		name := strings.TrimSuffix(base, path.Ext(base))
		if !strings.HasPrefix(name, "batch-") {
			continue
		}
		item := byName[dir+"/"+name]
		if item == nil {
			item = &bulkBatch{name: name}
			byName[dir+"/"+name] = item
		}
		if strings.EqualFold(path.Ext(base), ".xlsx") {
			item.xlsxKey = object.Key
		}
		if strings.EqualFold(path.Ext(base), ".zip") {
			item.zipKey = object.Key
		}
	}
	out := make([]bulkBatch, 0, len(byName))
	for _, item := range byName {
		out = append(out, *item)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].name < out[j].name })
	return out
}

func runBulk(ctx context.Context, opts options) error {
	if opts.uploadUserID == 0 || strings.TrimSpace(opts.batchKey) == "" {
		return errors.New("--upload-user-id and --batch-key are required with --all-prefix")
	}
	cfg := config.MustLoadDefault()
	source, err := sourceObjectStore(cfg)
	if err != nil {
		return err
	}
	objects, err := source.ListPrefix(ctx, opts.allPrefix)
	if err != nil {
		return err
	}
	batches := discoverBulkBatches(objects)
	if opts.reportDir == "" {
		opts.reportDir = filepath.Join(os.TempDir(), "knowledge-video-import-"+opts.batchKey)
	}
	if err := os.MkdirAll(opts.reportDir, 0o750); err != nil {
		return err
	}
	db, err := gorm.Open(postgres.Open(cfg.Postgres.DSN), &gorm.Config{})
	if err != nil {
		return err
	}
	repo := persistence.NewGormKnowledgeVideoRepository(db)
	var store *objectstorage.RustFS
	var queue *infraredis.KnowledgeVideoTranscodeQueue
	var rdb *redis.Client
	if !opts.dryRun {
		rdb = redis.NewClient(&redis.Options{Addr: cfg.Redis.Addr, Password: cfg.Redis.Password, DB: cfg.Redis.DB})
		if err := rdb.Ping(ctx).Err(); err != nil {
			return err
		}
		store, err = objectstorage.NewRustFS(config.KnowledgeVideoObjectStorageConfig(cfg))
		if err != nil {
			return err
		}
		queue = infraredis.NewKnowledgeVideoTranscodeQueue(rdb, cfg.RedisKeys.KnowledgeVideoTranscodeQueue)
		defer rdb.Close()
	}
	results := make([]bulkResult, 0, len(batches))
	for _, batch := range batches {
		result := bulkResult{Batch: batch.name}
		if !opts.dryRun {
			var existing model.EduKnowledgeVideoBatch
			key := "minio:" + opts.batchKey + ":" + batch.name
			err := db.WithContext(ctx).Where("zip_file_name = ?", key).First(&existing).Error
			if err == nil {
				result.Status = "skipped"
				results = append(results, result)
				continue
			}
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				result.Status = "failed"
				result.Error = err.Error()
				results = append(results, result)
				continue
			}
		}
		if batch.xlsxKey == "" || batch.zipKey == "" {
			result.Status = "invalid"
			result.Error = "batch must contain matching XLSX and ZIP"
			results = append(results, result)
			if !opts.continueOnError {
				break
			}
			continue
		}
		if err := processBulkBatch(ctx, opts, batch, source, repo, db, store, queue, &result); err != nil {
			result.Status = "failed"
			result.Error = err.Error()
			results = append(results, result)
			if !opts.continueOnError {
				break
			}
			continue
		}
		if result.Status == "" {
			result.Status = "dry-run"
		}
		if !opts.dryRun && result.Status != "skipped-empty" && result.Status != "skipped-existing" {
			result.Status = "imported"
		}
		results = append(results, result)
	}
	file, err := os.Create(path.Join(opts.reportDir, "summary.json"))
	if err != nil {
		return err
	}
	defer file.Close()
	if err := json.NewEncoder(file).Encode(results); err != nil {
		return err
	}
	if err := writeBulkCSV(path.Join(opts.reportDir, "valid-batches.csv"), results, false); err != nil {
		return err
	}
	return writeBulkCSV(path.Join(opts.reportDir, "invalid-batches.csv"), results, true)
}

func writeBulkCSV(filePath string, results []bulkResult, failures bool) error {
	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()
	writer := csv.NewWriter(file)
	defer writer.Flush()
	_ = writer.Write([]string{"batch", "status", "rows", "unique_objects", "skipped_empty_rows", "skipped_existing_rows", "error"})
	for _, result := range results {
		failed := result.Status == "failed" || result.Status == "invalid"
		if failed != failures {
			continue
		}
		_ = writer.Write([]string{result.Batch, result.Status, fmt.Sprint(result.Rows), fmt.Sprint(result.UniqueObjects), fmt.Sprint(result.SkippedEmpty), fmt.Sprint(result.SkippedExisting), result.Error})
	}
	return writer.Error()
}

func processBulkBatch(ctx context.Context, opts options, batch bulkBatch, source *objectstorage.RustFS, repo *persistence.GormKnowledgeVideoRepository, db *gorm.DB, store *objectstorage.RustFS, queue *infraredis.KnowledgeVideoTranscodeQueue, result *bulkResult) error {
	mapFile, err := os.CreateTemp("", "knowledge-video-bulk-*.xlsx")
	if err != nil {
		return err
	}
	mapPath := mapFile.Name()
	mapFile.Close()
	defer os.Remove(mapPath)
	if err := source.DownloadToFile(ctx, batch.xlsxKey, mapPath); err != nil {
		return err
	}
	_, strictIssues, strictErr := readMapping(mapPath)
	if strictErr != nil {
		return strictErr
	}
	rows, issues, err := readMappingWithOptions(mapPath, true)
	if err != nil {
		return err
	}
	if len(issues) > 0 {
		return formatIssues(issues)
	}
	for _, issue := range strictIssues {
		if issue.Field == "video_name" && issue.Message == "video name is required" {
			result.SkippedEmpty++
		}
	}
	if len(rows) == 0 {
		result.Status = "skipped-empty"
		return nil
	}
	points, err := repo.LookupKnowledgePoints(ctx, pointIDs(rows))
	if err != nil {
		return err
	}
	names := make(map[uint64]string, len(points))
	for _, point := range points {
		names[point.ID] = point.Name
	}
	for i := range rows {
		want, ok := names[rows[i].pointID]
		if !ok {
			return fmt.Errorf("row %d knowledge point %d does not exist", rows[i].row, rows[i].pointID)
		}
		if normalizeKnowledgePointName(rows[i].name) != want {
			return fmt.Errorf("row %d knowledge point name %q does not match dictionary %q", rows[i].row, rows[i].name, want)
		}
		rows[i].name = want
	}
	zipObject, err := source.Get(ctx, batch.zipKey, minio.GetObjectOptions{})
	if err != nil {
		return err
	}
	defer zipObject.Close()
	stat, err := source.Stat(ctx, batch.zipKey)
	if err != nil {
		return err
	}
	archive, err := zip.NewReader(zipObject, stat.Size)
	if err != nil {
		return err
	}
	entries := map[string]*zip.File{}
	duplicates := map[string]struct{}{}
	for _, entry := range archive.File {
		if isMetadataObject(entry.Name) || entry.FileInfo().IsDir() {
			continue
		}
		if _, exists := entries[path.Base(entry.Name)]; exists {
			duplicates[path.Base(entry.Name)] = struct{}{}
		}
		entries[path.Base(entry.Name)] = entry
	}
	if len(duplicates) > 0 {
		return fmt.Errorf("duplicate ZIP basenames: %v", sortedKeys(duplicates))
	}
	seen := map[string]struct{}{}
	for i := range rows {
		entry := entries[rows[i].objectRef]
		if entry == nil {
			return fmt.Errorf("row %d video %q missing from ZIP", rows[i].row, rows[i].objectRef)
		}
		rows[i].objectKey = entry.Name
		seen[entry.Name] = struct{}{}
	}
	rows, skippedExisting, err := filterExistingVideos(ctx, db, rows)
	if err != nil {
		return err
	}
	unique := make(map[string]struct{}, len(rows))
	for _, row := range rows {
		unique[row.objectKey] = struct{}{}
	}
	result.SkippedExisting = skippedExisting
	result.Rows = len(rows)
	result.UniqueObjects = len(unique)
	if len(rows) == 0 {
		result.Status = "skipped-existing"
		return nil
	}
	if opts.dryRun {
		return nil
	}
	batchID, err := repo.ReserveBatchID(ctx)
	if err != nil {
		return err
	}
	ids, err := repo.ReserveVideoIDs(ctx, len(rows))
	if err != nil {
		return err
	}
	now := time.Now()
	videos := make([]knowledgevideo.Video, 0, len(rows))
	copied := map[string]string{}
	for i, row := range rows {
		dest := copied[row.objectKey]
		if dest == "" {
			dest = fmt.Sprintf("direct-import/%d/%s", batchID, path.Base(row.objectKey))
			entry := entries[row.objectRef]
			in, e := entry.Open()
			if e != nil {
				return e
			}
			e = store.Put(ctx, dest, in, int64(entry.UncompressedSize64), videoContentType(path.Ext(entry.Name)))
			_ = in.Close()
			if e != nil {
				return e
			}
			copied[row.objectKey] = dest
		}
		videos = append(videos, knowledgevideo.Video{ID: ids[i], BatchID: batchID, KnowledgePointID: row.pointID, KnowledgePointName: row.name, SourceFileName: path.Base(row.objectKey), SourceObjectKey: dest, HLSObjectPrefix: fmt.Sprintf("hls/%d", ids[i]), Status: knowledgevideo.VideoPending, CreateTime: now, UpdateTime: now})
	}
	batchModel := knowledgevideo.Batch{ID: batchID, UploadUserID: opts.uploadUserID, ZipFileName: "minio:" + opts.batchKey + ":" + batch.name, XLSXFileName: path.Base(batch.xlsxKey), XLSXObjectKey: fmt.Sprintf("manifests/%d/mapping.xlsx", batchID), TotalCount: len(videos), Status: knowledgevideo.BatchProcessing, CreateTime: now, UpdateTime: now}
	if err := store.PutFile(ctx, batchModel.XLSXObjectKey, mapPath, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"); err != nil {
		return err
	}
	if err := repo.CreateBatch(ctx, batchModel, videos); err != nil {
		return err
	}
	for _, v := range videos {
		if err := queue.Enqueue(ctx, knowledgevideo.TranscodeTask{KnowledgeVideoID: v.ID, SourceObjectKey: v.SourceObjectKey, HLSObjectPrefix: v.HLSObjectPrefix, TaskID: fmt.Sprintf("knowledge-video-%d", v.ID)}); err == nil {
			_, _ = repo.SetEnqueueTime(ctx, v.ID, now)
		}
	}
	return nil
}

func pointIDs(rows []mappingRow) []uint64 {
	out := make([]uint64, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.pointID)
	}
	return out
}

func sortedKeys(values map[string]struct{}) []string {
	out := make([]string, 0, len(values))
	for value := range values {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}
