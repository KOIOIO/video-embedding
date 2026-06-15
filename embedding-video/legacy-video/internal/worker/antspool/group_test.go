package antspool

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

type stubRecorder struct {
	mu          sync.Mutex
	created     []string
	submitted   int
	submitErrs  int
	taskDone    int
	taskErrs    int
}

func (r *stubRecorder) OnPoolCreated(name string, size int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.created = append(r.created, name)
}

func (r *stubRecorder) OnSubmit(name string) {
	r.mu.Lock()
	r.submitted++
	r.mu.Unlock()
}

func (r *stubRecorder) OnSubmitError(name string, err error) {
	r.mu.Lock()
	r.submitErrs++
	r.mu.Unlock()
}

func (r *stubRecorder) OnTaskDone(name string, dur time.Duration) {
	r.mu.Lock()
	r.taskDone++
	r.mu.Unlock()
}

func (r *stubRecorder) OnTaskError(name string, err error, dur time.Duration) {
	r.mu.Lock()
	r.taskErrs++
	r.mu.Unlock()
}

func TestGroupWaitRunsAllSubmittedTasks(t *testing.T) {
	g, err := New(context.Background(), Options{Size: 2})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer g.Release()

	var mu sync.Mutex
	seen := make([]int, 0, 4)
	for i := 0; i < 4; i++ {
		i := i
		if err := g.Submit(func() error {
			mu.Lock()
			seen = append(seen, i)
			mu.Unlock()
			return nil
		}); err != nil {
			t.Fatalf("Submit() error = %v", err)
		}
	}

	if err := g.Wait(); err != nil {
		t.Fatalf("Wait() error = %v", err)
	}
	if len(seen) != 4 {
		t.Fatalf("executed tasks = %d, want 4", len(seen))
	}
}

func TestGroupWaitReturnsFirstTaskError(t *testing.T) {
	g, err := New(context.Background(), Options{Size: 2})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer g.Release()

	want := errors.New("boom")
	if err := g.Submit(func() error { return want }); err != nil {
		t.Fatalf("Submit() error = %v", err)
	}
	if err := g.Submit(func() error { return nil }); err != nil {
		t.Fatalf("Submit() error = %v", err)
	}

	if err := g.Wait(); !errors.Is(err, want) {
		t.Fatalf("Wait() error = %v, want %v", err, want)
	}
}

func TestGroupSubmitRespectsCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	g, err := New(ctx, Options{Size: 1})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer g.Release()

	if err := g.Submit(func() error { return nil }); !errors.Is(err, context.Canceled) {
		t.Fatalf("Submit() error = %v, want context.Canceled", err)
	}
}

func TestGroupMetricsRecorderReceivesLifecycleCallbacks(t *testing.T) {
	recorder := &stubRecorder{}
	g, err := New(context.Background(), Options{
		Name:     "vector.refine_asr",
		Size:     2,
		Recorder: recorder,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer g.Release()

	if err := g.Submit(func() error { return nil }); err != nil {
		t.Fatalf("Submit() error = %v", err)
	}
	if err := g.Submit(func() error { return errors.New("boom") }); err != nil {
		t.Fatalf("Submit() error = %v", err)
	}

	if err := g.Wait(); err == nil {
		t.Fatal("Wait() error = nil, want non-nil")
	}

	recorder.mu.Lock()
	defer recorder.mu.Unlock()
	if len(recorder.created) != 1 || recorder.created[0] != "vector.refine_asr" {
		t.Fatalf("created callbacks = %v, want [vector.refine_asr]", recorder.created)
	}
	if recorder.submitted != 2 {
		t.Fatalf("submitted callbacks = %d, want 2", recorder.submitted)
	}
	if recorder.taskDone != 1 {
		t.Fatalf("taskDone callbacks = %d, want 1", recorder.taskDone)
	}
	if recorder.taskErrs != 1 {
		t.Fatalf("taskErrs callbacks = %d, want 1", recorder.taskErrs)
	}
	if recorder.submitErrs != 0 {
		t.Fatalf("submitErrs callbacks = %d, want 0", recorder.submitErrs)
	}
}
