package embeddings_test

import (
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/go-ports/echovault/internal/config"
	"github.com/go-ports/echovault/internal/embeddings"
)

// cfg returns a MemoryConfig with the given provider set.
func cfg(provider, model, apiKey, baseURL string) *config.MemoryConfig {
	c := config.Default()
	c.Embedding.Provider = provider
	c.Embedding.Model = model
	c.Embedding.APIKey = apiKey
	c.Embedding.BaseURL = baseURL
	return c
}

func TestNewProvider_HappyPath(t *testing.T) {
	c := qt.New(t)

	c.Run("empty provider returns nil provider and no error", func(c *qt.C) {
		ep, err := embeddings.NewProvider(cfg("", "", "", ""))
		c.Assert(err, qt.IsNil)
		c.Assert(ep, qt.IsNil)
	})

	c.Run("none provider returns nil provider and no error", func(c *qt.C) {
		ep, err := embeddings.NewProvider(cfg("none", "", "", ""))
		c.Assert(err, qt.IsNil)
		c.Assert(ep, qt.IsNil)
	})

	c.Run("ollama provider returns non-nil Provider", func(c *qt.C) {
		ep, err := embeddings.NewProvider(cfg("ollama", "nomic-embed-text", "", ""))
		c.Assert(err, qt.IsNil)
		c.Assert(ep, qt.IsNotNil)
	})

	c.Run("ollama provider with custom base URL returns non-nil Provider", func(c *qt.C) {
		ep, err := embeddings.NewProvider(cfg("ollama", "nomic-embed-text", "", "http://my-ollama:11434"))
		c.Assert(err, qt.IsNil)
		c.Assert(ep, qt.IsNotNil)
	})

	c.Run("openai provider returns non-nil Provider", func(c *qt.C) {
		ep, err := embeddings.NewProvider(cfg("openai", "text-embedding-3-small", "sk-test", ""))
		c.Assert(err, qt.IsNil)
		c.Assert(ep, qt.IsNotNil)
	})

	c.Run("openrouter provider returns non-nil Provider", func(c *qt.C) {
		ep, err := embeddings.NewProvider(cfg("openrouter", "some-model", "or-test-key", ""))
		c.Assert(err, qt.IsNil)
		c.Assert(ep, qt.IsNotNil)
	})
}

func TestNewProvider_FailurePath(t *testing.T) {
	c := qt.New(t)

	c.Run("unknown provider returns error", func(c *qt.C) {
		ep, err := embeddings.NewProvider(cfg("unsupported-provider", "", "", ""))
		c.Assert(err, qt.IsNotNil)
		c.Assert(ep, qt.IsNil)
	})
}
