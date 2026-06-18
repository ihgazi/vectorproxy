package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// Traffic & Latency
	RequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "vectorproxy_requests_total",
		Help: "Total number of gRPC requests",
	}, []string{"method", "status"})

	RequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "vectorproxy_request_duration_seconds",
		Help:    "End-to-end gRPC request latency",
		Buckets: prometheus.DefBuckets,
	}, []string{"method"})

	// Caches
	CacheLookupsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "vectorproxy_cache_lookups_total",
		Help: "Total number of cache lookups",
	}, []string{"cache_type", "result"}) // result: "hit" or "miss", cache_type: "semantic_cache", "embedding_cache"

	// Upstream
	UpstreamDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "vectorproxy_upstream_duration_seconds",
		Help:    "Latency of external calls",
		Buckets: []float64{0.01, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
	}, []string{"dependency", "operation"})

	// Optimizations
	CoalescedRequestsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "vectorproxy_coalesced_requests_total",
		Help: "Number of identical concurrent requests merged by the Coalesce Interceptor",
	})

	BatchSize = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "vectorproxy_batch_size",
		Help:    "Number of points grouped together in a batch",
		Buckets: []float64{1, 5, 10, 20, 50, 100},
	}, []string{"operation"}) // operation: "SearchBatch", "EmbedBatch"
)
