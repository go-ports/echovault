// Package initcmd implements the `memory init` command.
package initcmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/go-ports/echovault/cmd/memory/shared"
	"github.com/go-ports/echovault/internal/config"
)

// Command implements `memory init`.
type Command struct {
	ctx *shared.Context
	cmd *cobra.Command
}

// New creates the init command.
func New(ctx *shared.Context) *Command {
	c := &Command{ctx: ctx}
	c.cmd = &cobra.Command{
		Use:   "init",
		Short: "Initialize the memory vault",
		RunE:  c.run,
	}
	return c
}

// Cmd returns the cobra command.
func (c *Command) Cmd() *cobra.Command { return c.cmd }

func (c *Command) run(cmd *cobra.Command, _ []string) error {
	home := c.ctx.MemoryHome
	if home == "" {
		home = config.GetMemoryHome()
	}
	vaultDir := filepath.Join(home, "vault")
	if err := os.MkdirAll(vaultDir, 0o755); err != nil {
		return fmt.Errorf("init: %w", err)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Memory vault initialized at %s\n", home)
	return nil
}
