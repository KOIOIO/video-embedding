package vectorworker

import (
	"context"
	"errors"
	"testing"

	"nlp-video-analysis/internal/config"
)

func TestBuildASRWSModelCandidatesPrefersPrimaryAndDeduplicates(t *testing.T) {
	got := buildASRWSModelCandidates("fun-asr-realtime-2026-02-28", []string{
		"fun-asr-realtime",
		"fun-asr-realtime-2026-02-28",
		"fun-asr-realtime-2025-11-07",
		"fun-asr-realtime-2025-09-15",
	})
	want := []string{
		"fun-asr-realtime-2026-02-28",
		"fun-asr-realtime",
		"fun-asr-realtime-2025-11-07",
		"fun-asr-realtime-2025-09-15",
	}
	if len(got) != len(want) {
		t.Fatalf("len(buildASRWSModelCandidates()) = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("candidate[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestNewOpenAICompatClientReturnsMissingAPIKeySentinel(t *testing.T) {
	t.Setenv("DASHSCOPE_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("ASR_API_KEY", "")

	_, err := newOpenAICompatClient(config.Config{})
	if !errors.Is(err, errMissingOpenAICompatAPIKey) {
		t.Fatalf("newOpenAICompatClient() error = %v, want errMissingOpenAICompatAPIKey", err)
	}
}

func TestIsASRQuotaExhaustedRespDetectsQuotaErrors(t *testing.T) {
	body := `{"message":"insufficient balance for current model quota"}`
	if !isASRQuotaExhaustedResp(429, body) {
		t.Fatal("expected quota error to be detected")
	}
}

func TestIsASRQuotaExhaustedRespDetectsFreeTierOnly403(t *testing.T) {
	body := `AllocationQuota.FreeTierOnly`
	if !isASRQuotaExhaustedResp(403, body) {
		t.Fatal("expected free-tier-only error to be detected")
	}
}

func TestIsASRQuotaExhaustedRespDetectsFreeTierExhaustedMessage(t *testing.T) {
	body := `The free tier of the model has been exhausted.`
	if !isASRQuotaExhaustedResp(403, body) {
		t.Fatal("expected free-tier exhausted message to be detected")
	}
}

func TestIsASRQuotaExhaustedRespIgnoresNonQuotaErrors(t *testing.T) {
	body := `{"message":"model not found"}`
	if isASRQuotaExhaustedResp(404, body) {
		t.Fatal("did not expect model-not-found to be treated as quota exhausted")
	}
}

func TestTranscribeWSRotatesOnQuotaExhaustion(t *testing.T) {
	orig := dashscopeWSRecognizeWavFunc
	defer func() { dashscopeWSRecognizeWavFunc = orig }()

	var gotURL string
	var gotModels []string
	dashscopeWSRecognizeWavFunc = func(ctx context.Context, wsURL string, apiKey string, model string, audioPath string) (string, error) {
		gotURL = wsURL
		gotModels = append(gotModels, model)
		if model == "fun-asr-realtime-2026-02-28" {
			return "", errors.New("AllocationQuota.FreeTierOnly")
		}
		if model == "fun-asr-realtime-2025-11-07" {
			return "转写成功", nil
		}
		return "", errors.New("unexpected model")
	}

	client := &openAICompatClient{
		apiKey:         "test-key",
		asrWSURL:       "wss://asr.example/ws",
		asrWSModel:     "fun-asr-realtime-2026-02-28",
		asrWSFallbacks: []string{"fun-asr-realtime-2025-11-07", "fun-asr-realtime-2025-09-15"},
	}

	text, err := client.transcribeViaWS(context.Background(), "/tmp/test.wav")
	if err != nil {
		t.Fatalf("transcribeViaWS() error = %v", err)
	}
	if text != "转写成功" {
		t.Fatalf("transcribeViaWS() text = %q", text)
	}
	if gotURL != "wss://asr.example/ws" {
		t.Fatalf("ws url = %q, want %q", gotURL, "wss://asr.example/ws")
	}
	wantModels := []string{"fun-asr-realtime-2026-02-28", "fun-asr-realtime-2025-11-07"}
	if len(gotModels) != len(wantModels) {
		t.Fatalf("models tried = %v, want %v", gotModels, wantModels)
	}
	for i := range wantModels {
		if gotModels[i] != wantModels[i] {
			t.Fatalf("models tried = %v, want %v", gotModels, wantModels)
		}
	}
}

func TestTranscribeWSStopsOnNonQuotaError(t *testing.T) {
	orig := dashscopeWSRecognizeWavFunc
	defer func() { dashscopeWSRecognizeWavFunc = orig }()

	var calls int
	dashscopeWSRecognizeWavFunc = func(ctx context.Context, wsURL string, apiKey string, model string, audioPath string) (string, error) {
		calls++
		return "", errors.New("permission denied")
	}

	client := &openAICompatClient{
		apiKey:         "test-key",
		asrWSModel:     "fun-asr-realtime-2026-02-28",
		asrWSFallbacks: []string{"fun-asr-realtime-2025-11-07"},
	}

	_, err := client.transcribeViaWS(context.Background(), "/tmp/test.wav")
	if err == nil {
		t.Fatal("expected transcribeViaWS() to fail")
	}
	if calls != 1 {
		t.Fatalf("calls = %d, want 1", calls)
	}
}
