// Package setup installs and uninstalls EchoVault integrations for supported
// coding agents (Claude Code, Cursor, Codex, OpenCode).
package setup

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

//go:embed skill.md
var skillMD []byte

// Result is the return value from all Setup/Uninstall functions.
type Result struct {
	Status  string // always "ok"
	Message string
}

func ok(msg string) Result          { return Result{Status: "ok", Message: msg} }
func okf(f string, a ...any) Result { return ok(fmt.Sprintf(f, a...)) }

// ---------------------------------------------------------------------------
// MCP config entries
// ---------------------------------------------------------------------------

var mcpConfig = map[string]any{
	"command": "memory",
	"args":    []any{"mcp"},
	"type":    "stdio",
}

var opencodeMCPConfig = map[string]any{
	"type":    "local",
	"command": []any{"memory", "mcp"},
}

// ---------------------------------------------------------------------------
// Default path helpers
// ---------------------------------------------------------------------------

// DefaultClaudeHome returns the default ~/.claude directory.
func DefaultClaudeHome() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".claude")
}

// DefaultCursorHome returns the default ~/.cursor directory.
func DefaultCursorHome() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".cursor")
}

// DefaultCodexHome returns the default ~/.codex directory.
func DefaultCodexHome() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".codex")
}

// ---------------------------------------------------------------------------
// JSON helpers
// ---------------------------------------------------------------------------

func readJSON(path string) map[string]any {
	data, err := os.ReadFile(path)
	if err != nil {
		return make(map[string]any)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil || m == nil {
		return make(map[string]any)
	}
	return m
}

func writeJSON(path string, data map[string]any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	return os.WriteFile(path, b, 0o644) // #nosec G306 -- agent config files (MCP server entries) do not contain secrets
}

// ---------------------------------------------------------------------------
// TOML helpers (text-based; only handles the [mcp_servers.echovault] table)
// ---------------------------------------------------------------------------

const tomlMCPSection = "\n[mcp_servers.echovault]\ncommand = \"memory\"\nargs = [\"mcp\"]\n"

func hasTOMLMCPSection(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	return strings.Contains(string(data), "mcp_servers.echovault")
}

func appendTOMLMCPSection(path string) (bool, error) {
	if hasTOMLMCPSection(path) {
		return false, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return false, err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return false, err
	}
	defer f.Close()
	_, err = f.WriteString(tomlMCPSection)
	return err == nil, err
}

func removeTOMLMCPSection(path string) (bool, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	content := string(data)
	if !strings.Contains(content, "mcp_servers.echovault") {
		return false, nil
	}
	// Process line-by-line: skip the [mcp_servers.echovault] header and its
	// key-value pairs up to the next TOML table header or EOF.
	lines := strings.Split(content, "\n")
	result := make([]string, 0, len(lines))
	inSection := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "[mcp_servers.echovault]" {
			inSection = true
			continue
		}
		if inSection && strings.HasPrefix(trimmed, "[") {
			inSection = false
		}
		if !inSection {
			result = append(result, line)
		}
	}
	cleaned := strings.TrimRight(strings.Join(result, "\n"), "\n") + "\n"
	return true, os.WriteFile(path, []byte(cleaned), 0o644) // #nosec G306 -- agent TOML config is not a sensitive credential file
}

// ---------------------------------------------------------------------------
// JSON mcpServers helpers (Claude Code, Cursor)
// ---------------------------------------------------------------------------

func installMCPServers(path string) (bool, error) {
	data := readJSON(path)
	servers, _ := data["mcpServers"].(map[string]any)
	if servers == nil {
		servers = make(map[string]any)
		data["mcpServers"] = servers
	}
	if _, exists := servers["echovault"]; exists {
		return false, nil
	}
	servers["echovault"] = mcpConfig
	return true, writeJSON(path, data)
}

func uninstallMCPServers(path string) (bool, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return false, nil
	}
	data := readJSON(path)
	servers, _ := data["mcpServers"].(map[string]any)
	if _, exists := servers["echovault"]; !exists {
		return false, nil
	}
	delete(servers, "echovault")
	if len(servers) == 0 {
		delete(data, "mcpServers")
	}
	if len(data) == 0 {
		return true, os.Remove(path)
	}
	return true, writeJSON(path, data)
}

