package embeddings

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const defaultOpenAIBase = "https://api.openai.com/v1"

// OpenAI calls the OpenAI (or compatible) embeddings API.
type OpenAI struct {
	Model   string
	APIKey  string // #nosec G117 -- APIKey is an intentional field name for the OpenAI authentication token
	BaseURL string
	client  *http.Client
}

// NewOpenAI returns an OpenAI provider. baseURL defaults to the OpenAI endpoint.
func NewOpenAI(model, apiKey, baseURL string) *OpenAI {
	if baseURL == "" {
		baseURL = defaultOpenAIBase
	}
	return &OpenAI{
		Model:   model,
		APIKey:  apiKey,
		BaseURL: strings.TrimRight(baseURL, "/"),
		client:  &http.Client{Timeout: 30 * time.Second},
	}
}

// Embed embeds a single text string.
func (o *OpenAI) Embed(ctx context.Context, text string) ([]float32, error) {
	results, err := o.EmbedBatch(ctx, []string{text})
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, fmt.Errorf("openai embed: empty response")
	}
	return results[0], nil
}

// EmbedBatch embeds multiple texts in a single API call.
func (o *OpenAI) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	reqBody := map[string]any{
		"model": o.Model,
		"input": texts,
	}
	headers := map[string]string{
		"Authorization": "Bearer " + o.APIKey,
	}

	var resp struct {
		Data []struct {
			Index     int       `json:"index"`
			Embedding []float32 `json:"embedding"`
		} `json:"data"`
	}
	if err := doJSON(ctx, o.client, http.MethodPost, o.BaseURL+"/embeddings", headers, reqBody, &resp); err != nil {
		return nil, fmt.Errorf("openai embed: %w", err)
	}
	if len(resp.Data) == 0 {
		return nil, fmt.Errorf("openai embed: empty data in response")
	}
	if len(resp.Data) != len(texts) {
		return nil, fmt.Errorf("openai embed: expected %d results, got %d", len(texts), len(resp.Data))
	}

	// Fill results by index to handle out-of-order responses and detect gaps.
	results := make([][]float32, len(texts))
	for _, d := range resp.Data {
		if d.Index < 0 || d.Index >= len(texts) {
			return nil, fmt.Errorf("openai embed: result index %d out of range [0, %d)", d.Index, len(texts))
		}
		results[d.Index] = d.Embedding
	}
	return results, nil
}
