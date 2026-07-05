package main

import (
	"math/rand"
	"testing"
	"time"
)

func TestBuildReactionPlansGeneratesUniqueUserSegmentPairs(t *testing.T) {
	users := []uint64{11, 12, 13}
	segments := []videoSegment{
		{ID: 101, VideoID: 1001},
		{ID: 102, VideoID: 1001},
		{ID: 103, VideoID: 1002},
	}
	now := time.Date(2026, 6, 22, 12, 0, 0, 0, time.UTC)

	plans, err := buildReactionPlans(users, segments, 7, rand.New(rand.NewSource(1)), now)
	if err != nil {
		t.Fatalf("buildReactionPlans returned error: %v", err)
	}
	if len(plans) != 7 {
		t.Fatalf("plan count = %d, want 7", len(plans))
	}

	seenPairs := map[[2]uint64]bool{}
	seenUsers := map[uint64]bool{}
	seenSegments := map[uint64]uint64{}
	seenReactions := map[string]bool{}
	for _, userID := range users {
		seenUsers[userID] = true
	}
	for _, segment := range segments {
		seenSegments[segment.ID] = segment.VideoID
	}

	for _, plan := range plans {
		if !seenUsers[plan.UserID] {
			t.Fatalf("unexpected user id %d", plan.UserID)
		}
		if videoID, ok := seenSegments[plan.VideoSegmentID]; !ok || videoID != plan.VideoID {
			t.Fatalf("unexpected segment/video pair: segment=%d video=%d", plan.VideoSegmentID, plan.VideoID)
		}
		pair := [2]uint64{plan.UserID, plan.VideoSegmentID}
		if seenPairs[pair] {
			t.Fatalf("duplicate user/segment pair: user=%d segment=%d", plan.UserID, plan.VideoSegmentID)
		}
		seenPairs[pair] = true
		seenReactions[plan.ReactionType] = true
		if plan.CreateTime.After(now) || plan.UpdateTime.Before(plan.CreateTime) {
			t.Fatalf("unexpected timestamps: create=%s update=%s now=%s", plan.CreateTime, plan.UpdateTime, now)
		}
	}
	if len(seenReactions) < 2 {
		t.Fatalf("expected varied reaction types, got %v", seenReactions)
	}
}

func TestBuildReactionPlansRejectsInsufficientCapacity(t *testing.T) {
	users := []uint64{11}
	segments := []videoSegment{{ID: 101, VideoID: 1001}}

	_, err := buildReactionPlans(users, segments, 2, rand.New(rand.NewSource(1)), time.Now())
	if err == nil {
		t.Fatal("expected error for insufficient unique user/segment pairs")
	}
}

func TestBuildReactionPlansExcludingSkipsExistingPairs(t *testing.T) {
	users := []uint64{11, 12}
	segments := []videoSegment{
		{ID: 101, VideoID: 1001},
		{ID: 102, VideoID: 1002},
	}
	existing := map[pairKey]bool{
		{UserID: 11, SegmentID: 101}: true,
		{UserID: 12, SegmentID: 102}: true,
	}

	plans, err := buildReactionPlansExcluding(users, segments, 2, existing, rand.New(rand.NewSource(3)), time.Now(), 24*time.Hour)
	if err != nil {
		t.Fatalf("buildReactionPlansExcluding returned error: %v", err)
	}
	if len(plans) != 2 {
		t.Fatalf("plan count = %d, want 2", len(plans))
	}
	for _, plan := range plans {
		if existing[pairKey{UserID: plan.UserID, SegmentID: plan.VideoSegmentID}] {
			t.Fatalf("generated existing pair user=%d segment=%d", plan.UserID, plan.VideoSegmentID)
		}
	}
}

func TestUniqueSegmentIDsReturnsSortedIDs(t *testing.T) {
	plans := []reactionPlan{
		{VideoSegmentID: 103},
		{VideoSegmentID: 101},
		{VideoSegmentID: 103},
		{VideoSegmentID: 102},
	}

	got := uniqueSegmentIDs(plans)
	want := []uint64{101, 102, 103}
	if len(got) != len(want) {
		t.Fatalf("len(uniqueSegmentIDs) = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("uniqueSegmentIDs[%d] = %d, want %d", i, got[i], want[i])
		}
	}
}
