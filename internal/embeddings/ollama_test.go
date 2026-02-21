package embeddings_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/go-ports/echovault/internal/embeddings"
)

// newOllamaEmbedServer starts a test HTTP server that responds to every request
// with an Ollama-style embedding response containing vec.
func newOllamaEmbedServer(t *testing.T, vec []float32) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"embedding": vec})
	}))
}

// newOllamaErrorServer starts a test HTTP server that always returns 500.
func newOllamaErrorServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}))
}

// ---------------------------------------------------------------------------
// Ollama.Embed
// ---------------------------------------------------------------------------

func TestOllamaEmbed_HappyPath(t *testing.T) {
	c := qt.New(t)

	cases := []struct {
		name string
		vec  []float32
	}{
		{"single-element vector", []float32{0.5}},
		{"multi-element vector", []float32{0.1, 0.5, 0.9}},
	}

	for _, tc := range cases {
		c.Run(tc.name, func(c *qt.C) {
			srv := newOllamaEmbedServer(t, tc.vec)
			defer srv.Close()

			o := embeddings.NewOllama("test-model", srv.URL)
			got, err := o.Embed(context.Background(), "hello world")
			c.Assert(err, qt.IsNil)
			c.Assert(got, qt.DeepEquals, tc.vec)
		})
	}
}

func TestOllamaEmbed_FailurePath(t *testing.T) {
	c := qt.New(t)

	c.Run("non-2xx response returns error", func(c *qt.C) {
		srv := newOllamaErrorServer(t)
		defer srv.Close()

		o := embeddings.NewOllama("test-model", srv.URL)
		got, err := o.Embed(context.Background(), "hello")
		c.Assert(err, qt.IsNotNil)
		c.Assert(got, qt.IsNil)
	})

	c.Run("empty embedding in response returns error", func(c *qt.C) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"embedding": make([]float32, 0)})
		}))
		defer srv.Close()

		o := embeddings.NewOllama("test-model", srv.URL)
		got, err := o.Embed(context.Background(), "hello")
		c.Assert(err, qt.IsNotNil)
		c.Assert(err, qt.ErrorMatches, ".*empty embedding.*")
		c.Assert(got, qt.IsNil)
	})
}

// ---------------------------------------------------------------------------
// Ollama.EmbedBatch
// ---------------------------------------------------------------------------

func TestOllamaEmbedBatch_HappyPath(t *testing.T) {
	c := qt.New(t)

	c.Run("each text receives the same fixed vector", func(c *qt.C) {
		fixedVec := []float32{1.0, 2.0}
		srv := newOllamaEmbedServer(t, fixedVec)
		defer srv.Close()

		o := embeddings.NewOllama("test-model", srv.URL)
		got, err := o.EmbedBatch(context.Background(), []string{"alpha", "beta", "gamma"})
		c.Assert(err, qt.IsNil)
		c.Assert(got, qt.DeepEquals, [][]float32{fixedVec, fixedVec, fixedVec})
	})
}

func TestOllamaEmbedBatch_FailurePath(t *testing.T) {
	c := qt.New(t)

	c.Run("server error on first text propagates and returns nil", func(c *qt.C) {
		srv := newOllamaErrorServer(t)
		defer srv.Close()

		o := embeddings.NewOllama("test-model", srv.URL)
		got, err := o.EmbedBatch(context.Background(), []string{"a", "b"})
		c.Assert(err, qt.IsNotNil)
		c.Assert(got, qt.IsNil)
	})
}

// ---------------------------------------------------------------------------
// IsOllamaModelLoaded
// ---------------------------------------------------------------------------

func TestIsOllamaModelLoaded_HappyPath(t *testing.T) {
	c := qt.New(t)

	c.Run("exact model name match returns true", func(c *qt.C) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"models":[{"name":"nomic-embed-text","model":"nomic-embed-text"}]}`))
		}))
		defer srv.Close()

		c.Assert(embeddings.IsOllamaModelLoaded("nomic-embed-text", srv.URL), qt.IsTrue)
	})

	c.Run("model with :tag suffix in response matches bare name query", func(c *qt.C) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"models":[{"name":"nomic-embed-text:latest"}]}`))
		}))
		defer srv.Close()

		c.Assert(embeddings.IsOllamaModelLoaded("nomic-embed-text", srv.URL), qt.IsTrue)
	})
}

func TestIsOllamaModelLoaded_FailurePath(t *testing.T) {
	c := qt.New(t)

	c.Run("model not in list returns false", func(c *qt.C) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"models":[{"name":"other-model"}]}`))
		}))
		defer srv.Close()

		c.Assert(embeddings.IsOllamaModelLoaded("nomic-embed-text", srv.URL), qt.IsFalse)
	})

	c.Run("empty model list returns false", func(c *qt.C) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"models":[]}`))
		}))
		defer srv.Close()

		c.Assert(embeddings.IsOllamaModelLoaded("nomic-embed-text", srv.URL), qt.IsFalse)
	})

	c.Run("server unreachable returns false", func(c *qt.C) {
		srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {}))
		srv.Close() // shut down immediately so the URL is unreachable

		c.Assert(embeddings.IsOllamaModelLoaded("nomic-embed-text", srv.URL), qt.IsFalse)
	})
}
