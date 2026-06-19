package main

import (
	"log"
	"net"
	"net/http"
	"time"

	pb "github.com/ihgazi/vectorproxy/gen/go/proto/proxy/v1"
	"github.com/ihgazi/vectorproxy/internal/batch"
	"github.com/ihgazi/vectorproxy/internal/cache"
	"github.com/ihgazi/vectorproxy/internal/embedding"
	"github.com/ihgazi/vectorproxy/internal/keygen"
	"github.com/ihgazi/vectorproxy/internal/middleware"
	"github.com/ihgazi/vectorproxy/internal/provider"
	"github.com/ihgazi/vectorproxy/internal/server"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func main() {
	store, _ := provider.NewVectorStore("qdrant", "localhost", 6334)

	baseEmbedder := embedding.NewOllamaEmbedder("http://localhost:11434", "nomic-embed-text")
	embedder, err := embedding.NewCachedEmbedder(baseEmbedder, 10000)
	if err != nil {
		log.Fatalf("failed to initialize exact string cache: %v", err)
	}
	embedInterceptor := middleware.NewEmbeddingInterceptor(embedder)

	var searchCacheInterceptor middleware.Interceptor
	semCache := initializeCache()
	if semCache != nil {
		searchCacheInterceptor = middleware.NewCacheInterceptor(semCache)
		defer semCache.Close()
	}

	// TODO: Make MinK value configurable
	vectorCoalescer := middleware.NewCoalesceInterceptor(keygen.NewVectorKeyGenerator(), 10)

	// Assemble Search pipeline in execution order:
	// Logging -> Embedding -> Request Coalescing -> Cache -> DB Search
	var interceptors []middleware.Interceptor
	interceptors = append(interceptors,
		embedInterceptor,
		vectorCoalescer,
	)
	if searchCacheInterceptor != nil {
		interceptors = append(interceptors, searchCacheInterceptor)
	}

	batchSize := 10 // TODO: Make this configurable
	batcher := batch.New(store.SearchBatch, batchSize, 100*time.Millisecond)
	go batcher.FlushLoop()

	searchHandler := middleware.Chain(
		batcher.Search,
		interceptors...,
	)

	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("failed to listen on port 50051: %v", err)
	}

	go func() {
		http.Handle("/metrics", promhttp.Handler())
		log.Println("Metrics server running on port 50052")
		if err := http.ListenAndServe(":50052", nil); err != nil {
			log.Fatalf("failed to start metrics server: %v", err)
		}
	}()

	s := grpc.NewServer(
		grpc.UnaryInterceptor(server.LoggingInterceptor),
	)
	pb.RegisterProxyServiceServer(s, server.NewProxyServer(searchHandler, store, embedder, semCache))

	reflection.Register(s)

	log.Println("Server is running on port 50051!")
	s.Serve(lis)
}

func initializeCache() cache.SemanticCache {
	cacheCfg := cache.Config{
		Provider:   "redis",
		Host:       "localhost",
		Port:       6379,
		Threshold:  0.90,
		TTLSeconds: 300,
	}
	semCache, err := cache.NewSemanticCache(cacheCfg)
	if err != nil {
		log.Printf("Failed to initialize semantic cache: %v", err)

		return nil
	}

	return semCache
}
