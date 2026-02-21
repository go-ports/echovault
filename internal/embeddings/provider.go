// Package embeddings provides an interface and implementations for embedding providers.
package embeddings

import (
	"context"
	"fmt"

	"github.com/go-ports/echovault/internal/config"
)

// Provider is the interface for embedding models.
type Provider interface {
	// Embed returns a float32 vector for the given text.
	Embed(ctx context.Context, text string) ([]float32, error)
	// EmbedBatch returns vectors for multiple texts.
	EmbedBatch(ctx context.Context, texts []string) ([][]float32, error)
}

// NewProvider constructs a Provider from the given config.
// Returns (nil, nil) when the provider is "" or "none".
func NewProvider(cfg *config.MemoryConfig) (Provider, error) {
	switch cfg.Embedding.Provider {
	case "ollama":
		baseURL := cfg.Embedding.BaseURL
		if baseURL == "" {
			baseURL = "http://localhost:11434"
		}
		return NewOllama(cfg.Embedding.Model, baseURL), nil

	case "openai":
		return NewOpenAI(cfg.Embedding.Model, cfg.Embedding.APIKey, ""), nil

	case "openrouter":
		const openRouterBase = "https://openrouter.ai/api/v1"
		return NewOpenAI(cfg.Embedding.Model, cfg.Embedding.APIKey, openRouterBase), nil

	case "", "none":
		return nil, nil

	default:
		return nil, fmt.Errorf("unknown embedding provider: %s", cfg.Embedding.Provider)
	}
}
