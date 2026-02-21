// Package mcpcmd implements the `memory mcp` command.
package mcpcmd

import (
	"github.com/spf13/cobra"

	"github.com/go-ports/echovault/cmd/memory/shared"
	internalmcp "github.com/go-ports/echovault/internal/mcp"
)

// Command implements `memory mcp`.
type Command struct {
	ctx *shared.Context
	cmd *cobra.Command
}

// New creates the mcp command.
func New(ctx *shared.Context) *Command {
	c := &Command{ctx: ctx}
	c.cmd = &cobra.Command{
		Use:   "mcp",
		Short: "Start the EchoVault MCP server (stdio transport)",
		RunE:  c.run,
	}
	return c
}

// Cmd returns the cobra command.
func (c *Command) Cmd() *cobra.Command { return c.cmd }

func (*Command) run(cmd *cobra.Command, _ []string) error {
	return internalmcp.Serve(cmd.Context())
}
