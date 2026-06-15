package ai

import (
	"context"
	"errors"
	"testing"
)

type stubEmbedder struct {
	vec []float32
	err error
}

func (s stubEmbedder) Embed(context.Context, string) ([]float32, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.vec, nil
}

func TestFallbackEmbedder_UsesFallbackWhenPrimaryFails(t *testing.T) {
	e := NewFallbackEmbedder(stubEmbedder{err: errors.New("timeout")}, stubEmbedder{vec: []float32{1, 2, 3}})
	vec, err := e.Embed(context.Background(), "hello")
	if err != nil {
		t.Fatalf("Embed error = %v", err)
	}
	if len(vec) != 3 {
		t.Fatalf("expected fallback vector, got %#v", vec)
	}
}

func TestLocalEmbedderIsDeterministic(t *testing.T) {
	e := NewLocalEmbedder(8)
	first, err := e.Embed(context.Background(), "hello world")
	if err != nil {
		t.Fatalf("Embed error = %v", err)
	}
	second, err := e.Embed(context.Background(), "hello world")
	if err != nil {
		t.Fatalf("Embed error = %v", err)
	}
	if len(first) != 8 || len(second) != 8 {
		t.Fatalf("unexpected vector lengths: %d %d", len(first), len(second))
	}
	for i := range first {
		if first[i] != second[i] {
			t.Fatalf("vector mismatch at %d: %v vs %v", i, first[i], second[i])
		}
	}
}
