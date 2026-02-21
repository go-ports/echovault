package config_test

import (
	"os"
	"path/filepath"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/go-ports/echovault/internal/config"
)

func TestDefault_HappyPath(t *testing.T) {
	c := qt.New(t)
	cfg := config.Default()
	c.Assert(cfg, qt.IsNotNil)
	c.Assert(cfg.Embedding.Provider, qt.Equals, "ollama")
	c.Assert(cfg.Embedding.Model, qt.Equals, "nomic-embed-text")
	c.Assert(cfg.Embedding.BaseURL, qt.Equals, "http://localhost:11434")
	c.Assert(cfg.Embedding.APIKey, qt.Equals, "")
	c.Assert(cfg.Context.Semantic, qt.Equals, "auto")
	c.Assert(cfg.Context.TopupRecent, qt.IsTrue)
}

func TestLoad_HappyPath(t *testing.T) {
	c := qt.New(t)

	c.Run("non-existent file returns defaults without error", func(c *qt.C) {
		cfg, err := config.Load("/nonexistent/config.yaml")
		c.Assert(err, qt.IsNil)
		c.Assert(cfg, qt.IsNotNil)
		c.Assert(cfg.Embedding.Provider, qt.Equals, "ollama")
		c.Assert(cfg.Context.Semantic, qt.Equals, "auto")
		c.Assert(cfg.Context.TopupRecent, qt.IsTrue)
	})

	tests := []struct {
		name            string
		yaml            string
		wantProvider    string
		wantModel       string
		wantBaseURL     string
		wantAPIKey      string
		wantSemantic    string
		wantTopupRecent bool
	}{
		{
			name:            "full embedding section overrides all fields",
			yaml:            "embedding:\n  provider: openai\n  model: text-embedding-3-small\n  base_url: https://api.openai.com/v1\n  api_key: sk-test\n",
			wantProvider:    "openai",
			wantModel:       "text-embedding-3-small",
			wantBaseURL:     "https://api.openai.com/v1",
			wantAPIKey:      "sk-test",
			wantSemantic:    "auto",
			wantTopupRecent: true,
		},
		{
			name:            "context semantic always",
			yaml:            "context:\n  semantic: always\n",
			wantProvider:    "ollama",
			wantModel:       "nomic-embed-text",
			wantBaseURL:     "http://localhost:11434",
			wantAPIKey:      "",
			wantSemantic:    "always",
			wantTopupRecent: true,
		},
		{
			name:            "context topup_recent disabled",
			yaml:            "context:\n  topup_recent: false\n",
			wantProvider:    "ollama",
			wantModel:       "nomic-embed-text",
			wantBaseURL:     "http://localhost:11434",
			wantAPIKey:      "",
			wantSemantic:    "auto",
			wantTopupRecent: false,
		},
		{
			name:            "context semantic never",
			yaml:            "context:\n  semantic: never\n",
			wantProvider:    "ollama",
			wantModel:       "nomic-embed-text",
			wantBaseURL:     "http://localhost:11434",
			wantAPIKey:      "",
			wantSemantic:    "never",
			wantTopupRecent: true,
		},
		{
			name:            "openrouter provider with custom base_url",
			yaml:            "embedding:\n  provider: openrouter\n  base_url: https://openrouter.ai/api/v1\n",
			wantProvider:    "openrouter",
			wantModel:       "nomic-embed-text",
			wantBaseURL:     "https://openrouter.ai/api/v1",
			wantAPIKey:      "",
			wantSemantic:    "auto",
			wantTopupRecent: true,
		},
	}

	for _, tt := range tests {
		c.Run(tt.name, func(c *qt.C) {
			tmp := t.TempDir()
			path := filepath.Join(tmp, "config.yaml")
			err := os.WriteFile(path, []byte(tt.yaml), 0o600)
			c.Assert(err, qt.IsNil)

			cfg, err := config.Load(path)
			c.Assert(err, qt.IsNil)
			c.Assert(cfg.Embedding.Provider, qt.Equals, tt.wantProvider)
			c.Assert(cfg.Embedding.Model, qt.Equals, tt.wantModel)
			c.Assert(cfg.Embedding.BaseURL, qt.Equals, tt.wantBaseURL)
			c.Assert(cfg.Embedding.APIKey, qt.Equals, tt.wantAPIKey)
			c.Assert(cfg.Context.Semantic, qt.Equals, tt.wantSemantic)
			c.Assert(cfg.Context.TopupRecent, qt.Equals, tt.wantTopupRecent)
		})
	}
}

func TestLoad_PartialOverrideRetainsDefaults(t *testing.T) {
	c := qt.New(t)

	tmp := t.TempDir()
	path := filepath.Join(tmp, "config.yaml")
	err := os.WriteFile(path, []byte("embedding:\n  provider: openrouter\n"), 0o600)
	c.Assert(err, qt.IsNil)

	cfg, err := config.Load(path)
	c.Assert(err, qt.IsNil)
	// Overridden field.
	c.Assert(cfg.Embedding.Provider, qt.Equals, "openrouter")
	// Defaults retained for unspecified fields.
	c.Assert(cfg.Embedding.Model, qt.Equals, "nomic-embed-text")
	c.Assert(cfg.Embedding.BaseURL, qt.Equals, "http://localhost:11434")
	c.Assert(cfg.Context.Semantic, qt.Equals, "auto")
	c.Assert(cfg.Context.TopupRecent, qt.IsTrue)
}

func TestLoad_EmptyProviderRetainsDefault(t *testing.T) {
	c := qt.New(t)

	// The Load function only overrides provider when the value is non-empty.
	tmp := t.TempDir()
	path := filepath.Join(tmp, "config.yaml")
	err := os.WriteFile(path, []byte("embedding:\n  provider: \"\"\n"), 0o600)
	c.Assert(err, qt.IsNil)

	cfg, err := config.Load(path)
	c.Assert(err, qt.IsNil)
	c.Assert(cfg.Embedding.Provider, qt.Equals, "ollama")
}

func TestResolveMemoryHome_EnvOverride(t *testing.T) {
	c := qt.New(t)

	tmp := t.TempDir()
	t.Setenv("MEMORY_HOME", tmp)

	path, source := config.ResolveMemoryHome()
	c.Assert(source, qt.Equals, "env")
	c.Assert(path, qt.Equals, tmp)
}
