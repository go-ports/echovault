// Package search implements tiered FTS5 + vector hybrid search.
package search

import (
	"context"
	"sort"

	"github.com/go-ports/echovault/internal/db"
	"github.com/go-ports/echovault/internal/embeddings"
)

// Result is a single search hit with a combined relevance score.
type Result struct {
	ID         string
	Score      float64
	Title      string
	What       string
	Why        string
	Impact     string
	Category   string
	Tags       string // raw JSON string from db
	Project    string
	Source     string
	CreatedAt  string
	HasDetails bool
	FilePath   string
}

// MergeResults combines FTS5 and vector search results with weighted scoring.
// ftsWeight defaults to 0.3, vecWeight to 0.7 when called from Tiered/HybridSearch.
func MergeResults(fts, vec []map[string]any, ftsWeight, vecWeight float64, limit int) []Result {
	normalizeRows(fts)
	normalizeRows(vec)

	// Combined map keyed by memory ID.
	combined := make(map[string]*Result, len(fts)+len(vec))

	for _, row := range fts {
		r := rowToResult(row)
		r.Score = ftsWeight * r.Score
		existing := r // copy
		combined[r.ID] = &existing
	}
	for _, row := range vec {
		r := rowToResult(row)
		if existing, ok := combined[r.ID]; ok {
			existing.Score += vecWeight * r.Score
		} else {
			r.Score = vecWeight * r.Score
			cp := r
			combined[r.ID] = &cp
		}
	}

	// Collect and sort descending by score.
	results := make([]Result, 0, len(combined))
	for _, r := range combined {
		results = append(results, *r)
	}
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	if limit > 0 && len(results) > limit {
		return results[:limit]
	}
	return results
}

// TieredSearch runs FTS first and only embeds when results are sparse.
// minFTS is the minimum number of FTS hits before skipping the embed call.
// Pass minFTS=0 to use the default of 3.
func TieredSearch(
	ctx context.Context,
	database *db.DB,
	ep embeddings.Provider,
	query string,
	limit, minFTS int,
	project, source string,
) ([]Result, error) {
	if minFTS <= 0 {
		minFTS = 3
	}

	ftsRows, err := database.FTSSearch(query, limit*2, project, source)
	if err != nil {
		return nil, err
	}

	// Normalize FTS scores in-place.
	normalizeRows(ftsRows)

	// Enough FTS results — return without calling the embedding provider.
	if len(ftsRows) >= minFTS {
		return toResults(ftsRows[:clamp(limit, len(ftsRows))]), nil
	}

	// No embedding provider — FTS-only fallback.
	if ep == nil {
		return toResults(ftsRows[:clamp(limit, len(ftsRows))]), nil
	}

	// Sparse FTS — fall back to hybrid search, embedding errors are non-fatal.
	vec, err := ep.Embed(ctx, query)
	if err != nil {
		return toResults(ftsRows[:clamp(limit, len(ftsRows))]), nil //nolint:nilerr // embedding errors are non-fatal; FTS results are returned as a fallback
	}
	vecRows, err := database.VectorSearch(vec, limit*2, project, source)
	if err != nil {
		return toResults(ftsRows[:clamp(limit, len(ftsRows))]), nil //nolint:nilerr // vector search errors are non-fatal; FTS results are returned as a fallback
	}

	return MergeResults(ftsRows, vecRows, 0.3, 0.7, limit), nil
}

// HybridSearch always runs both FTS and vector search (when ep != nil).
func HybridSearch(
	ctx context.Context,
	database *db.DB,
	ep embeddings.Provider,
	query string,
	limit int,
	project, source string,
) ([]Result, error) {
	ftsRows, err := database.FTSSearch(query, limit*2, project, source)
	if err != nil {
		return nil, err
	}

	// FTS-only mode when no embedding provider.
	if ep == nil {
		normalizeRows(ftsRows)
		return toResults(ftsRows[:clamp(limit, len(ftsRows))]), nil
	}

	vec, err := ep.Embed(ctx, query)
	if err != nil {
		return nil, err
	}
	vecRows, err := database.VectorSearch(vec, limit*2, project, source)
	if err != nil {
		return nil, err
	}

	return MergeResults(ftsRows, vecRows, 0.3, 0.7, limit), nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// normalizeRows divides each row's score by the maximum score, producing [0, 1].
func normalizeRows(rows []map[string]any) {
	if len(rows) == 0 {
		return
	}
	var maxScore float64
	for _, r := range rows {
		if s := asFloat(r["score"]); s > maxScore {
			maxScore = s
		}
	}
	if maxScore <= 0 {
		maxScore = 1.0
	}
	for _, r := range rows {
		r["score"] = asFloat(r["score"]) / maxScore
	}
}

// rowToResult converts a db map row into a Result.
func rowToResult(row map[string]any) Result {
	return Result{
		ID:         asString(row["id"]),
		Score:      asFloat(row["score"]),
		Title:      asString(row["title"]),
		What:       asString(row["what"]),
		Why:        asString(row["why"]),
		Impact:     asString(row["impact"]),
		Category:   asString(row["category"]),
		Tags:       asString(row["tags"]),
		Project:    asString(row["project"]),
		Source:     asString(row["source"]),
		CreatedAt:  asString(row["created_at"]),
		HasDetails: asBool(row["has_details"]),
		FilePath:   asString(row["file_path"]),
	}
}

// toResults converts a slice of db rows to []Result.
func toResults(rows []map[string]any) []Result {
	out := make([]Result, len(rows))
	for i, r := range rows {
		out[i] = rowToResult(r)
	}
	return out
}

func clamp(limit, n int) int {
	if limit <= 0 {
		return n
	}
	if limit < n {
		return limit
	}
	return n
}

func asString(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func asFloat(v any) float64 {
	if v == nil {
		return 0
	}
	switch n := v.(type) {
	case float64:
		return n
	case float32:
		return float64(n)
	case int64:
		return float64(n)
	case int:
		return float64(n)
	}
	return 0
}

func asBool(v any) bool {
	if v == nil {
		return false
	}
	switch b := v.(type) {
	case bool:
		return b
	case int64:
		return b != 0
	case int:
		return b != 0
	}
	return false
}
