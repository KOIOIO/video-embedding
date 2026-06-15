package antspool

import (
	"sync"
	"time"
)

var defaultRecorderMu sync.RWMutex
var defaultRecorder Recorder

func SetDefaultRecorder(recorder Recorder) {
	defaultRecorderMu.Lock()
	defer defaultRecorderMu.Unlock()
	defaultRecorder = recorder
}

func getDefaultRecorder() Recorder {
	defaultRecorderMu.RLock()
	defer defaultRecorderMu.RUnlock()
	return defaultRecorder
}

// Snapshot 描述单个协程池当前的轻量统计信息。
type Snapshot struct {
	Name           string
	Size           int
	SubmittedTotal int
	SubmitErrors   int
	CompletedTotal int
	TaskErrors     int
	LastTaskCost   time.Duration
}

// MemoryRecorder 以进程内存形式保存 ants 协程池摘要指标。
type MemoryRecorder struct {
	mu    sync.Mutex
	stats map[string]*Snapshot
}

func NewMemoryRecorder() *MemoryRecorder {
	return &MemoryRecorder{stats: make(map[string]*Snapshot)}
}

func (r *MemoryRecorder) ensure(name string) *Snapshot {
	if r.stats == nil {
		r.stats = make(map[string]*Snapshot)
	}
	if s, ok := r.stats[name]; ok {
		return s
	}
	s := &Snapshot{Name: name}
	r.stats[name] = s
	return s
}

func (r *MemoryRecorder) OnPoolCreated(name string, size int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s := r.ensure(name)
	s.Size = size
}

func (r *MemoryRecorder) OnSubmit(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.ensure(name).SubmittedTotal++
}

func (r *MemoryRecorder) OnSubmitError(name string, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.ensure(name).SubmitErrors++
}

func (r *MemoryRecorder) OnTaskDone(name string, dur time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s := r.ensure(name)
	s.CompletedTotal++
	s.LastTaskCost = dur
}

func (r *MemoryRecorder) OnTaskError(name string, err error, dur time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s := r.ensure(name)
	s.TaskErrors++
	s.LastTaskCost = dur
}

func (r *MemoryRecorder) Snapshot(name string) Snapshot {
	r.mu.Lock()
	defer r.mu.Unlock()
	if s, ok := r.stats[name]; ok {
		return *s
	}
	return Snapshot{Name: name}
}
