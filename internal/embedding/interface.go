package embedding

import "context"

// Embedder defines the interface for generating vector embeddings
type Embedder interface {
	Embed(ctx context.Context, text string) ([]float32, error)
}
