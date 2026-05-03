package qdrant

import (
	"context"
	"fmt"

	pb "github.com/ihgazi/vectorproxy/gen/go/proto/proxy/v1"
	"github.com/ihgazi/vectorproxy/internal/engine"
	"github.com/qdrant/go-client/qdrant"

	"google.golang.org/protobuf/types/known/structpb"
)

type QdrantClient interface {
	Query(ctx context.Context, query *qdrant.QueryPoints) ([]*qdrant.ScoredPoint, error)
	Close() error
}

type Client struct {
	qClient QdrantClient
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

	query := &qdrant.QueryPoints{
		CollectionName: req.Collection,
		Query:          qdrant.NewQuery(req.Vector...),
		Limit:          &qryLimit,
		WithPayload:    qdrant.NewWithPayload(true),
	}

	if req.Filter != nil {
		filter, err := translateFilter(req.Filter)
		if err != nil {
			return nil, fmt.Errorf("Failed to translate filter: %v", err)
		}
		query.Filter = filter
	}

	qryResp, err := c.qClient.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("Failed to execute search query: %v", err)
	}

	results := make([]*pb.SearchResult, len(qryResp))
	for i, point := range qryResp {
		var pl *structpb.Struct

		if len(point.Payload) > 0 {
			mp := make(map[string]any)
			for k, v := range point.Payload {
				mp[k] = v.GetStringValue() // TODO: Currently supporting only string payload, need to look into supporting generic payload types
			}

			pl, err = structpb.NewStruct(mp)
			if err != nil {
				// TODO: Implement custom logger
				fmt.Printf("WARNING: Failed to convert payload for ID %s: %v", point.Id, err)
			}
		}

		results[i] = &pb.SearchResult{
			Id:      point.Id.String(),
			Score:   point.Score,
			Payload: pl,
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

// translateFilter maps generic JSON logic to Qdrant's boolean conditions
func translateFilter(s *structpb.Struct) (*qdrant.Filter, error) {
	var conditions []*qdrant.Condition

	for key, value := range s.Fields {
		cond := qdrant.NewMatch(key, value.AsInterface().(string))
		conditions = append(conditions, cond)
	}

	if len(conditions) == 0 {
		return nil, nil
	}

	return &qdrant.Filter{
		Must: conditions,
	}, nil
}
