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
func mockHandler(resp search.SearchResponse, err error) middleware.SearchHandler {
	return func(ctx context.Context, req search.SearchQuery) (search.SearchResponse, error) {
		return resp, err
	}
}

func TestProxyServer_Search_HandlerCalled(t *testing.T) {
	expectedResp := search.SearchResponse{Results: []search.SearchResult{{ID: "1"}}}
	handler := mockHandler(expectedResp, nil)
	server := NewProxyServer(handler)

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
		return func(ctx context.Context, req search.SearchQuery) (search.SearchResponse, error) {
			called = true
			return next(ctx, req)
		}
	}
	expectedResp := search.SearchResponse{Results: []search.SearchResult{{ID: "2"}}}
	handler := middleware.Chain(
		mockHandler(expectedResp, nil),
		interceptor,
	)
	server := NewProxyServer(handler)

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
	handler := mockHandler(search.SearchResponse{}, expectedErr)
	server := NewProxyServer(handler)

	req := &pb.SearchRequest{Collection: "test"}
	_, err := server.Search(context.Background(), req)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if err != expectedErr {
		t.Errorf("expected error %v, got %v", expectedErr, err)
	}
}
