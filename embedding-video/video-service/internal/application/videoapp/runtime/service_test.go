package runtime

import "testing"

func TestMemoryActiveCounterStoreTracksCounters(t *testing.T) {
	store := NewMemoryActiveCounterStore()

	store.Inc("transcode_tasks_active")
	store.Inc("transcode_tasks_active")
	store.Inc("vector_coarse_clip_active")
	store.Dec("transcode_tasks_active")
	store.Dec("vector_tasks_active")

	snapshot := store.Snapshot()
	if snapshot["transcode_tasks_active"] != 1 {
		t.Fatalf("transcode_tasks_active = %d, want 1", snapshot["transcode_tasks_active"])
	}
	if snapshot["vector_coarse_clip_active"] != 1 {
		t.Fatalf("vector_coarse_clip_active = %d, want 1", snapshot["vector_coarse_clip_active"])
	}
	if snapshot["vector_tasks_active"] != 0 {
		t.Fatalf("vector_tasks_active = %d, want 0", snapshot["vector_tasks_active"])
	}
}

func TestMemoryActiveCounterStoreSnapshotIsCopy(t *testing.T) {
	store := NewMemoryActiveCounterStore()
	store.Inc("transcode_tasks_active")

	snapshot := store.Snapshot()
	snapshot["transcode_tasks_active"] = 99

	next := store.Snapshot()
	if next["transcode_tasks_active"] != 1 {
		t.Fatalf("store was mutated through snapshot: %d", next["transcode_tasks_active"])
	}
}

func TestMemoryActiveCounterStoreResetClearsKnownCounters(t *testing.T) {
	store := NewMemoryActiveCounterStore()
	for _, name := range counterNames {
		store.Inc(name)
	}

	store.Reset()
	snapshot := store.Snapshot()

	for _, name := range counterNames {
		if snapshot[name] != 0 {
			t.Fatalf("%s = %d, want 0", name, snapshot[name])
		}
	}
}
