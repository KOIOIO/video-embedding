package main

import (
	"context"
	"crypto/sha256"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	goredis "github.com/go-redis/redis/v8"
	"github.com/minio/minio-go/v7"
	"github.com/xuri/excelize/v2"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"video-service/internal/application/knowledgevideo"
	"video-service/internal/config"
	"video-service/internal/infrastructure/objectstorage"
	"video-service/internal/infrastructure/persistence"
	infraredis "video-service/internal/infrastructure/redis"
	"video-service/internal/model"
)

type options struct {
	mapping         string
	sourcePrefix    string
	batchKey        string
	uploadUserID    uint64
	dryRun          bool
	listOnly        bool
	inspectObject   string
	allPrefix       string
	reportDir       string
	continueOnError bool
}

type mappingRow struct {
	row       int
	pointID   uint64
	name      string
	objectRef string
	objectKey string
	size      int64
}

func main() {
	config.EnsureProjectRoot()
	opts := parseOptions()
	if err := run(context.Background(), opts); err != nil {
		log.Fatal(err)
	}
}

func parseOptions() options {
	var opts options
	flag.StringVar(&opts.mapping, "mapping", "", "XLSX mapping path")
	flag.StringVar(&opts.sourcePrefix, "source-prefix", "", "allowed MinIO object prefix")
	flag.StringVar(&opts.batchKey, "batch-key", "", "idempotency key")
	flag.Uint64Var(&opts.uploadUserID, "upload-user-id", 0, "batch owner user ID")
	flag.BoolVar(&opts.dryRun, "dry-run", false, "validate without writing")
	flag.BoolVar(&opts.listOnly, "list-only", false, "list source MinIO objects without database access")
	flag.StringVar(&opts.inspectObject, "inspect-object", "", "download and print one source XLSX object")
	flag.StringVar(&opts.allPrefix, "all-prefix", "", "process every batch below a MinIO prefix")
	flag.StringVar(&opts.reportDir, "report-dir", "", "bulk report directory")
	flag.BoolVar(&opts.continueOnError, "continue-on-error", true, "continue bulk processing after a batch failure")
	flag.Parse()
	return opts
}

