package embeddings

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Ollama calls a local Ollama server for embeddings.
type Ollama struct {
	Model   string
	BaseURL string
	client  *http.Client
}

// NewOllama returns an Ollama provider with a 30s timeout.
func NewOllama(model, baseURL string) *Ollama {
	return &Ollama{
		Model:   model,
		BaseURL: strings.TrimRight(baseURL, "/"),
		client:  &http.Client{Timeout: 30 * time.Second},
	}
}

// Embed calls POST /api/embeddings and returns the embedding vector.
func (o *Ollama) Embed(ctx context.Context, text string) ([]float32, error) {
	reqBody := map[string]any{
		"model":  o.Model,
		"prompt": text,
	}
	var resp struct {
		Embedding []float32 `json:"embedding"`
	}
	if err := doJSON(ctx, o.client, http.MethodPost, o.BaseURL+"/api/embeddings", nil, reqBody, &resp); err != nil {
		return nil, fmt.Errorf("ollama embed: %w", err)
	}
	if len(resp.Embedding) == 0 {
		return nil, fmt.Errorf("ollama embed: empty embedding returned")
	}
	return resp.Embedding, nil
}

// EmbedBatch embeds each text sequentially.
func (o *Ollama) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	results := make([][]float32, len(texts))
	for i, t := range texts {
		v, err := o.Embed(ctx, t)
		if err != nil {
			return nil, err
		}
		results[i] = v
	}
	return results, nil
}

// IsOllamaModelLoaded returns true if model is currently loaded in the Ollama server.
// Uses a 500 ms timeout; returns false on any error.
func IsOllamaModelLoaded(model, baseURL string) bool {
	client := &http.Client{Timeout: 500 * time.Millisecond}
	var resp struct {
		Models []struct {
			Name  string `json:"name"`
			Model string `json:"model"`
		} `json:"models"`
	}
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	if err := doJSON(ctx, client, http.MethodGet,
		strings.TrimRight(baseURL, "/")+"/api/ps",
		nil, nil, &resp,
	); err != nil {
		return false
	}

	target := normalizeModelName(model)
	for _, m := range resp.Models {
		n := m.Name
		if n == "" {
			n = m.Model
		}
		if normalizeModelName(n) == target {
			return true
		}
	}
	return false
}

// normalizeModelName strips the :tag suffix (e.g. "nomic-embed-text:latest" → "nomic-embed-text").
func normalizeModelName(name string) string {
	if idx := strings.IndexByte(name, ':'); idx >= 0 {
		return name[:idx]
	}
	return name
}
