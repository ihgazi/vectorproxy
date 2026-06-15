package middleware

import (
	"context"

	"github.com/ihgazi/vectorproxy/internal/store"
)

// SearchHandler is the functional signature for executing a search
type SearchHandler func(ctx context.Context, req *store.SearchQuery) (*store.SearchResponse, error)

// Interceptor is the function that takes a SearchHandler and returns a new SearchHandler
type Interceptor func(SearchHandler) SearchHandler

// Chain compiles multiple interceptors into a single SearchHandler
func Chain(final SearchHandler, interceptors ...Interceptor) SearchHandler {
	for i := len(interceptors) - 1; i >= 0; i-- {
		final = interceptors[i](final)
	}
	return final
}
