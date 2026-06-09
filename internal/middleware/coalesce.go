package middleware

import (
	"context"
	"log"

	"github.com/ihgazi/vectorproxy/internal/keygen"
	"github.com/ihgazi/vectorproxy/internal/search"
	"golang.org/x/sync/singleflight"
)

// NewCoalesceInterceptor wraps a search handler with Request Coalescing.
func NewCoalesceInterceptor(generator keygen.KeyGenerator) Interceptor {
	var group singleflight.Group

	return func(next SearchHandler) SearchHandler {
		return func(ctx context.Context, req *search.SearchQuery) (*search.SearchResponse, error) {
			// Generate a unique key for search query
			key := generator(req)

			if key == "" {
				// Bypass coalescing if no key is generated
				log.Printf("No key generated for query: %s, bypassing coalesing", req.Query)
				return next(ctx, req)
			}

			val, err, shared := group.Do(key, func() (interface{}, error) {
				return next(ctx, req)
			})

			if err != nil {
				return nil, err
			}

			if shared {
				log.Printf("Coalesced request for query: %s", key)
			}

			return val.(*search.SearchResponse), nil
		}
	}
}
