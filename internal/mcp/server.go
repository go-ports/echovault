// Package mcp provides the stdio MCP server exposing memory tools for coding agents.
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"

	"github.com/go-ports/echovault/internal/buildinfo"
	"github.com/go-ports/echovault/internal/models"
	"github.com/go-ports/echovault/internal/service"
)

var validCategories = []string{"decision", "bug", "pattern", "learning", "context"}

const saveDescription = `
Save a memory for future sessions. You MUST call this before ending any session where you made changes, fixed bugs, made decisions, or learned something. This is not optional — failing to save means the next session starts from zero.

Save when you:
- Made an architectural or design decision (chose X over Y)
- Fixed a bug (include root cause and solution)
- Discovered a non-obvious pattern or gotcha
- Learned something about the codebase not obvious from code
- Set up infrastructure, tooling, or configuration
- The user corrected you or clarified a requirement

Do NOT save: trivial changes (typos, formatting), info obvious from reading the code, or duplicates of existing memories. Write for a future agent with zero context.

When filling ` + "`details`" + `, prefer this structure:
- Context
- Options considered
- Decision
- Tradeoffs
- Follow-up`

const searchDescription = `Search memories using keyword and semantic search. Returns matching memories ranked by relevance. You MUST call this at session start before doing any work, and whenever the user's request relates to a topic that may have prior context.` //nolint:lll

const contextDescription = `Get memory context for the current project. You MUST call this at session start to load prior decisions, bugs, and context. Do not skip this step — prior sessions contain decisions and context that directly affect your current task. Use memory_search for specific topics.` //nolint:lll

// NewServer creates and registers all memory tools on a new MCP server.
// It is intentionally separate from Serve so that tests and other callers can
// obtain a fully configured server without committing to the stdio transport.
func NewServer(svc *service.Service) *mcpserver.MCPServer {
	s := mcpserver.NewMCPServer("echovault", buildinfo.Version)
	registerTools(s, svc)
	return s
}

// Serve starts the stdio MCP server, blocking until stdin closes.
func Serve(_ context.Context) error {
	svc, err := service.New("")
	if err != nil {
		return fmt.Errorf("mcp: init service: %w", err)
	}
	defer svc.Close()

	return mcpserver.ServeStdio(NewServer(svc))
}

// registerTools wires all three MCP tools into the server.
func registerTools(s *mcpserver.MCPServer, svc *service.Service) {
	s.AddTool(mcp.NewTool("memory_save",
		mcp.WithDescription(saveDescription),
		mcp.WithString("title",
			mcp.Description("Short title, max 60 chars."),
			mcp.Required(),
		),
		mcp.WithString("what",
			mcp.Description("1-2 sentences. The essence a future agent needs."),
			mcp.Required(),
		),
		mcp.WithString("why",
			mcp.Description("Reasoning behind the decision or fix."),
		),
		mcp.WithString("impact",
			mcp.Description("What changed as a result."),
		),
		mcp.WithArray("tags",
			mcp.Description("Relevant tags."),
			mcp.WithStringItems(),
		),
		mcp.WithString("category",
			mcp.Description("decision: chose X over Y. bug: fixed a problem. pattern: reusable gotcha. learning: non-obvious discovery. context: project setup/architecture."),
			mcp.Enum(validCategories...),
		),
		mcp.WithArray("related_files",
			mcp.Description("File paths involved."),
			mcp.WithStringItems(),
		),
		mcp.WithString("details",
			mcp.Description("Full context for a future agent with zero context. Prefer: Context, Options considered, Decision, Tradeoffs, Follow-up."),
		),
		mcp.WithString("project",
			mcp.Description("Project name. Auto-detected from cwd if omitted."),
		),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleSave(ctx, svc, req)
	})

	s.AddTool(mcp.NewTool("memory_search",
		mcp.WithDescription(searchDescription),
		mcp.WithString("query",
			mcp.Description("Search terms"),
			mcp.Required(),
		),
		mcp.WithNumber("limit",
			mcp.Description("Max results (default 5)"),
		),
		mcp.WithString("project",
			mcp.Description("Filter to project."),
		),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleSearch(ctx, svc, req)
	})

	s.AddTool(mcp.NewTool("memory_context",
		mcp.WithDescription(contextDescription),
		mcp.WithString("project",
			mcp.Description("Project name. Auto-detected from cwd if omitted."),
		),
		mcp.WithNumber("limit",
			mcp.Description("Max memories (default 10)"),
		),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleContext(ctx, svc, req)
	})
}

