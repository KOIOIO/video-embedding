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
