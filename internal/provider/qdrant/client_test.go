package qdrant

import (
	"context"
	"testing"

	"github.com/ihgazi/vectorproxy/internal/search"
	"github.com/qdrant/go-client/qdrant"
)

// MockQdrantClient is a manual mock for unit testing
type MockQdrantClient struct {
	QueryResult []*qdrant.ScoredPoint
	QueryErr    error
}

func (m *MockQdrantClient) Query(ctx context.Context, req *qdrant.QueryPoints) ([]*qdrant.ScoredPoint, error) {
	return m.QueryResult, m.QueryErr
}
func (m *MockQdrantClient) QueryBatch(ctx context.Context, req *qdrant.QueryBatchPoints) ([]*qdrant.BatchResult, error) {
	return nil, nil // Dummy implementation for now
}
func (m *MockQdrantClient) Close() error { return nil }

func TestTranslateFilter(t *testing.T) {
	filterMap := map[string]string{
		"category": "books",
		"status":   "published",
	}
	filter, err := translateFilter(filterMap)
	if err != nil {
		t.Fatalf("translateFilter failed: %v", err)
	}
	if filter == nil {
		t.Fatal("Expected filter to be non-nil")
	}
	if len(filter.Must) != 2 {
		t.Errorf("Expected 2 'Must' conditions, got %d", len(filter.Must))
	}
}

func TestSearchMapping(t *testing.T) {
	mockPoint := &qdrant.ScoredPoint{
		Id:    qdrant.NewIDUUID("550e8400-e29b-41d4-a716-446655440000"),
		Score: 0.95,
		Payload: map[string]*qdrant.Value{
			"name": qdrant.NewValueString("test-item"),
		},
	}
	mockClient := &MockQdrantClient{
		QueryResult: []*qdrant.ScoredPoint{mockPoint},
	}
	client := &Client{qClient: mockClient}

	req := search.SearchQuery{
		Collection: "test-collection",
		Vector:     []float32{0.1, 0.2, 0.3},
		TopK:       5,
	}

	resp, err := client.Search(context.Background(), &req)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if len(resp.Results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(resp.Results))
	}

	res := resp.Results[0]
	if res.ID != mockPoint.Id.String() {
		t.Errorf("Expected ID %s, got %s", mockPoint.Id.String(), res.ID)
	}
	if res.Score != mockPoint.Score {
		t.Errorf("Expected Score %f, got %f", mockPoint.Score, res.Score)
	}
	payloadName, ok := res.Payload["name"].(string)
	if !ok || payloadName != "test-item" {
		t.Errorf("Expected payload name 'test-item', got '%v'", res.Payload["name"])
	}
}