// ---------------------------------------------------------------------------
// JSON mcp helpers (OpenCode)
// ---------------------------------------------------------------------------

func installOpencodeMCP(path string) (bool, error) {
	data := readJSON(path)
	mcp, _ := data["mcp"].(map[string]any)
	if mcp == nil {
		mcp = make(map[string]any)
		data["mcp"] = mcp
	}
	if _, exists := mcp["echovault"]; exists {
		return false, nil
	}
	mcp["echovault"] = opencodeMCPConfig
	return true, writeJSON(path, data)
}

func uninstallOpencodeMCP(path string) (bool, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return false, nil
	}
	data := readJSON(path)
	mcp, _ := data["mcp"].(map[string]any)
	if _, exists := mcp["echovault"]; !exists {
		return false, nil
	}
	delete(mcp, "echovault")
	if len(mcp) == 0 {
		delete(data, "mcp")
	}
	if len(data) == 0 {
		return true, os.Remove(path)
	}
	return true, writeJSON(path, data)
}

// ---------------------------------------------------------------------------
// Old-hook removal (Claude Code / Cursor)
// ---------------------------------------------------------------------------

// removeOldHooks purges legacy EchoVault hook groups from a settings map.
// Returns the event names from which hooks were removed.
func removeOldHooks(settings map[string]any) []string { //nolint:gocognit // complexity is inherent to iterating nested hook structures across multiple event types
	hooks, _ := settings["hooks"].(map[string]any)
	if hooks == nil {
		return nil
	}
	fragments := []string{"memory context", "memory auto-save"}
	var removed []string
	for event, raw := range hooks {
		groups, ok := raw.([]any)
		if !ok {
			continue
		}
		filtered := groups[:0]
		for _, g := range groups {
			group, _ := g.(map[string]any)
			inner, _ := group["hooks"].([]any)
			keep := true
			for _, h := range inner {
				hm, _ := h.(map[string]any)
				cmd, _ := hm["command"].(string)
				for _, frag := range fragments {
					if strings.Contains(cmd, frag) {
						keep = false
						break
					}
				}
				if !keep {
					break
				}
			}
			if keep {
				filtered = append(filtered, g)
			}
		}
		if len(filtered) != len(groups) {
			removed = append(removed, event)
			if len(filtered) > 0 {
				hooks[event] = filtered
			} else {
				delete(hooks, event)
			}
		}
	}
	if len(hooks) == 0 {
		delete(settings, "hooks")
	}
	return removed
}

// ---------------------------------------------------------------------------
// Skill install / uninstall
// ---------------------------------------------------------------------------

func installSkill(agentHome string) (bool, error) {
	skillDir := filepath.Join(agentHome, "skills", "echovault")
	skillPath := filepath.Join(skillDir, "SKILL.md")
	if _, err := os.Stat(skillPath); err == nil {
		return false, nil // already exists
	}
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		return false, err
	}
	return true, os.WriteFile(skillPath, skillMD, 0o644) // #nosec G306 -- SKILL.md does not contain secrets
}

func uninstallSkill(agentHome string) (bool, error) {
	skillDir := filepath.Join(agentHome, "skills", "echovault")
	info, err := os.Lstat(skillDir)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return true, os.Remove(skillDir)
	}
	return true, os.RemoveAll(skillDir)
}

// ---------------------------------------------------------------------------
// Claude Code path helper
// ---------------------------------------------------------------------------

//revive:disable:flag-parameter
func claudeMCPPath(claudeHome string, project bool) string {
	if project {
		return filepath.Join(filepath.Dir(claudeHome), ".mcp.json")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".claude.json")
}

//revive:enable:flag-parameter

// ---------------------------------------------------------------------------
// SetupClaudeCode
// ---------------------------------------------------------------------------

