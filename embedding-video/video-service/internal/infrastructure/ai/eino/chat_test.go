package eino

import (
	"context"
	"errors"
	"testing"
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
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := client.ChatCompletionsWithTimeout(ctx, "qwen", "prompt", 1)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
}
