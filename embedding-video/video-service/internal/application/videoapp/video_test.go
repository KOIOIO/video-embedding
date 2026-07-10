package videoapp

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/pgvector/pgvector-go"

	domainvideo "nlp-video-analysis/internal/domain/video"
)

func TestListVideosPassesThroughRepo(t *testing.T) {
	repo := &videoTestRepo{
		listVideos: []domainvideo.Video{{ID: 1, Title: "physics"}},
	}
	svc := NewService(repo, nil, nil, nil, nil, nil, nil, Paths{})

	videos, err := svc.ListVideos(context.Background(), ListHLSOnly)
	if err != nil {
		t.Fatalf("ListVideos returned error: %v", err)
	}
	if repo.lastListFilter != ListHLSOnly {
		t.Fatalf("unexpected list filter: %v", repo.lastListFilter)
	}
	if len(videos) != 1 || videos[0].ID != 1 {
		t.Fatalf("unexpected videos: %+v", videos)
	}
}

func TestListRecommendPoolVideosPassesThroughRepo(t *testing.T) {
	repo := &videoTestRepo{
		recommendPool: []domainvideo.Video{{ID: 3, Title: "math"}},
	}
	svc := NewService(repo, nil, nil, nil, nil, nil, nil, Paths{})

	videos, err := svc.ListRecommendPoolVideos(context.Background())
	if err != nil {
		t.Fatalf("ListRecommendPoolVideos returned error: %v", err)
	}
	if repo.listRecommendPoolCalls != 1 {
		t.Fatalf("expected one recommend pool call, got %d", repo.listRecommendPoolCalls)
	}
	if len(videos) != 1 || videos[0].ID != 3 {
		t.Fatalf("unexpected recommend pool videos: %+v", videos)
	}
}

func TestDeleteVideoRejectsZeroID(t *testing.T) {
	svc := NewService(&videoTestRepo{}, nil, nil, nil, nil, nil, nil, Paths{})

	_, err := svc.DeleteVideo(context.Background(), 0, "")
	if err == nil {
		t.Fatal("expected validation error")
	}
	assertValidationMessage(t, err, "video_id is required")
}

func TestDeleteVideoPassesThroughRepo(t *testing.T) {
	repo := &videoTestRepo{deleteOK: true}
	svc := NewService(repo, nil, nil, nil, nil, nil, nil, Paths{})

	ok, err := svc.DeleteVideo(context.Background(), 12, "")
	if err != nil {
		t.Fatalf("DeleteVideo returned error: %v", err)
	}
	if !ok {
		t.Fatal("expected ok=true")
	}
	if repo.lastDeleteID != 12 {
		t.Fatalf("unexpected delete id: %d", repo.lastDeleteID)
	}
}

func TestUpdateVideoMetadataValidatesInputsAndTrimsTitle(t *testing.T) {
	svc := NewService(&videoTestRepo{}, nil, nil, nil, nil, nil, nil, Paths{})

	_, err := svc.UpdateVideoMetadata(context.Background(), 0, "title", "desc")
	if err == nil {
		t.Fatal("expected validation error for video id")
	}
	assertValidationMessage(t, err, "video_id is required")

	_, err = svc.UpdateVideoMetadata(context.Background(), 1, "   ", "desc")
	if err == nil {
		t.Fatal("expected validation error for title")
	}
	assertValidationMessage(t, err, "title is required")

	repo := &videoTestRepo{updateMetadataOK: true}
	svc = NewService(repo, nil, nil, nil, nil, nil, nil, Paths{})
	ok, err := svc.UpdateVideoMetadata(context.Background(), 7, "  高中化学  ", "desc")
	if err != nil {
		t.Fatalf("UpdateVideoMetadata returned error: %v", err)
	}
	if !ok {
		t.Fatal("expected ok=true")
	}
	if repo.lastUpdateMetadataID != 7 {
		t.Fatalf("unexpected metadata update id: %d", repo.lastUpdateMetadataID)
	}
	if repo.lastUpdateTitle != "高中化学" {
		t.Fatalf("expected trimmed title, got %q", repo.lastUpdateTitle)
	}
	if repo.lastUpdateDescription != "desc" {
		t.Fatalf("unexpected description: %q", repo.lastUpdateDescription)
	}
}

