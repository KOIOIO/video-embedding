package handler_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"nlp-video-analysis/internal/application/videoapp"
	"nlp-video-analysis/internal/http/handler"
)

type stubQuestionApp struct {
	listQuestionsFunc func(context.Context, videoapp.ListQuestionsInput) (videoapp.QuestionPage, error)
	getQuestionFunc   func(context.Context, uint64) (videoapp.QuestionItem, bool, error)

	listInput  videoapp.ListQuestionsInput
	questionID uint64
}

func (s *stubQuestionApp) ListQuestions(ctx context.Context, input videoapp.ListQuestionsInput) (videoapp.QuestionPage, error) {
	s.listInput = input
	if s.listQuestionsFunc != nil {
		return s.listQuestionsFunc(ctx, input)
	}
	return videoapp.QuestionPage{}, nil
}

func (s *stubQuestionApp) GetQuestion(ctx context.Context, id uint64) (videoapp.QuestionItem, bool, error) {
	s.questionID = id
	if s.getQuestionFunc != nil {
		return s.getQuestionFunc(ctx, id)
	}
	return videoapp.QuestionItem{}, false, nil
}

func TestListQuestions_Success(t *testing.T) {
	stub := &stubQuestionApp{
		listQuestionsFunc: func(_ context.Context, input videoapp.ListQuestionsInput) (videoapp.QuestionPage, error) {
			if input.Page != 2 || input.PageSize != 5 {
				t.Fatalf("unexpected list input: %+v", input)
			}
			return videoapp.QuestionPage{
				Total:    12,
				Page:     2,
				PageSize: 5,
				Items: []videoapp.QuestionItem{{
					ID:               33,
					Source:           "paper",
					SourceQuestionID: "q-33",
					Content:          "What is acceleration?",
					Answer:           "Rate of change of velocity",
					Analysis:         "Physics basics",
					Knowledge:        "mechanics",
					Subject:          "physics",
					Type:             "single",
					Status:           1,
					CreateTime:       time.Unix(1714300200, 0),
					UpdateTime:       time.Unix(1714300300, 0),
				}},
			}, nil
		},
	}
	h := handler.NewQuestionHandler(stub)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/questions?page=2&page_size=5", nil)
	router := gin.New()
	router.GET("/api/questions", h.ListQuestions)

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	assertBodyContains(t, w.Body.Bytes(), `"success":true`)
	assertBodyContains(t, w.Body.Bytes(), `"total":12`)
	assertBodyContains(t, w.Body.Bytes(), `"page":2`)
	assertBodyContains(t, w.Body.Bytes(), `"page_size":5`)
	assertBodyContains(t, w.Body.Bytes(), `"id":33`)
	assertBodyContains(t, w.Body.Bytes(), `"content":"What is acceleration?"`)
}

func TestListQuestions_RejectsMalformedPage(t *testing.T) {
	h := handler.NewQuestionHandler(&stubQuestionApp{})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/questions?page=abc&page_size=5", nil)
	router := gin.New()
	router.GET("/api/questions", h.ListQuestions)

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
	assertBodyContains(t, w.Body.Bytes(), `"code":"invalid_argument"`)
	assertBodyContains(t, w.Body.Bytes(), `"message":"page must be a positive integer"`)
}

func TestListQuestions_RejectsNonPositivePageSize(t *testing.T) {
	h := handler.NewQuestionHandler(&stubQuestionApp{})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/questions?page=1&page_size=0", nil)
	router := gin.New()
	router.GET("/api/questions", h.ListQuestions)

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
	assertBodyContains(t, w.Body.Bytes(), `"code":"invalid_argument"`)
	assertBodyContains(t, w.Body.Bytes(), `"message":"page_size must be a positive integer"`)
}

func TestGetQuestion_InvalidID(t *testing.T) {
	h := handler.NewQuestionHandler(&stubQuestionApp{})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/questions/0", nil)
	router := gin.New()
	router.GET("/api/questions/:id", h.GetQuestion)

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
	assertBodyContains(t, w.Body.Bytes(), `"code":"invalid_argument"`)
	assertBodyContains(t, w.Body.Bytes(), `"message":"id must be a positive integer"`)
}

func TestGetQuestion_Success(t *testing.T) {
	stub := &stubQuestionApp{
		getQuestionFunc: func(_ context.Context, id uint64) (videoapp.QuestionItem, bool, error) {
			if id != 44 {
				t.Fatalf("unexpected question id %d", id)
			}
			return videoapp.QuestionItem{
				ID:               44,
				Source:           "bank",
				SourceQuestionID: "bank-44",
				Content:          "Solve x^2=4",
				Answer:           "x=2 or x=-2",
				Analysis:         "Use square roots",
				Knowledge:        "algebra",
				Subject:          "math",
				Type:             "single",
				Status:           1,
				CreateTime:       time.Unix(1714300400, 0),
				UpdateTime:       time.Unix(1714300500, 0),
			}, true, nil
		},
	}
	h := handler.NewQuestionHandler(stub)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/questions/44", nil)
	router := gin.New()
	router.GET("/api/questions/:id", h.GetQuestion)

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	assertBodyContains(t, w.Body.Bytes(), `"id":44`)
	assertBodyContains(t, w.Body.Bytes(), `"source_question_id":"bank-44"`)
	assertBodyContains(t, w.Body.Bytes(), `"content":"Solve x^2=4"`)
}
