package recommendation

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/pgvector/pgvector-go"

	domainvideo "nlp-video-analysis/internal/domain/video"
)

type Candidate struct {
	VideoSegmentID uint64
	VideoID        uint64
	StartTimeSec   int
	EndTimeSec     int
	Distance       float64
	SegmentTitle   string
	VideoURL       string
	CoverURL       string
	Status         int16
	IsPublished    bool
	IsRecommend    bool
	ViewCount      int
	CreateTime     time.Time
	UpdateTime     time.Time
}

type ResultItem struct {
	QuestionID     uint64
	VideoID        uint64
	VideoSegmentID uint64
	RecommendScore float64
	IsWatched      bool
	WatchDuration  int
	StartTimeSec   int
	EndTimeSec     int
	Video          domainvideo.Video
	TitleOverride  string
}

type Record struct {
	QuestionID     uint64
	VideoID        uint64
	VideoSegmentID uint64
	RecommendScore float64
	IsWatched      bool
	WatchDuration  int
	StartTimeSec   int
	EndTimeSec     int
	Title          string
	VideoURL       string
	CoverURL       string
	Status         int16
	IsPublished    bool
	IsRecommend    bool
	ViewCount      int
	CreateTime     time.Time
	UpdateTime     time.Time
}

type RecommendByQuestionInput struct {
	QuestionID   uint64
	QuestionText string
	UserID       uint64
	Limit        int
}

type ListRecommendationsInput struct {
	QuestionID uint64
	UserID     uint64
	Limit      int
}

type ReportWatchInput struct {
	QuestionID     uint64
	UserID         uint64
	VideoSegmentID uint64
	IsWatched      bool
	WatchDuration  int
}

type Repository interface {
	GetSegmentEmbeddingDim(ctx context.Context) (int, error)
	GetQuestionEmbeddingTextByID(ctx context.Context, questionID uint64) (string, error)
	FindRecommendedSegments(ctx context.Context, query pgvector.Vector, limit int) ([]Candidate, error)
	SaveUserVideoRecommendation(ctx context.Context, userID uint64, questionID uint64, videoID uint64, segmentID uint64, score float64, now time.Time) error
	ListRecommendations(ctx context.Context, userID uint64, questionID uint64, limit int) ([]Record, error)
	GetVideoIDBySegmentID(ctx context.Context, segmentID uint64) (uint64, error)
	HasWatchedVideoForQuestion(ctx context.Context, userID uint64, questionID uint64, videoID uint64) (bool, error)
	SaveWatchRecord(ctx context.Context, userID uint64, videoID uint64, questionID uint64, segmentID uint64, isWatched bool, watchDuration int, now time.Time) (bool, error)
	IncrementViewCount(ctx context.Context, id uint64) (int, bool, error)
}

type Embedder interface {
	Embed(ctx context.Context, text string) ([]float32, error)
}

type Service struct {
	Repo                  Repository
	Embedder              Embedder
	Now                   func() time.Time
	InvalidArgument       func(message string) error
	IsProviderUnavailable func(error) bool
	NewDegradedError      func(reason string, items []ResultItem) error
	ErrVideoSegmentAbsent error
}

func (s Service) RecommendByQuestion(ctx context.Context, input RecommendByQuestionInput) ([]ResultItem, error) {
	userID := input.UserID
	if userID == 0 {
		userID = 1
	}
	questionText := strings.TrimSpace(input.QuestionText)
	if input.QuestionID == 0 && questionText == "" {
		return nil, s.InvalidArgument("question_text is required when question_id is absent")
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
		if s.IsProviderUnavailable != nil && s.IsProviderUnavailable(err) {
			return s.degradedRecommendByQuestion(ctx, input, err)
		}
		return nil, err
	}
	if targetDim <= 0 {
		targetDim = 1536
	}

	queryVec, err := s.BuildQuestionVector(ctx, input.QuestionID, questionText, targetDim)
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
	items := make([]ResultItem, 0, len(candidates))
	for _, c := range candidates {
		score := 0.0
		if c.Distance >= 0 {
			score = 1.0 / (1.0 + c.Distance)
		}
		if err := s.Repo.SaveUserVideoRecommendation(ctx, userID, input.QuestionID, c.VideoID, c.VideoSegmentID, score, now); err != nil {
			return nil, err
		}

		items = append(items, buildResultItem(input.QuestionID, c, score, false, 0))
	}

	return items, nil
}