// SetupClaudeCode installs EchoVault into Claude Code.
// claudeHome defaults to ~/.claude when empty.
//
//revive:disable:flag-parameter
func SetupClaudeCode(claudeHome string, project bool) Result {
	if claudeHome == "" {
		claudeHome = DefaultClaudeHome()
	}
	var installed []string

	// Clean legacy hooks from settings.json.
	settingsPath := filepath.Join(claudeHome, "settings.json")
	if _, err := os.Stat(settingsPath); err == nil { //nolint:nestif // settings migration logic requires checking multiple nested JSON structures
		settings := readJSON(settingsPath)
		removed := removeOldHooks(settings)
		if len(removed) > 0 {
			installed = append(installed, fmt.Sprintf("removed old hooks: %s", strings.Join(removed, ", ")))
		}
		if servers, ok := settings["mcpServers"].(map[string]any); ok {
			if _, has := servers["echovault"]; has {
				delete(servers, "echovault")
				if len(servers) == 0 {
					delete(settings, "mcpServers")
				}
				installed = append(installed, "migrated mcpServers from settings.json")
			}
		}
		_ = writeJSON(settingsPath, settings)
	}

	// Remove old skill.
	_, _ = uninstallSkill(claudeHome)

	// Install MCP config.
	mcpPath := claudeMCPPath(claudeHome, project)
	if added, err := installMCPServers(mcpPath); err == nil && added {
		scope := ".mcp.json"
		if !project {
			scope = "~/.claude.json"
		}
		installed = append(installed, fmt.Sprintf("mcpServers in %s", scope))
	}

	if len(installed) > 0 {
		return okf("Installed: %s", strings.Join(installed, ", "))
	}
	return ok("Already installed")
}

//revive:enable:flag-parameter

// ---------------------------------------------------------------------------
// SetupCursor
// ---------------------------------------------------------------------------

// SetupCursor installs EchoVault into Cursor.
// cursorHome defaults to ~/.cursor when empty.
func SetupCursor(cursorHome string) Result {
	if cursorHome == "" {
		cursorHome = DefaultCursorHome()
	}
	var installed []string

	// Remove old hooks.json.
	oldHooksPath := filepath.Join(cursorHome, "hooks.json")
	if _, err := os.Stat(oldHooksPath); err == nil {
		data := readJSON(oldHooksPath)
		hooks, _ := data["hooks"].(map[string]any)
		for event, raw := range hooks {
			groups, _ := raw.([]any)
			filtered := groups[:0]
			for _, g := range groups {
				gm, _ := g.(map[string]any)
				cmd, _ := gm["command"].(string)
				if !strings.Contains(cmd, "memory context") {
					filtered = append(filtered, g)
				}
			}
			if len(filtered) != len(groups) {
				installed = append(installed, fmt.Sprintf("removed old hook: %s", event))
				if len(filtered) > 0 {
					hooks[event] = filtered
				} else {
					delete(hooks, event)
				}
			}
		}
		_ = writeJSON(oldHooksPath, data)
	}

	// Remove old skill.
	_, _ = uninstallSkill(cursorHome)

	// Install MCP config.
	mcpPath := filepath.Join(cursorHome, "mcp.json")
	if added, err := installMCPServers(mcpPath); err == nil && added {
		installed = append(installed, "mcpServers")
	}

	if len(installed) > 0 {
		return okf("Installed: %s", strings.Join(installed, ", "))
	}
	return ok("Already installed")
}

// ---------------------------------------------------------------------------
// SetupCodex
// ---------------------------------------------------------------------------

const codexAgentsMDSection = `
## EchoVault — Persistent Memory

You have persistent memory across sessions. Use it.

### Session start — MANDATORY

Before doing any work, retrieve context:

` + "```bash\nmemory context --project\n```" + `

Search for relevant memories:

` + "```bash\nmemory search \"<relevant terms>\"\n```" + `

When results show "Details: available", fetch them:

` + "```bash\nmemory details <memory-id>\n```" + `

### Session end — MANDATORY

Before finishing any task that involved changes, debugging, decisions, or learning, save a memory:

` + "```bash" + `
memory save \
  --title "Short descriptive title" \
  --what "What happened or was decided" \
  --why "Reasoning behind it" \
  --impact "What changed as a result" \
  --tags "tag1,tag2,tag3" \
  --category "decision" \
  --related-files "path/to/file1,path/to/file2" \
  --source "codex" \
  --details "Context:

             Options considered:
             - Option A
             - Option B

             Decision:
             Tradeoffs:
             Follow-up:"
` + "```" + `

Categories: ` + "`decision`, `bug`, `pattern`, `learning`, `context`." + `

### Rules

- Retrieve before working. Save before finishing. No exceptions.
- Never include API keys, secrets, or credentials.
- Search before saving to avoid duplicates.
`

