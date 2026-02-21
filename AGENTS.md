# AGENTS.md

This file provides guidance to AI coding agents when working with code in this repository.

## Project Overview

EchoVault is a local memory system for coding agents. It persists decisions, bugs, patterns, and context across sessions in a SQLite database (with FTS5 full-text search and sqlite-vec vector embeddings) and exposes them through a CLI binary (`memory`) and an MCP (Model Context Protocol) stdio server.

## Build & Development Commands

All builds, tests, and linting MUST be run through the Makefile. Do NOT run `go build`, `go test`, `golangci-lint`, or other tools directly — always use the corresponding `make` target.

```bash
# Build for current platform (optimised)
make build

# Build without cross-compile flags (fastest local build)
make build-native

# Run all tests (CGO_CFLAGS set automatically for FTS5)
make test

# Run a single test (exception: no make target, use go test directly)
CGO_CFLAGS="-DSQLITE_ENABLE_FTS5" go test -run TestName ./path/to/package

# Run all linters (nolintguard, qtlint, golangci-lint) — must pass before commit
make lint

# Run linters with auto-fix
make lint-fix

# Install/tidy dependencies
make deps

# Clean build artifacts
make clean

# See all available targets
make help
```

## CGO Requirements

This project requires CGO throughout — `CGO_ENABLED=1` is always set by the Makefile. Two C-backed SQLite extensions are in use:

- **`github.com/mattn/go-sqlite3`** — SQLite driver compiled via CGO
- **`github.com/asg017/sqlite-vec-go-bindings`** — vector similarity search extension via CGO

When running tests directly (not via `make test`), always set:

```bash
CGO_CFLAGS="-DSQLITE_ENABLE_FTS5" go test ./...
```

Omitting `CGO_CFLAGS="-DSQLITE_ENABLE_FTS5"` causes "no such module: fts5" errors at runtime.

## Code Style (golangci-lint)

The following rules are enforced by `.golangci.yml`. Code MUST comply to pass CI.

### Import Organization (gci formatter)

Imports must be organized in three groups, separated by blank lines:
1. Standard library
2. Third-party packages
3. Local packages (`github.com/go-ports/echovault/...`)

```go
import (
    "context"
    "fmt"

    "github.com/spf13/cobra"

    "github.com/go-ports/echovault/cmd/memory/shared"
    "github.com/go-ports/echovault/internal/service"
)
```

### Import Aliases (importas)

- `github.com/frankban/quicktest` MUST be aliased as `qt`
- Import aliases must be lowercase alphanumeric (e.g., `mcpserver`, not `mcpServer`)
- No `v1`, `v2` style aliases

### Slice and Map Initialization (revive)

Use `make()` for empty slices and maps:

```go
// GOOD
data := make([]byte, 0)
results := make(map[string]int)

// BAD - will be flagged
data := []byte{}
results := map[string]int{}
```

### Function Signatures (revive)

**Repeated argument types** - use short form:
```go
// GOOD
func connect(host, port string) error

// BAD
func connect(host string, port string) error
```

**Return value limit** - maximum 3 return values.

**Receiver naming** - max 3 characters, consistent:
```go
// GOOD
func (s *Service) Save(...) error
func (c *Command) Cmd() *cobra.Command

// GOOD (unused receiver): omit the receiver name entirely
func (*Command) run(_ *cobra.Command, _ []string) error

// BAD - too long
func (svc *Service) Save(...) error

// BAD (unused receiver): do not use underscore receiver name
func (_ *Command) run(_ *cobra.Command, _ []string) error
```

### Error Handling Style (revive)

**Early return** - prefer early return to reduce nesting:
```go
// GOOD
func process(data []byte) error {
    if len(data) == 0 {
        return ErrEmptyData
    }
    // ... process data
    return nil
}
```

**No superfluous else** - don't use `else` after `return`/`break`/`continue`.

**Indent error flow** - happy path at lowest indentation:
```go
// GOOD
db, err := sql.Open("sqlite3", path)
if err != nil {
    return fmt.Errorf("db.Open: %w", err)
}
defer db.Close()
```

### Naming Conventions (revive)

**Error variables** - must start with `Err` (exported) or `err` (unexported):
```go
var ErrDimensionMismatch = errors.New("embedding dimension mismatch")  // exported
var errInternal = errors.New("internal error")                         // unexported
```

**Unused parameters** - prefix with `_`:
```go
func (*Command) run(_ *cobra.Command, _ []string) error {
    // cobra args not used
}
```

### File Naming (revive)

Filenames must be lowercase with underscores only (`^[_a-z][_a-z0-9]*.go$`):
```
server.go                ✓
server_test.go           ✓
server_internal_test.go  ✓
httpHandler.go           ✗ (no camelCase)
```

