// Package e2e_test contains end-to-end tests that exercise the full memory CLI
// by importing the root command and running it in-process with a temporary vault.
// Output is captured via cobra's SetOut so tests can run concurrently without
// affecting os.Stdout.
package e2e_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	qt "github.com/frankban/quicktest"

	rootcmd "github.com/go-ports/echovault/cmd/memory/root"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// runCmd executes the root command with the provided args and returns the
// captured stdout output along with any execution error.
// Output is captured via root.SetOut so tests can run concurrently without
// interfering with each other or with os.Stdout.
func runCmd(t *testing.T, args ...string) (string, error) {
	t.Helper()

	var buf bytes.Buffer
	root := rootcmd.New()
	root.SetOut(&buf)
	root.SetArgs(args)
	execErr := root.ExecuteContext(context.Background())

	return buf.String(), execErr
}

// extractID parses the memory UUID from a save command output line of the form
// "Saved: <title> (id: <uuid>)".
func extractID(output string) string {
	for _, line := range strings.Split(output, "\n") {
		start := strings.Index(line, "(id: ")
		end := strings.LastIndex(line, ")")
		if start >= 0 && end > start+5 {
			return line[start+5 : end]
		}
	}
	return ""
}

// ---------------------------------------------------------------------------
// Help
// ---------------------------------------------------------------------------

func TestHelp_HappyPath(t *testing.T) {
	c := qt.New(t)

	out, err := runCmd(t, "--help")
	c.Assert(err, qt.IsNil)
	c.Assert(out, qt.Contains, "EchoVault")
	c.Assert(out, qt.Contains, "memory")
}

// ---------------------------------------------------------------------------
// Init
// ---------------------------------------------------------------------------

func TestInit_HappyPath(t *testing.T) {
	c := qt.New(t)

	home := t.TempDir()
	out, err := runCmd(t, "--memory-home", home, "init")
	c.Assert(err, qt.IsNil)
	c.Assert(out, qt.Contains, "Memory vault initialized")
	c.Assert(out, qt.Contains, home)
}

// ---------------------------------------------------------------------------
// Save
// ---------------------------------------------------------------------------

func TestSave_HappyPath(t *testing.T) {
	c := qt.New(t)

	home := t.TempDir()
	out, err := runCmd(t, "--memory-home", home, "save",
		"--title", "Use make for builds",
		"--what", "All builds must go through make targets not go build directly",
		"--category", "pattern",
	)
	c.Assert(err, qt.IsNil)
	c.Assert(out, qt.Contains, "Saved: Use make for builds")
	c.Assert(out, qt.Contains, "(id:")
	c.Assert(out, qt.Contains, "File:")
}

func TestSave_FailurePath(t *testing.T) {
	c := qt.New(t)

	home := t.TempDir()

	c.Run("missing required --title flag returns error", func(c *qt.C) {
		_, err := runCmd(t, "--memory-home", home, "save",
			"--what", "something happened",
		)
		c.Assert(err, qt.IsNotNil)
	})

	c.Run("missing required --what flag returns error", func(c *qt.C) {
		_, err := runCmd(t, "--memory-home", home, "save",
			"--title", "some title",
		)
		c.Assert(err, qt.IsNotNil)
	})
}

// ---------------------------------------------------------------------------
// Search
// ---------------------------------------------------------------------------

func TestSearch_HappyPath(t *testing.T) {
	c := qt.New(t)

	home := t.TempDir()
	_, saveErr := runCmd(t, "--memory-home", home, "save",
		"--title", "CGO required for sqlite",
		"--what", "CGO must be enabled for go-sqlite3 and sqlite-vec extensions",
		"--category", "pattern",
	)
	c.Assert(saveErr, qt.IsNil)

	out, err := runCmd(t, "--memory-home", home, "search", "sqlite")
	c.Assert(err, qt.IsNil)
	c.Assert(out, qt.Contains, "Results")
	c.Assert(out, qt.Contains, "CGO required for sqlite")
}

func TestSearch_EmptyVault_HappyPath(t *testing.T) {
	c := qt.New(t)

	home := t.TempDir()
	out, err := runCmd(t, "--memory-home", home, "search", "anything")
	c.Assert(err, qt.IsNil)
	c.Assert(out, qt.Contains, "No results found")
}

