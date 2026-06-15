package videoapp

import (
	"context"
	"testing"
	"time"

	"github.com/pgvector/pgvector-go"

	domainvideo "nlp-video-analysis/internal/domain/video"
)

type stubVideoRepository struct {
	listRecommendationsFunc          func(context.Context, uint64, uint64, int) ([]RecommendationRecord, error)
	getSegmentEmbeddingDimFunc       func(context.Context) (int, error)
	getQuestionEmbeddingTextByIDFunc func(context.Context, uint64) (string, error)
	findRecommendedSegmentsFunc      func(context.Context, pgvector.Vector, int) ([]RecommendCandidate, error)
	findRandomPlayableSegmentFunc    func(context.Context) (RecommendResultItem, bool, error)
	saveUserVideoRecommendationFunc  func(context.Context, uint64, uint64, uint64, uint64, float64, time.Time) error
	getVideoIDBySegmentIDFunc        func(context.Context, uint64) (uint64, error)
	hasWatchedVideoForQuestionFunc   func(context.Context, uint64, uint64, uint64) (bool, error)
	incrementViewCountFunc           func(context.Context, uint64) (int, bool, error)
	saveWatchRecordFunc              func(context.Context, uint64, uint64, uint64, uint64, bool, int, time.Time) (bool, error)
}

