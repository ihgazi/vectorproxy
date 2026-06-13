package provider

import (
	"fmt"

	"github.com/ihgazi/vectorproxy/internal/provider/qdrant"
	"github.com/ihgazi/vectorproxy/internal/store"
)

func NewVectorStore(provider string, host string, port int) (store.VectorStore, error) {
	switch provider {
	case "qdrant":
		return qdrant.NewClient(host, port)
	default:
		return nil, fmt.Errorf("Unsupported vector store provider: %s", provider)
	}
}
