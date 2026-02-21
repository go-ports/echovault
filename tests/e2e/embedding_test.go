// Package e2e_test — end-to-end embedding pipeline tests.
//
// Each test exercises the full save→embed→vector-index path using lightweight
// in-process mock HTTP servers instead of real provider APIs.
package e2e_test

import (
	"encoding/json"
	"testing"

	qt "github.com/frankban/quicktest"
)

// ---------------------------------------------------------------------------
// CLI — save
// ---------------------------------------------------------------------------

// TestCLISaveWithEmbedding_HappyPath verifies that the CLI save command
// successfully embeds the memory via each configured provider and reports
// "Saved:" in the output.
func TestCLISaveWithEmbedding_HappyPath(t *testing.T) {
	c := qt.New(t)

	for _, tc := range embeddingCases {
		c.Run(tc.provider, func(c *qt.C) {
			srv := tc.startSrv(c.TB)
			home := c.TB.TempDir()
			writeEmbeddingCfg(c.TB, home, tc.provider, srv.URL)

			out, err := runCmd(c.TB, "--memory-home", home, "save",
				"--title", "Embedding pipeline test",
				"--what", "Testing the embedding pipeline with "+tc.provider,
				"--category", "pattern",
			)
			c.Assert(err, qt.IsNil)
			c.Assert(out, qt.Contains, "Saved: Embedding pipeline test")
		})
	}
}

// ---------------------------------------------------------------------------
// CLI — search (vector path)
// ---------------------------------------------------------------------------

// TestCLISearchWithEmbedding_HappyPath saves a memory with embeddings enabled
// and then searches for it, exercising the vector search path end-to-end.
func TestCLISearchWithEmbedding_HappyPath(t *testing.T) {
	c := qt.New(t)

	for _, tc := range embeddingCases {
		c.Run(tc.provider, func(c *qt.C) {
			srv := tc.startSrv(c.TB)
			home := c.TB.TempDir()
			writeEmbeddingCfg(c.TB, home, tc.provider, srv.URL)

			_, saveErr := runCmd(c.TB, "--memory-home", home, "save",
				"--title", "Vector search test",
				"--what", "Verifying vector search with "+tc.provider,
				"--category", "learning",
			)
			c.Assert(saveErr, qt.IsNil)

			out, err := runCmd(c.TB, "--memory-home", home, "search", "vector search")
			c.Assert(err, qt.IsNil)
			c.Assert(out, qt.Contains, "Vector search test")
		})
	}
}

// ---------------------------------------------------------------------------
// MCP — save
// ---------------------------------------------------------------------------

// TestMCPSaveWithEmbedding_HappyPath verifies that the MCP memory_save tool
// successfully embeds the memory via each configured provider and returns
// action "created".
func TestMCPSaveWithEmbedding_HappyPath(t *testing.T) {
	c := qt.New(t)

	for _, tc := range embeddingCases {
		c.Run(tc.provider, func(c *qt.C) {
			srv := tc.startSrv(c.TB)
			cl := newMCPClientWithEmbedding(c, tc.provider, srv.URL)

			text := callTool(c, cl, "memory_save", map[string]any{
				"title":    "MCP embedding test",
				"what":     "Testing MCP embedding pipeline with " + tc.provider,
				"category": "pattern",
				"project":  "echovault",
			})

			var saved map[string]any
			c.Assert(json.Unmarshal([]byte(text), &saved), qt.IsNil)
			c.Assert(saved["action"], qt.Equals, "created")
			c.Assert(saved["id"], qt.IsNotNil)
		})
	}
}

// ---------------------------------------------------------------------------
// MCP — search (vector path)
// ---------------------------------------------------------------------------

// TestMCPSearchWithEmbedding_HappyPath saves a memory with embeddings enabled
// and searches via memory_search to exercise the vector search path.
func TestMCPSearchWithEmbedding_HappyPath(t *testing.T) {
	c := qt.New(t)

	for _, tc := range embeddingCases {
		c.Run(tc.provider, func(c *qt.C) {
			srv := tc.startSrv(c.TB)
			cl := newMCPClientWithEmbedding(c, tc.provider, srv.URL)

			callTool(c, cl, "memory_save", map[string]any{
				"title":    "MCP vector search test",
				"what":     "Verifying MCP vector search with " + tc.provider,
				"category": "learning",
				"project":  "echovault",
			})

			text := callTool(c, cl, "memory_search", map[string]any{
				"query":   "vector search",
				"project": "echovault",
			})

			var results []map[string]any
			c.Assert(json.Unmarshal([]byte(text), &results), qt.IsNil)
			c.Assert(results, qt.HasLen, 1)
			c.Assert(results[0]["title"], qt.Equals, "MCP vector search test")
		})
	}
}
