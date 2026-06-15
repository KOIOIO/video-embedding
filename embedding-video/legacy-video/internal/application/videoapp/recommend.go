package videoapp

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/pgvector/pgvector-go"

	domainvideo "legacy-video/internal/domain/video"
)

var ErrVideoSegmentNotFound = errors.New("video_segment not found")

// RecommendByQuestion 根据题目向量召回最相关的视频片段，并记录推荐结果。
func (s *Service) RecommendByQuestion(ctx context.Context, input RecommendByQuestionInput) ([]RecommendResultItem, error) {
	userID := input.UserID
	if userID == 0 {
		userID = 1
	}
	limit := input.Limit
	if limit <= 0 {
		limit = 10
	}
	if limit > 50 {
		limit = 50
	}

	targetDim, err := s.Repo.GetSegmentEmbeddingDim(ctx)
	if err != nil {
		return nil, err
	}
	if targetDim <= 0 {
		targetDim = 1536
	}

	queryVec, err := s.buildQuestionVector(ctx, input.QuestionID, strings.TrimSpace(input.QuestionText), targetDim)
	if err != nil {
		return nil, err
	}

	candidates, err := s.Repo.FindRecommendedSegments(ctx, queryVec, limit)
	if err != nil {
		return nil, err
	}
	if len(candidates) == 0 {
		return nil, nil
	}

	now := s.Now()
	items := make([]RecommendResultItem, 0, len(candidates))
	for _, c := range candidates {
		score := 0.0
		if c.Distance >= 0 {
			score = 1.0 / (1.0 + c.Distance)
		}
		if err := s.Repo.SaveUserVideoRecommendation(ctx, userID, input.QuestionID, c.VideoID, c.VideoSegmentID, score, now); err != nil {
			return nil, err
		}

		items = append(items, RecommendResultItem{
			QuestionID:     input.QuestionID,
			VideoID:        c.VideoID,
			VideoSegmentID: c.VideoSegmentID,
			RecommendScore: score,
			IsWatched:      false,
			WatchDuration:  0,
			StartTimeSec:   c.StartTimeSec,
			EndTimeSec:     c.EndTimeSec,
			TitleOverride:  c.SegmentTitle,
			Video: domainvideo.Video{
				ID:          c.VideoID,
				Title:       c.SegmentTitle,
				VideoURL:    c.VideoURL,
				CoverURL:    c.CoverURL,
				IsPublished: c.IsPublished,
				IsRecommend: c.IsRecommend,
				ViewCount:   c.ViewCount,
				CreateTime:  c.CreateTime,
				UpdateTime:  c.UpdateTime,
			},
		})
	}

	return items, nil
}

// ListRecommendations 查询用户历史推荐结果。
func (s *Service) ListRecommendations(ctx context.Context, input ListRecommendationsInput) ([]RecommendResultItem, error) {
	userID := input.UserID
	if userID == 0 {
		userID = 1
	}
	limit := input.Limit
	if limit <= 0 {
		limit = 20
	}
	if limit > 200 {
		limit = 200
	}

	rows, err := s.Repo.ListRecommendations(ctx, userID, input.QuestionID, limit)
	if err != nil {
		return nil, err
	}

	items := make([]RecommendResultItem, 0, len(rows))
	for _, r := range rows {
		items = append(items, RecommendResultItem{
			QuestionID:     r.QuestionID,
			VideoID:        r.VideoID,
			VideoSegmentID: r.VideoSegmentID,
			RecommendScore: r.RecommendScore,
			IsWatched:      r.IsWatched,
			WatchDuration:  r.WatchDuration,
			Video: domainvideo.Video{
				ID:          r.VideoID,
				Title:       r.Title,
				VideoURL:    r.VideoURL,
				CoverURL:    r.CoverURL,
				IsPublished: r.IsPublished,
				IsRecommend: r.IsRecommend,
				ViewCount:   r.ViewCount,
				CreateTime:  r.CreateTime,
				UpdateTime:  r.UpdateTime,
			},
		})
	}

	return items, nil
}

// ReportWatch 记录用户对推荐片段的观看情况。
func (s *Service) ReportWatch(ctx context.Context, input ReportWatchInput) error {
	userID := input.UserID
	if userID == 0 {
		userID = 1
	}
	videoID, err := s.Repo.GetVideoIDBySegmentID(ctx, input.VideoSegmentID)
	if err != nil {
		return err
	}
	if videoID == 0 {
		return ErrVideoSegmentNotFound
	}
	return s.Repo.SaveWatchRecord(ctx, userID, videoID, input.QuestionID, input.VideoSegmentID, input.IsWatched, input.WatchDuration, s.Now())
}

// buildQuestionVector 根据题目 ID 或题目文本生成查询向量。
// 优先复用题库已存 embedding，回退到实时 Embedding 服务。
func (s *Service) buildQuestionVector(ctx context.Context, questionID uint64, questionText string, targetDim int) (pgvector.Vector, error) {
	if questionID != 0 {
		text, err := s.Repo.GetQuestionEmbeddingTextByID(ctx, questionID)
		if err != nil {
			return pgvector.Vector{}, err
		}
		vec, err := parseVectorText(text)
		if err != nil {
			return pgvector.Vector{}, fmt.Errorf("parse question embedding failed: %w", err)
		}
		vec = normalizeVectorDim(vec, targetDim)
		if len(vec) != targetDim {
			return pgvector.Vector{}, fmt.Errorf("question embedding dimension mismatch: got=%d want=%d", len(vec), targetDim)
		}
		return pgvector.NewVector(vec), nil
	}

	if questionText == "" {
		return pgvector.Vector{}, errors.New("question_text is required")
	}
	if s.Embedder == nil {
		return pgvector.Vector{}, errors.New("embedder not initialized")
	}
	vec, err := s.Embedder.Embed(ctx, questionText)
	if err != nil {
		return pgvector.Vector{}, err
	}
	vec = normalizeVectorDim(vec, targetDim)
	if len(vec) != targetDim {
		return pgvector.Vector{}, fmt.Errorf("embedding dimension mismatch: got=%d want=%d", len(vec), targetDim)
	}
	return pgvector.NewVector(vec), nil
}

// parseVectorText 解析数据库中保存的向量文本表示。
func parseVectorText(text string) ([]float32, error) {
	s := strings.TrimSpace(text)
	if s == "" {
		return nil, fmt.Errorf("empty")
	}
	if len(s) >= 2 {
		if (s[0] == '[' && s[len(s)-1] == ']') || (s[0] == '(' && s[len(s)-1] == ')') {
			s = s[1 : len(s)-1]
		}
	}
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, fmt.Errorf("empty")
	}
	parts := strings.Split(s, ",")
	out := make([]float32, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		f, err := strconv.ParseFloat(p, 32)
		if err != nil {
			return nil, err
		}
		out = append(out, float32(f))
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("empty")
	}
	return out, nil
}

// normalizeVectorDim 对查询向量做裁剪或补零，使其与片段 embedding 维度一致。
func normalizeVectorDim(v []float32, dim int) []float32 {
	if dim <= 0 {
		return v
	}
	if len(v) == dim || len(v) == 0 {
		return v
	}
	if len(v) > dim {
		return v[:dim]
	}
	out := make([]float32, dim)
	copy(out, v)
	return out
}
