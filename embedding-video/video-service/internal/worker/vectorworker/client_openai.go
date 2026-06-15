package vectorworker

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"go.uber.org/zap"
	"nlp-video-analysis/internal/config"
)

type openAICompatClient struct {
	compatBaseURL  string
	asrBaseURL     string
	asrWSURL       string
	apiKey         string
	asrWSModel     string
	asrWSFallbacks []string
	embedModel     string
	httpClient     *http.Client
}

var dashscopeWSRecognizeWavFunc = dashscopeWSRecognizeWav
var errMissingOpenAICompatAPIKey = errors.New("missing api key: set DASHSCOPE_API_KEY or OPENAI_API_KEY")

// newOpenAICompatClient 创建兼容 OpenAI 风格接口的客户端。
// 当前同时服务于 ASR、ChatCompletions 与 Embedding 三类请求。
func newOpenAICompatClient(cfg config.Config) (*openAICompatClient, error) {
	apiKey := firstNonEmpty(
		os.Getenv("DASHSCOPE_API_KEY"),
		os.Getenv("OPENAI_API_KEY"),
		os.Getenv("ASR_API_KEY"),
		cfg.ASR.APIKey,
		cfg.Embedding.APIKey,
	)
	if strings.TrimSpace(apiKey) == "" {
		return nil, errMissingOpenAICompatAPIKey
	}
	compatBaseURL := firstNonEmpty(
		os.Getenv("DASHSCOPE_BASE_URL"),
		os.Getenv("OPENAI_BASE_URL"),
		cfg.Embedding.BaseURL,
		config.DashscopeCompatBaseURL(),
	)
	compatBaseURL = normalizeBaseURL(compatBaseURL)
	asrBaseURL := firstNonEmpty(
		os.Getenv("ASR_BASE_URL"),
		cfg.ASR.BaseURL,
		compatBaseURL,
	)
	asrBaseURL = normalizeASRBaseURL(asrBaseURL)
	asrWSURL := config.ASRWSURL(cfg)
	asrWSModel := firstNonEmpty(os.Getenv("ASR_WS_MODEL"), cfg.ASR.Options.WSModel)
	asrWSFallbacks := cfg.ASR.Options.WSFallbacks
	embedModel := firstNonEmpty(os.Getenv("EMBED_MODEL"), cfg.Embedding.Options.Model, "text-embedding-v4")
	return &openAICompatClient{
		compatBaseURL:  compatBaseURL,
		asrBaseURL:     asrBaseURL,
		asrWSURL:       asrWSURL,
		apiKey:         apiKey,
		asrWSModel:     asrWSModel,
		asrWSFallbacks: asrWSFallbacks,
		embedModel:     embedModel,
		httpClient:     &http.Client{Timeout: 10 * time.Minute},
	}, nil
}

// Transcribe 使用 DashScope WebSocket 实时识别模型做转写。
// 当首选 ws-model 额度耗尽时，会按配置顺序切换到 ws-model-fallbacks。
func (c *openAICompatClient) Transcribe(ctx context.Context, audioPath string) (string, error) {
	audioPath = strings.TrimSpace(audioPath)
	if _, err := os.Stat(audioPath); err != nil {
		zap.L().Error("vectorize_asr_open_failed",
			zap.String("audio_path", audioPath),
			zap.Error(err))
		return "", err
	}
	return c.transcribeViaWS(ctx, audioPath)
}

func (c *openAICompatClient) transcribeViaWS(ctx context.Context, audioPath string) (string, error) {
	candidates := buildASRWSModelCandidates(selectDashscopeWSModel("", c.asrWSModel), c.asrWSFallbacks)
	var lastErr error
	for i, wsModel := range candidates {
		text, err := dashscopeWSRecognizeWavFunc(ctx, c.asrWSURL, c.apiKey, wsModel, audioPath)
		if err == nil {
			if i > 0 {
				zap.L().Warn("vectorize_asr_ws_model_rotated",
					zap.String("audio_path", audioPath),
					zap.String("model", wsModel),
					zap.Int("attempt", i+1))
			}
			return text, nil
		}
		lastErr = err
		if !isASRQuotaExhaustedResp(http.StatusForbidden, err.Error()) {
			return "", err
		}
		zap.L().Warn("vectorize_asr_ws_model_quota_exhausted",
			zap.String("audio_path", audioPath),
			zap.String("model", wsModel),
			zap.Int("attempt", i+1),
			zap.Error(err))
	}
	if lastErr != nil {
		return "", lastErr
	}
	return "", errors.New("no available asr ws model")
}

