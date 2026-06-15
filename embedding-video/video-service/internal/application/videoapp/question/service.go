package question

import (
	"context"
	"time"
)

type Item struct {
	ID               uint64
	Source           string
	SourceQuestionID string
	Content          string
	Answer           string
	Analysis         string
	Knowledge        string
	Subject          string
	Type             string
	Status           int16
	CreateTime       time.Time
	UpdateTime       time.Time
}

type Page struct {
	Total    int64
	Page     int
	PageSize int
	Items    []Item
}

type ListInput struct {
	Page     int
	PageSize int
}

type Repository interface {
	ListQuestions(ctx context.Context, page int, pageSize int) (Page, error)
	GetQuestionByID(ctx context.Context, id uint64) (Item, bool, error)
}

type Service struct {
	Repo            Repository
	InvalidArgument func(message string) error
}

func (s Service) ListQuestions(ctx context.Context, input ListInput) (Page, error) {
	page := input.Page
	if page < 1 {
		page = 1
	}
	pageSize := input.PageSize
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	return s.Repo.ListQuestions(ctx, page, pageSize)
}

func (s Service) GetQuestion(ctx context.Context, id uint64) (Item, bool, error) {
	if id == 0 {
		return Item{}, false, s.InvalidArgument("question_id is required")
	}
	return s.Repo.GetQuestionByID(ctx, id)
}
