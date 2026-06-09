package coalesce

import (
	"context"
	"sync"

	"github.com/ihgazi/vectorproxy/internal/keygen"
	"github.com/ihgazi/vectorproxy/internal/search"
)

// Handler mirrors the SearchHandler signature to keep this package decoupled.
type Handler func(ctx context.Context, req *search.SearchQuery) (*search.SearchResponse, error)

type inflightRequest struct {
	dispatchedK int32
	done        chan struct{}
	result      *search.SearchResponse
	err         error
}

// CapacityCoalescer coalesces in-flight search requests by key and TopK capacity.
type CapacityCoalescer struct {
	mu        sync.Mutex
	inflight  map[string]*inflightRequest
	generator keygen.KeyGenerator
	minK      int32 // Limits the minimum TopK to minimize bypasses for small requests.
}

// New creates a new CapacityCoalescer.
func New(generator keygen.KeyGenerator, minK int32) *CapacityCoalescer {
	return &CapacityCoalescer{
		inflight:  make(map[string]*inflightRequest),
		generator: generator,
		minK:      minK,
	}
}

// Do executes and returns the results of the given function, making
// sure that only one execution is in-flight for a given key and capacity.
func (c *CapacityCoalescer) Do(ctx context.Context, req *search.SearchQuery, fn Handler) (*search.SearchResponse, error) {
	key := c.generator(req)

	if key == "" {
		return fn(ctx, req)
	}

	c.mu.Lock()
	flight, exists := c.inflight[key]

	if exists {
		if req.TopK <= flight.dispatchedK {
			c.mu.Unlock()
			<-flight.done
			if flight.err != nil {
				return nil, flight.err
			}
			return trimResponse(flight.result, req.TopK), nil
		}
		// In-flight exists but req.TopK > dispatchedK. Bypass.
		c.mu.Unlock()
		return fn(ctx, req)
	}

	// No in-flight request exists.
	dispatchedK := req.TopK
	if dispatchedK < c.minK {
		dispatchedK = c.minK
	}

	flight = &inflightRequest{
		dispatchedK: dispatchedK,
		done:        make(chan struct{}),
	}
	c.inflight[key] = flight
	c.mu.Unlock()

	// Clone the request to modify TopK without affecting the original.
	modReq := *req
	modReq.TopK = dispatchedK
	result, err := fn(ctx, &modReq)

	flight.result = result
	flight.err = err
	close(flight.done)

	c.mu.Lock()
	delete(c.inflight, key)
	c.mu.Unlock()

	if err != nil {
		return nil, err
	}

	return trimResponse(result, req.TopK), nil
}

func trimResponse(resp *search.SearchResponse, topK int32) *search.SearchResponse {
	if resp == nil || int(topK) >= len(resp.Results) {
		return resp
	}
	trimmed := *resp
	trimmed.Results = make([]search.SearchResult, topK)
	copy(trimmed.Results, resp.Results[:topK])
	return &trimmed
}
