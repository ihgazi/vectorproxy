package server

import (
	"context"
	"log"
	"time"

	pb "github.com/ihgazi/vectorproxy/gen/go/proto/proxy/v1"
	"github.com/ihgazi/vectorproxy/internal/middleware"
	"github.com/ihgazi/vectorproxy/internal/store"

	"google.golang.org/grpc"
	structpb "google.golang.org/protobuf/types/known/structpb"
)

type ProxyServer struct {
	pb.UnimplementedProxyServiceServer
	handler     middleware.SearchHandler
	vectorStore store.VectorStore
}

func NewProxyServer(h middleware.SearchHandler, store store.VectorStore) *ProxyServer {
	return &ProxyServer{handler: h, vectorStore: store}
}

func (s *ProxyServer) Search(ctx context.Context, req *pb.SearchRequest) (*pb.SearchResponse, error) {
	qry := protoToDomainQuery(req)

	resp, err := s.handler(ctx, qry)

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

	up := &store.UpsertQuery{
		Collection: req.Collection,
		Points:     points,
	}

	err := s.vectorStore.Upsert(ctx, up)
	if err != nil {
		return nil, err
	}

	return &pb.UpsertResponse{}, nil
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
