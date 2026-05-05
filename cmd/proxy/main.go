package main

import (
	"log"
	"net"

	pb "github.com/ihgazi/vectorproxy/gen/go/proto/proxy/v1"
	"github.com/ihgazi/vectorproxy/internal/embedding"
	"github.com/ihgazi/vectorproxy/internal/middleware"
	"github.com/ihgazi/vectorproxy/internal/provider"
	"github.com/ihgazi/vectorproxy/internal/server"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func main() {
	store, _ := provider.NewVectorStore("qdrant", "localhost", 6334)

	embedInterceptor := middleware.NewEmbeddingInterceptor(embedding.NewOllamaEmbedder("http://localhost:11434", "nomic-embed-text"))

	handler := middleware.Chain(
		store.Search,
		embedInterceptor,
		middleware.LoggingInterceptor,
	)

	lis, _ := net.Listen("tcp", ":50051")
	s := grpc.NewServer()
	pb.RegisterProxyServiceServer(s, server.NewProxyServer(handler))

	reflection.Register(s)

	log.Println("Server is running on port 50051!")
	s.Serve(lis)
}
