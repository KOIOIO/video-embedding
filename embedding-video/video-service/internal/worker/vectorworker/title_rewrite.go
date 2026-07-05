package vectorworker

import (
	"context"
	"strings"

	"nlp-video-analysis/internal/worker/vectorworker/tasks"
)

func rewriteSegmentTitle(ctx context.Context, chat vectorChatClient, model string, timeoutMinutes int, text string) (string, error) {
	prompt := tasks.BuildSummaryRewritePrompt(text)
	out, err := chat.ChatCompletionsWithTimeout(ctx, model, prompt, timeoutMinutes)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}
