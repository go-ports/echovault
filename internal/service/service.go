// Package service implements the MemoryService orchestrator that wires together
// configuration, database, redaction, markdown, embeddings, and search.
package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/go-ports/echovault/internal/config"
	"github.com/go-ports/echovault/internal/db"
	"github.com/go-ports/echovault/internal/embeddings"
	"github.com/go-ports/echovault/internal/markdown"
	"github.com/go-ports/echovault/internal/models"
	"github.com/go-ports/echovault/internal/redaction"
	"github.com/go-ports/echovault/internal/search"
)

// Service orchestrates all memory operations.
type Service struct {
	MemoryHome string
	VaultDir   string
	Config     *config.MemoryConfig

	database       *db.DB
	embProvider    embeddings.Provider
	ignorePatterns []*regexp.Regexp
	vectorsOK      *bool
	mu             sync.Mutex
}

// New initialises a Service rooted at memoryHome.
// If memoryHome is empty it is resolved via config.GetMemoryHome.
func New(memoryHome string) (*Service, error) {
	if memoryHome == "" {
		memoryHome = config.GetMemoryHome()
	}

	vaultDir := filepath.Join(memoryHome, "vault")
	if err := os.MkdirAll(vaultDir, 0o755); err != nil {
		return nil, fmt.Errorf("service.New: create vault dir: %w", err)
	}

	cfg, err := config.Load(filepath.Join(memoryHome, "config.yaml"))
	if err != nil {
		return nil, fmt.Errorf("service.New: load config: %w", err)
	}

	database, err := db.Open(filepath.Join(memoryHome, "index.db"))
	if err != nil {
		return nil, fmt.Errorf("service.New: open db: %w", err)
	}

	return &Service{
		MemoryHome: memoryHome,
		VaultDir:   vaultDir,
		Config:     cfg,
		database:   database,
	}, nil
}

// Close releases all resources held by the service.
func (s *Service) Close() error {
	return s.database.Close()
}

// ---------------------------------------------------------------------------
// Lazy helpers
// ---------------------------------------------------------------------------

// embeddingProvider returns the Provider, lazily initialising it (thread-safe).
func (s *Service) embeddingProvider(_ context.Context) (embeddings.Provider, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.embProvider != nil {
		return s.embProvider, nil
	}
	ep, err := embeddings.NewProvider(s.Config)
	if err != nil {
		return nil, err
	}
	s.embProvider = ep
	return ep, nil
}

// getIgnorePatterns returns redaction patterns, lazily loaded from .memoryignore.
func (s *Service) getIgnorePatterns() []*regexp.Regexp {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ignorePatterns != nil {
		return s.ignorePatterns
	}
	patterns, err := redaction.LoadMemoryIgnore(filepath.Join(s.MemoryHome, ".memoryignore"))
	if err != nil {
		slog.Warn("failed to load .memoryignore", "err", err)
	}
	if patterns == nil {
		patterns = make([]*regexp.Regexp, 0)
	}
	s.ignorePatterns = patterns
	return patterns
}

// vectorsAvailable checks whether the vec table exists, caching the result.
func (s *Service) vectorsAvailable() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.vectorsOK != nil {
		return *s.vectorsOK
	}
	ok, err := s.database.HasVecTable()
	if err != nil {
		ok = false
	}
	s.vectorsOK = &ok
	return ok
}

