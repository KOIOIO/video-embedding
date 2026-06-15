package runtime

import (
	"context"
	goruntime "runtime"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/mem"

	"nlp-video-analysis/internal/http/dto"
)

var counterNames = []string{
	"transcode_tasks_active",
	"vector_tasks_active",
	"vector_coarse_clip_active",
	"vector_coarse_upload_active",
	"vector_coarse_asr_active",
	"vector_refine_asr_active",
}

type ActiveCounterStore interface {
	Inc(name string)
	Dec(name string)
	Snapshot() map[string]int
	Reset()
}

type memoryActiveCounterStore struct {
	mu     sync.Mutex
	counts map[string]int
}

func NewMemoryActiveCounterStore() ActiveCounterStore {
	counts := make(map[string]int, len(counterNames))
	for _, name := range counterNames {
		counts[name] = 0
	}
	return &memoryActiveCounterStore{counts: counts}
}

func (s *memoryActiveCounterStore) Inc(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.counts[name]++
}

func (s *memoryActiveCounterStore) Dec(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.counts[name] > 0 {
		s.counts[name]--
	}
}

func (s *memoryActiveCounterStore) Snapshot() map[string]int {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make(map[string]int, len(s.counts))
	for k, v := range s.counts {
		out[k] = v
	}
	return out
}

func (s *memoryActiveCounterStore) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for k := range s.counts {
		s.counts[k] = 0
	}
}

type MetricsService struct {
	Counters ActiveCounterStore
}

func (s MetricsService) GetSystemMetrics(ctx context.Context) (dto.SystemMetricsData, error) {
	vm, err := mem.VirtualMemoryWithContext(ctx)
	if err != nil {
		return dto.SystemMetricsData{}, err
	}
	cpuPercents, err := cpu.PercentWithContext(ctx, 0, false)
	if err != nil {
		return dto.SystemMetricsData{}, err
	}
	var m goruntime.MemStats
	goruntime.ReadMemStats(&m)
	cpuPercent := 0.0
	if len(cpuPercents) > 0 {
		cpuPercent = cpuPercents[0]
	}
	activeCounts := map[string]int{}
	if s.Counters != nil {
		activeCounts = s.Counters.Snapshot()
	}
	return dto.SystemMetricsData{
		CPUPercent:         cpuPercent,
		MemoryUsedBytes:    vm.Used,
		MemoryTotalBytes:   vm.Total,
		MemoryUsedPercent:  vm.UsedPercent,
		ProcessMemoryBytes: m.Alloc,
		Goroutines:         goruntime.NumGoroutine(),
		ActiveCounts:       activeCounts,
		Timestamp:          time.Now().Format(time.RFC3339),
	}, nil
}
