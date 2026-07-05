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

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

type ChatConfig struct {
	BaseURL string
	APIKey  string
}

type ChatRequest struct {
	Model       string
	System      string
	Prompt      string
	Temperature float64
}

type ChatGenerator func(context.Context, ChatRequest) (string, error)

type ChatClient struct {
	generate ChatGenerator
}

func NewChatClientWithGenerator(generate ChatGenerator) *ChatClient {
	return &ChatClient{generate: generate}
}

func NewChatClient(ctx context.Context, cfg ChatConfig) (*ChatClient, error) {
	baseURL := strings.TrimSpace(cfg.BaseURL)
	apiKey := strings.TrimSpace(cfg.APIKey)
	if baseURL == "" || apiKey == "" {
		return nil, errors.New("chat base-url/api-key is required")
	}
	chatModel := newOpenAICompatibleChatModel(baseURL, apiKey)
	return NewChatClientWithGenerator(func(runCtx context.Context, req ChatRequest) (string, error) {
		temp := float32(req.Temperature)
		resp, err := chatModel.Generate(runCtx, []*schema.Message{
			schema.SystemMessage(req.System),
			schema.UserMessage(req.Prompt),
		}, model.WithModel(req.Model), model.WithTemperature(temp))
		if err != nil {
			return "", err
		}
		if resp == nil {
			return "", errors.New("chat response is empty")
		}
		return resp.Content, nil
	}), nil
}

func (c *ChatClient) ChatCompletionsWithTimeout(ctx context.Context, model string, prompt string, timeoutMinutes int) (string, error) {
	model = strings.TrimSpace(model)
	if model == "" {
		return "", errors.New("model is required")
	}
	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		return "", errors.New("prompt is required")
	}
	if timeoutMinutes <= 0 {
		timeoutMinutes = 1
	}
	if c == nil || c.generate == nil {
		return "", errors.New("chat generator is required")
	}
	tctx, cancel := context.WithTimeout(ctx, time.Duration(timeoutMinutes)*time.Minute)
	defer cancel()
	out, err := c.generate(tctx, ChatRequest{
		Model:       model,
		System:      "You are a helpful assistant that only outputs valid JSON.",
		Prompt:      prompt,
		Temperature: 0.2,
	})
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

type openAICompatibleChatModel struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

func newOpenAICompatibleChatModel(baseURL string, apiKey string) *openAICompatibleChatModel {
	return &openAICompatibleChatModel{
		baseURL:    strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		apiKey:     strings.TrimSpace(apiKey),
		httpClient: &http.Client{Timeout: 10 * time.Minute},
	}
}

func (m *openAICompatibleChatModel) Generate(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	options := model.GetCommonOptions(&model.Options{}, opts...)
	modelName := ""
	if options.Model != nil {
		modelName = strings.TrimSpace(*options.Model)
	}
	if modelName == "" {
		return nil, errors.New("model is required")
	}
	temperature := float32(0.2)
	if options.Temperature != nil {
		temperature = *options.Temperature
	}

	type message struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	type reqBody struct {
		Model       string    `json:"model"`
		Messages    []message `json:"messages"`
		Temperature float32   `json:"temperature"`
	}
	messages := make([]message, 0, len(input))
	for _, msg := range input {
		if msg == nil {
			continue
		}
		messages = append(messages, message{
			Role:    string(msg.Role),
			Content: msg.Content,
		})
	}
	payload, err := json.Marshal(reqBody{
		Model:       modelName,
		Messages:    messages,
		Temperature: temperature,
	})
	if err != nil {
		return nil, err
	}

	var lastStatus int
	var lastBody string
	var lastURL string
	for _, u := range buildCandidateURLs(m.baseURL, "/chat/completions") {
		status, body, err := m.doJSON(ctx, u, payload)
		if err != nil {
			return nil, err
		}
		if status >= 200 && status < 300 {
			var out struct {
				Choices []struct {
					Message struct {
						Content string `json:"content"`
					} `json:"message"`
				} `json:"choices"`
			}
			if err := json.Unmarshal([]byte(body), &out); err != nil {
				return nil, err
			}
			if len(out.Choices) == 0 {
				return nil, errors.New("chat response has no choices")
			}
			return schema.AssistantMessage(strings.TrimSpace(out.Choices[0].Message.Content), nil), nil
		}
		lastStatus, lastBody, lastURL = status, body, u
		if status == http.StatusNotFound {
			continue
		}
		return nil, fmt.Errorf("chat http %d url=%s: %s", status, u, body)
	}
	return nil, fmt.Errorf("chat http %d url=%s: %s", lastStatus, lastURL, lastBody)
}

func (m *openAICompatibleChatModel) Stream(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	msg, err := m.Generate(ctx, input, opts...)
	if err != nil {
		return nil, err
	}
	sr, sw := schema.Pipe[*schema.Message](1)
	go func() {
		defer sw.Close()
		sw.Send(msg, nil)
	}()
	return sr, nil
}

func (m *openAICompatibleChatModel) doJSON(ctx context.Context, url string, payload []byte) (int, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return 0, "", err
	}
	req.Header.Set("Authorization", "Bearer "+m.apiKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := m.httpClient.Do(req)
	if err != nil {
		return 0, "", err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b), nil
}

func buildCandidateURLs(baseURL string, path string) []string {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
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
