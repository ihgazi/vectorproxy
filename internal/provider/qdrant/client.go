package qdrant

import (
	"context"
	"fmt"

	"github.com/ihgazi/vectorproxy/internal/engine"
	"github.com/ihgazi/vectorproxy/internal/search"
	"github.com/qdrant/go-client/qdrant"
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
		return nil, fmt.Errorf("Failed to create Qdrant client: %v", err)
	}
	return &Client{qClient: client}, nil
}

func (c *Client) Search(ctx context.Context, req search.SearchQuery) (search.SearchResponse, error) {
	topK := req.TopK
	if topK == 0 {
		topK = 10 // Default to limit 10 if not specified
	}

	qryLimit := uint64(topK)

	query := &qdrant.QueryPoints{
		CollectionName: req.Collection,
		Query:          qdrant.NewQuery(req.Vector...),
		Limit:          &qryLimit,
		WithPayload:    qdrant.NewWithPayload(true),
	}

	if req.Filter != nil {
		filter, err := translateFilter(req.Filter)
		if err != nil {
			return search.SearchResponse{}, fmt.Errorf("Failed to translate filter: %v", err)
		}
		query.Filter = filter
	}

	qryResp, err := c.qClient.Query(ctx, query)
	if err != nil {
		return search.SearchResponse{}, fmt.Errorf("Failed to execute search query: %v", err)
	}

	results := make([]search.SearchResult, len(qryResp))
	for i, point := range qryResp {
		var mp map[string]any

		if len(point.Payload) > 0 {
			mp = make(map[string]any)
			for k, v := range point.Payload {
				mp[k] = v.GetStringValue() // TODO: Currently supporting only string payload, need to look into supporting generic payload types
			}
		}

		results[i] = search.SearchResult{
			ID:      point.Id.String(),
			Score:   point.Score,
			Payload: mp,
		}
	}

	resp := search.SearchResponse{
		Results: results,
	}
	return resp, nil
}

func (c *Client) Close() error {
	return c.qClient.Close()
}

// translateFilter maps generic JSON logic to Qdrant's boolean conditions
func translateFilter(mp map[string]string) (*qdrant.Filter, error) {
	var conditions []*qdrant.Condition

	for key, value := range mp {
		cond := qdrant.NewMatch(key, value)
		conditions = append(conditions, cond)
	}

	if len(conditions) == 0 {
		return nil, nil
	}

	return &qdrant.Filter{
		Must: conditions,
	}, nil
}
