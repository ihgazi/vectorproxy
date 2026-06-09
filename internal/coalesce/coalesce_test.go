package coalesce

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ihgazi/vectorproxy/internal/search"
)

func TestCapacityCoalescer_BypassWhenNoKey(t *testing.T) {
	c := New(func(req *search.SearchQuery) string { return "" }, 5)

	dbCalled := false
	handler := func(ctx context.Context, req *search.SearchQuery) (*search.SearchResponse, error) {
		dbCalled = true
		return &search.SearchResponse{}, nil
	}

	_, err := c.Do(context.Background(), &search.SearchQuery{TopK: 3}, handler)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !dbCalled {
		t.Fatal("expected handler to be called")
	}
}

func TestCapacityCoalescer_CoalescesWhenFits(t *testing.T) {
	c := New(func(req *search.SearchQuery) string { return "key1" }, 5)

	var calls int32
	handler := func(ctx context.Context, req *search.SearchQuery) (*search.SearchResponse, error) {
		atomic.AddInt32(&calls, 1)
		time.Sleep(50 * time.Millisecond)
		return &search.SearchResponse{
			Results: []search.SearchResult{{ID: "1"}, {ID: "2"}, {ID: "3"}, {ID: "4"}, {ID: "5"}},
		}, nil
	}

	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			res, err := c.Do(context.Background(), &search.SearchQuery{TopK: 3}, handler)
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

func TestCapacityCoalescer_BypassWhenExceedsCapacity(t *testing.T) {
	c := New(func(req *search.SearchQuery) string { return "key1" }, 5)

	var calls int32
	waitCh := make(chan struct{})

	handler := func(ctx context.Context, req *search.SearchQuery) (*search.SearchResponse, error) {
		atomic.AddInt32(&calls, 1)
		if req.TopK == 5 {
			<-waitCh // Block the first request so the second one arrives while it's in-flight
		}
		return &search.SearchResponse{
			Results: make([]search.SearchResult, req.TopK),
		}, nil
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		c.Do(context.Background(), &search.SearchQuery{TopK: 5}, handler)
	}()

	time.Sleep(10 * time.Millisecond) // Ensure the first request is in-flight

	wg.Add(1)
	go func() {
		defer wg.Done()
		res, err := c.Do(context.Background(), &search.SearchQuery{TopK: 10}, handler)
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

func TestCapacityCoalescer_MinKUpgradesDispatch(t *testing.T) {
	c := New(func(req *search.SearchQuery) string { return "key1" }, 10)

	var dispatchedK int32
	handler := func(ctx context.Context, req *search.SearchQuery) (*search.SearchResponse, error) {
		dispatchedK = req.TopK
		return &search.SearchResponse{
			Results: make([]search.SearchResult, req.TopK),
		}, nil
	}

	res, err := c.Do(context.Background(), &search.SearchQuery{TopK: 3}, handler)
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

func TestCapacityCoalescer_TrimIsShallowCopy(t *testing.T) {
	c := New(func(req *search.SearchQuery) string { return "key1" }, 5)

	originalResp := &search.SearchResponse{
		Results: []search.SearchResult{{ID: "1"}, {ID: "2"}, {ID: "3"}, {ID: "4"}, {ID: "5"}},
	}

	handler := func(ctx context.Context, req *search.SearchQuery) (*search.SearchResponse, error) {
		time.Sleep(50 * time.Millisecond)
		return originalResp, nil
	}

	var wg sync.WaitGroup
	var r1, r2 *search.SearchResponse

	wg.Add(2)
	go func() {
		defer wg.Done()
		r1, _ = c.Do(context.Background(), &search.SearchQuery{TopK: 2}, handler)
	}()
	go func() {
		defer wg.Done()
		r2, _ = c.Do(context.Background(), &search.SearchQuery{TopK: 3}, handler)
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

	// Modifying one shouldn't affect the other or the original
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

func TestCapacityCoalescer_ErrorPropagation(t *testing.T) {
	c := New(func(req *search.SearchQuery) string { return "key1" }, 5)

	expectedErr := errors.New("handler error")
	handler := func(ctx context.Context, req *search.SearchQuery) (*search.SearchResponse, error) {
		time.Sleep(50 * time.Millisecond)
		return nil, expectedErr
	}

	var wg sync.WaitGroup
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := c.Do(context.Background(), &search.SearchQuery{TopK: 3}, handler)
			if err != expectedErr {
				t.Errorf("expected err %v, got %v", expectedErr, err)
			}
		}()
	}
	wg.Wait()
}
