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

	cl, err := mcpclient.NewInProcessClient(internalmcp.NewServer(svc))
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
	c.Assert(result.Tools, qt.HasLen, 3)

	names := make([]string, len(result.Tools))
	for i, tool := range result.Tools {
		names[i] = tool.Name
	}
	c.Assert(names, qt.Contains, "memory_save")
	c.Assert(names, qt.Contains, "memory_search")
	c.Assert(names, qt.Contains, "memory_context")
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
