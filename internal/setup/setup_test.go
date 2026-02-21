package setup_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/go-ports/echovault/internal/checkers"
	"github.com/go-ports/echovault/internal/setup"
)

// ---------------------------------------------------------------------------
// SetupClaudeCode / UninstallClaudeCode
// ---------------------------------------------------------------------------

func TestSetupClaudeCode_HappyPath(t *testing.T) {
	c := qt.New(t)

	// project=true is used throughout to write into a controlled temp dir:
	// claudeMCPPath returns filepath.Dir(claudeHome)/.mcp.json, so we pass
	// a subdirectory of the temp root as claudeHome.

	c.Run("first install creates .mcp.json with echovault entry", func(c *qt.C) {
		tmp := t.TempDir()
		claudeHome := filepath.Join(tmp, ".claude")

		result := setup.SetupClaudeCode(claudeHome, true)
		c.Assert(result.Status, qt.Equals, "ok")
		c.Assert(result.Message, qt.Contains, "Installed")

		data, err := os.ReadFile(filepath.Join(tmp, ".mcp.json"))
		c.Assert(err, qt.IsNil)
		c.Assert(data, checkers.JSONPathEquals("$.mcpServers.echovault.command"), "memory")
		c.Assert(data, checkers.JSONPathEquals("$.mcpServers.echovault.type"), "stdio")
	})

	c.Run("second install is idempotent", func(c *qt.C) {
		tmp := t.TempDir()
		claudeHome := filepath.Join(tmp, ".claude")

		setup.SetupClaudeCode(claudeHome, true)
		result := setup.SetupClaudeCode(claudeHome, true)
		c.Assert(result.Status, qt.Equals, "ok")
		c.Assert(result.Message, qt.Equals, "Already installed")
	})

	c.Run("existing settings.json legacy hooks are cleaned on install", func(c *qt.C) {
		tmp := t.TempDir()
		claudeHome := filepath.Join(tmp, ".claude")
		err := os.MkdirAll(claudeHome, 0o755)
		c.Assert(err, qt.IsNil)

		// Write a settings.json with a legacy echovault mcpServers entry.
		settings := `{"mcpServers":{"echovault":{"command":"memory"}}}`
		err = os.WriteFile(filepath.Join(claudeHome, "settings.json"), []byte(settings), 0o600)
		c.Assert(err, qt.IsNil)

		result := setup.SetupClaudeCode(claudeHome, true)
		c.Assert(result.Status, qt.Equals, "ok")
		// The legacy entry should have been migrated and the new .mcp.json created.
		c.Assert(result.Message, qt.Contains, "Installed")

		data, err := os.ReadFile(filepath.Join(tmp, ".mcp.json"))
		c.Assert(err, qt.IsNil)
		c.Assert(data, checkers.JSONPathEquals("$.mcpServers.echovault.command"), "memory")
	})
}

func TestUninstallClaudeCode_HappyPath(t *testing.T) {
	c := qt.New(t)

	c.Run("installed entry is removed", func(c *qt.C) {
		tmp := t.TempDir()
		claudeHome := filepath.Join(tmp, ".claude")

		setup.SetupClaudeCode(claudeHome, true)
		result := setup.UninstallClaudeCode(claudeHome, true)
		c.Assert(result.Status, qt.Equals, "ok")
		c.Assert(result.Message, qt.Contains, "Removed")
	})

	c.Run("nothing to remove when not installed", func(c *qt.C) {
		tmp := t.TempDir()
		claudeHome := filepath.Join(tmp, ".claude")

		result := setup.UninstallClaudeCode(claudeHome, true)
		c.Assert(result.Status, qt.Equals, "ok")
		c.Assert(result.Message, qt.Equals, "Nothing to remove")
	})

	c.Run("reinstall succeeds after uninstall", func(c *qt.C) {
		tmp := t.TempDir()
		claudeHome := filepath.Join(tmp, ".claude")

		setup.SetupClaudeCode(claudeHome, true)
		setup.UninstallClaudeCode(claudeHome, true)
		result := setup.SetupClaudeCode(claudeHome, true)
		c.Assert(result.Status, qt.Equals, "ok")
		c.Assert(result.Message, qt.Contains, "Installed")
	})
}

// ---------------------------------------------------------------------------
// SetupCursor / UninstallCursor
// ---------------------------------------------------------------------------

func TestSetupCursor_HappyPath(t *testing.T) {
	c := qt.New(t)

	c.Run("install creates mcp.json in cursor home", func(c *qt.C) {
		tmp := t.TempDir()

		result := setup.SetupCursor(tmp)
		c.Assert(result.Status, qt.Equals, "ok")
		c.Assert(result.Message, qt.Contains, "Installed")

		data, err := os.ReadFile(filepath.Join(tmp, "mcp.json"))
		c.Assert(err, qt.IsNil)
		c.Assert(data, checkers.JSONPathEquals("$.mcpServers.echovault.command"), "memory")
	})

	c.Run("second install is idempotent", func(c *qt.C) {
		tmp := t.TempDir()

		setup.SetupCursor(tmp)
		result := setup.SetupCursor(tmp)
		c.Assert(result.Status, qt.Equals, "ok")
		c.Assert(result.Message, qt.Equals, "Already installed")
	})
}

