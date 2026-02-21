// Package setupcmd implements the `memory setup` command group.
package setupcmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/go-ports/echovault/cmd/memory/shared"
	"github.com/go-ports/echovault/internal/setup"
)

// Command implements `memory setup`.
type Command struct {
	ctx *shared.Context
	cmd *cobra.Command
}

// New creates the setup command group.
func New(ctx *shared.Context) *Command {
	c := &Command{ctx: ctx}
	c.cmd = &cobra.Command{
		Use:   "setup",
		Short: "Install EchoVault hooks for an agent",
		RunE:  func(cmd *cobra.Command, _ []string) error { return cmd.Help() },
	}
	c.cmd.AddCommand(
		newSetupClaudeCode(ctx),
		newSetupCursor(ctx),
		newSetupCodex(ctx),
		newSetupOpencode(ctx),
	)
	return c
}

// Cmd returns the cobra command.
func (c *Command) Cmd() *cobra.Command { return c.cmd }

// ---------------------------------------------------------------------------
// setup claude-code
// ---------------------------------------------------------------------------

func newSetupClaudeCode(_ *shared.Context) *cobra.Command {
	var configDir string
	var project bool
	cmd := &cobra.Command{
		Use:   "claude-code",
		Short: "Install EchoVault MCP server into Claude Code",
		RunE: func(cmd *cobra.Command, _ []string) error {
			target := resolveConfigDir(".claude", configDir, project)
			result := setup.SetupClaudeCode(target, project)
			fmt.Fprintln(cmd.OutOrStdout(), result.Message)
			return nil
		},
	}
	cmd.Flags().StringVar(&configDir, "config-dir", "", "Path to .claude directory")
	cmd.Flags().BoolVar(&project, "project", false, "Install in current project instead of globally")
	return cmd
}

// ---------------------------------------------------------------------------
// setup cursor
// ---------------------------------------------------------------------------

func newSetupCursor(_ *shared.Context) *cobra.Command {
	var configDir string
	var project bool
	cmd := &cobra.Command{
		Use:   "cursor",
		Short: "Install EchoVault MCP server into Cursor",
		RunE: func(cmd *cobra.Command, _ []string) error {
			target := resolveConfigDir(".cursor", configDir, project)
			result := setup.SetupCursor(target)
			fmt.Fprintln(cmd.OutOrStdout(), result.Message)
			return nil
		},
	}
	cmd.Flags().StringVar(&configDir, "config-dir", "", "Path to .cursor directory")
	cmd.Flags().BoolVar(&project, "project", false, "Install in current project instead of globally")
	return cmd
}

// ---------------------------------------------------------------------------
// setup codex
// ---------------------------------------------------------------------------

func newSetupCodex(_ *shared.Context) *cobra.Command {
	var configDir string
	var project bool
	cmd := &cobra.Command{
		Use:   "codex",
		Short: "Install EchoVault into Codex AGENTS.md and config.toml",
		RunE: func(cmd *cobra.Command, _ []string) error {
			target := resolveConfigDir(".codex", configDir, project)
			result := setup.SetupCodex(target)
			fmt.Fprintln(cmd.OutOrStdout(), result.Message)
			return nil
		},
	}
	cmd.Flags().StringVar(&configDir, "config-dir", "", "Path to .codex directory")
	cmd.Flags().BoolVar(&project, "project", false, "Install in current project instead of globally")
	return cmd
}

// ---------------------------------------------------------------------------
// setup opencode
// ---------------------------------------------------------------------------

func newSetupOpencode(_ *shared.Context) *cobra.Command {
	var project bool
	cmd := &cobra.Command{
		Use:   "opencode",
		Short: "Install EchoVault MCP server into OpenCode",
		RunE: func(cmd *cobra.Command, _ []string) error {
			result := setup.SetupOpencode(project)
			fmt.Fprintln(cmd.OutOrStdout(), result.Message)
			return nil
		},
	}
	cmd.Flags().BoolVar(&project, "project", false, "Install in current project instead of globally")
	return cmd
}

// ---------------------------------------------------------------------------
// Helper
// ---------------------------------------------------------------------------

//revive:disable:flag-parameter
func resolveConfigDir(dotDir, configDir string, project bool) string {
	if configDir != "" {
		return configDir
	}
	if project {
		cwd, _ := os.Getwd()
		return filepath.Join(cwd, dotDir)
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, dotDir)
}

//revive:enable:flag-parameter
