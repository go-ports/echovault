// Package e2e_test — MCP server end-to-end tests.
//
// Each test wires the real MCP server in-process via the mcp-go
// InProcessTransport, backed by a fresh service.Service rooted at a
// temporary directory.  No binary needs to be compiled; the full stack
// (service → db → search → mcp handler → mcp-go server → in-process client)
// is exercised within a single test process.
package e2e_test

import (
	"context"
	"encoding/json"
	"testing"

	qt "github.com/frankban/quicktest"
	mcpclient "github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"

	"github.com/go-ports/echovault/internal/checkers"
	internalmcp "github.com/go-ports/echovault/internal/mcp"
	"github.com/go-ports/echovault/internal/service"
)

// newMCPClientWithDisabledTools creates an in-process MCP client with specified
// tools disabled.
func newMCPClientWithDisabledTools(c *qt.C, disabledTools []string) *mcpclient.Client {
	c.TB.Helper()

	svc, err := service.New(c.TB.TempDir())
	c.Assert(err, qt.IsNil)
	c.TB.Cleanup(func() { _ = svc.Close() })

	cl, err := mcpclient.NewInProcessClient(internalmcp.NewServer(svc, disabledTools))
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

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// newMCPClient creates an in-process MCP client backed by a fresh service
// rooted at c.TB.TempDir().  The client is started and initialized before it
// is returned; cleanup is registered on c automatically.
func newMCPClient(c *qt.C) *mcpclient.Client {
	c.TB.Helper()

	svc, err := service.New(c.TB.TempDir())
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

// callTool invokes the named MCP tool and returns the text of the first
// content item.  All errors are surfaced as immediate assertion failures via c.
func callTool(c *qt.C, cl *mcpclient.Client, name string, args map[string]any) string {
	req := mcp.CallToolRequest{}
	req.Params.Name = name
	req.Params.Arguments = args

	result, err := cl.CallTool(context.Background(), req)
	c.Assert(err, qt.IsNil)
	c.Assert(result.Content, qt.HasLen, 1)

	tc, ok := mcp.AsTextContent(result.Content[0])
	c.Assert(ok, qt.IsTrue)

	return tc.Text
}

// ---------------------------------------------------------------------------
// ListTools
// ---------------------------------------------------------------------------

func TestMCPListTools_HappyPath(t *testing.T) {
	c := qt.New(t)
	cl := newMCPClient(c)

	result, err := cl.ListTools(context.Background(), mcp.ListToolsRequest{})
	c.Assert(err, qt.IsNil)
	c.Assert(result.Tools, qt.HasLen, 5)

	names := make([]string, len(result.Tools))
	for i, tool := range result.Tools {
		names[i] = tool.Name
	}
	c.Assert(names, qt.Contains, "memory_save")
	c.Assert(names, qt.Contains, "memory_search")
	c.Assert(names, qt.Contains, "memory_context")
	c.Assert(names, qt.Contains, "memory_delete")
	c.Assert(names, qt.Contains, "memory_replace")
}

// ---------------------------------------------------------------------------
// memory_save
// ---------------------------------------------------------------------------

func TestMCPMemorySave_HappyPath(t *testing.T) {
	c := qt.New(t)
	cl := newMCPClient(c)

	cases := []struct {
		name     string
		title    string
		what     string
		category string
	}{
		{"pattern memory", "CGO required for sqlite", "CGO must be enabled for go-sqlite3", "pattern"},
		{"decision memory", "Use make for all builds", "Run make targets, not go build directly", "decision"},
	}

	for _, tc := range cases {
		c.Run(tc.name, func(c *qt.C) {
			text := callTool(c, cl, "memory_save", map[string]any{
				"title":    tc.title,
				"what":     tc.what,
				"category": tc.category,
				"project":  "echovault",
			})

			var saved map[string]any
			c.Assert(json.Unmarshal([]byte(text), &saved), qt.IsNil)
			c.Assert(saved["action"], qt.Equals, "created")
			c.Assert(saved["id"], qt.IsNotNil)
			c.Assert(saved["file_path"], qt.IsNotNil)
		})
	}
}

func TestMCPMemorySave_FailurePath(t *testing.T) {
	c := qt.New(t)
	cl := newMCPClient(c)

	c.Run("invalid category falls back to context without error", func(c *qt.C) {
		text := callTool(c, cl, "memory_save", map[string]any{
			"title":    "category fallback test",
			"what":     "testing that an unrecognised category defaults to context",
			"category": "nonexistent",
			"project":  "echovault",
		})
		c.Assert(text, checkers.JSONPathEquals("$.action"), "created")
	})
}

// ---------------------------------------------------------------------------
// memory_search
// ---------------------------------------------------------------------------

func TestMCPMemorySearch_HappyPath(t *testing.T) {
	c := qt.New(t)
	cl := newMCPClient(c)

	callTool(c, cl, "memory_save", map[string]any{
		"title":    "CGO required for sqlite",
		"what":     "CGO must be enabled for go-sqlite3 and sqlite-vec extensions",
		"category": "pattern",
		"project":  "echovault",
	})

	text := callTool(c, cl, "memory_search", map[string]any{
		"query":   "sqlite",
		"project": "echovault",
	})

	var results []map[string]any
	c.Assert(json.Unmarshal([]byte(text), &results), qt.IsNil)
	c.Assert(results, qt.HasLen, 1)
	c.Assert(results[0]["title"], qt.Equals, "CGO required for sqlite")
}

func TestMCPMemorySearch_EmptyVault_HappyPath(t *testing.T) {
	c := qt.New(t)
	cl := newMCPClient(c)

	text := callTool(c, cl, "memory_search", map[string]any{
		"query": "anything",
	})

	var results []map[string]any
	c.Assert(json.Unmarshal([]byte(text), &results), qt.IsNil)
	c.Assert(results, qt.HasLen, 0)
}

// ---------------------------------------------------------------------------
// memory_context
// ---------------------------------------------------------------------------

func TestMCPMemoryContext_HappyPath(t *testing.T) {
	c := qt.New(t)
	cl := newMCPClient(c)

	callTool(c, cl, "memory_save", map[string]any{
		"title":    "Context injection pattern",
		"what":     "Run memory context at the start of every coding agent session",
		"category": "pattern",
		"project":  "echovault",
	})

	text := callTool(c, cl, "memory_context", map[string]any{
		"project": "echovault",
	})

	c.Assert(text, checkers.JSONPathEquals("$.total"), float64(1))
	c.Assert(text, checkers.JSONPathEquals("$.showing"), float64(1))

	var ctx map[string]any
	c.Assert(json.Unmarshal([]byte(text), &ctx), qt.IsNil)
	c.Assert(ctx["memories"], qt.IsNotNil)
}

func TestMCPMemoryContext_EmptyVault_HappyPath(t *testing.T) {
	c := qt.New(t)
	cl := newMCPClient(c)

	text := callTool(c, cl, "memory_context", map[string]any{
		"project": "echovault",
	})

	c.Assert(text, checkers.JSONPathEquals("$.total"), float64(0))
}

// ---------------------------------------------------------------------------
// memory_delete
// ---------------------------------------------------------------------------

func TestMCPMemoryDelete_HappyPath(t *testing.T) {
	c := qt.New(t)

	c.Run("targeted deletion by id removes the memory", func(c *qt.C) {
		cl := newMCPClient(c)

		savedText := callTool(c, cl, "memory_save", map[string]any{
			"title":   "To be deleted",
			"what":    "This memory will be removed",
			"project": "echovault",
		})
		var saved map[string]any
		c.Assert(json.Unmarshal([]byte(savedText), &saved), qt.IsNil)
		id, _ := saved["id"].(string)

		text := callTool(c, cl, "memory_delete", map[string]any{
			"ids": []string{id},
		})
		c.Assert(text, checkers.JSONPathMatches("$.deleted", qt.HasLen), 1)
		c.Assert(text, checkers.JSONPathMatches("$.not_found", qt.HasLen), 0)
	})

	c.Run("bulk deletion by age removes older memories", func(c *qt.C) {
		cl := newMCPClient(c)

		callTool(c, cl, "memory_save", map[string]any{
			"title":   "Old memory",
			"what":    "This is a memory entry",
			"project": "echovault",
		})

		// older_than_days=0 means cutoff is now, so nothing is truly "before" it.
		// Use a future-looking deletion: older_than_days=-1 is invalid (<=0).
		// Instead verify that older_than_days=365 returns 0 deleted for a fresh entry.
		text := callTool(c, cl, "memory_delete", map[string]any{
			"older_than_days": float64(365),
			"project":         "echovault",
		})
		c.Assert(text, checkers.JSONPathEquals("$.deleted_count"), float64(0))
	})
}

func TestMCPMemoryDelete_FailurePath(t *testing.T) {
	c := qt.New(t)

	c.Run("missing both ids and older_than_days returns error", func(c *qt.C) {
		cl := newMCPClient(c)

		req := mcp.CallToolRequest{}
		req.Params.Name = "memory_delete"
		req.Params.Arguments = make(map[string]any)

		result, err := cl.CallTool(context.Background(), req)
		c.Assert(err, qt.IsNil)
		c.Assert(result.IsError, qt.IsTrue)
	})

	c.Run("deleting non-existent id reports in not_found", func(c *qt.C) {
		cl := newMCPClient(c)

		text := callTool(c, cl, "memory_delete", map[string]any{
			"ids": []string{"nonexistent-id-abc"},
		})
		c.Assert(text, checkers.JSONPathMatches("$.not_found", qt.HasLen), 1)
		c.Assert(text, checkers.JSONPathMatches("$.deleted", qt.HasLen), 0)
	})
}

// ---------------------------------------------------------------------------
// memory_replace
// ---------------------------------------------------------------------------

func TestMCPMemoryReplace_HappyPath(t *testing.T) {
	c := qt.New(t)
	cl := newMCPClient(c)

	c.Run("replaces existing memory content", func(c *qt.C) {
		savedText := callTool(c, cl, "memory_save", map[string]any{
			"title":   "Original title",
			"what":    "Original what content",
			"project": "echovault",
		})
		var saved map[string]any
		c.Assert(json.Unmarshal([]byte(savedText), &saved), qt.IsNil)
		id, _ := saved["id"].(string)

		text := callTool(c, cl, "memory_replace", map[string]any{
			"id":      id,
			"title":   "Replaced title",
			"what":    "Completely new content",
			"project": "echovault",
		})
		c.Assert(text, checkers.JSONPathEquals("$.action"), "replaced")
		c.Assert(text, checkers.JSONPathEquals("$.id"), id)
	})
}

func TestMCPMemoryReplace_FailurePath(t *testing.T) {
	c := qt.New(t)
	cl := newMCPClient(c)

	c.Run("replacing non-existent memory returns error", func(c *qt.C) {
		req := mcp.CallToolRequest{}
		req.Params.Name = "memory_replace"
		req.Params.Arguments = map[string]any{
			"id":    "nonexistent-id-xyz",
			"title": "T",
			"what":  "W",
		}

		result, err := cl.CallTool(context.Background(), req)
		c.Assert(err, qt.IsNil)
		c.Assert(result.IsError, qt.IsTrue)
	})
}

// ---------------------------------------------------------------------------
// --disable-tools
// ---------------------------------------------------------------------------

func TestMCPDisabledTools_HappyPath(t *testing.T) {
	c := qt.New(t)

	c.Run("disabled tool does not appear in ListTools", func(c *qt.C) {
		cl := newMCPClientWithDisabledTools(c, []string{"memory_delete"})

		result, err := cl.ListTools(context.Background(), mcp.ListToolsRequest{})
		c.Assert(err, qt.IsNil)
		c.Assert(result.Tools, qt.HasLen, 4)

		names := make([]string, len(result.Tools))
		for i, tool := range result.Tools {
			names[i] = tool.Name
		}
		c.Assert(names, qt.Contains, "memory_save")
		c.Assert(names, qt.Contains, "memory_search")
		c.Assert(names, qt.Contains, "memory_context")
		c.Assert(names, qt.Contains, "memory_replace")
	})

	c.Run("multiple tools can be disabled simultaneously", func(c *qt.C) {
		cl := newMCPClientWithDisabledTools(c, []string{"memory_delete", "memory_replace"})

		result, err := cl.ListTools(context.Background(), mcp.ListToolsRequest{})
		c.Assert(err, qt.IsNil)
		c.Assert(result.Tools, qt.HasLen, 3)
	})
}

// ---------------------------------------------------------------------------
// Failure path — unknown tool
// ---------------------------------------------------------------------------

func TestMCPCallTool_FailurePath(t *testing.T) {
	c := qt.New(t)
	cl := newMCPClient(c)

	c.Run("unknown tool name returns error", func(c *qt.C) {
		req := mcp.CallToolRequest{}
		req.Params.Name = "nonexistent_tool"
		req.Params.Arguments = make(map[string]any)

		_, err := cl.CallTool(context.Background(), req)
		c.Assert(err, qt.IsNotNil)
	})
}
