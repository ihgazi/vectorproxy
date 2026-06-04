package main

import (
	"log"
	"net"

	pb "github.com/ihgazi/vectorproxy/gen/go/proto/proxy/v1"
	"github.com/ihgazi/vectorproxy/internal/cache"
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

	var cacheInterceptor middleware.Interceptor
	semCache := initializeCache()
	if semCache != nil {
		cacheInterceptor = middleware.NewCacheInterceptor(semCache)
		defer semCache.Close()
	}

	var interceptors []middleware.Interceptor
	interceptors = append(interceptors, middleware.LoggingInterceptor, embedInterceptor)
	if cacheInterceptor != nil {
		interceptors = append(interceptors, cacheInterceptor)
	}

	handler := middleware.Chain(
		store.Search,
		interceptors...,
	)

	lis, _ := net.Listen("tcp", ":50051")
	s := grpc.NewServer()
	pb.RegisterProxyServiceServer(s, server.NewProxyServer(handler))

	reflection.Register(s)

	log.Println("Server is running on port 50051!")
	s.Serve(lis)
}

func initializeCache() cache.SemanticCache {
	cacheCfg := cache.Config{
		Provider:   "redis",
		Host:       "localhost",
		Port:       6379,
		Threshold:  0.95,
		TTLSeconds: 300,
	}
	semCache, err := cache.NewSemanticCache(cacheCfg)
	if err != nil {
		log.Fatalf("Failed to initialize semantic cache: %v", err)

		return nil
	}

	return semCache
}
