package qdrant

import (
	"context"
	"fmt"

	pb "github.com/ihgazi/vectorproxy/gen/go/proto/proxy/v1"
	"github.com/ihgazi/vectorproxy/internal/engine"
	"github.com/qdrant/go-client/qdrant"
)

type Client struct {
	qClient *qdrant.Client
}

func NewClient(host string, port int) (engine.VectorStore, error) {
	client, err := qdrant.NewClient(&qdrant.Config{
		Host: host,
		Port: port,
	})
	if err != nil {
		panic(fmt.Sprintf("Failed to create Qdrant client: %v", err))
	}
	return &Client{qClient: client}, nil
}

func (c *Client) Search(ctx context.Context, req *pb.SearchRequest) (*pb.SearchResponse, error) {
	if req.TopK == 0 {
		req.TopK = 10 // Default to limit 10 if not specified
	}

	qryLimit := uint64(req.TopK)
	// TODO: Implement Filter options in search query
	qryResp, err := c.qClient.Query(ctx, &qdrant.QueryPoints{
		CollectionName: req.Collection,
		Query:          qdrant.NewQuery(req.Vector...),
		Limit:          &qryLimit,
	})
	if err != nil {
		return nil, fmt.Errorf("Failed to execute search query: %v", err)
	}

	// TODO: Add Payload field to SearchResult
	results := make([]*pb.SearchResult, len(qryResp))
	for i, point := range qryResp {
		results[i] = &pb.SearchResult{
			Id:    point.Id.String(),
			Score: point.Score,
		}
	}

	resp := &pb.SearchResponse{
		Results: results,
	}
	return resp, nil
}

func (c *Client) Close() error {
	return c.qClient.Close()
}
