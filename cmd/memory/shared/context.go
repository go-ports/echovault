// Package shared holds the context passed to all CLI commands.
package shared

// Context carries global CLI state (flags set on the root command).
type Context struct {
	// MemoryHome overrides the memory home directory.
	// When empty, resolution falls through to MEMORY_HOME env var → persisted config → ~/.memory.
	MemoryHome string
}
