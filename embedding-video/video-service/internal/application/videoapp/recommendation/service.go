package recommendation

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
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
	KnowledgeTags  string
	VideoTitle     string
	Description    string
	VideoURL       string
	CoverURL       string
	Status         int16
	IsPublished    bool
	IsRecommend    bool
	ViewCount      int
	CreateTime     time.Time
	UpdateTime     time.Time
}

const (
	DefaultProfileModelVersion = "video_profile_v1"
	DefaultRecBoleModelVersion = "recbole_v1"
	MaxProfileRerankCandidates = 300
	WeakKnowledgeRecallLimit   = 50
	StrategyQuestionVector     = "question_vector"
	StrategyProfileRerank      = "profile_rerank"
	StrategyRecBole            = "recbole"
	StrategyGorse              = "gorse"
	StrategyKnowledgeMatch     = "knowledge_match"
	GorseModelVersion          = "gorse"
	KnowledgeMatchModelVersion = "knowledge_match_v1"
	EngineKnowledgeMatch       = "knowledge_match"
	EngineGorse                = "gorse"
	EngineRecBole              = "recbole"
)

type UserVideoProfile struct {
	UserID        uint64
	ProfileVector []float32
	ModelVersion  string
	Status        int16
	PositiveCount int
}

func (p UserVideoProfile) IsUsable() bool {
	return p.UserID != 0 && p.Status == 1 && p.PositiveCount > 0 && len(p.ProfileVector) > 0
}

type ProfileRerankQuery struct {
	UserID         uint64
	QuestionVector pgvector.Vector
	ProfileVector  pgvector.Vector
	Limit          int
}

type UserRecBoleEmbedding struct {
	UserID       uint64
	Vector       []float32
	ModelVersion string
	Status       int16
}

func (e UserRecBoleEmbedding) IsUsable() bool {
	return e.UserID != 0 && e.Status == 1 && len(e.Vector) > 0
}

type RecBoleQuery struct {
	UserID       uint64
	UserVector   pgvector.Vector
	ModelVersion string
	Limit        int
}

type RecBoleCandidate struct {
	Candidate
}

type WeakKnowledge struct {
	KnowledgePointID uint64
	Mastery          float64
	Name             string
	Description      string
}

type WeakKnowledgeVectorQuery struct {
	UserID           uint64
	Query            pgvector.Vector
	Limit            int
	RequireRecommend bool
}

