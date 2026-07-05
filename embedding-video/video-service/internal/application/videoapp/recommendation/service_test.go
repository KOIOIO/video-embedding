package recommendation

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/pgvector/pgvector-go"

	domainvideo "nlp-video-analysis/internal/domain/video"
)

func TestParseVectorTextAcceptsBracketAndParenFormats(t *testing.T) {
	tests := []struct {
		name string
		text string
		want []float32
	}{
		{name: "brackets", text: "[1, 2.5, -3]", want: []float32{1, 2.5, -3}},
		{name: "parens", text: "(4,5)", want: []float32{4, 5}},
		{name: "plain", text: "6, 7", want: []float32{6, 7}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseVectorText(tc.text)
			if err != nil {
				t.Fatalf("ParseVectorText returned error: %v", err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("ParseVectorText(%q) = %#v, want %#v", tc.text, got, tc.want)
			}
		})
	}
}

func TestParseVectorTextRejectsEmptyAndInvalidValues(t *testing.T) {
	for _, text := range []string{"", "[]", "[1, nope]"} {
		t.Run(text, func(t *testing.T) {
			if _, err := ParseVectorText(text); err == nil {
				t.Fatal("expected parse error")
			}
		})
	}
}

func TestNormalizeVectorDimPadsTruncatesAndLeavesDisabledDim(t *testing.T) {
	if got := NormalizeVectorDim([]float32{1, 2}, 4); !reflect.DeepEqual(got, []float32{1, 2, 0, 0}) {
		t.Fatalf("padded vector = %#v", got)
	}
	if got := NormalizeVectorDim([]float32{1, 2, 3}, 2); !reflect.DeepEqual(got, []float32{1, 2}) {
		t.Fatalf("truncated vector = %#v", got)
	}
	source := []float32{1, 2}
	if got := NormalizeVectorDim(source, 0); !reflect.DeepEqual(got, source) {
		t.Fatalf("dim disabled vector = %#v", got)
	}
}

func TestRecommendByQuestionNormalizesDefaultsAndPersistsScores(t *testing.T) {
	now := time.Date(2026, 5, 21, 12, 30, 0, 0, time.UTC)
	repo := &fakeRepository{
		embeddingDim: 3,
		candidates: []Candidate{{
			VideoSegmentID: 22,
			VideoID:        11,
			StartTimeSec:   3,
			EndTimeSec:     9,
			Distance:       0.25,
			SegmentTitle:   "segment",
			Status:         int16(domainvideo.StatusDone),
			VideoURL:       "/videos/raw/2026/05/21/lesson.mp4",
			IsPublished:    true,
		}},
	}
	svc := Service{
		Repo:            repo,
		Embedder:        fakeEmbedder{vector: []float32{1, 2}},
		Now:             func() time.Time { return now },
		InvalidArgument: invalidArgumentError,
	}

	items, err := svc.RecommendByQuestion(context.Background(), RecommendByQuestionInput{QuestionText: " free text "})
	if err != nil {
		t.Fatalf("RecommendByQuestion returned error: %v", err)
	}
	if repo.findLimit != 10 {
		t.Fatalf("find limit = %d, want default 10", repo.findLimit)
	}
	if !reflect.DeepEqual(repo.queryVec.Slice(), []float32{1, 2, 0}) {
		t.Fatalf("query vector = %#v, want padded vector", repo.queryVec.Slice())
	}
	if len(repo.saved) != 1 {
		t.Fatalf("saved count = %d, want 1", len(repo.saved))
	}
	if repo.saved[0].userID != 1 {
		t.Fatalf("saved userID = %d, want default 1", repo.saved[0].userID)
	}
	if repo.saved[0].score != 0.8 {
		t.Fatalf("score = %v, want 0.8", repo.saved[0].score)
	}
	if len(items) != 1 {
		t.Fatalf("items = %d, want 1", len(items))
	}
	if items[0].Video.Status != domainvideo.StatusDone || items[0].TitleOverride != "segment" {
		t.Fatalf("item = %+v", items[0])
	}
}

func TestRecommendByQuestionRecordsQuestionVectorExposuresWithRank(t *testing.T) {
	now := time.Date(2026, 6, 22, 10, 0, 0, 0, time.UTC)
	repo := &fakeRepository{
		embeddingDim: 2,
		questionText: "[1,0]",
		candidates: []Candidate{
			{VideoSegmentID: 22, VideoID: 11, Distance: 0.25},
			{VideoSegmentID: 44, VideoID: 33, Distance: 1.0},
		},
	}
	svc := Service{
		Repo:            repo,
		Now:             func() time.Time { return now },
		InvalidArgument: invalidArgumentError,
		NewRequestID:    func() string { return "req-fixed" },
	}

	if _, err := svc.RecommendByQuestion(context.Background(), RecommendByQuestionInput{QuestionID: 99, UserID: 7, Limit: 2}); err != nil {
		t.Fatalf("RecommendByQuestion returned error: %v", err)
	}

	if len(repo.savedExposures) != 2 {
		t.Fatalf("saved exposures = %d, want 2", len(repo.savedExposures))
	}
	first := repo.savedExposures[0]
	if first.RequestID != "req-fixed" || first.UserID != 7 || first.QuestionID != 99 || first.VideoID != 11 || first.VideoSegmentID != 22 {
		t.Fatalf("first exposure = %+v", first)
	}
	if first.Rank != 1 || first.Score != 0.8 || first.Strategy != StrategyQuestionVector || first.ModelVersion != "" || first.Now != now {
		t.Fatalf("first exposure metadata = %+v", first)
	}
	second := repo.savedExposures[1]
	if second.Rank != 2 || second.VideoSegmentID != 44 || second.Score != 0.5 {
		t.Fatalf("second exposure = %+v", second)
	}
}

