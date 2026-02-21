// Package savecmd implements the `memory save` command.
package savecmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/go-ports/echovault/cmd/memory/shared"
	"github.com/go-ports/echovault/internal/models"
	"github.com/go-ports/echovault/internal/service"
)

const detailsTemplate = `Context:

Options considered:
- Option A:
- Option B:

Decision:

Tradeoffs:

Follow-up:
`

// Command implements `memory save`.
type Command struct {
	ctx *shared.Context
	cmd *cobra.Command

	title           string
	what            string
	why             string
	impact          string
	tags            string
	category        string
	relatedFiles    string
	details         string
	detailsFile     string
	detailsTemplate bool
	source          string
	project         string
}

// New creates the save command.
func New(ctx *shared.Context) *Command {
	c := &Command{ctx: ctx}
	c.cmd = &cobra.Command{
		Use:   "save",
		Short: "Save a memory to the current session",
		RunE:  c.run,
	}

	f := c.cmd.Flags()
	f.StringVar(&c.title, "title", "", "Title of the memory (required)")
	f.StringVar(&c.what, "what", "", "What happened or was learned (required)")
	f.StringVar(&c.why, "why", "", "Why it matters")
	f.StringVar(&c.impact, "impact", "", "Impact or consequences")
	f.StringVar(&c.tags, "tags", "", "Comma-separated tags")
	f.StringVar(&c.category, "category", "", "Category: decision, pattern, bug, context, learning")
	f.StringVar(&c.relatedFiles, "related-files", "", "Comma-separated file paths")
	f.StringVar(&c.details, "details", "", "Extended details or context")
	f.StringVar(&c.detailsFile, "details-file", "", "Path to a file containing extended details")
	f.BoolVar(&c.detailsTemplate, "details-template", false, "Use a structured details template")
	f.StringVar(&c.source, "source", "", "Source of the memory (e.g. claude-code)")
	f.StringVar(&c.project, "project", "", "Project name (required)")

	_ = c.cmd.MarkFlagRequired("title")
	_ = c.cmd.MarkFlagRequired("what")
	_ = c.cmd.MarkFlagRequired("project")

	return c
}

// Cmd returns the cobra command.
func (c *Command) Cmd() *cobra.Command { return c.cmd }

func (c *Command) run(cmd *cobra.Command, _ []string) error {
	if c.details != "" && c.detailsFile != "" {
		return fmt.Errorf("use either --details or --details-file, not both")
	}

	resolvedDetails := c.details
	if c.detailsFile != "" {
		data, err := os.ReadFile(c.detailsFile)
		if err != nil {
			return fmt.Errorf("failed to read details file %q: %w", c.detailsFile, err)
		}
		resolvedDetails = string(data)
	}
	if c.detailsTemplate && strings.TrimSpace(resolvedDetails) == "" {
		resolvedDetails = detailsTemplate
	}

	tagList := splitCSV(c.tags)
	fileList := splitCSV(c.relatedFiles)

	svc, err := service.New(c.ctx.MemoryHome)
	if err != nil {
		return err
	}
	defer svc.Close()

	raw := &models.RawMemoryInput{
		Title:        c.title,
		What:         c.what,
		Why:          c.why,
		Impact:       c.impact,
		Tags:         tagList,
		Category:     c.category,
		RelatedFiles: fileList,
		Details:      resolvedDetails,
		Source:       c.source,
	}

	result, err := svc.Save(cmd.Context(), raw, c.project)
	if err != nil {
		return err
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Saved: %s (id: %s)\n", c.title, result.ID)
	fmt.Fprintf(cmd.OutOrStdout(), "File: %s\n", result.FilePath)
	for _, w := range result.Warnings {
		fmt.Fprintf(cmd.OutOrStdout(), "Warning: %s\n", w)
	}
	return nil
}

func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
