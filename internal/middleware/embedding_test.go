package middleware

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ihgazi/vectorproxy/internal/search"
)

type mockEmbedder struct {
	EmbedFunc func(ctx context.Context, text string) ([]float32, error)
}

func (m *mockEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	if m.EmbedFunc != nil {
		return m.EmbedFunc(ctx, text)
	}
	return []float32{0.1, 0.2}, nil
}

func TestEmbeddingInterceptor_SingleRequest(t *testing.T) {
	embedCalled := false
	me := &mockEmbedder{
		EmbedFunc: func(ctx context.Context, text string) ([]float32, error) {
			embedCalled = true
			return []float32{0.9, 0.9}, nil
		},
	}

	dbCalled := false
	dbHandler := func(ctx context.Context, req *search.SearchQuery) (*search.SearchResponse, error) {
		dbCalled = true
		if len(req.Vector) != 2 || req.Vector[0] != 0.9 {
			t.Errorf("expected generated vector, got %v", req.Vector)
		}
		return &search.SearchResponse{}, nil
	}

	interceptor := NewEmbeddingInterceptor(me)
	wrapped := interceptor(dbHandler)

	req := &search.SearchQuery{Collection: "test", Query: "hello"}
	_, err := wrapped(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !embedCalled {
		t.Error("expected embedder to be called")
	}
	if !dbCalled {
		t.Error("expected database handler to be called")
	}
}

func TestEmbeddingInterceptor_Coalescing(t *testing.T) {
	var embedCalls int32
	me := &mockEmbedder{
		EmbedFunc: func(ctx context.Context, text string) ([]float32, error) {
			atomic.AddInt32(&embedCalls, 1)
			time.Sleep(50 * time.Millisecond) // Simulate slow embedder
			return []float32{0.5, 0.5}, nil
		},
	}

	dbHandler := func(ctx context.Context, req *search.SearchQuery) (*search.SearchResponse, error) {
		return &search.SearchResponse{}, nil
	}

	interceptor := NewEmbeddingInterceptor(me)
	wrapped := interceptor(dbHandler)

	// Fire 10 concurrent requests for the same text
	var wg sync.WaitGroup
	numConcurrent := 10

	for i := 0; i < numConcurrent; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req := &search.SearchQuery{Collection: "test", Query: "what is go?"}
			_, _ = wrapped(context.Background(), req)
		}()
	}

	wg.Wait()

	// 10 concurrent requests should trigger only 1 embedding generation call
	if calls := atomic.LoadInt32(&embedCalls); calls != 1 {
		t.Errorf("expected exactly 1 embed call, got %d", calls)
	}
}
