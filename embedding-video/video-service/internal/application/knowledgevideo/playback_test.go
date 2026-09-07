package knowledgevideo

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestPlaybackListRejectsInvalidKnowledgePointID(t *testing.T) {
	_, err := (PlaybackService{Repo: &playbackRepoStub{}}).List(context.Background(), 0)
	var invalid *InvalidArgument
	if !errors.As(err, &invalid) || invalid.Field != "knowledge_point_id" {
		t.Fatalf("List() error = %v", err)
	}
}

func TestPlaybackListReturnsNotFoundForMissingKnowledgePoint(t *testing.T) {
	_, err := (PlaybackService{Repo: &playbackRepoStub{}}).List(context.Background(), 9)
	var notFound *NotFound
	if !errors.As(err, &notFound) {
		t.Fatalf("List() error = %v", err)
	}
}

func TestPlaybackListReturnsAllReadyVideosWithoutRecording(t *testing.T) {
	repo := &playbackRepoStub{
		point: DictionaryKnowledgePoint{ID: 9, Name: "一次函数"},
		readyVideos: []Video{
			{ID: 41, KnowledgePointID: 9, SourceFileName: "一次函数解析式.mp4", HLSMasterObjectKey: "hls/41/master.m3u8", Duration: 42, Status: VideoReady},
			{ID: 43, KnowledgePointID: 9, SourceFileName: "一次函数解析式.mp4.mp4", HLSMasterObjectKey: "hls/43/master.m3u8", Duration: 84, Status: VideoReady},
		},
	}
	got, err := (PlaybackService{Repo: repo, MediaRoutePrefix: "/knowledge-video-media"}).List(context.Background(), 9)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if got.KnowledgePointName != "一次函数" || len(got.Videos) != 2 {
		t.Fatalf("List() = %+v", got)
	}
	if got.Videos[0].DisplayName != "一次函数解析式" || got.Videos[1].DisplayName != "一次函数解析式.mp4" {
		t.Fatalf("display names = %+v", got.Videos)
	}
	if got.Videos[1].PlaybackURL != "/knowledge-video-media/hls/43/master.m3u8" {
		t.Fatalf("playback URL = %q", got.Videos[1].PlaybackURL)
	}
	if len(repo.records) != 0 {
		t.Fatalf("List() wrote playback records: %+v", repo.records)
	}
}

func TestPlaybackListReturnsEmptyVideosWhenNoneAreReady(t *testing.T) {
	repo := &playbackRepoStub{point: DictionaryKnowledgePoint{ID: 9, Name: "一次函数"}}
	got, err := (PlaybackService{Repo: repo}).List(context.Background(), 9)
	if err != nil || got.Videos == nil || len(got.Videos) != 0 {
		t.Fatalf("List() = %+v, %v", got, err)
	}
}

func TestPlaybackRecordValidatesVideoAndRecordsItsKnowledgePoint(t *testing.T) {
	now := time.Date(2026, 7, 27, 12, 0, 0, 0, time.UTC)
	repo := &playbackRepoStub{video: Video{ID: 88, KnowledgePointID: 9, Status: VideoReady}, videoFound: true}
	err := (PlaybackService{Repo: repo, Now: func() time.Time { return now }}).Record(context.Background(), 7, 88)
	if err != nil {
		t.Fatalf("Record() error = %v", err)
	}
	if len(repo.records) != 1 || repo.records[0] != (PlayRecord{UserID: 7, KnowledgePointID: 9, KnowledgeVideoID: 88, CreateTime: now}) {
		t.Fatalf("records = %+v", repo.records)
	}
}

func TestPlaybackRecordRejectsInvalidMissingAndNonReadyVideo(t *testing.T) {
	tests := []struct {
		name    string
		userID  uint64
		videoID uint64
		repo    *playbackRepoStub
		assert  func(error) bool
	}{
		{name: "invalid user", videoID: 88, repo: &playbackRepoStub{}, assert: func(err error) bool {
			var target *InvalidArgument
			return errors.As(err, &target) && target.Field == "user_id"
		}},
		{name: "invalid video", userID: 7, repo: &playbackRepoStub{}, assert: func(err error) bool {
			var target *InvalidArgument
			return errors.As(err, &target) && target.Field == "knowledge_video_id"
		}},
		{name: "missing", userID: 7, videoID: 88, repo: &playbackRepoStub{}, assert: func(err error) bool { var target *NotFound; return errors.As(err, &target) }},
		{name: "pending", userID: 7, videoID: 88, repo: &playbackRepoStub{video: Video{ID: 88, Status: VideoPending}, videoFound: true}, assert: func(err error) bool {
			var target *NotReady
			return errors.As(err, &target) && target.Status == VideoPending
		}},
		{name: "failed", userID: 7, videoID: 88, repo: &playbackRepoStub{video: Video{ID: 88, Status: VideoFailed}, videoFound: true}, assert: func(err error) bool {
			var target *NotReady
			return errors.As(err, &target) && target.Status == VideoFailed
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := (PlaybackService{Repo: tt.repo}).Record(context.Background(), tt.userID, tt.videoID)
			if !tt.assert(err) {
				t.Fatalf("Record() error = %v", err)
			}
		})
	}
}

type playbackRepoStub struct {
	point       DictionaryKnowledgePoint
	readyVideos []Video
	video       Video
	videoFound  bool
	records     []PlayRecord
	recordErr   error
}

func (r *playbackRepoStub) LookupKnowledgePoints(context.Context, []uint64) ([]DictionaryKnowledgePoint, error) {
	if r.point.ID == 0 {
		return []DictionaryKnowledgePoint{}, nil
	}
	return []DictionaryKnowledgePoint{r.point}, nil
}

func (r *playbackRepoStub) ListReadyByKnowledgePoint(context.Context, uint64) ([]Video, error) {
	return r.readyVideos, nil
}

func (r *playbackRepoStub) GetVideo(context.Context, uint64) (Video, bool, error) {
	return r.video, r.videoFound, nil
}

func (r *playbackRepoStub) RecordPlayback(_ context.Context, record PlayRecord) error {
	if r.recordErr != nil {
		return r.recordErr
	}
	r.records = append(r.records, record)
	return nil
}
