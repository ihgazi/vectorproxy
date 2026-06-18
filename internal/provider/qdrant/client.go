package qdrant

import (
	"context"
	"fmt"
	"time"

	"github.com/ihgazi/vectorproxy/internal/metrics"
	"github.com/ihgazi/vectorproxy/internal/store"
	"github.com/qdrant/go-client/qdrant"
)

type QdrantClient interface {
	Query(ctx context.Context, query *qdrant.QueryPoints) ([]*qdrant.ScoredPoint, error)
	QueryBatch(ctx context.Context, query *qdrant.QueryBatchPoints) ([]*qdrant.BatchResult, error)
	ListCollections(ctx context.Context) ([]string, error)
	CollectionExists(ctx context.Context, collectionName string) (bool, error)
	CreateCollection(ctx context.Context, req *qdrant.CreateCollection) error
	DeleteCollection(ctx context.Context, collectionName string) error
	Upsert(ctx context.Context, request *qdrant.UpsertPoints) (*qdrant.UpdateResult, error)
	Delete(ctx context.Context, request *qdrant.DeletePoints) (*qdrant.UpdateResult, error)
	Close() error
}

type Client struct {
	qClient QdrantClient
}

// NewClient initializes a new Qdrant client with the provided host and port.
func NewClient(host string, port int) (store.VectorStore, error) {
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
func (c *Client) Search(ctx context.Context, req *store.SearchQuery) (*store.SearchResponse, error) {
	start := time.Now()
	defer func() {
		metrics.UpstreamDuration.WithLabelValues("qdrant", "search").Observe(time.Since(start).Seconds())
	}()

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

	results := make([]store.SearchResult, len(qryResp))
	for i, point := range qryResp {
		results[i] = buildResult(point)
	}

	resp := store.SearchResponse{
		Results:  results,
		MaxLimit: len(results) < int(topK),
	}
	return &resp, nil
}

// SearchBatch consolidates multiple search queries into a single batch request to Qdrant.
func (c *Client) SearchBatch(ctx context.Context, reqs []*store.SearchQuery) ([]*store.SearchResponse, error) {
	start := time.Now()
	defer func() {
		metrics.UpstreamDuration.WithLabelValues("qdrant", "search_batch").Observe(time.Since(start).Seconds())
	}()

	if len(reqs) == 0 {
		return nil, nil
	}

	metrics.BatchSize.WithLabelValues("SearchBatch").Observe(float64(len(reqs)))

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

	responses := make([]*store.SearchResponse, len(batchResp))

	for i, qryResp := range batchResp {
		results := make([]store.SearchResult, len(qryResp.Result))
		for j, point := range qryResp.Result {
			results[j] = buildResult(point)
		}
		responses[i] = &store.SearchResponse{
			Results:  results,
			MaxLimit: len(results) < int(reqs[i].TopK),
		}
	}

	return responses, nil
}

// ListCollections retrieves the list of collection names from the Qdrant vector store.
func (c *Client) ListCollections(ctx context.Context) ([]string, error) {
	start := time.Now()
	defer func() {
		metrics.UpstreamDuration.WithLabelValues("qdrant", "list_collections").Observe(time.Since(start).Seconds())
	}()

	collections, err := c.qClient.ListCollections(ctx)
	if err != nil {
		return nil, fmt.Errorf("Failed to list collections: %v", err)
	}

	return collections, nil
}

func (c *Client) Close() error {
	return c.qClient.Close()
}

func (c *Client) Upsert(ctx context.Context, req *store.UpsertQuery) error {
	start := time.Now()
	defer func() {
		metrics.UpstreamDuration.WithLabelValues("qdrant", "upsert").Observe(time.Since(start).Seconds())
	}()

	metrics.BatchSize.WithLabelValues("Upsert").Observe(float64(len(req.Points)))

	exists, err := c.qClient.CollectionExists(ctx, req.Collection)
	if err != nil {
		return fmt.Errorf("Failed to check collection existence: %v", err)
	}

	if !exists {
		var dimension uint64
		if len(req.Points) > 0 && len(req.Points[0].Vector) > 0 {
			dimension = uint64(len(req.Points[0].Vector))
		} else {
			return fmt.Errorf("Cannot create collection: vector dimension unknown")
		}

		if err := c.qClient.CreateCollection(ctx, &qdrant.CreateCollection{
			CollectionName: req.Collection,
			VectorsConfig: qdrant.NewVectorsConfig(&qdrant.VectorParams{
				Size:     dimension,
				Distance: qdrant.Distance_Cosine,
			}),
		}); err != nil {
			return fmt.Errorf("Failed to auto-create collection: %v", err)
		}
	}

	// Qdrant Upsert API is asynchronous by default.
	// We set Wait to true to ensure the operation completes before returning.
	wait := true
	up := &qdrant.UpsertPoints{
		CollectionName: req.Collection,
		Wait:           &wait,
		Points:         buildPoints(req.Points),
	}

	resp, err := c.qClient.Upsert(ctx, up)
	if err != nil {
		return fmt.Errorf("Failed to upsert points: %v", err)
	}

	if resp.Status != qdrant.UpdateStatus_Completed {
		return fmt.Errorf("Failed to upsert points: Qdrant responded with status %s", resp.Status.String())
	}

	return nil
}

// DeleteCollection deletes an entire collection from the Qdrant vector store.
func (c *Client) DeleteCollection(ctx context.Context, collection string) error {
	start := time.Now()
	defer func() {
		metrics.UpstreamDuration.WithLabelValues("qdrant", "delete_collection").Observe(time.Since(start).Seconds())
	}()

	if err := c.qClient.DeleteCollection(ctx, collection); err != nil {
		return fmt.Errorf("Failed to delete collection: %v", err)
	}
	return nil
}

// DeletePoints deletes specific points by ID from a collection in the Qdrant vector store.
func (c *Client) DeletePoints(ctx context.Context, collection string, ids []string) error {
	start := time.Now()
	defer func() {
		metrics.UpstreamDuration.WithLabelValues("qdrant", "delete_points").Observe(time.Since(start).Seconds())
	}()

	pointIDs := make([]*qdrant.PointId, len(ids))
	for i, id := range ids {
		pointIDs[i] = qdrant.NewID(id)
	}

	wait := true
	resp, err := c.qClient.Delete(ctx, &qdrant.DeletePoints{
		CollectionName: collection,
		Wait:           &wait,
		Points:         qdrant.NewPointsSelector(pointIDs...),
	})
	if err != nil {
		return fmt.Errorf("Failed to delete points: %v", err)
	}

	if resp.Status != qdrant.UpdateStatus_Completed {
		return fmt.Errorf("Failed to delete points: Qdrant responded with status %s", resp.Status.String())
	}
	return nil
}

func buildResult(point *qdrant.ScoredPoint) store.SearchResult {
	var mp map[string]any

	if len(point.Payload) > 0 {
		mp = make(map[string]any)
		for k, v := range point.Payload {
			mp[k] = v.GetStringValue() // TODO: Currently supporting only string payload, need to look into supporting generic payload types
		}
	}

	return store.SearchResult{
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

func buildPoints(points []*store.Point) []*qdrant.PointStruct {
	var res []*qdrant.PointStruct

	for _, p := range points {
		payload := make(map[string]*qdrant.Value)

		payload["content"] = qdrant.NewValueString(p.Content)

		for k, v := range p.Payload {
			payload[k] = qdrant.NewValueString(fmt.Sprintf("%v", v)) // TODO: Currently supporting only string payload, need to look into supporting generic payload types
		}

		res = append(res, &qdrant.PointStruct{
			Id:      qdrant.NewID(p.ID),
			Vectors: qdrant.NewVectors(p.Vector...),
			Payload: payload,
		})
	}

	return res
}
