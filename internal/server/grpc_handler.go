package server

import (
	"context"
	"fmt"
	"log"
	"time"

	pb "github.com/ihgazi/vectorproxy/gen/go/proto/proxy/v1"
	"github.com/ihgazi/vectorproxy/internal/cache"
	"github.com/ihgazi/vectorproxy/internal/embedding"
	"github.com/ihgazi/vectorproxy/internal/middleware"
	"github.com/ihgazi/vectorproxy/internal/store"

	"google.golang.org/grpc"
	structpb "google.golang.org/protobuf/types/known/structpb"
)

type ProxyServer struct {
	pb.UnimplementedProxyServiceServer
	searchHandler middleware.SearchHandler
	vectorStore   store.VectorStore
	embedder      embedding.Embedder
	cache         cache.SemanticCache
}

func NewProxyServer(sh middleware.SearchHandler, store store.VectorStore, e embedding.Embedder, c cache.SemanticCache) *ProxyServer {
	return &ProxyServer{searchHandler: sh, vectorStore: store, embedder: e, cache: c}
}

func (s *ProxyServer) Search(ctx context.Context, req *pb.SearchRequest) (*pb.SearchResponse, error) {
	qry := protoToDomainQuery(req)

	resp, err := s.searchHandler(ctx, qry)

	if err != nil {
		return nil, err
	}

	return domainToProtoResponse(resp), nil
}

func (s *ProxyServer) ListCollections(ctx context.Context, req *pb.ListCollectionsRequest) (*pb.ListCollectionsResponse, error) {
	collections, err := s.vectorStore.ListCollections(ctx)
	if err != nil {
		return nil, err
	}

	return &pb.ListCollectionsResponse{Collections: collections}, nil
}

func (s *ProxyServer) Upsert(ctx context.Context, req *pb.UpsertRequest) (*pb.UpsertResponse, error) {
	var points []*store.Point

	for _, pbPoint := range req.Points {
		points = append(points, protoToDomainPoint(pbPoint))
	}

	// Embed points that have content but no vector
	if err := s.embedPoints(ctx, points); err != nil {
		return nil, err
	}

	up := &store.UpsertQuery{
		Collection: req.Collection,
		Points:     points,
	}

	if err := s.vectorStore.Upsert(ctx, up); err != nil {
		return nil, err
	}

	s.invalidateCache(req.Collection)

	return &pb.UpsertResponse{}, nil
}

// embedPoints generates vector embeddings for points that have content but no pre-computed vector.
func (s *ProxyServer) embedPoints(ctx context.Context, points []*store.Point) error {
	var texts []string
	var indexes []int

	for i, p := range points {
		if len(p.Vector) == 0 && p.Content != "" {
			texts = append(texts, p.Content)
			indexes = append(indexes, i)
		}
	}

	if len(texts) == 0 {
		return nil
	}

	vectors, err := s.embedder.EmbedBatch(ctx, texts)
	if err != nil {
		return fmt.Errorf("failed to generate batch embeddings: %w", err)
	}
	if len(vectors) != len(texts) {
		return fmt.Errorf("embedding batch size mismatch: expected %d, got %d", len(texts), len(vectors))
	}

	for i, vec := range vectors {
		points[indexes[i]].Vector = vec
	}
	return nil
}

func (s *ProxyServer) DeleteCollection(ctx context.Context, req *pb.DeleteCollectionRequest) (*pb.DeleteCollectionResponse, error) {
	if err := s.vectorStore.DeleteCollection(ctx, req.Collection); err != nil {
		return nil, err
	}

	s.invalidateCache(req.Collection)

	return &pb.DeleteCollectionResponse{}, nil
}

func (s *ProxyServer) DeletePoints(ctx context.Context, req *pb.DeletePointsRequest) (*pb.DeletePointsResponse, error) {
	if err := s.vectorStore.DeletePoints(ctx, req.Collection, req.Ids); err != nil {
		return nil, err
	}

	s.invalidateCache(req.Collection)

	return &pb.DeletePointsResponse{}, nil
}

// invalidateCache asynchronously clears cached entries for a collection.
func (s *ProxyServer) invalidateCache(collection string) {
	if s.cache == nil {
		return
	}
	go func(col string) {
		invCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := s.cache.Invalidate(invCtx, col); err != nil {
			log.Printf("Failed to invalidate cache for collection %s: %v", col, err)
		}
	}(collection)
}

func LoggingInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	start := time.Now()
	log.Printf("Incoming request: Method=%s", info.FullMethod)

	resp, err := handler(ctx, req)

	duration := time.Since(start)
	if err != nil {
		log.Printf("Request %s failed after %v: %v", info.FullMethod, duration, err)
	} else {
		log.Printf("Request %s successful after %v", info.FullMethod, duration)
	}

	return resp, err
}

func protoToDomainQuery(pbreq *pb.SearchRequest) *store.SearchQuery {
	pbFilter := pbreq.Filter.AsMap()
	if pbFilter == nil {
		pbFilter = make(map[string]any)
	}

	searchFilter := make(map[string]string)
	for k, v := range pbFilter {
		strVal, ok := v.(string)
		if !ok {
			log.Printf("Warning: Filter value for key '%s' is not a string, skipping", k)
			continue
		}
		searchFilter[k] = strVal
	}

	return &store.SearchQuery{
		Collection: pbreq.Collection,
		Vector:     pbreq.Vector,
		TopK:       pbreq.TopK,
		Filter:     searchFilter,
		Query:      pbreq.Query,
	}
}

func domainToProtoResponse(dmresp *store.SearchResponse) *pb.SearchResponse {
	protoResults := make([]*pb.SearchResult, len(dmresp.Results))
	for i, res := range dmresp.Results {
		payload, err := structpb.NewStruct(res.Payload)
		if err != nil {
			log.Printf("Failed to convert payload to protobuf struct: %v", err)
		}

		protoResults[i] = &pb.SearchResult{
			Id:      res.ID,
			Score:   res.Score,
			Payload: payload,
		}
	}

	return &pb.SearchResponse{Results: protoResults}
}

func protoToDomainPoint(pbpoint *pb.Point) *store.Point {
	payload := pbpoint.Payload.AsMap()
	if payload == nil {
		payload = make(map[string]any)
	}

	return &store.Point{
		ID:      pbpoint.Id,
		Vector:  pbpoint.Vector,
		Content: pbpoint.Content,
		Payload: payload,
	}
}