func TestRecommendByQuestionUsesQuestionEmbeddingText(t *testing.T) {
	repo := &fakeRepository{
		embeddingDim: 2,
		questionText: "[3, 4, 5]",
		candidates: []Candidate{{
			VideoSegmentID: 44,
			VideoID:        33,
			Distance:       -1,
		}},
	}
	svc := Service{
		Repo:            repo,
		Now:             time.Now,
		InvalidArgument: invalidArgumentError,
		Engine:          EngineTwoTower,
	}

	items, err := svc.RecommendByQuestion(context.Background(), RecommendByQuestionInput{QuestionID: 99, Limit: 100})
	if err != nil {
		t.Fatalf("RecommendByQuestion returned error: %v", err)
	}
	if repo.questionID != 99 {
		t.Fatalf("questionID = %d, want 99", repo.questionID)
	}
	if repo.findLimit != 50 {
		t.Fatalf("find limit = %d, want capped 50", repo.findLimit)
	}
	if !reflect.DeepEqual(repo.queryVec.Slice(), []float32{3, 4}) {
		t.Fatalf("query vector = %#v, want truncated question vector", repo.queryVec.Slice())
	}
	if items[0].RecommendScore != 0 {
		t.Fatalf("negative distance score = %v, want 0", items[0].RecommendScore)
	}
}

func TestRecommendByQuestionIgnoresUserVideoProfileWhenAvailable(t *testing.T) {
	now := time.Date(2026, 6, 22, 12, 0, 0, 0, time.UTC)
	repo := &fakeRepository{
		embeddingDim: 2,
		questionText: "[1,0]",
		candidates: []Candidate{{
			VideoSegmentID: 3,
			VideoID:        30,
			Distance:       0.25,
		}},
		profile: UserVideoProfile{
			UserID:        7,
			ProfileVector: []float32{0, 1},
			ModelVersion:  DefaultProfileModelVersion,
			Status:        1,
			PositiveCount: 1,
		},
		profileFound: true,
		profileCandidates: []ProfileCandidate{
			{
				Candidate:       Candidate{VideoSegmentID: 1, VideoID: 10, Distance: 0.20},
				ProfileDistance: 0.80,
			},
			{
				Candidate:       Candidate{VideoSegmentID: 2, VideoID: 20, Distance: 0.24},
				ProfileDistance: 0.05,
			},
		},
	}
	svc := Service{
		Repo:            repo,
		Now:             func() time.Time { return now },
		InvalidArgument: invalidArgumentError,
	}

	items, err := svc.RecommendByQuestion(context.Background(), RecommendByQuestionInput{QuestionID: 99, UserID: 7, Limit: 1})
	if err != nil {
		t.Fatalf("RecommendByQuestion returned error: %v", err)
	}
	if repo.profileRequested {
		t.Fatal("profile rerank should not be requested for by-question matching")
	}
	if repo.profileFindLimit != 0 {
		t.Fatalf("profile candidate limit = %d, want 0", repo.profileFindLimit)
	}
	if repo.findLimit != 1 {
		t.Fatalf("question vector find limit = %d, want 1", repo.findLimit)
	}
	if len(items) != 1 || items[0].VideoSegmentID != 3 {
		t.Fatalf("items = %+v, want question-vector segment 3", items)
	}
	if len(repo.saved) != 1 || repo.saved[0].segmentID != 3 {
		t.Fatalf("saved = %+v, want segment 3", repo.saved)
	}
	if repo.saved[0].score <= 0 {
		t.Fatalf("saved score = %v, want positive question-vector score", repo.saved[0].score)
	}
	if len(repo.savedExposures) != 1 {
		t.Fatalf("saved exposures = %d, want 1", len(repo.savedExposures))
	}
	if got := repo.savedExposures[0].Strategy; got != StrategyQuestionVector {
		t.Fatalf("exposure strategy = %q, want %q", got, StrategyQuestionVector)
	}
	if got := repo.savedExposures[0].ModelVersion; got != "" {
		t.Fatalf("exposure model version = %q, want empty", got)
	}
}

func TestRecommendByQuestionIgnoresTwoTowerRecallWhenUserEmbeddingAvailable(t *testing.T) {
	now := time.Date(2026, 6, 23, 10, 0, 0, 0, time.UTC)
	repo := &fakeRepository{
		embeddingDim:  2,
		questionText:  "[1,0]",
		candidates:    []Candidate{{VideoSegmentID: 103, VideoID: 13, Distance: 0.20}},
		twoTowerFound: true,
		twoTowerEmbedding: UserTowerEmbedding{
			UserID:       7,
			Vector:       []float32{0.2, 0.8},
			ModelVersion: DefaultTwoTowerModelVersion,
			Status:       1,
		},
		twoTowerCandidates: []TwoTowerCandidate{
			{
				Candidate: Candidate{VideoSegmentID: 101, VideoID: 11, Distance: 0.10},
			},
			{
				Candidate: Candidate{VideoSegmentID: 102, VideoID: 12, Distance: 0.25},
			},
		},
		profile: UserVideoProfile{
			UserID:        7,
			ProfileVector: []float32{0, 1},
			ModelVersion:  DefaultProfileModelVersion,
			Status:        1,
			PositiveCount: 1,
		},
		profileFound: true,
	}
	svc := Service{
		Repo:            repo,
		Now:             func() time.Time { return now },
		InvalidArgument: invalidArgumentError,
		NewRequestID:    func() string { return "req-two-tower" },
	}

	items, err := svc.RecommendByQuestion(context.Background(), RecommendByQuestionInput{QuestionID: 99, UserID: 7, Limit: 1})
	if err != nil {
		t.Fatalf("RecommendByQuestion returned error: %v", err)
	}
	if repo.twoTowerRequested {
		t.Fatal("two tower recall should not be requested for by-question matching")
	}
	if repo.twoTowerFindLimit != 0 {
		t.Fatalf("two tower find limit = %d, want 0", repo.twoTowerFindLimit)
	}
	if repo.profileRequested {
		t.Fatal("profile rerank should not be requested for by-question matching")
	}
	if repo.findLimit != 1 {
		t.Fatalf("question vector find limit = %d, want 1", repo.findLimit)
	}
	if len(items) != 1 || items[0].VideoSegmentID != 103 {
		t.Fatalf("items = %+v, want question-vector segment 103", items)
	}
	if len(repo.saved) != 1 || repo.saved[0].segmentID != 103 {
		t.Fatalf("saved = %+v, want segment 103", repo.saved)
	}
	if len(repo.savedExposures) != 1 {
		t.Fatalf("saved exposures = %d, want 1", len(repo.savedExposures))
	}
	exposure := repo.savedExposures[0]
	if exposure.RequestID != "req-two-tower" || exposure.Strategy != StrategyQuestionVector || exposure.ModelVersion != "" {
		t.Fatalf("exposure = %+v, want question vector metadata", exposure)
	}
}

