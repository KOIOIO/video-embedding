package embedding

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

	"nlp-video-analysis/internal/config"
)

type Client struct {
	cfg        config.EmbeddingConfig
	httpClient *http.Client
}

func NewClient(cfg config.EmbeddingConfig) *Client {
	return &Client{
		cfg:        cfg,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *Client) Embed(ctx context.Context, text string) ([]float32, error) {
	baseURL := strings.TrimSpace(c.cfg.BaseURL)
	apiKey := firstNonEmpty(os.Getenv("DASHSCOPE_API_KEY"), os.Getenv("OPENAI_API_KEY"), os.Getenv("EMBEDDING_API_KEY"), c.cfg.APIKey)
	model := strings.TrimSpace(c.cfg.Options.Model)
	if model == "" {
		model = "text-embedding-v4"
	}
	if baseURL == "" || apiKey == "" {
		return nil, errors.New("embedding base-url/api-key is required")
	}

	type reqBody struct {
		Model string   `json:"model"`
		Input []string `json:"input"`
	}
	payload, err := json.Marshal(reqBody{Model: model, Input: []string{text}})
	if err != nil {
		return nil, err
	}

	urls := buildCandidateURLs(baseURL, "/embeddings")
	var lastStatus int
	var lastBody string
	var lastURL string
	var out struct {
		Data []struct {
			Embedding []float64 `json:"embedding"`
		} `json:"data"`
	}

	for _, u := range urls {
		statusCode, body, err := c.doJSON(ctx, u, apiKey, payload)
		if err != nil {
			return nil, err
		}
		if statusCode >= 200 && statusCode < 300 {
			if err := json.Unmarshal([]byte(body), &out); err != nil {
				return nil, err
			}
			if len(out.Data) != 1 {
				return nil, fmt.Errorf("embedding size mismatch: got=%d want=1", len(out.Data))
			}
			v := make([]float32, 0, len(out.Data[0].Embedding))
			for _, x := range out.Data[0].Embedding {
				v = append(v, float32(x))
			}
			return v, nil
		}
		lastStatus, lastBody, lastURL = statusCode, body, u
		if statusCode == http.StatusNotFound {
			continue
		}
		return nil, fmt.Errorf("embedding http %d url=%s: %s", statusCode, u, body)
	}
	return nil, fmt.Errorf("embedding http %d url=%s: %s", lastStatus, lastURL, lastBody)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if v := strings.TrimSpace(value); v != "" {
			return v
		}
	}
	return ""
}

func (c *Client) doJSON(ctx context.Context, url string, apiKey string, payload []byte) (int, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return 0, "", err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, "", err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b), nil
}

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
	baseNoV1 := strings.TrimSuffix(baseURL, "/v1")
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
