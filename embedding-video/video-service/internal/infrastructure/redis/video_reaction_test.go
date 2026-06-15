package redis

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	goredis "github.com/go-redis/redis/v8"

	"nlp-video-analysis/internal/application/videoapp"
)

func newTestVideoReactionBuffer(t *testing.T) (*VideoReactionBuffer, *goredis.Client, func()) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	rdb := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	buffer := NewVideoReactionBuffer(rdb, "video:reaction:test")
	buffer.group = "test_group"
	buffer.consumer = "test_consumer"
	buffer.initializedAt = time.Hour
	buffer.pendingMinIdle = 0
	return buffer, rdb, func() {
		_ = rdb.Close()
		mr.Close()
	}
}

func TestVideoReactionBufferSubmitSwitchesAndCancelsWithImmediateCounts(t *testing.T) {
	ctx := context.Background()
	buffer, _, cleanup := newTestVideoReactionBuffer(t)
	defer cleanup()

	result, err := buffer.Submit(ctx, 21, 7, videoapp.VideoReactionLike, videoapp.VideoReactionCounts{}, "", false)
	if err != nil {
		t.Fatalf("submit like: %v", err)
	}
	if !result.Active || result.Counts.LikeCount != 1 || result.Counts.DoubleLikeCount != 0 {
		t.Fatalf("unexpected like result: %+v", result)
	}

	result, err = buffer.Submit(ctx, 21, 7, videoapp.VideoReactionDoubleLike, videoapp.VideoReactionCounts{}, "", false)
	if err != nil {
		t.Fatalf("switch double like: %v", err)
	}
	if !result.Active || result.Counts.LikeCount != 0 || result.Counts.DoubleLikeCount != 1 {
		t.Fatalf("unexpected switch result: %+v", result)
	}

	result, err = buffer.Submit(ctx, 21, 7, videoapp.VideoReactionDoubleLike, videoapp.VideoReactionCounts{}, "", false)
	if err != nil {
		t.Fatalf("cancel double like: %v", err)
	}
	if result.Active || result.Counts.LikeCount != 0 || result.Counts.DoubleLikeCount != 0 {
		t.Fatalf("unexpected cancel result: %+v", result)
	}
}

func TestVideoReactionBufferDequeueIncludesFinalActiveState(t *testing.T) {
	ctx := context.Background()
	buffer, _, cleanup := newTestVideoReactionBuffer(t)
	defer cleanup()

	if _, err := buffer.Submit(ctx, 21, 7, videoapp.VideoReactionLike, videoapp.VideoReactionCounts{}, "", false); err != nil {
		t.Fatalf("submit like: %v", err)
	}
	msg, err := buffer.Dequeue(ctx)
	if err != nil {
		t.Fatalf("dequeue: %v", err)
	}
	if msg.MessageID == "" || msg.Event.VideoID != 21 || msg.Event.UserID != 7 || msg.Event.ReactionType != videoapp.VideoReactionLike || !msg.Event.Active {
		t.Fatalf("unexpected message: %+v", msg)
	}
}

func TestVideoReactionBufferDequeueReclaimsPendingMessage(t *testing.T) {
	ctx := context.Background()
	buffer, _, cleanup := newTestVideoReactionBuffer(t)
	defer cleanup()

	if _, err := buffer.Submit(ctx, 21, 7, videoapp.VideoReactionLike, videoapp.VideoReactionCounts{}, "", false); err != nil {
		t.Fatalf("submit like: %v", err)
	}
	first, err := buffer.Dequeue(ctx)
	if err != nil {
		t.Fatalf("first dequeue: %v", err)
	}

	buffer.consumer = "replacement_consumer"
	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	reclaimed, err := buffer.Dequeue(ctx)
	if err != nil {
		t.Fatalf("reclaim dequeue: %v", err)
	}
	if reclaimed.MessageID != first.MessageID {
		t.Fatalf("reclaimed message id = %q, want %q", reclaimed.MessageID, first.MessageID)
	}
	if reclaimed.Event.VideoID != 21 || reclaimed.Event.UserID != 7 || reclaimed.Event.ReactionType != videoapp.VideoReactionLike || !reclaimed.Event.Active {
		t.Fatalf("unexpected reclaimed event: %+v", reclaimed.Event)
	}
}

func TestVideoReactionBufferSeedsCountsAndUserState(t *testing.T) {
	ctx := context.Background()
	buffer, _, cleanup := newTestVideoReactionBuffer(t)
	defer cleanup()

	result, err := buffer.Submit(ctx, 21, 7, videoapp.VideoReactionLike, videoapp.VideoReactionCounts{
		LikeCount:       5,
		DoubleLikeCount: 2,
	}, videoapp.VideoReactionLike, true)
	if err != nil {
		t.Fatalf("submit seeded cancel: %v", err)
	}
	if result.Active || result.Counts.LikeCount != 4 || result.Counts.DoubleLikeCount != 2 {
		t.Fatalf("unexpected seeded cancel result: %+v", result)
	}
}

func TestVideoReactionBufferGetCountsDoesNotReturnDislikeCount(t *testing.T) {
	ctx := context.Background()
	buffer, rdb, cleanup := newTestVideoReactionBuffer(t)
	defer cleanup()

	counts, err := buffer.GetCounts(ctx, 21, videoapp.VideoReactionCounts{LikeCount: 3, DoubleLikeCount: 1})
	if err != nil {
		t.Fatalf("get counts: %v", err)
	}
	if counts.LikeCount != 3 || counts.DoubleLikeCount != 1 {
		t.Fatalf("unexpected counts: %+v", counts)
	}
	if got := rdb.HGet(ctx, buffer.countsKey(21), "dislike").Val(); got != "0" {
		t.Fatalf("stored dislike seed = %q, want 0", got)
	}
}
