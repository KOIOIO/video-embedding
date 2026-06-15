package dto

type SystemMetricsData struct {
	CPUPercent         float64 `json:"cpu_percent"`
	MemoryUsedBytes    uint64  `json:"memory_used_bytes"`
	MemoryTotalBytes   uint64  `json:"memory_total_bytes"`
	MemoryUsedPercent  float64 `json:"memory_used_percent"`
	ProcessMemoryBytes uint64  `json:"process_memory_bytes"`
	Goroutines         int     `json:"goroutines"`
	ActiveCounts       map[string]int `json:"active_counts"`
	Timestamp          string  `json:"timestamp"`
}

type SystemMetricsResponse struct {
	Success bool              `json:"success"`
	Data    SystemMetricsData `json:"data"`
}
