package main

import (
	"net"

	pb "github.com/ihgazi/vectorproxy/gen/go/proto/proxy/v1"
	"github.com/ihgazi/vectorproxy/internal/provider"
	"github.com/ihgazi/vectorproxy/internal/server"
	"google.golang.org/grpc"
)

func main() {
	store, _ := provider.NewVectorStore("qdrant", "localhost", 6334)

	lis, _ := net.Listen("tcp", ":50051")
	s := grpc.NewServer()
	pb.RegisterProxyServiceServer(s, server.NewProxyServer(store))

	s.Serve(lis)
}
