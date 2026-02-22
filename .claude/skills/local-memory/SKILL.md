---
name: echovault
description: Local-first memory for coding agents. You MUST retrieve memories at session start and save memories before session end. This is not optional.
---

# EchoVault — Agent Memory System

You have persistent memory across sessions. USE IT.

IMPORTANT: Prefer the EchoVault MCP tools (memory_context / memory_search / memory_save / memory_replace / memory_delete) over invoking the `memory` CLI.

## Session start — MANDATORY
Before doing ANY work, retrieve context from previous sessions by calling the MCP tool `memory_context`.

`project` is required.

```json path=null start=null
{
  "project": "<project-name>",
  "limit": 10
}
```

If the user's request relates to a specific topic, also call `memory_search`.

```json path=null start=null
{
  "query": "<relevant terms>",
  "project": "<project-name>",
  "limit": 5
}
```

If you discover an existing memory is wrong or outdated, prefer `memory_replace` (overwrite) over `memory_save` (dedup/merge).

Do not skip this step. Prior sessions may contain decisions, bugs, and context that directly affect your current task.

## Session end — MANDATORY
Before ending your response to ANY task that involved making changes, debugging, deciding, or learning something, you MUST call the MCP tool `memory_save`. This is not optional. If you did meaningful work, save it.

`title`, `what`, and `project` are required.

```json path=null start=null
{
  "project": "<project-name>",
  "title": "Short descriptive title (max 60 chars)",
  "what": "1-2 sentences: what happened / what was decided",
  "why": "Reasoning behind it (optional)",
  "impact": "What changed as a result (optional)",
  "tags": ["tag1", "tag2"],
  "category": "<category>",
  "related_files": ["path/to/file1", "path/to/file2"],
  "details": "Full context for a future agent with zero context. Prefer: Context, Options considered, Decision, Tradeoffs, Follow-up."
}
```

Categories: `decision`, `bug`, `pattern`, `learning`, `context`.

### What to save

You MUST save when any of these happen:

- You made an architectural or design decision
- You fixed a bug (include root cause and solution)
- You discovered a non-obvious pattern or gotcha
- You set up infrastructure, tooling, or configuration
- You chose one approach over alternatives
- You learned something about the codebase that isn't in the code
- The user corrected you or clarified a requirement

### What NOT to save

- Trivial changes (typo fixes, formatting)
- Information that's already obvious from reading the code
- Duplicate of an existing memory (search first)

## Other MCP tools (optional)

Correct or remove stale memories as part of keeping the store accurate:

### Replace (overwrite) an existing memory

```json path=null start=null
{
  "id": "<memory-id-or-prefix>",
  "project": "<project-name>",
  "title": "Updated title (max 60 chars)",
  "what": "Updated 1-2 sentence summary",
  "why": "(optional)",
  "impact": "(optional)",
  "tags": ["tag1", "tag2"],
  "category": "<category>",
  "related_files": ["path/to/file1"],
  "details": "Updated full context"
}
```

### Delete memories

```json path=null start=null
{
  "ids": ["<memory-id-or-prefix>"]
}
```

## Agent setup (CLI-only, optional)

Only run these if the user explicitly asks (these are not MCP tools):

```bash
memory setup <agent>      # e.g. claude-code, cursor, codex
memory uninstall <agent>
```

## Maintenance (CLI-only, optional)

Only run these if the user explicitly asks (these are not MCP tools):

```bash
memory config    # show current configuration
memory sessions  # list session files
memory reindex   # rebuild search index
```

## Rules

- Retrieve before working. Save before finishing. No exceptions.
- Always capture thorough details — write for a future agent with no context.
- Never include API keys, secrets, or credentials.
- Wrap sensitive values in `<redacted>` tags.
- Search before saving to avoid duplicates.
- One memory per distinct decision or event. Don't bundle unrelated things.
