package gorsesync

import (
	"context"
	"fmt"

	recommendationapp "nlp-video-analysis/internal/application/videoapp/recommendation"
)

type Source interface {
	LoadUsers(ctx context.Context) ([]recommendationapp.GorseUser, error)
	LoadItems(ctx context.Context) ([]recommendationapp.GorseItem, error)
	LoadFeedback(ctx context.Context) ([]recommendationapp.GorseFeedback, error)
}

type Options struct {
	DryRun            bool
	BatchSize         int
	EnableGate        bool
	MinFeedbackCount  int
	MinRecommendItems int
}

type Result struct {
	DryRun     bool
	Users      int
	Items      int
	Feedback   int
	GatePassed bool
	GateReason string
}

type Syncer struct {
	Source  Source
	Client  recommendationapp.GorseClient
	Options Options
}

func (s Syncer) Run(ctx context.Context) (Result, error) {
	if s.Source == nil {
		return Result{}, fmt.Errorf("gorse sync source is nil")
	}
	users, err := s.Source.LoadUsers(ctx)
	if err != nil {
		return Result{}, err
	}
	items, err := s.Source.LoadItems(ctx)
	if err != nil {
		return Result{}, err
	}
	feedback, err := s.Source.LoadFeedback(ctx)
	if err != nil {
		return Result{}, err
	}
	result := Result{
		DryRun:     s.Options.DryRun,
		Users:      len(users),
		Items:      len(items),
		Feedback:   len(feedback),
		GatePassed: true,
	}
	if s.Options.EnableGate {
		if s.Options.MinFeedbackCount > 0 && len(feedback) < s.Options.MinFeedbackCount {
			result.GatePassed = false
			result.GateReason = fmt.Sprintf("feedback_count %d < %d", len(feedback), s.Options.MinFeedbackCount)
		}
		if result.GatePassed && s.Options.MinRecommendItems > 0 && len(items) < s.Options.MinRecommendItems {
			result.GatePassed = false
			result.GateReason = fmt.Sprintf("recommend_item_count %d < %d", len(items), s.Options.MinRecommendItems)
		}
	}
	if s.Options.DryRun || !result.GatePassed {
		return result, nil
	}
	if s.Client == nil {
		return Result{}, fmt.Errorf("gorse client is nil")
	}
	batchSize := s.Options.BatchSize
	if batchSize <= 0 {
		batchSize = 500
	}
	for start := 0; start < len(users); start += batchSize {
		end := min(start+batchSize, len(users))
		if err := s.Client.UpsertUsers(ctx, users[start:end]); err != nil {
			return Result{}, err
		}
	}
	for start := 0; start < len(items); start += batchSize {
		end := min(start+batchSize, len(items))
		if err := s.Client.UpsertItems(ctx, items[start:end]); err != nil {
			return Result{}, err
		}
	}
	for _, item := range items {
		if err := s.Client.PatchItem(ctx, item); err != nil {
			return Result{}, err
		}
	}
	for start := 0; start < len(feedback); start += batchSize {
		end := min(start+batchSize, len(feedback))
		if err := s.Client.PutFeedback(ctx, feedback[start:end]); err != nil {
			return Result{}, err
		}
	}
	return result, nil
}
