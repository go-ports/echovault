package embeddings

import (
	"context"
	"fmt"
	"net/http"
	"sort"
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

	// Sort by index to guarantee order matches inputs.
	sort.Slice(resp.Data, func(i, j int) bool {
		return resp.Data[i].Index < resp.Data[j].Index
	})

	results := make([][]float32, len(resp.Data))
	for i, d := range resp.Data {
		results[i] = d.Embedding
	}
	return results, nil
}
