package videoapp

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/pgvector/pgvector-go"

	domainvideo "nlp-video-analysis/internal/domain/video"
)

func TestPlayVideoReturnsCachedHLSAndRepairsDoneStatus(t *testing.T) {
	repo := &playTestRepo{
		getByIDResults: []playGetByIDResult{
			{video: domainvideo.Video{ID: 5, Status: domainvideo.StatusUploaded, VideoURL: "/videos/raw/2026/05/21/lesson.mp4"}, ok: true},
			{video: domainvideo.Video{ID: 5, Status: domainvideo.StatusUploaded, VideoURL: "/videos/raw/2026/05/21/lesson.mp4"}, ok: true},
		},
	}
	statusStore := &playTestStatusStore{
		getStatus: TranscodeStatus{Status: domainvideo.StatusDone, HLSURL: "/videos/hls/2026/05/21/lesson/master.m3u8"},
		getOK:     true,
	}
	svc := NewService(repo, nil, nil, statusStore, nil, nil, nil, Paths{HLSURLPrefix: "/videos/hls"})

	playURL, video, ok, err := svc.PlayVideo(context.Background(), 5)
	if err != nil {
		t.Fatalf("PlayVideo returned error: %v", err)
	}
	if !ok {
		t.Fatal("expected ok=true")
	}
	if playURL != "/videos/hls/2026/05/21/lesson/master.m3u8" {
		t.Fatalf("unexpected play url: %q", playURL)
	}
	if repo.incrementViewCountCalls != 1 {
		t.Fatalf("expected one view count increment, got %d", repo.incrementViewCountCalls)
	}
	if repo.updatedStatus != domainvideo.StatusDone {
		t.Fatalf("expected status repair to done, got %v", repo.updatedStatus)
	}
	if video.Status != domainvideo.StatusDone {
		t.Fatalf("expected returned video status done, got %v", video.Status)
	}
}

func TestPlayVideoReturnsDerivedHLSWhenVideoAlreadyDone(t *testing.T) {
	repo := &playTestRepo{
		getByIDResults: []playGetByIDResult{
			{video: domainvideo.Video{ID: 6, Status: domainvideo.StatusDone, VideoURL: "/videos/raw/2026/05/21/lesson.mp4"}, ok: true},
			{video: domainvideo.Video{ID: 6, Status: domainvideo.StatusDone, VideoURL: "/videos/raw/2026/05/21/lesson.mp4"}, ok: true},
		},
	}
	svc := NewService(repo, nil, nil, &playTestStatusStore{}, nil, nil, nil, Paths{HLSURLPrefix: "/videos/hls"})

	playURL, video, ok, err := svc.PlayVideo(context.Background(), 6)
	if err != nil {
		t.Fatalf("PlayVideo returned error: %v", err)
	}
	if !ok {
		t.Fatal("expected ok=true")
	}
	if playURL != "/videos/hls/2026/05/21/lesson/master.m3u8" {
		t.Fatalf("unexpected play url: %q", playURL)
	}
	if video.Status != domainvideo.StatusDone {
		t.Fatalf("expected video status done, got %v", video.Status)
	}
	if repo.updatedStatus != 0 {
		t.Fatalf("expected no status repair, got %v", repo.updatedStatus)
	}
}

func TestPlayVideoDerivesHLSFromConfiguredPrefixesAndMasterName(t *testing.T) {
	repo := &playTestRepo{
		getByIDResults: []playGetByIDResult{
			{video: domainvideo.Video{ID: 6, Status: domainvideo.StatusDone, VideoURL: "/media/source/2026/05/21/lesson.mp4"}, ok: true},
			{video: domainvideo.Video{ID: 6, Status: domainvideo.StatusDone, VideoURL: "/media/source/2026/05/21/lesson.mp4"}, ok: true},
		},
	}
	svc := NewService(repo, nil, nil, &playTestStatusStore{}, nil, nil, nil, Paths{
		RawURLPrefix:  "/media/source",
		HLSURLPrefix:  "/media/stream",
		HLSMasterName: "playlist.m3u8",
	})

	playURL, _, ok, err := svc.PlayVideo(context.Background(), 6)
	if err != nil {
		t.Fatalf("PlayVideo returned error: %v", err)
	}
	if !ok {
		t.Fatal("expected ok=true")
	}
	if playURL != "/media/stream/2026/05/21/lesson/playlist.m3u8" {
		t.Fatalf("unexpected play url: %q", playURL)
	}
}

