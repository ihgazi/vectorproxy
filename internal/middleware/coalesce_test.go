package middleware

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ihgazi/vectorproxy/internal/coalesce"
	"github.com/ihgazi/vectorproxy/internal/search"
)

func TestCoalesceInterceptor_BypassWhenNoKey(t *testing.T) {
	dbCalled := false
	dbHandler := func(ctx context.Context, req *search.SearchQuery) (*search.SearchResponse, error) {
		dbCalled = true
		return &search.SearchResponse{}, nil
	}

	// Generator returns empty string key -> should bypass
	dummyGen := func(req *search.SearchQuery) string { return "" }
	interceptor := NewCoalesceInterceptor(dummyGen)
	wrapped := interceptor(dbHandler)

	req := &search.SearchQuery{Collection: "test"}
	_, err := wrapped(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !dbCalled {
		t.Error("expected database handler to be called when key is empty")
	}
}

func TestCoalesceInterceptor_InFlightMerging(t *testing.T) {
	var dbCallCount int32
	dbHandler := func(ctx context.Context, req *search.SearchQuery) (*search.SearchResponse, error) {
		atomic.AddInt32(&dbCallCount, 1)
		time.Sleep(50 * time.Millisecond) // Simulate high latency (like embedding or DB fetch)
		return &search.SearchResponse{
			Results: []search.SearchResult{{ID: "shared-result"}},
		}, nil
	}

	// Create interceptor with string key generator
	stringGen := coalesce.NewStringKeyGenerator()
	interceptor := NewCoalesceInterceptor(stringGen)
	wrapped := interceptor(dbHandler)

	// Launch multiple concurrent duplicate requests
	var wg sync.WaitGroup
	numConcurrent := 10
	results := make([]*search.SearchResponse, numConcurrent)

	for i := 0; i < numConcurrent; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			req := &search.SearchQuery{Collection: "test-col", Query: "shared-prompt"}
			res, err := wrapped(context.Background(), req)
			if err != nil {
				t.Errorf("Goroutine %d failed: %v", idx, err)
			}
			results[idx] = res
		}(i)
	}

	wg.Wait()

	// Verify that the actual database/handler was only executed once
	if count := atomic.LoadInt32(&dbCallCount); count != 1 {
		t.Errorf("expected database to be called exactly once, but got executed %d times", count)
	}

	// Verify all concurrent callers received the correct shared result
	for idx, res := range results {
		if res == nil || len(res.Results) != 1 || res.Results[0].ID != "shared-result" {
			t.Errorf("Goroutine %d got invalid or nil result: %v", idx, res)
		}
	}
}

func TestCoalesceInterceptor_ErrorPropagation(t *testing.T) {
	expectedErr := errors.New("database failure")
	dbHandler := func(ctx context.Context, req *search.SearchQuery) (*search.SearchResponse, error) {
		return nil, expectedErr
	}

	stringGen := coalesce.NewStringKeyGenerator()
	interceptor := NewCoalesceInterceptor(stringGen)
	wrapped := interceptor(dbHandler)

	req := &search.SearchQuery{Collection: "test-col", Query: "error-prompt"}
	_, err := wrapped(context.Background(), req)
	if err != expectedErr {
		t.Errorf("expected error %v, got %v", expectedErr, err)
	}
}
