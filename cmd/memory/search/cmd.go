// Package searchcmd implements the `memory search` command.
package searchcmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/go-ports/echovault/cmd/memory/shared"
	"github.com/go-ports/echovault/internal/service"
)

// Command implements `memory search`.
type Command struct {
	ctx *shared.Context
	cmd *cobra.Command

	limit   int
	project bool
	source  string
}

// New creates the search command.
func New(ctx *shared.Context) *Command {
	c := &Command{ctx: ctx}
	c.cmd = &cobra.Command{
		Use:   "search <query>",
		Short: "Search memories using hybrid FTS5 + semantic search",
		Args:  cobra.ExactArgs(1),
		RunE:  c.run,
	}

	f := c.cmd.Flags()
	f.IntVar(&c.limit, "limit", 5, "Maximum number of results")
	f.BoolVar(&c.project, "project", false, "Filter to current project (current directory name)")
	f.StringVar(&c.source, "source", "", "Filter by source")

	return c
}

// Cmd returns the cobra command.
func (c *Command) Cmd() *cobra.Command { return c.cmd }

func (c *Command) run(cmd *cobra.Command, args []string) error {
	query := args[0]

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

	results, err := svc.Search(cmd.Context(), query, c.limit, projectName, c.source, true)
	if err != nil {
		return err
	}

	out := cmd.OutOrStdout()
	if len(results) == 0 {
		fmt.Fprintln(out, "No results found.")
		return nil
	}

	fmt.Fprintf(out, "\n Results (%d found) \n", len(results))

	for i, r := range results {
		src := ""
		if r.Source != "" {
			src = " | " + r.Source
		}
		detailsHint := ""
		if r.HasDetails {
			detailsHint = fmt.Sprintf("\n     Details: available (use `memory details %s`)", r.ID[:12])
		}

		createdAt := r.CreatedAt
		if len(createdAt) > 10 {
			createdAt = createdAt[:10]
		}

		fmt.Fprintf(out, "\n [%d] %s (score: %.2f)\n", i+1, r.Title, r.Score)
		fmt.Fprintf(out, "     %s | %s | %s%s\n", r.Category, createdAt, r.Project, src)
		fmt.Fprintf(out, "     What: %s\n", r.What)
		if r.Why != "" {
			fmt.Fprintf(out, "     Why: %s\n", r.Why)
		}
		if r.Impact != "" {
			fmt.Fprintf(out, "     Impact: %s\n", r.Impact)
		}
		if detailsHint != "" {
			fmt.Fprintln(out, detailsHint)
		}
	}
	return nil
}
