package embedding

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

// mockEmbedder allows injecting behaviors into the base embedder for testing.
type mockEmbedder struct {
	embedCalls      int
	embedBatchCalls int
	embedFunc       func(ctx context.Context, text string) ([]float32, error)
	embedBatchFunc  func(ctx context.Context, texts []string) ([][]float32, error)
}

func (m *mockEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	m.embedCalls++
	if m.embedFunc != nil {
		return m.embedFunc(ctx, text)
	}
	return []float32{1.0, 2.0}, nil
}

func (m *mockEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	m.embedBatchCalls++
	if m.embedBatchFunc != nil {
		return m.embedBatchFunc(ctx, texts)
	}
	return [][]float32{{1.0, 2.0}}, nil
}

func TestCachedEmbedder_Embed_HitAndMiss(t *testing.T) {
	mockBase := &mockEmbedder{}
	cached, err := NewCachedEmbedder(mockBase, 10)
	if err != nil {
		t.Fatalf("unexpected error creating CachedEmbedder: %v", err)
	}

	ctx := context.Background()
	text := "hello world"

	// 1. First call should be a miss, triggering underlying embedder
	vec1, err := cached.Embed(ctx, text)
	if err != nil {
		t.Fatalf("unexpected error on Embed: %v", err)
	}
	if mockBase.embedCalls != 1 {
		t.Errorf("expected 1 call to base Embed, got %d", mockBase.embedCalls)
	}

	// 2. Second call should be a hit, NOT triggering underlying embedder
	vec2, err := cached.Embed(ctx, text)
	if err != nil {
		t.Fatalf("unexpected error on Embed (cache hit): %v", err)
	}
	if mockBase.embedCalls != 1 {
		t.Errorf("expected still 1 call to base Embed after cache hit, got %d", mockBase.embedCalls)
	}

	// Ensure the returned vectors are correct
	if !reflect.DeepEqual(vec1, vec2) {
		t.Errorf("cached vector %v does not match original vector %v", vec2, vec1)
	}
}

func TestCachedEmbedder_Embed_ErrorPropagation(t *testing.T) {
	expectedErr := errors.New("underlying error")
	mockBase := &mockEmbedder{
		embedFunc: func(ctx context.Context, text string) ([]float32, error) {
			return nil, expectedErr
		},
	}

	cached, _ := NewCachedEmbedder(mockBase, 10)
	_, err := cached.Embed(context.Background(), "fail string")

	if err != expectedErr {
		t.Errorf("expected error %v, got %v", expectedErr, err)
	}
	
	// Subsequent call should retry and still hit base embedder (not cached)
	_, err2 := cached.Embed(context.Background(), "fail string")
	if err2 != expectedErr {
		t.Errorf("expected error %v on retry, got %v", expectedErr, err2)
	}
	if mockBase.embedCalls != 2 {
		t.Errorf("expected 2 calls to base Embed (errors shouldn't be cached), got %d", mockBase.embedCalls)
	}
}

func TestCachedEmbedder_EmbedBatch_PassThrough(t *testing.T) {
	mockBase := &mockEmbedder{}
	cached, _ := NewCachedEmbedder(mockBase, 10)

	texts := []string{"one", "two"}
	_, err := cached.EmbedBatch(context.Background(), texts)
	if err != nil {
		t.Fatalf("unexpected error on EmbedBatch: %v", err)
	}

	if mockBase.embedBatchCalls != 1 {
		t.Errorf("expected 1 call to base EmbedBatch, got %d", mockBase.embedBatchCalls)
	}
}

func TestNewCachedEmbedder_InvalidCapacity(t *testing.T) {
	// LRU cache requires positive capacity
	_, err := NewCachedEmbedder(&mockEmbedder{}, 0)
	if err == nil {
		t.Errorf("expected error when passing 0 capacity, got nil")
	}
}
