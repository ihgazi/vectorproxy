package batch

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ihgazi/vectorproxy/internal/store"
)

func TestRequestBatcher_SizeTrigger(t *testing.T) {
	var batchCount int32
	var reqCount int32

	handler := func(ctx context.Context, reqs []*store.SearchQuery) ([]*store.SearchResponse, error) {
		atomic.AddInt32(&batchCount, 1)
		atomic.AddInt32(&reqCount, int32(len(reqs)))

		resps := make([]*store.SearchResponse, len(reqs))
		for i := range reqs {
			resps[i] = &store.SearchResponse{Results: []store.SearchResult{{ID: "1"}}}
		}
		return resps, nil
	}

	batcher := New(handler, 3, 10*time.Second) // Long window, so size triggers first

	var wg sync.WaitGroup
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := batcher.Search(context.Background(), &store.SearchQuery{Collection: "c1"})
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		}()
	}

	// Also start the flush loop just in case, but size should trigger the flush immediately
	go batcher.FlushLoop()

	wg.Wait()

	if atomic.LoadInt32(&batchCount) != 1 {
		t.Errorf("Expected exactly 1 batch, got %d", batchCount)
	}
	if atomic.LoadInt32(&reqCount) != 3 {
		t.Errorf("Expected exactly 3 reqs, got %d", reqCount)
	}
}

func TestRequestBatcher_WindowTrigger(t *testing.T) {
	var batchCount int32

	handler := func(ctx context.Context, reqs []*store.SearchQuery) ([]*store.SearchResponse, error) {
		atomic.AddInt32(&batchCount, 1)
		resps := make([]*store.SearchResponse, len(reqs))
		for i := range reqs {
			resps[i] = &store.SearchResponse{}
		}
		return resps, nil
	}

	batcher := New(handler, 10, 50*time.Millisecond) // Large size, short window
	go batcher.FlushLoop()

	_, err := batcher.Search(context.Background(), &store.SearchQuery{Collection: "c1"})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if atomic.LoadInt32(&batchCount) != 1 {
		t.Errorf("Expected 1 batch, got %d", batchCount)
	}
}

func TestRequestBatcher_CollectionGrouping(t *testing.T) {
	var mu sync.Mutex
	collectionsSeen := make(map[string]int)

	handler := func(ctx context.Context, reqs []*store.SearchQuery) ([]*store.SearchResponse, error) {
		mu.Lock()
		collectionsSeen[reqs[0].Collection] = len(reqs)
		mu.Unlock()

		resps := make([]*store.SearchResponse, len(reqs))
		for i := range reqs {
			if reqs[i].Collection != reqs[0].Collection {
				t.Errorf("Mixed collections in batch: %s and %s", reqs[0].Collection, reqs[i].Collection)
			}
			resps[i] = &store.SearchResponse{}
		}
		return resps, nil
	}

	batcher := New(handler, 5, 50*time.Millisecond)
	go batcher.FlushLoop()

	var wg sync.WaitGroup
	// 2 for c1, 3 for c2
	reqs := []string{"c1", "c1", "c2", "c2", "c2"}
	for _, col := range reqs {
		wg.Add(1)
		go func(c string) {
			defer wg.Done()
			_, err := batcher.Search(context.Background(), &store.SearchQuery{Collection: c})
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		}(col)
	}

	wg.Wait()

	mu.Lock()
	defer mu.Unlock()
	if collectionsSeen["c1"] != 2 {
		t.Errorf("Expected 2 items for c1, got %d", collectionsSeen["c1"])
	}
	if collectionsSeen["c2"] != 3 {
		t.Errorf("Expected 3 items for c2, got %d", collectionsSeen["c2"])
	}
}

func TestRequestBatcher_BypassWhenDisabled(t *testing.T) {
	var batchCount int32

	handler := func(ctx context.Context, reqs []*store.SearchQuery) ([]*store.SearchResponse, error) {
		atomic.AddInt32(&batchCount, 1)
		resps := make([]*store.SearchResponse, len(reqs))
		for i := range reqs {
			resps[i] = &store.SearchResponse{}
		}
		return resps, nil
	}

	// Disable batcher (size=0)
	batcher := New(handler, 0, 50*time.Millisecond)
	go batcher.FlushLoop() // Should exit immediately

	var wg sync.WaitGroup
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := batcher.Search(context.Background(), &store.SearchQuery{Collection: "c1"})
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		}()
	}
	wg.Wait()

	// Since it's disabled, each request should trigger its own call to the handler
	if atomic.LoadInt32(&batchCount) != 3 {
		t.Errorf("Expected exactly 3 calls (bypassed), got %d", batchCount)
	}
}

func TestRequestBatcher_Cancellation(t *testing.T) {
	var reqCount int32

	handler := func(ctx context.Context, reqs []*store.SearchQuery) ([]*store.SearchResponse, error) {
		atomic.AddInt32(&reqCount, int32(len(reqs)))
		resps := make([]*store.SearchResponse, len(reqs))
		for i := range reqs {
			resps[i] = &store.SearchResponse{}
		}
		return resps, nil
	}

	batcher := New(handler, 2, 50*time.Millisecond)
	go batcher.FlushLoop()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, err := batcher.Search(ctx, &store.SearchQuery{Collection: "c1"})
		if err == nil || !errors.Is(err, context.Canceled) {
			t.Errorf("Expected context canceled error, got: %v", err)
		}
	}()

	// Add a valid request to trigger the flush of the cancelled one as well
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, err := batcher.Search(context.Background(), &store.SearchQuery{Collection: "c1"})
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
	}()

	wg.Wait()

	if atomic.LoadInt32(&reqCount) != 1 {
		t.Errorf("Expected exactly 1 request to reach handler (1 was cancelled), got %d", reqCount)
	}
}

func TestRequestBatcher_ErrorPropagation(t *testing.T) {
	expectedErr := errors.New("database failure")

	handler := func(ctx context.Context, reqs []*store.SearchQuery) ([]*store.SearchResponse, error) {
		return nil, expectedErr
	}

	batcher := New(handler, 3, 50*time.Millisecond)
	go batcher.FlushLoop()

	var wg sync.WaitGroup
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := batcher.Search(context.Background(), &store.SearchQuery{Collection: "c1"})
			if err != expectedErr {
				t.Errorf("Expected %v, got %v", expectedErr, err)
			}
		}()
	}

	wg.Wait()
}
