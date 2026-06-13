package middleware

import (
	"context"
	"log"

	"github.com/ihgazi/vectorproxy/internal/cache"
	"github.com/ihgazi/vectorproxy/internal/store"
)

// NewCacheInterceptor creates a SearchHandler middleware that hooks up the SemanticCache plugin.
func NewCacheInterceptor(c cache.SemanticCache) Interceptor {
	return func(next SearchHandler) SearchHandler {
		return func(ctx context.Context, req *store.SearchQuery) (*store.SearchResponse, error) {
			// A query vector is required to perform semantic cache lookups.
			// If no vector is present, bypass the cache and fall back.
			if len(req.Vector) == 0 {
				return next(ctx, req)
			}

			cachedResp, hit, err := c.Get(ctx, req.Collection, req.Vector, req.TopK)
			if err != nil {
				// Fail open on cache failures to guarantee service availability
				log.Printf("Semantic cache lookup failed: %v. Bypassing to DB.", err)
			} else if hit {
				log.Printf("Semantic cache HIT for collection: %s", req.Collection)
				return cachedResp, nil
			}

			log.Printf("Semantic cache MISS for collection: %s", req.Collection)

			// Cache Miss: Execute actual database search
			dbResp, err := next(ctx, req)
			if err != nil {
				return nil, err
			}

			// Populate cache with query and database response
			if dbResp != nil {
				log.Printf("Populating semantic cache for collection: %s", req.Collection)
				if setErr := c.Set(ctx, req.Collection, req.Vector, dbResp); setErr != nil {
					log.Printf("Failed to populate semantic cache: %v", setErr)
				}
			}

			return dbResp, nil
		}
	}
}