// ---------------------------------------------------------------------------
// Tool handlers
// ---------------------------------------------------------------------------

func handleSave(ctx context.Context, svc *service.Service, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	project := req.GetString("project", "")
	if project == "" {
		if cwd, err := os.Getwd(); err == nil {
			project = filepath.Base(cwd)
		}
	}

	category := req.GetString("category", "")
	if !isValidCategory(category) {
		category = "context"
	}

	raw := &models.RawMemoryInput{
		Title:        truncate(req.GetString("title", ""), 60),
		What:         req.GetString("what", ""),
		Why:          req.GetString("why", ""),
		Impact:       req.GetString("impact", ""),
		Tags:         req.GetStringSlice("tags", make([]string, 0)),
		Category:     category,
		RelatedFiles: req.GetStringSlice("related_files", make([]string, 0)),
		Details:      req.GetString("details", ""),
	}

	result, err := svc.Save(ctx, raw, project)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return jsonResult(map[string]any{
		"id":        result.ID,
		"file_path": result.FilePath,
		"action":    result.Action,
		"warnings":  result.Warnings,
	})
}

func handleSearch(ctx context.Context, svc *service.Service, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	query := req.GetString("query", "")
	limit := req.GetInt("limit", 5)
	if limit <= 0 {
		limit = 5
	}
	project := req.GetString("project", "")

	results, err := svc.Search(ctx, query, limit, project, "", true)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	clean := make([]map[string]any, 0, len(results))
	for _, r := range results {
		clean = append(clean, map[string]any{
			"id":          r.ID,
			"title":       r.Title,
			"what":        r.What,
			"why":         r.Why,
			"impact":      r.Impact,
			"category":    r.Category,
			"tags":        parseTags(r.Tags),
			"project":     r.Project,
			"created_at":  truncate(r.CreatedAt, 10),
			"score":       roundTwo(r.Score),
			"has_details": r.HasDetails,
		})
	}
	return jsonResult(clean)
}

func handleContext(ctx context.Context, svc *service.Service, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	project := req.GetString("project", "")
	if project == "" {
		if cwd, err := os.Getwd(); err == nil {
			project = filepath.Base(cwd)
		}
	}
	limit := req.GetInt("limit", 10)
	if limit <= 0 {
		limit = 10
	}

	results, total, err := svc.GetContext(ctx, limit, project, "", "", "never", false)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	memories := make([]map[string]any, 0, len(results))
	for _, r := range results {
		tagsRaw, _ := r["tags"].(string)
		dateStr, _ := r["created_at"].(string)
		memories = append(memories, map[string]any{
			"id":       r["id"],
			"title":    r["title"],
			"category": r["category"],
			"tags":     parseTags(tagsRaw),
			"date":     formatDate(dateStr),
		})
	}

	message := "Use memory_search for specific topics. IMPORTANT: You MUST call memory_save before this session ends if you make any changes, decisions, or discoveries."
	if total == 0 {
		message += " No memories found for project \"" + project + "\". If this is unexpected, retry with an explicit project name (e.g. memory_context(project: \"myproject\"))."
	}

	return jsonResult(map[string]any{
		"total":    total,
		"showing":  len(memories),
		"memories": memories,
		"message":  message,
	})
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func isValidCategory(c string) bool {
	for _, v := range validCategories {
		if v == c {
			return true
		}
	}
	return false
}

func jsonResult(v any) (*mcp.CallToolResult, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(string(b)), nil
}

func truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) > maxLen {
		return string(runes[:maxLen])
	}
	return s
}

func parseTags(raw string) []string {
	if raw == "" {
		return make([]string, 0)
	}
	var tags []string
	if err := json.Unmarshal([]byte(raw), &tags); err != nil {
		return make([]string, 0)
	}
	if tags == nil {
		return make([]string, 0)
	}
	return tags
}

func formatDate(dateStr string) string {
	if len(dateStr) >= 10 {
		dateStr = dateStr[:10]
	}
	t, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return dateStr
	}
	return t.Format("Jan 02")
}

// roundTwo rounds f to 2 decimal places.
func roundTwo(f float64) float64 {
	return math.Round(f*100) / 100
}
