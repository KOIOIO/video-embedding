# Eino Migration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Introduce Eino-backed Chat and Embedding adapters for `video-service/` while preserving the current ASR, worker, recommendation, and fallback behavior.

**Architecture:** Keep Eino behind project-owned interfaces so worker/application code does not depend directly on Eino types. First create a provider selection seam and composition client, then wire Eino Chat/Embedding adapters into HTTP recommendation and vector worker paths, and finally migrate the small title-rewrite workflow as a low-risk Chain/Graph pilot.

**Tech Stack:** Go + CloudWeGo Eino/eino-ext + Gin HTTP service + Redis Stream worker + GORM + pgvector + Go test

**Git constraint:** Do not run `git add`, `git commit`, `git checkout`, `git reset`, or other git-mutating commands unless the user explicitly re-authorizes git operations.

---

## Context

- Active backend service: `video-service/`.
- Design spec: `legacy-video/docs/superpowers/specs/2026-06-14-eino-migration-design.md`.
- Current hand-written model client: `video-service/internal/worker/vectorworker/client_openai.go`.
- Current HTTP recommendation embedder: `video-service/internal/infrastructure/embedding/client.go`.
- Current vector worker setup: `video-service/internal/worker/vectorworker/app.go`.
- Current HTTP app setup: `video-service/internal/http/app/app.go`.
- Current LLM segmentation flow: `video-service/internal/worker/vectorworker/hierarchical_stage_processing.go`.
- Current refine ASR/title rewrite/embedding flow: `video-service/internal/worker/vectorworker/tasks/asr.go`.

The plan intentionally leaves DashScope WebSocket ASR, FFmpeg, Redis Stream stage queues, GORM repositories, and HTTP routes in their current architecture.

## File Structure

**Create:**

- `video-service/internal/infrastructure/ai/provider.go`
  - Owns provider selection helpers and small AI interfaces shared by app/worker wiring.
- `video-service/internal/infrastructure/ai/eino/chat.go`
  - Eino-backed Chat adapter, exposed through project-owned methods.
- `video-service/internal/infrastructure/ai/eino/chat_test.go`
  - Chat adapter unit tests using an injected fake generator.
- `video-service/internal/infrastructure/ai/eino/embedding.go`
  - Eino-backed single-text and batch embedding adapters.
- `video-service/internal/infrastructure/ai/eino/embedding_test.go`
  - Embedding adapter unit tests using an injected fake embedder.
- `video-service/internal/worker/vectorworker/ai_client.go`
  - Vector worker AI client interfaces and composition wrapper.
- `video-service/internal/worker/vectorworker/title_rewrite.go`
  - Small title rewrite workflow wrapper.
- `video-service/internal/worker/vectorworker/title_rewrite_test.go`
  - Title rewrite workflow tests using a fake chat client.

**Modify:**

- `video-service/go.mod`
  - Add Eino dependencies after verifying exact versions and import paths.
- `video-service/internal/config/types.go`
  - Add `AI.Provider`.
- `video-service/internal/config/defaults.go`
  - Add `AIProvider(cfg)` helper with safe default `legacy`.
- `video-service/internal/config/loader_test.go`
  - Cover default and explicit AI provider behavior.
- `video-service/configs/video.yml`
  - Add explicit `AI.Provider: "legacy"`.
- `video-service/configs/video_prod.yml`
  - Add explicit `AI.Provider: "legacy"`.
- `video-service/internal/http/app/app.go`
  - Select HTTP recommendation embedder from provider config.
- `video-service/internal/worker/vectorworker/app.go`
  - Select vector Chat/Embedding adapters from provider config while preserving ASR.
- `video-service/internal/worker/vectorworker/stage_coarse.go`
  - Depend on a vector AI interface instead of concrete `*openAICompatClient`.
- `video-service/internal/worker/vectorworker/stage_refine.go`
  - Depend on a vector AI interface instead of concrete `*openAICompatClient`.
- `video-service/internal/worker/vectorworker/hierarchical_stage_processing.go`
  - Depend on a vector AI interface instead of concrete `*openAICompatClient`.
- `video-service/internal/worker/vectorworker/task.go`
  - Keep legacy mode compiling by accepting the same vector AI interface.
- `video-service/internal/worker/vectorworker/tasks/asr.go`
  - Keep its existing minimal AI interface; only call through title rewrite wrapper when present.

---

### Task 1: Calibrate Eino Dependency and API

