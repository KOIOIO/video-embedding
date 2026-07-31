package knowledgevideo

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"reflect"
	"testing"
	"time"
)

func TestImportCreatesBatchUploadsExpectedObjectsAndEnqueues(t *testing.T) {
	deps := newImportTestDependencies(t)
	result, err := deps.service.Import(context.Background(), deps.validInput(t))
	if err != nil {
		t.Fatalf("Import() error = %v", err)
	}
	if result != (ImportResult{BatchID: 101, TotalCount: 2, Status: BatchProcessing}) {
		t.Fatalf("Import() = %+v", result)
	}
	wantKeys := []string{"manifests/101/mapping.xlsx", "raw/201/source.mp4", "raw/202/source.webm"}
	if !reflect.DeepEqual(deps.store.putKeys, wantKeys) {
		t.Fatalf("uploaded keys = %v, want %v", deps.store.putKeys, wantKeys)
	}
	if deps.repository.createdBatch.ID != 101 || len(deps.repository.createdVideos) != 2 {
		t.Fatalf("created batch/videos = %+v/%+v", deps.repository.createdBatch, deps.repository.createdVideos)
	}
	if got := deps.repository.createdVideos[0]; got.ID != 201 || got.SourceObjectKey != "raw/201/source.mp4" || got.HLSObjectPrefix != "hls/201" || got.Status != VideoPending {
		t.Fatalf("first video = %+v", got)
	}
	wantTasks := []TranscodeTask{
		{KnowledgeVideoID: 201, SourceObjectKey: "raw/201/source.mp4", HLSObjectPrefix: "hls/201", TaskID: "knowledge-video-201"},
		{KnowledgeVideoID: 202, SourceObjectKey: "raw/202/source.webm", HLSObjectPrefix: "hls/202", TaskID: "knowledge-video-202"},
	}
	if !reflect.DeepEqual(deps.queue.tasks, wantTasks) {
		t.Fatalf("tasks = %+v, want %+v", deps.queue.tasks, wantTasks)
	}
	if len(deps.repository.enqueueTimes) != 2 {
		t.Fatalf("enqueue times = %v, want two", deps.repository.enqueueTimes)
	}
	deps.assertTempRootEmpty(t)
}

func TestImportAddsMultipleVideosToSameKnowledgePoint(t *testing.T) {
	deps := newImportTestDependencies(t)
	input := deps.validInput(t)
	input.Mapping = readerBytes(t, mappingXLSX(t, [][]string{
		{"id", "name", "video_name"},
		{"9", "一次函数", "lesson.mp4"},
		{"9", "一次函数", "second.webm"},
	}))

	result, err := deps.service.Import(context.Background(), input)
	if err != nil {
		t.Fatalf("Import() error = %v", err)
	}
	if result.TotalCount != 2 || len(deps.repository.createdVideos) != 2 {
		t.Fatalf("result/videos = %+v/%+v", result, deps.repository.createdVideos)
	}
	for _, video := range deps.repository.createdVideos {
		if video.KnowledgePointID != 9 {
			t.Fatalf("created video = %+v", video)
		}
	}
}

func TestImportValidationFailureHasNoPersistentSideEffects(t *testing.T) {
	deps := newImportTestDependencies(t)
	input := deps.validInput(t)
	input.Mapping = bytes.NewReader([]byte("not xlsx"))
	_, err := deps.service.Import(context.Background(), input)
	var validationErr *ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("Import() error = %v, want ValidationError", err)
	}
	if deps.repository.reserveCalls != 0 || len(deps.store.putKeys) != 0 || deps.repository.createCalls != 0 || len(deps.queue.tasks) != 0 {
		t.Fatalf("side effects: repo=%+v puts=%v tasks=%v", deps.repository, deps.store.putKeys, deps.queue.tasks)
	}
	deps.assertTempRootEmpty(t)
}

func TestImportSecondSourceUploadFailureDeletesSuccessfulObjectsInReverse(t *testing.T) {
	deps := newImportTestDependencies(t)
	deps.store.putErrAt = 3
	_, err := deps.service.Import(context.Background(), deps.validInput(t))
	if err == nil {
		t.Fatal("Import() error = nil")
	}
	wantDeletes := []string{"raw/201/source.mp4", "manifests/101/mapping.xlsx"}
	if !reflect.DeepEqual(deps.store.deletedKeys, wantDeletes) {
		t.Fatalf("deleted keys = %v, want %v", deps.store.deletedKeys, wantDeletes)
	}
	if deps.repository.createCalls != 0 || len(deps.queue.tasks) != 0 {
		t.Fatalf("create calls/tasks = %d/%v", deps.repository.createCalls, deps.queue.tasks)
	}
	deps.assertTempRootEmpty(t)
}

func TestImportDatabaseConflictDeletesAllUploadedObjects(t *testing.T) {
	deps := newImportTestDependencies(t)
	deps.repository.createErr = errors.New("active knowledge point conflict")
	_, err := deps.service.Import(context.Background(), deps.validInput(t))
	if !errors.Is(err, deps.repository.createErr) {
		t.Fatalf("Import() error = %v", err)
	}
	wantDeletes := []string{"raw/202/source.webm", "raw/201/source.mp4", "manifests/101/mapping.xlsx"}
	if !reflect.DeepEqual(deps.store.deletedKeys, wantDeletes) {
		t.Fatalf("deleted keys = %v, want %v", deps.store.deletedKeys, wantDeletes)
	}
	if len(deps.queue.tasks) != 0 {
		t.Fatalf("tasks = %v", deps.queue.tasks)
	}
	deps.assertTempRootEmpty(t)
}

