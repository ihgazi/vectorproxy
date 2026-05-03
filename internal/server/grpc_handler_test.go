package server

import (
	"context"
	"errors"
	"testing"

	pb "github.com/ihgazi/vectorproxy/gen/go/proto/proxy/v1"
)

type mockVectorStore struct {
	searchFunc func(ctx context.Context, req *pb.SearchRequest) (*pb.SearchResponse, error)
	closeFunc  func() error
}

func (m *mockVectorStore) Search(ctx context.Context, req *pb.SearchRequest) (*pb.SearchResponse, error) {
	return m.searchFunc(ctx, req)
}
func (m *mockVectorStore) Close() error {
	return m.closeFunc()
}

func TestProxyServer_Search_Success(t *testing.T) {
	mockVS := &mockVectorStore{
		searchFunc: func(ctx context.Context, req *pb.SearchRequest) (*pb.SearchResponse, error) {
			return &pb.SearchResponse{Results: []*pb.SearchResult{{Id: "1"}}}, nil
		},
		closeFunc: func() error { return nil },
	}
	s := &ProxyServer{store: mockVS}

	resp, err := s.Search(context.Background(), &pb.SearchRequest{})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(resp.Results) != 1 {
		t.Errorf("expected 1 result, got %d", len(resp.Results))
	}
}

func TestProxyServer_Search_Error(t *testing.T) {
	mockVS := &mockVectorStore{
		searchFunc: func(ctx context.Context, req *pb.SearchRequest) (*pb.SearchResponse, error) {
			return nil, errors.New("search error")
		},
		closeFunc: func() error { return nil },
	}
	s := &ProxyServer{store: mockVS}

	_, err := s.Search(context.Background(), &pb.SearchRequest{})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
}