**Files:**
- Modify: `video-service/go.mod`

- [ ] **Step 1: Inspect official Eino packages before writing adapter code**

Run from `video-service/`:

```bash
go list -m -versions github.com/cloudwego/eino
go list -m -versions github.com/cloudwego/eino-ext
```

Expected: both commands print available versions. Pick the latest stable versions returned by the module proxy.

- [ ] **Step 2: Add Eino modules**

Run from `video-service/`:

```bash
go get github.com/cloudwego/eino@latest
go get github.com/cloudwego/eino-ext@latest
```

Expected: `go.mod` and `go.sum` are updated with CloudWeGo Eino modules.

- [ ] **Step 3: Confirm concrete provider package names**

Run from `video-service/`:

```bash
go doc github.com/cloudwego/eino/components/model
go doc github.com/cloudwego/eino/components/embedding
go doc github.com/cloudwego/eino/schema
go doc github.com/cloudwego/eino-ext/components/model/openai
go doc github.com/cloudwego/eino-ext/components/embedding/openai
```

Expected: docs show the ChatModel, Embedding, schema message helpers, and OpenAI-compatible extension constructors. Use the names from this output in Task 5. If the extension package path differs, use the package path shown by `go list github.com/cloudwego/eino-ext/...`.

- [ ] **Step 4: Verify the module still resolves**

Run:

```bash
go test ./internal/config -run TestRuntimeConfigDefaultsPreserveExistingValues -v
```

Expected: PASS.

---

### Task 2: Add Provider Configuration

**Files:**
- Modify: `video-service/internal/config/types.go`
- Modify: `video-service/internal/config/defaults.go`
- Modify: `video-service/internal/config/loader_test.go`
- Modify: `video-service/configs/video.yml`
- Modify: `video-service/configs/video_prod.yml`

- [ ] **Step 1: Write the failing provider default tests**

Append these assertions to `TestRuntimeConfigDefaultsPreserveExistingValues` in `internal/config/loader_test.go`:

```go
	if got := AIProvider(cfg); got != "legacy" {
		t.Fatalf("AIProvider() = %q, want %q", got, "legacy")
	}
```

Append this assertion to `TestRuntimeConfigUsesExplicitValues` after `EmbeddingDim` is checked:

```go
	if got := AIProvider(cfg); got != "eino" {
		t.Fatalf("AIProvider() = %q, want %q", got, "eino")
	}
```

Set the explicit value in that test's config literal:

```go
		AI: AIConfig{
			EmbeddingDim: 768,
			Provider:     "eino",
		},
```

- [ ] **Step 2: Run the focused config test to verify it fails**

Run:

```bash
go test ./internal/config -run "TestRuntimeConfigDefaultsPreserveExistingValues|TestRuntimeConfigUsesExplicitValues" -v
```

Expected: FAIL with undefined `AIProvider` or missing `Provider`.

- [ ] **Step 3: Add the provider config field**

Modify `internal/config/types.go`:

```go
type AIConfig struct {
	EmbeddingDim int    `yaml:"EmbeddingDim"`
	Provider     string `yaml:"Provider"`
}
```

- [ ] **Step 4: Add the default helper**

Modify `internal/config/defaults.go`:

```go
const (
	defaultHTTPAddr                    = ":8081"
	// keep existing constants unchanged
	defaultAIProvider                  = "legacy"
)
```

Add the helper near `EmbeddingDim`:

```go
func AIProvider(cfg Config) string {
	provider := strings.ToLower(strings.TrimSpace(cfg.AI.Provider))
	if provider == "" {
		return defaultAIProvider
	}
	return provider
}
```

Keep the existing `EmbeddingDim` function unchanged.

- [ ] **Step 5: Add explicit YAML defaults**

In both `configs/video.yml` and `configs/video_prod.yml`, change the `AI` section to include:

```yaml
AI:
  Provider: "legacy"
  EmbeddingDim: 1536
```

If the section already has `EmbeddingDim`, preserve that value and add `Provider` above it.

- [ ] **Step 6: Run config tests**

Run:

```bash
go test ./internal/config -v
```

Expected: PASS.

---

### Task 3: Add Shared AI Provider Interfaces

**Files:**
- Create: `video-service/internal/infrastructure/ai/provider.go`

- [ ] **Step 1: Create provider interfaces and constants**

Create `internal/infrastructure/ai/provider.go`:

