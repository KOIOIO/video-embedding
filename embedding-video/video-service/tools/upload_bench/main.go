package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type requestResult struct {
	FilePath   string        `json:"file_path"`
	StatusCode int           `json:"status_code"`
	Duration   time.Duration `json:"duration"`
	ErrorCode  string        `json:"error_code,omitempty"`
	ErrorMsg   string        `json:"error_message,omitempty"`
	Success    bool          `json:"success"`
}

type fileSummary struct {
	Requests int   `json:"requests"`
	AvgMs    int64 `json:"avg_ms"`
}

type summary struct {
	TotalRequests   int                    `json:"total_requests"`
	SuccessRequests int                    `json:"success_requests"`
	FailedRequests  int                    `json:"failed_requests"`
	SuccessRate     float64                `json:"success_rate"`
	TotalDurationMs int64                  `json:"total_duration_ms"`
	ThroughputRPS   float64                `json:"throughput_rps"`
	AvgMs           int64                  `json:"avg_ms"`
	MinMs           int64                  `json:"min_ms"`
	MaxMs           int64                  `json:"max_ms"`
	P50Ms           int64                  `json:"p50_ms"`
	P90Ms           int64                  `json:"p90_ms"`
	P95Ms           int64                  `json:"p95_ms"`
	P99Ms           int64                  `json:"p99_ms"`
	StatusCodes     map[int]int            `json:"status_codes"`
	ErrorCodes      map[string]int         `json:"error_codes"`
	Files           map[string]fileSummary `json:"files"`
}

type output struct {
	StartedAt string          `json:"started_at"`
	BaseURL   string          `json:"base_url"`
	Dir       string          `json:"dir"`
	Summary   summary         `json:"summary"`
	Results   []requestResult `json:"results"`
}

type errorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type apiResponse struct {
	Success bool      `json:"success"`
	Error   errorBody `json:"error"`
}

func main() {
	baseURL := flag.String("base-url", defaultBaseURL(), "upload service base URL")
	dir := flag.String("dir", "", "directory containing video files")
	concurrency := flag.Int("concurrency", 2, "number of concurrent upload workers")
	requests := flag.Int("requests", 10, "total upload requests to send")
	timeout := flag.Duration("timeout", 10*time.Minute, "per-request timeout")
	titlePrefix := flag.String("title-prefix", "bench", "prefix for generated title")
	description := flag.String("description", "upload bench", "description form field")
	randomPick := flag.Bool("random", false, "pick files randomly instead of round-robin")
	outPath := flag.String("out", "", "optional JSON output file path")
	flag.Parse()

	if strings.TrimSpace(*dir) == "" {
		fatalf("--dir is required")
	}
	if *concurrency <= 0 {
		fatalf("--concurrency must be > 0")
	}
	if *requests <= 0 {
		fatalf("--requests must be > 0")
	}

	files, err := collectInputFiles(*dir)
	if err != nil {
		fatalf("collect video files: %v", err)
	}
	if len(files) == 0 {
		fatalf("no video files found in %s", *dir)
	}

	client := &http.Client{Timeout: *timeout}
	jobs := make(chan int)
	resultsCh := make(chan requestResult, *requests)
	var rrIndex uint64
	var wg sync.WaitGroup
	startedAt := time.Now()

	for w := 0; w < *concurrency; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for i := range jobs {
				filePath := pickFile(files, i, *randomPick, &rrIndex)
				result := uploadOne(client, strings.TrimRight(*baseURL, "/")+"/api/videos", filePath, fmt.Sprintf("%s-%d", *titlePrefix, i), *description)
				resultsCh <- result
			}
		}(w)
	}

	for i := 0; i < *requests; i++ {
		jobs <- i
	}
	close(jobs)
	wg.Wait()
	close(resultsCh)

	results := make([]requestResult, 0, *requests)
	for r := range resultsCh {
		results = append(results, r)
	}
	totalElapsed := time.Since(startedAt)
	s := buildSummary(results, totalElapsed)
	printSummary(s)

	if strings.TrimSpace(*outPath) != "" {
		payload := output{
			StartedAt: startedAt.Format(time.RFC3339),
			BaseURL:   *baseURL,
			Dir:       *dir,
			Summary:   s,
			Results:   results,
		}
		if err := writeJSON(*outPath, payload); err != nil {
			fatalf("write output: %v", err)
		}
	}
}

func defaultBaseURL() string {
	if v := strings.TrimSpace(os.Getenv("UPLOAD_BENCH_BASE_URL")); v != "" {
		return v
	}
	if addr := strings.TrimSpace(os.Getenv("HTTP_ADDR")); addr != "" {
		if strings.HasPrefix(addr, ":") {
			return "http://localhost" + addr
		}
		return "http://" + addr
	}
	return "http://localhost:8081"
}

func collectInputFiles(path string) ([]string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		if !isVideoFile(path) {
			return nil, fmt.Errorf("input file is not a supported video: %s", path)
		}
		return []string{path}, nil
	}
	return collectVideoFilesFromDir(path)
}

func collectVideoFilesFromDir(dir string) ([]string, error) {
	var out []string
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if isVideoFile(path) {
			out = append(out, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(out)
	return out, nil
}

func isVideoFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".mp4", ".mov", ".mkv", ".avi", ".flv", ".webm", ".m4v":
		return true
	default:
		return false
	}
}

