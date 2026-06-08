package middleware

import (
	"context"
	"log"

	"github.com/ihgazi/vectorproxy/internal/embedding"
	"github.com/ihgazi/vectorproxy/internal/search"
	"golang.org/x/sync/singleflight"
)

// TODO: Implement string caching on embedding results
func NewEmbeddingInterceptor(e embedding.Embedder) Interceptor {
	var group singleflight.Group

	return func(next SearchHandler) SearchHandler {
		return func(ctx context.Context, req *search.SearchQuery) (*search.SearchResponse, error) {
			if len(req.Vector) == 0 && req.Query != "" {
				val, err, shared := group.Do(req.Query, func() (interface{}, error) {
					return e.Embed(ctx, req.Query)
				})

				if err != nil {
					return nil, err
				}

				if shared {
					log.Printf("Duplicate embedding for query: %s", req.Query)
				}

				// Inject generated vector into search query
				req.Vector = val.([]float32)
			}

			return next(ctx, req)
		}
	}
}