func TestRandomPlayUsesActiveTwoTowerModelVersion(t *testing.T) {
	repo := &fakeRepository{
		activeTwoTowerVersion: "two_tower_v2",
		activeTwoTowerFound:   true,
		expectedTowerVersion:  "two_tower_v2",
		expectedRecallVersion: "two_tower_v2",
		twoTowerFound:         true,
		twoTowerEmbedding:     UserTowerEmbedding{UserID: 7, Vector: []float32{0.2, 0.8}, ModelVersion: "two_tower_v2", Status: 1},
		twoTowerCandidates:    []TwoTowerCandidate{{Candidate: Candidate{VideoSegmentID: 201, VideoID: 21, Distance: 0.10}}},
	}
	svc := Service{
		Repo:            repo,
		Now:             time.Now,
		InvalidArgument: invalidArgumentError,
		Engine:          EngineTwoTower,
	}

	items, err := svc.RandomPlay(context.Background(), RandomPlayInput{UserID: 7, Limit: 1})
	if err != nil {
		t.Fatalf("RandomPlay returned error: %v", err)
	}
	if len(items) != 1 || items[0].QuestionID != 0 || items[0].VideoSegmentID != 201 {
		t.Fatalf("items = %+v, want random-play two tower segment 201", items)
	}
	if !repo.activeTwoTowerRequested {
		t.Fatal("expected active two tower version to be requested")
	}
	if len(repo.savedExposures) != 1 {
		t.Fatalf("saved exposures = %d, want 1", len(repo.savedExposures))
	}
	if got := repo.savedExposures[0].ModelVersion; got != "two_tower_v2" {
		t.Fatalf("exposure model version = %q, want two_tower_v2", got)
	}
	if got := repo.savedExposures[0].Strategy; got != StrategyTwoTower {
		t.Fatalf("exposure strategy = %q, want %q", got, StrategyTwoTower)
	}
}

func TestRandomPlayUsesKnowledgeMatchByDefault(t *testing.T) {
	now := time.Date(2026, 6, 26, 12, 0, 0, 0, time.UTC)
	repo := &fakeRepository{
		embeddingDim: 3,
		weakKnowledge: []WeakKnowledge{{
			KnowledgePointID: 1,
			Mastery:          0.1,
			Name:             "一次函数",
			Description:      "图像与斜率",
		}},
		knowledgeVectorCandidates: []Candidate{{
			VideoSegmentID: 501,
			VideoID:        51,
			Distance:       0.2,
			SegmentTitle:   "weak knowledge match",
			Status:         int16(domainvideo.StatusDone),
			IsPublished:    true,
		}},
		twoTowerFound:      true,
		twoTowerEmbedding:  UserTowerEmbedding{UserID: 7, Vector: []float32{0.2, 0.8}, ModelVersion: DefaultTwoTowerModelVersion, Status: 1},
		twoTowerCandidates: []TwoTowerCandidate{{Candidate: Candidate{VideoSegmentID: 999, VideoID: 99, Distance: 0.1}}},
	}
	gorse := &fakeGorseClient{ids: []uint64{101}}
	embedder := &recordingEmbedder{vector: []float32{1, 2, 3}}
	svc := Service{
		Repo:            repo,
		Now:             func() time.Time { return now },
		InvalidArgument: invalidArgumentError,
		Gorse:           gorse,
		Embedder:        embedder,
		NewRequestID:    func() string { return "req-knowledge" },
	}

	items, err := svc.RandomPlay(context.Background(), RandomPlayInput{UserID: 7, Limit: 1})
	if err != nil {
		t.Fatalf("RandomPlay returned error: %v", err)
	}
	if len(items) != 1 || items[0].VideoSegmentID != 501 {
		t.Fatalf("items = %+v, want knowledge match segment 501", items)
	}
	if repo.weakKnowledgeUserID != 7 || repo.weakKnowledgeLimit != 10 {
		t.Fatalf("weak knowledge inputs userID=%d weakLimit=%d", repo.weakKnowledgeUserID, repo.weakKnowledgeLimit)
	}
	if repo.knowledgeVectorUserID != 7 || repo.knowledgeVectorLimit != WeakKnowledgeRecallLimit {
		t.Fatalf("knowledge vector inputs userID=%d limit=%d", repo.knowledgeVectorUserID, repo.knowledgeVectorLimit)
	}
	if got := repo.knowledgeVectorQuery.Slice(); !reflect.DeepEqual(got, []float32{1, 2, 3}) {
		t.Fatalf("knowledge query vector = %#v, want embedder vector", got)
	}
	if len(embedder.texts) != 1 || !strings.Contains(embedder.texts[0], "一次函数") || !strings.Contains(embedder.texts[0], "图像与斜率") {
		t.Fatalf("embedder texts = %#v, want weak knowledge name and description", embedder.texts)
	}
	if repo.twoTowerRequested {
		t.Fatal("two tower should not be requested by default knowledge_match engine")
	}
	if gorse.userID != 0 {
		t.Fatalf("gorse userID = %d, want no call", gorse.userID)
	}
	if len(repo.savedExposures) != 1 || repo.savedExposures[0].Strategy != StrategyKnowledgeMatch {
		t.Fatalf("exposures = %+v, want knowledge_match strategy", repo.savedExposures)
	}
}

