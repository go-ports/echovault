# Description

Provide a clear, concise summary of the change and the problem it solves.

## Type of Change

- [ ] Bug fix
- [ ] New feature
- [ ] Breaking change
- [ ] Performance improvement
- [ ] Documentation update
- [ ] Refactoring / tests only

## Related Issues

Fixes #
Relates to #

## Changes Made

- 
- 
- 

## How to Test

Describe exact steps/commands to validate. Include any env vars or flags needed.

```bash
# Build and run
make build
./bin/memory --memory-home /tmp/test-vault save --title "Test" --what "Smoke test"

# Search
./bin/memory --memory-home /tmp/test-vault search --query "test"

# MCP server (if applicable)
./bin/memory mcp
```

## Test Coverage

- [ ] New code is covered by tests
- [ ] All tests pass: `make test`
- [ ] Lint passes: `make lint`
- [ ] CGO requirements respected (`CGO_ENABLED=1`, `CGO_CFLAGS="-DSQLITE_ENABLE_FTS5"` for direct `go test`)

## Documentation

- [ ] Exported types and functions have Go doc comments
- [ ] `AGENTS.md` updated if new patterns, libraries, or conventions were introduced
- [ ] README updated (if applicable)

## Breaking Changes

Does this introduce breaking changes to the CLI, MCP tool arguments, database schema, or config format?

- [ ] Yes
- [ ] No

If yes, explain the impact and migration path (schema migrations, config changes, etc.).

## Performance / Security Impact

Call out any notable performance or security implications. Attach benchmarks or logs if relevant. Pay particular attention to:
- Embedding provider changes (latency, token usage)
- SQLite query changes (FTS5 / vector search performance)
- Redaction / secret handling

## Checklist

- [ ] Follows project style guidelines (`AGENTS.md`)
- [ ] No new lint warnings introduced
- [ ] Import groups ordered correctly (stdlib → third-party → local)
- [ ] `make()` used for empty slice/map initialization
- [ ] PR description follows this template
