package middleware

import (
	"context"

	"github.com/ihgazi/vectorproxy/internal/coalesce"
	"github.com/ihgazi/vectorproxy/internal/keygen"
	"github.com/ihgazi/vectorproxy/internal/store"
)

// NewCoalesceInterceptor wraps a search handler with Request Coalescing.
func NewCoalesceInterceptor(generator keygen.KeyGenerator, minK int32) Interceptor {
	c := coalesce.New(generator, minK)
	return func(next SearchHandler) SearchHandler {
		return func(ctx context.Context, req *store.SearchQuery) (*store.SearchResponse, error) {
			return c.Do(ctx, req, func(ctx context.Context, r *store.SearchQuery) (*store.SearchResponse, error) {
				return next(ctx, r)
			})
		}
	}
}