func TestPlayVideoReturnsRawURLWhenNotDone(t *testing.T) {
	repo := &playTestRepo{
		getByIDResults: []playGetByIDResult{
			{video: domainvideo.Video{ID: 7, Status: domainvideo.StatusProcessing, VideoURL: "/videos/raw/2026/05/21/lesson.mp4"}, ok: true},
			{video: domainvideo.Video{ID: 7, Status: domainvideo.StatusProcessing, VideoURL: "/videos/raw/2026/05/21/lesson.mp4"}, ok: true},
		},
	}
	svc := NewService(repo, nil, nil, &playTestStatusStore{}, nil, nil, nil, Paths{HLSURLPrefix: "/videos/hls"})

	playURL, _, ok, err := svc.PlayVideo(context.Background(), 7)
	if err != nil {
		t.Fatalf("PlayVideo returned error: %v", err)
	}
	if !ok {
		t.Fatal("expected ok=true")
	}
	if playURL != "/videos/raw/2026/05/21/lesson.mp4" {
		t.Fatalf("unexpected raw url: %q", playURL)
	}
}

func TestPlayVideoStopsWhenIncrementViewCountFails(t *testing.T) {
	repo := &playTestRepo{
		getByIDResults: []playGetByIDResult{{video: domainvideo.Video{ID: 8}, ok: true}},
		incrementErr:   errors.New("increment failed"),
	}
	statusStore := &playTestStatusStore{}
	svc := NewService(repo, nil, nil, statusStore, nil, nil, nil, Paths{})

	_, _, ok, err := svc.PlayVideo(context.Background(), 8)
	if err == nil {
		t.Fatal("expected increment error")
	}
	if ok {
		t.Fatal("expected ok=false when increment fails")
	}
	if err.Error() != "increment failed" {
		t.Fatalf("unexpected error: %v", err)
	}
	if statusStore.getCalls != 0 {
		t.Fatalf("expected status store not to be queried, got %d calls", statusStore.getCalls)
	}
	if repo.getByIDCalls != 1 {
		t.Fatalf("expected only initial GetByID call, got %d", repo.getByIDCalls)
	}
}

func TestResolvePlaybackURLPrefersCachedHLS(t *testing.T) {
	statusStore := &playTestStatusStore{
		getStatus: TranscodeStatus{Status: domainvideo.StatusDone, HLSURL: "/videos/hls/2026/05/21/cached/master.m3u8"},
		getOK:     true,
	}
	svc := NewService(nil, nil, nil, statusStore, nil, nil, nil, Paths{HLSURLPrefix: "/videos/hls"})

	playURL := svc.ResolvePlaybackURL(context.Background(), domainvideo.Video{
		ID:       9,
		Status:   domainvideo.StatusDone,
		VideoURL: "/videos/raw/2026/05/21/lesson.mp4",
	})
	if playURL != "/videos/hls/2026/05/21/cached/master.m3u8" {
		t.Fatalf("unexpected cached play url: %q", playURL)
	}
}

func TestGetViewCountReturnsInt64(t *testing.T) {
	repo := &playTestRepo{viewCount: 17, viewCountOK: true}
	svc := NewService(repo, nil, nil, nil, nil, nil, nil, Paths{})

	count, ok, err := svc.GetViewCount(context.Background(), 3)
	if err != nil {
		t.Fatalf("GetViewCount returned error: %v", err)
	}
	if !ok {
		t.Fatal("expected ok=true")
	}
	if count != 17 {
		t.Fatalf("unexpected count: %d", count)
	}
}