### nolint Comments

`nolint` directives require a specific linter name and an explanation comment (enforced by `nolintlint`):

```go
// GOOD
func init() { //nolint:gochecknoinits // registers sqlite-vec extension before any DB connection opens
    vec.Auto()
}

//nolint:lll // lll and errcheck are the only linters that may omit the explanation

// BAD - missing linter name
//nolint // ❌

// BAD - missing explanation
//nolint:gochecknoinits // ❌
```

### Prohibited Patterns

**No `init()` functions** except:
- `cmd/` packages (explicitly exempt)
- `internal/db/db.go` — the sqlite-vec extension must be registered before any connection opens; it carries a `//nolint:gochecknoinits` with explanation

**No `io/ioutil`** - use `io` or `os` instead.

**No naked returns** (except in ≤2 line functions).

**No dot imports**.

### Complexity Limits

- **Function length**: max 240 lines / 160 statements
- **Cyclomatic complexity**: max 21
- **Cognitive complexity**: max 30
- **Nesting depth**: max 6 levels
- **Line length**: max 240 characters

These limits are relaxed for `_test.go` files.

### Documentation Requirements

- All exported functions, types, and constants MUST have Go doc comments.
- Non-trivial unexported functions SHOULD have doc comments.
- Doc comments must explain **what** it does, **why** it exists (if not obvious), and **how** it works at a high level (for complex logic).

## Required Libraries

### CLI Framework: `github.com/spf13/cobra`

All CLI commands MUST use Cobra with a hierarchical folder structure.

**Folder Structure Pattern:**
```
cmd/memory/
├── main.go              # Entry point, calls run() → newRoot().ExecuteContext(ctx)
├── root.go              # Root command, wires all subcommands
├── shared/
│   └── context.go       # Shared context (MemoryHome flag)
├── save/
│   └── cmd.go           # `memory save` command
├── search/
│   └── cmd.go           # `memory search` command
└── mcp/
    └── cmd.go           # `memory mcp` command
```

**Command Implementation Pattern:**

```go
// file: cmd/memory/save/cmd.go
package savecmd

import (
    "github.com/spf13/cobra"

    "github.com/go-ports/echovault/cmd/memory/shared"
    "github.com/go-ports/echovault/internal/service"
)

// Command implements `memory save`.
type Command struct {
    ctx   *shared.Context
    cmd   *cobra.Command
    title string
    // ... other flag fields
}

// New creates the save command.
func New(ctx *shared.Context) *Command {
    c := &Command{ctx: ctx}
    c.cmd = &cobra.Command{
        Use:   "save",
        Short: "Save a memory to the current session",
        RunE:  c.run,
    }
    c.registerFlags()
    return c
}

// Cmd returns the cobra command.
func (c *Command) Cmd() *cobra.Command { return c.cmd }

func (c *Command) registerFlags() {
    f := c.cmd.Flags()
    f.StringVar(&c.title, "title", "", "Title of the memory (required)")
}

func (c *Command) run(cmd *cobra.Command, _ []string) error {
    svc, err := service.New(c.ctx.MemoryHome)
    if err != nil {
        return err
    }
    defer svc.Close()
    // ...
    return nil
}
```

**Conventions:**
- Each command lives in its own package/folder under `cmd/memory/`
- Package name is `<name>cmd` (e.g., `package savecmd`, `package mcpcmd`)
- `New(ctx *shared.Context) *Command` constructor pattern
- `Cmd() *cobra.Command` to expose the command
- Use `RunE` (not `Run`) for error handling
- Register flags in a `registerFlags()` method

### Error Handling

Use the standard library. No third-party error-wrapping package is used.

```go
import (
    "errors"
    "fmt"
)

// Sentinel errors at package level
var ErrDimensionMismatch = errors.New("embedding dimension mismatch")

// Wrap with context — use package.Function: prefix at package boundaries
if err != nil {
    return fmt.Errorf("service.New: create vault dir: %w", err)
}
```

**Conventions:**
- `errors.New()` for sentinel errors (enables `errors.Is` checking)
- `fmt.Errorf("context: %w", err)` to add context and preserve the chain
- Prefix error messages with `package.Function:` at package boundaries
- Lowercase error messages, no trailing punctuation
- Never build error strings with `fmt.Sprintf` without `%w` wrapping

### Logging: `log/slog` (standard library)

All logging MUST use the standard library `slog` package.

```go
import "log/slog"

slog.Info("memory saved", "id", id, "project", project)
slog.Warn("failed to load .memoryignore", "err", err)
slog.Debug("embedding generated", "dim", len(vec))
```

