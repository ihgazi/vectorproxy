package provider

import (
	"context"
	"testing"
)

type mockVectorStore struct{}

func (m *mockVectorStore) Search(ctx context.Context, req interface{}) (interface{}, error) {
	return nil, nil
}
func (m *mockVectorStore) Close() error { return nil }

func TestNewVectorStore_Qdrant(t *testing.T) {
	vs, err := NewVectorStore("qdrant", "localhost", 6334)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if vs == nil {
		t.Fatalf("expected a VectorStore, got nil")
	}
}

func TestNewVectorStore_Unsupported(t *testing.T) {
	vs, err := NewVectorStore("unsupported", "localhost", 1234)
	if err == nil {
		t.Fatalf("expected error for unsupported provider, got nil")
	}
	if vs != nil {
		t.Fatalf("expected nil VectorStore for unsupported provider, got %v", vs)
	}
}
