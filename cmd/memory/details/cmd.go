// Package detailscmd implements the `memory details` command.
package detailscmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/go-ports/echovault/cmd/memory/shared"
	"github.com/go-ports/echovault/internal/service"
)

// Command implements `memory details`.
type Command struct {
	ctx *shared.Context
	cmd *cobra.Command
}

// New creates the details command.
func New(ctx *shared.Context) *Command {
	c := &Command{ctx: ctx}
	c.cmd = &cobra.Command{
		Use:   "details <memory-id>",
		Short: "Fetch full details for a specific memory",
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

	detail, err := svc.GetDetails(args[0])
	if err != nil {
		return err
	}
	if detail == nil {
		fmt.Fprintf(cmd.OutOrStdout(), "No details found for memory %s\n", args[0])
		return nil
	}
	fmt.Fprintln(cmd.OutOrStdout(), detail.Body)
	return nil
}
