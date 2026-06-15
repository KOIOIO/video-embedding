package videoapp

import (
	"context"
	"testing"
)

func TestListQuestionsNormalizesPagination(t *testing.T) {
	repo := &questionTestRepo{}
	service := &Service{Repo: repo}

	_, err := service.ListQuestions(context.Background(), ListQuestionsInput{Page: 0, PageSize: 500})
	if err != nil {
		t.Fatalf("ListQuestions returned error: %v", err)
	}

	if repo.page != 1 {
		t.Fatalf("page = %d, want 1", repo.page)
	}
	if repo.pageSize != 20 {
		t.Fatalf("pageSize = %d, want 20", repo.pageSize)
	}
}

func TestGetQuestionRejectsZeroID(t *testing.T) {
	service := &Service{Repo: &questionTestRepo{}}

	_, ok, err := service.GetQuestion(context.Background(), 0)
	if err == nil {
		t.Fatal("expected error for zero question id")
	}
	if ok {
		t.Fatal("expected ok=false for invalid question id")
	}
}

func TestGetQuestionDelegatesToRepository(t *testing.T) {
	repo := &questionTestRepo{item: QuestionItem{ID: 9, Content: "question"}, found: true}
	service := &Service{Repo: repo}

	item, ok, err := service.GetQuestion(context.Background(), 9)
	if err != nil {
		t.Fatalf("GetQuestion returned error: %v", err)
	}
	if !ok {
		t.Fatal("expected question to be found")
	}
	if repo.getID != 9 {
		t.Fatalf("repo getID = %d, want 9", repo.getID)
	}
	if item.ID != 9 || item.Content != "question" {
		t.Fatalf("item = %#v", item)
	}
}

type questionTestRepo struct {
	stubVideoRepository
	page     int
	pageSize int
	getID    uint64
	item     QuestionItem
	found    bool
}

func (r *questionTestRepo) ListQuestions(_ context.Context, page int, pageSize int) (QuestionPage, error) {
	r.page = page
	r.pageSize = pageSize
	return QuestionPage{Page: page, PageSize: pageSize}, nil
}

func (r *questionTestRepo) GetQuestionByID(_ context.Context, id uint64) (QuestionItem, bool, error) {
	r.getID = id
	return r.item, r.found, nil
}
