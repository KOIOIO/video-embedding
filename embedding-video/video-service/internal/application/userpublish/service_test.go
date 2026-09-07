package userpublish

import (
	"context"
	"errors"
	"testing"

	"video-service/internal/application/videoapp"
	domainvideo "video-service/internal/domain/video"
	"video-service/internal/model"
)

type mockRepo struct {
	videos      map[uint64]*model.EduVideoResource
	nextID      uint64
	createErr   error
	getErr      error
	listErr     error
	updateErr   error
	listResults []model.EduVideoResource
	listTotal   int64
}

func newMockRepo() *mockRepo {
	return &mockRepo{
		videos: make(map[uint64]*model.EduVideoResource),
		nextID: 1,
	}
}

func (m *mockRepo) CreateVideo(_ context.Context, video *model.EduVideoResource) error {
	if m.createErr != nil {
		return m.createErr
	}
	video.ID = m.nextID
	m.nextID++
	m.videos[video.ID] = video
	return nil
}

func (m *mockRepo) GetByID(_ context.Context, id uint64) (*model.EduVideoResource, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	v, ok := m.videos[id]
	if !ok {
		return nil, nil
	}
	return v, nil
}

func (m *mockRepo) ListByUserID(_ context.Context, _ uint64, _, _ int) ([]model.EduVideoResource, int64, error) {
	if m.listErr != nil {
		return nil, 0, m.listErr
	}
	return m.listResults, m.listTotal, nil
}

func (m *mockRepo) UpdateStatus(_ context.Context, id uint64, status int16, errMsg string) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	if v, ok := m.videos[id]; ok {
		v.Status = status
		v.ErrorMsg = errMsg
	}
	return nil
}

type mockQueue struct {
	enqueued []videoapp.TranscodeTask
	err      error
}

func (q *mockQueue) Enqueue(_ context.Context, task videoapp.TranscodeTask) error {
	if q.err != nil {
		return q.err
	}
	q.enqueued = append(q.enqueued, task)
	return nil
}

type mockVectorQueue struct {
	enqueued []videoapp.VectorizeTask
}

func (q *mockVectorQueue) Enqueue(_ context.Context, task videoapp.VectorizeTask) error {
	q.enqueued = append(q.enqueued, task)
	return nil
}

func newTestService(repo Repository, queue TranscodeQueue) *Service {
	return NewService(repo, queue, &mockVectorQueue{})
}

func TestPublishVideo_Success(t *testing.T) {
	repo := newMockRepo()
	queue := &mockQueue{}
	svc := newTestService(repo, queue)

	video, err := svc.PublishVideo(context.Background(), 1001, "测试视频", "这是描述", "http://example.com/video.mp4", 60, "raw/test.mp4", "hls/test", "http://example.com/hls/test/master.m3u8")
	if err != nil {
		t.Fatalf("PublishVideo returned error: %v", err)
	}
	if video.ID == 0 {
		t.Fatal("video ID should be set")
	}
	if video.UserID != 1001 {
		t.Fatalf("UserID = %d, want 1001", video.UserID)
	}
	if video.SourceType != model.VideoSourceUserPublish {
		t.Fatalf("SourceType = %q, want %q", video.SourceType, model.VideoSourceUserPublish)
	}
	if video.Status != int16(domainvideo.StatusUploaded) {
		t.Fatalf("Status = %d, want %d", video.Status, domainvideo.StatusUploaded)
	}
	if !video.IsPublish {
		t.Fatal("IsPublish should be true")
	}
	if len(queue.enqueued) != 1 {
		t.Fatalf("enqueued = %d, want 1", len(queue.enqueued))
	}
	if queue.enqueued[0].VideoID != video.ID {
		t.Fatalf("enqueued VideoID = %d, want %d", queue.enqueued[0].VideoID, video.ID)
	}
}

func TestPublishVideo_EmptyTitle(t *testing.T) {
	repo := newMockRepo()
	svc := newTestService(repo, &mockQueue{})

	_, err := svc.PublishVideo(context.Background(), 1001, "  ", "desc", "http://example.com/video.mp4", 60, "raw/test.mp4", "hls/test", "http://example.com/hls/test/master.m3u8")
	if err == nil {
		t.Fatal("expected error for empty title")
	}
}

func TestPublishVideo_TitleTooLong(t *testing.T) {
	repo := newMockRepo()
	svc := newTestService(repo, &mockQueue{})

	longTitle := make([]rune, MaxTitleLength+1)
	for i := range longTitle {
		longTitle[i] = 'a'
	}
	_, err := svc.PublishVideo(context.Background(), 1001, string(longTitle), "desc", "http://example.com/video.mp4", 60, "raw/test.mp4", "hls/test", "http://example.com/hls/test/master.m3u8")
	if err == nil {
		t.Fatal("expected error for title too long")
	}
}