func run(ctx context.Context, opts options) error {
	if strings.TrimSpace(opts.allPrefix) != "" {
		return runBulk(ctx, opts)
	}
	if opts.listOnly || strings.TrimSpace(opts.inspectObject) != "" {
		if strings.TrimSpace(opts.sourcePrefix) == "" {
			return errors.New("--source-prefix is required with --list-only")
		}
		cfg := config.MustLoadDefault()
		sourceStore, err := sourceObjectStore(cfg)
		if err != nil {
			return err
		}
		if strings.TrimSpace(opts.inspectObject) != "" {
			objectKey := path.Join(strings.Trim(opts.sourcePrefix, "/"), strings.TrimPrefix(opts.inspectObject, "/"))
			temp, err := os.CreateTemp("", "knowledge-video-mapping-*.xlsx")
			if err != nil {
				return err
			}
			tempPath := temp.Name()
			_ = temp.Close()
			defer os.Remove(tempPath)
			if err := sourceStore.DownloadToFile(ctx, objectKey, tempPath); err != nil {
				return err
			}
			if book, openErr := excelize.OpenFile(tempPath); openErr == nil {
				if sheets := book.GetSheetList(); len(sheets) > 0 {
					if raw, rowsErr := book.GetRows(sheets[0]); rowsErr == nil && len(raw) > 0 {
						fmt.Printf("raw_header=%q\n", raw[0])
					}
				}
				_ = book.Close()
			}
			rows, issues, err := readMapping(tempPath)
			if err != nil {
				return err
			}
			fmt.Printf("object=%s rows=%d issues=%d\n", objectKey, len(rows), len(issues))
			for _, row := range rows {
				fmt.Printf("row=%d id=%d name=%s video_name=%s\n", row.row, row.pointID, row.name, row.objectRef)
			}
			for _, issue := range issues {
				fmt.Printf("issue row=%d field=%s message=%s\n", issue.Row, issue.Field, issue.Message)
			}
			return nil
		}
		objects, err := sourceStore.ListPrefix(ctx, opts.sourcePrefix)
		if err != nil {
			return err
		}
		sort.Slice(objects, func(i, j int) bool { return objects[i].Key < objects[j].Key })
		var total int64
		for _, object := range objects {
			if isMetadataObject(object.Key) {
				continue
			}
			total += object.Size
			fmt.Printf("%d\t%s\n", object.Size, object.Key)
		}
		fmt.Printf("objects=%d total_bytes=%d\n", len(objects), total)
		return nil
	}
	if strings.TrimSpace(opts.mapping) == "" || strings.TrimSpace(opts.sourcePrefix) == "" || strings.TrimSpace(opts.batchKey) == "" || opts.uploadUserID == 0 {
		return errors.New("--mapping, --source-prefix, --batch-key and positive --upload-user-id are required")
	}
	rows, issues, err := readMapping(opts.mapping)
	if err != nil {
		return err
	}
	if len(issues) > 0 {
		return formatIssues(issues)
	}

	cfg := config.MustLoadDefault()
	db, err := gorm.Open(postgres.Open(cfg.Postgres.DSN), &gorm.Config{})
	if err != nil {
		return fmt.Errorf("open postgres: %w", err)
	}
	rdb := goredis.NewClient(&goredis.Options{Addr: cfg.Redis.Addr, Password: cfg.Redis.Password, DB: cfg.Redis.DB})
	defer rdb.Close()
	if err := rdb.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("connect redis: %w", err)
	}
	store, err := objectstorage.NewRustFS(config.KnowledgeVideoObjectStorageConfig(cfg))
	if err != nil {
		return err
	}
	sourceStore, err := sourceObjectStore(cfg)
	if err != nil {
		return err
	}
	repo := persistence.NewGormKnowledgeVideoRepository(db)
	if err := resolveRows(ctx, repo, sourceStore, strings.Trim(opts.sourcePrefix, "/"), rows); err != nil {
		return err
	}
	rows, skippedExisting, err := filterExistingVideos(ctx, db, rows)
	if err != nil {
		return err
	}
	if skippedExisting > 0 {
		fmt.Printf("skipped_existing=%d\n", skippedExisting)
	}
	if len(rows) == 0 {
		fmt.Println("nothing to import: all rows already exist")
		return nil
	}
	unique := make(map[string]int64)
	var referencedBytes int64
	for _, row := range rows {
		unique[row.objectKey] = row.size
		referencedBytes += row.size
	}
	var uniqueBytes int64
	for _, size := range unique {
		uniqueBytes += size
	}
	fmt.Printf("validated rows=%d knowledge_points=%d unique_objects=%d reused_references=%d unique_bytes=%d referenced_bytes=%d\n", len(rows), uniquePointCount(rows), len(unique), len(rows)-len(unique), uniqueBytes, referencedBytes)
	if opts.dryRun {
		fmt.Println("dry_run=true writes=0")
		return nil
	}

	importKey := "minio:" + opts.batchKey
	var existing model.EduKnowledgeVideoBatch
	err = db.WithContext(ctx).Where("zip_file_name = ?", importKey).First(&existing).Error
	if err == nil {
		fmt.Printf("existing_batch_id=%d\n", existing.ID)
		return nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	batchID, err := repo.ReserveBatchID(ctx)
	if err != nil {
		return err
	}
	videoIDs, err := repo.ReserveVideoIDs(ctx, len(rows))
	if err != nil {
		return err
	}
	now := time.Now()
	manifestKey := fmt.Sprintf("manifests/%d/mapping.xlsx", batchID)
	copied := make(map[string]string, len(unique))
	uploadedKeys := make([]string, 0, len(unique)+1)
	for _, row := range rows {
		if _, exists := copied[row.objectKey]; exists {
			continue
		}
		digest := sha256.Sum256([]byte(row.objectKey))
		destinationKey := fmt.Sprintf("direct-import/%d/%x-%s", batchID, digest[:8], path.Base(row.objectKey))
		reader, openErr := sourceStore.Open(ctx, row.objectKey, minio.GetObjectOptions{})
		if openErr != nil {
			return openErr
		}
		putErr := store.Put(ctx, destinationKey, reader, row.size, videoContentType(path.Ext(row.objectKey)))
		closeErr := reader.Close()
		if putErr != nil {
			return putErr
		}
		if closeErr != nil {
			return closeErr
		}
		copied[row.objectKey] = destinationKey
		uploadedKeys = append(uploadedKeys, destinationKey)
	}
	videos := make([]knowledgevideo.Video, 0, len(rows))
	for i, row := range rows {
		videos = append(videos, knowledgevideo.Video{ID: videoIDs[i], BatchID: batchID, KnowledgePointID: row.pointID, KnowledgePointName: row.name, SourceFileName: path.Base(row.objectKey), SourceObjectKey: copied[row.objectKey], HLSObjectPrefix: fmt.Sprintf("hls/%d", videoIDs[i]), Status: knowledgevideo.VideoPending, CreateTime: now, UpdateTime: now})
	}
	batch := knowledgevideo.Batch{ID: batchID, UploadUserID: opts.uploadUserID, ZipFileName: importKey, XLSXFileName: filepath.Base(opts.mapping), XLSXObjectKey: manifestKey, TotalCount: len(videos), Status: knowledgevideo.BatchProcessing, CreateTime: now, UpdateTime: now}
	if err := store.PutFile(ctx, manifestKey, opts.mapping, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"); err != nil {
		deleteUploaded(ctx, store, uploadedKeys)
		return err
	}
	uploadedKeys = append(uploadedKeys, manifestKey)
	if err := repo.CreateBatch(ctx, batch, videos); err != nil {
		deleteUploaded(ctx, store, uploadedKeys)
		return err
	}
	queue := infraredis.NewKnowledgeVideoTranscodeQueue(rdb, cfg.RedisKeys.KnowledgeVideoTranscodeQueue)
	enqueued := 0
	for _, video := range videos {
		task := knowledgevideo.TranscodeTask{KnowledgeVideoID: video.ID, SourceObjectKey: video.SourceObjectKey, HLSObjectPrefix: video.HLSObjectPrefix, TaskID: fmt.Sprintf("knowledge-video-%d", video.ID)}
		if err := queue.Enqueue(ctx, task); err != nil {
			fmt.Fprintf(os.Stderr, "enqueue video_id=%d: %v\n", video.ID, err)
			continue
		}
		enqueued++
		_, _ = repo.SetEnqueueTime(ctx, video.ID, now)
	}
	fmt.Printf("batch_id=%d created=%d enqueued=%d\n", batchID, len(videos), enqueued)
	return nil
}