func pickFile(files []string, i int, randomPick bool, idx *uint64) string {
	if !randomPick {
		return files[i%len(files)]
	}
	n := atomic.AddUint64(idx, 1)
	return files[int(n)%len(files)]
}

func uploadOne(client *http.Client, url string, filePath string, title string, description string) requestResult {
	start := time.Now()
	res := requestResult{FilePath: filePath}
	file, err := os.Open(filePath)
	if err != nil {
		res.Duration = time.Since(start)
		res.ErrorMsg = err.Error()
		return res
	}
	defer file.Close()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", filepath.Base(filePath))
	if err != nil {
		res.Duration = time.Since(start)
		res.ErrorMsg = err.Error()
		return res
	}
	if _, err := io.Copy(part, file); err != nil {
		res.Duration = time.Since(start)
		res.ErrorMsg = err.Error()
		return res
	}
	_ = writer.WriteField("title", title)
	_ = writer.WriteField("description", description)
	if err := writer.Close(); err != nil {
		res.Duration = time.Since(start)
		res.ErrorMsg = err.Error()
		return res
	}

	req, err := http.NewRequest(http.MethodPost, url, &body)
	if err != nil {
		res.Duration = time.Since(start)
		res.ErrorMsg = err.Error()
		return res
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	rsp, err := client.Do(req)
	res.Duration = time.Since(start)
	if err != nil {
		res.ErrorMsg = err.Error()
		return res
	}
	defer rsp.Body.Close()
	res.StatusCode = rsp.StatusCode
	res.Success = rsp.StatusCode >= 200 && rsp.StatusCode < 300

	b, _ := io.ReadAll(rsp.Body)
	if !res.Success {
		var parsed apiResponse
		if json.Unmarshal(b, &parsed) == nil {
			res.ErrorCode = parsed.Error.Code
			res.ErrorMsg = parsed.Error.Message
		} else {
			res.ErrorMsg = strings.TrimSpace(string(b))
		}
	}
	return res
}

func buildSummary(results []requestResult, elapsed time.Duration) summary {
	s := summary{
		TotalRequests:   len(results),
		StatusCodes:     make(map[int]int),
		ErrorCodes:      make(map[string]int),
		Files:           make(map[string]fileSummary),
		TotalDurationMs: elapsed.Milliseconds(),
	}
	if len(results) == 0 {
		return s
	}
	durations := make([]int64, 0, len(results))
	var totalLatency int64
	s.MinMs = results[0].Duration.Milliseconds()
	for _, r := range results {
		ms := r.Duration.Milliseconds()
		durations = append(durations, ms)
		totalLatency += ms
		if r.Success {
			s.SuccessRequests++
		} else {
			s.FailedRequests++
		}
		s.StatusCodes[r.StatusCode]++
		if r.ErrorCode != "" {
			s.ErrorCodes[r.ErrorCode]++
		}
		if ms < s.MinMs {
			s.MinMs = ms
		}
		if ms > s.MaxMs {
			s.MaxMs = ms
		}
		fs := s.Files[r.FilePath]
		fs.Requests++
		fs.AvgMs += ms
		s.Files[r.FilePath] = fs
	}
	s.AvgMs = totalLatency / int64(len(results))
	if len(results) > 0 {
		s.SuccessRate = float64(s.SuccessRequests) / float64(len(results))
	}
	if elapsed > 0 {
		s.ThroughputRPS = float64(len(results)) / elapsed.Seconds()
	}
	sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })
	s.P50Ms = percentile(durations, 0.50)
	s.P90Ms = percentile(durations, 0.90)
	s.P95Ms = percentile(durations, 0.95)
	s.P99Ms = percentile(durations, 0.99)
	for k, fs := range s.Files {
		fs.AvgMs = fs.AvgMs / int64(fs.Requests)
		s.Files[k] = fs
	}
	return s
}

func percentile(values []int64, p float64) int64 {
	if len(values) == 0 {
		return 0
	}
	if p <= 0 {
		return values[0]
	}
	if p >= 1 {
		return values[len(values)-1]
	}
	idx := int(math.Ceil(float64(len(values))*p)) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(values) {
		idx = len(values) - 1
	}
	return values[idx]
}

func printSummary(s summary) {
	fmt.Printf("Total Requests: %d\n", s.TotalRequests)
	fmt.Printf("Success: %d\n", s.SuccessRequests)
	fmt.Printf("Failed: %d\n", s.FailedRequests)
	fmt.Printf("Success Rate: %.2f%%\n", s.SuccessRate*100)
	fmt.Printf("Elapsed: %d ms\n", s.TotalDurationMs)
	fmt.Printf("Throughput: %.2f req/s\n", s.ThroughputRPS)
	fmt.Printf("Avg: %d ms\n", s.AvgMs)
	fmt.Printf("Min: %d ms\n", s.MinMs)
	fmt.Printf("Max: %d ms\n", s.MaxMs)
	fmt.Printf("P50: %d ms\n", s.P50Ms)
	fmt.Printf("P90: %d ms\n", s.P90Ms)
	fmt.Printf("P95: %d ms\n", s.P95Ms)
	fmt.Printf("P99: %d ms\n", s.P99Ms)
}

func writeJSON(path string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
