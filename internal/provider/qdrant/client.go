package qdrant

import (
	"context"
	"fmt"

	"github.com/ihgazi/vectorproxy/internal/search"
	"github.com/qdrant/go-client/qdrant"
)

type QdrantClient interface {
	Query(ctx context.Context, query *qdrant.QueryPoints) ([]*qdrant.ScoredPoint, error)
	QueryBatch(ctx context.Context, query *qdrant.QueryBatchPoints) ([]*qdrant.BatchResult, error)
	ListCollections(ctx context.Context) ([]string, error)
	Close() error
}

type Client struct {
	qClient QdrantClient
}

// NewClient initializes a new Qdrant client with the provided host and port.
func NewClient(host string, port int) (search.VectorStore, error) {
	client, err := qdrant.NewClient(&qdrant.Config{
		Host: host,
		Port: port,
	})
	if err != nil {
		return nil, fmt.Errorf("Failed to create Qdrant client: %v", err)
	}
	return &Client{qClient: client}, nil
}

// Search executes a search query against the Qdrant vector store and returns the results.
func (c *Client) Search(ctx context.Context, req *search.SearchQuery) (*search.SearchResponse, error) {
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
			return nil, fmt.Errorf("Failed to translate filter: %v", err)
		}
		query.Filter = filter
	}

	qryResp, err := c.qClient.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("Failed to execute search query: %v", err)
	}

	results := make([]search.SearchResult, len(qryResp))
	for i, point := range qryResp {
		results[i] = buildResult(point)
	}

	resp := search.SearchResponse{
		Results:  results,
		MaxLimit: len(results) < int(topK),
	}
	return &resp, nil
}

// SearchBatch consolidates multiple search queries into a single batch request to Qdrant.
func (c *Client) SearchBatch(ctx context.Context, reqs []*search.SearchQuery) ([]*search.SearchResponse, error) {
	if len(reqs) == 0 {
		return nil, nil
	}

	collection := reqs[0].Collection
	var queryPoints []*qdrant.QueryPoints

	for _, req := range reqs {
		limit := uint64(req.TopK)

		if limit == 0 {
			limit = 10 // Default to limit 10 if not specified
		}

		qp := &qdrant.QueryPoints{
			CollectionName: req.Collection,
			Query:          qdrant.NewQuery(req.Vector...),
			Limit:          &limit,
			WithPayload:    qdrant.NewWithPayload(true),
		}

		if req.Filter != nil {
			if filter, err := translateFilter(req.Filter); err == nil {
				qp.Filter = filter
			}
		}

		queryPoints = append(queryPoints, qp)
	}

	batchReq := &qdrant.QueryBatchPoints{
		CollectionName: collection,
		QueryPoints:    queryPoints,
	}

	batchResp, err := c.qClient.QueryBatch(ctx, batchReq)
	if err != nil {
		return nil, fmt.Errorf("Failed to execute batch search: %v", err)
	}

	responses := make([]*search.SearchResponse, len(batchResp))

	for i, qryResp := range batchResp {
		results := make([]search.SearchResult, len(qryResp.Result))
		for j, point := range qryResp.Result {
			results[j] = buildResult(point)
		}
		responses[i] = &search.SearchResponse{
			Results:  results,
			MaxLimit: len(results) < int(reqs[i].TopK),
		}
	}

	return responses, nil
}

// ListCollections retrieves the list of collection names from the Qdrant vector store.
func (c *Client) ListCollections(ctx context.Context) ([]string, error) {
	collections, err := c.qClient.ListCollections(ctx)
	if err != nil {
		return nil, fmt.Errorf("Failed to list collections: %v", err)
	}

	return collections, nil
}

func (c *Client) Close() error {
	return c.qClient.Close()
}

func buildResult(point *qdrant.ScoredPoint) search.SearchResult {
	var mp map[string]any

	if len(point.Payload) > 0 {
		mp = make(map[string]any)
		for k, v := range point.Payload {
			mp[k] = v.GetStringValue() // TODO: Currently supporting only string payload, need to look into supporting generic payload types
		}
	}

	return search.SearchResult{
		ID:      point.Id.String(),
		Score:   point.Score,
		Payload: mp,
	}
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
