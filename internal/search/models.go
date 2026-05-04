package search

type SearchQuery struct {
	Collection string
	Vector     []float32
	TopK       int32
	Filter     map[string]string
}

type SearchResult struct {
	ID      string
	Score   float32
	Payload map[string]any
}

type SearchResponse struct {
	Results []SearchResult
}
