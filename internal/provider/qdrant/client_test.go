package qdrant

import (
	"context"
	"testing"

	pb "github.com/ihgazi/vectorproxy/gen/go/proto/proxy/v1"
	"github.com/qdrant/go-client/qdrant"
	"google.golang.org/protobuf/types/known/structpb"
)

// MockQdrantClient is a manual mock for unit testing
type MockQdrantClient struct {
	QueryResult []*qdrant.ScoredPoint
	QueryErr    error
}

func (m *MockQdrantClient) Query(ctx context.Context, req *qdrant.QueryPoints) ([]*qdrant.ScoredPoint, error) {
	return m.QueryResult, m.QueryErr
}

func (m *MockQdrantClient) Close() error { return nil }

func TestTranslateFilter(t *testing.T) {
	// Create a test payload using structpb
	filterMap := map[string]any{
		"category": "books",
		"status":   "published",
	}
	s, _ := structpb.NewStruct(filterMap)

	filter, err := translateFilter(s)
	if err != nil {
		t.Fatalf("translateFilter failed: %v", err)
	}

	if filter == nil {
		t.Fatal("Expected filter to be non-nil")
	}

	// Verify the conditions (Must should contain 2 conditions)
	if len(filter.Must) != 2 {
		t.Errorf("Expected 2 'Must' conditions, got %d", len(filter.Must))
	}
}

func TestSearchMapping(t *testing.T) {
	// 1. Setup Mock
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

	// 2. Execute Search
	req := &pb.SearchRequest{
		Collection: "test-collection",
		Vector:     []float32{0.1, 0.2, 0.3},
		TopK:       5,
	}

	resp, err := client.Search(context.Background(), req)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	// 3. Verify Mapping
	if len(resp.Results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(resp.Results))
	}

	res := resp.Results[0]
	if res.Id != mockPoint.Id.String() {
		t.Errorf("Expected ID %s, got %s", mockPoint.Id.String(), res.Id)
	}

	if res.Score != mockPoint.Score {
		t.Errorf("Expected Score %f, got %f", mockPoint.Score, res.Score)
	}

	// Verify payload mapping
	payloadName := res.Payload.Fields["name"].GetStringValue()
	if payloadName != "test-item" {
		t.Errorf("Expected payload name 'test-item', got '%s'", payloadName)
	}
}