// setVectorsOK updates the cached vector-availability flag.
func (s *Service) setVectorsOK(ok bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.vectorsOK = &ok
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

// mergeTags combines existing and extra tags, deduplicating case-insensitively.
func mergeTags(existing, extra []string) []string {
	norm := make(map[string]bool, len(existing))
	for _, t := range existing {
		norm[strings.ToLower(t)] = true
	}
	result := make([]string, len(existing))
	copy(result, existing)
	for _, t := range extra {
		if !norm[strings.ToLower(t)] {
			result = append(result, t)
			norm[strings.ToLower(t)] = true
		}
	}
	return result
}

// ensureVectors sets up the vec table for the given embedding dimension.
// Returns false when there is a dimension mismatch.
func (s *Service) ensureVectors(embedding []float32) bool {
	if err := s.database.EnsureVecTable(len(embedding)); err != nil {
		if errors.Is(err, db.ErrDimensionMismatch) {
			s.setVectorsOK(false)
		} else {
			slog.Warn("ensureVectors", "err", err)
		}
		return false
	}
	s.setVectorsOK(true)
	return true
}

// detailsWarnings returns quality warnings for memory details.
func detailsWarnings(raw *models.RawMemoryInput) []string {
	var warnings []string
	details := strings.TrimSpace(raw.Details)
	category := strings.ToLower(strings.TrimSpace(raw.Category))

	if (category == "decision" || category == "bug") && details == "" {
		warnings = append(warnings, fmt.Sprintf(
			"'%s' memories should include details. "+
				"Capture context, options considered, decision, tradeoffs, and follow-up.",
			category,
		))
		return warnings
	}

	if details == "" {
		return warnings
	}

	const minChars = 120
	if len(details) < minChars {
		warnings = append(warnings, fmt.Sprintf(
			"Details are brief (%d chars). Aim for at least %d chars for future-session context.",
			len(details), minChars,
		))
	}

	requiredSections := []string{"context", "options considered", "decision", "tradeoffs", "follow-up"}
	detailsLC := strings.ToLower(details)
	var missing []string
	for _, sec := range requiredSections {
		if !strings.Contains(detailsLC, sec) {
			missing = append(missing, sec)
		}
	}
	if len(missing) > 0 {
		warnings = append(warnings, "Details are missing recommended sections: "+strings.Join(missing, ", ")+".")
	}

	return warnings
}

// shouldUseSemantic determines whether semantic (vector) search should be used.
func (s *Service) shouldUseSemantic(mode string) bool {
	switch mode {
	case "never":
		return false
	case "always":
		return true
	}
	// "auto": for Ollama, only use if the model is currently loaded.
	if s.Config.Embedding.Provider == "ollama" {
		baseURL := s.Config.Embedding.BaseURL
		if baseURL == "" {
			baseURL = "http://localhost:11434"
		}
		return embeddings.IsOllamaModelLoaded(s.Config.Embedding.Model, baseURL)
	}
	return true
}

// resultsToMaps converts search.Result values into the map format used by
// GetContext/ListRecent so callers receive a uniform shape.
func resultsToMaps(results []search.Result) []map[string]any {
	out := make([]map[string]any, len(results))
	for i, r := range results {
		out[i] = map[string]any{
			"id":          r.ID,
			"title":       r.Title,
			"category":    r.Category,
			"tags":        r.Tags,
			"project":     r.Project,
			"source":      r.Source,
			"created_at":  r.CreatedAt,
			"has_details": r.HasDetails,
			"score":       r.Score,
		}
	}
	return out
}

// ---------------------------------------------------------------------------
// Save
// ---------------------------------------------------------------------------

// Save stores a memory with full pipeline: redact → dedup → markdown → db → embed.
// project is required and must be a non-empty string.
func (s *Service) Save(ctx context.Context, raw *models.RawMemoryInput, project string) (*models.SaveResult, error) { //nolint:gocognit,gocyclo // complexity is inherent to the dedup, redaction, markdown, db, and embedding pipeline
	if project == "" {
		return nil, fmt.Errorf("Save: project name is required")
	}

	today := time.Now().UTC().Format("2006-01-02")
	vaultProjectDir := filepath.Join(s.VaultDir, project)
	if err := os.MkdirAll(vaultProjectDir, 0o755); err != nil {
		return nil, fmt.Errorf("Save: create project dir: %w", err)
	}

	warnings := detailsWarnings(raw)

	// Redact all text fields.
	patterns := s.getIgnorePatterns()
	raw.Title = redaction.Redact(raw.Title, patterns)
	raw.What = redaction.Redact(raw.What, patterns)
	if raw.Why != "" {
		raw.Why = redaction.Redact(raw.Why, patterns)
	}
	if raw.Impact != "" {
		raw.Impact = redaction.Redact(raw.Impact, patterns)
	}
	if raw.Details != "" {
		raw.Details = redaction.Redact(raw.Details, patterns)
	}

	// Dedup check via FTS.
	dedupQuery := raw.Title + " " + raw.What
	candidates, dedupErr := s.database.FTSSearch(dedupQuery, 5, project, "")
	if dedupErr != nil {
		slog.Warn("Save: dedup search failed", "err", dedupErr)
	}
	if len(candidates) > 0 { //nolint:nestif // dedup logic requires evaluating multiple conditions across candidate results
		// Normalize top score against broader search for reliable thresholding.
		broad := candidates
		if len(broad) == 1 {
			if wider, err := s.database.FTSSearch(dedupQuery, 5, "", ""); err == nil && len(wider) > 0 {
				broad = wider
			}
		}
		var maxScore float64
		for _, c := range broad {
			if sc, ok := c["score"].(float64); ok && sc > maxScore {
				maxScore = sc
			}
		}

		top := candidates[0]
		var topScore float64
		if sc, ok := top["score"].(float64); ok {
			topScore = sc
		}
		var normalized float64
		if maxScore > 0 {
			normalized = topScore / maxScore
		}

		topTitle, _ := top["title"].(string)
		titleMatch := strings.EqualFold(strings.TrimSpace(raw.Title), strings.TrimSpace(topTitle))

		if normalized >= 0.7 && titleMatch {
			existingID, _ := top["id"].(string)
			existingFilePath, _ := top["file_path"].(string)

			var existingTags []string
			if tagsRaw, ok := top["tags"].(string); ok && tagsRaw != "" {
				_ = json.Unmarshal([]byte(tagsRaw), &existingTags)
			}
			mergedTags := mergeTags(existingTags, raw.Tags)

			var detailsAppend string
			if raw.Details != "" {
				detailsAppend = fmt.Sprintf("--- updated %s ---\n%s", today, raw.Details)
			}

			if _, err := s.database.UpdateMemory(existingID, raw.What, raw.Why, raw.Impact, mergedTags, detailsAppend); err != nil {
				return nil, fmt.Errorf("Save: update existing: %w", err)
			}

			// Re-embed the updated memory (non-fatal).
			if ep, err := s.embeddingProvider(ctx); err == nil && ep != nil {
				tagsStr := strings.Join(mergedTags, " ")
				embedText := fmt.Sprintf("%s %s %s %s %s", topTitle, raw.What, raw.Why, raw.Impact, tagsStr)
				if embedding, embedErr := ep.Embed(ctx, embedText); embedErr == nil {
					if s.ensureVectors(embedding) {
						if mem, found, dbErr := s.database.GetMemory(existingID); dbErr == nil && found {
							if rowid, ok := mem["rowid"].(int64); ok {
								if err := s.database.InsertVector(rowid, embedding); err != nil {
									slog.Warn("Save: re-embed insert vector", "err", err)
								}
							}
						}
					}
				} else {
					slog.Warn("Save: re-embed failed", "err", embedErr)
				}
			}

			return &models.SaveResult{
				ID:       existingID,
				FilePath: existingFilePath,
				Action:   "updated",
				Warnings: warnings,
			}, nil
		}
	}

	// Normal save path: create new memory.
	filePath := filepath.Join(vaultProjectDir, today+"-session.md")
	mem := models.FromRaw(raw, project, filePath)

	if err := markdown.WriteSessionMemory(vaultProjectDir, mem, today, raw.Details); err != nil {
		return nil, fmt.Errorf("Save: write markdown: %w", err)
	}

	rowid, err := s.database.InsertMemory(mem, raw.Details)
	if err != nil {
		return nil, fmt.Errorf("Save: insert memory: %w", err)
	}

	// Embed (non-fatal).
	if ep, epErr := s.embeddingProvider(ctx); epErr == nil && ep != nil {
		tagsStr := strings.Join(mem.Tags, " ")
		embedText := fmt.Sprintf("%s %s %s %s %s", mem.Title, mem.What, mem.Why, mem.Impact, tagsStr)
		if embedding, embedErr := ep.Embed(ctx, embedText); embedErr == nil {
			if !s.ensureVectors(embedding) {
				slog.Warn("Save: vector dimension mismatch — run 'memory reindex' to rebuild")
			} else if err := s.database.InsertVector(rowid, embedding); err != nil {
				slog.Warn("Save: insert vector", "err", err)
			}
		} else {
			slog.Warn("Save: embedding failed", "err", embedErr)
		}
	}

	return &models.SaveResult{
		ID:       mem.ID,
		FilePath: filePath,
		Action:   "created",
		Warnings: warnings,
	}, nil
}

// ---------------------------------------------------------------------------
// Search
// ---------------------------------------------------------------------------

// Search runs tiered FTS + vector search, falling back to FTS-only when vectors
// are unavailable or when useVectors is false.
//
//revive:disable:flag-parameter
func (s *Service) Search(ctx context.Context, query string, limit int, project, source string, useVectors bool) ([]search.Result, error) {
	if !useVectors {
		return search.HybridSearch(ctx, s.database, nil, query, limit, project, source)
	}

	if s.vectorsAvailable() {
		ep, err := s.embeddingProvider(ctx)
		if err != nil {
			slog.Warn("Search: embedding provider error", "err", err)
			ep = nil
		}
		results, err := search.TieredSearch(ctx, s.database, ep, query, limit, 0, project, source)
		if err == nil {
			return results, nil
		}
		if errors.Is(err, db.ErrDimensionMismatch) {
			s.setVectorsOK(false)
		} else {
			slog.Warn("Search: tiered search error", "err", err)
		}
	}

	// FTS-only fallback.
	return search.TieredSearch(ctx, s.database, nil, query, limit, 0, project, source)
}

//revive:enable:flag-parameter

// ---------------------------------------------------------------------------
// GetContext
// ---------------------------------------------------------------------------

// GetContext returns memory summaries for context injection along with the
// total count. semanticMode is one of "auto", "always", "never" (defaults to
// the value in Config when empty).
//
//revive:disable:flag-parameter
func (s *Service) GetContext( //nolint:gocognit // complexity from multiple semantic modes
	ctx context.Context,
	limit int,
	project, source, query, semanticMode string,
	topupRecent bool,
) ([]map[string]any, int, error) {
	total, err := s.database.CountMemories(project, source)
	if err != nil {
		return nil, 0, err
	}

	// Normalise semantic mode.
	if semanticMode == "" {
		semanticMode = s.Config.Context.Semantic
	}
	switch semanticMode {
	case "auto", "always", "never":
	default:
		semanticMode = "auto"
	}

	if query != "" { //nolint:nestif // top-up logic requires checking seen IDs across both search and recent results
		useVectors := s.shouldUseSemantic(semanticMode)
		results, err := s.Search(ctx, query, limit, project, source, useVectors)
		if err != nil {
			return nil, total, err
		}
		out := resultsToMaps(results)

		if topupRecent && len(out) < limit {
			recent, err := s.database.ListRecent(limit, project, source)
			if err == nil {
				seen := make(map[string]bool, len(out))
				for _, r := range out {
					if id, ok := r["id"].(string); ok {
						seen[id] = true
					}
				}
				for _, r := range recent {
					if id, ok := r["id"].(string); ok && seen[id] {
						continue
					}
					out = append(out, r)
					if len(out) >= limit {
						break
					}
				}
			}
		}
		return out, total, nil
	}

	recent, err := s.database.ListRecent(limit, project, source)
	if err != nil {
		return nil, total, err
	}
	return recent, total, nil
}

//revive:enable:flag-parameter

// ---------------------------------------------------------------------------
// GetDetails / Delete / CountMemories
// ---------------------------------------------------------------------------

// GetDetails fetches the extended body for a memory by ID or prefix.
func (s *Service) GetDetails(memoryID string) (*models.MemoryDetail, error) {
	return s.database.GetDetails(memoryID)
}

// Delete removes a memory by ID or prefix.
func (s *Service) Delete(memoryID string) (bool, error) {
	return s.database.DeleteMemory(memoryID)
}

// DeleteByFilter removes all memories older than olderThanDays, optionally
// filtered by project and/or category. Returns the number of deleted records.
func (s *Service) DeleteByFilter(project, category string, olderThanDays int) (int, error) {
	before := time.Now().UTC().AddDate(0, 0, -olderThanDays)
	return s.database.DeleteByFilter(project, category, before)
}

// reembedMemory re-generates and stores the embedding for an existing memory
// identified by id. All errors are logged as warnings and do not block the caller.
func (s *Service) reembedMemory(ctx context.Context, id, embedText string) {
	ep, err := s.embeddingProvider(ctx)
	if err != nil || ep == nil {
		return
	}
	embedding, err := ep.Embed(ctx, embedText)
	if err != nil {
		slog.Warn("reembedMemory: embedding failed", "err", err)
		return
	}
	if !s.ensureVectors(embedding) {
		return
	}
	mem, found, err := s.database.GetMemory(id)
	if err != nil || !found {
		return
	}
	rowid, ok := mem["rowid"].(int64)
	if !ok {
		return
	}
	if err := s.database.InsertVector(rowid, embedding); err != nil {
		slog.Warn("reembedMemory: insert vector", "err", err)
	}
}

// Replace fully overwrites an existing memory's content and re-embeds it.
// Returns a SaveResult with action "replaced", or an error if not found.
func (s *Service) Replace(ctx context.Context, id string, raw *models.RawMemoryInput) (*models.SaveResult, error) {
	// Redact all text fields.
	patterns := s.getIgnorePatterns()
	raw.Title = redaction.Redact(raw.Title, patterns)
	raw.What = redaction.Redact(raw.What, patterns)
	if raw.Why != "" {
		raw.Why = redaction.Redact(raw.Why, patterns)
	}
	if raw.Impact != "" {
		raw.Impact = redaction.Redact(raw.Impact, patterns)
	}
	if raw.Details != "" {
		raw.Details = redaction.Redact(raw.Details, patterns)
	}

	found, err := s.database.ReplaceMemory(
		id, raw.Title, raw.What, raw.Why, raw.Impact,
		raw.Tags, raw.RelatedFiles, raw.Category, raw.Details,
	)
	if err != nil {
		return nil, fmt.Errorf("Replace: %w", err)
	}
	if !found {
		return nil, fmt.Errorf("Replace: memory %q not found", id)
	}

	// Re-embed the replaced memory (non-fatal).
	tagsStr := strings.Join(raw.Tags, " ")
	embedText := fmt.Sprintf("%s %s %s %s %s", raw.Title, raw.What, raw.Why, raw.Impact, tagsStr)
	s.reembedMemory(ctx, id, embedText)

	return &models.SaveResult{
		ID:     id,
		Action: "replaced",
	}, nil
}

// CountMemories returns the total count of memories matching optional filters.
func (s *Service) CountMemories(project, source string) (int, error) {
	return s.database.CountMemories(project, source)
}

// ---------------------------------------------------------------------------
// Reindex
// ---------------------------------------------------------------------------

// Reindex rebuilds the vector table using the current embedding provider.
// progress is called with (current, total) after each memory is embedded; may be nil.
func (s *Service) Reindex(ctx context.Context, progress func(current, total int)) (*models.ReindexResult, error) {
	ep, err := s.embeddingProvider(ctx)
	if err != nil {
		return nil, fmt.Errorf("Reindex: embedding provider: %w", err)
	}
	if ep == nil {
		return nil, fmt.Errorf("Reindex: no embedding provider configured")
	}

	// Detect dimension from provider.
	probe, err := ep.Embed(ctx, "dimension probe")
	if err != nil {
		return nil, fmt.Errorf("Reindex: probe embed: %w", err)
	}
	dim := len(probe)

	// Rebuild vec table.
	if err := s.database.DropVecTable(); err != nil {
		return nil, fmt.Errorf("Reindex: drop vec table: %w", err)
	}
	if err := s.database.SetEmbeddingDim(dim); err != nil {
		return nil, fmt.Errorf("Reindex: set embedding dim: %w", err)
	}
	if err := s.database.CreateVecTable(dim); err != nil {
		return nil, fmt.Errorf("Reindex: create vec table: %w", err)
	}

	// Re-embed all memories.
	memories, err := s.database.ListAllForReindex()
	if err != nil {
		return nil, fmt.Errorf("Reindex: list memories: %w", err)
	}
	total := len(memories)

	for i, mem := range memories {
		tags := ""
		if tagsRaw, ok := mem["tags"].(string); ok && tagsRaw != "" {
			var tagSlice []string
			if jsonErr := json.Unmarshal([]byte(tagsRaw), &tagSlice); jsonErr == nil {
				tags = strings.Join(tagSlice, " ")
			} else {
				tags = tagsRaw
			}
		}

		title, _ := mem["title"].(string)
		what, _ := mem["what"].(string)
		why, _ := mem["why"].(string)
		impact, _ := mem["impact"].(string)
		embedText := fmt.Sprintf("%s %s %s %s %s", title, what, why, impact, tags)

		embedding, err := ep.Embed(ctx, embedText)
		if err != nil {
			return nil, fmt.Errorf("Reindex: embed memory: %w", err)
		}

		rowid, ok := mem["rowid"].(int64)
		if !ok {
			continue
		}
		if err := s.database.InsertVector(rowid, embedding); err != nil {
			return nil, fmt.Errorf("Reindex: insert vector: %w", err)
		}

		if progress != nil {
			progress(i+1, total)
		}
	}

	s.setVectorsOK(true)
	return &models.ReindexResult{
		Count: total,
		Dim:   dim,
		Model: s.Config.Embedding.Model,
	}, nil
}
