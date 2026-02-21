package embeddings_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/go-ports/echovault/internal/embeddings"
)

// ---------------------------------------------------------------------------
// OpenAI.Embed
// ---------------------------------------------------------------------------

func TestOpenAIEmbed_HappyPath(t *testing.T) {
	c := qt.New(t)

	c.Run("single text returns embedding vector", func(c *qt.C) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":[{"index":0,"embedding":[0.1,0.2,0.3]}]}`))
		}))
		defer srv.Close()

		o := embeddings.NewOpenAI("text-embedding-3-small", "sk-test", srv.URL)
		got, err := o.Embed(context.Background(), "hello")
		c.Assert(err, qt.IsNil)
		c.Assert(got, qt.DeepEquals, []float32{0.1, 0.2, 0.3})
	})

	c.Run("authorization header is forwarded to the server", func(c *qt.C) {
		var capturedAuth string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			capturedAuth = r.Header.Get("Authorization")
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":[{"index":0,"embedding":[1.0]}]}`))
		}))
		defer srv.Close()

		o := embeddings.NewOpenAI("model", "my-secret-key", srv.URL)
		_, err := o.Embed(context.Background(), "test")
		c.Assert(err, qt.IsNil)
		c.Assert(capturedAuth, qt.Equals, "Bearer my-secret-key")
	})
}

func TestOpenAIEmbed_FailurePath(t *testing.T) {
	c := qt.New(t)

	c.Run("non-2xx response returns error", func(c *qt.C) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
		}))
		defer srv.Close()

		o := embeddings.NewOpenAI("model", "bad-key", srv.URL)
		got, err := o.Embed(context.Background(), "hello")
		c.Assert(err, qt.IsNotNil)
		c.Assert(got, qt.IsNil)
	})
}

// ---------------------------------------------------------------------------
// OpenAI.EmbedBatch
// ---------------------------------------------------------------------------

func TestOpenAIEmbedBatch_HappyPath(t *testing.T) {
	c := qt.New(t)

	c.Run("results are ordered by index regardless of server response order", func(c *qt.C) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			// Deliberately return index 1 before index 0 to verify sorting.
			_, _ = w.Write([]byte(`{"data":[{"index":1,"embedding":[0.2]},{"index":0,"embedding":[0.1]}]}`))
		}))
		defer srv.Close()

		o := embeddings.NewOpenAI("text-embedding-3-small", "sk-test", srv.URL)
		got, err := o.EmbedBatch(context.Background(), []string{"first", "second"})
		c.Assert(err, qt.IsNil)
		c.Assert(got, qt.HasLen, 2)
		c.Assert(got[0], qt.DeepEquals, []float32{0.1})
		c.Assert(got[1], qt.DeepEquals, []float32{0.2})
	})

	c.Run("single text returns one embedding", func(c *qt.C) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":[{"index":0,"embedding":[1.0,2.0]}]}`))
		}))
		defer srv.Close()

		o := embeddings.NewOpenAI("model", "sk-test", srv.URL)
		got, err := o.EmbedBatch(context.Background(), []string{"only"})
		c.Assert(err, qt.IsNil)
		c.Assert(got, qt.DeepEquals, [][]float32{{1.0, 2.0}})
	})
}

func TestOpenAIEmbedBatch_FailurePath(t *testing.T) {
	c := qt.New(t)

	c.Run("non-2xx response returns error", func(c *qt.C) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "rate limited", http.StatusTooManyRequests)
		}))
		defer srv.Close()

		o := embeddings.NewOpenAI("model", "sk-test", srv.URL)
		got, err := o.EmbedBatch(context.Background(), []string{"a", "b"})
		c.Assert(err, qt.IsNotNil)
		c.Assert(got, qt.IsNil)
	})

	c.Run("empty data array in response returns error", func(c *qt.C) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":[]}`))
		}))
		defer srv.Close()

		o := embeddings.NewOpenAI("model", "sk-test", srv.URL)
		got, err := o.EmbedBatch(context.Background(), []string{"a"})
		c.Assert(err, qt.IsNotNil)
		c.Assert(err, qt.ErrorMatches, ".*empty data.*")
		c.Assert(got, qt.IsNil)
	})
}
