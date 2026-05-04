package server

import (
	"context"

	pb "github.com/ihgazi/vectorproxy/gen/go/proto/proxy/v1"
	"github.com/ihgazi/vectorproxy/internal/middleware"
)

type ProxyServer struct {
	pb.UnimplementedProxyServiceServer
	handler middleware.SearchHandler
}

func NewProxyServer(h middleware.SearchHandler) *ProxyServer {
	return &ProxyServer{handler: h}
}

func (s *ProxyServer) Search(ctx context.Context, req *pb.SearchRequest) (*pb.SearchResponse, error) {
	return s.handler(ctx, req)
}
