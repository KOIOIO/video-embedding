package videoapp

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/pgvector/pgvector-go"

	recommendationapp "nlp-video-analysis/internal/application/videoapp/recommendation"
	domainvideo "nlp-video-analysis/internal/domain/video"
)

type stubVideoRepository struct {
	listRecommendationsFunc          func(context.Context, uint64, uint64, int) ([]RecommendationRecord, error)
	getSegmentEmbeddingDimFunc       func(context.Context) (int, error)
	getQuestionEmbeddingTextByIDFunc func(context.Context, uint64) (string, error)
	findRecommendedSegmentsFunc      func(context.Context, pgvector.Vector, int) ([]RecommendCandidate, error)
	findWeakKnowledgeSegmentsFunc    func(context.Context, uint64, int, int) ([]RecommendCandidate, error)
	listWeakKnowledgeFunc            func(context.Context, uint64, int) ([]WeakKnowledge, error)
	findWeakKnowledgeVectorFunc      func(context.Context, uint64, pgvector.Vector, int) ([]RecommendCandidate, error)
	findRandomPlayableSegmentFunc    func(context.Context) (RecommendResultItem, bool, error)
	findRandomExcludingFunc          func(context.Context, []uint64) (RecommendResultItem, bool, error)
	saveUserVideoRecommendationFunc  func(context.Context, uint64, uint64, uint64, uint64, float64, time.Time) error
	getVideoIDBySegmentIDFunc        func(context.Context, uint64) (uint64, error)
	hasWatchedVideoForQuestionFunc   func(context.Context, uint64, uint64, uint64) (bool, error)
	incrementViewCountFunc           func(context.Context, uint64) (int, bool, error)
	saveWatchRecordFunc              func(context.Context, uint64, uint64, uint64, uint64, bool, int, time.Time) (bool, error)
	saveRecommendationExposuresFunc  func(context.Context, []RecommendationExposure) error
	markRecommendationExposureFunc   func(context.Context, uint64, uint64, uint64, time.Time) error
	getUserRecBoleEmbeddingFunc      func(context.Context, uint64, string) (UserRecBoleEmbedding, bool, error)
	findRecBoleSegmentsFunc          func(context.Context, RecBoleQuery) ([]RecBoleCandidate, error)
	getActiveRecBoleVersionFunc      func(context.Context) (string, bool, error)
	hydrateSegmentsFunc              func(context.Context, uint64, []uint64) ([]RecommendCandidate, error)
	getSegmentUserReactionFunc       func(context.Context, uint64, uint64) (VideoReactionType, bool, bool, error)
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
func (s *stubVideoRepository) SubmitSegmentReaction(context.Context, uint64, uint64, VideoReactionType) (bool, bool, error) {
	panic("unexpected call")
}
func (s *stubVideoRepository) GetSegmentUserReaction(ctx context.Context, segmentID uint64, userID uint64) (VideoReactionType, bool, bool, error) {
	if s.getSegmentUserReactionFunc != nil {
		return s.getSegmentUserReactionFunc(ctx, segmentID, userID)
	}
	return "", false, true, nil
}
func (s *stubVideoRepository) GetSegmentReactionCounts(context.Context, uint64) (VideoReactionCounts, bool, error) {
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
func (s *stubVideoRepository) FindRecommendedSegmentsByWeakKnowledge(ctx context.Context, userID uint64, limit int, weakLimit int) ([]RecommendCandidate, error) {
	if s.findWeakKnowledgeSegmentsFunc != nil {
		return s.findWeakKnowledgeSegmentsFunc(ctx, userID, limit, weakLimit)
	}
	return nil, nil
}
func (s *stubVideoRepository) ListWeakKnowledge(ctx context.Context, userID uint64, limit int) ([]WeakKnowledge, error) {
	if s.listWeakKnowledgeFunc != nil {
		return s.listWeakKnowledgeFunc(ctx, userID, limit)
	}
	return nil, nil
}
func (s *stubVideoRepository) FindRecommendedSegmentsByWeakKnowledgeVector(ctx context.Context, input WeakKnowledgeVectorQuery) ([]RecommendCandidate, error) {
	if s.findWeakKnowledgeVectorFunc != nil {
		return s.findWeakKnowledgeVectorFunc(ctx, input.UserID, input.Query, input.Limit)
	}
	return nil, nil
}
func (s *stubVideoRepository) FindRandomPlayableSegment(ctx context.Context) (RecommendResultItem, bool, error) {
	if s.findRandomPlayableSegmentFunc != nil {
		return s.findRandomPlayableSegmentFunc(ctx)
	}
	panic("unexpected call")
}
func (s *stubVideoRepository) FindRandomPlayableSegmentExcluding(ctx context.Context, excludedSegmentIDs []uint64) (RecommendResultItem, bool, error) {
	if s.findRandomExcludingFunc != nil {
		return s.findRandomExcludingFunc(ctx, excludedSegmentIDs)
	}
	return s.FindRandomPlayableSegment(ctx)
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
func (s *stubVideoRepository) SaveRecommendationExposures(ctx context.Context, exposures []RecommendationExposure) error {
	if s.saveRecommendationExposuresFunc != nil {
		return s.saveRecommendationExposuresFunc(ctx, exposures)
	}
	return nil
}
func (s *stubVideoRepository) MarkRecommendationExposureWatched(ctx context.Context, userID uint64, questionID uint64, segmentID uint64, now time.Time) error {
	if s.markRecommendationExposureFunc != nil {
		return s.markRecommendationExposureFunc(ctx, userID, questionID, segmentID, now)
	}
	return nil
}
func (s *stubVideoRepository) GetUserRecBoleEmbedding(ctx context.Context, userID uint64, modelVersion string) (UserRecBoleEmbedding, bool, error) {
	if s.getUserRecBoleEmbeddingFunc != nil {
		return s.getUserRecBoleEmbeddingFunc(ctx, userID, modelVersion)
	}
	return UserRecBoleEmbedding{}, false, nil
}
func (s *stubVideoRepository) FindRecommendedSegmentsForRecBole(ctx context.Context, input RecBoleQuery) ([]RecBoleCandidate, error) {
	if s.findRecBoleSegmentsFunc != nil {
		return s.findRecBoleSegmentsFunc(ctx, input)
	}
	return nil, nil
}
func (s *stubVideoRepository) GetActiveRecBoleModelVersion(ctx context.Context) (string, bool, error) {
	if s.getActiveRecBoleVersionFunc != nil {
		return s.getActiveRecBoleVersionFunc(ctx)
	}
	return "", false, nil
}
func (s *stubVideoRepository) HydrateRecommendedSegmentsByID(ctx context.Context, userID uint64, ids []uint64) ([]RecommendCandidate, error) {
	if s.hydrateSegmentsFunc != nil {
		return s.hydrateSegmentsFunc(ctx, userID, ids)
	}
	return nil, nil
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

	item, found, err := svc.RandomPlayVideoSegment(context.Background(), RandomPlayVideoSegmentInput{})
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

func TestRandomPlayVideoSegmentIncludesUserSegmentReaction(t *testing.T) {
	var reactionSegmentID uint64
	var reactionUserID uint64
	svc := NewService(&stubVideoRepository{
		findRandomPlayableSegmentFunc: func(context.Context) (RecommendResultItem, bool, error) {
			return RecommendResultItem{
				VideoID:        11,
				VideoSegmentID: 101,
				Video: domainvideo.Video{
					ID:          11,
					Title:       "video title",
					VideoURL:    "/videos/raw/2026/06/09/playable.mp4",
					Status:      domainvideo.StatusDone,
					IsPublished: true,
				},
			}, true, nil
		},
		saveUserVideoRecommendationFunc: func(context.Context, uint64, uint64, uint64, uint64, float64, time.Time) error {
			return nil
		},
		saveRecommendationExposuresFunc: func(context.Context, []RecommendationExposure) error {
			return nil
		},
		getSegmentUserReactionFunc: func(_ context.Context, segmentID uint64, userID uint64) (VideoReactionType, bool, bool, error) {
			reactionSegmentID = segmentID
			reactionUserID = userID
			return VideoReactionDoubleLike, true, true, nil
		},
	}, nil, nil, nil, nil, nil, nil, Paths{})

	item, found, err := svc.RandomPlayVideoSegment(context.Background(), RandomPlayVideoSegmentInput{UserID: 7})
	if err != nil {
		t.Fatalf("RandomPlayVideoSegment returned error: %v", err)
	}
	if !found {
		t.Fatal("expected found=true")
	}
	if reactionSegmentID != 101 || reactionUserID != 7 {
		t.Fatalf("reaction lookup segmentID=%d userID=%d, want 101 7", reactionSegmentID, reactionUserID)
	}
	if !item.UserReacted || item.UserReactionType != VideoReactionDoubleLike {
		t.Fatalf("reaction state = active %v type %q, want double_like active", item.UserReacted, item.UserReactionType)
	}
}

func TestRandomPlayVideoSegmentUsesWarmSegmentReactionStore(t *testing.T) {
	store := &videoTestReactionStore{
		hasUserReaction:       true,
		getUserReactionType:   VideoReactionDislike,
		getUserReactionActive: true,
		getUserReactionFound:  true,
	}
	svc := NewService(&stubVideoRepository{
		findRandomPlayableSegmentFunc: func(context.Context) (RecommendResultItem, bool, error) {
			return RecommendResultItem{
				VideoID:        11,
				VideoSegmentID: 101,
				Video: domainvideo.Video{
					ID:          11,
					Title:       "video title",
					VideoURL:    "/videos/raw/2026/06/09/playable.mp4",
					Status:      domainvideo.StatusDone,
					IsPublished: true,
				},
			}, true, nil
		},
		saveUserVideoRecommendationFunc: func(context.Context, uint64, uint64, uint64, uint64, float64, time.Time) error {
			return nil
		},
		saveRecommendationExposuresFunc: func(context.Context, []RecommendationExposure) error {
			return nil
		},
		getSegmentUserReactionFunc: func(context.Context, uint64, uint64) (VideoReactionType, bool, bool, error) {
			t.Fatal("database reaction lookup should not be used when segment reaction store is warm")
			return "", false, false, nil
		},
	}, nil, nil, nil, nil, nil, nil, Paths{})
	svc.SegmentReactionStore = store

	item, found, err := svc.RandomPlayVideoSegment(context.Background(), RandomPlayVideoSegmentInput{UserID: 7})
	if err != nil {
		t.Fatalf("RandomPlayVideoSegment returned error: %v", err)
	}
	if !found {
		t.Fatal("expected found=true")
	}
	if store.lastGetUserReactionVideoID != 101 || store.lastGetUserReactionUserID != 7 {
		t.Fatalf("store reaction lookup videoID=%d userID=%d, want 101 7", store.lastGetUserReactionVideoID, store.lastGetUserReactionUserID)
	}
	if !item.UserReacted || item.UserReactionType != VideoReactionDislike {
		t.Fatalf("reaction state = active %v type %q, want dislike active", item.UserReacted, item.UserReactionType)
	}
}

func TestRandomPlayVideoSegmentReportsMissingWhenRepositoryDoesNotSupportIt(t *testing.T) {
	svc := &Service{}

	_, found, err := svc.RandomPlayVideoSegment(context.Background(), RandomPlayVideoSegmentInput{})
	if err != nil {
		t.Fatalf("RandomPlayVideoSegment returned error: %v", err)
	}
	if found {
		t.Fatal("expected found=false")
	}
}

func TestRandomPlayVideoSegmentFallbackRecordsRecommendationExposure(t *testing.T) {
	now := time.Date(2026, 6, 23, 13, 0, 0, 0, time.UTC)
	var saved struct {
		userID     uint64
		questionID uint64
		videoID    uint64
		segmentID  uint64
		score      float64
	}
	var exposures []RecommendationExposure
	svc := NewService(&stubVideoRepository{
		findRandomPlayableSegmentFunc: func(context.Context) (RecommendResultItem, bool, error) {
			return RecommendResultItem{
				VideoID:        11,
				VideoSegmentID: 101,
				RecommendScore: 0.42,
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
		saveUserVideoRecommendationFunc: func(_ context.Context, userID uint64, questionID uint64, videoID uint64, segmentID uint64, score float64, _ time.Time) error {
			saved.userID = userID
			saved.questionID = questionID
			saved.videoID = videoID
			saved.segmentID = segmentID
			saved.score = score
			return nil
		},
		saveRecommendationExposuresFunc: func(_ context.Context, rows []RecommendationExposure) error {
			exposures = append(exposures, rows...)
			return nil
		},
	}, nil, nil, nil, nil, nil, nil, Paths{})
	svc.Now = func() time.Time { return now }

	item, found, err := svc.RandomPlayVideoSegment(context.Background(), RandomPlayVideoSegmentInput{UserID: 7})
	if err != nil {
		t.Fatalf("RandomPlayVideoSegment returned error: %v", err)
	}
	if !found {
		t.Fatal("expected found=true")
	}
	if item.VideoSegmentID != 101 || item.IsWatched || item.WatchDuration != 0 {
		t.Fatalf("unexpected fallback item: %+v", item)
	}
	if saved.userID != 7 || saved.questionID != 0 || saved.videoID != 11 || saved.segmentID != 101 || saved.score != 0.42 {
		t.Fatalf("unexpected saved recommendation: %+v", saved)
	}
	if len(exposures) != 1 {
		t.Fatalf("expected one exposure, got %d", len(exposures))
	}
	if !strings.HasPrefix(exposures[0].RequestID, "random-play-") || exposures[0].Strategy != RecommendStrategyRandomPlay || exposures[0].QuestionID != 0 {
		t.Fatalf("unexpected exposure: %+v", exposures[0])
	}
}

func TestRandomPlayVideoSegmentFallbackExcludesRecentlyReturnedSegments(t *testing.T) {
	recent := &stubRecentSegmentStore{recent: []uint64{101}}
	var excluded []uint64
	svc := NewService(&stubVideoRepository{
		findRandomExcludingFunc: func(_ context.Context, excludedSegmentIDs []uint64) (RecommendResultItem, bool, error) {
			excluded = append([]uint64(nil), excludedSegmentIDs...)
			return RecommendResultItem{
				VideoID:        12,
				VideoSegmentID: 102,
				RecommendScore: 0.4,
				Video: domainvideo.Video{
					ID:          12,
					Title:       "fresh video",
					VideoURL:    "/raw/12.mp4",
					Status:      domainvideo.StatusDone,
					IsPublished: true,
				},
			}, true, nil
		},
		findRandomPlayableSegmentFunc: func(context.Context) (RecommendResultItem, bool, error) {
			t.Fatal("unfiltered random fallback should not be used when exclusion has a result")
			return RecommendResultItem{}, false, nil
		},
		saveUserVideoRecommendationFunc: func(context.Context, uint64, uint64, uint64, uint64, float64, time.Time) error {
			return nil
		},
		saveRecommendationExposuresFunc: func(context.Context, []RecommendationExposure) error {
			return nil
		},
	}, nil, nil, nil, nil, nil, nil, Paths{})
	svc.RecentSegments = recent
	svc.RecentSegmentTTL = 30 * time.Minute

	item, found, err := svc.RandomPlayVideoSegment(context.Background(), RandomPlayVideoSegmentInput{UserID: 6})
	if err != nil {
		t.Fatalf("RandomPlayVideoSegment returned error: %v", err)
	}
	if !found || item.VideoSegmentID != 102 {
		t.Fatalf("item=%+v found=%v, want segment 102", item, found)
	}
	if len(excluded) != 1 || excluded[0] != 101 {
		t.Fatalf("excluded = %v, want [101]", excluded)
	}
	if len(recent.marked) != 1 || recent.marked[0] != 102 {
		t.Fatalf("marked recent = %v, want [102]", recent.marked)
	}
}

func TestRandomPlayVideoSegmentReturnsBucketItemWithoutRandomFallback(t *testing.T) {
	now := time.Date(2026, 7, 6, 10, 30, 0, 0, time.UTC)
	bucket := &stubRandomPlayBucket{items: []RecommendResultItem{{
		VideoID:               21,
		VideoSegmentID:        201,
		RecommendScore:        1,
		StartTimeSec:          5,
		EndTimeSec:            35,
		TitleOverride:         "bucket segment",
		RecommendStrategy:     RecommendStrategyGorse,
		RecommendModelVersion: recommendationapp.GorseModelVersion,
		Video: domainvideo.Video{
			ID:          21,
			Title:       "bucket video",
			VideoURL:    "/raw/21.mp4",
			CoverURL:    "/covers/21.jpg",
			Status:      domainvideo.StatusDone,
			IsPublished: true,
		},
	}}}
	recent := &stubRecentSegmentStore{}
	var savedSegmentID uint64
	var exposures []RecommendationExposure
	svc := NewService(&stubVideoRepository{
		hydrateSegmentsFunc: func(_ context.Context, userID uint64, ids []uint64) ([]RecommendCandidate, error) {
			t.Fatalf("bucket hit should not hydrate from PG, got userID=%d ids=%v", userID, ids)
			return nil, nil
		},
		findRandomPlayableSegmentFunc: func(context.Context) (RecommendResultItem, bool, error) {
			t.Fatal("random fallback should not be used when bucket has a valid segment")
			return RecommendResultItem{}, false, nil
		},
		saveUserVideoRecommendationFunc: func(_ context.Context, userID uint64, questionID uint64, videoID uint64, segmentID uint64, score float64, gotNow time.Time) error {
			if userID != 7 || questionID != 0 || videoID != 21 || segmentID != 201 || gotNow != now {
				t.Fatalf("unexpected saved recommendation userID=%d questionID=%d videoID=%d segmentID=%d now=%s", userID, questionID, videoID, segmentID, gotNow)
			}
			savedSegmentID = segmentID
			return nil
		},
		saveRecommendationExposuresFunc: func(_ context.Context, rows []RecommendationExposure) error {
			exposures = append(exposures, rows...)
			return nil
		},
	}, nil, nil, nil, nil, nil, nil, Paths{})
	svc.Now = func() time.Time { return now }
	svc.RandomPlayBucket = bucket
	svc.RecentSegments = recent
	svc.RecentSegmentTTL = 30 * time.Minute

	item, found, err := svc.RandomPlayVideoSegment(context.Background(), RandomPlayVideoSegmentInput{UserID: 7})
	if err != nil {
		t.Fatalf("RandomPlayVideoSegment returned error: %v", err)
	}
	if !found || item.VideoSegmentID != 201 || item.VideoID != 21 || item.Video.VideoURL != "/raw/21.mp4" {
		t.Fatalf("item=%+v found=%v, want bucket segment 201", item, found)
	}
	if savedSegmentID != 201 {
		t.Fatalf("saved segment = %d, want 201", savedSegmentID)
	}
	if len(exposures) != 1 || exposures[0].VideoSegmentID != 201 || exposures[0].Strategy != RecommendStrategyGorse || exposures[0].ModelVersion != recommendationapp.GorseModelVersion {
		t.Fatalf("exposures = %+v, want one gorse exposure for segment 201", exposures)
	}
	if len(recent.marked) != 1 || recent.marked[0] != 201 {
		t.Fatalf("marked recent = %v, want [201]", recent.marked)
	}
}

func TestRandomPlayVideoSegmentRefillsBucketFromGorsePreviewWithSourceMetadata(t *testing.T) {
	now := time.Date(2026, 7, 6, 11, 0, 0, 0, time.UTC)
	bucket := &stubRandomPlayBucket{}
	gorse := &videoAppFakeGorseClient{ids: []uint64{101, 102, 103}}
	var exposures []RecommendationExposure
	svc := NewService(&stubVideoRepository{
		hydrateSegmentsFunc: func(_ context.Context, userID uint64, ids []uint64) ([]RecommendCandidate, error) {
			if userID != 7 || len(ids) != 3 || ids[0] != 101 || ids[1] != 102 || ids[2] != 103 {
				t.Fatalf("unexpected hydrate inputs: userID=%d ids=%v", userID, ids)
			}
			return []RecommendCandidate{
				{VideoID: 1101, VideoSegmentID: 101, SegmentTitle: "gorse segment", VideoURL: "/raw/101.mp4", Status: int16(domainvideo.StatusDone), IsPublished: true},
				{VideoID: 1102, VideoSegmentID: 102, SegmentTitle: "gorse segment", VideoURL: "/raw/102.mp4", Status: int16(domainvideo.StatusDone), IsPublished: true},
				{VideoID: 1103, VideoSegmentID: 103, SegmentTitle: "gorse segment", VideoURL: "/raw/103.mp4", Status: int16(domainvideo.StatusDone), IsPublished: true},
			}, nil
		},
		findRandomPlayableSegmentFunc: func(context.Context) (RecommendResultItem, bool, error) {
			t.Fatal("random fallback should not be used when gorse preview returns items")
			return RecommendResultItem{}, false, nil
		},
		saveUserVideoRecommendationFunc: func(context.Context, uint64, uint64, uint64, uint64, float64, time.Time) error {
			return nil
		},
		saveRecommendationExposuresFunc: func(_ context.Context, rows []RecommendationExposure) error {
			exposures = append(exposures, rows...)
			return nil
		},
	}, nil, nil, nil, nil, nil, nil, Paths{})
	svc.Now = func() time.Time { return now }
	svc.RecommendationEngine = recommendationapp.EngineGorse
	svc.GorseClient = gorse
	svc.GorseOptions = recommendationapp.GorseOptions{CandidateLimit: 3, MinRecommendItems: 1}
	svc.RandomPlayBucket = bucket

	item, found, err := svc.RandomPlayVideoSegment(context.Background(), RandomPlayVideoSegmentInput{UserID: 7})
	if err != nil {
		t.Fatalf("RandomPlayVideoSegment returned error: %v", err)
	}
	if !found || item.VideoSegmentID != 101 {
		t.Fatalf("item=%+v found=%v, want first gorse segment 101", item, found)
	}
	if len(bucket.items) != 2 || bucket.items[0].VideoSegmentID != 102 || bucket.items[0].RecommendStrategy != RecommendStrategyGorse || bucket.items[0].RecommendModelVersion != recommendationapp.GorseModelVersion {
		t.Fatalf("bucket items = %+v, want remaining gorse items with source metadata", bucket.items)
	}
	if len(exposures) != 1 || exposures[0].VideoSegmentID != 101 || exposures[0].Strategy != RecommendStrategyGorse || exposures[0].ModelVersion != recommendationapp.GorseModelVersion {
		t.Fatalf("exposures = %+v, want gorse exposure for returned preview item", exposures)
	}
}

func TestRandomPlayVideoSegmentRefillsBucketFromRecBolePreviewWithSourceMetadata(t *testing.T) {
	now := time.Date(2026, 7, 8, 15, 30, 0, 0, time.UTC)
	bucket := &stubRandomPlayBucket{}
	var saved []uint64
	var exposures []RecommendationExposure
	svc := NewService(&stubVideoRepository{
		getActiveRecBoleVersionFunc: func(context.Context) (string, bool, error) {
			return "recbole_v2", true, nil
		},
		getUserRecBoleEmbeddingFunc: func(_ context.Context, userID uint64, modelVersion string) (UserRecBoleEmbedding, bool, error) {
			if userID != 7 || modelVersion != "recbole_v2" {
				t.Fatalf("unexpected RecBole lookup: userID=%d modelVersion=%q", userID, modelVersion)
			}
			return UserRecBoleEmbedding{UserID: 7, Vector: []float32{0.2, 0.8}, ModelVersion: modelVersion, Status: 1}, true, nil
		},
		findRecBoleSegmentsFunc: func(_ context.Context, input RecBoleQuery) ([]RecBoleCandidate, error) {
			if input.UserID != 7 || input.ModelVersion != "recbole_v2" || input.Limit != 100 {
				t.Fatalf("unexpected RecBole query: %+v", input)
			}
			return []RecBoleCandidate{
				{RecommendCandidate: RecommendCandidate{VideoID: 1101, VideoSegmentID: 101, Distance: 0.25, SegmentTitle: "recbole segment", VideoURL: "/raw/101.mp4", Status: int16(domainvideo.StatusDone), IsPublished: true}},
				{RecommendCandidate: RecommendCandidate{VideoID: 1102, VideoSegmentID: 102, Distance: 0.10, SegmentTitle: "recbole segment", VideoURL: "/raw/102.mp4", Status: int16(domainvideo.StatusDone), IsPublished: true}},
				{RecommendCandidate: RecommendCandidate{VideoID: 1103, VideoSegmentID: 103, Distance: 0.40, SegmentTitle: "recbole segment", VideoURL: "/raw/103.mp4", Status: int16(domainvideo.StatusDone), IsPublished: true}},
			}, nil
		},
		findRandomPlayableSegmentFunc: func(context.Context) (RecommendResultItem, bool, error) {
			t.Fatal("random fallback should not be used when recbole preview returns items")
			return RecommendResultItem{}, false, nil
		},
		saveUserVideoRecommendationFunc: func(_ context.Context, _ uint64, _ uint64, _ uint64, segmentID uint64, _ float64, _ time.Time) error {
			saved = append(saved, segmentID)
			return nil
		},
		saveRecommendationExposuresFunc: func(_ context.Context, rows []RecommendationExposure) error {
			exposures = append(exposures, rows...)
			return nil
		},
	}, nil, nil, nil, nil, nil, nil, Paths{})
	svc.Now = func() time.Time { return now }
	svc.RecommendationEngine = recommendationapp.EngineRecBole
	svc.RandomPlayBucket = bucket

	item, found, err := svc.RandomPlayVideoSegment(context.Background(), RandomPlayVideoSegmentInput{UserID: 7})
	if err != nil {
		t.Fatalf("RandomPlayVideoSegment returned error: %v", err)
	}
	if !found || item.VideoSegmentID != 102 {
		t.Fatalf("item=%+v found=%v, want first recbole segment 102", item, found)
	}
	if len(bucket.items) != 2 || bucket.items[0].VideoSegmentID != 101 || bucket.items[0].RecommendStrategy != RecommendStrategyRecBole || bucket.items[0].RecommendModelVersion != "recbole_v2" {
		t.Fatalf("bucket items = %+v, want remaining recbole items with source metadata", bucket.items)
	}
	if len(saved) != 1 || saved[0] != 102 {
		t.Fatalf("saved recommendations = %v, want only returned segment 102", saved)
	}
	if len(exposures) != 1 || exposures[0].VideoSegmentID != 102 || exposures[0].Strategy != RecommendStrategyRecBole || exposures[0].ModelVersion != "recbole_v2" {
		t.Fatalf("exposures = %+v, want recbole exposure for returned preview item", exposures)
	}
}

func TestRandomPlayVideoSegmentRefillsBucketFromWeakKnowledgePreviewWithoutRecordingSurplus(t *testing.T) {
	now := time.Date(2026, 7, 6, 10, 45, 0, 0, time.UTC)
	bucket := &stubRandomPlayBucket{}
	var saved []uint64
	var exposures []RecommendationExposure
	svc := NewService(&stubVideoRepository{
		getSegmentEmbeddingDimFunc: func(context.Context) (int, error) {
			return 3, nil
		},
		listWeakKnowledgeFunc: func(_ context.Context, userID uint64, limit int) ([]WeakKnowledge, error) {
			if userID != 7 || limit != 10 {
				t.Fatalf("weak knowledge userID=%d limit=%d, want 7 10", userID, limit)
			}
			return []WeakKnowledge{{Name: "函数", Description: "图像"}}, nil
		},
		findWeakKnowledgeVectorFunc: func(_ context.Context, userID uint64, _ pgvector.Vector, limit int) ([]RecommendCandidate, error) {
			if userID != 7 || limit != 50 {
				t.Fatalf("weak knowledge vector userID=%d limit=%d, want 7 50", userID, limit)
			}
			return []RecommendCandidate{
				{VideoID: 1101, VideoSegmentID: 101, SegmentTitle: "prefetched segment", VideoURL: "/raw/101.mp4", Status: int16(domainvideo.StatusDone), IsPublished: true},
				{VideoID: 1102, VideoSegmentID: 102, SegmentTitle: "prefetched segment", VideoURL: "/raw/102.mp4", Status: int16(domainvideo.StatusDone), IsPublished: true},
				{VideoID: 1103, VideoSegmentID: 103, SegmentTitle: "prefetched segment", VideoURL: "/raw/103.mp4", Status: int16(domainvideo.StatusDone), IsPublished: true},
				{VideoID: 1104, VideoSegmentID: 104, SegmentTitle: "prefetched segment", VideoURL: "/raw/104.mp4", Status: int16(domainvideo.StatusDone), IsPublished: true},
				{VideoID: 1105, VideoSegmentID: 105, SegmentTitle: "prefetched segment", VideoURL: "/raw/105.mp4", Status: int16(domainvideo.StatusDone), IsPublished: true},
			}, nil
		},
		findRandomPlayableSegmentFunc: func(context.Context) (RecommendResultItem, bool, error) {
			t.Fatal("random fallback should not be used when preview fills bucket")
			return RecommendResultItem{}, false, nil
		},
		saveUserVideoRecommendationFunc: func(_ context.Context, _ uint64, _ uint64, _ uint64, segmentID uint64, _ float64, _ time.Time) error {
			saved = append(saved, segmentID)
			return nil
		},
		saveRecommendationExposuresFunc: func(_ context.Context, rows []RecommendationExposure) error {
			exposures = append(exposures, rows...)
			return nil
		},
	}, nil, nil, nil, nil, nil, nil, Paths{})
	svc.Now = func() time.Time { return now }
	svc.RandomPlayBucket = bucket
	svc.Embedder = &stubEmbedder{vector: []float32{1, 0, 0}}
	svc.RecommendationEngine = recommendationapp.EngineKnowledgeMatch

	item, found, err := svc.RandomPlayVideoSegment(context.Background(), RandomPlayVideoSegmentInput{UserID: 7})
	if err != nil {
		t.Fatalf("RandomPlayVideoSegment returned error: %v", err)
	}
	if !found || item.VideoSegmentID != 101 {
		t.Fatalf("item=%+v found=%v, want first prefetched segment 101", item, found)
	}
	if len(bucket.items) != 4 || bucket.items[0].VideoSegmentID != 102 || bucket.items[3].VideoSegmentID != 105 || bucket.items[0].Video.VideoURL != "/raw/102.mp4" {
		t.Fatalf("bucket items after return = %v, want [102 103 104 105]", bucket.items)
	}
	if len(saved) != 1 || saved[0] != 101 {
		t.Fatalf("saved recommendations = %v, want only returned segment 101", saved)
	}
	if len(exposures) != 1 || exposures[0].VideoSegmentID != 101 {
		t.Fatalf("exposures = %+v, want only returned segment 101", exposures)
	}
}

func TestRandomPlayVideoSegmentIgnoresBucketErrorAndUsesExistingFallback(t *testing.T) {
	bucket := &stubRandomPlayBucket{popErr: errors.New("redis unavailable")}
	svc := NewService(&stubVideoRepository{
		findRandomPlayableSegmentFunc: func(context.Context) (RecommendResultItem, bool, error) {
			return RecommendResultItem{
				VideoID:        12,
				VideoSegmentID: 102,
				Video: domainvideo.Video{
					ID:          12,
					Title:       "fallback video",
					VideoURL:    "/raw/12.mp4",
					Status:      domainvideo.StatusDone,
					IsPublished: true,
				},
			}, true, nil
		},
		saveUserVideoRecommendationFunc: func(context.Context, uint64, uint64, uint64, uint64, float64, time.Time) error {
			return nil
		},
		saveRecommendationExposuresFunc: func(context.Context, []RecommendationExposure) error {
			return nil
		},
	}, nil, nil, nil, nil, nil, nil, Paths{})
	svc.RandomPlayBucket = bucket

	item, found, err := svc.RandomPlayVideoSegment(context.Background(), RandomPlayVideoSegmentInput{UserID: 7})
	if err != nil {
		t.Fatalf("RandomPlayVideoSegment returned error: %v", err)
	}
	if !found || item.VideoSegmentID != 102 {
		t.Fatalf("item=%+v found=%v, want fallback segment 102", item, found)
	}
}

func TestRandomPlayVideoSegmentUsesRecBoleAndRecordsRecommendation(t *testing.T) {
	now := time.Date(2026, 6, 23, 12, 30, 0, 0, time.UTC)
	var saved struct {
		userID     uint64
		questionID uint64
		videoID    uint64
		segmentID  uint64
		score      float64
	}
	var exposures []RecommendationExposure
	randomFallbackCalls := 0
	svc := NewService(&stubVideoRepository{
		getActiveRecBoleVersionFunc: func(context.Context) (string, bool, error) {
			return "recbole_v2", true, nil
		},
		getUserRecBoleEmbeddingFunc: func(_ context.Context, userID uint64, modelVersion string) (UserRecBoleEmbedding, bool, error) {
			if userID != 7 || modelVersion != "recbole_v2" {
				t.Fatalf("unexpected RecBole lookup: userID=%d modelVersion=%q", userID, modelVersion)
			}
			return UserRecBoleEmbedding{UserID: 7, Vector: []float32{0.2, 0.8}, ModelVersion: modelVersion, Status: 1}, true, nil
		},
		findRecBoleSegmentsFunc: func(_ context.Context, input RecBoleQuery) ([]RecBoleCandidate, error) {
			if input.UserID != 7 || input.ModelVersion != "recbole_v2" || input.Limit != 100 {
				t.Fatalf("unexpected RecBole query: %+v", input)
			}
			return []RecBoleCandidate{{RecommendCandidate: RecommendCandidate{
				VideoID:        11,
				VideoSegmentID: 101,
				StartTimeSec:   10,
				EndTimeSec:     40,
				Distance:       0.25,
				SegmentTitle:   "personalized segment",
				VideoURL:       "/videos/raw/2026/06/09/playable.mp4",
				CoverURL:       "/covers/11.jpg",
				Status:         int16(domainvideo.StatusDone),
				IsPublished:    true,
			}}}, nil
		},
		saveUserVideoRecommendationFunc: func(_ context.Context, userID uint64, questionID uint64, videoID uint64, segmentID uint64, score float64, _ time.Time) error {
			saved.userID = userID
			saved.questionID = questionID
			saved.videoID = videoID
			saved.segmentID = segmentID
			saved.score = score
			return nil
		},
		saveRecommendationExposuresFunc: func(_ context.Context, rows []RecommendationExposure) error {
			exposures = append(exposures, rows...)
			return nil
		},
		findRandomPlayableSegmentFunc: func(context.Context) (RecommendResultItem, bool, error) {
			randomFallbackCalls++
			return RecommendResultItem{}, false, nil
		},
	}, nil, nil, nil, nil, nil, nil, Paths{})
	svc.Now = func() time.Time { return now }
	svc.RecommendationEngine = "recbole"

	item, found, err := svc.RandomPlayVideoSegment(context.Background(), RandomPlayVideoSegmentInput{UserID: 7})
	if err != nil {
		t.Fatalf("RandomPlayVideoSegment returned error: %v", err)
	}
	if !found {
		t.Fatal("expected found=true")
	}
	if randomFallbackCalls != 0 {
		t.Fatalf("expected no random fallback, got %d calls", randomFallbackCalls)
	}
	if item.VideoSegmentID != 101 || item.QuestionID != 0 || item.IsWatched || item.WatchDuration != 0 {
		t.Fatalf("unexpected item: %+v", item)
	}
	if saved.userID != 7 || saved.questionID != 0 || saved.videoID != 11 || saved.segmentID != 101 {
		t.Fatalf("unexpected saved recommendation: %+v", saved)
	}
	if saved.score <= 0 {
		t.Fatalf("expected positive saved score, got %v", saved.score)
	}
	if len(exposures) != 1 {
		t.Fatalf("expected one exposure, got %d", len(exposures))
	}
	if exposures[0].UserID != 7 || exposures[0].QuestionID != 0 || exposures[0].VideoSegmentID != 101 || exposures[0].Strategy != RecommendStrategyRecBole || exposures[0].ModelVersion != "recbole_v2" {
		t.Fatalf("unexpected exposure: %+v", exposures[0])
	}
}

func TestExternalRecBoleItemIDsUsesRecBoleWithoutSideEffects(t *testing.T) {
	saveCalls := 0
	exposureCalls := 0
	svc := NewService(&stubVideoRepository{
		getActiveRecBoleVersionFunc: func(context.Context) (string, bool, error) {
			return "recbole_v2", true, nil
		},
		getUserRecBoleEmbeddingFunc: func(_ context.Context, userID uint64, modelVersion string) (UserRecBoleEmbedding, bool, error) {
			if userID != 7 || modelVersion != "recbole_v2" {
				t.Fatalf("unexpected RecBole lookup: userID=%d modelVersion=%q", userID, modelVersion)
			}
			return UserRecBoleEmbedding{UserID: 7, Vector: []float32{0.2, 0.8}, ModelVersion: modelVersion, Status: 1}, true, nil
		},
		findRecBoleSegmentsFunc: func(_ context.Context, input RecBoleQuery) ([]RecBoleCandidate, error) {
			if input.UserID != 7 || input.ModelVersion != "recbole_v2" || input.Limit != 100 {
				t.Fatalf("unexpected RecBole query: %+v", input)
			}
			return []RecBoleCandidate{
				{RecommendCandidate: RecommendCandidate{VideoSegmentID: 101, VideoID: 11, Distance: 0.25}},
				{RecommendCandidate: RecommendCandidate{VideoSegmentID: 102, VideoID: 12, Distance: 0.10}},
				{RecommendCandidate: RecommendCandidate{VideoSegmentID: 103, VideoID: 13, Distance: 0.40}},
			}, nil
		},
		saveUserVideoRecommendationFunc: func(context.Context, uint64, uint64, uint64, uint64, float64, time.Time) error {
			saveCalls++
			return nil
		},
		saveRecommendationExposuresFunc: func(context.Context, []RecommendationExposure) error {
			exposureCalls++
			return nil
		},
		findRandomPlayableSegmentFunc: func(context.Context) (RecommendResultItem, bool, error) {
			t.Fatal("external RecBole should not use random fallback")
			return RecommendResultItem{}, false, nil
		},
	}, nil, nil, nil, nil, nil, nil, Paths{})
	svc.RecommendationEngine = "gorse"
	svc.GorseClient = &videoAppFakeGorseClient{ids: []uint64{999}}

	ids, err := svc.ExternalRecBoleItemIDs(context.Background(), RandomPlayVideoSegmentInput{UserID: 7, Limit: 2})
	if err != nil {
		t.Fatalf("ExternalRecBoleItemIDs returned error: %v", err)
	}
	want := []uint64{102, 101}
	if len(ids) != len(want) || ids[0] != want[0] || ids[1] != want[1] {
		t.Fatalf("ids = %v, want %v", ids, want)
	}
	if saveCalls != 0 || exposureCalls != 0 {
		t.Fatalf("external RecBole must not write recommendations or exposures: save=%d exposure=%d", saveCalls, exposureCalls)
	}
}

func TestRandomPlayVideoSegmentUsesKnowledgeMatchByDefault(t *testing.T) {
	now := time.Date(2026, 6, 26, 12, 20, 0, 0, time.UTC)
	var savedSegmentID uint64
	var exposures []RecommendationExposure
	randomFallbackCalls := 0
	embedder := &stubEmbedder{vector: []float32{1, 2, 3}}
	svc := NewService(&stubVideoRepository{
		getSegmentEmbeddingDimFunc: func(context.Context) (int, error) {
			return 3, nil
		},
		listWeakKnowledgeFunc: func(_ context.Context, userID uint64, limit int) ([]WeakKnowledge, error) {
			if userID != 7 || limit != 10 {
				t.Fatalf("unexpected weak knowledge inputs: userID=%d limit=%d", userID, limit)
			}
			return []WeakKnowledge{{KnowledgePointID: 1, Mastery: 0.1, Name: "一次函数", Description: "图像与斜率"}}, nil
		},
		findWeakKnowledgeVectorFunc: func(_ context.Context, userID uint64, query pgvector.Vector, limit int) ([]RecommendCandidate, error) {
			if userID != 7 || limit != recommendationapp.WeakKnowledgeRecallLimit {
				t.Fatalf("unexpected vector inputs: userID=%d limit=%d", userID, limit)
			}
			if got := query.Slice(); len(got) != 3 || got[0] != 1 || got[1] != 2 || got[2] != 3 {
				t.Fatalf("query vector = %#v, want embedder vector", got)
			}
			return []RecommendCandidate{{
				VideoID:        11,
				VideoSegmentID: 101,
				StartTimeSec:   10,
				EndTimeSec:     40,
				Distance:       0.1,
				SegmentTitle:   "一次函数图像",
				VideoURL:       "/raw/11.mp4",
				Status:         int16(domainvideo.StatusDone),
				IsPublished:    true,
				IsRecommend:    true,
			}}, nil
		},
		saveUserVideoRecommendationFunc: func(_ context.Context, _ uint64, _ uint64, _ uint64, segmentID uint64, _ float64, _ time.Time) error {
			savedSegmentID = segmentID
			return nil
		},
		saveRecommendationExposuresFunc: func(_ context.Context, rows []RecommendationExposure) error {
			exposures = append(exposures, rows...)
			return nil
		},
		findRandomPlayableSegmentFunc: func(context.Context) (RecommendResultItem, bool, error) {
			randomFallbackCalls++
			return RecommendResultItem{}, false, nil
		},
	}, nil, nil, nil, nil, nil, embedder, Paths{})
	svc.Now = func() time.Time { return now }

	item, found, err := svc.RandomPlayVideoSegment(context.Background(), RandomPlayVideoSegmentInput{UserID: 7, Limit: 1})
	if err != nil {
		t.Fatalf("RandomPlayVideoSegment returned error: %v", err)
	}
	if !found || item.VideoSegmentID != 101 || item.TitleOverride != "一次函数图像" {
		t.Fatalf("item=%+v found=%v, want knowledge match segment 101", item, found)
	}
	if randomFallbackCalls != 0 {
		t.Fatalf("expected no random fallback, got %d calls", randomFallbackCalls)
	}
	if savedSegmentID != 101 {
		t.Fatalf("savedSegmentID=%d, want 101", savedSegmentID)
	}
	if len(embedder.texts) != 1 || !strings.Contains(embedder.texts[0], "一次函数") || !strings.Contains(embedder.texts[0], "图像与斜率") {
		t.Fatalf("embedder texts=%#v, want weak knowledge text", embedder.texts)
	}
	if len(exposures) != 1 || exposures[0].Strategy != RecommendStrategyKnowledgeMatch || exposures[0].ModelVersion != "knowledge_match_v1" {
		t.Fatalf("exposures=%+v, want knowledge_match strategy", exposures)
	}
}

func TestRandomPlayVideoSegmentUsesConfiguredGorseClient(t *testing.T) {
	now := time.Date(2026, 6, 26, 11, 30, 0, 0, time.UTC)
	var savedSegmentID uint64
	var exposures []RecommendationExposure
	gorse := &videoAppFakeGorseClient{ids: []uint64{101}}
	svc := NewService(&stubVideoRepository{
		hydrateSegmentsFunc: func(_ context.Context, userID uint64, ids []uint64) ([]RecommendCandidate, error) {
			if userID != 7 || len(ids) != 1 || ids[0] != 101 {
				t.Fatalf("unexpected hydrate inputs: userID=%d ids=%v", userID, ids)
			}
			return []RecommendCandidate{{
				VideoID:        11,
				VideoSegmentID: 101,
				StartTimeSec:   10,
				EndTimeSec:     40,
				SegmentTitle:   "gorse segment",
				VideoURL:       "/raw/11.mp4",
				Status:         int16(domainvideo.StatusDone),
				IsPublished:    true,
				IsRecommend:    true,
			}}, nil
		},
		saveUserVideoRecommendationFunc: func(_ context.Context, _ uint64, _ uint64, _ uint64, segmentID uint64, _ float64, _ time.Time) error {
			savedSegmentID = segmentID
			return nil
		},
		saveRecommendationExposuresFunc: func(_ context.Context, rows []RecommendationExposure) error {
			exposures = append(exposures, rows...)
			return nil
		},
		findRandomPlayableSegmentFunc: func(context.Context) (RecommendResultItem, bool, error) {
			t.Fatal("random fallback should not be used")
			return RecommendResultItem{}, false, nil
		},
	}, nil, nil, nil, nil, nil, nil, Paths{})
	svc.Now = func() time.Time { return now }
	svc.RecommendationEngine = "gorse"
	svc.GorseClient = gorse
	svc.GorseOptions = recommendationapp.GorseOptions{CandidateLimit: 3, WriteBackEnabled: true}

	item, found, err := svc.RandomPlayVideoSegment(context.Background(), RandomPlayVideoSegmentInput{UserID: 7, Limit: 1})
	if err != nil {
		t.Fatalf("RandomPlayVideoSegment returned error: %v", err)
	}
	if !found || item.VideoSegmentID != 101 || item.TitleOverride != "gorse segment" {
		t.Fatalf("item=%+v found=%v, want gorse segment 101", item, found)
	}
	if gorse.userID != 7 || gorse.n != 3 {
		t.Fatalf("gorse call userID=%d n=%d, want 7 3", gorse.userID, gorse.n)
	}
	if savedSegmentID != 101 {
		t.Fatalf("savedSegmentID=%d, want 101", savedSegmentID)
	}
	if len(exposures) != 1 || exposures[0].Strategy != RecommendStrategyGorse {
		t.Fatalf("exposures=%+v, want gorse strategy", exposures)
	}
	if len(gorse.feedback) != 1 || gorse.feedback[0].FeedbackType != "exposure" {
		t.Fatalf("feedback=%+v, want exposure", gorse.feedback)
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
	}, nil, nil, nil, nil, nil, &stubEmbedder{vector: []float32{1, 2, 3}}, Paths{})

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

func TestRecommendByQuestion_PersistsExposureLog(t *testing.T) {
	var exposures []RecommendationExposure
	svc := NewService(&stubVideoRepository{
		getSegmentEmbeddingDimFunc: func(context.Context) (int, error) {
			return 3, nil
		},
		findRecommendedSegmentsFunc: func(_ context.Context, query pgvector.Vector, limit int) ([]RecommendCandidate, error) {
			return []RecommendCandidate{{
				VideoID:        21,
				VideoSegmentID: 31,
				Distance:       0.1,
				Status:         int16(domainvideo.StatusDone),
				VideoURL:       "/videos/raw/2026/04/28/by-id.mp4",
			}}, nil
		},
		getQuestionEmbeddingTextByIDFunc: func(context.Context, uint64) (string, error) {
			return "[1,2,3]", nil
		},
		saveUserVideoRecommendationFunc: func(_ context.Context, _ uint64, _ uint64, _ uint64, _ uint64, _ float64, _ time.Time) error {
			return nil
		},
		saveRecommendationExposuresFunc: func(_ context.Context, rows []RecommendationExposure) error {
			exposures = append(exposures, rows...)
			return nil
		},
	}, nil, nil, nil, nil, nil, nil, Paths{})

	if _, err := svc.RecommendByQuestion(context.Background(), RecommendByQuestionInput{QuestionID: 99, UserID: 7, Limit: 3}); err != nil {
		t.Fatalf("recommend by question id: %v", err)
	}
	if len(exposures) != 1 {
		t.Fatalf("expected one exposure, got %d", len(exposures))
	}
	if exposures[0].UserID != 7 || exposures[0].QuestionID != 99 || exposures[0].VideoID != 21 || exposures[0].VideoSegmentID != 31 {
		t.Fatalf("unexpected exposure: %+v", exposures[0])
	}
	if exposures[0].Rank != 1 || exposures[0].Strategy != RecommendStrategyQuestionVector {
		t.Fatalf("unexpected exposure metadata: %+v", exposures[0])
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
	var markedUserID uint64
	var markedSegmentID uint64
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
		markRecommendationExposureFunc: func(_ context.Context, userID uint64, questionID uint64, segmentID uint64, _ time.Time) error {
			if questionID != 3 {
				t.Fatalf("unexpected exposure questionID: %d", questionID)
			}
			markedUserID = userID
			markedSegmentID = segmentID
			return nil
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
	if markedUserID != 7 || markedSegmentID != 204 {
		t.Fatalf("expected exposure marked for user 7 segment 204, got userID=%d segmentID=%d", markedUserID, markedSegmentID)
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
	texts  []string
}

type videoAppFakeGorseClient struct {
	userID   uint64
	n        int
	ids      []uint64
	feedback []recommendationapp.GorseFeedback
}

type stubRecentSegmentStore struct {
	recent     []uint64
	marked     []uint64
	lastUserID uint64
	lastTTL    time.Duration
	ttl        time.Duration
}

type stubRandomPlayBucket struct {
	items    []RecommendResultItem
	filled   [][]RecommendResultItem
	popErr   error
	lastTTL  time.Duration
	lastUser uint64
	lastMax  int
	ttl      time.Duration
}

func (s *stubRandomPlayBucket) Pop(ctx context.Context, userID uint64) (RecommendResultItem, bool, error) {
	s.lastUser = userID
	if s.popErr != nil {
		return RecommendResultItem{}, false, s.popErr
	}
	if len(s.items) == 0 {
		return RecommendResultItem{}, false, nil
	}
	item := s.items[0]
	s.items = s.items[1:]
	return item, true, nil
}

func (s *stubRandomPlayBucket) Fill(ctx context.Context, userID uint64, items []RecommendResultItem, maxSize int, ttl time.Duration) error {
	s.lastUser = userID
	s.lastMax = maxSize
	s.lastTTL = ttl
	s.filled = append(s.filled, append([]RecommendResultItem(nil), items...))
	seen := make(map[uint64]bool, len(s.items)+len(items))
	for _, item := range s.items {
		segmentID := item.VideoSegmentID
		if segmentID == 0 || seen[segmentID] {
			continue
		}
		seen[segmentID] = true
	}
	for _, item := range items {
		segmentID := item.VideoSegmentID
		if segmentID == 0 || seen[segmentID] || len(s.items) >= maxSize {
			continue
		}
		seen[segmentID] = true
		s.items = append(s.items, item)
	}
	return nil
}

func (s *stubRandomPlayBucket) Len(ctx context.Context, userID uint64) (int64, error) {
	s.lastUser = userID
	return int64(len(s.items)), nil
}

func (s *stubRandomPlayBucket) List(ctx context.Context, userID uint64) ([]RecommendResultItem, error) {
	s.lastUser = userID
	return append([]RecommendResultItem(nil), s.items...), nil
}

func (s *stubRandomPlayBucket) TTL(ctx context.Context, userID uint64) (time.Duration, error) {
	s.lastUser = userID
	return s.ttl, nil
}

func (s *stubRecentSegmentStore) FilterRecent(ctx context.Context, userID uint64, segmentIDs []uint64) (map[uint64]bool, error) {
	s.lastUserID = userID
	recent := make(map[uint64]bool, len(s.recent))
	for _, segmentID := range s.recent {
		recent[segmentID] = true
	}
	out := make(map[uint64]bool)
	for _, segmentID := range segmentIDs {
		if recent[segmentID] {
			out[segmentID] = true
		}
	}
	return out, nil
}

func (s *stubRecentSegmentStore) ListRecent(ctx context.Context, userID uint64) ([]uint64, error) {
	s.lastUserID = userID
	return append([]uint64(nil), s.recent...), nil
}

func (s *stubRecentSegmentStore) MarkReturned(ctx context.Context, userID uint64, segmentID uint64, ttl time.Duration) error {
	s.lastUserID = userID
	s.lastTTL = ttl
	s.marked = append(s.marked, segmentID)
	return nil
}

func (s *stubRecentSegmentStore) TTL(ctx context.Context, userID uint64) (time.Duration, error) {
	s.lastUserID = userID
	return s.ttl, nil
}

func (c *videoAppFakeGorseClient) Recommend(_ context.Context, userID uint64, n int) ([]uint64, error) {
	c.userID = userID
	c.n = n
	return c.ids, nil
}

func (c *videoAppFakeGorseClient) PutFeedback(_ context.Context, feedback []recommendationapp.GorseFeedback) error {
	c.feedback = append(c.feedback, feedback...)
	return nil
}

func (c *videoAppFakeGorseClient) UpsertUsers(context.Context, []recommendationapp.GorseUser) error {
	return nil
}

func (c *videoAppFakeGorseClient) UpsertItems(context.Context, []recommendationapp.GorseItem) error {
	return nil
}

func (c *videoAppFakeGorseClient) PatchItem(context.Context, recommendationapp.GorseItem) error {
	return nil
}

func (s *stubEmbedder) Embed(_ context.Context, text string) ([]float32, error) {
	if s.err != nil {
		return nil, s.err
	}
	s.texts = append(s.texts, text)
	return s.vector, nil
}

type stubStatusStore struct{}

func (stubStatusStore) Set(context.Context, string, domainvideo.Status, string, time.Duration) error {
	panic("unexpected call")
}

func (stubStatusStore) Get(context.Context, string) (TranscodeStatus, bool, error) {
	return TranscodeStatus{}, false, nil
}
