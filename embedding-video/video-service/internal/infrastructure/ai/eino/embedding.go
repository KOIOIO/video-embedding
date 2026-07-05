package eino

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	einoembedding "github.com/cloudwego/eino/components/embedding"
)

type EmbeddingConfig struct {
	BaseURL string
	APIKey  string
	Model   string
}

type BatchEmbedFunc func(context.Context, []string) ([][]float32, error)

type EmbeddingClient struct {
	embed BatchEmbedFunc
}

func NewEmbeddingClientWithBatchFunc(embed BatchEmbedFunc) *EmbeddingClient {
	return &EmbeddingClient{embed: embed}
}

func NewEmbeddingClient(ctx context.Context, cfg EmbeddingConfig) (*EmbeddingClient, error) {
	baseURL := strings.TrimSpace(cfg.BaseURL)
	apiKey := strings.TrimSpace(cfg.APIKey)
	modelName := strings.TrimSpace(cfg.Model)
	if modelName == "" {
		modelName = "text-embedding-v4"
	}
	if baseURL == "" || apiKey == "" {
		return nil, errors.New("embedding base-url/api-key is required")
	}
	embedder := newOpenAICompatibleEmbedder(baseURL, apiKey, modelName)
	return NewEmbeddingClientWithBatchFunc(func(runCtx context.Context, texts []string) ([][]float32, error) {
		vecs, err := embedder.EmbedStrings(runCtx, texts, einoembedding.WithModel(modelName))
		if err != nil {
			return nil, err
		}
		out := make([][]float32, 0, len(vecs))
		for _, vec := range vecs {
			converted := make([]float32, 0, len(vec))
			for _, x := range vec {
				converted = append(converted, float32(x))
			}
			out = append(out, converted)
		}
		return out, nil
	}), nil
}

func (c *EmbeddingClient) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	if c == nil || c.embed == nil {
		return nil, errors.New("embedding function is required")
	}
	cleaned := make([]string, len(texts))
	for i, text := range texts {
		cleaned[i] = strings.TrimSpace(text)
	}
	vecs, err := c.embed(ctx, cleaned)
	if err != nil {
		return nil, err
	}
	if len(vecs) != len(cleaned) {
		return nil, fmt.Errorf("embedding size mismatch: got=%d want=%d", len(vecs), len(cleaned))
	}
	return vecs, nil
}

func (c *EmbeddingClient) EmbedText(ctx context.Context, text string) ([]float32, error) {
	vecs, err := c.Embed(ctx, []string{text})
	if err != nil {
		return nil, err
	}
	if len(vecs) != 1 {
		return nil, fmt.Errorf("embedding size mismatch: got=%d want=1", len(vecs))
	}
	return vecs[0], nil
}

type openAICompatibleEmbedder struct {
	baseURL    string
	apiKey     string
	model      string
	httpClient *http.Client
}

func newOpenAICompatibleEmbedder(baseURL string, apiKey string, model string) *openAICompatibleEmbedder {
	return &openAICompatibleEmbedder{
		baseURL:    strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		apiKey:     strings.TrimSpace(apiKey),
		model:      strings.TrimSpace(model),
		httpClient: &http.Client{Timeout: 10 * time.Minute},
	}
}

func (e *openAICompatibleEmbedder) EmbedStrings(ctx context.Context, texts []string, opts ...einoembedding.Option) ([][]float64, error) {
	options := einoembedding.GetCommonOptions(&einoembedding.Options{Model: &e.model}, opts...)
	modelName := e.model
	if options.Model != nil && strings.TrimSpace(*options.Model) != "" {
		modelName = strings.TrimSpace(*options.Model)
	}
	type reqBody struct {
		Model string   `json:"model"`
		Input []string `json:"input"`
	}
	payload, err := json.Marshal(reqBody{Model: modelName, Input: texts})
	if err != nil {
		return nil, err
	}
	var lastStatus int
	var lastBody string
	var lastURL string
	for _, u := range buildCandidateURLs(e.baseURL, "/embeddings") {
		status, body, err := e.doJSON(ctx, u, payload)
		if err != nil {
			return nil, err
		}
		if status >= 200 && status < 300 {
			var out struct {
				Data []struct {
					Embedding []float64 `json:"embedding"`
				} `json:"data"`
			}
			if err := json.Unmarshal([]byte(body), &out); err != nil {
				return nil, err
			}
			if len(out.Data) != len(texts) {
				return nil, fmt.Errorf("embedding size mismatch: got=%d want=%d", len(out.Data), len(texts))
			}
			vecs := make([][]float64, 0, len(out.Data))
			for _, d := range out.Data {
				vecs = append(vecs, d.Embedding)
			}
			return vecs, nil
		}
		lastStatus, lastBody, lastURL = status, body, u
		if status == http.StatusNotFound {
			continue
		}
		return nil, fmt.Errorf("embedding http %d url=%s: %s", status, u, body)
	}
	return nil, fmt.Errorf("embedding http %d url=%s: %s", lastStatus, lastURL, lastBody)
}

func (e *openAICompatibleEmbedder) doJSON(ctx context.Context, url string, payload []byte) (int, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return 0, "", err
	}
	req.Header.Set("Authorization", "Bearer "+e.apiKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := e.httpClient.Do(req)
	if err != nil {
		return 0, "", err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b), nil
}
