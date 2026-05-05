package middleware

import (
	"context"
	"log"
	"time"

	"github.com/ihgazi/vectorproxy/internal/search"
)

func LoggingInterceptor(next SearchHandler) SearchHandler {
	return func(ctx context.Context, req *search.SearchQuery) (*search.SearchResponse, error) {
		start := time.Now()

		log.Printf("Incoming search request: Collection=%s, TopK=%d", req.Collection, req.TopK)

		resp, err := next(ctx, req)

		duration := time.Since(start)
		if err != nil {
			log.Printf("Search request failed after %v: %v", duration, err)
		} else {
			log.Printf("Search request successful: %d results after %v", len(resp.Results), duration)
		}

		return resp, err
	}
}
