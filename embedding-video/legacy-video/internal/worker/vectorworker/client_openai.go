package vectorworker

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.uber.org/zap"
	"legacy-video/internal/config"
)

type openAICompatClient struct {
	compatBaseURL string
	asrBaseURL    string
	apiKey        string
	asrModel      string
	asrWSModel    string
	embedModel    string
	httpClient    *http.Client
}

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
		return nil, errors.New("missing api key: set DASHSCOPE_API_KEY or OPENAI_API_KEY")
	}
	compatBaseURL := firstNonEmpty(
		os.Getenv("DASHSCOPE_BASE_URL"),
		os.Getenv("OPENAI_BASE_URL"),
		cfg.Embedding.BaseURL,
		"https://dashscope.aliyuncs.com/compatible-mode/v1",
	)
	compatBaseURL = normalizeBaseURL(compatBaseURL)
	asrBaseURL := firstNonEmpty(
		os.Getenv("ASR_BASE_URL"),
		cfg.ASR.BaseURL,
		compatBaseURL,
	)
	asrBaseURL = normalizeASRBaseURL(asrBaseURL)
	asrModel := firstNonEmpty(os.Getenv("ASR_MODEL"), cfg.ASR.Options.Model, "paraformer-v2")
	asrWSModel := firstNonEmpty(os.Getenv("ASR_WS_MODEL"), cfg.ASR.Options.WSModel)
	embedModel := firstNonEmpty(os.Getenv("EMBED_MODEL"), cfg.Embedding.Options.Model, "text-embedding-v4")
	return &openAICompatClient{
		compatBaseURL: compatBaseURL,
		asrBaseURL:    asrBaseURL,
		apiKey:        apiKey,
		asrModel:      asrModel,
		asrWSModel:    asrWSModel,
		embedModel:    embedModel,
		httpClient:    &http.Client{Timeout: 10 * time.Minute},
	}, nil
}

// Transcribe 调用 ASR 接口识别音频文本。
// 当 DashScope 兼容接口不可用或模型不支持时，会按配置回退到 WebSocket 实时识别链路。
func (c *openAICompatClient) Transcribe(ctx context.Context, audioPath string) (string, error) {
	audioPath = strings.TrimSpace(audioPath)
	f, err := os.Open(audioPath)
	if err != nil {
		zap.L().Error("vectorize_asr_open_failed",
			zap.String("audio_path", audioPath),
			zap.Error(err))
		return "", err
	}
	defer f.Close()

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)

	fw, err := w.CreateFormFile("file", filepath.Base(audioPath))
	if err != nil {
		_ = w.Close()
		zap.L().Error("vectorize_asr_form_file_failed",
			zap.String("audio_path", audioPath),
			zap.Error(err))
		return "", err
	}
	if _, err := io.Copy(fw, f); err != nil {
		_ = w.Close()
		zap.L().Error("vectorize_asr_form_copy_failed",
			zap.String("audio_path", audioPath),
			zap.Error(err))
		return "", err
	}
	_ = w.WriteField("model", c.asrModel)
	if err := w.Close(); err != nil {
		zap.L().Error("vectorize_asr_form_close_failed",
			zap.String("audio_path", audioPath),
			zap.String("model", c.asrModel),
			zap.Error(err))
		return "", err
	}

	ct := w.FormDataContentType()
	body := buf.Bytes()
	urls := buildCandidateURLs(c.asrBaseURL, "/audio/transcriptions")
	var lastStatus int
	var lastBody string
	var lastURL string
	modelNotFound := false
	for _, u := range urls {
		status, respBody, err := c.doRequest(ctx, http.MethodPost, u, map[string]string{
			"Authorization": "Bearer " + c.apiKey,
			"Content-Type":  ct,
		}, body)
		if err != nil {
			zap.L().Error("vectorize_asr_http_request_failed",
				zap.String("audio_path", audioPath),
				zap.String("model", c.asrModel),
				zap.String("url", u),
				zap.Error(err))
			return "", err
		}
		if status >= 200 && status < 300 {
			var out struct {
				Text string `json:"text"`
			}
			if err := json.Unmarshal([]byte(respBody), &out); err != nil {
				return "", err
			}
			return strings.TrimSpace(out.Text), nil
		}
		lastStatus, lastBody, lastURL = status, respBody, u
		if status == http.StatusNotFound {
			continue
		}
		if strings.Contains(c.asrBaseURL, "dashscope.aliyuncs.com") && isModelNotFoundResp(status, respBody) {
			modelNotFound = true
			break
		}
		zap.L().Error("vectorize_asr_http_failed",
			zap.String("audio_path", audioPath),
			zap.String("model", c.asrModel),
			zap.String("url", u),
			zap.Int("status", status),
			zap.String("body", truncateForLog(respBody, 1000)))
		return "", fmt.Errorf("asr http %d url=%s: %s", status, u, respBody)
	}
	if strings.Contains(c.asrBaseURL, "/api/v1") && !isRealtimeASRModel(c.asrModel) {
		zap.L().Error("vectorize_asr_native_api_not_supported",
			zap.String("audio_path", audioPath),
			zap.String("requested_model", c.asrModel),
			zap.String("asr_base_url", c.asrBaseURL),
			zap.String("last_url", lastURL),
			zap.Int("status", lastStatus),
			zap.String("body", truncateForLog(lastBody, 1000)))
		return "", fmt.Errorf("native dashscope api/v1 transcription is not implemented for model=%s; request failed url=%s status=%d", c.asrModel, lastURL, lastStatus)
	}
	if strings.Contains(c.asrBaseURL, "dashscope.aliyuncs.com") && (lastStatus == http.StatusNotFound || modelNotFound) {
		wsModel := selectDashscopeWSModel(c.asrModel, c.asrWSModel)
		return dashscopeWSRecognizeWav(ctx, c.apiKey, wsModel, audioPath)
	}
	zap.L().Error("vectorize_asr_failed",
		zap.String("audio_path", audioPath),
		zap.String("model", c.asrModel),
		zap.String("url", lastURL),
		zap.Int("status", lastStatus),
		zap.String("body", truncateForLog(lastBody, 1000)))
	return "", fmt.Errorf("asr http %d url=%s: %s", lastStatus, lastURL, lastBody)
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
