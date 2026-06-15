package middleware

import (
	"context"
	"fmt"
	"log"

	"github.com/ihgazi/vectorproxy/internal/embedding"
	"github.com/ihgazi/vectorproxy/internal/store"
	"golang.org/x/sync/singleflight"
)

// TODO: Implement string caching on embedding results
func NewEmbeddingInterceptor(e embedding.Embedder) SearchInterceptor {
	var group singleflight.Group

	return func(next SearchHandler) SearchHandler {
		return func(ctx context.Context, req *store.SearchQuery) (*store.SearchResponse, error) {
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

// NewUpsertEmbeddingInterceptor extracts texts from Upsert Points and embeds them in a single batch
func NewUpsertEmbeddingInterceptor(e embedding.Embedder) UpsertInterceptor {
	return func(next UpsertHandler) UpsertHandler {
		return func(ctx context.Context, req *store.UpsertQuery) error {
			var texts []string
			var pointIdxs []int

			for i, p := range req.Points {
				if len(p.Vector) == 0 && p.Content != "" {
					texts = append(texts, p.Content)
					pointIdxs = append(pointIdxs, i)
				}
			}

			if len(texts) > 0 {
				vectors, err := e.EmbedBatch(ctx, texts)
				if err != nil {
					return fmt.Errorf("failed to generate batch embeddings: %w", err)
				}
				if len(vectors) != len(texts) {
					return fmt.Errorf("embedding batch size mismatch")
				}
				for i, vec := range vectors {
					req.Points[pointIdxs[i]].Vector = vec
				}
			}

			return next(ctx, req)
		}
	}
}