func (s Service) ListRecommendations(ctx context.Context, input ListRecommendationsInput) ([]ResultItem, error) {
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

	items := make([]ResultItem, 0, len(rows))
	for _, r := range rows {
		items = append(items, ResultItem{
			QuestionID:     r.QuestionID,
			VideoID:        r.VideoID,
			VideoSegmentID: r.VideoSegmentID,
			RecommendScore: r.RecommendScore,
			IsWatched:      r.IsWatched,
			WatchDuration:  r.WatchDuration,
			StartTimeSec:   r.StartTimeSec,
			EndTimeSec:     r.EndTimeSec,
			Video: domainvideo.Video{
				ID:          r.VideoID,
				Title:       r.Title,
				VideoURL:    r.VideoURL,
				CoverURL:    r.CoverURL,
				Status:      domainvideo.Status(r.Status),
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

func (s Service) ReportWatch(ctx context.Context, input ReportWatchInput) error {
	if input.WatchDuration < 0 {
		return s.InvalidArgument("watch_duration must be greater than or equal to 0")
	}
	userID := input.UserID
	if userID == 0 {
		userID = 1
	}
	videoID, err := s.Repo.GetVideoIDBySegmentID(ctx, input.VideoSegmentID)
	if err != nil {
		return err
	}
	if videoID == 0 {
		return s.ErrVideoSegmentAbsent
	}
	alreadyCounted, err := s.Repo.HasWatchedVideoForQuestion(ctx, userID, input.QuestionID, videoID)
	if err != nil {
		return err
	}
	created, err := s.Repo.SaveWatchRecord(ctx, userID, videoID, input.QuestionID, input.VideoSegmentID, input.IsWatched, input.WatchDuration, s.Now())
	if err != nil {
		return err
	}
	if !created || alreadyCounted {
		return nil
	}
	if _, _, err := s.Repo.IncrementViewCount(ctx, videoID); err != nil {
		return err
	}
	return nil
}

func (s Service) BuildQuestionVector(ctx context.Context, questionID uint64, questionText string, targetDim int) (pgvector.Vector, error) {
	if questionID != 0 {
		text, err := s.Repo.GetQuestionEmbeddingTextByID(ctx, questionID)
		if err != nil {
			return pgvector.Vector{}, err
		}
		vec, err := ParseVectorText(text)
		if err != nil {
			return pgvector.Vector{}, fmt.Errorf("parse question embedding failed: %w", err)
		}
		vec = NormalizeVectorDim(vec, targetDim)
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
	vec = NormalizeVectorDim(vec, targetDim)
	if len(vec) != targetDim {
		return pgvector.Vector{}, fmt.Errorf("embedding dimension mismatch: got=%d want=%d", len(vec), targetDim)
	}
	return pgvector.NewVector(vec), nil
}

func ParseVectorText(text string) ([]float32, error) {
	value := strings.TrimSpace(text)
	if value == "" {
		return nil, fmt.Errorf("empty")
	}
	if len(value) >= 2 {
		if (value[0] == '[' && value[len(value)-1] == ']') || (value[0] == '(' && value[len(value)-1] == ')') {
			value = value[1 : len(value)-1]
		}
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, fmt.Errorf("empty")
	}
	parts := strings.Split(value, ",")
	out := make([]float32, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		f, err := strconv.ParseFloat(part, 32)
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

func NormalizeVectorDim(v []float32, dim int) []float32 {
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

func MaxInt(a int, b int) int {
	if a > b {
		return a
	}
	return b
}

func MaxUint(a uint64, b uint64) uint64 {
	if a > b {
		return a
	}
	return b
}

func (s Service) degradedRecommendByQuestion(ctx context.Context, input RecommendByQuestionInput, cause error) ([]ResultItem, error) {
	if input.QuestionText == "" && input.QuestionID != 0 {
		if text, err := s.Repo.GetQuestionEmbeddingTextByID(ctx, input.QuestionID); err == nil {
			input.QuestionText = text
		}
	}
	items, err := s.degradedRecommendByQuestionWithText(ctx, input)
	if err != nil {
		return nil, s.NewDegradedError(cause.Error(), nil)
	}
	return items, s.NewDegradedError(cause.Error(), items)
}

func (s Service) degradedRecommendByQuestionWithText(ctx context.Context, input RecommendByQuestionInput) ([]ResultItem, error) {
	if s.Embedder == nil {
		return nil, s.NewDegradedError("embedder_unavailable", nil)
	}
	text := strings.TrimSpace(input.QuestionText)
	if text == "" {
		return nil, s.NewDegradedError("question_text_unavailable", nil)
	}
	vec, err := s.Embedder.Embed(ctx, text)
	if err != nil {
		return nil, s.NewDegradedError(err.Error(), nil)
	}
	targetDim, err := s.Repo.GetSegmentEmbeddingDim(ctx)
	if err != nil {
		return nil, s.NewDegradedError(err.Error(), nil)
	}
	if targetDim <= 0 {
		targetDim = len(vec)
	}
	if targetDim <= 0 {
		return nil, s.NewDegradedError("embedding_dim_unavailable", nil)
	}
	if len(vec) > targetDim {
		vec = vec[:targetDim]
	}
	if len(vec) < targetDim {
		padded := make([]float32, targetDim)
		copy(padded, vec)
		vec = padded
	}
	queryVec := pgvector.NewVector(vec)
	candidates, err := s.Repo.FindRecommendedSegments(ctx, queryVec, MaxInt(input.Limit, 3))
	if err != nil {
		return nil, s.NewDegradedError(err.Error(), nil)
	}
	if len(candidates) == 0 {
		return []ResultItem{}, s.NewDegradedError("provider_unavailable", []ResultItem{})
	}
	now := s.Now()
	items := make([]ResultItem, 0, len(candidates))
	for _, c := range candidates {
		score := 0.0
		if c.Distance >= 0 {
			score = 1.0 / (1.0 + c.Distance)
		}
		items = append(items, buildResultItem(input.QuestionID, c, score, false, 0))
		_ = s.Repo.SaveUserVideoRecommendation(ctx, MaxUint(input.UserID, 1), input.QuestionID, c.VideoID, c.VideoSegmentID, score, now)
	}
	return items, s.NewDegradedError("provider_unavailable", items)
}

func buildResultItem(questionID uint64, c Candidate, score float64, watched bool, watchDuration int) ResultItem {
	return ResultItem{
		QuestionID:     questionID,
		VideoID:        c.VideoID,
		VideoSegmentID: c.VideoSegmentID,
		RecommendScore: score,
		IsWatched:      watched,
		WatchDuration:  watchDuration,
		StartTimeSec:   c.StartTimeSec,
		EndTimeSec:     c.EndTimeSec,
		TitleOverride:  c.SegmentTitle,
		Video: domainvideo.Video{
			ID:          c.VideoID,
			Title:       c.SegmentTitle,
			VideoURL:    c.VideoURL,
			CoverURL:    c.CoverURL,
			Status:      domainvideo.Status(c.Status),
			IsPublished: c.IsPublished,
			IsRecommend: c.IsRecommend,
			ViewCount:   c.ViewCount,
			CreateTime:  c.CreateTime,
			UpdateTime:  c.UpdateTime,
		},
	}
}