func TestGetTranscodeStatusPassesThroughStore(t *testing.T) {
	statusStore := &playTestStatusStore{
		getStatus: TranscodeStatus{Status: domainvideo.StatusProcessing, HLSURL: "/videos/hls/processing/master.m3u8"},
		getOK:     true,
	}
	svc := NewService(nil, nil, nil, statusStore, nil, nil, nil, Paths{})

	status, ok, err := svc.GetTranscodeStatus(context.Background(), "task-1")
	if err != nil {
		t.Fatalf("GetTranscodeStatus returned error: %v", err)
	}
	if !ok {
		t.Fatal("expected ok=true")
	}
	if status.Status != domainvideo.StatusProcessing {
		t.Fatalf("unexpected status: %v", status.Status)
	}
	if status.HLSURL != "/videos/hls/processing/master.m3u8" {
		t.Fatalf("unexpected hls url: %q", status.HLSURL)
	}
}

func TestDeriveHLSURLFromRaw(t *testing.T) {
	tests := []struct {
		name       string
		rawURL     string
		rawPrefix  string
		hlsPrefix  string
		masterName string
		want       string
	}{
		{name: "empty", rawURL: "", rawPrefix: "/videos/raw", hlsPrefix: "/videos/hls", masterName: "master.m3u8", want: ""},
		{name: "invalid", rawURL: "/videos/raw/lesson.mp4", rawPrefix: "/videos/raw", hlsPrefix: "/videos/hls", masterName: "master.m3u8", want: ""},
		{name: "valid", rawURL: "/videos/raw/2026/05/21/lesson.mp4", rawPrefix: "/videos/raw", hlsPrefix: "/videos/hls", masterName: "master.m3u8", want: "/videos/hls/2026/05/21/lesson/master.m3u8"},
		{name: "configured", rawURL: "/media/source/2026/05/21/lesson.mp4", rawPrefix: "/media/source", hlsPrefix: "/media/stream", masterName: "playlist.m3u8", want: "/media/stream/2026/05/21/lesson/playlist.m3u8"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := deriveHLSURLFromRaw(tc.rawURL, tc.rawPrefix, tc.hlsPrefix, tc.masterName); got != tc.want {
				t.Fatalf("deriveHLSURLFromRaw(%q) = %q, want %q", tc.rawURL, got, tc.want)
			}
		})
	}
}

type playGetByIDResult struct {
	video domainvideo.Video
	ok    bool
	err   error
}

type playTestRepo struct {
	getByIDResults          []playGetByIDResult
	getByIDCalls            int
	incrementViewCountCalls int
	incrementErr            error
	updatedStatus           domainvideo.Status
	viewCount               int
	viewCountOK             bool
	viewCountErr            error
}

