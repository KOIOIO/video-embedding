package videoapp

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/pgvector/pgvector-go"

	recommendationapp "nlp-video-analysis/internal/application/videoapp/recommendation"
)

var ErrVideoSegmentNotFound = errors.New("video_segment not found")

type RandomPlayableSegmentRepository interface {
	FindRandomPlayableSegment(ctx context.Context) (RecommendResultItem, bool, error)
}

type RandomPlayableSegmentExclusionRepository interface {
	FindRandomPlayableSegmentExcluding(ctx context.Context, excludedSegmentIDs []uint64) (RecommendResultItem, bool, error)
}

// RecommendByQuestion 根据题目向量召回最相关的视频片段，并记录推荐结果。
func (s *Service) RecommendByQuestion(ctx context.Context, input RecommendByQuestionInput) ([]RecommendResultItem, error) {
	items, err := newRecommendationService(s).RecommendByQuestion(ctx, recommendationapp.RecommendByQuestionInput{
		QuestionID:   input.QuestionID,
		QuestionText: input.QuestionText,
		UserID:       input.UserID,
		Limit:        input.Limit,
	})
	if err != nil {
		return nil, err
	}
	return mapRecommendItemsFromApp(items), nil
}

// ListRecommendations 查询用户历史推荐结果。
func (s *Service) ListRecommendations(ctx context.Context, input ListRecommendationsInput) ([]RecommendResultItem, error) {
	rows, err := newRecommendationService(s).ListRecommendations(ctx, recommendationapp.ListRecommendationsInput{
		QuestionID: input.QuestionID,
		UserID:     input.UserID,
		Limit:      input.Limit,
	})
	if err != nil {
		return nil, err
	}
	return mapRecommendItemsFromApp(rows), nil
}

// ReportWatch 记录用户对推荐片段的观看情况。
func (s *Service) ReportWatch(ctx context.Context, input ReportWatchInput) error {
	return newRecommendationService(s).ReportWatch(ctx, recommendationapp.ReportWatchInput{
		QuestionID:     input.QuestionID,
		UserID:         input.UserID,
		VideoSegmentID: input.VideoSegmentID,
		IsWatched:      input.IsWatched,
		WatchDuration:  input.WatchDuration,
	})
}

// RandomPlayVideoSegment 返回一个可播放且未删除、已发布的视频片段。
// 有 user_id 时先按配置的推荐引擎召回，未命中时回退到随机片段。
func (s *Service) RandomPlayVideoSegment(ctx context.Context, input RandomPlayVideoSegmentInput) (RecommendResultItem, bool, error) {
	recSvc := newRecommendationService(s)
	items, err := recSvc.RandomPlay(ctx, recommendationapp.RandomPlayInput{
		UserID: input.UserID,
		Limit:  input.Limit,
	})
	if err != nil {
		return RecommendResultItem{}, false, err
	}
	if len(items) > 0 {
		return mapRecommendItemsFromApp(items)[0], true, nil
	}

	randomRepo, ok := s.Repo.(RandomPlayableSegmentRepository)
	if !ok {
		return RecommendResultItem{}, false, nil
	}
	item, found, err := s.findRandomPlayableSegment(ctx, randomRepo, input.UserID)
	if err != nil || !found {
		return item, found, err
	}
	if input.UserID > 0 {
		now := time.Now()
		if s.Now != nil {
			now = s.Now()
		}
		if err := s.Repo.SaveUserVideoRecommendation(ctx, input.UserID, 0, item.VideoID, item.VideoSegmentID, item.RecommendScore, now); err != nil {
			return RecommendResultItem{}, false, err
		}
		if exposureRepo, ok := s.Repo.(RecommendationExposureRepository); ok {
			if err := exposureRepo.SaveRecommendationExposures(ctx, []RecommendationExposure{{
				RequestID:      "random-play-" + strconv.FormatInt(now.UnixNano(), 10),
				UserID:         input.UserID,
				QuestionID:     0,
				VideoID:        item.VideoID,
				VideoSegmentID: item.VideoSegmentID,
				Rank:           1,
				Score:          item.RecommendScore,
				Strategy:       RecommendStrategyRandomPlay,
				Now:            now,
			}}); err != nil {
				return RecommendResultItem{}, false, err
			}
		}
	}
	newRecommendationService(s).MarkRecentReturned(ctx, input.UserID, item.VideoSegmentID)
	return item, true, nil
}

