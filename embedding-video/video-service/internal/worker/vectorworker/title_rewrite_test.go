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