```go
package ai

import (
	"context"
	"strings"
)

const (
	ProviderLegacy = "legacy"
	ProviderEino   = "eino"
)

type ChatClient interface {
	ChatCompletionsWithTimeout(ctx context.Context, model string, prompt string, timeoutMinutes int) (string, error)
}

type BatchEmbedder interface {
	Embed(ctx context.Context, texts []string) ([][]float32, error)
}

type TextEmbedder interface {
	Embed(ctx context.Context, text string) ([]float32, error)
}

func NormalizeProvider(provider string) string {
	provider = strings.ToLower(strings.TrimSpace(provider))
	switch provider {
	case ProviderEino:
		return ProviderEino
	default:
		return ProviderLegacy
	}
}
```

- [ ] **Step 2: Add provider normalization tests**

Create `internal/infrastructure/ai/provider_test.go`:

```go
package ai

import "testing"

func TestNormalizeProvider(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "empty", in: "", want: ProviderLegacy},
		{name: "legacy", in: "legacy", want: ProviderLegacy},
		{name: "eino", in: "eino", want: ProviderEino},
		{name: "case and spaces", in: " EINO ", want: ProviderEino},
		{name: "unknown", in: "experimental", want: ProviderLegacy},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NormalizeProvider(tt.in); got != tt.want {
				t.Fatalf("NormalizeProvider(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
```

- [ ] **Step 3: Run provider tests**

Run:

```bash
go test ./internal/infrastructure/ai -run TestNormalizeProvider -v
```

Expected: PASS.

---

### Task 4: Introduce Vector Worker AI Composition

**Files:**
- Create: `video-service/internal/worker/vectorworker/ai_client.go`
- Modify: `video-service/internal/worker/vectorworker/stage_coarse.go`
- Modify: `video-service/internal/worker/vectorworker/stage_refine.go`
- Modify: `video-service/internal/worker/vectorworker/hierarchical_stage_processing.go`
- Modify: `video-service/internal/worker/vectorworker/task.go`

- [ ] **Step 1: Add the vector AI interfaces and composition client**

Create `internal/worker/vectorworker/ai_client.go`:

```go
package vectorworker

import "context"

type vectorTranscriber interface {
	Transcribe(ctx context.Context, audioPath string) (string, error)
}

type vectorChatClient interface {
	ChatCompletionsWithTimeout(ctx context.Context, model string, prompt string, timeoutMinutes int) (string, error)
}

type vectorBatchEmbedder interface {
	Embed(ctx context.Context, texts []string) ([][]float32, error)
}

type vectorAIClient interface {
	vectorTranscriber
	vectorChatClient
	vectorBatchEmbedder
}

type composedVectorAIClient struct {
	transcriber vectorTranscriber
	chat        vectorChatClient
	embedder    vectorBatchEmbedder
}

func newComposedVectorAIClient(transcriber vectorTranscriber, chat vectorChatClient, embedder vectorBatchEmbedder) *composedVectorAIClient {
	return &composedVectorAIClient{
		transcriber: transcriber,
		chat:        chat,
		embedder:    embedder,
	}
}

func (c *composedVectorAIClient) Transcribe(ctx context.Context, audioPath string) (string, error) {
	return c.transcriber.Transcribe(ctx, audioPath)
}

func (c *composedVectorAIClient) ChatCompletionsWithTimeout(ctx context.Context, model string, prompt string, timeoutMinutes int) (string, error) {
	return c.chat.ChatCompletionsWithTimeout(ctx, model, prompt, timeoutMinutes)
}

func (c *composedVectorAIClient) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	return c.embedder.Embed(ctx, texts)
}
```

- [ ] **Step 2: Replace concrete client type in stage structs**

In `stage_coarse.go`, change:

```go
	client        *openAICompatClient
```

to:

```go
	client        vectorAIClient
```

Change the constructor argument:

```go
func newProductionCoarseStageProcessor(store *objectstorage.RustFS, ff *transcode.FFmpegTranscoder, client vectorAIClient, tmpRoot string, asrWorkers int, coarseWorkers int, stageRecorder *vectorStageRecorder) *productionCoarseStageProcessor {
```

In `stage_refine.go`, change:

```go
	client              *openAICompatClient
```

to:

```go
	client              vectorAIClient
```

Change the constructor argument:

