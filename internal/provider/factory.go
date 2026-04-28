package provider

import (
	"fmt"

	"github.com/ihgazi/vectorproxy/internal/engine"
	"github.com/ihgazi/vectorproxy/internal/provider/qdrant"
)

func NewVectorStore(provider string, host string, port int) (engine.VectorStore, error) {
	switch provider {
	case "qdrant":
		return qdrant.NewClient(host, port)
	default:
		return nil, fmt.Errorf("Unsupported vector store provider: %s", provider)
	}
}