// truncateForLog 截断日志中的响应体，避免错误日志过长。
func truncateForLog(s string, max int) string {
	s = strings.TrimSpace(s)
	if max <= 0 || len(s) <= max {
		return s
	}
	return s[:max]
}

// isRealtimeASRModel 判断当前 ASR 模型名是否属于实时识别模型。
func isRealtimeASRModel(model string) bool {
	model = strings.ToLower(strings.TrimSpace(model))
	return strings.Contains(model, "realtime")
}

func buildASRWSModelCandidates(primary string, fallbacks []string) []string {
	seen := make(map[string]struct{}, 1+len(fallbacks))
	out := make([]string, 0, 1+len(fallbacks))
	appendModel := func(model string) {
		model = normalizeDashscopeWSModel(model)
		if strings.TrimSpace(model) == "" {
			return
		}
		if _, ok := seen[model]; ok {
			return
		}
		seen[model] = struct{}{}
		out = append(out, model)
	}
	appendModel(primary)
	for _, fallback := range fallbacks {
		appendModel(fallback)
	}
	if len(out) == 0 {
		appendModel("fun-asr-realtime")
	}
	return out
}

func isASRQuotaExhaustedResp(status int, body string) bool {
	if status < 400 || status >= 500 {
		return false
	}
	b := strings.ToLower(strings.TrimSpace(body))
	return strings.Contains(b, "quota") ||
		strings.Contains(b, "allocationquota.freetieronly") ||
		strings.Contains(b, "free tier") ||
		strings.Contains(b, "has been exhausted") ||
		strings.Contains(b, "insufficient balance") ||
		strings.Contains(b, "余额不足") ||
		strings.Contains(b, "额度不足")
}

// selectDashscopeWSModel 选择 DashScope WebSocket ASR 要使用的模型名。
func selectDashscopeWSModel(requestedModel string, configuredWSModel string) string {
	if strings.TrimSpace(configuredWSModel) != "" {
		return normalizeDashscopeWSModel(configuredWSModel)
	}
	req := normalizeDashscopeWSModel(requestedModel)
	if strings.Contains(strings.ToLower(req), "realtime") || strings.EqualFold(req, "fun-asr-realtime") {
		return req
	}
	return "fun-asr-realtime"
}

// isModelNotFoundResp 判断响应是否属于“模型不存在”一类错误。
func isModelNotFoundResp(status int, body string) bool {
	if status == http.StatusNotFound {
		return true
	}
	if status < 400 || status >= 500 {
		return false
	}
	b := strings.ToLower(strings.TrimSpace(body))
	return strings.Contains(b, "model not found")
}

// Embed 调用 Embedding 接口批量生成文本向量。
func (c *openAICompatClient) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	type reqBody struct {
		Model string   `json:"model"`
		Input []string `json:"input"`
	}
	payload, err := json.Marshal(reqBody{
		Model: c.embedModel,
		Input: texts,
	})
	if err != nil {
		return nil, err
	}

	urls := buildCandidateURLs(c.compatBaseURL, "/embeddings")
	var lastStatus int
	var lastBody string
	var lastURL string
	var out struct {
		Data []struct {
			Embedding []float64 `json:"embedding"`
		} `json:"data"`
	}
	for _, u := range urls {
		status, respBody, err := c.doRequest(ctx, http.MethodPost, u, map[string]string{
			"Authorization": "Bearer " + c.apiKey,
			"Content-Type":  "application/json",
		}, payload)
		if err != nil {
			return nil, err
		}
		if status >= 200 && status < 300 {
			if err := json.Unmarshal([]byte(respBody), &out); err != nil {
				return nil, err
			}
			if len(out.Data) != len(texts) {
				return nil, fmt.Errorf("embedding size mismatch: got=%d want=%d", len(out.Data), len(texts))
			}
			vecs := make([][]float32, 0, len(out.Data))
			for _, d := range out.Data {
				v := make([]float32, 0, len(d.Embedding))
				for _, x := range d.Embedding {
					v = append(v, float32(x))
				}
				vecs = append(vecs, v)
			}
			return vecs, nil
		}
		lastStatus, lastBody, lastURL = status, respBody, u
		if status == http.StatusNotFound {
			continue
		}
		return nil, fmt.Errorf("embedding http %d url=%s: %s", status, u, respBody)
	}
	return nil, fmt.Errorf("embedding http %d url=%s: %s", lastStatus, lastURL, lastBody)
}

