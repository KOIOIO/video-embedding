package videoapp

import "testing"

func TestRuntimeCountersIncDecAndSnapshot(t *testing.T) {
	runtimeCounters.Reset()
	runtimeCounters.Inc("transcode_tasks_active")
	runtimeCounters.Inc("transcode_tasks_active")
	runtimeCounters.Inc("vector_coarse_clip_active")
	runtimeCounters.Dec("transcode_tasks_active")

	s := runtimeCounters.Snapshot()
	if s["transcode_tasks_active"] != 1 {
		t.Fatalf("transcode_tasks_active = %d, want 1", s["transcode_tasks_active"])
	}
	if s["vector_coarse_clip_active"] != 1 {
		t.Fatalf("vector_coarse_clip_active = %d, want 1", s["vector_coarse_clip_active"])
	}
	if s["vector_tasks_active"] != 0 {
		t.Fatalf("vector_tasks_active = %d, want 0", s["vector_tasks_active"])
	}
}
