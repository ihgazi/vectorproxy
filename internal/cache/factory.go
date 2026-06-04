package cache

import (
	"fmt"
)

// Config holds the semantic cache plugin configuration parameters.
type Config struct {
	Provider   string
	Host       string
	Port       int
	Threshold  float32 // Similarity threshold
	Dimension  int     // Vector dimensionality
	IndexName  string  // Vector index name
	TTLSeconds int
}

// NewSemanticCache instantiates a customizable semantic cache backend based on the Config.
func NewSemanticCache(cfg Config) (SemanticCache, error) {
	switch cfg.Provider {
	case "redis":
		return NewRedisCache(cfg)
	default:
		return nil, fmt.Errorf("unsupported cache provider: %s", cfg.Provider)
	}
}