func (s *stubVideoRepository) Create(context.Context, *domainvideo.Video) error {
	panic("unexpected call")
}
func (s *stubVideoRepository) List(context.Context, ListFilter) ([]domainvideo.Video, error) {
	panic("unexpected call")
}
func (s *stubVideoRepository) ListRecommendPool(context.Context) ([]domainvideo.Video, error) {
	panic("unexpected call")
}
func (s *stubVideoRepository) GetByID(context.Context, uint64) (domainvideo.Video, bool, error) {
	panic("unexpected call")
}
func (s *stubVideoRepository) DeleteByID(context.Context, uint64) (bool, error) {
	panic("unexpected call")
}
func (s *stubVideoRepository) UpdateMetadata(context.Context, uint64, string, string) (bool, error) {
	panic("unexpected call")
}
func (s *stubVideoRepository) UpdatePublished(context.Context, uint64, bool) (bool, error) {
	panic("unexpected call")
}
func (s *stubVideoRepository) UpdateRecommend(context.Context, uint64, bool, uint64, int16, float64) (bool, error) {
	panic("unexpected call")
}
func (s *stubVideoRepository) IncrementViewCount(ctx context.Context, videoID uint64) (int, bool, error) {
	if s.incrementViewCountFunc != nil {
		return s.incrementViewCountFunc(ctx, videoID)
	}
	panic("unexpected call")
}
func (s *stubVideoRepository) GetViewCount(context.Context, uint64) (int, bool, error) {
	panic("unexpected call")
}
func (s *stubVideoRepository) SubmitVideoReaction(context.Context, uint64, uint64, VideoReactionType) (bool, bool, error) {
	panic("unexpected call")
}
func (s *stubVideoRepository) ApplyVideoReactionState(context.Context, uint64, uint64, VideoReactionType, bool) (bool, error) {
	panic("unexpected call")
}
func (s *stubVideoRepository) GetVideoUserReaction(context.Context, uint64, uint64) (VideoReactionType, bool, bool, error) {
	panic("unexpected call")
}
func (s *stubVideoRepository) GetVideoReactionCounts(context.Context, uint64) (VideoReactionCounts, bool, error) {
	panic("unexpected call")
}
func (s *stubVideoRepository) FindSimilar(context.Context, uint64, int) ([]domainvideo.Video, error) {
	panic("unexpected call")
}
func (s *stubVideoRepository) UpdateCoverByID(context.Context, uint64, string) (bool, error) {
	panic("unexpected call")
}
func (s *stubVideoRepository) UpdateStatusByID(context.Context, uint64, domainvideo.Status, string) error {
	panic("unexpected call")
}
func (s *stubVideoRepository) GetSegmentEmbeddingDim(ctx context.Context) (int, error) {
	if s.getSegmentEmbeddingDimFunc != nil {
		return s.getSegmentEmbeddingDimFunc(ctx)
	}
	panic("unexpected call")
}
func (s *stubVideoRepository) GetQuestionEmbeddingTextByID(ctx context.Context, questionID uint64) (string, error) {
	if s.getQuestionEmbeddingTextByIDFunc != nil {
		return s.getQuestionEmbeddingTextByIDFunc(ctx, questionID)
	}
	panic("unexpected call")
}
func (s *stubVideoRepository) ListQuestions(context.Context, int, int) (QuestionPage, error) {
	panic("unexpected call")
}
func (s *stubVideoRepository) GetQuestionByID(context.Context, uint64) (QuestionItem, bool, error) {
	panic("unexpected call")
}
func (s *stubVideoRepository) FindRecommendedSegments(ctx context.Context, query pgvector.Vector, limit int) ([]RecommendCandidate, error) {
	if s.findRecommendedSegmentsFunc != nil {
		return s.findRecommendedSegmentsFunc(ctx, query, limit)
	}
	panic("unexpected call")
}
func (s *stubVideoRepository) FindRandomPlayableSegment(ctx context.Context) (RecommendResultItem, bool, error) {
	if s.findRandomPlayableSegmentFunc != nil {
		return s.findRandomPlayableSegmentFunc(ctx)
	}
	panic("unexpected call")
}
func (s *stubVideoRepository) SaveUserVideoRecommendation(ctx context.Context, userID uint64, questionID uint64, videoID uint64, segmentID uint64, score float64, now time.Time) error {
	if s.saveUserVideoRecommendationFunc != nil {
		return s.saveUserVideoRecommendationFunc(ctx, userID, questionID, videoID, segmentID, score, now)
	}
	panic("unexpected call")
}
func (s *stubVideoRepository) ListRecommendations(ctx context.Context, userID uint64, questionID uint64, limit int) ([]RecommendationRecord, error) {
	if s.listRecommendationsFunc != nil {
		return s.listRecommendationsFunc(ctx, userID, questionID, limit)
	}
	panic("unexpected call")
}
func (s *stubVideoRepository) GetVideoIDBySegmentID(ctx context.Context, segmentID uint64) (uint64, error) {
	if s.getVideoIDBySegmentIDFunc != nil {
		return s.getVideoIDBySegmentIDFunc(ctx, segmentID)
	}
	panic("unexpected call")
}
func (s *stubVideoRepository) HasWatchedVideoForQuestion(ctx context.Context, userID uint64, questionID uint64, videoID uint64) (bool, error) {
	if s.hasWatchedVideoForQuestionFunc != nil {
		return s.hasWatchedVideoForQuestionFunc(ctx, userID, questionID, videoID)
	}
	panic("unexpected call")
}
func (s *stubVideoRepository) SaveWatchRecord(ctx context.Context, userID uint64, videoID uint64, questionID uint64, segmentID uint64, isWatched bool, watchDuration int, now time.Time) (bool, error) {
	if s.saveWatchRecordFunc != nil {
		return s.saveWatchRecordFunc(ctx, userID, videoID, questionID, segmentID, isWatched, watchDuration, now)
	}
	panic("unexpected call")
}

func TestRandomPlayVideoSegmentReturnsRepositoryResult(t *testing.T) {
	svc := NewService(&stubVideoRepository{
		findRandomPlayableSegmentFunc: func(context.Context) (RecommendResultItem, bool, error) {
			return RecommendResultItem{
				VideoID:        11,
				VideoSegmentID: 101,
				StartTimeSec:   10,
				EndTimeSec:     40,
				TitleOverride:  "segment title",
				Video: domainvideo.Video{
					ID:          11,
					Title:       "video title",
					VideoURL:    "/videos/raw/2026/06/09/playable.mp4",
					CoverURL:    "/covers/11.jpg",
					Status:      domainvideo.StatusDone,
					IsPublished: true,
				},
			}, true, nil
		},
	}, nil, nil, nil, nil, nil, nil, Paths{})

	item, found, err := svc.RandomPlayVideoSegment(context.Background())
	if err != nil {
		t.Fatalf("RandomPlayVideoSegment returned error: %v", err)
	}
	if !found {
		t.Fatal("expected found=true")
	}
	if item.VideoSegmentID != 101 || item.VideoID != 11 {
		t.Fatalf("unexpected item: %+v", item)
	}
	if item.TitleOverride != "segment title" {
		t.Fatalf("unexpected title override: %q", item.TitleOverride)
	}
}

