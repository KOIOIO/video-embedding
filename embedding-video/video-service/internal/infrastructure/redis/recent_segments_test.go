package redis

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	goredis "github.com/go-redis/redis/v8"
)

func TestRecentSegmentStoreMarksFiltersListsAndExpiresByUser(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	defer mr.Close()
	rdb := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	defer rdb.Close()
	store := NewRecentSegmentStore(rdb, "test:recent:")
	ctx := context.Background()

	if err := store.MarkReturned(ctx, 6, 101, 30*time.Minute); err != nil {
		t.Fatalf("MarkReturned returned error: %v", err)
	}

	recent, err := store.FilterRecent(ctx, 6, []uint64{101, 102})
	if err != nil {
		t.Fatalf("FilterRecent returned error: %v", err)
	}
	if !reflect.DeepEqual(recent, map[uint64]bool{101: true}) {
		t.Fatalf("recent = %#v, want segment 101 only", recent)
	}
	otherUserRecent, err := store.FilterRecent(ctx, 7, []uint64{101})
	if err != nil {
		t.Fatalf("FilterRecent other user returned error: %v", err)
	}
	if len(otherUserRecent) != 0 {
		t.Fatalf("other user recent = %#v, want empty", otherUserRecent)
	}
	listed, err := store.ListRecent(ctx, 6)
	if err != nil {
		t.Fatalf("ListRecent returned error: %v", err)
	}
	if !reflect.DeepEqual(listed, []uint64{101}) {
		t.Fatalf("listed = %#v, want [101]", listed)
	}
	mr.FastForward(31 * time.Minute)
	recent, err = store.FilterRecent(ctx, 6, []uint64{101})
	if err != nil {
		t.Fatalf("FilterRecent after expiry returned error: %v", err)
	}
	if len(recent) != 0 {
		t.Fatalf("recent after expiry = %#v, want empty", recent)
	}
}

func TestRecentSegmentStoreTrimsToConfiguredMaxSize(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	defer mr.Close()
	rdb := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	defer rdb.Close()
	store := NewRecentSegmentStoreWithOptions(rdb, RecentSegmentStoreOptions{
		Prefix:  "test:recent:",
		MaxSize: 3,
	})
	ctx := context.Background()

	for _, segmentID := range []uint64{101, 102, 103, 104} {
		if err := store.MarkReturned(ctx, 6, segmentID, 30*time.Minute); err != nil {
			t.Fatalf("MarkReturned(%d) returned error: %v", segmentID, err)
		}
	}

	listed, err := store.ListRecent(ctx, 6)
	if err != nil {
		t.Fatalf("ListRecent returned error: %v", err)
	}
	if !reflect.DeepEqual(listed, []uint64{102, 103, 104}) {
		t.Fatalf("listed = %#v, want the newest 3 segment IDs", listed)
	}

	recent, err := store.FilterRecent(ctx, 6, []uint64{101, 102, 103, 104})
	if err != nil {
		t.Fatalf("FilterRecent returned error: %v", err)
	}
	if !reflect.DeepEqual(recent, map[uint64]bool{102: true, 103: true, 104: true}) {
		t.Fatalf("recent = %#v, want only the newest 3 segment IDs", recent)
	}

	cardinality, err := rdb.ZCard(ctx, store.key(6)).Result()
	if err != nil {
		t.Fatalf("ZCard returned error: %v", err)
	}
	if cardinality != 3 {
		t.Fatalf("ZCard = %d, want 3", cardinality)
	}
}

func TestRecentSegmentStoreReplacesLegacySetKey(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	defer mr.Close()
	rdb := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	defer rdb.Close()
	store := NewRecentSegmentStoreWithOptions(rdb, RecentSegmentStoreOptions{
		Prefix:  "test:recent:",
		MaxSize: 3,
	})
	ctx := context.Background()

	if err := rdb.SAdd(ctx, store.key(6), "101").Err(); err != nil {
		t.Fatalf("seed legacy set: %v", err)
	}
	if err := rdb.Expire(ctx, store.key(6), 30*time.Minute).Err(); err != nil {
		t.Fatalf("expire legacy set: %v", err)
	}

	if err := store.MarkReturned(ctx, 6, 102, 30*time.Minute); err != nil {
		t.Fatalf("MarkReturned returned error: %v", err)
	}

	keyType, err := rdb.Type(ctx, store.key(6)).Result()
	if err != nil {
		t.Fatalf("Type returned error: %v", err)
	}
	if keyType != "zset" {
		t.Fatalf("key type = %q, want zset", keyType)
	}
	listed, err := store.ListRecent(ctx, 6)
	if err != nil {
		t.Fatalf("ListRecent returned error: %v", err)
	}
	if !reflect.DeepEqual(listed, []uint64{102}) {
		t.Fatalf("listed = %#v, want only the newly marked segment", listed)
	}
}