func TestSetVideoPublishedPassesThroughRepo(t *testing.T) {
	repo := &videoTestRepo{updatePublishedOK: true}
	svc := NewService(repo, nil, nil, nil, nil, nil, nil, Paths{})

	ok, err := svc.SetVideoPublished(context.Background(), 9, true)
	if err != nil {
		t.Fatalf("SetVideoPublished returned error: %v", err)
	}
	if !ok {
		t.Fatal("expected ok=true")
	}
	if repo.lastUpdatePublishedID != 9 || !repo.lastPublishedValue {
		t.Fatalf("unexpected published call: id=%d value=%v", repo.lastUpdatePublishedID, repo.lastPublishedValue)
	}
}

func TestSetVideoRecommendPassesThroughRepo(t *testing.T) {
	repo := &videoTestRepo{updateRecommendOK: true}
	svc := NewService(repo, nil, nil, nil, nil, nil, nil, Paths{})

	ok, err := svc.SetVideoRecommend(context.Background(), 10, true, 101, 2, 0.95)
	if err != nil {
		t.Fatalf("SetVideoRecommend returned error: %v", err)
	}
	if !ok {
		t.Fatal("expected ok=true")
	}
	if repo.lastUpdateRecommendID != 10 || !repo.lastRecommendValue || repo.lastRecommendUserID != 101 || repo.lastRecommendLevel != 2 || repo.lastRecommendScore != 0.95 {
		t.Fatalf("unexpected recommend call: %+v", repo)
	}
}

func TestGetSimilarVideosPassesThroughRepo(t *testing.T) {
	repo := &videoTestRepo{similarVideos: []domainvideo.Video{{ID: 8}}}
	svc := NewService(repo, nil, nil, nil, nil, nil, nil, Paths{})

	videos, err := svc.GetSimilarVideos(context.Background(), 15, 6)
	if err != nil {
		t.Fatalf("GetSimilarVideos returned error: %v", err)
	}
	if repo.lastFindSimilarID != 15 || repo.lastFindSimilarLimit != 6 {
		t.Fatalf("unexpected similar query: id=%d limit=%d", repo.lastFindSimilarID, repo.lastFindSimilarLimit)
	}
	if len(videos) != 1 || videos[0].ID != 8 {
		t.Fatalf("unexpected similar videos: %+v", videos)
	}
}

func TestSetVideoCoverValidatesAndPassesThroughRepo(t *testing.T) {
	svc := NewService(&videoTestRepo{}, nil, nil, nil, nil, nil, nil, Paths{})

	_, err := svc.SetVideoCover(context.Background(), 1, "   ")
	if err == nil {
		t.Fatal("expected validation error")
	}
	assertValidationMessage(t, err, "cover_url is required")

	repo := &videoTestRepo{updateCoverOK: true}
	svc = NewService(repo, nil, nil, nil, nil, nil, nil, Paths{})
	ok, err := svc.SetVideoCover(context.Background(), 11, "/videos/cover/2026/05/21/cover.jpg")
	if err != nil {
		t.Fatalf("SetVideoCover returned error: %v", err)
	}
	if !ok {
		t.Fatal("expected ok=true")
	}
	if repo.lastUpdateCoverID != 11 || repo.lastUpdateCoverURL != "/videos/cover/2026/05/21/cover.jpg" {
		t.Fatalf("unexpected cover update: id=%d url=%q", repo.lastUpdateCoverID, repo.lastUpdateCoverURL)
	}
}