```go
func newProductionRefineStageProcessor(db *gorm.DB, store *objectstorage.RustFS, ff *transcode.FFmpegTranscoder, client vectorAIClient, tmpRoot string, coarseSegmentSec int, refineMinSegmentSec int, refineMaxSegmentSec int, llmModel string, llmTimeoutMinutes int, asrWorkers int, embedBatch int, embeddingDim int, tailCfg tasks.TailAlignmentConfig, stageRecorder *vectorStageRecorder) *productionRefineStageProcessor {
```

- [ ] **Step 3: Replace concrete client type in hierarchical inputs**

In `hierarchical_stage_processing.go`, change:

```go
	Client        *openAICompatClient
```

to:

```go
	Client        vectorAIClient
```

and change:

```go
	Client              *openAICompatClient
```

to:

```go
	Client              vectorAIClient
```

- [ ] **Step 4: Replace concrete client type in legacy task signature**

In `task.go`, change the `handleVectorizeTask` client parameter from:

```go
	client *openAICompatClient,
```

to:

```go
	client vectorAIClient,
```

- [ ] **Step 5: Run vectorworker tests**

Run:

```bash
go test ./internal/worker/vectorworker -run "TestBuildASRWSModelCandidatesPrefersPrimaryAndDeduplicates|TestTranscribeWSRotatesOnQuotaExhaustion|TestTranscribeWSStopsOnNonQuotaError" -v
```

Expected: PASS.

---

### Task 5: Add Eino Chat Adapter

**Files:**
- Create: `video-service/internal/infrastructure/ai/eino/chat.go`
- Create: `video-service/internal/infrastructure/ai/eino/chat_test.go`

- [ ] **Step 1: Write adapter tests using an injected generator**

Create `internal/infrastructure/ai/eino/chat_test.go`:

```go
package eino

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestChatClientValidatesModelAndPrompt(t *testing.T) {
	client := NewChatClientWithGenerator(func(context.Context, ChatRequest) (string, error) {
		return "{}", nil
	})
	if _, err := client.ChatCompletionsWithTimeout(context.Background(), "", "prompt", 1); err == nil {
		t.Fatal("expected missing model to fail")
	}
	if _, err := client.ChatCompletionsWithTimeout(context.Background(), "qwen", "", 1); err == nil {
		t.Fatal("expected missing prompt to fail")
	}
}

func TestChatClientPassesRequestToGenerator(t *testing.T) {
	var got ChatRequest
	client := NewChatClientWithGenerator(func(ctx context.Context, req ChatRequest) (string, error) {
		got = req
		return `{"segments":[]}`, nil
	})
	out, err := client.ChatCompletionsWithTimeout(context.Background(), "qwen-plus", "hello", 2)
	if err != nil {
		t.Fatalf("ChatCompletionsWithTimeout error = %v", err)
	}
	if out != `{"segments":[]}` {
		t.Fatalf("output = %q", out)
	}
	if got.Model != "qwen-plus" || got.Prompt != "hello" || got.Temperature != 0.2 {
		t.Fatalf("request = %#v", got)
	}
}

func TestChatClientPropagatesGeneratorError(t *testing.T) {
	want := errors.New("provider down")
	client := NewChatClientWithGenerator(func(context.Context, ChatRequest) (string, error) {
		return "", want
	})
	_, err := client.ChatCompletionsWithTimeout(context.Background(), "qwen", "prompt", 1)
	if !errors.Is(err, want) {
		t.Fatalf("error = %v, want %v", err, want)
	}
}

func TestChatClientTimeout(t *testing.T) {
	client := NewChatClientWithGenerator(func(ctx context.Context, req ChatRequest) (string, error) {
		<-ctx.Done()
		return "", ctx.Err()
	})
	start := time.Now()
	_, err := client.ChatCompletionsWithTimeout(context.Background(), "qwen", "prompt", -1)
	if err == nil {
		t.Fatal("expected timeout")
	}
	if time.Since(start) > 2*time.Second {
		t.Fatal("timeout test took too long")
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run:

```bash
go test ./internal/infrastructure/ai/eino -run TestChatClient -v
```

Expected: FAIL with missing package or undefined chat adapter types.

- [ ] **Step 3: Implement the testable chat wrapper**

Create `internal/infrastructure/ai/eino/chat.go`:

```go
package eino

import (
	"context"
	"errors"
	"strings"
	"time"
)

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
```

- [ ] **Step 4: Implement the real Eino constructor**

After Task 1 confirms exact Eino API names, add a constructor to `chat.go` that uses Eino's OpenAI-compatible ChatModel. The constructor must preserve the request semantics from the legacy client:

```go
type ChatConfig struct {
	BaseURL string
	APIKey  string
}