func TestRandomPlayKnowledgeMatchReranksWeakKnowledgeTextAboveUnmatchedVectorHit(t *testing.T) {
	repo := &fakeRepository{
		embeddingDim: 3,
		weakKnowledge: []WeakKnowledge{{
			KnowledgePointID: 1,
			Mastery:          0.1,
			Name:             "一次函数",
			Description:      "图像 斜率",
		}},
		knowledgeVectorCandidates: []Candidate{
			{
				VideoSegmentID: 701,
				VideoID:        71,
				Distance:       0.01,
				SegmentTitle:   "语文阅读理解：文章结构分析",
				KnowledgeTags:  "阅读理解,文章结构",
			},
			{
				VideoSegmentID: 702,
				VideoID:        72,
				Distance:       0.2,
				SegmentTitle:   "一次函数图像与斜率",
				KnowledgeTags:  "一次函数,图像,斜率",
			},
		},
	}
	svc := Service{
		Repo:            repo,
		Now:             time.Now,
		InvalidArgument: invalidArgumentError,
		Embedder:        &recordingEmbedder{vector: []float32{1, 2, 3}},
	}

	items, err := svc.RandomPlay(context.Background(), RandomPlayInput{UserID: 6, Limit: 1})
	if err != nil {
		t.Fatalf("RandomPlay returned error: %v", err)
	}
	if len(items) != 1 || items[0].VideoSegmentID != 702 {
		t.Fatalf("items = %+v, want weak-knowledge text matched segment 702", items)
	}
	if repo.knowledgeVectorLimit <= 1 {
		t.Fatalf("knowledge vector limit = %d, want expanded recall pool", repo.knowledgeVectorLimit)
	}
}

func TestRandomPlayKnowledgeMatchFallsBackToPlayableEmbeddingPoolWhenRecommendPoolEmpty(t *testing.T) {
	repo := &fakeRepository{
		embeddingDim: 3,
		weakKnowledge: []WeakKnowledge{{
			KnowledgePointID: 4361,
			Mastery:          0.15,
			Name:             "平面图形的初步认识",
			Description:      "认识多边形、圆等基本的平面图形。",
		}},
		knowledgeVectorCandidatesByRequireRecommend: map[bool][]Candidate{
			true: nil,
			false: {
				{
					VideoSegmentID: 66,
					VideoID:        4,
					Distance:       0.3,
					SegmentTitle:   "相交线与平行线考点总结",
					KnowledgeTags:  "平面图形,直线,相交线,平行线",
					Status:         int16(domainvideo.StatusDone),
					IsPublished:    true,
					IsRecommend:    false,
				},
			},
		},
	}
	svc := Service{
		Repo:     repo,
		Embedder: fakeEmbedder{vector: []float32{1, 0, 0}},
		Now:      time.Now,
	}

	items, err := svc.RandomPlay(context.Background(), RandomPlayInput{UserID: 6, Limit: 1})
	if err != nil {
		t.Fatalf("RandomPlay returned error: %v", err)
	}
	if len(items) != 1 || items[0].VideoSegmentID != 66 {
		t.Fatalf("items = %+v, want weak-knowledge candidate from playable embedding pool", items)
	}
	if !reflect.DeepEqual(repo.knowledgeVectorRequireRecommendCalls, []bool{true, false}) {
		t.Fatalf("requireRecommend calls = %v, want strict pool then playable pool", repo.knowledgeVectorRequireRecommendCalls)
	}
	if len(repo.savedExposures) != 1 || repo.savedExposures[0].Strategy != StrategyKnowledgeMatch {
		t.Fatalf("saved exposures = %+v, want knowledge_match exposure", repo.savedExposures)
	}
}

func TestRandomPlaySkipsRecentlyReturnedSegment(t *testing.T) {
	repo := &fakeRepository{
		embeddingDim: 3,
		weakKnowledge: []WeakKnowledge{{
			KnowledgePointID: 1,
			Mastery:          0.1,
			Name:             "一次函数",
			Description:      "图像 斜率",
		}},
		knowledgeVectorCandidates: []Candidate{
			{
				VideoSegmentID: 101,
				VideoID:        11,
				SegmentTitle:   "一次函数图像",
				Status:         int16(domainvideo.StatusDone),
				IsPublished:    true,
				IsRecommend:    true,
			},
			{
				VideoSegmentID: 102,
				VideoID:        12,
				SegmentTitle:   "一次函数斜率",
				Status:         int16(domainvideo.StatusDone),
				IsPublished:    true,
				IsRecommend:    true,
			},
		},
	}
	recency := &fakeRecentSegmentStore{recent: map[uint64]bool{101: true}}
	svc := Service{
		Repo:             repo,
		Embedder:         fakeEmbedder{vector: []float32{1, 0, 0}},
		Now:              time.Now,
		RecentSegments:   recency,
		RecentSegmentTTL: 30 * time.Minute,
	}

	items, err := svc.RandomPlay(context.Background(), RandomPlayInput{UserID: 6, Limit: 1})
	if err != nil {
		t.Fatalf("RandomPlay returned error: %v", err)
	}
	if len(items) != 1 || items[0].VideoSegmentID != 102 {
		t.Fatalf("items = %+v, want only non-recent segment 102", items)
	}
	if len(repo.saved) != 1 || repo.saved[0].segmentID != 102 {
		t.Fatalf("saved recommendations = %+v, want segment 102 only", repo.saved)
	}
	if len(repo.savedExposures) != 1 || repo.savedExposures[0].VideoSegmentID != 102 {
		t.Fatalf("saved exposures = %+v, want segment 102 only", repo.savedExposures)
	}
	if len(recency.marked) != 1 || recency.marked[0] != 102 {
		t.Fatalf("marked recent segments = %v, want [102]", recency.marked)
	}
	if recency.lastUserID != 6 || recency.lastTTL != 30*time.Minute {
		t.Fatalf("recent store userID=%d ttl=%s, want user 6 ttl 30m", recency.lastUserID, recency.lastTTL)
	}
}