func TestSearch_FailurePath(t *testing.T) {
	c := qt.New(t)

	home := t.TempDir()

	c.Run("missing query argument returns error", func(c *qt.C) {
		_, err := runCmd(t, "--memory-home", home, "search")
		c.Assert(err, qt.IsNotNil)
	})
}

// ---------------------------------------------------------------------------
// Delete
// ---------------------------------------------------------------------------

func TestDelete_HappyPath(t *testing.T) {
	c := qt.New(t)

	home := t.TempDir()
	saveOut, saveErr := runCmd(t, "--memory-home", home, "save",
		"--title", "Temporary architectural decision",
		"--what", "Made a temporary architectural decision to revisit later",
	)
	c.Assert(saveErr, qt.IsNil)

	id := extractID(saveOut)
	c.Assert(id, qt.Not(qt.Equals), "")

	out, err := runCmd(t, "--memory-home", home, "delete", id)
	c.Assert(err, qt.IsNil)
	c.Assert(out, qt.Contains, "Deleted memory")
}

func TestDelete_NotFound_HappyPath(t *testing.T) {
	c := qt.New(t)

	home := t.TempDir()
	out, err := runCmd(t, "--memory-home", home, "delete", "nonexistent-id-prefix")
	c.Assert(err, qt.IsNil)
	c.Assert(out, qt.Contains, "No memory found")
}

func TestDelete_FailurePath(t *testing.T) {
	c := qt.New(t)

	home := t.TempDir()

	c.Run("missing ID argument returns error", func(c *qt.C) {
		_, err := runCmd(t, "--memory-home", home, "delete")
		c.Assert(err, qt.IsNotNil)
	})
}

// ---------------------------------------------------------------------------
// Details
// ---------------------------------------------------------------------------

func TestDetails_HappyPath(t *testing.T) {
	c := qt.New(t)

	home := t.TempDir()
	saveOut, saveErr := runCmd(t, "--memory-home", home, "save",
		"--title", "Architecture decision",
		"--what", "Chose SQLite for local persistent storage",
		"--details", "context: needed an embedded database\noptions considered: SQLite, BoltDB\ndecision: chose SQLite\ntradeoffs: requires CGO compilation\nfollow-up: revisit if CGO becomes a blocker",
	)
	c.Assert(saveErr, qt.IsNil)

	id := extractID(saveOut)
	c.Assert(id, qt.Not(qt.Equals), "")

	out, err := runCmd(t, "--memory-home", home, "details", id)
	c.Assert(err, qt.IsNil)
	c.Assert(out, qt.Contains, "context: needed an embedded database")
}

func TestDetails_NoDetails_HappyPath(t *testing.T) {
	c := qt.New(t)

	home := t.TempDir()
	saveOut, saveErr := runCmd(t, "--memory-home", home, "save",
		"--title", "Quick note",
		"--what", "A quick note without any extended details attached",
	)
	c.Assert(saveErr, qt.IsNil)

	id := extractID(saveOut)
	c.Assert(id, qt.Not(qt.Equals), "")

	out, err := runCmd(t, "--memory-home", home, "details", id)
	c.Assert(err, qt.IsNil)
	c.Assert(out, qt.Contains, "No details found")
}

// ---------------------------------------------------------------------------
// Context
// ---------------------------------------------------------------------------

func TestContext_HappyPath(t *testing.T) {
	c := qt.New(t)

	home := t.TempDir()
	_, saveErr := runCmd(t, "--memory-home", home, "save",
		"--title", "Context injection pattern",
		"--what", "Run memory context at the start of every coding agent session",
		"--category", "pattern",
	)
	c.Assert(saveErr, qt.IsNil)

	out, err := runCmd(t, "--memory-home", home, "context")
	c.Assert(err, qt.IsNil)
	c.Assert(out, qt.Contains, "Available memories")
	c.Assert(out, qt.Contains, "Context injection pattern")
}

func TestContext_EmptyVault_HappyPath(t *testing.T) {
	c := qt.New(t)

	home := t.TempDir()
	out, err := runCmd(t, "--memory-home", home, "context")
	c.Assert(err, qt.IsNil)
	c.Assert(out, qt.Contains, "No memories found")
}