func TestSubmitVideoReactionValidatesInputs(t *testing.T) {
	svc := NewService(&videoTestRepo{}, nil, nil, nil, nil, nil, nil, Paths{})

	_, _, err := svc.SubmitVideoReaction(context.Background(), 0, 7, VideoReactionLike)
	if err == nil {
		t.Fatal("expected validation error for video id")
	}
	assertValidationMessage(t, err, "video_id is required")

	_, _, err = svc.SubmitVideoReaction(context.Background(), 9, 0, VideoReactionLike)
	if err == nil {
		t.Fatal("expected validation error for user id")
	}
	assertValidationMessage(t, err, "user_id is required")

	_, _, err = svc.SubmitVideoReaction(context.Background(), 9, 7, VideoReactionType("bad"))
	if err == nil {
		t.Fatal("expected validation error for reaction type")
	}
	assertValidationMessage(t, err, "reaction_type must be one of like, double_like, dislike")
}

func TestSubmitVideoReactionPassesThroughRepo(t *testing.T) {
	repo := &videoTestRepo{
		submitReactionActive: true,
		submitReactionOK:     true,
		reactionCountsOK:     true,
	}
	svc := NewService(repo, nil, nil, nil, nil, nil, nil, Paths{})

	result, ok, err := svc.SubmitVideoReaction(context.Background(), 15, 23, VideoReactionDoubleLike)
	if err != nil {
		t.Fatalf("SubmitVideoReaction returned error: %v", err)
	}
	if !ok || !result.Active {
		t.Fatalf("unexpected result: result=%+v ok=%v", result, ok)
	}
	if repo.lastReactionVideoID != 15 || repo.lastReactionUserID != 23 || repo.lastReactionType != VideoReactionDoubleLike {
		t.Fatalf("unexpected repo call: %+v", repo)
	}
}

func TestSubmitVideoReactionUsesReactionStoreWithDBSeed(t *testing.T) {
	repo := &videoTestRepo{
		reactionCounts:        VideoReactionCounts{LikeCount: 5, DoubleLikeCount: 2},
		reactionCountsOK:      true,
		getUserReactionType:   VideoReactionDoubleLike,
		getUserReactionActive: true,
		getUserReactionFound:  true,
	}
	store := &videoTestReactionStore{
		submitResult: VideoReactionResult{
			Active:       true,
			ReactionType: VideoReactionLike,
			Counts:       VideoReactionCounts{LikeCount: 6, DoubleLikeCount: 1},
		},
	}
	svc := NewService(repo, nil, nil, nil, nil, nil, nil, Paths{})
	svc.ReactionStore = store

	result, ok, err := svc.SubmitVideoReaction(context.Background(), 18, 23, VideoReactionLike)
	if err != nil {
		t.Fatalf("SubmitVideoReaction returned error: %v", err)
	}
	if !ok || !result.Active || result.Counts.LikeCount != 6 || result.Counts.DoubleLikeCount != 1 {
		t.Fatalf("unexpected result: result=%+v ok=%v", result, ok)
	}
	if repo.lastReactionVideoID != 0 {
		t.Fatalf("expected no direct SubmitVideoReaction DB call, got video id %d", repo.lastReactionVideoID)
	}
	if store.lastSubmitVideoID != 18 || store.lastSubmitUserID != 23 || store.lastSubmitType != VideoReactionLike {
		t.Fatalf("unexpected store submit call: %+v", store)
	}
	if store.lastSubmitSeed.LikeCount != 5 || store.lastSubmitSeed.DoubleLikeCount != 2 {
		t.Fatalf("unexpected count seed: %+v", store.lastSubmitSeed)
	}
	if store.lastSubmitSeedType != VideoReactionDoubleLike || !store.lastSubmitSeedActive {
		t.Fatalf("unexpected user seed: type=%q active=%v", store.lastSubmitSeedType, store.lastSubmitSeedActive)
	}
}

