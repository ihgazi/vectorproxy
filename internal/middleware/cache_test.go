package middleware

import (
	"context"
	"errors"
	"testing"

	"github.com/ihgazi/vectorproxy/internal/store"
)

// MockSemanticCache implements the cache.SemanticCache interface for testing.
type MockSemanticCache struct {
	GetFunc func(ctx context.Context, collection string, vector []float32, topK int32) (*store.SearchResponse, bool, error)
	SetFunc func(ctx context.Context, collection string, vector []float32, resp *store.SearchResponse) error
}

func (m *MockSemanticCache) Get(ctx context.Context, collection string, vector []float32, topK int32) (*store.SearchResponse, bool, error) {
	if m.GetFunc != nil {
		return m.GetFunc(ctx, collection, vector, topK)
	}
	return nil, false, nil
}

func (m *MockSemanticCache) Set(ctx context.Context, collection string, vector []float32, resp *store.SearchResponse) error {
	if m.SetFunc != nil {
		return m.SetFunc(ctx, collection, vector, resp)
	}
	return nil
}

func (m *MockSemanticCache) Invalidate(ctx context.Context, collection string) error {
	return nil
}

func (m *MockSemanticCache) Close() error { return nil }

func TestCacheInterceptor_BypassWhenNoVector(t *testing.T) {
	called := false
	dbHandler := func(ctx context.Context, req *store.SearchQuery) (*store.SearchResponse, error) {
		called = true
		return &store.SearchResponse{}, nil
	}

	mc := &MockSemanticCache{}
	interceptor := NewCacheInterceptor(mc)
	wrapped := interceptor(dbHandler)

	req := &store.SearchQuery{Collection: "test"} // No vector
	_, err := wrapped(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatal("expected database handler to be called when vector is absent")
	}
}

