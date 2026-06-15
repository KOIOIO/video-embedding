package question

import (
	"context"
	"errors"
	"testing"
)

func TestListQuestionsNormalizesPagination(t *testing.T) {
	repo := &fakeRepository{}
	svc := Service{Repo: repo}

	page, err := svc.ListQuestions(context.Background(), ListInput{Page: 0, PageSize: 500})
	if err != nil {
		t.Fatalf("ListQuestions returned error: %v", err)
	}

	if repo.page != 1 {
		t.Fatalf("repo page = %d, want 1", repo.page)
	}
	if repo.pageSize != 20 {
		t.Fatalf("repo pageSize = %d, want 20", repo.pageSize)
	}
	if page.Page != 1 || page.PageSize != 20 {
		t.Fatalf("page = %+v, want Page=1 PageSize=20", page)
	}
}

func TestListQuestionsKeepsValidPagination(t *testing.T) {
	repo := &fakeRepository{}
	svc := Service{Repo: repo}

	if _, err := svc.ListQuestions(context.Background(), ListInput{Page: 3, PageSize: 50}); err != nil {
		t.Fatalf("ListQuestions returned error: %v", err)
	}

	if repo.page != 3 {
		t.Fatalf("repo page = %d, want 3", repo.page)
	}
	if repo.pageSize != 50 {
		t.Fatalf("repo pageSize = %d, want 50", repo.pageSize)
	}
}

func TestGetQuestionRejectsZeroID(t *testing.T) {
	invalidErr := errors.New("invalid")
	svc := Service{
		Repo: &fakeRepository{},
		InvalidArgument: func(message string) error {
			if message != "question_id is required" {
				t.Fatalf("message = %q, want question_id is required", message)
			}
			return invalidErr
		},
	}

	_, ok, err := svc.GetQuestion(context.Background(), 0)
	if !errors.Is(err, invalidErr) {
		t.Fatalf("err = %v, want %v", err, invalidErr)
	}
	if ok {
		t.Fatal("ok = true, want false")
	}
}

func TestGetQuestionDelegatesToRepository(t *testing.T) {
	repo := &fakeRepository{item: Item{ID: 9, Content: "question"}, found: true}
	svc := Service{Repo: repo}

	item, ok, err := svc.GetQuestion(context.Background(), 9)
	if err != nil {
		t.Fatalf("GetQuestion returned error: %v", err)
	}
	if !ok {
		t.Fatal("ok = false, want true")
	}
	if repo.getID != 9 {
		t.Fatalf("repo getID = %d, want 9", repo.getID)
	}
	if item.ID != 9 || item.Content != "question" {
		t.Fatalf("item = %+v, want ID=9 Content=question", item)
	}
}

type fakeRepository struct {
	page     int
	pageSize int
	getID    uint64
	item     Item
	found    bool
}

func (r *fakeRepository) ListQuestions(_ context.Context, page int, pageSize int) (Page, error) {
	r.page = page
	r.pageSize = pageSize
	return Page{Page: page, PageSize: pageSize}, nil
}

func (r *fakeRepository) GetQuestionByID(_ context.Context, id uint64) (Item, bool, error) {
	r.getID = id
	return r.item, r.found, nil
}