func TestGetVideoReactionCountsPassesThroughRepo(t *testing.T) {
	repo := &videoTestRepo{
		reactionCounts:   VideoReactionCounts{LikeCount: 4, DoubleLikeCount: 2},
		reactionCountsOK: true,
	}
	svc := NewService(repo, nil, nil, nil, nil, nil, nil, Paths{})

	counts, ok, err := svc.GetVideoReactionCounts(context.Background(), 18)
	if err != nil {
		t.Fatalf("GetVideoReactionCounts returned error: %v", err)
	}
	if !ok {
		t.Fatal("expected ok=true")
	}
	if counts.LikeCount != 4 || counts.DoubleLikeCount != 2 {
		t.Fatalf("unexpected counts: %+v", counts)
	}
	if repo.lastReactionCountID != 18 {
		t.Fatalf("unexpected reaction count id: %d", repo.lastReactionCountID)
	}
}

func TestGetVideoReactionCountsUsesReactionStoreSeededFromRepo(t *testing.T) {
	repo := &videoTestRepo{
		reactionCounts:   VideoReactionCounts{LikeCount: 4, DoubleLikeCount: 2},
		reactionCountsOK: true,
	}
	store := &videoTestReactionStore{
		getCountsResult: VideoReactionCounts{LikeCount: 7, DoubleLikeCount: 3},
	}
	svc := NewService(repo, nil, nil, nil, nil, nil, nil, Paths{})
	svc.ReactionStore = store

	counts, ok, err := svc.GetVideoReactionCounts(context.Background(), 18)
	if err != nil {
		t.Fatalf("GetVideoReactionCounts returned error: %v", err)
	}
	if !ok || counts.LikeCount != 7 || counts.DoubleLikeCount != 3 {
		t.Fatalf("unexpected counts: %+v ok=%v", counts, ok)
	}
	if store.lastGetCountsVideoID != 18 {
		t.Fatalf("unexpected store video id: %d", store.lastGetCountsVideoID)
	}
	if store.lastGetCountsSeed.LikeCount != 4 || store.lastGetCountsSeed.DoubleLikeCount != 2 {
		t.Fatalf("unexpected store seed: %+v", store.lastGetCountsSeed)
	}
}

func TestGetVideoReactionCountsUsesWarmReactionStoreWithoutRepoRead(t *testing.T) {
	repo := &videoTestRepo{}
	store := &videoTestReactionStore{
		hasCounts:       true,
		getCountsResult: VideoReactionCounts{LikeCount: 8, DoubleLikeCount: 4},
	}
	svc := NewService(repo, nil, nil, nil, nil, nil, nil, Paths{})
	svc.ReactionStore = store

	counts, ok, err := svc.GetVideoReactionCounts(context.Background(), 18)
	if err != nil {
		t.Fatalf("GetVideoReactionCounts returned error: %v", err)
	}
	if !ok || counts.LikeCount != 8 || counts.DoubleLikeCount != 4 {
		t.Fatalf("unexpected counts: %+v ok=%v", counts, ok)
	}
	if repo.lastReactionCountID != 0 {
		t.Fatalf("expected no DB count read for warm Redis counts, got video id %d", repo.lastReactionCountID)
	}
}

func TestSubmitSegmentReactionValidatesInputs(t *testing.T) {
	svc := NewService(&videoTestRepo{}, nil, nil, nil, nil, nil, nil, Paths{})

	_, _, err := svc.SubmitSegmentReaction(context.Background(), 0, 7, VideoReactionLike)
	if err == nil {
		t.Fatal("expected validation error for segment id")
	}
	assertValidationMessage(t, err, "segment_id is required")

	_, _, err = svc.SubmitSegmentReaction(context.Background(), 9, 0, VideoReactionLike)
	if err == nil {
		t.Fatal("expected validation error for user id")
	}
	assertValidationMessage(t, err, "user_id is required")

	_, _, err = svc.SubmitSegmentReaction(context.Background(), 9, 7, VideoReactionType("bad"))
	if err == nil {
		t.Fatal("expected validation error for reaction type")
	}
	assertValidationMessage(t, err, "reaction_type must be one of like, double_like, dislike")
}

