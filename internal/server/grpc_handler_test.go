package server

import (
	"context"
	"errors"
	"testing"

	pb "github.com/ihgazi/vectorproxy/gen/go/proto/proxy/v1"
	"github.com/ihgazi/vectorproxy/internal/middleware"
	"github.com/ihgazi/vectorproxy/internal/search"
)

// A simple mock handler for testing
func mockHandler(resp *search.SearchResponse, err error) middleware.SearchHandler {
	return func(ctx context.Context, req *search.SearchQuery) (*search.SearchResponse, error) {
		return resp, err
	}
}

// MockVectorStore for testing
type MockVectorStore struct {
	Collections []string
	Err         error
}

func (m *MockVectorStore) Search(ctx context.Context, req *search.SearchQuery) (*search.SearchResponse, error) {
	return nil, nil
}
func (m *MockVectorStore) SearchBatch(ctx context.Context, reqs []*search.SearchQuery) ([]*search.SearchResponse, error) {
	return nil, nil
}
func (m *MockVectorStore) ListCollections(ctx context.Context) ([]string, error) {
	return m.Collections, m.Err
}
func (m *MockVectorStore) Close() error { return nil }

func TestProxyServer_Search_HandlerCalled(t *testing.T) {
	expectedResp := search.SearchResponse{Results: []search.SearchResult{{ID: "1"}}}
	handler := mockHandler(&expectedResp, nil)
	store := &MockVectorStore{}
	server := NewProxyServer(handler, store)

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
		return func(ctx context.Context, req *search.SearchQuery) (*search.SearchResponse, error) {
			called = true
			return next(ctx, req)
		}
	}
	expectedResp := search.SearchResponse{Results: []search.SearchResult{{ID: "2"}}}
	handler := middleware.Chain(
		mockHandler(&expectedResp, nil),
		interceptor,
	)
	store := &MockVectorStore{}
	server := NewProxyServer(handler, store)

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
	server := NewProxyServer(handler, store)

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
	server := NewProxyServer(mockHandler(nil, nil), store)

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
