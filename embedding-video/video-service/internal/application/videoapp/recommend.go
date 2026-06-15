package videoapp

import (
	"context"
	"errors"
	"time"

	"github.com/pgvector/pgvector-go"

	recommendationapp "nlp-video-analysis/internal/application/videoapp/recommendation"
)

var ErrVideoSegmentNotFound = errors.New("video_segment not found")

type RandomPlayableSegmentRepository interface {
	FindRandomPlayableSegment(ctx context.Context) (RecommendResultItem, bool, error)
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

// RandomPlayVideoSegment 随机返回一个可播放且未删除、已发布的视频片段。
func (s *Service) RandomPlayVideoSegment(ctx context.Context) (RecommendResultItem, bool, error) {
	repo, ok := s.Repo.(RandomPlayableSegmentRepository)
	if !ok {
		return RecommendResultItem{}, false, nil
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
		Now:                   s.Now,
		InvalidArgument:       InvalidArgumentError,
		IsProviderUnavailable: IsAIProviderUnavailable,
		NewDegradedError: func(reason string, items []recommendationapp.ResultItem) error {
			return DegradedError{Reason: reason, Items: mapRecommendItemsFromApp(items)}
		},
		ErrVideoSegmentAbsent: ErrVideoSegmentNotFound,
	}
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

func (a recommendationRepositoryAdapter) SaveUserVideoRecommendation(ctx context.Context, userID uint64, questionID uint64, videoID uint64, segmentID uint64, score float64, now time.Time) error {
	return a.repo.SaveUserVideoRecommendation(ctx, userID, questionID, videoID, segmentID, score, now)
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