func TestSubmitSegmentReactionPassesThroughRepo(t *testing.T) {
	repo := &videoTestRepo{
		submitSegmentReactionActive: true,
		submitSegmentReactionOK:     true,
		segmentReactionCountsOK:     true,
	}
	svc := NewService(repo, nil, nil, nil, nil, nil, nil, Paths{})

	result, ok, err := svc.SubmitSegmentReaction(context.Background(), 25, 23, VideoReactionDoubleLike)
	if err != nil {
		t.Fatalf("SubmitSegmentReaction returned error: %v", err)
	}
	if !ok || !result.Active {
		t.Fatalf("unexpected result: result=%+v ok=%v", result, ok)
	}
	if repo.lastReactionSegmentID != 25 || repo.lastReactionUserID != 23 || repo.lastReactionType != VideoReactionDoubleLike {
		t.Fatalf("unexpected repo call: %+v", repo)
	}
}

func TestSubmitSegmentReactionUsesReactionStoreWithDBSeed(t *testing.T) {
	repo := &videoTestRepo{
		segmentReactionCounts:    VideoReactionCounts{LikeCount: 5, DoubleLikeCount: 2},
		segmentReactionCountsOK:  true,
		getSegmentReactionType:   VideoReactionDoubleLike,
		getSegmentReactionActive: true,
		getSegmentReactionFound:  true,
	}
	store := &videoTestReactionStore{
		submitResult: VideoReactionResult{
			Active:       true,
			ReactionType: VideoReactionLike,
			Counts:       VideoReactionCounts{LikeCount: 6, DoubleLikeCount: 1},
		},
	}
	svc := NewService(repo, nil, nil, nil, nil, nil, nil, Paths{})
	svc.SegmentReactionStore = store

	result, ok, err := svc.SubmitSegmentReaction(context.Background(), 28, 23, VideoReactionLike)
	if err != nil {
		t.Fatalf("SubmitSegmentReaction returned error: %v", err)
	}
	if !ok || !result.Active || result.Counts.LikeCount != 6 || result.Counts.DoubleLikeCount != 1 {
		t.Fatalf("unexpected result: result=%+v ok=%v", result, ok)
	}
	if repo.lastReactionSegmentID != 0 {
		t.Fatalf("expected no direct SubmitSegmentReaction DB call, got segment id %d", repo.lastReactionSegmentID)
	}
	if store.lastSubmitVideoID != 28 || store.lastSubmitUserID != 23 || store.lastSubmitType != VideoReactionLike {
		t.Fatalf("unexpected store submit call: %+v", store)
	}
	if store.lastSubmitSeed.LikeCount != 5 || store.lastSubmitSeed.DoubleLikeCount != 2 {
		t.Fatalf("unexpected count seed: %+v", store.lastSubmitSeed)
	}
	if store.lastSubmitSeedType != VideoReactionDoubleLike || !store.lastSubmitSeedActive {
		t.Fatalf("unexpected user seed: type=%q active=%v", store.lastSubmitSeedType, store.lastSubmitSeedActive)
	}
}

func TestGetSegmentReactionCountsUsesReactionStoreSeededFromRepo(t *testing.T) {
	repo := &videoTestRepo{
		segmentReactionCounts:   VideoReactionCounts{LikeCount: 4, DoubleLikeCount: 2},
		segmentReactionCountsOK: true,
	}
	store := &videoTestReactionStore{
		getCountsResult: VideoReactionCounts{LikeCount: 7, DoubleLikeCount: 3},
	}
	svc := NewService(repo, nil, nil, nil, nil, nil, nil, Paths{})
	svc.SegmentReactionStore = store

	counts, ok, err := svc.GetSegmentReactionCounts(context.Background(), 28)
	if err != nil {
		t.Fatalf("GetSegmentReactionCounts returned error: %v", err)
	}
	if !ok || counts.LikeCount != 7 || counts.DoubleLikeCount != 3 {
		t.Fatalf("unexpected counts: %+v ok=%v", counts, ok)
	}
	if store.lastGetCountsVideoID != 28 {
		t.Fatalf("unexpected store segment id: %d", store.lastGetCountsVideoID)
	}
	if store.lastGetCountsSeed.LikeCount != 4 || store.lastGetCountsSeed.DoubleLikeCount != 2 {
		t.Fatalf("unexpected store seed: %+v", store.lastGetCountsSeed)
	}
}

