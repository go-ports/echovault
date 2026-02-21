// Package configcmd implements the `memory config` command group.
package configcmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/go-ports/echovault/cmd/memory/shared"
	"github.com/go-ports/echovault/internal/config"
)

const configTemplate = `# EchoVault configuration
# Docs: https://github.com/go-ports/echovault#configure-embeddings-optional

# Embedding provider for semantic search.
# Without this, keyword search (FTS5) still works.
embedding:
  provider: ollama              # ollama | openai
  model: nomic-embed-text
  # api_key: sk-...            # required for openai

# How memories are retrieved at session start.
# "auto" uses vectors when available, falls back to keywords.
context:
  semantic: auto                # auto | always | never
  topup_recent: true            # also include recent memories
`

// Command implements `memory config`.
type Command struct {
	ctx *shared.Context
	cmd *cobra.Command
}

// New creates the config command group.
func New(ctx *shared.Context) *Command {
	c := &Command{ctx: ctx}
	c.cmd = &cobra.Command{
		Use:   "config",
		Short: "Show or manage configuration",
		RunE:  c.runShow,
	}
	c.cmd.AddCommand(
		newConfigInit(ctx),
		newSetHome(ctx),
		newClearHome(ctx),
	)
	return c
}

// Cmd returns the cobra command.
func (c *Command) Cmd() *cobra.Command { return c.cmd }

func (c *Command) runShow(cmd *cobra.Command, _ []string) error {
	home, source := config.ResolveMemoryHome()
	if c.ctx.MemoryHome != "" {
		home = c.ctx.MemoryHome
		source = "flag"
	}
	cfg, err := config.Load(filepath.Join(home, "config.yaml"))
	if err != nil {
		return err
	}
	data := map[string]any{
		"embedding": map[string]any{
			"provider": cfg.Embedding.Provider,
			"model":    cfg.Embedding.Model,
			"base_url": cfg.Embedding.BaseURL,
			"api_key":  redactAPIKey(cfg.Embedding.APIKey),
		},
		"context": map[string]any{
			"semantic":     cfg.Context.Semantic,
			"topup_recent": cfg.Context.TopupRecent,
		},
		"memory_home":        home,
		"memory_home_source": source,
	}
	b, err := yaml.Marshal(data)
	if err != nil {
		return err
	}
	fmt.Fprint(cmd.OutOrStdout(), string(b))
	return nil
}

// ---------------------------------------------------------------------------
// config init
// ---------------------------------------------------------------------------

func newConfigInit(ctx *shared.Context) *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Generate a starter config.yaml",
		RunE: func(cmd *cobra.Command, _ []string) error {
			home := ctx.MemoryHome
			if home == "" {
				home = config.GetMemoryHome()
			}
			cfgPath := filepath.Join(home, "config.yaml")
			out := cmd.OutOrStdout()
			if _, err := os.Stat(cfgPath); err == nil && !force {
				fmt.Fprintf(out, "Config already exists at %s\n", cfgPath)
				fmt.Fprintln(out, "Use --force to overwrite.")
				return nil
			}
			if err := os.MkdirAll(home, 0o755); err != nil {
				return err
			}
			if err := os.WriteFile(cfgPath, []byte(configTemplate), 0o600); err != nil {
				return err
			}
			fmt.Fprintf(out, "Created %s\n", cfgPath)
			fmt.Fprintln(out, "Edit the file to configure your embedding provider.")
			return nil
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "Overwrite existing config")
	return cmd
}

// ---------------------------------------------------------------------------
// config set-home
// ---------------------------------------------------------------------------

func newSetHome(ctx *shared.Context) *cobra.Command {
	return &cobra.Command{
		Use:   "set-home <path>",
		Short: "Persist memory home location (used when MEMORY_HOME is unset)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			resolved, err := config.SetPersistedMemoryHome(args[0])
			if err != nil {
				return err
			}
			if err := os.MkdirAll(filepath.Join(resolved, "vault"), 0o755); err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "Persisted memory home: %s\n", resolved)
			fmt.Fprintln(out, "Override anytime with MEMORY_HOME.")
			_ = ctx // future use
			return nil
		},
	}
}

// ---------------------------------------------------------------------------
// config clear-home
// ---------------------------------------------------------------------------

func newClearHome(_ *shared.Context) *cobra.Command {
	return &cobra.Command{
		Use:   "clear-home",
		Short: "Remove persisted memory home location from global config",
		RunE: func(cmd *cobra.Command, _ []string) error {
			changed, err := config.ClearPersistedMemoryHome()
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			if changed {
				fmt.Fprintln(out, "Cleared persisted memory home setting.")
			} else {
				fmt.Fprintln(out, "No persisted memory home setting was found.")
			}
			return nil
		},
	}
}

func redactAPIKey(key string) string {
	if key != "" {
		return "<redacted>"
	}
	return ""
}