// SetupCodex installs EchoVault into Codex (AGENTS.md + config.toml MCP).
// codexHome defaults to ~/.codex when empty.
func SetupCodex(codexHome string) Result {
	if codexHome == "" {
		codexHome = DefaultCodexHome()
	}
	var installed []string

	// AGENTS.md.
	agentsPath := filepath.Join(codexHome, "AGENTS.md")
	existing, _ := os.ReadFile(agentsPath)
	if !strings.Contains(string(existing), "## EchoVault") {
		if err := os.MkdirAll(filepath.Dir(agentsPath), 0o755); err == nil {
			content := strings.TrimRight(string(existing), "\n") + "\n" + codexAgentsMDSection
			if err := os.WriteFile(agentsPath, []byte(content), 0o644); err == nil { // #nosec G306 -- AGENTS.md does not contain secrets
				installed = append(installed, "AGENTS.md")
			}
		}
	}

	// config.toml MCP entry.
	tomlPath := filepath.Join(codexHome, "config.toml")
	if added, err := appendTOMLMCPSection(tomlPath); err == nil && added {
		installed = append(installed, "config.toml")
	}

	// Skill (legacy — Codex doesn't support skills but we install for future).
	if added, err := installSkill(codexHome); err == nil && added {
		installed = append(installed, "skill")
	}

	if len(installed) == 0 {
		return ok("Already installed")
	}
	msg := fmt.Sprintf("Installed: %s", strings.Join(installed, ", "))
	msg += "\nNote: Auto-persist is only available for Claude Code. Codex relies on AGENTS.md instructions for saving."
	return ok(msg)
}

// ---------------------------------------------------------------------------
// SetupOpencode
// ---------------------------------------------------------------------------

//revive:disable:flag-parameter
func opencodeMCPPath(project bool) string {
	if project {
		cwd, _ := os.Getwd()
		return filepath.Join(cwd, "opencode.json")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "opencode", "opencode.json")
}

//revive:enable:flag-parameter

// SetupOpencode installs EchoVault into OpenCode.
//
//revive:disable:flag-parameter
func SetupOpencode(project bool) Result {
	path := opencodeMCPPath(project)
	if added, err := installOpencodeMCP(path); err == nil && added {
		scope := "opencode.json"
		if !project {
			scope = "~/.config/opencode/opencode.json"
		}
		return okf("Installed: mcp in %s", scope)
	}
	return ok("Already installed")
}

//revive:enable:flag-parameter

// ---------------------------------------------------------------------------
// Uninstall functions
// ---------------------------------------------------------------------------

// UninstallClaudeCode removes EchoVault from Claude Code.
func UninstallClaudeCode(claudeHome string, project bool) Result {
	if claudeHome == "" {
		claudeHome = DefaultClaudeHome()
	}
	var removed []string

	// Target scope.
	mcpPath := claudeMCPPath(claudeHome, project)
	if done, err := uninstallMCPServers(mcpPath); err == nil && done {
		removed = append(removed, fmt.Sprintf("mcpServers from %s", filepath.Base(mcpPath)))
	}

	// Legacy settings.json.
	settingsPath := filepath.Join(claudeHome, "settings.json")
	if _, err := os.Stat(settingsPath); err == nil { //nolint:nestif // legacy settings cleanup requires checking multiple nested JSON structures
		settings := readJSON(settingsPath)
		if servers, ok := settings["mcpServers"].(map[string]any); ok {
			if _, has := servers["echovault"]; has {
				delete(servers, "echovault")
				if len(servers) == 0 {
					delete(settings, "mcpServers")
				}
				removed = append(removed, "legacy mcpServers from settings.json")
			}
		}
		old := removeOldHooks(settings)
		removed = append(removed, old...)
		_ = writeJSON(settingsPath, settings)
	}

	// Skill.
	if done, err := uninstallSkill(claudeHome); err == nil && done {
		removed = append(removed, "skill")
	}

	if len(removed) > 0 {
		return okf("Removed: %s", strings.Join(removed, ", "))
	}
	return ok("Nothing to remove")
}

