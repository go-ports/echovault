// Package mcpcmd implements the `memory mcp` command.
package mcpcmd

import (
	"strings"

	"github.com/spf13/cobra"

	"github.com/go-ports/echovault/cmd/memory/shared"
	internalmcp "github.com/go-ports/echovault/internal/mcp"
)

// Command implements `memory mcp`.
type Command struct {
	ctx           *shared.Context
	cmd           *cobra.Command
	disabledTools string
}

// New creates the mcp command.
func New(ctx *shared.Context) *Command {
	c := &Command{ctx: ctx}
	c.cmd = &cobra.Command{
		Use:   "mcp",
		Short: "Start the EchoVault MCP server (stdio transport)",
		RunE:  c.run,
	}
	c.registerFlags()
	return c
}

// Cmd returns the cobra command.
func (c *Command) Cmd() *cobra.Command { return c.cmd }

func (c *Command) registerFlags() {
	c.cmd.Flags().StringVar(
		&c.disabledTools,
		"disable-tools",
		"",
		"Comma-separated list of MCP tool names to disable (e.g. memory_delete,memory_save).",
	)
}

func (c *Command) run(cmd *cobra.Command, _ []string) error {
	disabled := parseToolNames(c.disabledTools)
	return internalmcp.Serve(cmd.Context(), disabled)
}

// parseToolNames splits a comma-separated tool-name string into a trimmed slice.
// An empty or blank input returns nil.
func parseToolNames(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			result = append(result, t)
		}
	}
	return result
}