func TestPublishVideo_DescriptionTooLong(t *testing.T) {
	repo := newMockRepo()
	svc := newTestService(repo, &mockQueue{})

	longDesc := make([]rune, MaxDescriptionLength+1)
	for i := range longDesc {
		longDesc[i] = 'a'
	}
	_, err := svc.PublishVideo(context.Background(), 1001, "title", string(longDesc), "http://example.com/video.mp4", 60, "raw/test.mp4", "hls/test", "http://example.com/hls/test/master.m3u8")
	if err == nil {
		t.Fatal("expected error for description too long")
	}
}

func TestPublishVideo_EmptyVideoURL(t *testing.T) {
	repo := newMockRepo()
	svc := newTestService(repo, &mockQueue{})

	_, err := svc.PublishVideo(context.Background(), 1001, "title", "desc", "  ", 60, "raw/test.mp4", "hls/test", "http://example.com/hls/test/master.m3u8")
	if err == nil {
		t.Fatal("expected error for empty video URL")
	}
}

func TestPublishVideo_ZeroUserID(t *testing.T) {
	repo := newMockRepo()
	svc := newTestService(repo, &mockQueue{})

	_, err := svc.PublishVideo(context.Background(), 0, "title", "desc", "http://example.com/video.mp4", 60, "raw/test.mp4", "hls/test", "http://example.com/hls/test/master.m3u8")
	if err == nil {
		t.Fatal("expected error for zero user ID")
	}
}

func TestPublishVideo_EnqueueFailureMarksFailed(t *testing.T) {
	repo := newMockRepo()
	queue := &mockQueue{err: errors.New("queue unavailable")}
	svc := newTestService(repo, queue)

	_, err := svc.PublishVideo(context.Background(), 1001, "title", "desc", "http://example.com/video.mp4", 60, "raw/test.mp4", "hls/test", "http://example.com/hls/test/master.m3u8")
	if err == nil {
		t.Fatal("expected error from queue failure")
	}
	// Video should have been created then marked as failed
	if len(repo.videos) != 1 {
		t.Fatalf("videos = %d, want 1", len(repo.videos))
	}
	for _, v := range repo.videos {
		if v.Status != int16(domainvideo.StatusFailed) {
			t.Fatalf("Status = %d, want %d (failed)", v.Status, domainvideo.StatusFailed)
		}
	}
}

func TestGetVideoStatus_Success(t *testing.T) {
	repo := newMockRepo()
	svc := newTestService(repo, &mockQueue{})

	video, _ := svc.PublishVideo(context.Background(), 1001, "title", "desc", "http://example.com/video.mp4", 60, "raw/test.mp4", "hls/test", "http://example.com/hls/test/master.m3u8")
	repo.videos[video.ID].Status = int16(domainvideo.StatusDone)
	repo.videos[video.ID].ErrorMsg = ""

	status, errMsg, err := svc.GetVideoStatus(context.Background(), video.ID)
	if err != nil {
		t.Fatalf("GetVideoStatus returned error: %v", err)
	}
	if status != int16(domainvideo.StatusDone) {
		t.Fatalf("status = %d, want %d", status, domainvideo.StatusDone)
	}
	if errMsg != "" {
		t.Fatalf("errMsg = %q, want empty", errMsg)
	}
}

func TestGetVideoStatus_NotFound(t *testing.T) {
	repo := newMockRepo()
	svc := newTestService(repo, &mockQueue{})

	_, _, err := svc.GetVideoStatus(context.Background(), 9999)
	if err == nil {
		t.Fatal("expected error for not found")
	}
}

func TestListUserVideos_Success(t *testing.T) {
	repo := newMockRepo()
	repo.listResults = []model.EduVideoResource{
		{ID: 1, Title: "video1"},
		{ID: 2, Title: "video2"},
	}
	repo.listTotal = 2
	svc := newTestService(repo, &mockQueue{})

	list, total, err := svc.ListUserVideos(context.Background(), 1001, 1, 12)
	if err != nil {
		t.Fatalf("ListUserVideos returned error: %v", err)
	}
	if total != 2 {
		t.Fatalf("total = %d, want 2", total)
	}
	if len(list) != 2 {
		t.Fatalf("len(list) = %d, want 2", len(list))
	}
}

func TestListUserVideos_DefaultPagination(t *testing.T) {
	repo := newMockRepo()
	svc := newTestService(repo, &mockQueue{})

	_, _, err := svc.ListUserVideos(context.Background(), 1001, 0, 0)
	if err != nil {
		t.Fatalf("ListUserVideos with default pagination returned error: %v", err)
	}
}
