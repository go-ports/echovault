// Package sessionscmd implements the `memory sessions` command.
package sessionscmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/go-ports/echovault/cmd/memory/shared"
	"github.com/go-ports/echovault/internal/service"
)

// Command implements `memory sessions`.
type Command struct {
	ctx *shared.Context
	cmd *cobra.Command

	limit   int
	project string
}

// New creates the sessions command.
func New(ctx *shared.Context) *Command {
	c := &Command{ctx: ctx}
	c.cmd = &cobra.Command{
		Use:   "sessions",
		Short: "List recent session markdown files",
		RunE:  c.run,
	}

	f := c.cmd.Flags()
	f.IntVar(&c.limit, "limit", 10, "Maximum number of sessions to show")
	f.StringVar(&c.project, "project", "", "Filter by project name")

	return c
}

// Cmd returns the cobra command.
func (c *Command) Cmd() *cobra.Command { return c.cmd }

func (c *Command) run(cmd *cobra.Command, _ []string) error {
	svc, err := service.New(c.ctx.MemoryHome)
	if err != nil {
		return err
	}
	vaultDir := svc.VaultDir
	svc.Close()

	type sessionEntry struct {
		proj  string
		fname string
	}
	var sessions []sessionEntry

	entries, err := os.ReadDir(vaultDir)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	// Sort project dirs alphabetically.
	projDirs := make([]string, 0)
	for _, e := range entries {
		if e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
			projDirs = append(projDirs, e.Name())
		}
	}
	sort.Strings(projDirs)

	for _, proj := range projDirs {
		if c.project != "" && proj != c.project {
			continue
		}
		projPath := filepath.Join(vaultDir, proj)
		files, err := os.ReadDir(projPath)
		if err != nil {
			continue
		}
		// Sort files reverse-alphabetically (newest first by date prefix).
		sort.Slice(files, func(i, j int) bool {
			return files[i].Name() > files[j].Name()
		})
		for _, f := range files {
			if strings.HasSuffix(f.Name(), "-session.md") {
				sessions = append(sessions, sessionEntry{proj: proj, fname: f.Name()})
			}
		}
	}

	out := cmd.OutOrStdout()
	if len(sessions) == 0 {
		fmt.Fprintln(out, "No sessions found.")
		return nil
	}

	fmt.Fprintln(out, "\nSessions:")
	limit := c.limit
	if limit > len(sessions) {
		limit = len(sessions)
	}
	for _, s := range sessions[:limit] {
		dateStr := strings.TrimSuffix(s.fname, "-session.md")
		fmt.Fprintf(out, "  %s | %s\n", dateStr, s.proj)
	}
	return nil
}