// ExternalTwoTowerItemIDs returns two-tower candidate segment IDs for Gorse external recommenders.
// It is intentionally side-effect free: final exposure and recommendation records are written
// by the normal Gorse-backed random-play path after Gorse selects the returned candidates.
func (s *Service) ExternalTwoTowerItemIDs(ctx context.Context, input RandomPlayVideoSegmentInput) ([]uint64, error) {
	return newRecommendationService(s).RecommendTwoTowerItemIDs(ctx, input.UserID, input.Limit)
}

func (s *Service) findRandomPlayableSegment(ctx context.Context, repo RandomPlayableSegmentRepository, userID uint64) (RecommendResultItem, bool, error) {
	if userID == 0 || s.RecentSegments == nil {
		return repo.FindRandomPlayableSegment(ctx)
	}
	excludedIDs, err := s.RecentSegments.ListRecent(ctx, userID)
	if err != nil || len(excludedIDs) == 0 {
		return repo.FindRandomPlayableSegment(ctx)
	}
	if excludingRepo, ok := s.Repo.(RandomPlayableSegmentExclusionRepository); ok {
		item, found, err := excludingRepo.FindRandomPlayableSegmentExcluding(ctx, excludedIDs)
		if err != nil || found {
			return item, found, err
		}
	}
	return repo.FindRandomPlayableSegment(ctx)
}

// buildQuestionVector 根据题目 ID 或题目文本生成查询向量。
// 优先复用题库已存 embedding，回退到实时 Embedding 服务。
func (s *Service) buildQuestionVector(ctx context.Context, questionID uint64, questionText string, targetDim int) (pgvector.Vector, error) {
	return newRecommendationService(s).BuildQuestionVector(ctx, questionID, questionText, targetDim)
}

// parseVectorText 解析数据库中保存的向量文本表示。
func parseVectorText(text string) ([]float32, error) {
	return recommendationapp.ParseVectorText(text)
}

// normalizeVectorDim 对查询向量做裁剪或补零，使其与片段 embedding 维度一致。
func normalizeVectorDim(v []float32, dim int) []float32 {
	return recommendationapp.NormalizeVectorDim(v, dim)
}

func newRecommendationService(s *Service) recommendationapp.Service {
	return recommendationapp.Service{
		Repo:                  recommendationRepositoryAdapter{repo: s.Repo},
		Embedder:              s.Embedder,
		Gorse:                 s.GorseClient,
		Engine:                s.RecommendationEngine,
		GorseOptions:          s.GorseOptions,
		Now:                   s.Now,
		RecentSegments:        s.RecentSegments,
		RecentSegmentTTL:      s.RecentSegmentTTL,
		InvalidArgument:       InvalidArgumentError,
		IsProviderUnavailable: IsAIProviderUnavailable,
		NewDegradedError: func(reason string, items []recommendationapp.ResultItem) error {
			return DegradedError{Reason: reason, Items: mapRecommendItemsFromApp(items)}
		},
		ErrVideoSegmentAbsent: ErrVideoSegmentNotFound,
		ProfileUpdater:        s.ProfileUpdater,
		UserTowerUpdater:      userTowerUpdater(s),
	}
}

func userTowerUpdater(s *Service) recommendationapp.UserTowerUpdater {
	if s == nil || s.Repo == nil {
		return nil
	}
	if updater, ok := s.Repo.(UserTowerEmbeddingUpdater); ok {
		return updater
	}
	return nil
}

type recommendationRepositoryAdapter struct {
	repo VideoRepository
}

func (a recommendationRepositoryAdapter) GetSegmentEmbeddingDim(ctx context.Context) (int, error) {
	return a.repo.GetSegmentEmbeddingDim(ctx)
}