func (*playTestRepo) Create(context.Context, *domainvideo.Video) error { panic("unexpected call") }
func (*playTestRepo) List(context.Context, ListFilter) ([]domainvideo.Video, error) {
	panic("unexpected call")
}
func (*playTestRepo) ListRecommendPool(context.Context) ([]domainvideo.Video, error) {
	panic("unexpected call")
}
func (r *playTestRepo) GetByID(context.Context, uint64) (domainvideo.Video, bool, error) {
	if r.getByIDCalls >= len(r.getByIDResults) {
		panic("unexpected GetByID call")
	}
	result := r.getByIDResults[r.getByIDCalls]
	r.getByIDCalls++
	return result.video, result.ok, result.err
}
func (*playTestRepo) DeleteByID(context.Context, uint64) (bool, error) { panic("unexpected call") }
func (*playTestRepo) UpdateMetadata(context.Context, uint64, string, string) (bool, error) {
	panic("unexpected call")
}
func (*playTestRepo) UpdatePublished(context.Context, uint64, bool) (bool, error) {
	panic("unexpected call")
}
func (*playTestRepo) UpdateRecommend(context.Context, uint64, bool, uint64, int16, float64) (bool, error) {
	panic("unexpected call")
}
func (r *playTestRepo) IncrementViewCount(context.Context, uint64) (int, bool, error) {
	r.incrementViewCountCalls++
	if r.incrementErr != nil {
		return 0, false, r.incrementErr
	}
	return 1, true, nil
}
func (r *playTestRepo) GetViewCount(context.Context, uint64) (int, bool, error) {
	return r.viewCount, r.viewCountOK, r.viewCountErr
}
func (*playTestRepo) SubmitVideoReaction(context.Context, uint64, uint64, VideoReactionType) (bool, bool, error) {
	panic("unexpected call")
}
func (*playTestRepo) ApplyVideoReactionState(context.Context, uint64, uint64, VideoReactionType, bool) (bool, error) {
	panic("unexpected call")
}
func (*playTestRepo) GetVideoUserReaction(context.Context, uint64, uint64) (VideoReactionType, bool, bool, error) {
	panic("unexpected call")
}
func (*playTestRepo) GetVideoReactionCounts(context.Context, uint64) (VideoReactionCounts, bool, error) {
	panic("unexpected call")
}
func (*playTestRepo) FindSimilar(context.Context, uint64, int) ([]domainvideo.Video, error) {
	panic("unexpected call")
}
func (*playTestRepo) UpdateCoverByID(context.Context, uint64, string) (bool, error) {
	panic("unexpected call")
}
func (r *playTestRepo) UpdateStatusByID(context.Context, uint64, domainvideo.Status, string) error {
	r.updatedStatus = domainvideo.StatusDone
	return nil
}
func (*playTestRepo) GetSegmentEmbeddingDim(context.Context) (int, error) { panic("unexpected call") }
func (*playTestRepo) GetQuestionEmbeddingTextByID(context.Context, uint64) (string, error) {
	panic("unexpected call")
}
func (*playTestRepo) ListQuestions(context.Context, int, int) (QuestionPage, error) {
	panic("unexpected call")
}
func (*playTestRepo) GetQuestionByID(context.Context, uint64) (QuestionItem, bool, error) {
	panic("unexpected call")
}
func (*playTestRepo) FindRecommendedSegments(context.Context, pgvector.Vector, int) ([]RecommendCandidate, error) {
	panic("unexpected call")
}
func (*playTestRepo) FindRecommendedSegmentsByWeakKnowledge(context.Context, uint64, int, int) ([]RecommendCandidate, error) {
	return nil, nil
}
func (*playTestRepo) SaveUserVideoRecommendation(context.Context, uint64, uint64, uint64, uint64, float64, time.Time) error {
	panic("unexpected call")
}
func (*playTestRepo) ListRecommendations(context.Context, uint64, uint64, int) ([]RecommendationRecord, error) {
	panic("unexpected call")
}
func (*playTestRepo) GetVideoIDBySegmentID(context.Context, uint64) (uint64, error) {
	panic("unexpected call")
}
func (*playTestRepo) HasWatchedVideoForQuestion(context.Context, uint64, uint64, uint64) (bool, error) {
	panic("unexpected call")
}
func (*playTestRepo) SaveWatchRecord(context.Context, uint64, uint64, uint64, uint64, bool, int, time.Time) (bool, error) {
	panic("unexpected call")
}

type playTestStatusStore struct {
	getStatus TranscodeStatus
	getOK     bool
	getErr    error
	getCalls  int
}

func (*playTestStatusStore) Set(context.Context, string, domainvideo.Status, string, time.Duration) error {
	panic("unexpected call")
}

func (s *playTestStatusStore) Get(context.Context, string) (TranscodeStatus, bool, error) {
	s.getCalls++
	return s.getStatus, s.getOK, s.getErr
}
