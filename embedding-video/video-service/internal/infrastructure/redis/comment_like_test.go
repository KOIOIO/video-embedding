package redis

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	goredis "github.com/go-redis/redis/v8"

	"video-service/internal/application/videoapp"
)

func newTestCommentLikeBuffer(t *testing.T) (*CommentLikeBuffer, *goredis.Client, func()) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	rdb := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	buffer := NewCommentLikeBuffer(rdb, "comment:like:test")
	buffer.group = "test_group"
	buffer.consumer = "test_consumer"
	buffer.pendingMinIdle = 0
	return buffer, rdb, func() {
		_ = rdb.Close()
		mr.Close()
	}
}

func TestCommentLikeBufferSubmitSwitchesAndCancels(t *testing.T) {
	ctx := context.Background()
	buffer, _, cleanup := newTestCommentLikeBuffer(t)
	defer cleanup()

	result, err := buffer.Submit(ctx, 101, 7, videoapp.VideoReactionLike, videoapp.VideoReactionCounts{}, "", false)
	if err != nil {
		t.Fatalf("submit like: %v", err)
	}
	if !result.Active || result.Counts.LikeCount != 1 || result.Counts.DoubleLikeCount != 0 {
		t.Fatalf("unexpected like result: %+v", result)
	}

	result, err = buffer.Submit(ctx, 101, 7, videoapp.VideoReactionDoubleLike, videoapp.VideoReactionCounts{}, "", false)
	if err != nil {
		t.Fatalf("switch double like: %v", err)
	}
	if !result.Active || result.Counts.LikeCount != 0 || result.Counts.DoubleLikeCount != 1 {
		t.Fatalf("unexpected switch result: %+v", result)
	}

	result, err = buffer.Submit(ctx, 101, 7, videoapp.VideoReactionDoubleLike, videoapp.VideoReactionCounts{}, "", false)
	if err != nil {
		t.Fatalf("cancel double like: %v", err)
	}
	if result.Active || result.Counts.LikeCount != 0 || result.Counts.DoubleLikeCount != 0 {
		t.Fatalf("unexpected cancel result: %+v", result)
	}

	result, err = buffer.Submit(ctx, 101, 7, videoapp.VideoReactionDislike, videoapp.VideoReactionCounts{}, "", false)
	if err != nil {
		t.Fatalf("submit dislike: %v", err)
	}
	if !result.Active || result.ReactionType != videoapp.VideoReactionDislike || result.Counts.LikeCount != 0 || result.Counts.DoubleLikeCount != 0 {
		t.Fatalf("unexpected dislike result: %+v", result)
	}
}

func TestCommentLikeBufferSeedsCountsFromDatabaseValue(t *testing.T) {
	ctx := context.Background()
	buffer, _, cleanup := newTestCommentLikeBuffer(t)
	defer cleanup()

	counts, err := buffer.GetCounts(ctx, 202, videoapp.VideoReactionCounts{LikeCount: 9, DoubleLikeCount: 2})
	if err != nil {
		t.Fatalf("get counts: %v", err)
	}
	if counts.LikeCount != 9 || counts.DoubleLikeCount != 2 {
		t.Fatalf("seeded counts = %+v, want 9/2", counts)
	}

	result, err := buffer.Submit(ctx, 202, 7, videoapp.VideoReactionLike, videoapp.VideoReactionCounts{LikeCount: 9, DoubleLikeCount: 2}, "", false)
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if !result.Active || result.Counts.LikeCount != 10 || result.Counts.DoubleLikeCount != 2 {
		t.Fatalf("unexpected result after seed: %+v", result)
	}
}

func TestCommentLikeBufferUserReactionReadAndSeed(t *testing.T) {
	ctx := context.Background()
	buffer, _, cleanup := newTestCommentLikeBuffer(t)
	defer cleanup()

	reactionType, _, found, err := buffer.GetUserReaction(ctx, 303, 7)
	if err != nil {
		t.Fatalf("get user reaction: %v", err)
	}
	if found || reactionType != "" {
		t.Fatalf("unexpected initial user reaction state: type=%q found=%v", reactionType, found)
	}

	if err := buffer.SeedUserReaction(ctx, 303, 7, videoapp.VideoReactionDislike, true); err != nil {
		t.Fatalf("seed user reaction: %v", err)
	}
	reactionType, active, found, err := buffer.GetUserReaction(ctx, 303, 7)
	if err != nil {
		t.Fatalf("get user reaction after seed: %v", err)
	}
	if !found || !active || reactionType != videoapp.VideoReactionDislike {
		t.Fatalf("expected seeded dislike state, got type=%q active=%v found=%v", reactionType, active, found)
	}
}

func TestCommentLikeBufferDequeueRequeuesAndDeadLetters(t *testing.T) {
	ctx := context.Background()
	buffer, _, cleanup := newTestCommentLikeBuffer(t)
	defer cleanup()

	if _, err := buffer.Submit(ctx, 404, 7, videoapp.VideoReactionLike, videoapp.VideoReactionCounts{}, "", false); err != nil {
		t.Fatalf("submit: %v", err)
	}
	msg, err := buffer.Dequeue(ctx)
	if err != nil {
		t.Fatalf("dequeue: %v", err)
	}
	if msg.Event.CommentID != 404 || msg.Event.UserID != 7 || !msg.Event.Active || msg.Event.ReactionType != videoapp.VideoReactionLike {
		t.Fatalf("unexpected event: %+v", msg.Event)
	}

	if err := buffer.Requeue(ctx, msg, time.Second, "test"); err != nil {
		t.Fatalf("requeue: %v", err)
	}
	msg, err = buffer.Dequeue(ctx)
	if err != nil {
		t.Fatalf("dequeue after requeue: %v", err)
	}
	if err := buffer.MoveToDeadLetter(ctx, msg, "comment_not_found"); err != nil {
		t.Fatalf("dead letter: %v", err)
	}

	msgs, err := buffer.rdb.XRange(ctx, buffer.streamKey+":dlq", "-", "+").Result()
	if err != nil {
		t.Fatalf("read dlq: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("dlq message count = %d, want 1", len(msgs))
	}
	payload, _ := msgs[0].Values["payload"].(string)
	if payload == "" || !strings.Contains(payload, `"comment_id":404`) {
		t.Fatalf("unexpected dlq payload: %q", payload)
	}
	reason, _ := msgs[0].Values["reason"].(string)
	if reason != "comment_not_found" {
		t.Fatalf("dlq reason = %q, want comment_not_found", reason)
	}
}
