// Package deletecmd implements the `memory delete` command.
package deletecmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/go-ports/echovault/cmd/memory/shared"
	"github.com/go-ports/echovault/internal/service"
)

// Command implements `memory delete`.
type Command struct {
	ctx *shared.Context
	cmd *cobra.Command
}

// New creates the delete command.
func New(ctx *shared.Context) *Command {
	c := &Command{ctx: ctx}
	c.cmd = &cobra.Command{
		Use:   "delete <memory-id>",
		Short: "Delete a memory by ID or prefix",
		Args:  cobra.ExactArgs(1),
		RunE:  c.run,
	}
	return c
}

// Cmd returns the cobra command.
func (c *Command) Cmd() *cobra.Command { return c.cmd }

func (c *Command) run(cmd *cobra.Command, args []string) error {
	svc, err := service.New(c.ctx.MemoryHome)
	if err != nil {
		return err
	}
	defer svc.Close()

	deleted, err := svc.Delete(args[0])
	if err != nil {
		return err
	}
	if deleted {
		fmt.Fprintf(cmd.OutOrStdout(), "Deleted memory %s\n", args[0])
	} else {
		fmt.Fprintf(cmd.OutOrStdout(), "No memory found for %s\n", args[0])
	}
	return nil
}
