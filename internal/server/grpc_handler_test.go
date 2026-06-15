package server

import (
	"context"
	"errors"
	"testing"

	pb "github.com/ihgazi/vectorproxy/gen/go/proto/proxy/v1"
	"github.com/ihgazi/vectorproxy/internal/middleware"
	"github.com/ihgazi/vectorproxy/internal/store"
	"google.golang.org/grpc"
)

// A simple mock handler for testing
func mockHandler(resp *store.SearchResponse, err error) middleware.SearchHandler {
	return func(ctx context.Context, req *store.SearchQuery) (*store.SearchResponse, error) {
		return resp, err
	}
}

// MockEmbedder for testing
type MockEmbedder struct{}

func (m *MockEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	return []float32{0.1, 0.2, 0.3}, nil
}
func (m *MockEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	vectors := make([][]float32, len(texts))
	for i := range texts {
		vectors[i] = []float32{0.1, 0.2, 0.3}
	}
	return vectors, nil
}

// MockVectorStore for testing
type MockVectorStore struct {
	Collections []string
	Err         error
}

func (m *MockVectorStore) Search(ctx context.Context, req *store.SearchQuery) (*store.SearchResponse, error) {
	return nil, nil
}
func (m *MockVectorStore) SearchBatch(ctx context.Context, reqs []*store.SearchQuery) ([]*store.SearchResponse, error) {
	return nil, nil
}
func (m *MockVectorStore) ListCollections(ctx context.Context) ([]string, error) {
	return m.Collections, m.Err
}
func (m *MockVectorStore) Upsert(ctx context.Context, req *store.UpsertQuery) error {
	return nil
}
func (m *MockVectorStore) DeleteCollection(ctx context.Context, collection string) error {
	return m.Err
}
func (m *MockVectorStore) DeletePoints(ctx context.Context, collection string, ids []string) error {
	return m.Err
}
func (m *MockVectorStore) Close() error { return nil }

func TestProxyServer_Search_HandlerCalled(t *testing.T) {
	expectedResp := store.SearchResponse{Results: []store.SearchResult{{ID: "1"}}}
	handler := mockHandler(&expectedResp, nil)
	store := &MockVectorStore{}
	server := NewProxyServer(handler, store, &MockEmbedder{}, nil)

	req := &pb.SearchRequest{Collection: "test"}
	resp, err := server.Search(context.Background(), req)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	expectedProto := domainToProtoResponse(&expectedResp)
	if resp.String() != expectedProto.String() {
		t.Errorf("expected response %v, got %v", expectedProto, resp)
	}
}

func TestProxyServer_Search_InterceptorCalled(t *testing.T) {
	called := false
	interceptor := func(next middleware.SearchHandler) middleware.SearchHandler {
		return func(ctx context.Context, req *store.SearchQuery) (*store.SearchResponse, error) {
			called = true
			return next(ctx, req)
		}
	}
	expectedResp := store.SearchResponse{Results: []store.SearchResult{{ID: "2"}}}
	handler := middleware.Chain(
		mockHandler(&expectedResp, nil),
		interceptor,
	)
	store := &MockVectorStore{}
	server := NewProxyServer(handler, store, &MockEmbedder{}, nil)

	req := &pb.SearchRequest{Collection: "test"}
	resp, err := server.Search(context.Background(), req)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !called {
		t.Errorf("expected interceptor to be called")
	}
	expectedProto := domainToProtoResponse(&expectedResp)
	if resp.String() != expectedProto.String() {
		t.Errorf("expected response %v, got %v", expectedProto, resp)
	}
}

func TestProxyServer_Search_HandlerError(t *testing.T) {
	expectedErr := errors.New("handler error")
	handler := mockHandler(nil, expectedErr)
	store := &MockVectorStore{}
	server := NewProxyServer(handler, store, &MockEmbedder{}, nil)

	req := &pb.SearchRequest{Collection: "test"}
	_, err := server.Search(context.Background(), req)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if err != expectedErr {
		t.Errorf("expected error %v, got %v", expectedErr, err)
	}
}

func TestProxyServer_ListCollections(t *testing.T) {
	expectedCollections := []string{"collection1", "collection2"}
	store := &MockVectorStore{Collections: expectedCollections}
	server := NewProxyServer(mockHandler(nil, nil), store, &MockEmbedder{}, nil)

	req := &pb.ListCollectionsRequest{}
	resp, err := server.ListCollections(context.Background(), req)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(resp.Collections) != 2 {
		t.Fatalf("expected 2 collections, got %d", len(resp.Collections))
	}
	if resp.Collections[0] != "collection1" || resp.Collections[1] != "collection2" {
		t.Errorf("expected %v, got %v", expectedCollections, resp.Collections)
	}
}

func TestLoggingInterceptor(t *testing.T) {
	expectedResp := "test response"
	expectedErr := errors.New("test error")

	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return expectedResp, expectedErr
	}

	info := &grpc.UnaryServerInfo{
		FullMethod: "/proxy.v1.ProxyService/Test",
	}

	resp, err := LoggingInterceptor(context.Background(), nil, info, handler)

	if resp != expectedResp {
		t.Errorf("expected response %v, got %v", expectedResp, resp)
	}
	if err != expectedErr {
		t.Errorf("expected error %v, got %v", expectedErr, err)
	}
}

func TestProxyServer_Upsert(t *testing.T) {
	store := &MockVectorStore{}
	server := NewProxyServer(mockHandler(nil, nil), store, &MockEmbedder{}, nil)

	req := &pb.UpsertRequest{
		Collection: "test",
		Points: []*pb.Point{
			{Id: "1", Content: "foo"},
			{Id: "2", Content: "bar"},
		},
	}
	_, err := server.Upsert(context.Background(), req)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestProxyServer_DeleteCollection(t *testing.T) {
	store := &MockVectorStore{}
	server := NewProxyServer(mockHandler(nil, nil), store, &MockEmbedder{}, nil)

	req := &pb.DeleteCollectionRequest{Collection: "test"}
	_, err := server.DeleteCollection(context.Background(), req)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestProxyServer_DeletePoints(t *testing.T) {
	store := &MockVectorStore{}
	server := NewProxyServer(mockHandler(nil, nil), store, &MockEmbedder{}, nil)

	req := &pb.DeletePointsRequest{
		Collection: "test",
		Ids:        []string{"1", "2", "3"},
	}
	_, err := server.DeletePoints(context.Background(), req)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}
