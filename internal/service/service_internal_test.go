package service

import (
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/go-ports/echovault/internal/models"
	"github.com/go-ports/echovault/internal/search"
)

// ---------------------------------------------------------------------------
// mergeTags
// ---------------------------------------------------------------------------

func TestMergeTags_HappyPath(t *testing.T) {
	c := qt.New(t)

	cases := []struct {
		name     string
		existing []string
		extra    []string
		wantLen  int
		wantHas  []string
	}{
		{
			name:     "disjoint slices are combined",
			existing: []string{"alpha", "beta"},
			extra:    []string{"gamma"},
			wantLen:  3,
			wantHas:  []string{"alpha", "beta", "gamma"},
		},
		{
			name:     "case-insensitive dedup prevents double add",
			existing: []string{"Foo"},
			extra:    []string{"foo", "FOO"},
			wantLen:  1,
			wantHas:  []string{"Foo"},
		},
		{
			name:     "empty existing returns all extra",
			existing: make([]string, 0),
			extra:    []string{"x", "y"},
			wantLen:  2,
			wantHas:  []string{"x", "y"},
		},
		{
			name:     "nil extra leaves existing unchanged",
			existing: []string{"a", "b"},
			extra:    nil,
			wantLen:  2,
			wantHas:  []string{"a", "b"},
		},
	}

	for _, tc := range cases {
		c.Run(tc.name, func(c *qt.C) {
			got := mergeTags(tc.existing, tc.extra)
			c.Assert(got, qt.HasLen, tc.wantLen)
			for _, tag := range tc.wantHas {
				c.Assert(got, qt.Contains, tag)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// detailsWarnings
// ---------------------------------------------------------------------------

func TestDetailsWarnings_HappyPath(t *testing.T) {
	c := qt.New(t)

	allSections := "context options considered decision tradeoffs follow-up " +
		"more text here to reach the minimum character count threshold required by the validation logic"

	c.Run("decision with no details produces one warning", func(c *qt.C) {
		raw := &models.RawMemoryInput{Category: "decision", Details: ""}
		warnings := detailsWarnings(raw)
		c.Assert(warnings, qt.HasLen, 1)
		c.Assert(warnings[0], qt.Contains, "decision")
	})

	c.Run("bug with no details produces one warning", func(c *qt.C) {
		raw := &models.RawMemoryInput{Category: "bug", Details: ""}
		warnings := detailsWarnings(raw)
		c.Assert(warnings, qt.HasLen, 1)
	})

	c.Run("other category with no details produces no warning", func(c *qt.C) {
		raw := &models.RawMemoryInput{Category: "pattern", Details: ""}
		warnings := detailsWarnings(raw)
		c.Assert(warnings, qt.HasLen, 0)
	})

	c.Run("empty details with no category produces no warning", func(c *qt.C) {
		raw := &models.RawMemoryInput{Details: ""}
		warnings := detailsWarnings(raw)
		c.Assert(warnings, qt.HasLen, 0)
	})

	c.Run("short details produces brevity warning", func(c *qt.C) {
		raw := &models.RawMemoryInput{Details: "brief"}
		warnings := detailsWarnings(raw)
		c.Assert(len(warnings) >= 1, qt.IsTrue)
		c.Assert(warnings[0], qt.Contains, "chars")
	})

	c.Run("long details with all required sections produces no warnings", func(c *qt.C) {
		raw := &models.RawMemoryInput{Details: allSections}
		warnings := detailsWarnings(raw)
		c.Assert(warnings, qt.HasLen, 0)
	})

	c.Run("long details missing sections produces a warning", func(c *qt.C) {
		long := "this is a very long detail text that exceeds the minimum char count but does not include the required structural headings at all"
		raw := &models.RawMemoryInput{Details: long}
		warnings := detailsWarnings(raw)
		c.Assert(len(warnings) >= 1, qt.IsTrue)
		c.Assert(warnings[len(warnings)-1], qt.Contains, "missing")
	})
}

// ---------------------------------------------------------------------------
// resultsToMaps
// ---------------------------------------------------------------------------

func TestResultsToMaps_HappyPath(t *testing.T) {
	c := qt.New(t)

	c.Run("empty slice returns empty slice", func(c *qt.C) {
		got := resultsToMaps(make([]search.Result, 0))
		c.Assert(got, qt.HasLen, 0)
	})

	c.Run("converts all fields correctly", func(c *qt.C) {
		results := []search.Result{{
			ID:         "r1",
			Title:      "My Title",
			Category:   "decision",
			Tags:       `["go"]`,
			Project:    "myproject",
			Source:     "claude",
			CreatedAt:  "2024-01-15T00:00:00Z",
			HasDetails: true,
			Score:      0.85,
		}}
		got := resultsToMaps(results)
		c.Assert(got, qt.HasLen, 1)
		m := got[0]
		c.Assert(m["id"], qt.Equals, "r1")
		c.Assert(m["title"], qt.Equals, "My Title")
		c.Assert(m["category"], qt.Equals, "decision")
		c.Assert(m["project"], qt.Equals, "myproject")
		c.Assert(m["source"], qt.Equals, "claude")
		c.Assert(m["has_details"], qt.Equals, true)
		c.Assert(m["score"], qt.Equals, 0.85)
	})
}
