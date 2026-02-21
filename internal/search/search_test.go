package search_test

import (
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/go-ports/echovault/internal/search"
)

// row is a convenience helper that builds a minimal db-row map for MergeResults.
func row(id string, score float64) map[string]any {
	return map[string]any{
		"id": id, "score": score,
		"title": id, "what": "", "why": "", "impact": "",
		"category": "", "tags": "", "project": "", "source": "",
		"created_at": "", "has_details": false, "file_path": "",
	}
}

func TestMergeResults_HappyPath(t *testing.T) {
	c := qt.New(t)

	c.Run("empty inputs return empty result", func(c *qt.C) {
		got := search.MergeResults(nil, nil, 0.3, 0.7, 10)
		c.Assert(got, qt.HasLen, 0)
	})

	c.Run("FTS-only results are weighted by ftsWeight", func(c *qt.C) {
		fts := []map[string]any{row("a", 1.0)}
		got := search.MergeResults(fts, nil, 0.5, 0.5, 10)
		c.Assert(got, qt.HasLen, 1)
		c.Assert(got[0].ID, qt.Equals, "a")
		// normalised score = 1.0 (single row); weighted = 1.0 * ftsWeight
		c.Assert(got[0].Score, qt.Equals, 0.5)
	})

	c.Run("vec-only results are weighted by vecWeight", func(c *qt.C) {
		vec := []map[string]any{row("b", 1.0)}
		got := search.MergeResults(nil, vec, 0.3, 0.7, 10)
		c.Assert(got, qt.HasLen, 1)
		c.Assert(got[0].ID, qt.Equals, "b")
		c.Assert(got[0].Score, qt.Equals, 0.7)
	})

	c.Run("overlapping IDs accumulate FTS and vec scores", func(c *qt.C) {
		fts := []map[string]any{row("shared", 1.0)}
		vec := []map[string]any{row("shared", 1.0)}
		got := search.MergeResults(fts, vec, 0.3, 0.7, 10)
		c.Assert(got, qt.HasLen, 1)
		// ftsWeight*1 + vecWeight*1 = 1.0
		c.Assert(got[0].Score, qt.Equals, 1.0)
	})

	c.Run("results are sorted descending by score", func(c *qt.C) {
		fts := []map[string]any{row("lo", 1.0), row("hi", 2.0)}
		got := search.MergeResults(fts, nil, 1.0, 0.0, 10)
		c.Assert(got, qt.HasLen, 2)
		c.Assert(got[0].ID, qt.Equals, "hi")
		c.Assert(got[1].ID, qt.Equals, "lo")
	})

	c.Run("positive limit truncates result set", func(c *qt.C) {
		fts := []map[string]any{row("a", 1.0), row("b", 2.0), row("c", 3.0)}
		got := search.MergeResults(fts, nil, 1.0, 0.0, 2)
		c.Assert(got, qt.HasLen, 2)
	})

	c.Run("zero limit returns all results", func(c *qt.C) {
		fts := []map[string]any{row("a", 1.0), row("b", 2.0)}
		got := search.MergeResults(fts, nil, 1.0, 0.0, 0)
		c.Assert(got, qt.HasLen, 2)
	})

	c.Run("non-overlapping FTS and vec are both included", func(c *qt.C) {
		fts := []map[string]any{row("fts-only", 1.0)}
		vec := []map[string]any{row("vec-only", 1.0)}
		got := search.MergeResults(fts, vec, 0.3, 0.7, 10)
		c.Assert(got, qt.HasLen, 2)
	})

	c.Run("result fields are populated from the row map", func(c *qt.C) {
		fts := []map[string]any{{
			"id": "r1", "score": float64(1.0),
			"title": "My Title", "what": "what text", "why": "why text",
			"impact": "impact text", "category": "decision",
			"tags": `["go"]`, "project": "proj", "source": "claude",
			"created_at": "2024-01-15T00:00:00Z", "has_details": true,
			"file_path": "/vault/proj/2024-01-15-session.md",
		}}
		got := search.MergeResults(fts, nil, 1.0, 0.0, 10)
		c.Assert(got, qt.HasLen, 1)
		r := got[0]
		c.Assert(r.ID, qt.Equals, "r1")
		c.Assert(r.Title, qt.Equals, "My Title")
		c.Assert(r.What, qt.Equals, "what text")
		c.Assert(r.Why, qt.Equals, "why text")
		c.Assert(r.Impact, qt.Equals, "impact text")
		c.Assert(r.Category, qt.Equals, "decision")
		c.Assert(r.Project, qt.Equals, "proj")
		c.Assert(r.Source, qt.Equals, "claude")
		c.Assert(r.HasDetails, qt.IsTrue)
	})
}
