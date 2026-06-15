package playback

import (
	"context"
	"errors"
	"testing"

	domainvideo "nlp-video-analysis/internal/domain/video"
)

func TestPlayVideoPrefersCachedDoneHLSAndRepairsStatus(t *testing.T) {
	repo := &fakeVideoRepository{
		videos: []repoVideoResult{
			{video: domainvideo.Video{ID: 7, Status: domainvideo.StatusUploaded, VideoURL: "/videos/raw/2026/05/21/lesson.mp4"}, ok: true},
			{video: domainvideo.Video{ID: 7, Status: domainvideo.StatusUploaded, VideoURL: "/videos/raw/2026/05/21/lesson.mp4"}, ok: true},
		},
	}
	statusStore := &fakeStatusStore{
		status: Status{Status: domainvideo.StatusDone, HLSURL: "/videos/hls/2026/05/21/lesson/master.m3u8"},
		ok:     true,
	}
	svc := Service{Repo: repo, StatusStore: statusStore}

	playURL, video, ok, err := svc.PlayVideo(context.Background(), 7)
	if err != nil {
		t.Fatalf("PlayVideo returned error: %v", err)
	}
	if !ok {
		t.Fatal("ok = false, want true")
	}
	if playURL != "/videos/hls/2026/05/21/lesson/master.m3u8" {
		t.Fatalf("playURL = %q", playURL)
	}
	if repo.incrementCalls != 1 {
		t.Fatalf("increment calls = %d, want 1", repo.incrementCalls)
	}
	if repo.updatedStatus != domainvideo.StatusDone {
		t.Fatalf("updated status = %v, want done", repo.updatedStatus)
	}
	if video.Status != domainvideo.StatusDone {
		t.Fatalf("returned status = %v, want done", video.Status)
	}
}

func TestPlayVideoReturnsDerivedHLSForDoneVideo(t *testing.T) {
	repo := &fakeVideoRepository{
		videos: []repoVideoResult{
			{video: domainvideo.Video{ID: 8, Status: domainvideo.StatusDone, VideoURL: "/media/source/2026/05/21/lesson.mp4"}, ok: true},
			{video: domainvideo.Video{ID: 8, Status: domainvideo.StatusDone, VideoURL: "/media/source/2026/05/21/lesson.mp4"}, ok: true},
		},
	}
	svc := Service{
		Repo:          repo,
		StatusStore:   &fakeStatusStore{},
		RawURLPrefix:  "/media/source",
		HLSURLPrefix:  "/media/stream",
		HLSMasterName: "playlist.m3u8",
	}

	playURL, _, ok, err := svc.PlayVideo(context.Background(), 8)
	if err != nil {
		t.Fatalf("PlayVideo returned error: %v", err)
	}
	if !ok {
		t.Fatal("ok = false, want true")
	}
	if playURL != "/media/stream/2026/05/21/lesson/playlist.m3u8" {
		t.Fatalf("playURL = %q", playURL)
	}
	if repo.updateCalls != 0 {
		t.Fatalf("status update calls = %d, want 0", repo.updateCalls)
	}
}

func TestPlayVideoStopsWhenIncrementFails(t *testing.T) {
	errIncrement := errors.New("increment failed")
	repo := &fakeVideoRepository{
		videos:       []repoVideoResult{{video: domainvideo.Video{ID: 9}, ok: true}},
		incrementErr: errIncrement,
	}
	statusStore := &fakeStatusStore{}
	svc := Service{Repo: repo, StatusStore: statusStore}

	_, _, ok, err := svc.PlayVideo(context.Background(), 9)
	if !errors.Is(err, errIncrement) {
		t.Fatalf("err = %v, want %v", err, errIncrement)
	}
	if ok {
		t.Fatal("ok = true, want false")
	}
	if repo.getCalls != 1 {
		t.Fatalf("get calls = %d, want 1", repo.getCalls)
	}
	if statusStore.getCalls != 0 {
		t.Fatalf("status get calls = %d, want 0", statusStore.getCalls)
	}
}