// UninstallCursor removes EchoVault from Cursor.
func UninstallCursor(cursorHome string) Result {
	if cursorHome == "" {
		cursorHome = DefaultCursorHome()
	}
	var removed []string

	mcpPath := filepath.Join(cursorHome, "mcp.json")
	if done, err := uninstallMCPServers(mcpPath); err == nil && done {
		removed = append(removed, "mcpServers")
	}

	// Old hooks.json.
	oldHooksPath := filepath.Join(cursorHome, "hooks.json")
	if _, err := os.Stat(oldHooksPath); err == nil {
		data := readJSON(oldHooksPath)
		hooks, _ := data["hooks"].(map[string]any)
		for event, raw := range hooks {
			groups, _ := raw.([]any)
			filtered := groups[:0]
			for _, g := range groups {
				gm, _ := g.(map[string]any)
				cmd, _ := gm["command"].(string)
				if !strings.Contains(cmd, "memory context") {
					filtered = append(filtered, g)
				}
			}
			if len(filtered) != len(groups) {
				removed = append(removed, event)
				if len(filtered) > 0 {
					hooks[event] = filtered
				} else {
					delete(hooks, event)
				}
			}
		}
		_ = writeJSON(oldHooksPath, data)
	}

	if done, err := uninstallSkill(cursorHome); err == nil && done {
		removed = append(removed, "skill")
	}

	if len(removed) > 0 {
		return okf("Removed: %s", strings.Join(removed, ", "))
	}
	return ok("Nothing to remove")
}

// replaceEchoVaultSection is the ReplaceAllStringFunc callback for removeCodexAgentsSection.
// It removes the matched EchoVault block, preserving any following ## heading.
func replaceEchoVaultSection(m string) string {
	headingStart := strings.Index(m, "##")
	if headingStart < 0 {
		return ""
	}
	headingEnd := strings.Index(m[headingStart:], "\n")
	if headingEnd < 0 {
		return ""
	}
	body := m[headingStart+headingEnd+1:]
	if idx := strings.Index(body, "\n## "); idx >= 0 {
		return "\n" + body[idx+1:]
	}
	return ""
}

// removeCodexAgentsSection strips the ## EchoVault block from AGENTS.md content.
// Returns the cleaned content and true when a change was made.
func removeCodexAgentsSection(content string) (string, bool) {
	if !strings.Contains(content, "## EchoVault") {
		return content, false
	}
	re := regexp.MustCompile(`(?s)\n*## EchoVault[^\n]*\n.*?(?:\n## |\z)`)
	cleaned := re.ReplaceAllStringFunc(content, replaceEchoVaultSection)
	return strings.TrimRight(cleaned, "\n") + "\n", true
}

// UninstallCodex removes EchoVault from Codex (AGENTS.md + config.toml).
func UninstallCodex(codexHome string) Result {
	if codexHome == "" {
		codexHome = DefaultCodexHome()
	}
	var removed []string

	// Remove AGENTS.md section.
	agentsPath := filepath.Join(codexHome, "AGENTS.md")
	if data, err := os.ReadFile(agentsPath); err == nil {
		if cleaned, changed := removeCodexAgentsSection(string(data)); changed {
			_ = os.WriteFile(agentsPath, []byte(cleaned), 0o644) // #nosec G306 -- AGENTS.md does not contain secrets
			removed = append(removed, "AGENTS.md")
		}
	}

	// Remove config.toml entry.
	tomlPath := filepath.Join(codexHome, "config.toml")
	if done, err := removeTOMLMCPSection(tomlPath); err == nil && done {
		removed = append(removed, "config.toml")
	}

	if done, err := uninstallSkill(codexHome); err == nil && done {
		removed = append(removed, "skill")
	}

	if len(removed) > 0 {
		return okf("Removed: %s", strings.Join(removed, ", "))
	}
	return ok("Nothing to remove")
}

// UninstallOpencode removes EchoVault from OpenCode.
//
//revive:disable:flag-parameter
func UninstallOpencode(project bool) Result {
	path := opencodeMCPPath(project)
	if done, err := uninstallOpencodeMCP(path); err == nil && done {
		scope := "opencode.json"
		if !project {
			scope = "~/.config/opencode/opencode.json"
		}
		return okf("Removed: mcp from %s", scope)
	}
	return ok("Nothing to remove")
}

//revive:enable:flag-parameter