func NewChatClient(ctx context.Context, cfg ChatConfig) (*ChatClient, error) {
	baseURL := strings.TrimSpace(cfg.BaseURL)
	apiKey := strings.TrimSpace(cfg.APIKey)
	if baseURL == "" || apiKey == "" {
		return nil, errors.New("chat base-url/api-key is required")
	}

	modelFactory, err := newOpenAICompatibleChatModelFactory(ctx, baseURL, apiKey)
	if err != nil {
		return nil, err
	}

	return NewChatClientWithGenerator(func(runCtx context.Context, req ChatRequest) (string, error) {
		return modelFactory.Generate(runCtx, req)
	}), nil
}
```

Implement `newOpenAICompatibleChatModelFactory` in the same file using the constructor and message types confirmed by `go doc` in Task 1. It must send:

```text
system: You are a helpful assistant that only outputs valid JSON.
user:   req.Prompt
model:  req.Model
temperature: 0.2
```

- [ ] **Step 5: Run chat adapter tests**

Run:

```bash
go test ./internal/infrastructure/ai/eino -run TestChatClient -v
```

Expected: PASS.

---

### Task 6: Add Eino Embedding Adapter

**Files:**
- Create: `video-service/internal/infrastructure/ai/eino/embedding.go`
- Create: `video-service/internal/infrastructure/ai/eino/embedding_test.go`

- [ ] **Step 1: Write adapter tests using an injected batch embed function**

Create `internal/infrastructure/ai/eino/embedding_test.go`:

```go
package eino

import (
	"context"
	"errors"
	"testing"
)

func TestBatchEmbeddingClientReturnsVectorsInOrder(t *testing.T) {
	client := NewEmbeddingClientWithBatchFunc(func(ctx context.Context, texts []string) ([][]float32, error) {
		return [][]float32{{1, 2}, {3, 4}}, nil
	})
	vecs, err := client.Embed(context.Background(), []string{"a", "b"})
	if err != nil {
		t.Fatalf("Embed error = %v", err)
	}
	if len(vecs) != 2 || vecs[0][0] != 1 || vecs[1][0] != 3 {
		t.Fatalf("vectors = %#v", vecs)
	}
}

func TestBatchEmbeddingClientRejectsSizeMismatch(t *testing.T) {
	client := NewEmbeddingClientWithBatchFunc(func(ctx context.Context, texts []string) ([][]float32, error) {
		return [][]float32{{1, 2}}, nil
	})
	_, err := client.Embed(context.Background(), []string{"a", "b"})
	if err == nil {
		t.Fatal("expected size mismatch")
	}
}

func TestTextEmbeddingClientUsesBatchEmbedding(t *testing.T) {
	client := NewEmbeddingClientWithBatchFunc(func(ctx context.Context, texts []string) ([][]float32, error) {
		if len(texts) != 1 || texts[0] != "hello" {
			t.Fatalf("texts = %#v", texts)
		}
		return [][]float32{{0.5, 0.25}}, nil
	})
	vec, err := client.EmbedText(context.Background(), "hello")
	if err != nil {
		t.Fatalf("EmbedText error = %v", err)
	}
	if len(vec) != 2 || vec[0] != 0.5 {
		t.Fatalf("vector = %#v", vec)
	}
}

