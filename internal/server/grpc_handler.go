package server

import (
	"context"
	"log"

	pb "github.com/ihgazi/vectorproxy/gen/go/proto/proxy/v1"
	"github.com/ihgazi/vectorproxy/internal/middleware"
	"github.com/ihgazi/vectorproxy/internal/search"

	structpb "google.golang.org/protobuf/types/known/structpb"
)

type ProxyServer struct {
	pb.UnimplementedProxyServiceServer
	handler middleware.SearchHandler
}

func NewProxyServer(h middleware.SearchHandler) *ProxyServer {
	return &ProxyServer{handler: h}
}

func (s *ProxyServer) Search(ctx context.Context, req *pb.SearchRequest) (*pb.SearchResponse, error) {
	qry := protoToDomainQuery(req)

	resp, err := s.handler(ctx, qry)

	if err != nil {
		return nil, err
	}

	return domainToProtoResponse(&resp), nil
}

func protoToDomainQuery(pbreq *pb.SearchRequest) search.SearchQuery {
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

	return search.SearchQuery{
		Collection: pbreq.Collection,
		Vector:     pbreq.Vector,
		TopK:       pbreq.TopK,
		Filter:     searchFilter,
	}
}

func domainToProtoResponse(dmresp *search.SearchResponse) *pb.SearchResponse {
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