func assertValidationMessage(t *testing.T, err error, want string) {
	t.Helper()
	var validationErr ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("expected ValidationError, got %T", err)
	}
	if validationErr.Error() != want {
		t.Fatalf("unexpected validation message: %q", validationErr.Error())
	}
}

type videoTestRepo struct {
	listVideos                  []domainvideo.Video
	listErr                     error
	lastListFilter              ListFilter
	listRecommendPoolCalls      int
	recommendPool               []domainvideo.Video
	deleteOK                    bool
	deleteErr                   error
	lastDeleteID                uint64
	updateMetadataOK            bool
	updateMetadataErr           error
	lastUpdateMetadataID        uint64
	lastUpdateTitle             string
	lastUpdateDescription       string
	updatePublishedOK           bool
	updatePublishedErr          error
	lastUpdatePublishedID       uint64
	lastPublishedValue          bool
	updateRecommendOK           bool
	updateRecommendErr          error
	lastUpdateRecommendID       uint64
	lastRecommendValue          bool
	lastRecommendUserID         uint64
	lastRecommendLevel          int16
	lastRecommendScore          float64
	similarVideos               []domainvideo.Video
	findSimilarErr              error
	lastFindSimilarID           uint64
	lastFindSimilarLimit        int
	updateCoverOK               bool
	updateCoverErr              error
	lastUpdateCoverID           uint64
	lastUpdateCoverURL          string
	submitReactionActive        bool
	submitReactionOK            bool
	submitReactionErr           error
	lastReactionVideoID         uint64
	lastReactionSegmentID       uint64
	lastReactionUserID          uint64
	lastReactionType            VideoReactionType
	reactionCounts              VideoReactionCounts
	reactionCountsOK            bool
	reactionCountsErr           error
	lastReactionCountID         uint64
	lastSegmentReactionCountID  uint64
	getUserReactionType         VideoReactionType
	getUserReactionActive       bool
	getUserReactionFound        bool
	lastUserReactionVideoID     uint64
	lastUserReactionUserID      uint64
	submitSegmentReactionActive bool
	submitSegmentReactionOK     bool
	submitSegmentReactionErr    error
	segmentReactionCounts       VideoReactionCounts
	segmentReactionCountsOK     bool
	segmentReactionCountsErr    error
	getSegmentReactionType      VideoReactionType
	getSegmentReactionActive    bool
	getSegmentReactionFound     bool
	lastSegmentUserReactionID   uint64
	lastSegmentUserID           uint64
}

type videoTestReactionStore struct {
	hasCounts                  bool
	hasUserReaction            bool
	getUserReactionType        VideoReactionType
	getUserReactionActive      bool
	getUserReactionFound       bool
	submitResult               VideoReactionResult
	getCountsResult            VideoReactionCounts
	lastSubmitVideoID          uint64
	lastSubmitUserID           uint64
	lastSubmitType             VideoReactionType
	lastSubmitSeed             VideoReactionCounts
	lastSubmitSeedType         VideoReactionType
	lastSubmitSeedActive       bool
	lastGetCountsVideoID       uint64
	lastGetCountsSeed          VideoReactionCounts
	lastGetUserReactionVideoID uint64
	lastGetUserReactionUserID  uint64
}

