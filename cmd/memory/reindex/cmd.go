// Package reindexcmd implements the `memory reindex` command.
package reindexcmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/go-ports/echovault/cmd/memory/shared"
	"github.com/go-ports/echovault/internal/service"
)

// Command implements `memory reindex`.
type Command struct {
	ctx *shared.Context
	cmd *cobra.Command
}

// New creates the reindex command.
func New(ctx *shared.Context) *Command {
	c := &Command{ctx: ctx}
	c.cmd = &cobra.Command{
		Use:   "reindex",
		Short: "Rebuild vector index with current embedding provider",
		RunE:  c.run,
	}
	return c
}

// Cmd returns the cobra command.
func (c *Command) Cmd() *cobra.Command { return c.cmd }

func (c *Command) run(cmd *cobra.Command, _ []string) error {
	svc, err := service.New(c.ctx.MemoryHome)
	if err != nil {
		return err
	}
	defer svc.Close()

	total, err := svc.CountMemories("", "")
	if err != nil {
		return err
	}

	out := cmd.OutOrStdout()
	if total == 0 {
		fmt.Fprintln(out, "No memories to reindex.")
		return nil
	}

	fmt.Fprintf(out, "Reindexing %d memories with %s/%s...\n",
		total, svc.Config.Embedding.Provider, svc.Config.Embedding.Model)

	result, err := svc.Reindex(cmd.Context(), func(current, count int) {
		fmt.Fprintf(out, "\r  %d/%d", current, count)
		if current == count {
			fmt.Fprintln(out)
		}
	})
	if err != nil {
		return err
	}

	fmt.Fprintf(out, "Re-indexed %d memories with %s (%d dims)\n",
		result.Count, result.Model, result.Dim)
	return nil
}