func sourceObjectStore(_ config.Config) (*objectstorage.RustFS, error) {
	endpoint := strings.TrimSpace(os.Getenv("SOURCE_MINIO_ENDPOINT"))
	bucket := strings.TrimSpace(os.Getenv("SOURCE_MINIO_BUCKET"))
	accessKey := strings.TrimSpace(os.Getenv("SOURCE_MINIO_ACCESS_KEY"))
	secretKey := strings.TrimSpace(os.Getenv("SOURCE_MINIO_SECRET_KEY"))
	if endpoint == "" || bucket == "" || accessKey == "" || secretKey == "" {
		return nil, errors.New("SOURCE_MINIO_ENDPOINT, SOURCE_MINIO_BUCKET, SOURCE_MINIO_ACCESS_KEY and SOURCE_MINIO_SECRET_KEY are required")
	}
	useSSL, _ := strconv.ParseBool(os.Getenv("SOURCE_MINIO_USE_SSL"))
	return objectstorage.NewRustFS(objectstorage.Config{Endpoint: endpoint, Bucket: bucket, AccessKey: accessKey, SecretKey: secretKey, UseSSL: useSSL, BucketLookup: "path"})
}

func deleteUploaded(ctx context.Context, store *objectstorage.RustFS, keys []string) {
	cleanupCtx := context.WithoutCancel(ctx)
	for i := len(keys) - 1; i >= 0; i-- {
		_ = store.Delete(cleanupCtx, keys[i])
	}
}

func videoContentType(ext string) string {
	switch strings.ToLower(ext) {
	case ".mp4", ".m4v":
		return "video/mp4"
	case ".mov":
		return "video/quicktime"
	case ".webm":
		return "video/webm"
	case ".avi":
		return "video/x-msvideo"
	case ".mkv":
		return "video/x-matroska"
	default:
		return "application/octet-stream"
	}
}

func readMapping(filePath string) ([]mappingRow, []knowledgevideo.ValidationIssue, error) {
	return readMappingWithOptions(filePath, false)
}