// ChatCompletions 调用兼容 OpenAI 的聊天接口，要求模型只返回 JSON 文本。
func (c *openAICompatClient) ChatCompletions(ctx context.Context, model string, prompt string) (string, error) {
	model = strings.TrimSpace(model)
	if model == "" {
		return "", errors.New("model is required")
	}
	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		return "", errors.New("prompt is required")
	}

	type message struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	type reqBody struct {
		Model       string    `json:"model"`
		Messages    []message `json:"messages"`
		Temperature float64   `json:"temperature"`
	}
	payload, err := json.Marshal(reqBody{
		Model: model,
		Messages: []message{
			{Role: "system", Content: "You are a helpful assistant that only outputs valid JSON."},
			{Role: "user", Content: prompt},
		},
		Temperature: 0.2,
	})
	if err != nil {
		return "", err
	}

	urls := buildCandidateURLs(c.compatBaseURL, "/chat/completions")
	var lastStatus int
	var lastBody string
	var lastURL string
	for _, u := range urls {
		// 对单个候选 URL 做有限次重试，吸收 429 和部分 5xx 短暂抖动。
		for attempt := 0; attempt < 3; attempt++ {
			status, respBody, err := c.doRequest(ctx, http.MethodPost, u, map[string]string{
				"Authorization": "Bearer " + c.apiKey,
				"Content-Type":  "application/json",
			}, payload)
			if err != nil {
				if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
					return "", err
				}
				lastStatus, lastBody, lastURL = 0, err.Error(), u
			} else if status >= 200 && status < 300 {
				var out struct {
					Choices []struct {
						Message struct {
							Content string `json:"content"`
						} `json:"message"`
					} `json:"choices"`
				}
				if err := json.Unmarshal([]byte(respBody), &out); err != nil {
					return "", err
				}
				if len(out.Choices) == 0 {
					return "", errors.New("chat response has no choices")
				}
				return strings.TrimSpace(out.Choices[0].Message.Content), nil
			} else {
				lastStatus, lastBody, lastURL = status, respBody, u
				if status == http.StatusNotFound {
					break
				}
				if status != http.StatusTooManyRequests && status < 500 {
					return "", fmt.Errorf("chat http %d url=%s: %s", status, u, respBody)
				}
			}

			if attempt < 2 {
				var d time.Duration
				if attempt == 0 {
					d = 200 * time.Millisecond
				} else {
					d = 800 * time.Millisecond
				}
				select {
				case <-time.After(d):
				case <-ctx.Done():
					return "", ctx.Err()
				}
			}
		}
	}
	return "", fmt.Errorf("chat http %d url=%s: %s", lastStatus, lastURL, lastBody)
}

// ChatCompletionsWithTimeout 为 LLM 调用补一层显式超时控制。
func (c *openAICompatClient) ChatCompletionsWithTimeout(ctx context.Context, model string, prompt string, timeoutMinutes int) (string, error) {
	if timeoutMinutes <= 0 {
		timeoutMinutes = 3
	}
	tctx, cancel := context.WithTimeout(ctx, time.Duration(timeoutMinutes)*time.Minute)
	defer cancel()
	return c.ChatCompletions(tctx, model, prompt)
}

// doRequest 发送一个 HTTP 请求并返回状态码与原始响应体。
func (c *openAICompatClient) doRequest(ctx context.Context, method string, url string, headers map[string]string, body []byte) (int, string, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(body))
	if err != nil {
		return 0, "", err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, "", err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b), nil
}

// buildCandidateURLs 为兼容不同 baseURL 形态生成多个候选请求地址。
func buildCandidateURLs(baseURL string, path string) []string {
	baseURL = strings.TrimSpace(baseURL)
	baseURL = strings.TrimRight(baseURL, "/")
	path = strings.TrimSpace(path)
	if path == "" {
		path = "/"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	baseNoV1 := baseURL
	baseNoV1 = strings.TrimSuffix(baseNoV1, "/v1")

	candidates := []string{
		baseURL + path,
		baseNoV1 + "/v1" + path,
		baseNoV1 + path,
	}
	uniq := make([]string, 0, len(candidates))
	seen := make(map[string]struct{}, len(candidates))
	for _, u := range candidates {
		u = strings.TrimSpace(u)
		if u == "" {
			continue
		}
		if _, ok := seen[u]; ok {
			continue
		}
		seen[u] = struct{}{}
		uniq = append(uniq, u)
	}
	return uniq
}