func TestEmbeddingClientPropagatesProviderError(t *testing.T) {
	want := errors.New("embedding unavailable")
	client := NewEmbeddingClientWithBatchFunc(func(ctx context.Context, texts []string) ([][]float32, error) {
		return nil, want
	})
	_, err := client.Embed(context.Background(), []string{"a"})
	if !errors.Is(err, want) {
		t.Fatalf("error = %v, want %v", err, want)
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run:

```bash
go test ./internal/infrastructure/ai/eino -run "TestBatchEmbeddingClient|TestTextEmbeddingClient|TestEmbeddingClient" -v
```

Expected: FAIL with undefined embedding adapter types.

- [ ] **Step 3: Implement the testable embedding wrapper**

Create `internal/infrastructure/ai/eino/embedding.go`:

```go
package eino

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

type BatchEmbedFunc func(context.Context, []string) ([][]float32, error)

type EmbeddingClient struct {
	embed BatchEmbedFunc
}

func NewEmbeddingClientWithBatchFunc(embed BatchEmbedFunc) *EmbeddingClient {
	return &EmbeddingClient{embed: embed}
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
```

- [ ] **Step 4: Implement the real Eino constructor**

After Task 1 confirms exact Eino embedding API names, add:

```go
type EmbeddingConfig struct {
	BaseURL string
	APIKey  string
	Model   string
}

func NewEmbeddingClient(ctx context.Context, cfg EmbeddingConfig) (*EmbeddingClient, error) {
	baseURL := strings.TrimSpace(cfg.BaseURL)
	apiKey := strings.TrimSpace(cfg.APIKey)
	model := strings.TrimSpace(cfg.Model)
	if model == "" {
		model = "text-embedding-v4"
	}
	if baseURL == "" || apiKey == "" {
		return nil, errors.New("embedding base-url/api-key is required")
	}

	embedder, err := newOpenAICompatibleEmbeddingModel(ctx, baseURL, apiKey, model)
	if err != nil {
		return nil, err
	}
	return NewEmbeddingClientWithBatchFunc(embedder.Embed), nil
}
```

Implement `newOpenAICompatibleEmbeddingModel` with the Eino extension constructor confirmed by Task 1. Convert provider output to `[][]float32` in this adapter, not in worker/application code.

- [ ] **Step 5: Run embedding adapter tests**

Run:

```bash
go test ./internal/infrastructure/ai/eino -run "TestBatchEmbeddingClient|TestTextEmbeddingClient|TestEmbeddingClient" -v
```

Expected: PASS.

---

### Task 7: Wire HTTP Recommendation Embedder Provider Selection

**Files:**
- Modify: `video-service/internal/http/app/app.go`
- Test: existing `video-service/internal/http/app/app_test.go`

- [ ] **Step 1: Add a provider factory helper in HTTP app**

In `internal/http/app/app.go`, add imports:

```go
	einoai "legacy-video/http/internal/infrastructure/ai/eino"
```

Add a helper near `New`:

```go
func newRecommendationEmbedder(ctx context.Context, cfg config.Config) aiinfra.Embedder {
	provider := aiinfra.NormalizeProvider(config.AIProvider(cfg))
	if provider == aiinfra.ProviderEino {
		client, err := einoai.NewEmbeddingClient(ctx, einoai.EmbeddingConfig{
			BaseURL: cfg.Embedding.BaseURL,
			APIKey:  firstNonEmpty(os.Getenv("DASHSCOPE_API_KEY"), os.Getenv("OPENAI_API_KEY"), os.Getenv("EMBEDDING_API_KEY"), cfg.Embedding.APIKey),
			Model:   cfg.Embedding.Options.Model,
		})
		if err == nil {
			return textEmbedderAdapter{client: client}
		}
		zap.L().Warn("eino_embedding_init_failed_fallback_legacy", zap.Error(err))
	}
	return embedding.NewClient(cfg.Embedding)
}

type textEmbedderAdapter struct {
	client interface {
		EmbedText(context.Context, string) ([]float32, error)
	}
}

func (a textEmbedderAdapter) Embed(ctx context.Context, text string) ([]float32, error) {
	return a.client.EmbedText(ctx, text)
}
```

If `firstNonEmpty` is not available in this package, add a local helper:

```go
func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if v := strings.TrimSpace(value); v != "" {
			return v
		}
	}
	return ""
}
```

- [ ] **Step 2: Use the helper in app construction**

Replace:

```go
	primaryEmbedder := embedding.NewClient(cfg.Embedding)
```

with:

```go
	primaryEmbedder := newRecommendationEmbedder(ctx, cfg)
```

Keep:

```go
	fallbackEmbedder := aiinfra.NewFallbackEmbedder(primaryEmbedder, aiinfra.NewLocalEmbedder(config.EmbeddingDim(cfg)))
```

unchanged.

- [ ] **Step 3: Run HTTP app tests**

Run:

```bash
go test ./internal/http/app -v
```

Expected: PASS.

---

### Task 8: Wire Vector Worker Provider Selection

**Files:**
- Modify: `video-service/internal/worker/vectorworker/app.go`
- Test: `video-service/internal/worker/vectorworker/client_openai_test.go`

- [ ] **Step 1: Add provider-aware vector client factory**

In `internal/worker/vectorworker/app.go`, add imports:

```go
	aiinfra "legacy-video/http/internal/infrastructure/ai"
	einoai "legacy-video/http/internal/infrastructure/ai/eino"
```

Add a helper near `Register`:

```go
func newVectorAIClient(ctx context.Context, cfg config.Config) (vectorAIClient, error) {
	legacyClient, err := newOpenAICompatClient(cfg)
	if err != nil {
		return nil, err
	}
	if aiinfra.NormalizeProvider(config.AIProvider(cfg)) != aiinfra.ProviderEino {
		return legacyClient, nil
	}

	apiKey := firstNonEmpty(
		os.Getenv("DASHSCOPE_API_KEY"),
		os.Getenv("OPENAI_API_KEY"),
		os.Getenv("ASR_API_KEY"),
		cfg.ASR.APIKey,
		cfg.Embedding.APIKey,
	)
	chatClient, chatErr := einoai.NewChatClient(ctx, einoai.ChatConfig{
		BaseURL: legacyClient.compatBaseURL,
		APIKey:  apiKey,
	})
	embedClient, embedErr := einoai.NewEmbeddingClient(ctx, einoai.EmbeddingConfig{
		BaseURL: legacyClient.compatBaseURL,
		APIKey:  apiKey,
		Model:   legacyClient.embedModel,
	})
	if chatErr != nil || embedErr != nil {
		if chatErr != nil {
			zap.L().Warn("eino_chat_init_failed_fallback_legacy", zap.Error(chatErr))
		}
		if embedErr != nil {
			zap.L().Warn("eino_embedding_init_failed_fallback_legacy", zap.Error(embedErr))
		}
		return legacyClient, nil
	}
	return newComposedVectorAIClient(legacyClient, chatClient, embedClient), nil
}
```

- [ ] **Step 2: Use the factory in Register**

Replace:

```go
	client, err := newOpenAICompatClient(cfg)
```

with:

```go
	client, err := newVectorAIClient(app.Context(), cfg)
```

Keep the existing missing API key handling and fatal behavior unchanged.

- [ ] **Step 3: Add a unit test for legacy default selection**

Append to `client_openai_test.go`:

```go
func TestNewVectorAIClientDefaultsToLegacy(t *testing.T) {
	t.Setenv("DASHSCOPE_API_KEY", "test-key")
	cfg := config.Config{}
	client, err := newVectorAIClient(context.Background(), cfg)
	if err != nil {
		t.Fatalf("newVectorAIClient error = %v", err)
	}
	if _, ok := client.(*openAICompatClient); !ok {
		t.Fatalf("client type = %T, want *openAICompatClient", client)
	}
}
```

- [ ] **Step 4: Run vectorworker tests**

Run:

```bash
go test ./internal/worker/vectorworker -run "TestNewVectorAIClientDefaultsToLegacy|TestNewOpenAICompatClientReturnsMissingAPIKeySentinel|TestTranscribeWSRotatesOnQuotaExhaustion" -v
```

Expected: PASS.

---

### Task 9: Add Title Rewrite Workflow Wrapper

**Files:**
- Create: `video-service/internal/worker/vectorworker/title_rewrite.go`
- Create: `video-service/internal/worker/vectorworker/title_rewrite_test.go`
- Modify: `video-service/internal/worker/vectorworker/tasks/asr.go`

- [ ] **Step 1: Add title rewrite wrapper tests**

Create `internal/worker/vectorworker/title_rewrite_test.go`:

```go
package vectorworker

import (
	"context"
	"strings"
	"testing"
)

type fakeTitleChat struct {
	gotModel   string
	gotPrompt  string
	gotTimeout int
	out        string
	err        error
}

func (f *fakeTitleChat) ChatCompletionsWithTimeout(ctx context.Context, model string, prompt string, timeoutMinutes int) (string, error) {
	f.gotModel = model
	f.gotPrompt = prompt
	f.gotTimeout = timeoutMinutes
	return f.out, f.err
}

func TestRewriteSegmentTitleUsesSummaryPrompt(t *testing.T) {
	chat := &fakeTitleChat{out: " 函数单调性 "}
	got, err := rewriteSegmentTitle(context.Background(), chat, "qwen-plus", 5, "这里讲函数单调性的定义。")
	if err != nil {
		t.Fatalf("rewriteSegmentTitle error = %v", err)
	}
	if got != "函数单调性" {
		t.Fatalf("title = %q", got)
	}
	if chat.gotModel != "qwen-plus" || chat.gotTimeout != 5 {
		t.Fatalf("model/timeout = %q/%d", chat.gotModel, chat.gotTimeout)
	}
	if !strings.Contains(chat.gotPrompt, "只根据提供的正文生成标题") {
		t.Fatalf("prompt = %q", chat.gotPrompt)
	}
}
```

- [ ] **Step 2: Implement the wrapper**

Create `internal/worker/vectorworker/title_rewrite.go`:

```go
package vectorworker

import (
	"context"
	"strings"

	"legacy-video/http/internal/worker/vectorworker/tasks"
)

func rewriteSegmentTitle(ctx context.Context, chat vectorChatClient, model string, timeoutMinutes int, text string) (string, error) {
	prompt := tasks.BuildSummaryRewritePrompt(text)
	out, err := chat.ChatCompletionsWithTimeout(ctx, model, prompt, timeoutMinutes)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}
```

- [ ] **Step 3: Keep current task-level behavior unchanged**

Do not directly import `vectorworker` from `tasks/asr.go`, because that would create a package cycle. Instead, leave `tasks/asr.go` using its current local closure:

```go
rewriteSummary = func(runCtx context.Context, text string) (string, error) {
	prompt := BuildSummaryRewritePrompt(text)
	return client.ChatCompletionsWithTimeout(runCtx, hints.LLMModel, prompt, hints.LLMTimeoutMinutes)
}
```

The wrapper is a pilot seam for moving title rewrite workflow up into `vectorworker` orchestration in a later task. It must not change runtime behavior yet.

- [ ] **Step 4: Run title rewrite tests**

Run:

```bash
go test ./internal/worker/vectorworker -run TestRewriteSegmentTitleUsesSummaryPrompt -v
```

Expected: PASS.

---

### Task 10: Verify Focused Packages

**Files:**
- No code changes.

- [ ] **Step 1: Run focused package tests**

Run from `video-service/`:

```bash
go test ./internal/config ./internal/infrastructure/ai ./internal/infrastructure/ai/eino ./internal/infrastructure/embedding ./internal/worker/vectorworker ./internal/worker/vectorworker/tasks ./internal/application/videoapp/recommendation ./internal/http/app
```

Expected: PASS.

- [ ] **Step 2: Run full service tests if focused tests pass**

Run:

```bash
go test ./...
```

Expected: PASS. If infrastructure-dependent tests fail because Postgres, Redis, or object storage are unavailable, record the exact failing package and error before making further changes.

---

### Task 11: Update Migration Notes After Implementation

**Files:**
- Modify: `legacy-video/docs/superpowers/specs/2026-06-14-eino-migration-design.md`

- [ ] **Step 1: Add implementation outcome notes**

Append a short section to the design doc:

```markdown
## Implementation Notes

- `AI.Provider` controls whether Eino-backed adapters are selected. The default is `legacy`.
- Eino Chat and Embedding adapters are isolated under `video-service/internal/infrastructure/ai/eino/`.
- Vector worker ASR remains on the existing DashScope WebSocket implementation.
- HTTP recommendation embedding remains protected by `FallbackEmbedder`.
```

- [ ] **Step 2: Re-run focused docs grep**

Run:

```bash
rg -n "TBD|TODO|待定|占位" ../legacy-video/docs/superpowers/specs/2026-06-14-eino-migration-design.md ../legacy-video/docs/superpowers/plans/2026-06-14-eino-migration.md
```

Expected: no output.

---

## Self-Review Checklist

- Spec coverage:
  - Chat replacement: Tasks 5 and 8.
  - Embedding replacement: Tasks 6, 7, and 8.
  - Provider config and rollback default: Task 2.
  - Business-layer interface isolation: Tasks 3 and 4.
  - ASR non-replacement: Task 8 preserves `openAICompatClient` as transcriber.
  - Title rewrite workflow pilot: Task 9.
  - Retriever/RAG phase: intentionally not implemented; the spec says it only starts when product requirements exist.
- Placeholder scan:
  - The plan contains no implementation placeholders. The only Eino API calibration step is explicit because the exact constructor names must be read from installed module docs before writing provider-specific code.
- Type consistency:
  - HTTP embedder uses `EmbedText` through a local adapter to satisfy existing `aiinfra.Embedder`.
  - Vector worker uses `vectorAIClient` for `Transcribe`, `ChatCompletionsWithTimeout`, and batch `Embed`.
  - Existing `tasks.openAICompatClient` interface remains compatible with `vectorAIClient`.
