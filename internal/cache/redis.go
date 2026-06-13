package cache

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"sync"
	"time"

	"github.com/ihgazi/vectorproxy/internal/keygen"
	"github.com/ihgazi/vectorproxy/internal/store"
	"github.com/redis/go-redis/v9"
)

// RedisCache represents a semantic cache backed by a Redis-VL system.
type RedisCache struct {
	client         *redis.Client
	threshold      float32
	ttl            time.Duration
	createdIndices sync.Map
}

// NewRedisCache creates a new instance of the Redis semantic cache.
func NewRedisCache(cfg Config) (SemanticCache, error) {
	log.Printf("Initializing Redis Semantic Cache on %s:%d", cfg.Host, cfg.Port)
	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)

	rClient := redis.NewClient(
		&redis.Options{
			Addr:     addr,
			Protocol: 2, // Force RESP2 protocol to ensure consistent array-based responses from RediSearch
		},
	)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := rClient.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis at %s: %v", addr, err)
	}

	ttl := 300 * time.Second // Default TTL of 5 minutes
	if cfg.TTLSeconds > 0 {
		ttl = time.Duration(cfg.TTLSeconds) * time.Second
	}

	return &RedisCache{
		client:    rClient,
		threshold: cfg.Threshold,
		ttl:       ttl,
	}, nil
}

// Get queries Redis vector similarity space for a match.
func (r *RedisCache) Get(ctx context.Context, collection string, vector []float32, topK int32) (*store.SearchResponse, bool, error) {
	indexName := getIndexName(collection)

	// Check if index is loaded in cache tracking map
	_, created := r.createdIndices.Load(collection)
	if !created {
		// Check if index already exists in Redis
		_, err := r.client.Do(ctx, "FT.INFO", indexName).Result()

		if err != nil {
			return nil, false, nil
		}

		r.createdIndices.Store(collection, true)
	}

	// Convert float32 vector to raw binary bytes (Little Endian)
	vectorBytes, err := float32SliceToBytes(vector)
	if err != nil {
		return nil, false, fmt.Errorf("failed to serialize query vector: %v", err)
	}

	// Perform KNN vector search using FT.SEARCH (KNN limit 1)
	res, err := r.client.Do(ctx,
		"FT.SEARCH", indexName,
		"*=>[KNN 1 @vector $vec AS score]",
		"PARAMS", "2", "vec", vectorBytes,
		"DIALECT", "2",
	).Result()

	if err != nil {
		return nil, false, fmt.Errorf("Redis FT.Search query failed: %v", err)
	}

	return parseSearchResults(res, r.threshold, topK)
}

// Set saves the search query vector and associated results into Redis.
func (r *RedisCache) Set(ctx context.Context, collection string, vector []float32, resp *store.SearchResponse) error {
	_, created := r.createdIndices.Load(collection)
	if !created {
		if err := r.createIndex(ctx, collection, len(vector)); err != nil {
			return err
		}

		r.createdIndices.Store(collection, true)
	}

	// Generate key
	docID := keygen.HashVector(vector)
	key := fmt.Sprintf("%s%s", getPrefix(collection), docID)

	// Serialize query vector
	vecBytes, err := float32SliceToBytes(vector)
	if err != nil {
		return fmt.Errorf("failed to serialize query vector: %v", err)
	}

	// Serialize response struct to JSON
	respBytes, err := json.Marshal(resp)
	if err != nil {
		return fmt.Errorf("failed to serialize response: %v", err)
	}

	// Store fields using HSET
	pipe := r.client.Pipeline()
	pipe.HSet(ctx, key, "vector", vecBytes, "response", string(respBytes))
	pipe.Expire(ctx, key, r.ttl)

	_, err = pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to save cache entry in Redis: %v", err)
	}

	return nil
}

// Close closes the Redis client connection.
func (r *RedisCache) Close() error {
	return r.client.Close()
}

// createIndex creates a vector search index for a specific collection
func (r *RedisCache) createIndex(ctx context.Context, collection string, dim int) error {
	indexName := getIndexName(collection)
	prefix := getPrefix(collection)

	_, err := r.client.Do(ctx, "FT.INFO", indexName).Result()
	if err == nil {
		return nil
	}

	// Create new HNSW index for the collection
	cmd := []any{
		"FT.CREATE", indexName,
		"ON", "HASH",
		"PREFIX", "1", prefix,
		"SCHEMA",
		"vector", "VECTOR", "HNSW", "6",
		"TYPE", "FLOAT32",
		"DIM", strconv.Itoa(dim),
		"DISTANCE_METRIC", "COSINE",
		"response", "TEXT",
	}

	_, err = r.client.Do(ctx, cmd...).Result()
	if err != nil {
		return fmt.Errorf("failed to create Redis index: %v", err)
	}

	log.Printf("Created Redis vector index: %s with prefix: %s", indexName, prefix)
	return nil
}

func getIndexName(collection string) string {
	return fmt.Sprintf("idx:vectorproxy:%s", collection)
}

func getPrefix(collection string) string {
	return fmt.Sprintf("vectorproxy:cache:%s:", collection)
}

func getString(v any) (string, bool) {
	switch val := v.(type) {
	case string:
		return val, true
	case []byte:
		return string(val), true
	default:
		return "", false
	}
}

func float32SliceToBytes(vec []float32) ([]byte, error) {
	buf := new(bytes.Buffer)
	err := binary.Write(buf, binary.LittleEndian, vec)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Parses FT.Search raw response interface
func parseSearchResults(res any, threshold float32, topK int32) (*store.SearchResponse, bool, error) {
	arr, ok := res.([]any)
	if !ok || len(arr) < 3 {
		return nil, false, nil
	}

	count, ok := arr[0].(int64)
	if !ok || count == 0 {
		return nil, false, nil
	}

	fields, ok := arr[2].([]any)
	if !ok {
		return nil, false, nil
	}

	var jsonResponse string
	var score float32
	var hasScore, hasResponse bool

	for i := 0; i < len(fields)-1; i += 2 {
		if i+1 >= len(fields) {
			break
		}

		k, ok1 := getString(fields[i])
		v, ok2 := getString(fields[i+1])

		if !ok1 || !ok2 {
			continue
		}

		if k == "response" {
			jsonResponse = v
			hasResponse = true
		} else if k == "score" {
			val, err := strconv.ParseFloat(v, 32)
			if err == nil {
				score = float32(val)
				hasScore = true
			}
		}
	}

	if !hasResponse {
		return nil, false, nil
	}

	if hasScore {
		similarity := 1.0 - score
		if similarity < threshold {
			return nil, false, nil
		}
	}

	// Deserialize JSON string into structured store.SearchResponse
	var searchResp store.SearchResponse
	if err := json.Unmarshal([]byte(jsonResponse), &searchResp); err != nil {
		return nil, false, fmt.Errorf("failed to deserialize cached response: %v", err)
	}

	// Verify whether the deserialized response contains at least topK results
	// If it contains sufficient results (or we know the DB is exhausted), trim it and return
	if len(searchResp.Results) >= int(topK) || searchResp.MaxLimit {
		if len(searchResp.Results) > int(topK) {
			searchResp.Results = searchResp.Results[:topK]
		}
		return &searchResp, true, nil
	}

	return nil, false, nil
}
