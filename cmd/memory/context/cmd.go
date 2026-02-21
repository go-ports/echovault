// Package contextcmd implements the `memory context` command.
package contextcmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/go-ports/echovault/cmd/memory/shared"
	"github.com/go-ports/echovault/internal/config"
	"github.com/go-ports/echovault/internal/service"
)

// Command implements `memory context`.
type Command struct {
	ctx *shared.Context
	cmd *cobra.Command

	project      bool
	source       string
	limit        int
	query        string
	semanticMode string
	showConfig   bool
	outputFormat string
}

// New creates the context command.
func New(ctx *shared.Context) *Command {
	c := &Command{ctx: ctx}
	c.cmd = &cobra.Command{
		Use:   "context",
		Short: "Output memory pointers for agent context injection",
		RunE:  c.run,
	}

	f := c.cmd.Flags()
	f.BoolVar(&c.project, "project", false, "Filter to current project (current directory name)")
	f.StringVar(&c.source, "source", "", "Filter by source")
	f.IntVar(&c.limit, "limit", 10, "Maximum number of pointers")
	f.StringVar(&c.query, "query", "", "Semantic search query for filtering")
	f.BoolVar(&c.showConfig, "show-config", false, "Show effective configuration and exit")
	f.StringVar(&c.outputFormat, "format", "hook", "Output format: hook | agents-md")

	// --semantic controls the search mode: always|never|auto.
	f.StringVar(&c.semanticMode, "semantic", "", "Force semantic search (always|never|auto)")

	return c
}

// Cmd returns the cobra command.
func (c *Command) Cmd() *cobra.Command { return c.cmd }

func (c *Command) run(cmd *cobra.Command, _ []string) error {
	out := cmd.OutOrStdout()
	if c.showConfig {
		home := c.ctx.MemoryHome
		if home == "" {
			home = config.GetMemoryHome()
		}
		cfg, err := config.Load(filepath.Join(home, "config.yaml"))
		if err != nil {
			return err
		}
		data := map[string]any{
			"embedding": map[string]any{
				"provider": cfg.Embedding.Provider,
				"model":    cfg.Embedding.Model,
				"base_url": cfg.Embedding.BaseURL,
				"api_key":  redactAPIKey(cfg.Embedding.APIKey),
			},
			"context": map[string]any{
				"semantic":     cfg.Context.Semantic,
				"topup_recent": cfg.Context.TopupRecent,
			},
			"memory_home": home,
		}
		for k, v := range data {
			fmt.Fprintf(out, "%s: %v\n", k, v)
		}
		return nil
	}

	var projectName string
	if c.project {
		if cwd, err := os.Getwd(); err == nil {
			projectName = filepath.Base(cwd)
		}
	}

	svc, err := service.New(c.ctx.MemoryHome)
	if err != nil {
		return err
	}
	defer svc.Close()

	topupRecent := svc.Config.Context.TopupRecent
	results, total, err := svc.GetContext(
		cmd.Context(),
		c.limit,
		projectName,
		c.source,
		c.query,
		c.semanticMode,
		topupRecent,
	)
	if err != nil {
		return err
	}

	if len(results) == 0 {
		fmt.Fprintln(out, "No memories found.")
		return nil
	}

	if c.outputFormat == "agents-md" {
		fmt.Fprintln(out, "## Memory Context")
		fmt.Fprintln(out)
	}

	fmt.Fprintf(out, "Available memories (%d total, showing %d):\n", total, len(results))

	for _, r := range results {
		dateStr, _ := r["created_at"].(string)
		if len(dateStr) > 10 {
			dateStr = dateStr[:10]
		}
		dateDisplay := dateStr
		if t, err := time.Parse("2006-01-02", dateStr); err == nil {
			dateDisplay = t.Format("Jan 02")
		}

		title, _ := r["title"].(string)
		if title == "" {
			title = "Untitled"
		}
		cat, _ := r["category"].(string)
		tagsRaw, _ := r["tags"].(string)
		tagsList := parseTags(tagsRaw)

		catPart := ""
		if cat != "" {
			catPart = " [" + cat + "]"
		}
		tagsPart := ""
		if len(tagsList) > 0 {
			tagsPart = " [" + joinStrings(tagsList, ",") + "]"
		}

		fmt.Fprintf(out, "- [%s] %s%s%s\n", dateDisplay, title, catPart, tagsPart)
	}

	if c.outputFormat == "agents-md" {
		fmt.Fprintln(out)
	}
	fmt.Fprintln(out, "Use `memory search <query>` for full details on any memory.")
	return nil
}

func redactAPIKey(key string) string {
	if key != "" {
		return "<redacted>"
	}
	return ""
}

func parseTags(raw string) []string {
	if raw == "" {
		return nil
	}
	var tags []string
	_ = json.Unmarshal([]byte(raw), &tags)
	return tags
}

func joinStrings(ss []string, sep string) string {
	result := ""
	for i, s := range ss {
		if i > 0 {
			result += sep
		}
		result += s
	}
	return result
}