**Log Levels:**
- `Debug`: Verbose diagnostics, embedding/search timing
- `Info`: Lifecycle events (memory saved, reindex complete)
- `Warn`: Non-fatal degraded-mode issues (embedding errors, missing config — operation continues with fallback)
- `Error`: Failures that require attention

### MCP Server: `github.com/mark3labs/mcp-go`

The MCP stdio server is implemented in `internal/mcp/server.go` and exposes three tools:
`memory_save`, `memory_search`, `memory_context`.

```go
import (
    "github.com/mark3labs/mcp-go/mcp"
    mcpserver "github.com/mark3labs/mcp-go/server"
)

s := mcpserver.NewMCPServer("echovault", buildinfo.Version)
s.AddTool(mcp.NewTool("memory_save",
    mcp.WithDescription("..."),
    mcp.WithString("title", mcp.Required()),
), handlerFunc)
return mcpserver.ServeStdio(s)
```

### Testing: `github.com/frankban/quicktest` (aliased as `qt`)

All tests MUST use `quicktest` for assertions. The alias `qt` is enforced by `.golangci.yml`.

```go
import qt "github.com/frankban/quicktest"

func TestOpen_HappyPath(t *testing.T) {
    c := qt.New(t)

    db, err := db.Open(t.TempDir() + "/index.db")
    c.Assert(err, qt.IsNil)
    defer db.Close()
}
```

**Common qt Checkers:**
- `qt.Equals` — exact equality
- `qt.DeepEquals` — deep equality for structs, slices, maps
- `qt.IsNil` / `qt.IsNotNil` — nil checks
- `qt.IsTrue` / `qt.IsFalse` — boolean checks
- `qt.ErrorMatches` — error message matches regex pattern
- `qt.ErrorIs` — `errors.Is` style checking (wrapped errors)
- `qt.Contains` — substring/element containment
- `qt.HasLen` — length check

**Prefer `qt.ErrorIs` for sentinel errors** — it properly handles wrapped errors.

**Custom JSON Checkers (`internal/checkers`):**

For tests that assert on JSON output, use the project-local checkers:

```go
import "github.com/go-ports/echovault/internal/checkers"

// Assert that $.id in the JSON response equals "abc"
c.Assert(jsonOutput, checkers.JSONPathEquals("$.id"), "abc")

// Assert with a custom checker (e.g. qt.Contains)
c.Assert(jsonOutput, checkers.JSONPathMatches("$.title", qt.Contains), "partial")
```

**qtlint Rules** (enforced by `github.com/go-extras/qtlint`):

```go
// BAD: Use qt.IsNotNil instead of qt.Not(qt.IsNil)
c.Assert(got, qt.Not(qt.IsNil))     // ❌
c.Assert(got, qt.IsNotNil)           // ✓

// BAD: Use qt.IsFalse instead of qt.Not(qt.IsTrue)
c.Assert(value, qt.Not(qt.IsTrue))  // ❌
c.Assert(value, qt.IsFalse)          // ✓

// BAD: Use qt.HasLen instead of len(x), qt.Equals
c.Assert(len(s), qt.Equals, 3)      // ❌
c.Assert(s, qt.HasLen, 3)            // ✓
```

## Testing Standards

### Declarative Tests Only

All tests MUST be purely declarative. The following are **prohibited** in test functions:
- `if` statements
- `switch` statements
- `goto` statements

`for` loops are allowed for table-driven tests (iterating over a static list of cases). Keep loop bodies simple and avoid using loops to encode branching logic.

**Exception — end-to-end tests**: Tests in `tests/e2e` are exempt from the declarative-only rule. Conditionals (`if`, `switch`) are permitted in e2e tests where necessary to handle environment variance, process orchestration, or multi-step flows that cannot be expressed declaratively.

**Go 1.22+ note**: range variables are per-iteration, so the historical `tt := tt` workaround is not needed when using `c.Run()`/closures.

### Do not hide conditionals in helper functions

Avoid helper functions that mask conditional logic (e.g., choosing between `qt.ErrorIs` and `qt.IsNil` based on test-case fields). Write explicit assertions per case, even if repetitive.

### Separate happy-path and failure-path tests

Do not mix success and error cases in the same table. Prefer:
- `TestXxx_HappyPath` and `TestXxx_FailurePath`, or
- separate `c.Run("happy ...")` and `c.Run("failure ...")` groups with distinct tables.

