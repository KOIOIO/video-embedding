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