func TestCacheInterceptor_CacheHit(t *testing.T) {
	dbCalled := false
	dbHandler := func(ctx context.Context, req *store.SearchQuery) (*store.SearchResponse, error) {
		dbCalled = true
		return &store.SearchResponse{}, nil
	}

	expectedResp := &store.SearchResponse{
		Results: []store.SearchResult{{ID: "cached-id", Score: 0.99}},
	}

	mc := &MockSemanticCache{
		GetFunc: func(ctx context.Context, collection string, vector []float32, topK int32) (*store.SearchResponse, bool, error) {
			return expectedResp, true, nil
		},
	}

	interceptor := NewCacheInterceptor(mc)
	wrapped := interceptor(dbHandler)

	req := &store.SearchQuery{Collection: "test", Vector: []float32{0.1, 0.2}}
	resp, err := wrapped(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dbCalled {
		t.Fatal("expected database handler NOT to be called on cache hit")
	}
	if len(resp.Results) != 1 || resp.Results[0].ID != "cached-id" {
		t.Errorf("expected cached response, got %v", resp)
	}
}

func TestCacheInterceptor_CacheMissAndSet(t *testing.T) {
	dbCalled := false
	expectedResp := &store.SearchResponse{
		Results: []store.SearchResult{{ID: "db-id", Score: 0.88}},
	}
	dbHandler := func(ctx context.Context, req *store.SearchQuery) (*store.SearchResponse, error) {
		dbCalled = true
		return expectedResp, nil
	}

	setCalled := false
	mc := &MockSemanticCache{
		GetFunc: func(ctx context.Context, collection string, vector []float32, topK int32) (*store.SearchResponse, bool, error) {
			return nil, false, nil // Cache miss
		},
		SetFunc: func(ctx context.Context, collection string, vector []float32, resp *store.SearchResponse) error {
			setCalled = true
			if resp != expectedResp {
				t.Errorf("expected set payload %v, got %v", expectedResp, resp)
			}
			return nil
		},
	}

	interceptor := NewCacheInterceptor(mc)
	wrapped := interceptor(dbHandler)

	req := &store.SearchQuery{Collection: "test", Vector: []float32{0.1, 0.2}}
	resp, err := wrapped(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !dbCalled {
		t.Fatal("expected database handler to be called on cache miss")
	}
	if !setCalled {
		t.Fatal("expected cache Set to be called on cache miss")
	}
	if resp != expectedResp {
		t.Errorf("expected db response, got %v", resp)
	}
}

func TestCacheInterceptor_FailOpenOnCacheError(t *testing.T) {
	dbCalled := false
	expectedResp := &store.SearchResponse{
		Results: []store.SearchResult{{ID: "db-id"}},
	}
	dbHandler := func(ctx context.Context, req *store.SearchQuery) (*store.SearchResponse, error) {
		dbCalled = true
		return expectedResp, nil
	}

	mc := &MockSemanticCache{
		GetFunc: func(ctx context.Context, collection string, vector []float32, topK int32) (*store.SearchResponse, bool, error) {
			return nil, false, errors.New("cache error")
		},
	}

	interceptor := NewCacheInterceptor(mc)
	wrapped := interceptor(dbHandler)

	req := &store.SearchQuery{Collection: "test", Vector: []float32{0.1, 0.2}}
	resp, err := wrapped(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v (should fail open)", err)
	}
	if !dbCalled {
		t.Fatal("expected database handler to be called when cache returns an error (fail open)")
	}
	if resp != expectedResp {
		t.Errorf("expected db response, got %v", resp)
	}
}

// TestCacheInterceptor_TopKHit_SufficientResults verifies that when the cache
// returns a hit with exactly topK results (already trimmed), the DB is not called.
func TestCacheInterceptor_TopKHit_SufficientResults(t *testing.T) {
	dbCalled := false
	dbHandler := func(ctx context.Context, req *store.SearchQuery) (*store.SearchResponse, error) {
		dbCalled = true
		return &store.SearchResponse{}, nil
	}

	// Simulates what the real cache returns after trimming: exactly topK=3 results.
	trimmedResp := &store.SearchResponse{
		Results: []store.SearchResult{{ID: "r1"}, {ID: "r2"}, {ID: "r3"}},
	}

	mc := &MockSemanticCache{
		GetFunc: func(ctx context.Context, collection string, vector []float32, topK int32) (*store.SearchResponse, bool, error) {
			return trimmedResp, true, nil
		},
	}

	interceptor := NewCacheInterceptor(mc)
	wrapped := interceptor(dbHandler)

	req := &store.SearchQuery{Collection: "test", Vector: []float32{0.1, 0.2}, TopK: 3}
	resp, err := wrapped(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dbCalled {
		t.Fatal("expected database handler NOT to be called when cache has sufficient results for topK")
	}
	if len(resp.Results) != 3 {
		t.Errorf("expected 3 results, got %d", len(resp.Results))
	}
}

// TestCacheInterceptor_TopKMiss_InsufficientResults verifies that when the cache
// returns a miss (because stored results < topK), the DB is queried.
func TestCacheInterceptor_TopKMiss_InsufficientResults(t *testing.T) {
	dbCalled := false
	dbResp := &store.SearchResponse{
		Results: []store.SearchResult{{ID: "r1"}, {ID: "r2"}, {ID: "r3"}, {ID: "r4"}, {ID: "r5"}},
	}
	dbHandler := func(ctx context.Context, req *store.SearchQuery) (*store.SearchResponse, error) {
		dbCalled = true
		return dbResp, nil
	}

	mc := &MockSemanticCache{
		GetFunc: func(ctx context.Context, collection string, vector []float32, topK int32) (*store.SearchResponse, bool, error) {
			// Simulates the real cache returning a miss: only 2 results cached, topK=5 requested.
			return nil, false, nil
		},
	}

	interceptor := NewCacheInterceptor(mc)
	wrapped := interceptor(dbHandler)

	req := &store.SearchQuery{Collection: "test", Vector: []float32{0.1, 0.2}, TopK: 5}
	resp, err := wrapped(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !dbCalled {
		t.Fatal("expected database handler to be called when cache has insufficient results for topK")
	}
	if len(resp.Results) != 5 {
		t.Errorf("expected 5 results from DB response, got %d", len(resp.Results))
	}
}

// TestCacheInterceptor_TopKForwardedToGet verifies that the interceptor correctly
// forwards req.TopK to the cache.Get call, so the cache can apply its trimming logic.
func TestCacheInterceptor_TopKForwardedToGet(t *testing.T) {
	var receivedTopK int32 = -1

	mc := &MockSemanticCache{
		GetFunc: func(ctx context.Context, collection string, vector []float32, topK int32) (*store.SearchResponse, bool, error) {
			receivedTopK = topK
			return nil, false, nil
		},
	}

	dbHandler := func(ctx context.Context, req *store.SearchQuery) (*store.SearchResponse, error) {
		return &store.SearchResponse{}, nil
	}

	interceptor := NewCacheInterceptor(mc)
	wrapped := interceptor(dbHandler)

	req := &store.SearchQuery{Collection: "test", Vector: []float32{0.1, 0.2}, TopK: 7}
	if _, err := wrapped(context.Background(), req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if receivedTopK != 7 {
		t.Errorf("expected topK=7 to be forwarded to cache.Get, got %d", receivedTopK)
	}
}