func (*videoTestRepo) Create(context.Context, *domainvideo.Video) error { panic("unexpected call") }
func (r *videoTestRepo) List(context.Context, ListFilter) ([]domainvideo.Video, error) {
	r.lastListFilter = ListHLSOnly
	return r.listVideos, r.listErr
}
func (r *videoTestRepo) ListRecommendPool(context.Context) ([]domainvideo.Video, error) {
	r.listRecommendPoolCalls++
	return r.recommendPool, nil
}
func (*videoTestRepo) GetByID(context.Context, uint64) (domainvideo.Video, bool, error) {
	panic("unexpected call")
}
func (r *videoTestRepo) DeleteByID(context.Context, uint64) (bool, error) {
	r.lastDeleteID = 12
	return r.deleteOK, r.deleteErr
}
func (r *videoTestRepo) UpdateMetadata(_ context.Context, id uint64, title string, description string) (bool, error) {
	r.lastUpdateMetadataID = id
	r.lastUpdateTitle = title
	r.lastUpdateDescription = description
	return r.updateMetadataOK, r.updateMetadataErr
}
func (r *videoTestRepo) UpdatePublished(_ context.Context, id uint64, isPublished bool) (bool, error) {
	r.lastUpdatePublishedID = id
	r.lastPublishedValue = isPublished
	return r.updatePublishedOK, r.updatePublishedErr
}
func (r *videoTestRepo) UpdateRecommend(_ context.Context, id uint64, isRecommend bool, userID uint64, recommendLevel int16, recommendScore float64) (bool, error) {
	r.lastUpdateRecommendID = id
	r.lastRecommendValue = isRecommend
	r.lastRecommendUserID = userID
	r.lastRecommendLevel = recommendLevel
	r.lastRecommendScore = recommendScore
	return r.updateRecommendOK, r.updateRecommendErr
}
func (*videoTestRepo) IncrementViewCount(context.Context, uint64) (int, bool, error) {
	panic("unexpected call")
}
func (*videoTestRepo) GetViewCount(context.Context, uint64) (int, bool, error) {
	panic("unexpected call")
}
func (r *videoTestRepo) FindSimilar(_ context.Context, id uint64, limit int) ([]domainvideo.Video, error) {
	r.lastFindSimilarID = id
	r.lastFindSimilarLimit = limit
	return r.similarVideos, r.findSimilarErr
}
func (r *videoTestRepo) UpdateCoverByID(_ context.Context, id uint64, coverURL string) (bool, error) {
	r.lastUpdateCoverID = id
	r.lastUpdateCoverURL = coverURL
	return r.updateCoverOK, r.updateCoverErr
}
func (r *videoTestRepo) SubmitVideoReaction(_ context.Context, videoID uint64, userID uint64, reactionType VideoReactionType) (bool, bool, error) {
	r.lastReactionVideoID = videoID
	r.lastReactionUserID = userID
	r.lastReactionType = reactionType
	return r.submitReactionActive, r.submitReactionOK, r.submitReactionErr
}
func (*videoTestRepo) ApplyVideoReactionState(context.Context, uint64, uint64, VideoReactionType, bool) (bool, error) {
	panic("unexpected call")
}
func (r *videoTestRepo) SubmitSegmentReaction(_ context.Context, segmentID uint64, userID uint64, reactionType VideoReactionType) (bool, bool, error) {
	r.lastReactionSegmentID = segmentID
	r.lastReactionUserID = userID
	r.lastReactionType = reactionType
	return r.submitSegmentReactionActive, r.submitSegmentReactionOK, r.submitSegmentReactionErr
}
func (*videoTestRepo) ApplySegmentReactionState(context.Context, uint64, uint64, VideoReactionType, bool) (bool, error) {
	panic("unexpected call")
}
func (r *videoTestRepo) GetVideoUserReaction(_ context.Context, videoID uint64, userID uint64) (VideoReactionType, bool, bool, error) {
	r.lastUserReactionVideoID = videoID
	r.lastUserReactionUserID = userID
	return r.getUserReactionType, r.getUserReactionActive, r.getUserReactionFound, nil
}
func (r *videoTestRepo) GetVideoReactionCounts(_ context.Context, videoID uint64) (VideoReactionCounts, bool, error) {
	r.lastReactionCountID = videoID
	return r.reactionCounts, r.reactionCountsOK, r.reactionCountsErr
}
func (r *videoTestRepo) GetSegmentUserReaction(_ context.Context, segmentID uint64, userID uint64) (VideoReactionType, bool, bool, error) {
	r.lastSegmentUserReactionID = segmentID
	r.lastSegmentUserID = userID
	return r.getSegmentReactionType, r.getSegmentReactionActive, r.getSegmentReactionFound, nil
}
func (r *videoTestRepo) GetSegmentReactionCounts(_ context.Context, segmentID uint64) (VideoReactionCounts, bool, error) {
	r.lastSegmentReactionCountID = segmentID
	return r.segmentReactionCounts, r.segmentReactionCountsOK, r.segmentReactionCountsErr
}
func (*videoTestRepo) UpdateStatusByID(context.Context, uint64, domainvideo.Status, string) error {
	panic("unexpected call")
}
func (*videoTestRepo) GetSegmentEmbeddingDim(context.Context) (int, error) { panic("unexpected call") }
func (*videoTestRepo) GetQuestionEmbeddingTextByID(context.Context, uint64) (string, error) {
	panic("unexpected call")
}
func (*videoTestRepo) ListQuestions(context.Context, int, int) (QuestionPage, error) {
	panic("unexpected call")
}
func (*videoTestRepo) GetQuestionByID(context.Context, uint64) (QuestionItem, bool, error) {
	panic("unexpected call")
}
func (*videoTestRepo) FindRecommendedSegments(context.Context, pgvector.Vector, int) ([]RecommendCandidate, error) {
	panic("unexpected call")
}
func (*videoTestRepo) FindRecommendedSegmentsByWeakKnowledge(context.Context, uint64, int, int) ([]RecommendCandidate, error) {
	return nil, nil
}
func (*videoTestRepo) SaveUserVideoRecommendation(context.Context, uint64, uint64, uint64, uint64, float64, time.Time) error {
	panic("unexpected call")
}
func (*videoTestRepo) ListRecommendations(context.Context, uint64, uint64, int) ([]RecommendationRecord, error) {
	panic("unexpected call")
}
func (*videoTestRepo) GetVideoIDBySegmentID(context.Context, uint64) (uint64, error) {
	panic("unexpected call")
}
func (*videoTestRepo) HasWatchedVideoForQuestion(context.Context, uint64, uint64, uint64) (bool, error) {
	panic("unexpected call")
}
func (*videoTestRepo) SaveWatchRecord(context.Context, uint64, uint64, uint64, uint64, bool, int, time.Time) (bool, error) {
	panic("unexpected call")
}
func (s *videoTestReactionStore) HasCounts(context.Context, uint64) (bool, error) {
	return s.hasCounts, nil
}