func TestRandomPlayKnowledgeMatchDoesNotFallBackToTwoTower(t *testing.T) {
	repo := &fakeRepository{
		twoTowerFound:      true,
		twoTowerEmbedding:  UserTowerEmbedding{UserID: 7, Vector: []float32{0.2, 0.8}, ModelVersion: DefaultTwoTowerModelVersion, Status: 1},
		twoTowerCandidates: []TwoTowerCandidate{{Candidate: Candidate{VideoSegmentID: 999, VideoID: 99, Distance: 0.1}}},
	}
	svc := Service{
		Repo:            repo,
		Now:             time.Now,
		InvalidArgument: invalidArgumentError,
	}

	items, err := svc.RandomPlay(context.Background(), RandomPlayInput{UserID: 7, Limit: 1})
	if err != nil {
		t.Fatalf("RandomPlay returned error: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("items = %+v, want empty so outer random fallback can run", items)
	}
	if repo.twoTowerRequested {
		t.Fatal("two tower should not be requested when knowledge match has no candidates")
	}
}

func TestRandomPlayKnowledgeMatchDoesNotUseLegacyKeywordFallback(t *testing.T) {
	repo := &fakeRepository{
		knowledgeCandidates: []Candidate{{
			VideoSegmentID: 501,
			VideoID:        51,
			Distance:       0.2,
		}},
	}
	svc := Service{
		Repo:            repo,
		Now:             time.Now,
		InvalidArgument: invalidArgumentError,
	}

	items, err := svc.RandomPlay(context.Background(), RandomPlayInput{UserID: 7, Limit: 1})
	if err != nil {
		t.Fatalf("RandomPlay returned error: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("items = %+v, want empty without weak knowledge embedding", items)
	}
	if repo.knowledgeUserID != 0 {
		t.Fatalf("legacy keyword fallback userID=%d, want no call", repo.knowledgeUserID)
	}
}

func TestRandomPlayUsesGorseWhenEngineEnabled(t *testing.T) {
	now := time.Date(2026, 6, 26, 11, 0, 0, 0, time.UTC)
	repo := &fakeRepository{
		hydratedCandidates: []Candidate{
			{VideoSegmentID: 101, VideoID: 11, SegmentTitle: "first", Status: int16(domainvideo.StatusDone), IsPublished: true},
			{VideoSegmentID: 202, VideoID: 22, SegmentTitle: "second", Status: int16(domainvideo.StatusDone), IsPublished: true},
		},
	}
	gorse := &fakeGorseClient{ids: []uint64{101, 0, 202}}
	svc := Service{
		Repo:            repo,
		Now:             func() time.Time { return now },
		InvalidArgument: invalidArgumentError,
		Gorse:           gorse,
		Engine:          EngineGorse,
		GorseOptions: GorseOptions{
			CandidateLimit:   3,
			WriteBackEnabled: true,
		},
		NewRequestID: func() string { return "req-gorse" },
	}

	items, err := svc.RandomPlay(context.Background(), RandomPlayInput{UserID: 7, Limit: 2})
	if err != nil {
		t.Fatalf("RandomPlay returned error: %v", err)
	}
	if len(items) != 2 || items[0].VideoSegmentID != 101 || items[1].VideoSegmentID != 202 {
		t.Fatalf("items = %+v, want gorse hydrated 101,202", items)
	}
	if gorse.userID != 7 || gorse.n != 3 {
		t.Fatalf("gorse call userID=%d n=%d, want 7 3", gorse.userID, gorse.n)
	}
	if repo.hydrateUserID != 7 || !reflect.DeepEqual(repo.hydrateIDs, []uint64{101, 202}) {
		t.Fatalf("hydrate userID=%d ids=%v, want 7 [101 202]", repo.hydrateUserID, repo.hydrateIDs)
	}
	if repo.twoTowerRequested {
		t.Fatal("two tower should not be requested when gorse succeeds")
	}
	if len(repo.saved) != 2 || repo.saved[0].segmentID != 101 || repo.saved[1].segmentID != 202 {
		t.Fatalf("saved = %+v, want gorse recommendations", repo.saved)
	}
	if len(repo.savedExposures) != 2 {
		t.Fatalf("exposures = %d, want 2", len(repo.savedExposures))
	}
	if repo.savedExposures[0].RequestID != "req-gorse" || repo.savedExposures[0].Strategy != StrategyGorse || repo.savedExposures[0].ModelVersion != GorseModelVersion {
		t.Fatalf("first exposure = %+v, want gorse metadata", repo.savedExposures[0])
	}
	if len(gorse.feedback) != 2 || gorse.feedback[0].FeedbackType != "exposure" || gorse.feedback[0].UserID != "7" || gorse.feedback[0].ItemID != "101" {
		t.Fatalf("feedback = %+v, want exposure feedback", gorse.feedback)
	}
}

func TestRandomPlayGorseEngineDoesNotFallBackToTwoTowerWhenGorseReturnsTooFewCandidates(t *testing.T) {
	repo := &fakeRepository{
		activeTwoTowerVersion: "two_tower_v2",
		activeTwoTowerFound:   true,
		twoTowerFound:         true,
		twoTowerEmbedding:     UserTowerEmbedding{UserID: 7, Vector: []float32{0.2, 0.8}, ModelVersion: "two_tower_v2", Status: 1},
		twoTowerCandidates:    []TwoTowerCandidate{{Candidate: Candidate{VideoSegmentID: 301, VideoID: 31, Distance: 0.10}}},
	}
	gorse := &fakeGorseClient{ids: []uint64{999}}
	svc := Service{
		Repo:            repo,
		Now:             time.Now,
		InvalidArgument: invalidArgumentError,
		Gorse:           gorse,
		Engine:          EngineGorse,
		GorseOptions: GorseOptions{
			CandidateLimit:    3,
			MinRecommendItems: 1,
		},
	}

	items, err := svc.RandomPlay(context.Background(), RandomPlayInput{UserID: 7, Limit: 1})
	if err != nil {
		t.Fatalf("RandomPlay returned error: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("items = %+v, want empty so outer random fallback can run", items)
	}
	if repo.twoTowerRequested {
		t.Fatal("two tower should not be requested when gorse engine is sparse")
	}
}

func TestRandomPlayTwoTowerEngineBypassesGorse(t *testing.T) {
	repo := &fakeRepository{
		activeTwoTowerVersion: "two_tower_v2",
		activeTwoTowerFound:   true,
		twoTowerFound:         true,
		twoTowerEmbedding:     UserTowerEmbedding{UserID: 7, Vector: []float32{0.2, 0.8}, ModelVersion: "two_tower_v2", Status: 1},
		twoTowerCandidates:    []TwoTowerCandidate{{Candidate: Candidate{VideoSegmentID: 401, VideoID: 41, Distance: 0.10}}},
	}
	gorse := &fakeGorseClient{ids: []uint64{101}}
	svc := Service{
		Repo:            repo,
		Now:             time.Now,
		InvalidArgument: invalidArgumentError,
		Gorse:           gorse,
		Engine:          EngineTwoTower,
	}

	items, err := svc.RandomPlay(context.Background(), RandomPlayInput{UserID: 7, Limit: 1})
	if err != nil {
		t.Fatalf("RandomPlay returned error: %v", err)
	}
	if gorse.userID != 0 {
		t.Fatalf("gorse userID = %d, want no call", gorse.userID)
	}
	if len(items) != 1 || items[0].VideoSegmentID != 401 {
		t.Fatalf("items = %+v, want two tower segment 401", items)
	}
}

func TestRecommendByQuestionUsesQuestionVectorWhenUserProfileIsMissing(t *testing.T) {
	repo := &fakeRepository{
		embeddingDim: 2,
		questionText: "[1,0]",
		candidates: []Candidate{{
			VideoSegmentID: 11,
			VideoID:        22,
			Distance:       0.25,
		}},
	}
	svc := Service{
		Repo:            repo,
		Now:             time.Now,
		InvalidArgument: invalidArgumentError,
	}

	items, err := svc.RecommendByQuestion(context.Background(), RecommendByQuestionInput{QuestionID: 99, UserID: 7, Limit: 1})
	if err != nil {
		t.Fatalf("RecommendByQuestion returned error: %v", err)
	}
	if repo.profileRequested {
		t.Fatal("profile should not be requested for by-question matching")
	}
	if repo.findLimit != 1 {
		t.Fatalf("question vector find limit = %d, want 1", repo.findLimit)
	}
	if len(items) != 1 || items[0].VideoSegmentID != 11 {
		t.Fatalf("items = %+v, want question-vector segment 11", items)
	}
}

func TestRecommendByQuestionRejectsMissingQuestionInput(t *testing.T) {
	errInvalid := errors.New("invalid")
	svc := Service{
		Repo: &fakeRepository{},
		InvalidArgument: func(message string) error {
			if message != "question_text is required when question_id is absent" {
				t.Fatalf("message = %q", message)
			}
			return errInvalid
		},
	}

	_, err := svc.RecommendByQuestion(context.Background(), RecommendByQuestionInput{})
	if !errors.Is(err, errInvalid) {
		t.Fatalf("err = %v, want %v", err, errInvalid)
	}
}

func TestReportWatchIncrementsOnlyNewParentVideoWatch(t *testing.T) {
	now := time.Unix(1, 0)
	repo := &fakeRepository{segmentVideoID: 17, watchCreated: true}
	updater := &fakeProfileUpdater{}
	svc := Service{
		Repo:                  repo,
		Now:                   func() time.Time { return now },
		InvalidArgument:       invalidArgumentError,
		ErrVideoSegmentAbsent: errors.New("segment missing"),
		ProfileUpdater:        updater,
		UserTowerUpdater:      updater,
	}

	err := svc.ReportWatch(context.Background(), ReportWatchInput{
		QuestionID:     3,
		UserID:         7,
		VideoSegmentID: 204,
		IsWatched:      true,
		WatchDuration:  45,
	})
	if err != nil {
		t.Fatalf("ReportWatch returned error: %v", err)
	}
	if repo.incrementedVideoID != 17 {
		t.Fatalf("incremented videoID = %d, want 17", repo.incrementedVideoID)
	}
	if repo.savedWatch.videoID != 17 || repo.savedWatch.questionID != 3 || repo.savedWatch.segmentID != 204 {
		t.Fatalf("saved watch = %+v", repo.savedWatch)
	}
	if updater.calls != 1 || updater.lastUserID != 7 {
		t.Fatalf("profile updater calls=%d userID=%d, want one call for user 7", updater.calls, updater.lastUserID)
	}
	if updater.towerCalls != 1 || updater.lastTowerUserID != 7 {
		t.Fatalf("tower updater calls=%d userID=%d, want one call for user 7", updater.towerCalls, updater.lastTowerUserID)
	}
	if repo.markedExposure.userID != 7 || repo.markedExposure.questionID != 3 || repo.markedExposure.segmentID != 204 || repo.markedExposure.now != now {
		t.Fatalf("marked exposure = %+v", repo.markedExposure)
	}
}

func TestReportWatchSkipsIncrementWhenAlreadyCounted(t *testing.T) {
	repo := &fakeRepository{segmentVideoID: 17, alreadyCounted: true, watchCreated: true}
	svc := Service{
		Repo:                  repo,
		Now:                   time.Now,
		InvalidArgument:       invalidArgumentError,
		ErrVideoSegmentAbsent: errors.New("segment missing"),
	}

	if err := svc.ReportWatch(context.Background(), ReportWatchInput{QuestionID: 3, VideoSegmentID: 204}); err != nil {
		t.Fatalf("ReportWatch returned error: %v", err)
	}
	if repo.incrementedVideoID != 0 {
		t.Fatalf("incremented videoID = %d, want 0", repo.incrementedVideoID)
	}
	if repo.savedWatch.userID != 1 {
		t.Fatalf("saved userID = %d, want default 1", repo.savedWatch.userID)
	}
}

type fakeRepository struct {
	embeddingDim                                int
	questionText                                string
	questionID                                  uint64
	candidates                                  []Candidate
	profile                                     UserVideoProfile
	profileFound                                bool
	profileRequested                            bool
	profileCandidates                           []ProfileCandidate
	profileFindLimit                            int
	findLimit                                   int
	queryVec                                    pgvector.Vector
	saved                                       []savedRecommendation
	segmentVideoID                              uint64
	alreadyCounted                              bool
	watchCreated                                bool
	savedWatch                                  savedWatch
	incrementedVideoID                          uint64
	savedExposures                              []ExposureRecord
	markedExposure                              markedExposure
	twoTowerEmbedding                           UserTowerEmbedding
	twoTowerFound                               bool
	twoTowerRequested                           bool
	twoTowerCandidates                          []TwoTowerCandidate
	twoTowerFindLimit                           int
	activeTwoTowerVersion                       string
	activeTwoTowerFound                         bool
	activeTwoTowerRequested                     bool
	expectedTowerVersion                        string
	expectedRecallVersion                       string
	hydrateIDs                                  []uint64
	hydrateUserID                               uint64
	hydratedCandidates                          []Candidate
	weakKnowledge                               []WeakKnowledge
	weakKnowledgeUserID                         uint64
	weakKnowledgeLimit                          int
	knowledgeVectorCandidates                   []Candidate
	knowledgeVectorCandidatesByRequireRecommend map[bool][]Candidate
	knowledgeVectorRequireRecommendCalls        []bool
	knowledgeVectorUserID                       uint64
	knowledgeVectorLimit                        int
	knowledgeVectorQuery                        pgvector.Vector
	knowledgeCandidates                         []Candidate
	knowledgeUserID                             uint64
	knowledgeLimit                              int
	knowledgeWeakLimit                          int
}

func (r *fakeRepository) GetSegmentEmbeddingDim(context.Context) (int, error) {
	return r.embeddingDim, nil
}

func (r *fakeRepository) GetQuestionEmbeddingTextByID(_ context.Context, questionID uint64) (string, error) {
	r.questionID = questionID
	return r.questionText, nil
}

func (r *fakeRepository) FindRecommendedSegments(_ context.Context, query pgvector.Vector, limit int) ([]Candidate, error) {
	r.queryVec = query
	r.findLimit = limit
	return r.candidates, nil
}

func (r *fakeRepository) GetUserVideoProfile(_ context.Context, userID uint64, modelVersion string) (UserVideoProfile, bool, error) {
	r.profileRequested = true
	if r.profile.UserID != 0 && r.profile.UserID != userID {
		return UserVideoProfile{}, false, nil
	}
	if r.profile.ModelVersion != "" && r.profile.ModelVersion != modelVersion {
		return UserVideoProfile{}, false, nil
	}
	return r.profile, r.profileFound, nil
}

func (r *fakeRepository) FindRecommendedSegmentsForProfileRerank(_ context.Context, input ProfileRerankQuery) ([]ProfileCandidate, error) {
	r.queryVec = input.QuestionVector
	r.profileFindLimit = input.Limit
	return r.profileCandidates, nil
}

func (r *fakeRepository) GetUserTowerEmbedding(_ context.Context, userID uint64, modelVersion string) (UserTowerEmbedding, bool, error) {
	r.twoTowerRequested = true
	if r.expectedTowerVersion != "" && modelVersion != r.expectedTowerVersion {
		return UserTowerEmbedding{}, false, errors.New("unexpected tower model version: " + modelVersion)
	}
	if r.twoTowerEmbedding.UserID != 0 && r.twoTowerEmbedding.UserID != userID {
		return UserTowerEmbedding{}, false, nil
	}
	if r.twoTowerEmbedding.ModelVersion != "" && r.twoTowerEmbedding.ModelVersion != modelVersion {
		return UserTowerEmbedding{}, false, nil
	}
	return r.twoTowerEmbedding, r.twoTowerFound, nil
}

func (r *fakeRepository) FindRecommendedSegmentsForTwoTower(_ context.Context, input TwoTowerQuery) ([]TwoTowerCandidate, error) {
	if r.expectedRecallVersion != "" && input.ModelVersion != r.expectedRecallVersion {
		return nil, errors.New("unexpected recall model version: " + input.ModelVersion)
	}
	r.twoTowerFindLimit = input.Limit
	return r.twoTowerCandidates, nil
}

func (r *fakeRepository) HydrateRecommendedSegmentsByID(_ context.Context, userID uint64, ids []uint64) ([]Candidate, error) {
	r.hydrateUserID = userID
	r.hydrateIDs = append([]uint64(nil), ids...)
	return r.hydratedCandidates, nil
}

func (r *fakeRepository) ListWeakKnowledge(ctx context.Context, userID uint64, limit int) ([]WeakKnowledge, error) {
	r.weakKnowledgeUserID = userID
	r.weakKnowledgeLimit = limit
	return r.weakKnowledge, nil
}

func (r *fakeRepository) FindRecommendedSegmentsByWeakKnowledgeVector(_ context.Context, input WeakKnowledgeVectorQuery) ([]Candidate, error) {
	r.knowledgeVectorUserID = input.UserID
	r.knowledgeVectorQuery = input.Query
	r.knowledgeVectorLimit = input.Limit
	r.knowledgeVectorRequireRecommendCalls = append(r.knowledgeVectorRequireRecommendCalls, input.RequireRecommend)
	if r.knowledgeVectorCandidatesByRequireRecommend != nil {
		return r.knowledgeVectorCandidatesByRequireRecommend[input.RequireRecommend], nil
	}
	return r.knowledgeVectorCandidates, nil
}

func (r *fakeRepository) FindRecommendedSegmentsByWeakKnowledge(_ context.Context, userID uint64, limit int, weakLimit int) ([]Candidate, error) {
	r.knowledgeUserID = userID
	r.knowledgeLimit = limit
	r.knowledgeWeakLimit = weakLimit
	return r.knowledgeCandidates, nil
}

func (r *fakeRepository) GetActiveTwoTowerModelVersion(context.Context) (string, bool, error) {
	r.activeTwoTowerRequested = true
	return r.activeTwoTowerVersion, r.activeTwoTowerFound, nil
}

func (r *fakeRepository) SaveUserVideoRecommendation(_ context.Context, userID uint64, questionID uint64, videoID uint64, segmentID uint64, score float64, now time.Time) error {
	r.saved = append(r.saved, savedRecommendation{
		userID:     userID,
		questionID: questionID,
		videoID:    videoID,
		segmentID:  segmentID,
		score:      score,
		now:        now,
	})
	return nil
}

func (r *fakeRepository) SaveRecommendationExposures(_ context.Context, exposures []ExposureRecord) error {
	r.savedExposures = append(r.savedExposures, exposures...)
	return nil
}

func (r *fakeRepository) ListRecommendations(context.Context, uint64, uint64, int) ([]Record, error) {
	panic("unexpected call")
}

func (r *fakeRepository) GetVideoIDBySegmentID(context.Context, uint64) (uint64, error) {
	return r.segmentVideoID, nil
}

func (r *fakeRepository) HasWatchedVideoForQuestion(context.Context, uint64, uint64, uint64) (bool, error) {
	return r.alreadyCounted, nil
}

func (r *fakeRepository) SaveWatchRecord(_ context.Context, userID uint64, videoID uint64, questionID uint64, segmentID uint64, isWatched bool, watchDuration int, now time.Time) (bool, error) {
	r.savedWatch = savedWatch{
		userID:        userID,
		videoID:       videoID,
		questionID:    questionID,
		segmentID:     segmentID,
		isWatched:     isWatched,
		watchDuration: watchDuration,
		now:           now,
	}
	return r.watchCreated, nil
}

func (r *fakeRepository) MarkRecommendationExposureWatched(_ context.Context, userID uint64, questionID uint64, segmentID uint64, now time.Time) error {
	r.markedExposure = markedExposure{
		userID:     userID,
		questionID: questionID,
		segmentID:  segmentID,
		now:        now,
	}
	return nil
}

func (r *fakeRepository) IncrementViewCount(_ context.Context, id uint64) (int, bool, error) {
	r.incrementedVideoID = id
	return 1, true, nil
}

type fakeEmbedder struct {
	vector []float32
}

func (e fakeEmbedder) Embed(context.Context, string) ([]float32, error) {
	return e.vector, nil
}

type recordingEmbedder struct {
	vector []float32
	texts  []string
}

func (e *recordingEmbedder) Embed(_ context.Context, text string) ([]float32, error) {
	e.texts = append(e.texts, text)
	return e.vector, nil
}

type savedRecommendation struct {
	userID     uint64
	questionID uint64
	videoID    uint64
	segmentID  uint64
	score      float64
	now        time.Time
}

type savedWatch struct {
	userID        uint64
	videoID       uint64
	questionID    uint64
	segmentID     uint64
	isWatched     bool
	watchDuration int
	now           time.Time
}

type markedExposure struct {
	userID     uint64
	questionID uint64
	segmentID  uint64
	now        time.Time
}

type fakeRecentSegmentStore struct {
	recent     map[uint64]bool
	marked     []uint64
	lastUserID uint64
	lastTTL    time.Duration
}

func (s *fakeRecentSegmentStore) FilterRecent(ctx context.Context, userID uint64, segmentIDs []uint64) (map[uint64]bool, error) {
	s.lastUserID = userID
	out := make(map[uint64]bool)
	for _, segmentID := range segmentIDs {
		if s.recent[segmentID] {
			out[segmentID] = true
		}
	}
	return out, nil
}

func (s *fakeRecentSegmentStore) ListRecent(ctx context.Context, userID uint64) ([]uint64, error) {
	s.lastUserID = userID
	out := make([]uint64, 0, len(s.recent))
	for segmentID, recent := range s.recent {
		if recent {
			out = append(out, segmentID)
		}
	}
	return out, nil
}

func (s *fakeRecentSegmentStore) MarkReturned(ctx context.Context, userID uint64, segmentID uint64, ttl time.Duration) error {
	s.lastUserID = userID
	s.lastTTL = ttl
	s.marked = append(s.marked, segmentID)
	return nil
}

func invalidArgumentError(message string) error {
	return errors.New(message)
}

type fakeProfileUpdater struct {
	calls           int
	lastUserID      uint64
	towerCalls      int
	lastTowerUserID uint64
	err             error
	towerErr        error
}

type fakeGorseClient struct {
	userID   uint64
	n        int
	ids      []uint64
	err      error
	feedback []GorseFeedback
}

func (c *fakeGorseClient) Recommend(_ context.Context, userID uint64, n int) ([]uint64, error) {
	c.userID = userID
	c.n = n
	if c.err != nil {
		return nil, c.err
	}
	return c.ids, nil
}

func (c *fakeGorseClient) PutFeedback(_ context.Context, feedback []GorseFeedback) error {
	c.feedback = append(c.feedback, feedback...)
	return nil
}

func (c *fakeGorseClient) UpsertUsers(context.Context, []GorseUser) error {
	return nil
}

func (c *fakeGorseClient) UpsertItems(context.Context, []GorseItem) error {
	return nil
}

func (c *fakeGorseClient) PatchItem(context.Context, GorseItem) error {
	return nil
}

func (u *fakeProfileUpdater) RebuildUserVideoProfile(_ context.Context, userID uint64, _ string, _ time.Time) error {
	u.calls++
	u.lastUserID = userID
	return u.err
}

func (u *fakeProfileUpdater) RebuildUserTowerEmbedding(_ context.Context, userID uint64, _ string, _ time.Time) error {
	u.towerCalls++
	u.lastTowerUserID = userID
	return u.towerErr
}
