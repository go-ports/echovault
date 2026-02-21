// Package mcp provides the stdio MCP server exposing memory tools for coding agents.
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"

	"github.com/go-ports/echovault/internal/buildinfo"
	"github.com/go-ports/echovault/internal/models"
	"github.com/go-ports/echovault/internal/service"
)

var validCategories = []string{"decision", "bug", "pattern", "learning", "context"}

const deleteDescription = `Delete one or more memories to keep your memory store lean and accurate.

Use this tool in two ways:

1. Targeted deletion (ids): remove specific memories whose content you have determined is
outdated, incorrect, or no longer relevant. Workflow — call memory_context or memory_search
to review existing memories, reason about which ones are stale (e.g. a decision was reversed,
a bug fix no longer applies, a pattern was removed from the codebase), then pass their IDs here.

2. Bulk deletion by age (older_than_days): remove all memories older than N days, optionally scoped to a project or category. Use this for periodic housekeeping.

At least one of ` + "`ids`" + ` or ` + "`older_than_days`" + ` must be provided.`

const replaceDescription = `Fully replace the content of an existing memory with new, correct information.

Prefer this over memory_save when the existing memory contains wrong or outdated information that must be overwritten rather than appended to. memory_save deduplicates and merges; memory_replace discards the old content entirely.

Workflow: use memory_search or memory_context to find the memory ID, then call memory_replace with the corrected content.

All fields except ` + "`id`" + ` behave the same as in memory_save.`

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

// NewServer creates and registers memory tools on a new MCP server.
// Tools listed in disabledTools are skipped during registration.
// It is intentionally separate from Serve so that tests and other callers can
// obtain a fully configured server without committing to the stdio transport.
func NewServer(svc *service.Service, disabledTools []string) *mcpserver.MCPServer {
	s := mcpserver.NewMCPServer("echovault", buildinfo.Version)
	registerTools(s, svc, disabledTools)
	return s
}

// Serve starts the stdio MCP server, blocking until stdin closes.
// Tools listed in disabledTools are not registered and will be unavailable.
func Serve(_ context.Context, disabledTools []string) error {
	svc, err := service.New("")
	if err != nil {
		return fmt.Errorf("mcp: init service: %w", err)
	}
	defer svc.Close()

	return mcpserver.ServeStdio(NewServer(svc, disabledTools))
}

// isDisabled returns true when name appears in the disabled list.
func isDisabled(name string, disabled []string) bool {
	for _, d := range disabled {
		if d == name {
			return true
		}
	}
	return false
}

// registerTools wires all MCP tools into the server, skipping any in disabledTools.
func registerTools(s *mcpserver.MCPServer, svc *service.Service, disabledTools []string) {
	if !isDisabled("memory_save", disabledTools) {
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
				mcp.Description("Project name (required)."),
				mcp.Required(),
			),
		), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return handleSave(ctx, svc, req)
		})
	}

	if !isDisabled("memory_search", disabledTools) {
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
	}

	if !isDisabled("memory_context", disabledTools) {
		s.AddTool(mcp.NewTool("memory_context",
			mcp.WithDescription(contextDescription),
			mcp.WithString("project",
				mcp.Description("Project name (required)."),
				mcp.Required(),
			),
			mcp.WithNumber("limit",
				mcp.Description("Max memories (default 10)"),
			),
		), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return handleContext(ctx, svc, req)
		})
	}

	if !isDisabled("memory_delete", disabledTools) {
		s.AddTool(mcp.NewTool("memory_delete",
			mcp.WithDescription(deleteDescription),
			mcp.WithArray("ids",
				mcp.Description("IDs (or prefixes) of memories to delete."),
				mcp.WithStringItems(),
			),
			mcp.WithNumber("older_than_days",
				mcp.Description("Delete all memories older than this many days."),
			),
			mcp.WithString("project",
				mcp.Description("Scope bulk deletion to this project (only with older_than_days)."),
			),
			mcp.WithString("category",
				mcp.Description("Scope bulk deletion to this category (only with older_than_days)."),
				mcp.Enum(validCategories...),
			),
		), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return handleDelete(ctx, svc, req)
		})
	}

	if !isDisabled("memory_replace", disabledTools) {
		s.AddTool(mcp.NewTool("memory_replace",
			mcp.WithDescription(replaceDescription),
			mcp.WithString("id",
				mcp.Description("ID (or prefix) of the memory to replace."),
				mcp.Required(),
			),
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
				mcp.Description("Full context for a future agent with zero context."),
			),
			mcp.WithString("project",
				mcp.Description("Project name."),
			),
		), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return handleReplace(ctx, svc, req)
		})
	}
}

// ---------------------------------------------------------------------------
// Tool handlers
// ---------------------------------------------------------------------------

func handleSave(ctx context.Context, svc *service.Service, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	project := req.GetString("project", "")
	if project == "" {
		return mcp.NewToolResultError("'project' is required"), nil
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
		return mcp.NewToolResultError("'project' is required"), nil
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
		message += " No memories found for project \"" + project + "\"."
	}

	return jsonResult(map[string]any{
		"total":    total,
		"showing":  len(memories),
		"memories": memories,
		"message":  message,
	})
}

func handleDelete(_ context.Context, svc *service.Service, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	ids := req.GetStringSlice("ids", make([]string, 0))
	olderThanDays := req.GetInt("older_than_days", 0)

	if len(ids) == 0 && olderThanDays <= 0 {
		return mcp.NewToolResultError("at least one of 'ids' or 'older_than_days' must be provided"), nil
	}

	if len(ids) > 0 {
		deleted := make([]string, 0, len(ids))
		notFound := make([]string, 0)
		for _, id := range ids {
			found, err := svc.Delete(id)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("delete %q: %s", id, err.Error())), nil
			}
			if found {
				deleted = append(deleted, id)
			} else {
				notFound = append(notFound, id)
			}
		}
		return jsonResult(map[string]any{
			"deleted":   deleted,
			"not_found": notFound,
		})
	}

	// Bulk deletion by age.
	project := req.GetString("project", "")
	category := req.GetString("category", "")
	count, err := svc.DeleteByFilter(project, category, olderThanDays)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return jsonResult(map[string]any{
		"deleted_count":   count,
		"older_than_days": olderThanDays,
		"project":         project,
		"category":        category,
	})
}

func handleReplace(ctx context.Context, svc *service.Service, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	id := req.GetString("id", "")
	if id == "" {
		return mcp.NewToolResultError("'id' is required"), nil
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

	result, err := svc.Replace(ctx, id, raw)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return jsonResult(map[string]any{
		"id":     result.ID,
		"action": result.Action,
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
