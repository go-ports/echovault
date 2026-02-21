// Package uninstallcmd implements the `memory uninstall` command group.
package uninstallcmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/go-ports/echovault/cmd/memory/shared"
	"github.com/go-ports/echovault/internal/setup"
)

// Command implements `memory uninstall`.
type Command struct {
	ctx *shared.Context
	cmd *cobra.Command
}

// New creates the uninstall command group.
func New(ctx *shared.Context) *Command {
	c := &Command{ctx: ctx}
	c.cmd = &cobra.Command{
		Use:   "uninstall",
		Short: "Remove EchoVault hooks for an agent",
		RunE:  func(cmd *cobra.Command, _ []string) error { return cmd.Help() },
	}
	c.cmd.AddCommand(
		newUninstallClaudeCode(ctx),
		newUninstallCursor(ctx),
		newUninstallCodex(ctx),
		newUninstallOpencode(ctx),
	)
	return c
}

// Cmd returns the cobra command.
func (c *Command) Cmd() *cobra.Command { return c.cmd }

func newUninstallClaudeCode(_ *shared.Context) *cobra.Command {
	var configDir string
	var project bool
	cmd := &cobra.Command{
		Use:   "claude-code",
		Short: "Remove EchoVault from Claude Code",
		RunE: func(cmd *cobra.Command, _ []string) error {
			target := resolveConfigDir(".claude", configDir, project)
			result := setup.UninstallClaudeCode(target, project)
			fmt.Fprintln(cmd.OutOrStdout(), result.Message)
			return nil
		},
	}
	cmd.Flags().StringVar(&configDir, "config-dir", "", "Path to .claude directory")
	cmd.Flags().BoolVar(&project, "project", false, "Uninstall from current project instead of globally")
	return cmd
}

func newUninstallCursor(_ *shared.Context) *cobra.Command {
	var configDir string
	cmd := &cobra.Command{
		Use:   "cursor",
		Short: "Remove EchoVault from Cursor",
		RunE: func(cmd *cobra.Command, _ []string) error {
			target := resolveConfigDir(".cursor", configDir, false)
			result := setup.UninstallCursor(target)
			fmt.Fprintln(cmd.OutOrStdout(), result.Message)
			return nil
		},
	}
	cmd.Flags().StringVar(&configDir, "config-dir", "", "Path to .cursor directory")
	return cmd
}

func newUninstallCodex(_ *shared.Context) *cobra.Command {
	var configDir string
	cmd := &cobra.Command{
		Use:   "codex",
		Short: "Remove EchoVault from Codex",
		RunE: func(cmd *cobra.Command, _ []string) error {
			target := resolveConfigDir(".codex", configDir, false)
			result := setup.UninstallCodex(target)
			fmt.Fprintln(cmd.OutOrStdout(), result.Message)
			return nil
		},
	}
	cmd.Flags().StringVar(&configDir, "config-dir", "", "Path to .codex directory")
	return cmd
}

func newUninstallOpencode(_ *shared.Context) *cobra.Command {
	var project bool
	cmd := &cobra.Command{
		Use:   "opencode",
		Short: "Remove EchoVault from OpenCode",
		RunE: func(cmd *cobra.Command, _ []string) error {
			result := setup.UninstallOpencode(project)
			fmt.Fprintln(cmd.OutOrStdout(), result.Message)
			return nil
		},
	}
	cmd.Flags().BoolVar(&project, "project", false, "Uninstall from current project instead of globally")
	return cmd
}

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
