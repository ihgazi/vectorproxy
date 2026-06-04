package middleware

import (
	"context"
	"errors"
	"testing"

	"github.com/ihgazi/vectorproxy/internal/search"
)

// MockSemanticCache implements the cache.SemanticCache interface for testing.
type MockSemanticCache struct {
	GetFunc func(ctx context.Context, collection string, vector []float32) (*search.SearchResponse, bool, error)
	SetFunc func(ctx context.Context, collection string, vector []float32, resp *search.SearchResponse) error
}

func (m *MockSemanticCache) Get(ctx context.Context, collection string, vector []float32) (*search.SearchResponse, bool, error) {
	if m.GetFunc != nil {
		return m.GetFunc(ctx, collection, vector)
	}
	return nil, false, nil
}

func (m *MockSemanticCache) Set(ctx context.Context, collection string, vector []float32, resp *search.SearchResponse) error {
	if m.SetFunc != nil {
		return m.SetFunc(ctx, collection, vector, resp)
	}
	return nil
}

func (m *MockSemanticCache) Close() error { return nil }

func TestCacheInterceptor_BypassWhenNoVector(t *testing.T) {
	called := false
	dbHandler := func(ctx context.Context, req *search.SearchQuery) (*search.SearchResponse, error) {
		called = true
		return &search.SearchResponse{}, nil
	}

	mc := &MockSemanticCache{}
	interceptor := NewCacheInterceptor(mc)
	wrapped := interceptor(dbHandler)

	req := &search.SearchQuery{Collection: "test"} // No vector
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
	dbHandler := func(ctx context.Context, req *search.SearchQuery) (*search.SearchResponse, error) {
		dbCalled = true
		return &search.SearchResponse{}, nil
	}

	expectedResp := &search.SearchResponse{
		Results: []search.SearchResult{{ID: "cached-id", Score: 0.99}},
	}

	mc := &MockSemanticCache{
		GetFunc: func(ctx context.Context, collection string, vector []float32) (*search.SearchResponse, bool, error) {
			return expectedResp, true, nil
		},
	}

	interceptor := NewCacheInterceptor(mc)
	wrapped := interceptor(dbHandler)

	req := &search.SearchQuery{Collection: "test", Vector: []float32{0.1, 0.2}}
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
	expectedResp := &search.SearchResponse{
		Results: []search.SearchResult{{ID: "db-id", Score: 0.88}},
	}
	dbHandler := func(ctx context.Context, req *search.SearchQuery) (*search.SearchResponse, error) {
		dbCalled = true
		return expectedResp, nil
	}

	setCalled := false
	mc := &MockSemanticCache{
		GetFunc: func(ctx context.Context, collection string, vector []float32) (*search.SearchResponse, bool, error) {
			return nil, false, nil // Cache miss
		},
		SetFunc: func(ctx context.Context, collection string, vector []float32, resp *search.SearchResponse) error {
			setCalled = true
			if resp != expectedResp {
				t.Errorf("expected set payload %v, got %v", expectedResp, resp)
			}
			return nil
		},
	}

	interceptor := NewCacheInterceptor(mc)
	wrapped := interceptor(dbHandler)

	req := &search.SearchQuery{Collection: "test", Vector: []float32{0.1, 0.2}}
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
	expectedResp := &search.SearchResponse{
		Results: []search.SearchResult{{ID: "db-id"}},
	}
	dbHandler := func(ctx context.Context, req *search.SearchQuery) (*search.SearchResponse, error) {
		dbCalled = true
		return expectedResp, nil
	}

	mc := &MockSemanticCache{
		GetFunc: func(ctx context.Context, collection string, vector []float32) (*search.SearchResponse, bool, error) {
			return nil, false, errors.New("cache error")
		},
	}

	interceptor := NewCacheInterceptor(mc)
	wrapped := interceptor(dbHandler)

	req := &search.SearchQuery{Collection: "test", Vector: []float32{0.1, 0.2}}
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
