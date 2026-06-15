package antspool

import (
	"errors"
	"testing"
	"time"
)

func TestMemoryRecorderTracksPoolStats(t *testing.T) {
	recorder := NewMemoryRecorder()

	recorder.OnPoolCreated("vector", 3)
	recorder.OnSubmit("vector")
	recorder.OnSubmit("vector")
	recorder.OnSubmitError("vector", errors.New("queue full"))
	recorder.OnTaskDone("vector", 25*time.Millisecond)
	recorder.OnTaskError("vector", errors.New("failed"), 50*time.Millisecond)

	snap := recorder.Snapshot("vector")
	if snap.Name != "vector" || snap.Size != 3 {
		t.Fatalf("snapshot identity = %#v", snap)
	}
	if snap.SubmittedTotal != 2 || snap.SubmitErrors != 1 || snap.CompletedTotal != 1 || snap.TaskErrors != 1 {
		t.Fatalf("snapshot counters = %#v", snap)
	}
	if snap.LastTaskCost != 50*time.Millisecond {
		t.Fatalf("LastTaskCost = %v, want 50ms", snap.LastTaskCost)
	}
}

func TestDefaultRecorderCanBeSetAndRead(t *testing.T) {
	previous := getDefaultRecorder()
	t.Cleanup(func() { SetDefaultRecorder(previous) })

	recorder := NewMemoryRecorder()
	SetDefaultRecorder(recorder)
	if got := getDefaultRecorder(); got != recorder {
		t.Fatalf("getDefaultRecorder() = %p, want %p", got, recorder)
	}
}
