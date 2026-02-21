// Package buildinfo holds build-time variables injected via ldflags.
package buildinfo

// Populated by -ldflags at build time; defaults used for local dev.
var (
	Version   = "dev"
	BuildDate = "unknown"
	GitCommit = "unknown"
	GitBranch = "unknown"
)