```go
func TestOpen_HappyPath(t *testing.T) {
    c := qt.New(t)

    cases := []struct {
        name string
        path string
    }{
        {"temp dir", t.TempDir() + "/index.db"},
    }

    for _, tc := range cases {
        c.Run(tc.name, func(c *qt.C) {
            db, err := db.Open(tc.path)
            c.Assert(err, qt.IsNil)
            c.Assert(db, qt.IsNotNil)
            _ = db.Close()
        })
    }
}

func TestOpen_FailurePath(t *testing.T) {
    c := qt.New(t)

    c.Run("invalid path returns error", func(c *qt.C) {
        _, err := db.Open("/no/such/dir/index.db")
        c.Assert(err, qt.IsNotNil)
    })
}
```

### End-to-End Tests

All e2e tests MUST be placed in `tests/e2e`. They exercise the full stack by loading the root Cobra command directly (without spawning a subprocess), driving it through its CLI interface as if it were the compiled binary. As noted above, conditionals are permitted in e2e tests.

### Black-Box Testing (Default)

By default, all tests use black-box testing:
- Test file: `*_test.go`
- Package name: `package db_test` (note the `_test` suffix)
- Only test exported API

```go
// file: db_test.go
package db_test

import (
    "testing"

    qt "github.com/frankban/quicktest"
    "github.com/go-ports/echovault/internal/db"
)

func TestInsertMemory_HappyPath(t *testing.T) {
    c := qt.New(t)
    database, err := db.Open(t.TempDir() + "/index.db")
    c.Assert(err, qt.IsNil)
    defer database.Close()
    // test using only exported types
}
```

### White-Box Testing (Exception)

White-box testing (same package, access to unexported symbols) is permitted ONLY when:
1. Testing unexported functions critical for correctness
2. Testing internal state that cannot be observed through the public API
3. There is a clear technical justification

**Requirements for white-box tests:**
- File naming: `*_internal_test.go`
- Package name: `package db` (no `_test` suffix)
- Include a comment at the top (after the package declaration) explaining the justification

```go
// file: search_internal_test.go
package search

// White-box testing required: normalizeRows and type-coercion helpers
// (asString, asFloat, asBool, clamp) are unexported and drive the correctness
// of merged search results. Their behaviour cannot be observed through the
// public MergeResults API, which returns only the final ranked list.

import (
    "testing"

    qt "github.com/frankban/quicktest"
)

func TestNormalizeRows_HappyPath(t *testing.T) {
    c := qt.New(t)
    // ...
}
```

## Architecture

The binary is `memory` (built from `cmd/memory/`). Core internal packages:

- **`internal/service`** — orchestrates all memory operations: wires config, db, redaction, markdown, embeddings, and search. Entry point for all CLI commands via `service.New(memoryHome)`.
- **`internal/db`** — SQLite layer: schema creation (FTS5 triggers, sqlite-vec table), CRUD, full-text search, vector search. Requires CGO.
- **`internal/search`** — hybrid search combining FTS5 and vector results with weighted scoring and tiered fallback (`TieredSearch`, `HybridSearch`, `MergeResults`).
- **`internal/embeddings`** — embedding providers (`Provider` interface): Ollama, OpenAI, OpenRouter. Returns `[]float32` vectors.
- **`internal/config`** — per-vault YAML config (`config.yaml`) and memory home resolution.
- **`internal/models`** — core data types: `Memory`, `RawMemoryInput`, `SaveResult`, `SearchResult`, etc.
- **`internal/markdown`** — reads and writes memory markdown files in the vault directory.
- **`internal/redaction`** — loads `.memoryignore` regex patterns; redacts secrets from memory content before persisting.
- **`internal/mcp`** — stdio MCP server exposing `memory_save`, `memory_search`, `memory_context` tools.
- **`internal/setup`** — first-run setup: creates vault directory, installs the AGENTS.md / CLAUDE.md section into project files.
- **`internal/checkers`** — custom `qt` checkers (`JSONPathEquals`, `JSONPathMatches`) for JSON assertion in tests.
- **`internal/buildinfo`** — version, build date, git commit injected by Makefile ldflags.

### Memory Home Resolution

Priority: `--memory-home` flag → `MEMORY_HOME` env → persisted global config (`~/.config/echovault/config.yaml`) → `~/.memory`.

Per-vault config: `<memory-home>/config.yaml`. Vault markdown files: `<memory-home>/vault/`. SQLite index: `<memory-home>/index.db`.

### Memory Categories

Valid category values (enforced by the model and MCP layer):
`decision`, `pattern`, `bug`, `context`, `learning`

### Embedding Providers

Configured in `<memory-home>/config.yaml`:

```yaml
embedding:
  provider: ollama       # ollama | openai | openrouter | none
  model: nomic-embed-text
  base_url: http://localhost:11434
  api_key: ""            # required for openai / openrouter
```

`provider: none` (or empty) disables embeddings — search falls back to FTS5 only. Embedding errors during search are non-fatal; FTS results are returned as a fallback.
