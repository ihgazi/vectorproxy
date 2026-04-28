package server

import (
	"context"

	pb "github.com/ihgazi/vectorproxy/gen/go/proto/proxy/v1"
	"github.com/ihgazi/vectorproxy/internal/engine"
)

type ProxyServer struct {
	pb.UnimplementedProxyServiceServer
	store engine.VectorStore
}

func NewProxyServer(store engine.VectorStore) *ProxyServer {
	return &ProxyServer{store: store}
}

func (s *ProxyServer) Search(ctx context.Context, req *pb.SearchRequest) (*pb.SearchResponse, error) {
	return s.store.Search(ctx, req)
}