func TestResolvePlaybackURLFallsBackWhenCachedHLSIsBlank(t *testing.T) {
	statusStore := &fakeStatusStore{
		status: Status{Status: domainvideo.StatusDone, HLSURL: "   "},
		ok:     true,
	}
	svc := Service{StatusStore: statusStore, HLSURLPrefix: "/videos/hls"}

	playURL := svc.ResolvePlaybackURL(context.Background(), domainvideo.Video{
		ID:       10,
		Status:   domainvideo.StatusDone,
		VideoURL: "/videos/raw/2026/05/21/lesson.mp4",
	})

	if playURL != "/videos/hls/2026/05/21/lesson/master.m3u8" {
		t.Fatalf("playURL = %q", playURL)
	}
}

func TestGetViewCountConvertsRepositoryValue(t *testing.T) {
	repo := &fakeVideoRepository{viewCount: 17, viewCountOK: true}
	svc := Service{Repo: repo}

	count, ok, err := svc.GetViewCount(context.Background(), 11)
	if err != nil {
		t.Fatalf("GetViewCount returned error: %v", err)
	}
	if !ok {
		t.Fatal("ok = false, want true")
	}
	if count != 17 {
		t.Fatalf("count = %d, want 17", count)
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
		{name: "invalid path", rawURL: "/videos/raw/lesson.mp4", rawPrefix: "/videos/raw", hlsPrefix: "/videos/hls", masterName: "master.m3u8", want: ""},
		{name: "default prefixes", rawURL: "/videos/raw/2026/05/21/lesson.mp4", rawPrefix: "", hlsPrefix: "", masterName: "", want: "/videos/hls/2026/05/21/lesson/master.m3u8"},
		{name: "configured prefixes", rawURL: "/media/source/2026/05/21/lesson.mp4", rawPrefix: "/media/source", hlsPrefix: "/media/stream", masterName: "playlist.m3u8", want: "/media/stream/2026/05/21/lesson/playlist.m3u8"},
		{name: "legacy raw segment", rawURL: "/videos/raw/2026/05/21/lesson.mov", rawPrefix: "/other", hlsPrefix: "/videos/hls/", masterName: "master.m3u8", want: "/videos/hls/2026/05/21/lesson/master.m3u8"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := deriveHLSURLFromRaw(tc.rawURL, tc.rawPrefix, tc.hlsPrefix, tc.masterName)
			if got != tc.want {
				t.Fatalf("deriveHLSURLFromRaw() = %q, want %q", got, tc.want)
			}
		})
	}
}

type repoVideoResult struct {
	video domainvideo.Video
	ok    bool
	err   error
}

type fakeVideoRepository struct {
	videos         []repoVideoResult
	getCalls       int
	incrementCalls int
	incrementErr   error
	updateCalls    int
	updatedStatus  domainvideo.Status
	viewCount      int
	viewCountOK    bool
}

func (r *fakeVideoRepository) GetByID(context.Context, uint64) (domainvideo.Video, bool, error) {
	if r.getCalls >= len(r.videos) {
		panic("unexpected GetByID call")
	}
	result := r.videos[r.getCalls]
	r.getCalls++
	return result.video, result.ok, result.err
}

func (r *fakeVideoRepository) IncrementViewCount(context.Context, uint64) (int, bool, error) {
	r.incrementCalls++
	if r.incrementErr != nil {
		return 0, false, r.incrementErr
	}
	return 1, true, nil
}

func (r *fakeVideoRepository) UpdateStatusByID(_ context.Context, _ uint64, status domainvideo.Status, _ string) error {
	r.updateCalls++
	r.updatedStatus = status
	return nil
}

func (r *fakeVideoRepository) GetViewCount(context.Context, uint64) (int, bool, error) {
	return r.viewCount, r.viewCountOK, nil
}

type fakeStatusStore struct {
	status   Status
	ok       bool
	err      error
	getCalls int
}

func (s *fakeStatusStore) Get(context.Context, string) (Status, bool, error) {
	s.getCalls++
	return s.status, s.ok, s.err
}
