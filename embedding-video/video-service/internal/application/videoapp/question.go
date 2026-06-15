package videoapp

import (
	"context"
	"time"

	questionapp "nlp-video-analysis/internal/application/videoapp/question"
)

type QuestionItem struct {
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

type QuestionPage struct {
	Total    int64
	Page     int
	PageSize int
	Items    []QuestionItem
}

type ListQuestionsInput struct {
	Page     int
	PageSize int
}

func (s *Service) ListQuestions(ctx context.Context, input ListQuestionsInput) (QuestionPage, error) {
	svc := questionapp.Service{
		Repo:            questionRepositoryAdapter{repo: s.Repo},
		InvalidArgument: InvalidArgumentError,
	}
	page, err := svc.ListQuestions(ctx, questionapp.ListInput{
		Page:     input.Page,
		PageSize: input.PageSize,
	})
	if err != nil {
		return QuestionPage{}, err
	}
	return mapQuestionPage(page), nil
}

func (s *Service) GetQuestion(ctx context.Context, id uint64) (QuestionItem, bool, error) {
	svc := questionapp.Service{
		Repo:            questionRepositoryAdapter{repo: s.Repo},
		InvalidArgument: InvalidArgumentError,
	}
	item, ok, err := svc.GetQuestion(ctx, id)
	if err != nil || !ok {
		return QuestionItem{}, ok, err
	}
	return mapQuestionItem(item), true, nil
}

type questionRepositoryAdapter struct {
	repo VideoRepository
}

func (a questionRepositoryAdapter) ListQuestions(ctx context.Context, page int, pageSize int) (questionapp.Page, error) {
	result, err := a.repo.ListQuestions(ctx, page, pageSize)
	if err != nil {
		return questionapp.Page{}, err
	}
	return mapQuestionPageToApp(result), nil
}

func (a questionRepositoryAdapter) GetQuestionByID(ctx context.Context, id uint64) (questionapp.Item, bool, error) {
	item, ok, err := a.repo.GetQuestionByID(ctx, id)
	if err != nil || !ok {
		return questionapp.Item{}, ok, err
	}
	return mapQuestionItemToApp(item), true, nil
}

func mapQuestionPage(page questionapp.Page) QuestionPage {
	items := make([]QuestionItem, 0, len(page.Items))
	for _, item := range page.Items {
		items = append(items, mapQuestionItem(item))
	}
	return QuestionPage{
		Total:    page.Total,
		Page:     page.Page,
		PageSize: page.PageSize,
		Items:    items,
	}
}

func mapQuestionPageToApp(page QuestionPage) questionapp.Page {
	items := make([]questionapp.Item, 0, len(page.Items))
	for _, item := range page.Items {
		items = append(items, mapQuestionItemToApp(item))
	}
	return questionapp.Page{
		Total:    page.Total,
		Page:     page.Page,
		PageSize: page.PageSize,
		Items:    items,
	}
}

func mapQuestionItem(item questionapp.Item) QuestionItem {
	return QuestionItem{
		ID:               item.ID,
		Source:           item.Source,
		SourceQuestionID: item.SourceQuestionID,
		Content:          item.Content,
		Answer:           item.Answer,
		Analysis:         item.Analysis,
		Knowledge:        item.Knowledge,
		Subject:          item.Subject,
		Type:             item.Type,
		Status:           item.Status,
		CreateTime:       item.CreateTime,
		UpdateTime:       item.UpdateTime,
	}
}

func mapQuestionItemToApp(item QuestionItem) questionapp.Item {
	return questionapp.Item{
		ID:               item.ID,
		Source:           item.Source,
		SourceQuestionID: item.SourceQuestionID,
		Content:          item.Content,
		Answer:           item.Answer,
		Analysis:         item.Analysis,
		Knowledge:        item.Knowledge,
		Subject:          item.Subject,
		Type:             item.Type,
		Status:           item.Status,
		CreateTime:       item.CreateTime,
		UpdateTime:       item.UpdateTime,
	}
}