func (s *videoTestReactionStore) HasUserReaction(context.Context, uint64, uint64) (bool, error) {
	return s.hasUserReaction, nil
}

func (s *videoTestReactionStore) GetUserReaction(_ context.Context, videoID uint64, userID uint64) (VideoReactionType, bool, bool, error) {
	s.lastGetUserReactionVideoID = videoID
	s.lastGetUserReactionUserID = userID
	return s.getUserReactionType, s.getUserReactionActive, s.getUserReactionFound, nil
}

func (s *videoTestReactionStore) Submit(_ context.Context, videoID uint64, userID uint64, reactionType VideoReactionType, seed VideoReactionCounts, seedUserReaction VideoReactionType, seedUserActive bool) (VideoReactionResult, error) {
	s.lastSubmitVideoID = videoID
	s.lastSubmitUserID = userID
	s.lastSubmitType = reactionType
	s.lastSubmitSeed = seed
	s.lastSubmitSeedType = seedUserReaction
	s.lastSubmitSeedActive = seedUserActive
	return s.submitResult, nil
}

func (s *videoTestReactionStore) GetCounts(_ context.Context, videoID uint64, seed VideoReactionCounts) (VideoReactionCounts, error) {
	s.lastGetCountsVideoID = videoID
	s.lastGetCountsSeed = seed
	return s.getCountsResult, nil
}
