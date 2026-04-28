package engine

import (
	"context"

	pb "github.com/ihgazi/vectorproxy/gen/go/proto/proxy/v1"
)

// VectorStore defines the interface for a vector database.
// TODO: Implement vector Insert and Delete operations
type VectorStore interface {
	Search(ctx context.Context, req *pb.SearchRequest) (*pb.SearchResponse, error)
	Close() error
}