func TestImportEnqueueFailureLeavesCommittedPendingVideo(t *testing.T) {
	deps := newImportTestDependencies(t)
	deps.queue.errAt = 1
	result, err := deps.service.Import(context.Background(), deps.validInput(t))
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != BatchProcessing || deps.repository.createdVideos[0].Status != VideoPending {
		t.Fatalf("result=%+v rows=%+v", result, deps.repository.createdVideos)
	}
	if len(deps.store.deletedKeys) != 0 {
		t.Fatalf("committed objects were deleted: %v", deps.store.deletedKeys)
	}
	if _, exists := deps.repository.enqueueTimes[201]; exists {
		t.Fatal("failed enqueue should remain immediately reconcilable")
	}
	if _, exists := deps.repository.enqueueTimes[202]; !exists {
		t.Fatal("successful enqueue time was not recorded")
	}
	deps.assertTempRootEmpty(t)
}

type importTestDependencies struct {
	service    *ImportService
	repository *importTestRepository
	store      *importTestStore
	queue      *importTestQueue
	tempRoot   string
}

func newImportTestDependencies(t *testing.T) *importTestDependencies {
	t.Helper()
	repository := &importTestRepository{}
	store := &importTestStore{}
	queue := &importTestQueue{}
	tempRoot := t.TempDir()
	service := &ImportService{
		Validator:  Validator{Dictionary: fakeDictionary{9: "一次函数", 10: "二次函数"}, Limits: testValidationLimits()},
		Repository: repository,
		Store:      store,
		Queue:      queue,
		TempRoot:   tempRoot,
		Now:        func() time.Time { return time.Date(2026, 7, 23, 12, 0, 0, 0, time.UTC) },
	}
	return &importTestDependencies{service: service, repository: repository, store: store, queue: queue, tempRoot: tempRoot}
}

func (d *importTestDependencies) validInput(t *testing.T) ImportInput {
	t.Helper()
	return ImportInput{
		UploadUserID: 7,
		ArchiveName:  "videos.zip",
		MappingName:  "mapping.xlsx",
		Archive: readerBytes(t, zipArchive(t,
			zipEntry{name: "chapter/lesson.mp4", body: "video-one"},
			zipEntry{name: "second.webm", body: "video-two"},
		)),
		Mapping: readerBytes(t, mappingXLSX(t, [][]string{
			{"id", "name", "video_name"},
			{"9", "一次函数", "lesson.mp4"},
			{"10", "二次函数", "second.webm"},
		})),
	}
}

func (d *importTestDependencies) assertTempRootEmpty(t *testing.T) {
	t.Helper()
	entries, err := os.ReadDir(d.tempRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("temporary entries remain: %v", entries)
	}
}

func readerBytes(t *testing.T, reader io.Reader) *bytes.Reader {
	t.Helper()
	data, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	return bytes.NewReader(data)
}

type importTestRepository struct {
	reserveCalls  int
	createCalls   int
	createErr     error
	createdBatch  Batch
	createdVideos []Video
	enqueueTimes  map[uint64]time.Time
}

func (r *importTestRepository) ReserveBatchID(context.Context) (uint64, error) {
	r.reserveCalls++
	return 101, nil
}

func (r *importTestRepository) ReserveVideoIDs(_ context.Context, count int) ([]uint64, error) {
	r.reserveCalls++
	ids := make([]uint64, count)
	for i := range ids {
		ids[i] = uint64(201 + i)
	}
	return ids, nil
}

func (r *importTestRepository) CreateBatch(_ context.Context, batch Batch, videos []Video) error {
	r.createCalls++
	if r.createErr != nil {
		return r.createErr
	}
	r.createdBatch = batch
	r.createdVideos = append([]Video(nil), videos...)
	return nil
}

func (r *importTestRepository) SetEnqueueTime(_ context.Context, id uint64, value time.Time) (bool, error) {
	if r.enqueueTimes == nil {
		r.enqueueTimes = make(map[uint64]time.Time)
	}
	r.enqueueTimes[id] = value
	return true, nil
}

type importTestStore struct {
	putKeys     []string
	deletedKeys []string
	putErrAt    int
}

func (s *importTestStore) PutFile(_ context.Context, objectKey, filePath, _ string) error {
	s.putKeys = append(s.putKeys, objectKey)
	if _, err := os.Stat(filePath); err != nil {
		return err
	}
	if s.putErrAt > 0 && len(s.putKeys) == s.putErrAt {
		return errors.New("object upload failed")
	}
	return nil
}

func (s *importTestStore) Delete(_ context.Context, objectKey string) error {
	s.deletedKeys = append(s.deletedKeys, objectKey)
	return nil
}

type importTestQueue struct {
	tasks []TranscodeTask
	errAt int
	calls int
}

func (q *importTestQueue) Enqueue(_ context.Context, task TranscodeTask) error {
	q.calls++
	if q.errAt > 0 && q.calls == q.errAt {
		return errors.New("redis unavailable")
	}
	q.tasks = append(q.tasks, task)
	return nil
}
