package markdown_test

import (
	"os"
	"path/filepath"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/go-ports/echovault/internal/markdown"
	"github.com/go-ports/echovault/internal/models"
)

// ---------------------------------------------------------------------------
// RenderSection
// ---------------------------------------------------------------------------

func TestRenderSection_HappyPath(t *testing.T) {
	c := qt.New(t)

	cases := []struct {
		name    string
		mem     *models.Memory
		details string
		want    string
	}{
		{
			name:    "title and what only",
			mem:     &models.Memory{Title: "Foo", What: "bar"},
			details: "",
			want:    "### Foo\n**What:** bar",
		},
		{
			name:    "with why",
			mem:     &models.Memory{Title: "Foo", What: "bar", Why: "baz"},
			details: "",
			want:    "### Foo\n**What:** bar\n**Why:** baz",
		},
		{
			name:    "with why and impact",
			mem:     &models.Memory{Title: "Foo", What: "bar", Why: "baz", Impact: "qux"},
			details: "",
			want:    "### Foo\n**What:** bar\n**Why:** baz\n**Impact:** qux",
		},
		{
			name:    "with source",
			mem:     &models.Memory{Title: "Foo", What: "bar", Source: "claude"},
			details: "",
			want:    "### Foo\n**What:** bar\n**Source:** claude",
		},
		{
			name:    "all fields",
			mem:     &models.Memory{Title: "Foo", What: "bar", Why: "baz", Impact: "qux", Source: "claude"},
			details: "",
			want:    "### Foo\n**What:** bar\n**Why:** baz\n**Impact:** qux\n**Source:** claude",
		},
		{
			name:    "with details block",
			mem:     &models.Memory{Title: "Foo", What: "bar"},
			details: "d",
			want:    "### Foo\n**What:** bar\n\n<details>\nd\n</details>",
		},
		{
			name:    "with all fields and details",
			mem:     &models.Memory{Title: "Foo", What: "bar", Why: "baz", Impact: "qux", Source: "claude"},
			details: "extra context",
			want:    "### Foo\n**What:** bar\n**Why:** baz\n**Impact:** qux\n**Source:** claude\n\n<details>\nextra context\n</details>",
		},
	}

	for _, tc := range cases {
		c.Run(tc.name, func(c *qt.C) {
			got := markdown.RenderSection(tc.mem, tc.details)
			c.Assert(got, qt.Equals, tc.want)
		})
	}
}

// ---------------------------------------------------------------------------
// WriteSessionMemory
// ---------------------------------------------------------------------------

func TestWriteSessionMemory_HappyPath(t *testing.T) {
	c := qt.New(t)

	c.Run("creates new session file with correct content", func(c *qt.C) {
		dir := t.TempDir()
		mem := &models.Memory{
			Title:   "Test Memory",
			What:    "something important happened",
			Project: "myproject",
		}
		err := markdown.WriteSessionMemory(dir, mem, "2024-01-15", "")
		c.Assert(err, qt.IsNil)

		data, err := os.ReadFile(filepath.Join(dir, "2024-01-15-session.md"))
		c.Assert(err, qt.IsNil)
		content := string(data)
		c.Assert(content, qt.Contains, "# 2024-01-15 Session")
		c.Assert(content, qt.Contains, "project: myproject")
		c.Assert(content, qt.Contains, "### Test Memory")
		c.Assert(content, qt.Contains, "**What:** something important happened")
	})

	c.Run("appends second memory to existing session file", func(c *qt.C) {
		dir := t.TempDir()
		mem1 := &models.Memory{
			Title:   "First Memory",
			What:    "first thing",
			Project: "myproject",
		}
		mem2 := &models.Memory{
			Title:   "Second Memory",
			What:    "second thing",
			Project: "myproject",
		}
		err := markdown.WriteSessionMemory(dir, mem1, "2024-01-15", "")
		c.Assert(err, qt.IsNil)
		err = markdown.WriteSessionMemory(dir, mem2, "2024-01-15", "")
		c.Assert(err, qt.IsNil)

		data, err := os.ReadFile(filepath.Join(dir, "2024-01-15-session.md"))
		c.Assert(err, qt.IsNil)
		content := string(data)
		c.Assert(content, qt.Contains, "### First Memory")
		c.Assert(content, qt.Contains, "### Second Memory")
	})

	c.Run("includes details block when provided", func(c *qt.C) {
		dir := t.TempDir()
		mem := &models.Memory{
			Title:   "Detailed Memory",
			What:    "something with details",
			Project: "proj",
		}
		err := markdown.WriteSessionMemory(dir, mem, "2024-01-15", "Extra context here.")
		c.Assert(err, qt.IsNil)

		data, err := os.ReadFile(filepath.Join(dir, "2024-01-15-session.md"))
		c.Assert(err, qt.IsNil)
		content := string(data)
		c.Assert(content, qt.Contains, "<details>")
		c.Assert(content, qt.Contains, "Extra context here.")
		c.Assert(content, qt.Contains, "</details>")
	})

	c.Run("groups memory under category heading", func(c *qt.C) {
		dir := t.TempDir()
		mem := &models.Memory{
			Title:    "A Decision",
			What:     "we decided something",
			Project:  "proj",
			Category: "decision",
		}
		err := markdown.WriteSessionMemory(dir, mem, "2024-01-15", "")
		c.Assert(err, qt.IsNil)

		data, err := os.ReadFile(filepath.Join(dir, "2024-01-15-session.md"))
		c.Assert(err, qt.IsNil)
		content := string(data)
		c.Assert(content, qt.Contains, "## Decisions")
		c.Assert(content, qt.Contains, "### A Decision")
	})

	c.Run("frontmatter accumulates tags on append", func(c *qt.C) {
		dir := t.TempDir()
		mem1 := &models.Memory{
			Title:   "First",
			What:    "first",
			Project: "proj",
			Tags:    []string{"alpha", "beta"},
		}
		mem2 := &models.Memory{
			Title:   "Second",
			What:    "second",
			Project: "proj",
			Tags:    []string{"gamma"},
		}
		err := markdown.WriteSessionMemory(dir, mem1, "2024-01-15", "")
		c.Assert(err, qt.IsNil)
		err = markdown.WriteSessionMemory(dir, mem2, "2024-01-15", "")
		c.Assert(err, qt.IsNil)

		data, err := os.ReadFile(filepath.Join(dir, "2024-01-15-session.md"))
		c.Assert(err, qt.IsNil)
		content := string(data)
		c.Assert(content, qt.Contains, "alpha")
		c.Assert(content, qt.Contains, "beta")
		c.Assert(content, qt.Contains, "gamma")
	})
}
