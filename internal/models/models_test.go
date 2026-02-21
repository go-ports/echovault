package models_test

import (
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/go-ports/echovault/internal/models"
)

func TestFromRaw_HappyPath(t *testing.T) {
	c := qt.New(t)

	tests := []struct {
		name         string
		raw          *models.RawMemoryInput
		project      string
		filePath     string
		wantTitle    string
		wantWhat     string
		wantWhy      string
		wantImpact   string
		wantCategory string
		wantProject  string
		wantSource   string
		wantFilePath string
		wantAnchor   string
		wantTagsLen  int
	}{
		{
			name: "all fields set",
			raw: &models.RawMemoryInput{
				Title:    "Test Memory Title",
				What:     "Something happened",
				Why:      "Because of reason",
				Impact:   "Changed behavior",
				Tags:     []string{"go", "testing"},
				Category: "decision",
				Source:   "claude-code",
			},
			project:      "myproject",
			filePath:     "/vault/2024-01-15-session.md",
			wantTitle:    "Test Memory Title",
			wantWhat:     "Something happened",
			wantWhy:      "Because of reason",
			wantImpact:   "Changed behavior",
			wantCategory: "decision",
			wantProject:  "myproject",
			wantSource:   "claude-code",
			wantFilePath: "/vault/2024-01-15-session.md",
			wantAnchor:   "test-memory-title",
			wantTagsLen:  2,
		},
		{
			name: "minimal fields only",
			raw: &models.RawMemoryInput{
				Title: "Minimal",
				What:  "Just what",
			},
			project:      "",
			filePath:     "/vault/session.md",
			wantTitle:    "Minimal",
			wantWhat:     "Just what",
			wantWhy:      "",
			wantImpact:   "",
			wantCategory: "",
			wantProject:  "",
			wantSource:   "",
			wantFilePath: "/vault/session.md",
			wantAnchor:   "minimal",
			wantTagsLen:  0,
		},
		{
			name: "title with special chars produces clean anchor",
			raw: &models.RawMemoryInput{
				Title: "Fix (the) Bug #42!",
				What:  "Resolved",
			},
			project:      "proj",
			filePath:     "/v/f.md",
			wantTitle:    "Fix (the) Bug #42!",
			wantWhat:     "Resolved",
			wantAnchor:   "fix-the-bug-42",
			wantTagsLen:  0,
			wantProject:  "proj",
			wantFilePath: "/v/f.md",
		},
	}

	for _, tt := range tests {
		c.Run(tt.name, func(c *qt.C) {
			mem := models.FromRaw(tt.raw, tt.project, tt.filePath)
			c.Assert(mem, qt.IsNotNil)
			// ID is a UUID v4; verify format only.
			c.Assert(mem.ID, qt.Matches, `[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}`)
			c.Assert(mem.Title, qt.Equals, tt.wantTitle)
			c.Assert(mem.What, qt.Equals, tt.wantWhat)
			c.Assert(mem.Why, qt.Equals, tt.wantWhy)
			c.Assert(mem.Impact, qt.Equals, tt.wantImpact)
			c.Assert(mem.Category, qt.Equals, tt.wantCategory)
			c.Assert(mem.Project, qt.Equals, tt.wantProject)
			c.Assert(mem.Source, qt.Equals, tt.wantSource)
			c.Assert(mem.FilePath, qt.Equals, tt.wantFilePath)
			c.Assert(mem.SectionAnchor, qt.Equals, tt.wantAnchor)
			c.Assert(mem.Tags, qt.HasLen, tt.wantTagsLen)
			c.Assert(mem.CreatedAt.IsZero(), qt.IsFalse)
			c.Assert(mem.UpdatedAt.IsZero(), qt.IsFalse)
		})
	}
}

func TestFromRaw_IDsAreUnique(t *testing.T) {
	c := qt.New(t)

	raw := &models.RawMemoryInput{Title: "T", What: "W"}
	a := models.FromRaw(raw, "", "")
	b := models.FromRaw(raw, "", "")
	c.Assert(a.ID, qt.Not(qt.Equals), b.ID)
}

func TestCategoryHeadings(t *testing.T) {
	c := qt.New(t)

	c.Assert(models.CategoryHeadings["decision"], qt.Equals, "Decisions")
	c.Assert(models.CategoryHeadings["pattern"], qt.Equals, "Patterns")
	c.Assert(models.CategoryHeadings["bug"], qt.Equals, "Bugs Fixed")
	c.Assert(models.CategoryHeadings["context"], qt.Equals, "Context")
	c.Assert(models.CategoryHeadings["learning"], qt.Equals, "Learnings")
}

func TestValidCategories(t *testing.T) {
	c := qt.New(t)

	c.Assert(models.ValidCategories, qt.HasLen, 5)
	c.Assert(models.ValidCategories, qt.Contains, "decision")
	c.Assert(models.ValidCategories, qt.Contains, "pattern")
	c.Assert(models.ValidCategories, qt.Contains, "bug")
	c.Assert(models.ValidCategories, qt.Contains, "context")
	c.Assert(models.ValidCategories, qt.Contains, "learning")
}