func readMappingWithOptions(filePath string, skipEmptyVideo bool) ([]mappingRow, []knowledgevideo.ValidationIssue, error) {
	book, err := excelize.OpenFile(filePath)
	if err != nil {
		return nil, nil, fmt.Errorf("open mapping: %w", err)
	}
	defer book.Close()
	sheets := book.GetSheetList()
	if len(sheets) == 0 {
		return nil, []knowledgevideo.ValidationIssue{{Field: "mapping", Message: "mapping must contain a worksheet"}}, nil
	}
	values, err := book.GetRows(sheets[0])
	if err != nil {
		return nil, nil, err
	}
	issues := make([]knowledgevideo.ValidationIssue, 0)
	want := []string{"id", "name", "video_name"}
	accepted := [][]string{{"id", "ID"}, {"name", "完整路径"}, {"video_name", "视频文件名称"}}
	if len(values) == 0 {
		return nil, []knowledgevideo.ValidationIssue{{Row: 1, Field: "mapping", Message: "mapping header is required"}}, nil
	}
	for i, header := range want {
		if !containsString(accepted[i], cell(values[0], i)) {
			issues = append(issues, knowledgevideo.ValidationIssue{Row: 1, Field: header, Message: "header must be exactly " + header})
		}
	}
	if len(values[0]) > 3 && strings.TrimSpace(cell(values[0], 3)) != "" {
		issues = append(issues, knowledgevideo.ValidationIssue{Row: 1, Field: "mapping", Message: "mapping must contain exactly three columns"})
	}
	rows := make([]mappingRow, 0, len(values)-1)
	for i := 1; i < len(values); i++ {
		idText := strings.TrimSpace(cell(values[i], 0))
		name := strings.TrimSpace(cell(values[i], 1))
		ref := strings.TrimSpace(cell(values[i], 2))
		if idText == "" && name == "" && ref == "" {
			continue
		}
		id, parseErr := strconv.ParseUint(idText, 10, 64)
		if parseErr != nil || id == 0 {
			issues = append(issues, knowledgevideo.ValidationIssue{Row: i + 1, Field: "id", Message: "id must be a positive integer"})
		}
		if name == "" {
			issues = append(issues, knowledgevideo.ValidationIssue{Row: i + 1, Field: "name", Message: "name is required"})
		}
		if ref == "" && !skipEmptyVideo {
			issues = append(issues, knowledgevideo.ValidationIssue{Row: i + 1, Field: "video_name", Message: "video name is required"})
		}
		if id > 0 && name != "" && ref != "" {
			rows = append(rows, mappingRow{row: i + 1, pointID: id, name: name, objectRef: ref})
		}
	}
	return rows, issues, nil
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func resolveRows(ctx context.Context, dictionary knowledgevideo.DictionaryLookup, store *objectstorage.RustFS, prefix string, rows []mappingRow) error {
	ids := make([]uint64, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.pointID)
	}
	points, err := dictionary.LookupKnowledgePoints(ctx, ids)
	if err != nil {
		return err
	}
	pointNames := make(map[uint64]string, len(points))
	for _, point := range points {
		pointNames[point.ID] = point.Name
	}
	objects, err := store.ListPrefix(ctx, prefix)
	if err != nil {
		return err
	}
	byBase := make(map[string][]objectstorage.ObjectInfo)
	byKey := make(map[string]objectstorage.ObjectInfo)
	for _, object := range objects {
		if isMetadataObject(object.Key) {
			continue
		}
		byKey[object.Key] = object
		byBase[path.Base(object.Key)] = append(byBase[path.Base(object.Key)], object)
	}
	issues := make([]knowledgevideo.ValidationIssue, 0)
	for i := range rows {
		row := &rows[i]
		wantName, ok := pointNames[row.pointID]
		if !ok {
			issues = append(issues, knowledgevideo.ValidationIssue{Row: row.row, Field: "id", Message: "knowledge point does not exist"})
		} else if normalizeKnowledgePointName(row.name) != wantName {
			issues = append(issues, knowledgevideo.ValidationIssue{Row: row.row, Field: "name", Message: "knowledge point name does not match dictionary"})
		} else {
			row.name = wantName
		}
		var matches []objectstorage.ObjectInfo
		if strings.Contains(row.objectRef, "/") {
			key := path.Join(prefix, strings.TrimPrefix(row.objectRef, "/"))
			if object, found := byKey[key]; found {
				matches = []objectstorage.ObjectInfo{object}
			}
		} else {
			matches = byBase[row.objectRef]
		}
		if len(matches) == 0 {
			issues = append(issues, knowledgevideo.ValidationIssue{Row: row.row, Field: "video_name", Message: "video is missing from object storage"})
			continue
		}
		if len(matches) > 1 {
			issues = append(issues, knowledgevideo.ValidationIssue{Row: row.row, Field: "video_name", Message: "video basename is ambiguous"})
			continue
		}
		if !supportedVideoExtension(path.Ext(matches[0].Key)) {
			issues = append(issues, knowledgevideo.ValidationIssue{Row: row.row, Field: "video_name", Message: "unsupported video extension"})
			continue
		}
		row.objectKey, row.size = matches[0].Key, matches[0].Size
	}
	if len(issues) > 0 {
		return &knowledgevideo.ValidationError{Issues: issues}
	}
	return nil
}

