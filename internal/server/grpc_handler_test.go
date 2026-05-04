package server

import (
	"context"
	"errors"
	"testing"

	pb "github.com/ihgazi/vectorproxy/gen/go/proto/proxy/v1"
	"github.com/ihgazi/vectorproxy/internal/middleware"
)

// A simple mock handler for testing
func mockHandler(resp *pb.SearchResponse, err error) middleware.SearchHandler {
	return func(ctx context.Context, req *pb.SearchRequest) (*pb.SearchResponse, error) {
		return resp, err
	}
}

func TestProxyServer_Search_HandlerCalled(t *testing.T) {
	expectedResp := &pb.SearchResponse{Results: []*pb.SearchResult{{Id: "1"}}}
	handler := mockHandler(expectedResp, nil)
	server := NewProxyServer(handler)

	resp, err := server.Search(context.Background(), &pb.SearchRequest{})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if resp != expectedResp {
		t.Errorf("expected response %v, got %v", expectedResp, resp)
	}
}

func TestProxyServer_Search_InterceptorCalled(t *testing.T) {
	called := false
	interceptor := func(next middleware.SearchHandler) middleware.SearchHandler {
		return func(ctx context.Context, req *pb.SearchRequest) (*pb.SearchResponse, error) {
			called = true
			return next(ctx, req)
		}
	}
	expectedResp := &pb.SearchResponse{Results: []*pb.SearchResult{{Id: "2"}}}
	handler := middleware.Chain(
		mockHandler(expectedResp, nil),
		interceptor,
	)
	server := NewProxyServer(handler)

	resp, err := server.Search(context.Background(), &pb.SearchRequest{})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !called {
		t.Errorf("expected interceptor to be called")
	}
	if resp != expectedResp {
		t.Errorf("expected response %v, got %v", expectedResp, resp)
	}
}

func TestProxyServer_Search_HandlerError(t *testing.T) {
	expectedErr := errors.New("handler error")
	handler := mockHandler(nil, expectedErr)
	server := NewProxyServer(handler)

	_, err := server.Search(context.Background(), &pb.SearchRequest{})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if err != expectedErr {
		t.Errorf("expected error %v, got %v", expectedErr, err)
	}
}
