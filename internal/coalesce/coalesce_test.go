package coalesce

import (
	"testing"

	"github.com/ihgazi/vectorproxy/internal/search"
)

func TestHashVector(t *testing.T) {
	v1 := []float32{0.1, -0.2, 0.3}
	v2 := []float32{0.1, -0.2, 0.3}
	v3 := []float32{0.1, 0.2, 0.3}

	h1 := hashVector(v1)
	h2 := hashVector(v2)
	h3 := hashVector(v3)

	if h1 != h2 {
		t.Errorf("hashVector should be deterministic; expected equal hashes for identical vectors, got %q and %q", h1, h2)
	}

	if h1 == h3 {
		t.Errorf("hashVector should have no collisions; expected different hashes for different vectors, got same hash %q", h1)
	}
}

func TestStringKeyGenerator(t *testing.T) {
	gen := NewStringKeyGenerator()

	// 1. Should return empty key for empty query prompt
	reqEmpty := &search.SearchQuery{Collection: "test", Query: ""}
	if k := gen(reqEmpty); k != "" {
		t.Errorf("expected empty string key, got %q", k)
	}

	// 2. Should return key formatted correctly
	req := &search.SearchQuery{Collection: "books", Query: "Go programming"}
	expected := "str:books:Go programming"
	if k := gen(req); k != expected {
		t.Errorf("expected key %q, got %q", expected, k)
	}
}

func TestVectorKeyGenerator(t *testing.T) {
	gen := NewVectorKeyGenerator()

	// 1. Should return empty key for empty vector
	reqEmpty := &search.SearchQuery{Collection: "test", Vector: nil}
	if k := gen(reqEmpty); k != "" {
		t.Errorf("expected empty string key, got %q", k)
	}

	// 2. Should return key formatted with vector hash
	req := &search.SearchQuery{Collection: "images", Vector: []float32{0.5, 0.6}}
	hash := hashVector(req.Vector)
	expected := "vec:images:" + hash
	if k := gen(req); k != expected {
		t.Errorf("expected key %q, got %q", expected, k)
	}
}
