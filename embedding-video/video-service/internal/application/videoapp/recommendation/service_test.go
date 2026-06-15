package recommendation

import (
	"context"
	"errors"
	"reflect"
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
	repo := &fakeRepository{segmentVideoID: 17, watchCreated: true}
	svc := Service{
		Repo:                  repo,
		Now:                   func() time.Time { return time.Unix(1, 0) },
		InvalidArgument:       invalidArgumentError,
		ErrVideoSegmentAbsent: errors.New("segment missing"),
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
	embeddingDim       int
	questionText       string
	questionID         uint64
	candidates         []Candidate
	findLimit          int
	queryVec           pgvector.Vector
	saved              []savedRecommendation
	segmentVideoID     uint64
	alreadyCounted     bool
	watchCreated       bool
	savedWatch         savedWatch
	incrementedVideoID uint64
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

func invalidArgumentError(message string) error {
	return errors.New(message)
}
