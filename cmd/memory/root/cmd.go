// Package rootcmd wires the root cobra.Command for the memory CLI binary.
package rootcmd

import (
	"github.com/spf13/cobra"

	configcmd "github.com/go-ports/echovault/cmd/memory/config"
	contextcmd "github.com/go-ports/echovault/cmd/memory/context"
	deletecmd "github.com/go-ports/echovault/cmd/memory/delete"
	detailscmd "github.com/go-ports/echovault/cmd/memory/details"
	initcmd "github.com/go-ports/echovault/cmd/memory/init"
	mcpcmd "github.com/go-ports/echovault/cmd/memory/mcp"
	reindexcmd "github.com/go-ports/echovault/cmd/memory/reindex"
	savecmd "github.com/go-ports/echovault/cmd/memory/save"
	searchcmd "github.com/go-ports/echovault/cmd/memory/search"
	sessionscmd "github.com/go-ports/echovault/cmd/memory/sessions"
	setupcmd "github.com/go-ports/echovault/cmd/memory/setup"
	"github.com/go-ports/echovault/cmd/memory/shared"
	uninstallcmd "github.com/go-ports/echovault/cmd/memory/uninstall"
)

// New creates and returns the root cobra.Command for the memory CLI.
func New() *cobra.Command {
	ctx := &shared.Context{}

	root := &cobra.Command{
		Use:           "memory",
		Short:         "EchoVault — local memory for coding agents",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE:          func(cmd *cobra.Command, _ []string) error { return cmd.Help() },
	}

	root.PersistentFlags().StringVar(
		&ctx.MemoryHome, "memory-home", "",
		"Override memory home directory (default: $MEMORY_HOME env → persisted config → ~/.memory)",
	)

	root.AddCommand(
		initcmd.New(ctx).Cmd(),
		savecmd.New(ctx).Cmd(),
		searchcmd.New(ctx).Cmd(),
		detailscmd.New(ctx).Cmd(),
		deletecmd.New(ctx).Cmd(),
		contextcmd.New(ctx).Cmd(),
		reindexcmd.New(ctx).Cmd(),
		sessionscmd.New(ctx).Cmd(),
		configcmd.New(ctx).Cmd(),
		setupcmd.New(ctx).Cmd(),
		uninstallcmd.New(ctx).Cmd(),
		mcpcmd.New(ctx).Cmd(),
	)

	return root
}
