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
