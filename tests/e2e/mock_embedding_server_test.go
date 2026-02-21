// Package e2e_test — shared mock HTTP server helpers for embedding provider tests.
// These helpers let e2e tests exercise the full save→embed→vector-index pipeline
// without calling real external APIs.
package e2e_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	qt "github.com/frankban/quicktest"
	mcpclient "github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"

	internalmcp "github.com/go-ports/echovault/internal/mcp"
	"github.com/go-ports/echovault/internal/service"
)

// fixedEmbeddingVec is the deterministic vector returned by every mock embedding
// server. Four dimensions keeps tests fast; production models use 384–3072.
var fixedEmbeddingVec = []float32{0.1, 0.2, 0.3, 0.4}

// embeddingCase describes one provider variant for table-driven embedding tests.
type embeddingCase struct {
	provider string
	startSrv func(tb testing.TB) *httptest.Server
}

// embeddingCases is the canonical table of provider variants shared across all
// CLI and MCP embedding tests.
var embeddingCases = []embeddingCase{
	{
		provider: "ollama",
		startSrv: func(tb testing.TB) *httptest.Server { return newOllamaMockServer(tb, "test-model") },
	},
	{
		provider: "openai",
		startSrv: func(tb testing.TB) *httptest.Server { return newOpenAIMockServer(tb) },
	},
	{
		provider: "openrouter",
		startSrv: func(tb testing.TB) *httptest.Server { return newOpenAIMockServer(tb) },
	},
}

// newOllamaMockServer starts a test HTTP server that mimics the Ollama embedding
// API. It responds to:
//   - GET /api/ps          — reports model as loaded (satisfies "auto" semantic mode)
//   - POST /api/embeddings — returns fixedEmbeddingVec for every request
//
// Cleanup is registered on tb automatically.
func newOllamaMockServer(tb testing.TB, model string) *httptest.Server {
	tb.Helper()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/ps", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"models": []map[string]any{{"name": model, "model": model}},
		})
	})
	mux.HandleFunc("/api/embeddings", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"embedding": fixedEmbeddingVec})
	})

	srv := httptest.NewServer(mux)
	tb.Cleanup(srv.Close)
	return srv
}

// newOpenAIMockServer starts a test HTTP server that mimics the OpenAI embeddings
// API (POST /embeddings). It builds a correctly-indexed data entry for every input
// text in the request body, returning fixedEmbeddingVec for each.
// The same server covers openrouter, which uses the identical wire format.
//
// Cleanup is registered on tb automatically.
func newOpenAIMockServer(tb testing.TB) *httptest.Server {
	tb.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var reqBody struct {
			Input []string `json:"input"`
		}
		err := json.NewDecoder(r.Body).Decode(&reqBody)
		if err != nil {
			http.Error(w, "invalid request body: "+err.Error(), http.StatusBadRequest)
			return
		}

		data := make([]map[string]any, len(reqBody.Input))
		for i := range reqBody.Input {
			data[i] = map[string]any{"index": i, "embedding": fixedEmbeddingVec}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"data": data})
	}))
	tb.Cleanup(srv.Close)
	return srv
}

// writeEmbeddingCfg writes a config.yaml into home that configures the named
// embedding provider to use baseURL. context.semantic is set to "always" so
// tests exercise the vector search path unconditionally, without relying on the
// Ollama model-loaded HTTP check.
func writeEmbeddingCfg(tb testing.TB, home, provider, baseURL string) {
	tb.Helper()

	content := fmt.Sprintf(
		"embedding:\n  provider: %s\n  model: test-model\n  base_url: %s\ncontext:\n  semantic: always\n",
		provider, baseURL,
	)
	if err := os.WriteFile(filepath.Join(home, "config.yaml"), []byte(content), 0o600); err != nil {
		tb.Fatalf("writeEmbeddingCfg: %v", err)
	}
}

// newMCPClientWithEmbedding creates an in-process MCP client backed by a fresh
// service whose embedding provider is configured to use baseURL. It mirrors
// newMCPClient but writes a provider config.yaml before initialising the service.
func newMCPClientWithEmbedding(c *qt.C, provider, baseURL string) *mcpclient.Client {
	c.TB.Helper()

	home := c.TB.TempDir()
	writeEmbeddingCfg(c.TB, home, provider, baseURL)

	svc, err := service.New(home)
	c.Assert(err, qt.IsNil)
	c.TB.Cleanup(func() { _ = svc.Close() })

	cl, err := mcpclient.NewInProcessClient(internalmcp.NewServer(svc, nil))
	c.Assert(err, qt.IsNil)
	c.TB.Cleanup(func() { _ = cl.Close() })

	c.Assert(cl.Start(context.Background()), qt.IsNil)

	initReq := mcp.InitializeRequest{}
	initReq.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initReq.Params.ClientInfo = mcp.Implementation{Name: "e2e-test", Version: "0.0.1"}
	_, err = cl.Initialize(context.Background(), initReq)
	c.Assert(err, qt.IsNil)

	return cl
}
