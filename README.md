<p align="center">
  <img src="assets/echovault-gopher-icon.svg" width="120" height="120" alt="EchoVault" />
</p>

<h1 align="center">EchoVault — Go</h1>

<p align="center">
  Local memory for coding agents. Your agent remembers decisions, bugs, and context across sessions — no cloud, no API keys, no cost.
</p>

<p align="center">
  <a href="#install">Install</a> · <a href="#features">Features</a> · <a href="#how-it-works">How it works</a> · <a href="#commands">Commands</a>
</p>

---

This is the **Go port** of [EchoVault](https://github.com/mraza007/echovault). It is a single static binary with no Python runtime dependency. The vault format and MCP interface are fully compatible with the Python version — you can switch between them without losing any memories.

## Features

**Works with 4 agents** — Claude Code, Cursor, Codex, OpenCode. One command sets up MCP config for your agent.

**MCP native** — Runs as an MCP server exposing `memory_save`, `memory_search`, and `memory_context` as tools. Agents call them directly — no shell hooks needed.

**Local-first** — Everything stays on your machine. Memories are stored as Markdown in `~/.memory/vault/`, readable in Obsidian or any editor.

**Zero idle cost** — No background processes, no daemon, no RAM overhead. The MCP server only runs when the agent starts it.

**Hybrid search** — FTS5 keyword search works out of the box. Add Ollama or OpenAI for semantic vector search.

**Secret redaction** — 3-layer redaction strips API keys, passwords, and credentials before anything hits disk. Supports explicit `<redacted>` tags, pattern detection, and custom `.memoryignore` rules.

**Cross-agent** — Memories saved by Claude Code are searchable in Cursor, Codex, and OpenCode. One vault, many agents.

**Obsidian-compatible** — Session files are valid Markdown with YAML frontmatter. Point Obsidian at `~/.memory/vault/` and browse your agent's memory visually.

## Install

### Pre-built binary

Download the latest release for your platform from the [releases page](https://github.com/go-ports/echovault/releases) and place the binary somewhere on your `$PATH`.

### Build from source

```bash
git clone https://github.com/go-ports/echovault.git
cd echovault
make build          # produces ./bin/memory
sudo cp bin/memory /usr/local/bin/
```

> **CGO required.** The binary links against `go-sqlite3` and `sqlite-vec`, so a C compiler (gcc/clang) must be present. On macOS: `xcode-select --install`. On Debian/Ubuntu: `apt install build-essential`.

### First run

```bash
memory init
memory setup claude-code   # or: cursor, codex, opencode
```

That's it. `memory setup` installs the MCP server config automatically.

By default the config is installed globally. To install for a specific project:

```bash
cd ~/my-project
memory setup claude-code --project   # writes .mcp.json in project root
memory setup opencode --project      # writes opencode.json in project root
memory setup codex --project         # writes .codex/config.toml + AGENTS.md
```

### Configure embeddings (optional)

Embeddings enable semantic search. Without them, you still get fast keyword search via FTS5.

Generate a starter config:

```bash
memory config init
```

This creates `~/.memory/config.yaml` with sensible defaults:

```yaml
embedding:
  provider: ollama              # ollama | openai | openrouter
  model: nomic-embed-text

context:
  semantic: auto                # auto | always | never
  topup_recent: true
```

**What each section does:**

- **`embedding`** — How memories get turned into vectors for semantic search. `ollama` runs locally; `openai` and `openrouter` call cloud APIs. `nomic-embed-text` is a good local model for Ollama.
- **`context`** — Controls how memories are retrieved at session start. `auto` uses vector search when embeddings are available, falls back to keywords. `topup_recent` also includes recent memories so the agent has fresh context.

For cloud providers, add `api_key` under the provider section. API keys are redacted in `memory config` output.

### Configure memory location

By default, EchoVault stores data in `~/.memory`.

You can change that in two ways:

- `MEMORY_HOME=/path/to/memory` (highest priority, per-shell/per-process)
- `memory config set-home /path/to/memory` (persistent default)

Useful commands:

```bash
memory config set-home /path/to/memory
memory config clear-home
memory config
```

`memory config` shows both `memory_home` and `memory_home_source` (`env`, `config`, or `default`).

The `--memory-home` global flag overrides everything for a single invocation:

```bash
memory --memory-home /tmp/test-vault search "authentication"
```

## Usage

Once set up, your agent uses memory via MCP tools:

- **Session start** — agent calls `memory_context` to load prior decisions and context
- **During work** — agent calls `memory_search` to find relevant memories
- **Session end** — agent calls `memory_save` to persist decisions, bugs, and learnings

The MCP tool descriptions instruct agents to save and retrieve automatically. No manual prompting needed in most cases.

You can also use the CLI directly:

```bash
memory save --title "Switched to JWT auth" \
  --what "Replaced session cookies with JWT" \
  --why "Needed stateless auth for API" \
  --impact "All endpoints now require Bearer token" \
  --tags "auth,jwt" --category "decision" \
  --details "Context:
Options considered:
- Keep session cookies
- Move to JWT
Decision:
Tradeoffs:
Follow-up:"

memory search "authentication"
memory details <id>
memory context --project
```

For long details, use `--details-file notes.md`. To scaffold structured details automatically, use `--details-template`.

## How it works

```
~/.memory/
├── vault/                    # Obsidian-compatible Markdown
│   └── my-project/
│       └── 2026-02-01-session.md
├── index.db                  # SQLite: FTS5 + sqlite-vec
└── config.yaml               # Embedding provider config
```

- **Markdown vault** — one file per session per project, with YAML frontmatter
- **SQLite index** — FTS5 for keywords, sqlite-vec for semantic vectors
- **Compact pointers** — search returns ~50-token summaries; full details fetched on demand
- **3-layer redaction** — explicit tags, pattern matching, and `.memoryignore` rules

## Supported agents

| Agent | Setup command | What gets installed |
|-------|---------------|---------------------|
| Claude Code | `memory setup claude-code` | MCP server in `.mcp.json` (project) or `~/.claude.json` (global) |
| Cursor | `memory setup cursor` | MCP server in `.cursor/mcp.json` |
| Codex | `memory setup codex` | MCP server in `.codex/config.toml` + `AGENTS.md` fallback |
| OpenCode | `memory setup opencode` | MCP server in `opencode.json` (project) or `~/.config/opencode/opencode.json` (global) |

All agents share the same memory vault at your effective `memory_home` path (default `~/.memory/`). A memory saved by Claude Code is searchable from Cursor, Codex, or OpenCode.

## Commands

| Command | Description |
|---------|-------------|
| `memory init` | Create vault at effective memory home |
| `memory setup <agent>` | Install MCP server config for an agent |
| `memory uninstall <agent>` | Remove MCP server config for an agent |
| `memory save ...` | Save a memory (`--details-file` and `--details-template` supported) |
| `memory search "query"` | Hybrid FTS + semantic search |
| `memory details <id>` | Full details for a memory |
| `memory delete <id>` | Delete a memory by ID or prefix |
| `memory context --project` | List memories for current project |
| `memory sessions` | List session files |
| `memory config` | Show effective config |
| `memory config init` | Generate a starter config.yaml |
| `memory config set-home <path>` | Persist default memory location |
| `memory config clear-home` | Remove persisted memory location |
| `memory reindex` | Rebuild vectors after changing provider |
| `memory mcp` | Start the MCP server (stdio transport) |

### Global flags

| Flag | Description |
|------|-------------|
| `--memory-home <path>` | Override memory home for this invocation |
| `--help` | Show help for any command |

## Uninstall

```bash
memory uninstall claude-code   # or: cursor, codex, opencode
rm /usr/local/bin/memory
```

To also remove all stored memories: `rm -rf ~/.memory/`

## Privacy

Everything stays local by default. If you configure OpenAI or OpenRouter for embeddings, those API calls go to their servers. Use Ollama for fully local operation.

## License

MIT — see [LICENSE](../LICENSE).