func (a recommendationRepositoryAdapter) GetQuestionEmbeddingTextByID(ctx context.Context, questionID uint64) (string, error) {
	return a.repo.GetQuestionEmbeddingTextByID(ctx, questionID)
}

func (a recommendationRepositoryAdapter) FindRecommendedSegments(ctx context.Context, query pgvector.Vector, limit int) ([]recommendationapp.Candidate, error) {
	items, err := a.repo.FindRecommendedSegments(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	out := make([]recommendationapp.Candidate, 0, len(items))
	for _, item := range items {
		out = append(out, recommendationapp.Candidate{
			VideoSegmentID: item.VideoSegmentID,
			VideoID:        item.VideoID,
			StartTimeSec:   item.StartTimeSec,
			EndTimeSec:     item.EndTimeSec,
			Distance:       item.Distance,
			SegmentTitle:   item.SegmentTitle,
			KnowledgeTags:  item.KnowledgeTags,
			VideoTitle:     item.VideoTitle,
			Description:    item.Description,
			VideoURL:       item.VideoURL,
			CoverURL:       item.CoverURL,
			Status:         item.Status,
			IsPublished:    item.IsPublished,
			IsRecommend:    item.IsRecommend,
			ViewCount:      item.ViewCount,
			CreateTime:     item.CreateTime,
			UpdateTime:     item.UpdateTime,
		})
	}
	return out, nil
}

func (a recommendationRepositoryAdapter) GetUserVideoProfile(ctx context.Context, userID uint64, modelVersion string) (recommendationapp.UserVideoProfile, bool, error) {
	profileRepo, ok := a.repo.(VideoProfileRepository)
	if !ok {
		return recommendationapp.UserVideoProfile{}, false, nil
	}
	profile, found, err := profileRepo.GetUserVideoProfile(ctx, userID, modelVersion)
	if err != nil || !found {
		return recommendationapp.UserVideoProfile{}, found, err
	}
	return recommendationapp.UserVideoProfile{
		UserID:        profile.UserID,
		ProfileVector: profile.ProfileVector,
		ModelVersion:  profile.ModelVersion,
		Status:        profile.Status,
		PositiveCount: profile.PositiveCount,
	}, true, nil
}

func (a recommendationRepositoryAdapter) FindRecommendedSegmentsForProfileRerank(ctx context.Context, input recommendationapp.ProfileRerankQuery) ([]recommendationapp.ProfileCandidate, error) {
	profileRepo, ok := a.repo.(VideoProfileRepository)
	if !ok {
		return nil, nil
	}
	items, err := profileRepo.FindRecommendedSegmentsForProfileRerank(ctx, ProfileRerankQuery{
		UserID:         input.UserID,
		QuestionVector: input.QuestionVector,
		ProfileVector:  input.ProfileVector,
		Limit:          input.Limit,
	})
	if err != nil {
		return nil, err
	}
	out := make([]recommendationapp.ProfileCandidate, 0, len(items))
	for _, item := range items {
		out = append(out, recommendationapp.ProfileCandidate{
			Candidate: recommendationapp.Candidate{
				VideoSegmentID: item.VideoSegmentID,
				VideoID:        item.VideoID,
				StartTimeSec:   item.StartTimeSec,
				EndTimeSec:     item.EndTimeSec,
				Distance:       item.Distance,
				SegmentTitle:   item.SegmentTitle,
				KnowledgeTags:  item.KnowledgeTags,
				VideoTitle:     item.VideoTitle,
				Description:    item.Description,
				VideoURL:       item.VideoURL,
				CoverURL:       item.CoverURL,
				Status:         item.Status,
				IsPublished:    item.IsPublished,
				IsRecommend:    item.IsRecommend,
				ViewCount:      item.ViewCount,
				CreateTime:     item.CreateTime,
				UpdateTime:     item.UpdateTime,
			},
			ProfileDistance:   item.ProfileDistance,
			LikeCount:         item.LikeCount,
			DoubleLikeCount:   item.DoubleLikeCount,
			UserDisliked:      item.UserDisliked,
			UserVideoDisliked: item.UserVideoDisliked,
			UserWatched:       item.UserWatched,
		})
	}
	return out, nil
}

func (a recommendationRepositoryAdapter) GetUserTowerEmbedding(ctx context.Context, userID uint64, modelVersion string) (recommendationapp.UserTowerEmbedding, bool, error) {
	twoTowerRepo, ok := a.repo.(TwoTowerRepository)
	if !ok {
		return recommendationapp.UserTowerEmbedding{}, false, nil
	}
	embedding, found, err := twoTowerRepo.GetUserTowerEmbedding(ctx, userID, modelVersion)
	if err != nil || !found {
		return recommendationapp.UserTowerEmbedding{}, found, err
	}
	return recommendationapp.UserTowerEmbedding{
		UserID:       embedding.UserID,
		Vector:       embedding.Vector,
		ModelVersion: embedding.ModelVersion,
		Status:       embedding.Status,
	}, true, nil
}

func (a recommendationRepositoryAdapter) FindRecommendedSegmentsForTwoTower(ctx context.Context, input recommendationapp.TwoTowerQuery) ([]recommendationapp.TwoTowerCandidate, error) {
	twoTowerRepo, ok := a.repo.(TwoTowerRepository)
	if !ok {
		return nil, nil
	}
	items, err := twoTowerRepo.FindRecommendedSegmentsForTwoTower(ctx, TwoTowerQuery{
		UserID:       input.UserID,
		UserVector:   input.UserVector,
		ModelVersion: input.ModelVersion,
		Limit:        input.Limit,
	})
	if err != nil {
		return nil, err
	}
	out := make([]recommendationapp.TwoTowerCandidate, 0, len(items))
	for _, item := range items {
		out = append(out, recommendationapp.TwoTowerCandidate{
			Candidate: recommendationapp.Candidate{
				VideoSegmentID: item.VideoSegmentID,
				VideoID:        item.VideoID,
				StartTimeSec:   item.StartTimeSec,
				EndTimeSec:     item.EndTimeSec,
				Distance:       item.Distance,
				SegmentTitle:   item.SegmentTitle,
				KnowledgeTags:  item.KnowledgeTags,
				VideoTitle:     item.VideoTitle,
				Description:    item.Description,
				VideoURL:       item.VideoURL,
				CoverURL:       item.CoverURL,
				Status:         item.Status,
				IsPublished:    item.IsPublished,
				IsRecommend:    item.IsRecommend,
				ViewCount:      item.ViewCount,
				CreateTime:     item.CreateTime,
				UpdateTime:     item.UpdateTime,
			},
		})
	}
	return out, nil
}

func (a recommendationRepositoryAdapter) FindRecommendedSegmentsByWeakKnowledge(ctx context.Context, userID uint64, limit int, weakLimit int) ([]recommendationapp.Candidate, error) {
	items, err := a.repo.FindRecommendedSegmentsByWeakKnowledge(ctx, userID, limit, weakLimit)
	if err != nil {
		return nil, err
	}
	return mapRecommendCandidatesToRecommendation(items), nil
}

func (a recommendationRepositoryAdapter) ListWeakKnowledge(ctx context.Context, userID uint64, limit int) ([]recommendationapp.WeakKnowledge, error) {
	repo, ok := a.repo.(WeakKnowledgeVectorRepository)
	if !ok {
		return nil, nil
	}
	items, err := repo.ListWeakKnowledge(ctx, userID, limit)
	if err != nil {
		return nil, err
	}
	out := make([]recommendationapp.WeakKnowledge, 0, len(items))
	for _, item := range items {
		out = append(out, recommendationapp.WeakKnowledge{
			KnowledgePointID: item.KnowledgePointID,
			Mastery:          item.Mastery,
			Name:             item.Name,
			Description:      item.Description,
		})
	}
	return out, nil
}

func (a recommendationRepositoryAdapter) FindRecommendedSegmentsByWeakKnowledgeVector(ctx context.Context, input recommendationapp.WeakKnowledgeVectorQuery) ([]recommendationapp.Candidate, error) {
	repo, ok := a.repo.(WeakKnowledgeVectorRepository)
	if !ok {
		return nil, nil
	}
	items, err := repo.FindRecommendedSegmentsByWeakKnowledgeVector(ctx, WeakKnowledgeVectorQuery{
		UserID:           input.UserID,
		Query:            input.Query,
		Limit:            input.Limit,
		RequireRecommend: input.RequireRecommend,
	})
	if err != nil {
		return nil, err
	}
	return mapRecommendCandidatesToRecommendation(items), nil
}

func mapRecommendCandidatesToRecommendation(items []RecommendCandidate) []recommendationapp.Candidate {
	out := make([]recommendationapp.Candidate, 0, len(items))
	for _, item := range items {
		out = append(out, recommendationapp.Candidate{
			VideoSegmentID: item.VideoSegmentID,
			VideoID:        item.VideoID,
			StartTimeSec:   item.StartTimeSec,
			EndTimeSec:     item.EndTimeSec,
			Distance:       item.Distance,
			SegmentTitle:   item.SegmentTitle,
			KnowledgeTags:  item.KnowledgeTags,
			VideoTitle:     item.VideoTitle,
			Description:    item.Description,
			VideoURL:       item.VideoURL,
			CoverURL:       item.CoverURL,
			Status:         item.Status,
			IsPublished:    item.IsPublished,
			IsRecommend:    item.IsRecommend,
			ViewCount:      item.ViewCount,
			CreateTime:     item.CreateTime,
			UpdateTime:     item.UpdateTime,
		})
	}
	return out
}

func (a recommendationRepositoryAdapter) HydrateRecommendedSegmentsByID(ctx context.Context, userID uint64, ids []uint64) ([]recommendationapp.Candidate, error) {
	gorseRepo, ok := a.repo.(GorseHydrationRepository)
	if !ok {
		return nil, nil
	}
	items, err := gorseRepo.HydrateRecommendedSegmentsByID(ctx, userID, ids)
	if err != nil {
		return nil, err
	}
	out := make([]recommendationapp.Candidate, 0, len(items))
	for _, item := range items {
		out = append(out, recommendationapp.Candidate{
			VideoSegmentID: item.VideoSegmentID,
			VideoID:        item.VideoID,
			StartTimeSec:   item.StartTimeSec,
			EndTimeSec:     item.EndTimeSec,
			Distance:       item.Distance,
			SegmentTitle:   item.SegmentTitle,
			KnowledgeTags:  item.KnowledgeTags,
			VideoTitle:     item.VideoTitle,
			Description:    item.Description,
			VideoURL:       item.VideoURL,
			CoverURL:       item.CoverURL,
			Status:         item.Status,
			IsPublished:    item.IsPublished,
			IsRecommend:    item.IsRecommend,
			ViewCount:      item.ViewCount,
			CreateTime:     item.CreateTime,
			UpdateTime:     item.UpdateTime,
		})
	}
	return out, nil
}

func (a recommendationRepositoryAdapter) GetActiveTwoTowerModelVersion(ctx context.Context) (string, bool, error) {
	versionRepo, ok := a.repo.(TwoTowerModelVersionRepository)
	if !ok {
		return "", false, nil
	}
	return versionRepo.GetActiveTwoTowerModelVersion(ctx)
}

func (a recommendationRepositoryAdapter) SaveUserVideoRecommendation(ctx context.Context, userID uint64, questionID uint64, videoID uint64, segmentID uint64, score float64, now time.Time) error {
	return a.repo.SaveUserVideoRecommendation(ctx, userID, questionID, videoID, segmentID, score, now)
}

func (a recommendationRepositoryAdapter) SaveRecommendationExposures(ctx context.Context, exposures []recommendationapp.ExposureRecord) error {
	exposureRepo, ok := a.repo.(RecommendationExposureRepository)
	if !ok {
		return nil
	}
	rows := make([]RecommendationExposure, 0, len(exposures))
	for _, exposure := range exposures {
		rows = append(rows, RecommendationExposure{
			RequestID:      exposure.RequestID,
			UserID:         exposure.UserID,
			QuestionID:     exposure.QuestionID,
			VideoID:        exposure.VideoID,
			VideoSegmentID: exposure.VideoSegmentID,
			Rank:           exposure.Rank,
			Score:          exposure.Score,
			Strategy:       exposure.Strategy,
			ModelVersion:   exposure.ModelVersion,
			Now:            exposure.Now,
		})
	}
	return exposureRepo.SaveRecommendationExposures(ctx, rows)
}

func (a recommendationRepositoryAdapter) ListRecommendations(ctx context.Context, userID uint64, questionID uint64, limit int) ([]recommendationapp.Record, error) {
	rows, err := a.repo.ListRecommendations(ctx, userID, questionID, limit)
	if err != nil {
		return nil, err
	}
	out := make([]recommendationapp.Record, 0, len(rows))
	for _, row := range rows {
		out = append(out, recommendationapp.Record{
			QuestionID:     row.QuestionID,
			VideoID:        row.VideoID,
			VideoSegmentID: row.VideoSegmentID,
			RecommendScore: row.RecommendScore,
			IsWatched:      row.IsWatched,
			WatchDuration:  row.WatchDuration,
			StartTimeSec:   row.StartTimeSec,
			EndTimeSec:     row.EndTimeSec,
			Title:          row.Title,
			VideoURL:       row.VideoURL,
			CoverURL:       row.CoverURL,
			Status:         row.Status,
			IsPublished:    row.IsPublished,
			IsRecommend:    row.IsRecommend,
			ViewCount:      row.ViewCount,
			CreateTime:     row.CreateTime,
			UpdateTime:     row.UpdateTime,
		})
	}
	return out, nil
}

func (a recommendationRepositoryAdapter) GetVideoIDBySegmentID(ctx context.Context, segmentID uint64) (uint64, error) {
	return a.repo.GetVideoIDBySegmentID(ctx, segmentID)
}

func (a recommendationRepositoryAdapter) HasWatchedVideoForQuestion(ctx context.Context, userID uint64, questionID uint64, videoID uint64) (bool, error) {
	return a.repo.HasWatchedVideoForQuestion(ctx, userID, questionID, videoID)
}

func (a recommendationRepositoryAdapter) SaveWatchRecord(ctx context.Context, userID uint64, videoID uint64, questionID uint64, segmentID uint64, isWatched bool, watchDuration int, now time.Time) (bool, error) {
	return a.repo.SaveWatchRecord(ctx, userID, videoID, questionID, segmentID, isWatched, watchDuration, now)
}

func (a recommendationRepositoryAdapter) MarkRecommendationExposureWatched(ctx context.Context, userID uint64, questionID uint64, segmentID uint64, now time.Time) error {
	exposureRepo, ok := a.repo.(RecommendationExposureRepository)
	if !ok {
		return nil
	}
	return exposureRepo.MarkRecommendationExposureWatched(ctx, userID, questionID, segmentID, now)
}

func (a recommendationRepositoryAdapter) IncrementViewCount(ctx context.Context, id uint64) (int, bool, error) {
	return a.repo.IncrementViewCount(ctx, id)
}

func mapRecommendItemsFromApp(items []recommendationapp.ResultItem) []RecommendResultItem {
	out := make([]RecommendResultItem, 0, len(items))
	for _, item := range items {
		out = append(out, RecommendResultItem{
			QuestionID:     item.QuestionID,
			VideoID:        item.VideoID,
			VideoSegmentID: item.VideoSegmentID,
			RecommendScore: item.RecommendScore,
			IsWatched:      item.IsWatched,
			WatchDuration:  item.WatchDuration,
			StartTimeSec:   item.StartTimeSec,
			EndTimeSec:     item.EndTimeSec,
			Video:          item.Video,
			TitleOverride:  item.TitleOverride,
		})
	}
	return out
}