type ResultItem struct {
	QuestionID     uint64
	VideoID        uint64
	VideoSegmentID uint64
	RecommendScore float64
	Strategy       string
	ModelVersion   string
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

type RandomPlayInput struct {
	UserID uint64
	Limit  int
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

type ExposureRecord struct {
	RequestID      string
	UserID         uint64
	QuestionID     uint64
	VideoID        uint64
	VideoSegmentID uint64
	Rank           int
	Score          float64
	Strategy       string
	ModelVersion   string
	Now            time.Time
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

type ExposureRepository interface {
	SaveRecommendationExposures(ctx context.Context, exposures []ExposureRecord) error
	MarkRecommendationExposureWatched(ctx context.Context, userID uint64, questionID uint64, segmentID uint64, now time.Time) error
}

type ProfileRepository interface {
	GetUserVideoProfile(ctx context.Context, userID uint64, modelVersion string) (UserVideoProfile, bool, error)
	FindRecommendedSegmentsForProfileRerank(ctx context.Context, input ProfileRerankQuery) ([]ProfileCandidate, error)
}

type RecBoleRepository interface {
	GetUserRecBoleEmbedding(ctx context.Context, userID uint64, modelVersion string) (UserRecBoleEmbedding, bool, error)
	FindRecommendedSegmentsForRecBole(ctx context.Context, input RecBoleQuery) ([]RecBoleCandidate, error)
}

type GorseHydrationRepository interface {
	HydrateRecommendedSegmentsByID(ctx context.Context, userID uint64, ids []uint64) ([]Candidate, error)
}

type WeakKnowledgeVectorRepository interface {
	ListWeakKnowledge(ctx context.Context, userID uint64, limit int) ([]WeakKnowledge, error)
	FindRecommendedSegmentsByWeakKnowledgeVector(ctx context.Context, input WeakKnowledgeVectorQuery) ([]Candidate, error)
}

type RecBoleModelVersionRepository interface {
	GetActiveRecBoleModelVersion(ctx context.Context) (string, bool, error)
}

type RecentSegmentStore interface {
	FilterRecent(ctx context.Context, userID uint64, segmentIDs []uint64) (map[uint64]bool, error)
	ListRecent(ctx context.Context, userID uint64) ([]uint64, error)
	MarkReturned(ctx context.Context, userID uint64, segmentID uint64, ttl time.Duration) error
}

type Embedder interface {
	Embed(ctx context.Context, text string) ([]float32, error)
}

type Service struct {
	Repo                  Repository
	Embedder              Embedder
	Gorse                 GorseClient
	Engine                string
	GorseOptions          GorseOptions
	Now                   func() time.Time
	InvalidArgument       func(message string) error
	IsProviderUnavailable func(error) bool
	NewDegradedError      func(reason string, items []ResultItem) error
	ErrVideoSegmentAbsent error
	RecentSegments        RecentSegmentStore
	RecentSegmentTTL      time.Duration
	NewRequestID          func() string
}

type GorseOptions struct {
	CandidateLimit    int
	MinRecommendItems int
	WriteBackEnabled  bool
	ShadowMode        bool
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
	exposures := make([]ExposureRecord, 0, len(candidates))
	requestID := s.newRequestID()
	items := make([]ResultItem, 0, len(candidates))
	for i, c := range candidates {
		score := 0.0
		if c.Distance >= 0 {
			score = 1.0 / (1.0 + c.Distance)
		}
		if err := s.Repo.SaveUserVideoRecommendation(ctx, userID, input.QuestionID, c.VideoID, c.VideoSegmentID, score, now); err != nil {
			return nil, err
		}

		items = append(items, withRecommendationSource(buildResultItem(input.QuestionID, c, score, false, 0), StrategyQuestionVector, ""))
		exposures = append(exposures, buildExposureRecord(requestID, userID, input.QuestionID, c.VideoID, c.VideoSegmentID, i+1, score, StrategyQuestionVector, "", now))
	}
	if err := s.saveRecommendationExposures(ctx, exposures); err != nil {
		return nil, err
	}

	return items, nil
}

func (s Service) PreviewRecommendByQuestion(ctx context.Context, input RecommendByQuestionInput) ([]ResultItem, error) {
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

	items := make([]ResultItem, 0, len(candidates))
	for _, c := range candidates {
		score := 0.0
		if c.Distance >= 0 {
			score = 1.0 / (1.0 + c.Distance)
		}
		items = append(items, withRecommendationSource(buildResultItem(input.QuestionID, c, score, false, 0), StrategyQuestionVector, ""))
	}
	return items, nil
}

func (s Service) RandomPlay(ctx context.Context, input RandomPlayInput) ([]ResultItem, error) {
	if input.UserID == 0 {
		return nil, nil
	}
	limit := normalizeRandomPlayLimit(input.Limit)

	if s.recommendationEngine() == EngineGorse && !s.GorseOptions.ShadowMode {
		if items, ok, err := s.recommendByGorse(ctx, input.UserID, limit); err != nil {
			return nil, err
		} else if ok {
			return items, nil
		}
	}
	if s.recommendationEngine() == EngineRecBole {
		return s.randomPlayByRecBole(ctx, input.UserID, limit)
	}
	return s.recommendByKnowledgeMatch(ctx, input.UserID, limit)
}

func (s Service) PreviewRandomPlay(ctx context.Context, input RandomPlayInput) ([]ResultItem, error) {
	if input.UserID == 0 {
		return nil, nil
	}
	limit := normalizeRandomPlayLimit(input.Limit)
	engine := s.recommendationEngine()
	if engine == EngineGorse && !s.GorseOptions.ShadowMode {
		if items, ok, err := s.previewByGorse(ctx, input.UserID, limit); err != nil {
			return nil, err
		} else if ok {
			return items, nil
		}
	}
	if engine == EngineRecBole {
		return s.previewByRecBole(ctx, input.UserID, limit)
	}
	return s.previewByKnowledgeMatch(ctx, input.UserID, limit)
}

func normalizeRandomPlayLimit(limit int) int {
	if limit <= 0 {
		return 1
	}
	if limit > 50 {
		return 50
	}
	return limit
}

func (s Service) randomPlayByRecBole(ctx context.Context, userID uint64, limit int) ([]ResultItem, error) {
	recBoleRepo, ok := s.Repo.(RecBoleRepository)
	if !ok {
		return nil, nil
	}
	modelVersion, err := s.activeRecBoleModelVersion(ctx)
	if err != nil {
		return nil, err
	}
	userEmbedding, found, err := recBoleRepo.GetUserRecBoleEmbedding(ctx, userID, modelVersion)
	if err != nil {
		return nil, err
	}
	if !found || !userEmbedding.IsUsable() {
		return nil, nil
	}

	return s.recommendByRecBole(ctx, recBoleRepo, RecommendByQuestionInput{
		UserID: userID,
		Limit:  limit,
	}, userID, userEmbedding, modelVersion, limit)
}

func (s Service) previewByRecBole(ctx context.Context, userID uint64, limit int) ([]ResultItem, error) {
	recBoleRepo, ok := s.Repo.(RecBoleRepository)
	if !ok {
		return nil, nil
	}
	modelVersion, err := s.activeRecBoleModelVersion(ctx)
	if err != nil {
		return nil, err
	}
	userEmbedding, found, err := recBoleRepo.GetUserRecBoleEmbedding(ctx, userID, modelVersion)
	if err != nil {
		return nil, err
	}
	if !found || !userEmbedding.IsUsable() {
		return nil, nil
	}
	candidates, err := recBoleRepo.FindRecommendedSegmentsForRecBole(ctx, RecBoleQuery{
		UserID:       userID,
		UserVector:   pgvector.NewVector(userEmbedding.Vector),
		ModelVersion: modelVersion,
		Limit:        profileCandidateLimit(limit),
	})
	if err != nil {
		return nil, err
	}
	if len(candidates) == 0 {
		return nil, nil
	}
	candidates = s.filterRecentRecBoleCandidates(ctx, userID, candidates)
	if len(candidates) == 0 {
		return nil, nil
	}
	items := BuildRecBoleRankedItems(candidates, limit)
	for i := range items {
		items[i] = withRecommendationSource(items[i], StrategyRecBole, modelVersion)
	}
	return items, nil
}

func (s Service) RecommendRecBoleItemIDs(ctx context.Context, userID uint64, limit int) ([]uint64, error) {
	if userID == 0 {
		return nil, nil
	}
	if limit <= 0 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}
	recBoleRepo, ok := s.Repo.(RecBoleRepository)
	if !ok {
		return nil, nil
	}
	modelVersion, err := s.activeRecBoleModelVersion(ctx)
	if err != nil {
		return nil, err
	}
	userEmbedding, found, err := recBoleRepo.GetUserRecBoleEmbedding(ctx, userID, modelVersion)
	if err != nil {
		return nil, err
	}
	if !found || !userEmbedding.IsUsable() {
		return nil, nil
	}
	candidates, err := recBoleRepo.FindRecommendedSegmentsForRecBole(ctx, RecBoleQuery{
		UserID:       userID,
		UserVector:   pgvector.NewVector(userEmbedding.Vector),
		ModelVersion: modelVersion,
		Limit:        profileCandidateLimit(limit),
	})
	if err != nil {
		return nil, err
	}
	if len(candidates) == 0 {
		return nil, nil
	}
	candidates = s.filterRecentRecBoleCandidates(ctx, userID, candidates)
	if len(candidates) == 0 {
		return nil, nil
	}
	items := BuildRecBoleRankedItems(candidates, limit)
	ids := make([]uint64, 0, len(items))
	seen := make(map[uint64]bool, len(items))
	for _, item := range items {
		if item.VideoSegmentID == 0 || seen[item.VideoSegmentID] {
			continue
		}
		seen[item.VideoSegmentID] = true
		ids = append(ids, item.VideoSegmentID)
	}
	return ids, nil
}

func (s Service) recommendationEngine() string {
	engine := strings.ToLower(strings.TrimSpace(s.Engine))
	if engine == "" {
		return EngineKnowledgeMatch
	}
	return engine
}

func (s Service) recommendByKnowledgeMatch(ctx context.Context, userID uint64, limit int) ([]ResultItem, error) {
	const weakKnowledgeLimit = 10
	candidates, err := s.recommendWeakKnowledgeCandidates(ctx, userID, limit, weakKnowledgeLimit)
	if err != nil {
		return nil, err
	}
	if len(candidates) == 0 {
		return nil, nil
	}
	candidates = s.filterRecentCandidates(ctx, userID, candidates)
	if len(candidates) == 0 {
		return nil, nil
	}
	if len(candidates) > limit {
		candidates = candidates[:limit]
	}
	now := s.Now()
	requestID := s.newRequestID()
	items := make([]ResultItem, 0, len(candidates))
	exposures := make([]ExposureRecord, 0, len(candidates))
	for i, candidate := range candidates {
		score := gorseRankScore(i)
		item := withRecommendationSource(buildResultItem(0, candidate, score, false, 0), StrategyKnowledgeMatch, KnowledgeMatchModelVersion)
		items = append(items, item)
		s.markRecentReturned(ctx, userID, candidate.VideoSegmentID)
		if err := s.Repo.SaveUserVideoRecommendation(ctx, userID, 0, candidate.VideoID, candidate.VideoSegmentID, score, now); err != nil {
			return nil, err
		}
		exposures = append(exposures, buildExposureRecord(requestID, userID, 0, candidate.VideoID, candidate.VideoSegmentID, i+1, score, StrategyKnowledgeMatch, KnowledgeMatchModelVersion, now))
	}
	if err := s.saveRecommendationExposures(ctx, exposures); err != nil {
		return nil, err
	}
	return items, nil
}

func (s Service) previewByKnowledgeMatch(ctx context.Context, userID uint64, limit int) ([]ResultItem, error) {
	const weakKnowledgeLimit = 10
	candidates, err := s.recommendWeakKnowledgeCandidates(ctx, userID, limit, weakKnowledgeLimit)
	if err != nil {
		return nil, err
	}
	if len(candidates) == 0 {
		return nil, nil
	}
	candidates = s.filterRecentCandidates(ctx, userID, candidates)
	if len(candidates) == 0 {
		return nil, nil
	}
	if len(candidates) > limit {
		candidates = candidates[:limit]
	}
	items := make([]ResultItem, 0, len(candidates))
	for i, candidate := range candidates {
		items = append(items, withRecommendationSource(buildResultItem(0, candidate, gorseRankScore(i), false, 0), StrategyKnowledgeMatch, KnowledgeMatchModelVersion))
	}
	return items, nil
}

func (s Service) recommendWeakKnowledgeCandidates(ctx context.Context, userID uint64, limit int, weakKnowledgeLimit int) ([]Candidate, error) {
	repo, ok := s.Repo.(WeakKnowledgeVectorRepository)
	if !ok || s.Embedder == nil {
		return nil, nil
	}
	weakKnowledge, err := repo.ListWeakKnowledge(ctx, userID, weakKnowledgeLimit)
	if err != nil {
		return nil, err
	}
	queryText := buildWeakKnowledgeEmbeddingText(weakKnowledge)
	if queryText == "" {
		return nil, nil
	}
	targetDim, err := s.Repo.GetSegmentEmbeddingDim(ctx)
	if err != nil {
		return nil, err
	}
	if targetDim <= 0 {
		targetDim = 1536
	}
	vec, err := s.Embedder.Embed(ctx, queryText)
	if err != nil {
		return nil, err
	}
	vec = NormalizeVectorDim(vec, targetDim)
	if len(vec) != targetDim {
		return nil, fmt.Errorf("weak knowledge embedding dimension mismatch: got=%d want=%d", len(vec), targetDim)
	}
	recallLimit := MaxInt(limit, WeakKnowledgeRecallLimit)
	query := WeakKnowledgeVectorQuery{
		UserID:           userID,
		Query:            pgvector.NewVector(vec),
		Limit:            recallLimit,
		RequireRecommend: true,
	}
	candidates, err := repo.FindRecommendedSegmentsByWeakKnowledgeVector(ctx, query)
	if err != nil {
		return nil, err
	}
	if len(candidates) == 0 {
		query.RequireRecommend = false
		candidates, err = repo.FindRecommendedSegmentsByWeakKnowledgeVector(ctx, query)
		if err != nil {
			return nil, err
		}
	}
	return rerankWeakKnowledgeCandidates(candidates, weakKnowledge, recallLimit), nil
}

func buildWeakKnowledgeEmbeddingText(rows []WeakKnowledge) string {
	parts := make([]string, 0, len(rows))
	for _, row := range rows {
		name := strings.TrimSpace(row.Name)
		description := strings.TrimSpace(row.Description)
		switch {
		case name != "" && description != "":
			parts = append(parts, name+"："+description)
		case name != "":
			parts = append(parts, name)
		case description != "":
			parts = append(parts, description)
		}
	}
	return strings.Join(parts, "\n")
}

func rerankWeakKnowledgeCandidates(candidates []Candidate, weakKnowledge []WeakKnowledge, limit int) []Candidate {
	if len(candidates) == 0 {
		return nil
	}
	tokens := weakKnowledgeTokens(weakKnowledge)
	if len(tokens) == 0 {
		return trimCandidates(candidates, limit)
	}
	type scoredCandidate struct {
		candidate Candidate
		score     int
		index     int
	}
	scored := make([]scoredCandidate, 0, len(candidates))
	for i, candidate := range candidates {
		scored = append(scored, scoredCandidate{
			candidate: candidate,
			score:     weakKnowledgeTextScore(candidate, tokens),
			index:     i,
		})
	}
	sort.SliceStable(scored, func(i, j int) bool {
		if scored[i].score != scored[j].score {
			return scored[i].score > scored[j].score
		}
		return scored[i].index < scored[j].index
	})
	if limit <= 0 || limit > len(scored) {
		limit = len(scored)
	}
	out := make([]Candidate, 0, limit)
	for i := 0; i < limit; i++ {
		out = append(out, scored[i].candidate)
	}
	return out
}

func weakKnowledgeTextScore(candidate Candidate, tokens []string) int {
	primary := strings.ToLower(strings.Join([]string{
		candidate.SegmentTitle,
		candidate.KnowledgeTags,
	}, " "))
	secondary := strings.ToLower(strings.Join([]string{
		candidate.VideoTitle,
		candidate.Description,
	}, " "))
	score := 0
	for _, token := range tokens {
		switch {
		case strings.Contains(primary, token):
			score += 2
		case strings.Contains(secondary, token):
			score++
		}
	}
	return score
}

func weakKnowledgeTokens(rows []WeakKnowledge) []string {
	seen := map[string]bool{}
	tokens := make([]string, 0, len(rows)*2)
	for _, row := range rows {
		for _, token := range splitKnowledgeTokens(row.Name + " " + row.Description) {
			if seen[token] {
				continue
			}
			seen[token] = true
			tokens = append(tokens, token)
		}
	}
	return tokens
}

func splitKnowledgeTokens(text string) []string {
	parts := strings.FieldsFunc(text, func(r rune) bool {
		switch r {
		case ' ', '\t', '\n', '\r', ',', '，', ';', '；', '/', '|', '、', '.', '。', ':', '：', '(', ')', '（', '）', '[', ']', '【', '】':
			return true
		default:
			return false
		}
	})
	tokens := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.ToLower(strings.TrimSpace(part))
		if part == "" {
			continue
		}
		tokens = append(tokens, part)
	}
	return tokens
}

func trimCandidates(candidates []Candidate, limit int) []Candidate {
	if limit <= 0 || limit >= len(candidates) {
		return candidates
	}
	return candidates[:limit]
}

func (s Service) recommendByGorse(ctx context.Context, userID uint64, limit int) ([]ResultItem, bool, error) {
	if s.Gorse == nil {
		return nil, false, nil
	}
	hydrator, ok := s.Repo.(GorseHydrationRepository)
	if !ok {
		return nil, false, nil
	}
	candidateLimit := s.GorseOptions.CandidateLimit
	if candidateLimit <= 0 {
		candidateLimit = profileCandidateLimit(limit)
	}
	if candidateLimit < limit {
		candidateLimit = limit
	}
	ids, err := s.Gorse.Recommend(ctx, userID, candidateLimit)
	if err != nil {
		return nil, false, nil
	}
	ids = uniquePositiveUint64s(ids)
	if len(ids) == 0 {
		return nil, false, nil
	}
	candidates, err := hydrator.HydrateRecommendedSegmentsByID(ctx, userID, ids)
	if err != nil {
		return nil, false, err
	}
	if len(candidates) == 0 {
		return nil, false, nil
	}
	if minItems := s.GorseOptions.MinRecommendItems; minItems > 0 && len(candidates) < minItems {
		return nil, false, nil
	}
	candidates = s.filterRecentCandidates(ctx, userID, candidates)
	if len(candidates) == 0 {
		return nil, false, nil
	}
	if len(candidates) > limit {
		candidates = candidates[:limit]
	}
	now := s.Now()
	requestID := s.newRequestID()
	items := make([]ResultItem, 0, len(candidates))
	exposures := make([]ExposureRecord, 0, len(candidates))
	feedback := make([]GorseFeedback, 0, len(candidates))
	for i, candidate := range candidates {
		score := gorseRankScore(i)
		item := withRecommendationSource(buildResultItem(0, candidate, score, false, 0), StrategyGorse, GorseModelVersion)
		items = append(items, item)
		s.markRecentReturned(ctx, userID, candidate.VideoSegmentID)
		if err := s.Repo.SaveUserVideoRecommendation(ctx, userID, 0, candidate.VideoID, candidate.VideoSegmentID, score, now); err != nil {
			return nil, false, err
		}
		exposures = append(exposures, buildExposureRecord(requestID, userID, 0, candidate.VideoID, candidate.VideoSegmentID, i+1, score, StrategyGorse, GorseModelVersion, now))
		if mapped, ok := MapGorseFeedback(GorseFeedbackSource{
			UserID:         userID,
			VideoSegmentID: candidate.VideoSegmentID,
			Kind:           GorseFeedbackExposure,
			EventTime:      now,
		}); ok {
			feedback = append(feedback, mapped)
		}
	}
	if err := s.saveRecommendationExposures(ctx, exposures); err != nil {
		return nil, false, err
	}
	if s.GorseOptions.WriteBackEnabled {
		_ = s.Gorse.PutFeedback(ctx, feedback)
	}
	return items, true, nil
}

func (s Service) previewByGorse(ctx context.Context, userID uint64, limit int) ([]ResultItem, bool, error) {
	if s.Gorse == nil {
		return nil, false, nil
	}
	hydrator, ok := s.Repo.(GorseHydrationRepository)
	if !ok {
		return nil, false, nil
	}
	candidateLimit := s.GorseOptions.CandidateLimit
	if candidateLimit <= 0 {
		candidateLimit = profileCandidateLimit(limit)
	}
	if candidateLimit < limit {
		candidateLimit = limit
	}
	ids, err := s.Gorse.Recommend(ctx, userID, candidateLimit)
	if err != nil {
		return nil, false, nil
	}
	ids = uniquePositiveUint64s(ids)
	if len(ids) == 0 {
		return nil, false, nil
	}
	candidates, err := hydrator.HydrateRecommendedSegmentsByID(ctx, userID, ids)
	if err != nil {
		return nil, false, err
	}
	if len(candidates) == 0 {
		return nil, false, nil
	}
	if minItems := s.GorseOptions.MinRecommendItems; minItems > 0 && len(candidates) < minItems {
		return nil, false, nil
	}
	candidates = s.filterRecentCandidates(ctx, userID, candidates)
	if len(candidates) == 0 {
		return nil, false, nil
	}
	if len(candidates) > limit {
		candidates = candidates[:limit]
	}
	items := make([]ResultItem, 0, len(candidates))
	for i, candidate := range candidates {
		items = append(items, withRecommendationSource(buildResultItem(0, candidate, gorseRankScore(i), false, 0), StrategyGorse, GorseModelVersion))
	}
	return items, true, nil
}

func (s Service) activeRecBoleModelVersion(ctx context.Context) (string, error) {
	repo, ok := s.Repo.(RecBoleModelVersionRepository)
	if !ok {
		return DefaultRecBoleModelVersion, nil
	}
	version, found, err := repo.GetActiveRecBoleModelVersion(ctx)
	if err != nil {
		return "", err
	}
	version = strings.TrimSpace(version)
	if !found || version == "" {
		return DefaultRecBoleModelVersion, nil
	}
	return version, nil
}

func (s Service) recommendByRecBole(ctx context.Context, repo RecBoleRepository, input RecommendByQuestionInput, userID uint64, userEmbedding UserRecBoleEmbedding, modelVersion string, limit int) ([]ResultItem, error) {
	candidateLimit := profileCandidateLimit(limit)
	candidates, err := repo.FindRecommendedSegmentsForRecBole(ctx, RecBoleQuery{
		UserID:       userID,
		UserVector:   pgvector.NewVector(userEmbedding.Vector),
		ModelVersion: modelVersion,
		Limit:        candidateLimit,
	})
	if err != nil {
		return nil, err
	}
	if len(candidates) == 0 {
		return nil, nil
	}
	candidates = s.filterRecentRecBoleCandidates(ctx, userID, candidates)
	if len(candidates) == 0 {
		return nil, nil
	}
	items := BuildRecBoleRankedItems(candidates, limit)
	now := s.Now()
	exposures := make([]ExposureRecord, 0, len(items))
	requestID := s.newRequestID()
	for i := range items {
		items[i].QuestionID = input.QuestionID
		items[i] = withRecommendationSource(items[i], StrategyRecBole, modelVersion)
		s.markRecentReturned(ctx, userID, items[i].VideoSegmentID)
		if err := s.Repo.SaveUserVideoRecommendation(ctx, userID, input.QuestionID, items[i].VideoID, items[i].VideoSegmentID, items[i].RecommendScore, now); err != nil {
			return nil, err
		}
		exposures = append(exposures, buildExposureRecord(requestID, userID, input.QuestionID, items[i].VideoID, items[i].VideoSegmentID, i+1, items[i].RecommendScore, StrategyRecBole, modelVersion, now))
	}
	if err := s.saveRecommendationExposures(ctx, exposures); err != nil {
		return nil, err
	}
	return items, nil
}

func (s Service) recommendByProfileRerank(ctx context.Context, repo ProfileRepository, input RecommendByQuestionInput, userID uint64, queryVec pgvector.Vector, profile UserVideoProfile, limit int) ([]ResultItem, error) {
	candidateLimit := profileCandidateLimit(limit)
	candidates, err := repo.FindRecommendedSegmentsForProfileRerank(ctx, ProfileRerankQuery{
		UserID:         userID,
		QuestionVector: queryVec,
		ProfileVector:  pgvector.NewVector(profile.ProfileVector),
		Limit:          candidateLimit,
	})
	if err != nil {
		return nil, err
	}
	if len(candidates) == 0 {
		return nil, nil
	}
	candidates = s.filterRecentProfileCandidates(ctx, userID, candidates)
	if len(candidates) == 0 {
		return nil, nil
	}
	items := RerankProfileCandidates(candidates, limit)
	now := s.Now()
	exposures := make([]ExposureRecord, 0, len(items))
	requestID := s.newRequestID()
	for i := range items {
		items[i].QuestionID = input.QuestionID
		items[i] = withRecommendationSource(items[i], StrategyProfileRerank, DefaultProfileModelVersion)
		s.markRecentReturned(ctx, userID, items[i].VideoSegmentID)
		if err := s.Repo.SaveUserVideoRecommendation(ctx, userID, input.QuestionID, items[i].VideoID, items[i].VideoSegmentID, items[i].RecommendScore, now); err != nil {
			return nil, err
		}
		exposures = append(exposures, buildExposureRecord(requestID, userID, input.QuestionID, items[i].VideoID, items[i].VideoSegmentID, i+1, items[i].RecommendScore, StrategyProfileRerank, DefaultProfileModelVersion, now))
	}
	if err := s.saveRecommendationExposures(ctx, exposures); err != nil {
		return nil, err
	}
	return items, nil
}

func profileCandidateLimit(limit int) int {
	if limit <= 0 {
		limit = 10
	}
	candidateLimit := limit * 10
	if candidateLimit < 100 {
		candidateLimit = 100
	}
	if candidateLimit > MaxProfileRerankCandidates {
		candidateLimit = MaxProfileRerankCandidates
	}
	return candidateLimit
}

func (s Service) filterRecentCandidates(ctx context.Context, userID uint64, candidates []Candidate) []Candidate {
	if s.RecentSegments == nil || userID == 0 || len(candidates) == 0 || s.recentSegmentTTL() <= 0 {
		return candidates
	}
	ids := make([]uint64, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate.VideoSegmentID == 0 {
			continue
		}
		ids = append(ids, candidate.VideoSegmentID)
	}
	if len(ids) == 0 {
		return candidates
	}
	recent, err := s.RecentSegments.FilterRecent(ctx, userID, ids)
	if err != nil || len(recent) == 0 {
		return candidates
	}
	filtered := make([]Candidate, 0, len(candidates))
	for _, candidate := range candidates {
		if recent[candidate.VideoSegmentID] {
			continue
		}
		filtered = append(filtered, candidate)
	}
	return filtered
}

func (s Service) filterRecentRecBoleCandidates(ctx context.Context, userID uint64, candidates []RecBoleCandidate) []RecBoleCandidate {
	base := make([]Candidate, 0, len(candidates))
	for _, candidate := range candidates {
		base = append(base, candidate.Candidate)
	}
	filteredBase := s.filterRecentCandidates(ctx, userID, base)
	if len(filteredBase) == len(base) {
		return candidates
	}
	kept := make(map[uint64]bool, len(filteredBase))
	for _, candidate := range filteredBase {
		kept[candidate.VideoSegmentID] = true
	}
	filtered := make([]RecBoleCandidate, 0, len(filteredBase))
	for _, candidate := range candidates {
		if kept[candidate.VideoSegmentID] {
			filtered = append(filtered, candidate)
		}
	}
	return filtered
}

func (s Service) filterRecentProfileCandidates(ctx context.Context, userID uint64, candidates []ProfileCandidate) []ProfileCandidate {
	base := make([]Candidate, 0, len(candidates))
	for _, candidate := range candidates {
		base = append(base, candidate.Candidate)
	}
	filteredBase := s.filterRecentCandidates(ctx, userID, base)
	if len(filteredBase) == len(base) {
		return candidates
	}
	kept := make(map[uint64]bool, len(filteredBase))
	for _, candidate := range filteredBase {
		kept[candidate.VideoSegmentID] = true
	}
	filtered := make([]ProfileCandidate, 0, len(filteredBase))
	for _, candidate := range candidates {
		if kept[candidate.VideoSegmentID] {
			filtered = append(filtered, candidate)
		}
	}
	return filtered
}

func (s Service) markRecentReturned(ctx context.Context, userID uint64, segmentID uint64) {
	if s.RecentSegments == nil || userID == 0 || segmentID == 0 {
		return
	}
	ttl := s.recentSegmentTTL()
	if ttl <= 0 {
		return
	}
	_ = s.RecentSegments.MarkReturned(ctx, userID, segmentID, ttl)
}

func (s Service) MarkRecentReturned(ctx context.Context, userID uint64, segmentID uint64) {
	s.markRecentReturned(ctx, userID, segmentID)
}

func (s Service) recentSegmentTTL() time.Duration {
	if s.RecentSegmentTTL > 0 {
		return s.RecentSegmentTTL
	}
	return 30 * time.Minute
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
	now := s.Now()
	created, err := s.Repo.SaveWatchRecord(ctx, userID, videoID, input.QuestionID, input.VideoSegmentID, input.IsWatched, input.WatchDuration, now)
	if err != nil {
		return err
	}
	if exposureRepo, ok := s.Repo.(ExposureRepository); ok {
		if err := exposureRepo.MarkRecommendationExposureWatched(ctx, userID, input.QuestionID, input.VideoSegmentID, now); err != nil {
			return err
		}
	}
	if !created || alreadyCounted {
		return nil
	}
	if _, _, err := s.Repo.IncrementViewCount(ctx, videoID); err != nil {
		return err
	}
	return nil
}

func (s Service) saveRecommendationExposures(ctx context.Context, exposures []ExposureRecord) error {
	if len(exposures) == 0 {
		return nil
	}
	exposureRepo, ok := s.Repo.(ExposureRepository)
	if !ok {
		return nil
	}
	return exposureRepo.SaveRecommendationExposures(ctx, exposures)
}

func (s Service) newRequestID() string {
	if s.NewRequestID != nil {
		if requestID := strings.TrimSpace(s.NewRequestID()); requestID != "" {
			return requestID
		}
	}
	var b [16]byte
	if _, err := rand.Read(b[:]); err == nil {
		return hex.EncodeToString(b[:])
	}
	return fmt.Sprintf("%d", s.Now().UnixNano())
}

func buildExposureRecord(requestID string, userID uint64, questionID uint64, videoID uint64, segmentID uint64, rank int, score float64, strategy string, modelVersion string, now time.Time) ExposureRecord {
	return ExposureRecord{
		RequestID:      requestID,
		UserID:         userID,
		QuestionID:     questionID,
		VideoID:        videoID,
		VideoSegmentID: segmentID,
		Rank:           rank,
		Score:          score,
		Strategy:       strategy,
		ModelVersion:   modelVersion,
		Now:            now,
	}
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

func uniquePositiveUint64s(values []uint64) []uint64 {
	seen := make(map[uint64]bool, len(values))
	out := make([]uint64, 0, len(values))
	for _, value := range values {
		if value == 0 || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func gorseRankScore(rankZeroBased int) float64 {
	if rankZeroBased < 0 {
		rankZeroBased = 0
	}
	return 1 / float64(rankZeroBased+1)
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
		items = append(items, withRecommendationSource(buildResultItem(input.QuestionID, c, score, false, 0), StrategyQuestionVector, ""))
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

func withRecommendationSource(item ResultItem, strategy string, modelVersion string) ResultItem {
	item.Strategy = strategy
	item.ModelVersion = modelVersion
	return item
}
