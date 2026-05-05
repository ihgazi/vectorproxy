package middleware

import (
	"context"

	"github.com/ihgazi/vectorproxy/internal/embedding"
	"github.com/ihgazi/vectorproxy/internal/search"
)

func NewEmbeddingInterceptor(e embedding.Embedder) Interceptor {
	return func(next SearchHandler) SearchHandler {
		return func(ctx context.Context, req *search.SearchQuery) (*search.SearchResponse, error) {
			if len(req.Vector) == 0 && req.Query != "" {
				vec, err := e.Embed(ctx, req.Query)
				if err != nil {
					return nil, err
				}

				// Inject generated vector into search query
				req.Vector = vec
			}

			return next(ctx, req)
		}
	}
}
