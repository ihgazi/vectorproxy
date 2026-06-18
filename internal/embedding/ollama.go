package embedding

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/ihgazi/vectorproxy/internal/metrics"
)

type OllamaEmbedder struct {
	BaseURL string
	Model   string
	Client  *http.Client
}

func NewOllamaEmbedder(baseURL, model string) *OllamaEmbedder {
	return &OllamaEmbedder{
		BaseURL: baseURL,
		Model:   model,
		Client:  &http.Client{},
	}
}

func (e *OllamaEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	reqBody, err := json.Marshal(map[string]string{
		"model": e.Model,
		"input": text,
	})
	if err != nil {
		return nil, fmt.Errorf("Invalid embedding request: %v", err)
	}

	res, err := e.getEmbedding(ctx, fmt.Sprintf("%s/api/embed", e.BaseURL), reqBody)
	if err != nil {
		return nil, err
	}

	return res[0], nil
}

func (e *OllamaEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	reqBody, err := json.Marshal(map[string]interface{}{
		"model": e.Model,
		"input": texts,
	})

	if err != nil {
		return nil, fmt.Errorf("Invalid embedding request: %v", err)
	}

	res, err := e.getEmbedding(ctx, fmt.Sprintf("%s/api/embed", e.BaseURL), reqBody)
	if err != nil {
		return nil, err
	}

	return res, nil
}

func (e *OllamaEmbedder) getEmbedding(ctx context.Context, url string, payload []byte) ([][]float32, error) {
	start := time.Now()
	defer func() {
		metrics.UpstreamDuration.WithLabelValues("ollama", "embed").Observe(time.Since(start).Seconds())
	}()

	req, err := http.NewRequestWithContext(ctx, "POST", fmt.Sprintf("%s/api/embed", e.BaseURL), bytes.NewBuffer(payload))
	if err != nil {
		return nil, fmt.Errorf("Failed to fetch embedding from Ollama: %v", err)
	}

	resp, err := e.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Failed to fetch embedding from Ollama: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Ollama embedding request failed with status: %s", resp.Status)
	}

	var res struct {
		Embeddings [][]float32 `json:"embeddings"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, fmt.Errorf("Failed to decode Ollama embedding response: %v", err)
	}

	return res.Embeddings, err
}