func TestUninstallCursor_HappyPath(t *testing.T) {
	c := qt.New(t)

	c.Run("installed entry is removed", func(c *qt.C) {
		tmp := t.TempDir()

		setup.SetupCursor(tmp)
		result := setup.UninstallCursor(tmp)
		c.Assert(result.Status, qt.Equals, "ok")
		c.Assert(result.Message, qt.Contains, "Removed")
	})

	c.Run("nothing to remove when not installed", func(c *qt.C) {
		tmp := t.TempDir()

		result := setup.UninstallCursor(tmp)
		c.Assert(result.Status, qt.Equals, "ok")
		c.Assert(result.Message, qt.Equals, "Nothing to remove")
	})

	c.Run("reinstall succeeds after uninstall", func(c *qt.C) {
		tmp := t.TempDir()

		setup.SetupCursor(tmp)
		setup.UninstallCursor(tmp)
		result := setup.SetupCursor(tmp)
		c.Assert(result.Status, qt.Equals, "ok")
		c.Assert(result.Message, qt.Contains, "Installed")
	})
}

// ---------------------------------------------------------------------------
// SetupCodex / UninstallCodex
// ---------------------------------------------------------------------------

func TestSetupCodex_HappyPath(t *testing.T) {
	c := qt.New(t)

	c.Run("install creates AGENTS.md and config.toml", func(c *qt.C) {
		tmp := t.TempDir()

		result := setup.SetupCodex(tmp)
		c.Assert(result.Status, qt.Equals, "ok")
		c.Assert(result.Message, qt.Contains, "Installed")

		agentsData, err := os.ReadFile(filepath.Join(tmp, "AGENTS.md"))
		c.Assert(err, qt.IsNil)
		c.Assert(string(agentsData), qt.Contains, "## EchoVault")
		c.Assert(string(agentsData), qt.Contains, "memory save")

		tomlData, err := os.ReadFile(filepath.Join(tmp, "config.toml"))
		c.Assert(err, qt.IsNil)
		c.Assert(string(tomlData), qt.Contains, "mcp_servers.echovault")
	})

	c.Run("second install is idempotent", func(c *qt.C) {
		tmp := t.TempDir()

		setup.SetupCodex(tmp)
		result := setup.SetupCodex(tmp)
		c.Assert(result.Status, qt.Equals, "ok")
		c.Assert(result.Message, qt.Equals, "Already installed")
	})

	c.Run("install appends EchoVault section to existing AGENTS.md", func(c *qt.C) {
		tmp := t.TempDir()
		agentsPath := filepath.Join(tmp, "AGENTS.md")
		err := os.WriteFile(agentsPath, []byte("# Existing Instructions\n\nDo things.\n"), 0o600) // #nosec G306 -- test fixture, not a sensitive file
		c.Assert(err, qt.IsNil)

		result := setup.SetupCodex(tmp)
		c.Assert(result.Status, qt.Equals, "ok")

		data, err := os.ReadFile(agentsPath)
		c.Assert(err, qt.IsNil)
		content := string(data)
		c.Assert(content, qt.Contains, "# Existing Instructions")
		c.Assert(content, qt.Contains, "## EchoVault")
	})
}

func TestUninstallCodex_HappyPath(t *testing.T) {
	c := qt.New(t)

	c.Run("removes EchoVault section from AGENTS.md and entry from config.toml", func(c *qt.C) {
		tmp := t.TempDir()

		setup.SetupCodex(tmp)
		result := setup.UninstallCodex(tmp)
		c.Assert(result.Status, qt.Equals, "ok")
		c.Assert(result.Message, qt.Contains, "Removed")

		agentsData, err := os.ReadFile(filepath.Join(tmp, "AGENTS.md"))
		c.Assert(err, qt.IsNil)
		c.Assert(strings.Contains(string(agentsData), "## EchoVault"), qt.IsFalse)

		tomlData, err := os.ReadFile(filepath.Join(tmp, "config.toml"))
		c.Assert(err, qt.IsNil)
		c.Assert(strings.Contains(string(tomlData), "mcp_servers.echovault"), qt.IsFalse)
	})

	c.Run("nothing to remove when not installed", func(c *qt.C) {
		tmp := t.TempDir()

		result := setup.UninstallCodex(tmp)
		c.Assert(result.Status, qt.Equals, "ok")
		c.Assert(result.Message, qt.Equals, "Nothing to remove")
	})

	c.Run("reinstall succeeds after uninstall", func(c *qt.C) {
		tmp := t.TempDir()

		setup.SetupCodex(tmp)
		setup.UninstallCodex(tmp)
		result := setup.SetupCodex(tmp)
		c.Assert(result.Status, qt.Equals, "ok")
		c.Assert(result.Message, qt.Contains, "Installed")
	})

	c.Run("uninstall preserves preceding content in AGENTS.md", func(c *qt.C) {
		tmp := t.TempDir()
		agentsPath := filepath.Join(tmp, "AGENTS.md")
		err := os.WriteFile(agentsPath, []byte("# Keep This\n\nExisting instructions.\n"), 0o600) // #nosec G306 -- test fixture, not a sensitive file
		c.Assert(err, qt.IsNil)

		setup.SetupCodex(tmp)
		setup.UninstallCodex(tmp)

		data, err := os.ReadFile(agentsPath)
		c.Assert(err, qt.IsNil)
		c.Assert(string(data), qt.Contains, "Keep This")
		c.Assert(strings.Contains(string(data), "## EchoVault"), qt.IsFalse)
	})
}
