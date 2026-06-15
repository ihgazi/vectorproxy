package middleware

import (
	"context"

	"github.com/ihgazi/vectorproxy/internal/store"
)

// SearchHandler is the functional signature for executing a search
type SearchHandler func(ctx context.Context, req *store.SearchQuery) (*store.SearchResponse, error)

// SearchInterceptor is the function that takes a SearchHandler and returns a new SearchHandler
type SearchInterceptor func(SearchHandler) SearchHandler

// SearchChain compiles multiple interceptors into a single SearchHandler
func SearchChain(final SearchHandler, interceptors ...SearchInterceptor) SearchHandler {
	for i := len(interceptors) - 1; i >= 0; i-- {
		final = interceptors[i](final)
	}
	return final
}

// UpsertHandler is the functional signature for executing an upsert
type UpsertHandler func(ctx context.Context, req *store.UpsertQuery) error

// UpsertInterceptor is the function that takes an UpsertHandler and returns a new UpsertHandler
type UpsertInterceptor func(UpsertHandler) UpsertHandler

// UpsertChain compiles multiple interceptors into a single UpsertHandler
func UpsertChain(final UpsertHandler, interceptors ...UpsertInterceptor) UpsertHandler {
	for i := len(interceptors) - 1; i >= 0; i-- {
		final = interceptors[i](final)
	}
	return final
}