func TestRandomPlayVideoSegmentReportsMissingWhenRepositoryDoesNotSupportIt(t *testing.T) {
	svc := &Service{}

	_, found, err := svc.RandomPlayVideoSegment(context.Background())
	if err != nil {
		t.Fatalf("RandomPlayVideoSegment returned error: %v", err)
	}
	if found {
		t.Fatal("expected found=false")
	}
}

func TestListRecommendations_PreservesSegmentTiming(t *testing.T) {
	svc := NewService(&stubVideoRepository{
		listRecommendationsFunc: func(_ context.Context, userID uint64, questionID uint64, limit int) ([]RecommendationRecord, error) {
			if userID != 7 || questionID != 3 || limit != 5 {
				t.Fatalf("unexpected query inputs: userID=%d questionID=%d limit=%d", userID, questionID, limit)
			}
			return []RecommendationRecord{{
				QuestionID:     3,
				VideoID:        17,
				VideoSegmentID: 204,
				RecommendScore: 0.88,
				IsWatched:      true,
				WatchDuration:  45,
				StartTimeSec:   120,
				EndTimeSec:     165,
				Status:         int16(domainvideo.StatusDone),
				VideoURL:       "/videos/raw/2026/04/28/history.mp4",
				Title:          "History",
			}}, nil
		},
	}, nil, nil, nil, nil, nil, nil, Paths{})

	items, err := svc.ListRecommendations(context.Background(), ListRecommendationsInput{UserID: 7, QuestionID: 3, Limit: 5})
	if err != nil {
		t.Fatalf("list recommendations: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].StartTimeSec != 120 {
		t.Fatalf("expected start_time_sec 120, got %d", items[0].StartTimeSec)
	}
	if items[0].EndTimeSec != 165 {
		t.Fatalf("expected end_time_sec 165, got %d", items[0].EndTimeSec)
	}
	if items[0].Video.Status != domainvideo.StatusDone {
		t.Fatalf("expected done status, got %v", items[0].Video.Status)
	}
}

func TestRecommendByQuestion_PersistsTextOnlyHistory(t *testing.T) {
	saveCalls := 0
	var savedQuestionID uint64
	svc := NewService(&stubVideoRepository{
		getSegmentEmbeddingDimFunc: func(context.Context) (int, error) {
			return 3, nil
		},
		findRecommendedSegmentsFunc: func(_ context.Context, query pgvector.Vector, limit int) ([]RecommendCandidate, error) {
			return []RecommendCandidate{{
				VideoID:        11,
				VideoSegmentID: 22,
				StartTimeSec:   3,
				EndTimeSec:     9,
				Distance:       0.2,
				Status:         int16(domainvideo.StatusDone),
				VideoURL:       "/videos/raw/2026/04/28/free-text.mp4",
			}}, nil
		},
		saveUserVideoRecommendationFunc: func(_ context.Context, _ uint64, questionID uint64, _ uint64, _ uint64, _ float64, _ time.Time) error {
			saveCalls++
			savedQuestionID = questionID
			return nil
		},
	}, nil, nil, nil, nil, nil, stubEmbedder{vector: []float32{1, 2, 3}}, Paths{})

	items, err := svc.RecommendByQuestion(context.Background(), RecommendByQuestionInput{QuestionText: "free text", UserID: 7, Limit: 3})
	if err != nil {
		t.Fatalf("recommend by question: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if saveCalls != 1 {
		t.Fatalf("expected persistence for text-only recommendation history, got %d save calls", saveCalls)
	}
	if savedQuestionID != 0 {
		t.Fatalf("expected free-text recommendation history to persist with questionID=0, got %d", savedQuestionID)
	}
	if items[0].Video.Status != domainvideo.StatusDone {
		t.Fatalf("expected done status, got %v", items[0].Video.Status)
	}
}

func TestRecommendByQuestion_PersistsQuestionIDHistory(t *testing.T) {
	saveCalls := 0
	var savedQuestionID uint64
	svc := NewService(&stubVideoRepository{
		getSegmentEmbeddingDimFunc: func(context.Context) (int, error) {
			return 3, nil
		},
		findRecommendedSegmentsFunc: func(_ context.Context, query pgvector.Vector, limit int) ([]RecommendCandidate, error) {
			return []RecommendCandidate{{
				VideoID:        21,
				VideoSegmentID: 31,
				StartTimeSec:   10,
				EndTimeSec:     18,
				Distance:       0.1,
				Status:         int16(domainvideo.StatusDone),
				VideoURL:       "/videos/raw/2026/04/28/by-id.mp4",
			}}, nil
		},
		getQuestionEmbeddingTextByIDFunc: func(context.Context, uint64) (string, error) {
			return "[1,2,3]", nil
		},
		saveUserVideoRecommendationFunc: func(_ context.Context, _ uint64, questionID uint64, _ uint64, _ uint64, _ float64, _ time.Time) error {
			saveCalls++
			savedQuestionID = questionID
			return nil
		},
	}, nil, nil, nil, nil, nil, nil, Paths{})

	items, err := svc.RecommendByQuestion(context.Background(), RecommendByQuestionInput{QuestionID: 99, UserID: 7, Limit: 3})
	if err != nil {
		t.Fatalf("recommend by question id: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if saveCalls != 1 {
		t.Fatalf("expected one persisted record for question-id recommendation, got %d", saveCalls)
	}
	if savedQuestionID != 99 {
		t.Fatalf("expected questionID=99 to be persisted, got %d", savedQuestionID)
	}
}

func TestResolvePlaybackURL_DerivesHLSFromDoneRecommendationVideoState(t *testing.T) {
	svc := NewService(nil, nil, nil, stubStatusStore{}, nil, nil, nil, Paths{HLSURLPrefix: "/videos/hls"})

	playURL := svc.ResolvePlaybackURL(context.Background(), domainvideo.Video{
		ID:       99,
		Status:   domainvideo.StatusDone,
		VideoURL: "/videos/raw/2026/04/28/algebra.mp4",
	})

	if playURL != "/videos/hls/2026/04/28/algebra/master.m3u8" {
		t.Fatalf("unexpected play url: %q", playURL)
	}
}

func TestReportWatch_IncrementsParentVideoViewCount(t *testing.T) {
	var incrementedVideoID uint64
	var savedVideoID uint64
	svc := NewService(&stubVideoRepository{
		getVideoIDBySegmentIDFunc: func(_ context.Context, segmentID uint64) (uint64, error) {
			if segmentID != 204 {
				t.Fatalf("unexpected segmentID: %d", segmentID)
			}
			return 17, nil
		},
		hasWatchedVideoForQuestionFunc: func(_ context.Context, userID uint64, questionID uint64, videoID uint64) (bool, error) {
			if userID != 7 || questionID != 3 || videoID != 17 {
				t.Fatalf("unexpected existence inputs: userID=%d questionID=%d videoID=%d", userID, questionID, videoID)
			}
			return false, nil
		},
		incrementViewCountFunc: func(_ context.Context, videoID uint64) (int, bool, error) {
			incrementedVideoID = videoID
			return 6, true, nil
		},
		saveWatchRecordFunc: func(_ context.Context, userID uint64, videoID uint64, questionID uint64, segmentID uint64, isWatched bool, watchDuration int, _ time.Time) (bool, error) {
			if userID != 7 || questionID != 3 || segmentID != 204 || !isWatched || watchDuration != 45 {
				t.Fatalf("unexpected watch record inputs: userID=%d questionID=%d segmentID=%d isWatched=%v watchDuration=%d", userID, questionID, segmentID, isWatched, watchDuration)
			}
			savedVideoID = videoID
			return true, nil
		},
	}, nil, nil, nil, nil, nil, nil, Paths{})

	err := svc.ReportWatch(context.Background(), ReportWatchInput{QuestionID: 3, UserID: 7, VideoSegmentID: 204, IsWatched: true, WatchDuration: 45})
	if err != nil {
		t.Fatalf("report watch: %v", err)
	}
	if incrementedVideoID != 17 {
		t.Fatalf("expected incremented videoID 17, got %d", incrementedVideoID)
	}
	if savedVideoID != 17 {
		t.Fatalf("expected saved watch record videoID 17, got %d", savedVideoID)
	}
}

func TestReportWatch_DoesNotIncrementViewCountForExistingWatchRecord(t *testing.T) {
	incrementCalls := 0
	svc := NewService(&stubVideoRepository{
		getVideoIDBySegmentIDFunc: func(_ context.Context, segmentID uint64) (uint64, error) {
			if segmentID != 204 {
				t.Fatalf("unexpected segmentID: %d", segmentID)
			}
			return 17, nil
		},
		hasWatchedVideoForQuestionFunc: func(_ context.Context, userID uint64, questionID uint64, videoID uint64) (bool, error) {
			if userID != 7 || questionID != 3 || videoID != 17 {
				t.Fatalf("unexpected existence inputs: userID=%d questionID=%d videoID=%d", userID, questionID, videoID)
			}
			return true, nil
		},
		incrementViewCountFunc: func(_ context.Context, videoID uint64) (int, bool, error) {
			incrementCalls++
			return 6, true, nil
		},
		saveWatchRecordFunc: func(_ context.Context, userID uint64, videoID uint64, questionID uint64, segmentID uint64, isWatched bool, watchDuration int, _ time.Time) (bool, error) {
			if userID != 7 || videoID != 17 || questionID != 3 || segmentID != 204 || !isWatched || watchDuration != 45 {
				t.Fatalf("unexpected watch record inputs: userID=%d videoID=%d questionID=%d segmentID=%d isWatched=%v watchDuration=%d", userID, videoID, questionID, segmentID, isWatched, watchDuration)
			}
			return false, nil
		},
	}, nil, nil, nil, nil, nil, nil, Paths{})

	err := svc.ReportWatch(context.Background(), ReportWatchInput{QuestionID: 3, UserID: 7, VideoSegmentID: 204, IsWatched: true, WatchDuration: 45})
	if err != nil {
		t.Fatalf("report watch: %v", err)
	}
	if incrementCalls != 0 {
		t.Fatalf("expected no view count increment for existing watch record, got %d calls", incrementCalls)
	}
}

func TestReportWatch_DoesNotIncrementViewCountForDifferentSegmentOfSameVideo(t *testing.T) {
	incrementCalls := 0
	svc := NewService(&stubVideoRepository{
		getVideoIDBySegmentIDFunc: func(_ context.Context, segmentID uint64) (uint64, error) {
			if segmentID != 205 {
				t.Fatalf("unexpected segmentID: %d", segmentID)
			}
			return 17, nil
		},
		hasWatchedVideoForQuestionFunc: func(_ context.Context, userID uint64, questionID uint64, videoID uint64) (bool, error) {
			if userID != 7 || questionID != 3 || videoID != 17 {
				t.Fatalf("unexpected existence inputs: userID=%d questionID=%d videoID=%d", userID, questionID, videoID)
			}
			return true, nil
		},
		incrementViewCountFunc: func(_ context.Context, videoID uint64) (int, bool, error) {
			incrementCalls++
			return 6, true, nil
		},
		saveWatchRecordFunc: func(_ context.Context, userID uint64, videoID uint64, questionID uint64, segmentID uint64, isWatched bool, watchDuration int, _ time.Time) (bool, error) {
			if userID != 7 || videoID != 17 || questionID != 3 || segmentID != 205 || !isWatched || watchDuration != 45 {
				t.Fatalf("unexpected watch record inputs: userID=%d videoID=%d questionID=%d segmentID=%d isWatched=%v watchDuration=%d", userID, videoID, questionID, segmentID, isWatched, watchDuration)
			}
			return true, nil
		},
	}, nil, nil, nil, nil, nil, nil, Paths{})

	err := svc.ReportWatch(context.Background(), ReportWatchInput{QuestionID: 3, UserID: 7, VideoSegmentID: 205, IsWatched: true, WatchDuration: 45})
	if err != nil {
		t.Fatalf("report watch: %v", err)
	}
	if incrementCalls != 0 {
		t.Fatalf("expected no view count increment for different segment of same video, got %d calls", incrementCalls)
	}
}

type stubEmbedder struct {
	vector []float32
	err    error
}

func (s stubEmbedder) Embed(context.Context, string) ([]float32, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.vector, nil
}

type stubStatusStore struct{}

func (stubStatusStore) Set(context.Context, string, domainvideo.Status, string, time.Duration) error {
	panic("unexpected call")
}

func (stubStatusStore) Get(context.Context, string) (TranscodeStatus, bool, error) {
	return TranscodeStatus{}, false, nil
}
