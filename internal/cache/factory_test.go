package cache

import (
	"testing"
)

// TODO: Add tests for Redis cache functionality
func TestNewSemanticCache_Unsupported(t *testing.T) {
	cfg := Config{
		Provider: "unsupported",
	}
	c, err := NewSemanticCache(cfg)
	if err == nil {
		t.Fatal("expected error for unsupported provider, got nil")
	}
	if c != nil {
		t.Fatalf("expected nil cache, got %v", c)
	}
}
