package gorsesync

import (
	"context"
	"testing"
	"time"

	recommendationapp "nlp-video-analysis/internal/application/videoapp/recommendation"
)

func TestSyncerDryRunCollectsCountsWithoutWriting(t *testing.T) {
	source := &fakeSource{
		users:    []recommendationapp.GorseUser{{UserID: "7"}},
		items:    []recommendationapp.GorseItem{{ItemID: "101"}},
		feedback: []recommendationapp.GorseFeedback{{FeedbackType: "like", UserID: "7", ItemID: "101", Timestamp: time.Now(), Value: 2}},
	}
	client := &fakeClient{}
	syncer := Syncer{
		Source: source,
		Client: client,
		Options: Options{
			DryRun:            true,
			BatchSize:         1,
			EnableGate:        true,
			MinFeedbackCount:  1,
			MinRecommendItems: 1,
		},
	}

	result, err := syncer.Run(context.Background())
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if !result.DryRun || result.Users != 1 || result.Items != 1 || result.Feedback != 1 || !result.GatePassed {
		t.Fatalf("result = %+v", result)
	}
	if client.userWrites != 0 || client.itemWrites != 0 || client.feedbackWrites != 0 {
		t.Fatalf("client writes users=%d items=%d feedback=%d, want 0", client.userWrites, client.itemWrites, client.feedbackWrites)
	}
}

func TestSyncerGateBlocksSparseLiveSync(t *testing.T) {
	source := &fakeSource{
		users:    []recommendationapp.GorseUser{{UserID: "7"}},
		items:    []recommendationapp.GorseItem{{ItemID: "101"}},
		feedback: []recommendationapp.GorseFeedback{{FeedbackType: "like", UserID: "7", ItemID: "101", Timestamp: time.Now(), Value: 2}},
	}
	client := &fakeClient{}
	syncer := Syncer{
		Source: source,
		Client: client,
		Options: Options{
			BatchSize:         10,
			EnableGate:        true,
			MinFeedbackCount:  2,
			MinRecommendItems: 2,
		},
	}

	result, err := syncer.Run(context.Background())
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if result.GatePassed {
		t.Fatalf("GatePassed = true, want false: %+v", result)
	}
	if result.GateReason == "" {
		t.Fatalf("GateReason empty: %+v", result)
	}
	if client.userWrites != 0 || client.itemWrites != 0 || client.feedbackWrites != 0 {
		t.Fatalf("client writes users=%d items=%d feedback=%d, want 0", client.userWrites, client.itemWrites, client.feedbackWrites)
	}
}

func TestSyncerWritesBatchesWhenGatePasses(t *testing.T) {
	source := &fakeSource{
		users: []recommendationapp.GorseUser{
			{UserID: "7"},
			{UserID: "8"},
			{UserID: "9"},
		},
		items: []recommendationapp.GorseItem{
			{ItemID: "101"},
			{ItemID: "102"},
		},
		feedback: []recommendationapp.GorseFeedback{
			{FeedbackType: "like", UserID: "7", ItemID: "101", Timestamp: time.Now(), Value: 2},
			{FeedbackType: "watch", UserID: "8", ItemID: "102", Timestamp: time.Now(), Value: 0.8},
			{FeedbackType: "exposure", UserID: "9", ItemID: "101", Timestamp: time.Now(), Value: 1},
		},
	}
	client := &fakeClient{}
	syncer := Syncer{
		Source: source,
		Client: client,
		Options: Options{
			BatchSize:         2,
			EnableGate:        true,
			MinFeedbackCount:  3,
			MinRecommendItems: 2,
		},
	}

	result, err := syncer.Run(context.Background())
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if !result.GatePassed || result.Users != 3 || result.Items != 2 || result.Feedback != 3 {
		t.Fatalf("result = %+v", result)
	}
	if client.userWrites != 2 || client.itemWrites != 1 || client.feedbackWrites != 2 {
		t.Fatalf("writes users=%d items=%d feedback=%d, want 2 1 2", client.userWrites, client.itemWrites, client.feedbackWrites)
	}
	if client.itemPatches != 2 {
		t.Fatalf("itemPatches = %d, want 2", client.itemPatches)
	}
}

type fakeSource struct {
	users    []recommendationapp.GorseUser
	items    []recommendationapp.GorseItem
	feedback []recommendationapp.GorseFeedback
}

func (s *fakeSource) LoadUsers(context.Context) ([]recommendationapp.GorseUser, error) {
	return s.users, nil
}

func (s *fakeSource) LoadItems(context.Context) ([]recommendationapp.GorseItem, error) {
	return s.items, nil
}

func (s *fakeSource) LoadFeedback(context.Context) ([]recommendationapp.GorseFeedback, error) {
	return s.feedback, nil
}

type fakeClient struct {
	userWrites     int
	itemWrites     int
	itemPatches    int
	feedbackWrites int
}

func (c *fakeClient) Recommend(context.Context, uint64, int) ([]uint64, error) {
	return nil, nil
}

func (c *fakeClient) PutFeedback(context.Context, []recommendationapp.GorseFeedback) error {
	c.feedbackWrites++
	return nil
}

func (c *fakeClient) UpsertUsers(context.Context, []recommendationapp.GorseUser) error {
	c.userWrites++
	return nil
}

func (c *fakeClient) UpsertItems(context.Context, []recommendationapp.GorseItem) error {
	c.itemWrites++
	return nil
}

func (c *fakeClient) PatchItem(context.Context, recommendationapp.GorseItem) error {
	c.itemPatches++
	return nil
}
