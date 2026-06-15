package redis

import (
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	goredis "github.com/go-redis/redis/v8"
)

func newTestRuntimeCounterStore(t *testing.T) (*RuntimeCounterStore, *miniredis.Miniredis, func()) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	rdb := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	store := NewRuntimeCounterStore(rdb, "video:runtime:active:")
	cleanup := func() {
		_ = rdb.Close()
		mr.Close()
	}
	return store, mr, cleanup
}

func TestRuntimeCounterStoreSnapshotTracksIncDec(t *testing.T) {
	store, _, cleanup := newTestRuntimeCounterStore(t)
	defer cleanup()

	store.Inc("transcode_tasks_active")
	store.Inc("transcode_tasks_active")
	store.Inc("vector_coarse_asr_active")
	store.Dec("transcode_tasks_active")

	snapshot := store.Snapshot()
	if snapshot["transcode_tasks_active"] != 1 {
		t.Fatalf("transcode_tasks_active = %d, want 1", snapshot["transcode_tasks_active"])
	}
	if snapshot["vector_coarse_asr_active"] != 1 {
		t.Fatalf("vector_coarse_asr_active = %d, want 1", snapshot["vector_coarse_asr_active"])
	}
}

func TestRuntimeCounterStoreSnapshotDropsExpiredInstanceCounts(t *testing.T) {
	store, mr, cleanup := newTestRuntimeCounterStore(t)
	defer cleanup()

	store.Inc("vector_coarse_asr_active")
	if got := store.Snapshot()["vector_coarse_asr_active"]; got != 1 {
		t.Fatalf("snapshot before expiry = %d, want 1", got)
	}

	mr.FastForward(3 * time.Minute)

	if got := store.Snapshot()["vector_coarse_asr_active"]; got != 0 {
		t.Fatalf("snapshot after expiry = %d, want 0", got)
	}
}

func TestRuntimeCounterStoreSnapshotAggregatesMultipleInstances(t *testing.T) {
	storeA, _, cleanupA := newTestRuntimeCounterStore(t)
	defer cleanupA()

	rdb := storeA.rdb
	storeB := NewRuntimeCounterStore(rdb, "video:runtime:active:")

	storeA.Inc("vector_tasks_active")
	storeB.Inc("vector_tasks_active")
	storeB.Inc("vector_tasks_active")

	if got := storeA.Snapshot()["vector_tasks_active"]; got != 3 {
		t.Fatalf("aggregated vector_tasks_active = %d, want 3", got)
	}

	storeB.Dec("vector_tasks_active")
	if got := storeA.Snapshot()["vector_tasks_active"]; got != 2 {
		t.Fatalf("aggregated vector_tasks_active after dec = %d, want 2", got)
	}
}
