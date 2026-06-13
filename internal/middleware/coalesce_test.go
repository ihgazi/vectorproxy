package middleware

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ihgazi/vectorproxy/internal/keygen"
	"github.com/ihgazi/vectorproxy/internal/store"
)

func TestCoalesceInterceptor_BypassWhenNoKey(t *testing.T) {
	dbCalled := false
	dbHandler := func(ctx context.Context, req *store.SearchQuery) (*store.SearchResponse, error) {
		dbCalled = true
		return &store.SearchResponse{}, nil
	}

	dummyGen := func(req *store.SearchQuery) string { return "" }
	interceptor := NewCoalesceInterceptor(dummyGen, 5)
	wrapped := interceptor(dbHandler)

	req := &store.SearchQuery{Collection: "test", TopK: 3}
	_, err := wrapped(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !dbCalled {
		t.Error("expected database handler to be called when key is empty")
	}
}

func TestCoalesceInterceptor_CoalescesWhenFits(t *testing.T) {
	var calls int32
	dbHandler := func(ctx context.Context, req *store.SearchQuery) (*store.SearchResponse, error) {
		atomic.AddInt32(&calls, 1)
		time.Sleep(50 * time.Millisecond)
		return &store.SearchResponse{
			Results: []store.SearchResult{{ID: "1"}, {ID: "2"}, {ID: "3"}, {ID: "4"}, {ID: "5"}},
		}, nil
	}

	stringGen := keygen.NewStringKeyGenerator()
	interceptor := NewCoalesceInterceptor(stringGen, 5)
	wrapped := interceptor(dbHandler)

	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req := &store.SearchQuery{Collection: "test-col", Query: "prompt", TopK: 3}
			res, err := wrapped(context.Background(), req)
			if err != nil {
				t.Errorf("unexpected err: %v", err)
			}
			if len(res.Results) != 3 {
				t.Errorf("expected 3 results, got %d", len(res.Results))
			}
		}()
	}
	wg.Wait()

	if calls != 1 {
		t.Errorf("expected 1 call, got %d", calls)
	}
}

func TestCoalesceInterceptor_BypassWhenExceedsCapacity(t *testing.T) {
	var calls int32
	waitCh := make(chan struct{})

	dbHandler := func(ctx context.Context, req *store.SearchQuery) (*store.SearchResponse, error) {
		atomic.AddInt32(&calls, 1)
		if req.TopK == 5 {
			<-waitCh // Block the first request so the second one arrives while it's in-flight
		}
		return &store.SearchResponse{
			Results: make([]store.SearchResult, req.TopK),
		}, nil
	}

	stringGen := keygen.NewStringKeyGenerator()
	interceptor := NewCoalesceInterceptor(stringGen, 5)
	wrapped := interceptor(dbHandler)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		wrapped(context.Background(), &store.SearchQuery{Collection: "col", Query: "q", TopK: 5})
	}()

	time.Sleep(10 * time.Millisecond) // Ensure the first request is in-flight

	wg.Add(1)
	go func() {
		defer wg.Done()
		res, err := wrapped(context.Background(), &store.SearchQuery{Collection: "col", Query: "q", TopK: 10})
		if err != nil {
			t.Errorf("unexpected err: %v", err)
		}
		if len(res.Results) != 10 {
			t.Errorf("expected 10 results, got %d", len(res.Results))
		}
		close(waitCh) // unblock the first request
	}()

	wg.Wait()

	if calls != 2 {
		t.Errorf("expected 2 calls, got %d", calls)
	}
}

func TestCoalesceInterceptor_MinKUpgradesDispatch(t *testing.T) {
	var dispatchedK int32
	dbHandler := func(ctx context.Context, req *store.SearchQuery) (*store.SearchResponse, error) {
		dispatchedK = req.TopK
		return &store.SearchResponse{
			Results: make([]store.SearchResult, req.TopK),
		}, nil
	}

	stringGen := keygen.NewStringKeyGenerator()
	interceptor := NewCoalesceInterceptor(stringGen, 10)
	wrapped := interceptor(dbHandler)

	req := &store.SearchQuery{Collection: "col", Query: "q", TopK: 3}
	res, err := wrapped(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if dispatchedK != 10 {
		t.Errorf("expected dispatchedK to be upgraded to 10, got %d", dispatchedK)
	}
	if len(res.Results) != 3 {
		t.Errorf("expected 3 results due to trimming, got %d", len(res.Results))
	}
}

func TestCoalesceInterceptor_TrimIsShallowCopy(t *testing.T) {
	originalResp := &store.SearchResponse{
		Results: []store.SearchResult{{ID: "1"}, {ID: "2"}, {ID: "3"}, {ID: "4"}, {ID: "5"}},
	}

	dbHandler := func(ctx context.Context, req *store.SearchQuery) (*store.SearchResponse, error) {
		time.Sleep(50 * time.Millisecond)
		return originalResp, nil
	}

	stringGen := keygen.NewStringKeyGenerator()
	interceptor := NewCoalesceInterceptor(stringGen, 5)
	wrapped := interceptor(dbHandler)

	var wg sync.WaitGroup
	var r1, r2 *store.SearchResponse

	wg.Add(2)
	go func() {
		defer wg.Done()
		r1, _ = wrapped(context.Background(), &store.SearchQuery{Collection: "c", Query: "q", TopK: 2})
	}()
	go func() {
		defer wg.Done()
		r2, _ = wrapped(context.Background(), &store.SearchQuery{Collection: "c", Query: "q", TopK: 3})
	}()

	wg.Wait()

	if len(originalResp.Results) != 5 {
		t.Errorf("original response was mutated")
	}
	if len(r1.Results) != 2 {
		t.Errorf("expected 2 results, got %d", len(r1.Results))
	}
	if len(r2.Results) != 3 {
		t.Errorf("expected 3 results, got %d", len(r2.Results))
	}

	if r1 != nil && len(r1.Results) > 0 {
		r1.Results[0].ID = "mutated"
	}
	if originalResp.Results[0].ID == "mutated" {
		t.Errorf("shallow copy failed, original mutated")
	}
	if r2 != nil && len(r2.Results) > 0 && r2.Results[0].ID == "mutated" {
		t.Errorf("shallow copy failed, peer mutated")
	}
}

func TestCoalesceInterceptor_ErrorPropagation(t *testing.T) {
	expectedErr := errors.New("handler error")
	dbHandler := func(ctx context.Context, req *store.SearchQuery) (*store.SearchResponse, error) {
		time.Sleep(50 * time.Millisecond)
		return nil, expectedErr
	}

	stringGen := keygen.NewStringKeyGenerator()
	interceptor := NewCoalesceInterceptor(stringGen, 5)
	wrapped := interceptor(dbHandler)

	var wg sync.WaitGroup
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := wrapped(context.Background(), &store.SearchQuery{Collection: "c", Query: "q", TopK: 3})
			if err != expectedErr {
				t.Errorf("expected err %v, got %v", expectedErr, err)
			}
		}()
	}
	wg.Wait()
}
