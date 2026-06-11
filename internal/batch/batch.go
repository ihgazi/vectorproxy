package batch

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/ihgazi/vectorproxy/internal/search"
)

// BatchHandler mirrors the SearchHandler signature to keep this package decoupled.
type BatchHandler func(ctx context.Context, req []*search.SearchQuery) ([]*search.SearchResponse, error)

// batchItem represents a single search query in a batch request.
type batchItem struct {
	req      *search.SearchQuery
	ctx      context.Context
	respChan chan *search.SearchResponse
	errChan  chan error
}

// RequestBatcher batches incoming search requests and dispatches them together to minimze overhead.
type RequestBatcher struct {
	next         BatchHandler
	mu           sync.Mutex
	buffers      map[string][]*batchItem // Map of collection name to batch items
	maxBatchSize int
	batchWindow  time.Duration
	flushTrigger chan struct{}
}

// New creates a new RequestBatcher.
func New(next BatchHandler, maxBatchSize int, batchWindow time.Duration) *RequestBatcher {
	return &RequestBatcher{
		next:         next,
		buffers:      make(map[string][]*batchItem),
		maxBatchSize: maxBatchSize,
		batchWindow:  batchWindow,
		flushTrigger: make(chan struct{}, 1),
	}
}

// Search adds incoming requsts to the batch buffer.
// It triggers batch dispatch when either batch window elapses or max batch size is reached.
func (b *RequestBatcher) Search(ctx context.Context, req *search.SearchQuery) (*search.SearchResponse, error) {
	if b.maxBatchSize <= 0 || b.batchWindow <= 0 {
		// If no batching is configured, call the next handler directly.
		resp, err := b.next(ctx, []*search.SearchQuery{req})
		if err != nil {
			return nil, err
		}
		return resp[0], nil
	}

	item := &batchItem{
		req:      req,
		ctx:      ctx,
		respChan: make(chan *search.SearchResponse, 1),
		errChan:  make(chan error, 1),
	}

	b.mu.Lock()
	b.buffers[req.Collection] = append(b.buffers[req.Collection], item)
	b.mu.Unlock()

	if len(b.buffers[req.Collection]) >= b.maxBatchSize {
		select {
		case b.flushTrigger <- struct{}{}:
		default:
		}
	}

	// Block until the background loop flushes this item and sends the result.
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case err := <-item.errChan:
		return nil, err
	case resp := <-item.respChan:
		return resp, nil
	}
}

// FlushLoop runs in the background to periodically check the batch buffer and dispatch requests.
func (b *RequestBatcher) FlushLoop() {
	if b.batchWindow <= 0 || b.maxBatchSize <= 0 {
		return
	}
	ticker := time.NewTicker(b.batchWindow)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			b.doFlush()
		case <-b.flushTrigger:
			b.doFlush()
		}
	}
}

func (b *RequestBatcher) doFlush() {
	b.mu.Lock()
	// Swap the buffers out quickly to release the lock
	bufs := b.buffers
	b.buffers = make(map[string][]*batchItem)
	b.mu.Unlock()

	for collection, batch := range bufs {
		if len(batch) == 0 {
			continue
		}

		// Run each collection flush concurrently
		go b.flushCollection(collection, batch)
	}
}

func (b *RequestBatcher) flushCollection(collection string, batch []*batchItem) {
	// Extract search queries for the batch request
	var reqs []*search.SearchQuery
	var activeItems []*batchItem
	for _, item := range batch {
		if item.ctx.Err() == nil {
			reqs = append(reqs, item.req)
			activeItems = append(activeItems, item)
		}
	}

	if len(reqs) == 0 {
		return
	}

	// Dispatch the batch request
	log.Printf("Dispatching batch of %d requests for collection: %s", len(reqs), collection)
	batchResps, err := b.next(context.Background(), reqs)

	// Fan-out the responses
	if err != nil {
		for _, item := range activeItems {
			item.errChan <- err
		}
		return
	}

	for i, item := range activeItems {
		item.respChan <- batchResps[i]
	}
}