func normalizeKnowledgePointName(name string) string {
	parts := strings.Split(name, "/")
	return strings.TrimSpace(parts[len(parts)-1])
}

func supportedVideoExtension(ext string) bool {
	switch strings.ToLower(ext) {
	case ".mp4", ".mov", ".mkv", ".avi", ".webm", ".m4v":
		return true
	default:
		return false
	}
}

func isMetadataObject(key string) bool {
	for _, part := range strings.Split(key, "/") {
		if part == "__MACOSX" || part == ".DS_Store" || strings.HasPrefix(part, "._") {
			return true
		}
	}
	return false
}

func cell(row []string, index int) string {
	if index >= len(row) {
		return ""
	}
	return row[index]
}

func formatIssues(issues []knowledgevideo.ValidationIssue) error {
	sort.SliceStable(issues, func(i, j int) bool { return issues[i].Row < issues[j].Row })
	var lines []string
	for _, issue := range issues {
		lines = append(lines, fmt.Sprintf("row=%d field=%s: %s", issue.Row, issue.Field, issue.Message))
	}
	return errors.New(strings.Join(lines, "\n"))
}

func uniquePointCount(rows []mappingRow) int {
	ids := make(map[uint64]struct{})
	for _, row := range rows {
		ids[row.pointID] = struct{}{}
	}
	return len(ids)
}

func filterExistingVideos(ctx context.Context, db *gorm.DB, rows []mappingRow) ([]mappingRow, int, error) {
	if len(rows) == 0 {
		return rows, 0, nil
	}
	pointIDs := make(map[uint64]struct{}, len(rows))
	names := make(map[string]struct{}, len(rows))
	for _, row := range rows {
		pointIDs[row.pointID] = struct{}{}
		names[path.Base(row.objectKey)] = struct{}{}
	}
	var videos []model.EduKnowledgeVideo
	if err := db.WithContext(ctx).Where("deleted = 0 AND knowledge_point_id IN ? AND source_file_name IN ?", sortedPointIDs(pointIDs), sortedNames(names)).Find(&videos).Error; err != nil {
		return nil, 0, fmt.Errorf("query existing videos: %w", err)
	}
	existing := make(map[string]struct{}, len(videos))
	for _, video := range videos {
		existing[existingVideoKey(video.KnowledgePointID, video.SourceFileName)] = struct{}{}
	}
	kept, skipped := filterRowsByExisting(rows, existing)
	return kept, skipped, nil
}

func existingVideoKey(pointID uint64, fileName string) string {
	return strconv.FormatUint(pointID, 10) + "\x00" + fileName
}

func filterRowsByExisting(rows []mappingRow, existing map[string]struct{}) ([]mappingRow, int) {
	kept := make([]mappingRow, 0, len(rows))
	skipped := 0
	for _, row := range rows {
		if _, ok := existing[existingVideoKey(row.pointID, path.Base(row.objectKey))]; ok {
			skipped++
			continue
		}
		kept = append(kept, row)
	}
	return kept, skipped
}

func sortedPointIDs(ids map[uint64]struct{}) []uint64 {
	out := make([]uint64, 0, len(ids))
	for id := range ids {
		out = append(out, id)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func sortedNames(names map[string]struct{}) []string {
	out := make([]string, 0, len(names))
	for name := range names {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}
