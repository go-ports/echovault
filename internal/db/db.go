// Package db manages the SQLite database with FTS5 and sqlite-vec extensions.
package db

import (
	"database/sql"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"strconv"
	"strings"
	"time"

	vec "github.com/asg017/sqlite-vec-go-bindings/cgo"
	_ "github.com/mattn/go-sqlite3" // registers the sqlite3 driver with database/sql

	"github.com/go-ports/echovault/internal/models"
)

func init() { //nolint:gochecknoinits // registers sqlite-vec extension with go-sqlite3 before any DB connection opens
	// Register the sqlite-vec extension with go-sqlite3 before any connection opens.
	vec.Auto()
}

// ErrDimensionMismatch is returned when a new embedding dimension differs from the one stored.
var ErrDimensionMismatch = errors.New("embedding dimension mismatch")

// DB wraps a *sql.DB with the path it was opened from.
type DB struct {
	db   *sql.DB
	path string
}

// Open opens (or creates) the SQLite database at path and initialises the schema.
func Open(path string) (*DB, error) {
	sqldb, err := sql.Open("sqlite3", path+"?_journal_mode=WAL&_foreign_keys=on")
	if err != nil {
		return nil, fmt.Errorf("db.Open: %w", err)
	}
	d := &DB{db: sqldb, path: path}
	if err := d.createSchema(); err != nil {
		_ = sqldb.Close()
		return nil, fmt.Errorf("db.Open createSchema: %w", err)
	}
	return d, nil
}

// Close closes the underlying database connection.
func (d *DB) Close() error {
	return d.db.Close()
}

// ---------------------------------------------------------------------------
// Schema
// ---------------------------------------------------------------------------

func (d *DB) createSchema() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS memories (
			rowid     INTEGER PRIMARY KEY AUTOINCREMENT,
			id        TEXT UNIQUE NOT NULL,
			title     TEXT NOT NULL,
			what      TEXT NOT NULL,
			why       TEXT,
			impact    TEXT,
			tags      TEXT,
			category  TEXT,
			project   TEXT NOT NULL,
			source    TEXT,
			related_files TEXT,
			file_path     TEXT NOT NULL,
			section_anchor TEXT,
			created_at     TEXT NOT NULL,
			updated_at     TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS memory_details (
			memory_id TEXT PRIMARY KEY REFERENCES memories(id),
			body      TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS meta (
			key   TEXT PRIMARY KEY,
			value TEXT NOT NULL
		)`,
		`CREATE VIRTUAL TABLE IF NOT EXISTS memories_fts USING fts5(
			title, what, why, impact, tags, category, project, source,
			content='memories', content_rowid='rowid',
			tokenize='porter unicode61'
		)`,
		`CREATE TRIGGER IF NOT EXISTS memories_ai AFTER INSERT ON memories BEGIN
			INSERT INTO memories_fts(rowid, title, what, why, impact, tags, category, project, source)
			VALUES (new.rowid, new.title, new.what, new.why, new.impact, new.tags, new.category, new.project, new.source);
		END`,
		`CREATE TRIGGER IF NOT EXISTS memories_au AFTER UPDATE ON memories BEGIN
			INSERT INTO memories_fts(memories_fts, rowid, title, what, why, impact, tags, category, project, source)
			VALUES ('delete', old.rowid, old.title, old.what, old.why, old.impact, old.tags, old.category, old.project, old.source);
			INSERT INTO memories_fts(rowid, title, what, why, impact, tags, category, project, source)
			VALUES (new.rowid, new.title, new.what, new.why, new.impact, new.tags, new.category, new.project, new.source);
		END`,
		`CREATE TRIGGER IF NOT EXISTS memories_ad AFTER DELETE ON memories BEGIN
			INSERT INTO memories_fts(memories_fts, rowid, title, what, why, impact, tags, category, project, source)
			VALUES ('delete', old.rowid, old.title, old.what, old.why, old.impact, old.tags, old.category, old.project, old.source);
		END`,
	}

	for _, s := range stmts {
		if _, err := d.db.Exec(s); err != nil {
			return fmt.Errorf("createSchema exec: %w\nSQL: %s", err, s)
		}
	}

	// Migration: add updated_count column if missing.
	rows, err := d.db.Query("PRAGMA table_info(memories)")
	if err != nil {
		return err
	}
	cols := make(map[string]bool)
	for rows.Next() {
		var cid int
		var name, typ string
		var notNull, pk int
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &typ, &notNull, &dflt, &pk); err != nil {
			rows.Close()
			return err
		}
		cols[name] = true
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	if !cols["updated_count"] {
		if _, err := d.db.Exec("ALTER TABLE memories ADD COLUMN updated_count INTEGER DEFAULT 0"); err != nil {
			return fmt.Errorf("migration updated_count: %w", err)
		}
	}

	// Recreate vec table if dimension was previously persisted.
	if dim, ok, err := d.GetEmbeddingDim(); err == nil && ok {
		if err := d.createVecTable(dim); err != nil {
			return fmt.Errorf("createSchema createVecTable: %w", err)
		}
	}

	return nil
}

// ---------------------------------------------------------------------------
// Vector table helpers
// ---------------------------------------------------------------------------

// CreateVecTable creates the vec0 virtual table with the given embedding dimension.
// It is safe to call when the table already exists (uses IF NOT EXISTS).
func (d *DB) CreateVecTable(dim int) error { return d.createVecTable(dim) }

func (d *DB) createVecTable(dim int) error {
	_, err := d.db.Exec(fmt.Sprintf(
		`CREATE VIRTUAL TABLE IF NOT EXISTS memories_vec USING vec0(
			rowid INTEGER PRIMARY KEY,
			embedding float[%d]
		)`, dim,
	))
	return err
}

// HasVecTable returns true if the memories_vec table exists.
func (d *DB) HasVecTable() (bool, error) {
	var name string
	err := d.db.QueryRow(
		`SELECT name FROM sqlite_master WHERE type='table' AND name='memories_vec'`,
	).Scan(&name)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}

// DropVecTable drops the memories_vec virtual table if it exists.
func (d *DB) DropVecTable() error {
	_, err := d.db.Exec("DROP TABLE IF EXISTS memories_vec")
	return err
}

// GetEmbeddingDim reads the stored embedding dimension from the meta table.
func (d *DB) GetEmbeddingDim() (int, bool, error) {
	val, ok, err := d.GetMeta("embedding_dim")
	if !ok || err != nil {
		return 0, false, err
	}
	dim, err := strconv.Atoi(val)
	if err != nil {
		return 0, false, err
	}
	return dim, true, nil
}

// SetEmbeddingDim persists the embedding dimension in the meta table.
func (d *DB) SetEmbeddingDim(dim int) error {
	return d.SetMeta("embedding_dim", strconv.Itoa(dim))
}

// EnsureVecTable ensures the vector table exists with the given dimension.
// Returns ErrDimensionMismatch if the stored dimension differs.
func (d *DB) EnsureVecTable(dim int) error {
	stored, ok, err := d.GetEmbeddingDim()
	if err != nil {
		return err
	}
	if !ok {
		if err := d.SetEmbeddingDim(dim); err != nil {
			return err
		}
		return d.createVecTable(dim)
	}
	if stored != dim {
		return fmt.Errorf("%w: database has %d, provider returned %d. Run 'memory reindex' to rebuild",
			ErrDimensionMismatch, stored, dim)
	}
	return nil
}

// ---------------------------------------------------------------------------
// CRUD
// ---------------------------------------------------------------------------

// InsertMemory inserts a memory record and optional details body.
// Returns the rowid of the inserted row.
func (d *DB) InsertMemory(mem *models.Memory, details string) (int64, error) {
	tagsJSON, err := json.Marshal(mem.Tags)
	if err != nil {
		return 0, err
	}
	filesJSON, err := json.Marshal(mem.RelatedFiles)
	if err != nil {
		return 0, err
	}

	res, err := d.db.Exec(`
		INSERT INTO memories (
			id, title, what, why, impact, tags, category, project,
			source, related_files, file_path, section_anchor,
			created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		mem.ID, mem.Title, mem.What, mem.Why, mem.Impact,
		string(tagsJSON), mem.Category, mem.Project, mem.Source,
		string(filesJSON), mem.FilePath, mem.SectionAnchor,
		mem.CreatedAt.Format(time.RFC3339), mem.UpdatedAt.Format(time.RFC3339),
	)
	if err != nil {
		return 0, fmt.Errorf("InsertMemory: %w", err)
	}

	rowid, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}

	if details != "" {
		if _, err := d.db.Exec(
			`INSERT INTO memory_details (memory_id, body) VALUES (?, ?)`,
			mem.ID, details,
		); err != nil {
			return rowid, fmt.Errorf("InsertMemory details: %w", err)
		}
	}
	return rowid, nil
}

// InsertVector stores an embedding vector for the given memory rowid.
// Silently skips if the vec table does not exist.
func (d *DB) InsertVector(rowid int64, embedding []float32) error {
	ok, err := d.HasVecTable()
	if err != nil || !ok {
		return err
	}
	b := float32sToBytes(embedding)
	_, err = d.db.Exec(
		`INSERT OR REPLACE INTO memories_vec (rowid, embedding) VALUES (?, ?)`,
		rowid, b,
	)
	return err
}

// GetMemory fetches a single memory by exact ID.
func (d *DB) GetMemory(id string) (map[string]any, bool, error) {
	rows, err := d.db.Query(`
		SELECT m.*,
		       EXISTS(SELECT 1 FROM memory_details WHERE memory_id = m.id) AS has_details
		FROM memories m WHERE m.id = ? LIMIT 1`, id)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()
	results, err := scanRows(rows)
	if err != nil || len(results) == 0 {
		return nil, false, err
	}
	return results[0], true, nil
}

// GetDetails returns the full details body for a memory (prefix-matched ID).
func (d *DB) GetDetails(id string) (*models.MemoryDetail, error) {
	var memID, body string
	err := d.db.QueryRow(
		`SELECT memory_id, body FROM memory_details WHERE memory_id LIKE ?`,
		id+"%",
	).Scan(&memID, &body)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &models.MemoryDetail{MemoryID: memID, Body: body}, nil
}

// UpdateMemory updates mutable fields of an existing memory (prefix-matched ID).
// Empty string arguments are skipped. nil tags are skipped.
// Returns true if the memory was found and updated.
func (d *DB) UpdateMemory(id, what, why, impact string, tags []string, detailsAppend string) (bool, error) {
	// Resolve full ID.
	var fullID string
	err := d.db.QueryRow(
		`SELECT id FROM memories WHERE id LIKE ?`, id+"%",
	).Scan(&fullID)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	sets := []string{"updated_count = updated_count + 1", "updated_at = ?"}
	params := []any{time.Now().UTC().Format(time.RFC3339)}

	if what != "" {
		sets = append(sets, "what = ?")
		params = append(params, what)
	}
	if why != "" {
		sets = append(sets, "why = ?")
		params = append(params, why)
	}
	if impact != "" {
		sets = append(sets, "impact = ?")
		params = append(params, impact)
	}
	if tags != nil {
		b, _ := json.Marshal(tags)
		sets = append(sets, "tags = ?")
		params = append(params, string(b))
	}

	params = append(params, fullID)
	updQ := "UPDATE memories SET " + strings.Join(sets, ", ") + " WHERE id = ?" // #nosec G202 -- SET clause columns are hardcoded; values flow through ? bound parameters
	_, err = d.db.Exec(updQ, params...)
	if err != nil {
		return false, fmt.Errorf("UpdateMemory: %w", err)
	}

	if detailsAppend != "" {
		var existing string
		scanErr := d.db.QueryRow(
			`SELECT body FROM memory_details WHERE memory_id = ?`, fullID,
		).Scan(&existing)
		switch {
		case errors.Is(scanErr, sql.ErrNoRows):
			_, err = d.db.Exec(
				`INSERT INTO memory_details (memory_id, body) VALUES (?, ?)`,
				fullID, detailsAppend,
			)
		case scanErr == nil:
			_, err = d.db.Exec(
				`UPDATE memory_details SET body = ? WHERE memory_id = ?`,
				existing+"\n\n"+detailsAppend, fullID,
			)
		default:
			err = scanErr
		}
		if err != nil {
			return false, fmt.Errorf("UpdateMemory details: %w", err)
		}
	}

	return true, nil
}

// DeleteMemory deletes a memory and its details by exact ID or prefix.
// Returns true if a record was found and deleted.
func (d *DB) DeleteMemory(id string) (bool, error) {
	var fullID string
	var rowid int64
	err := d.db.QueryRow(
		`SELECT id, rowid FROM memories WHERE id LIKE ?`, id+"%",
	).Scan(&fullID, &rowid)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	if _, err := d.db.Exec(`DELETE FROM memory_details WHERE memory_id = ?`, fullID); err != nil {
		return false, err
	}
	// Clean up vector index before deleting the memory row (rowid is needed).
	if _, err := d.db.Exec(`DELETE FROM memories_vec WHERE rowid = ?`, rowid); err != nil {
		// Non-fatal: vec table may not exist yet.
		slog.Debug("DeleteMemory: vec cleanup skipped", "err", err)
	}
	if _, err := d.db.Exec(`DELETE FROM memories WHERE id = ?`, fullID); err != nil {
		return false, err
	}
	return true, nil
}

// DeleteByFilter deletes all memories whose created_at is before `before`,
// optionally filtered by project and/or category.
// Returns the number of deleted records.
func (d *DB) DeleteByFilter(project, category string, before time.Time) (int, error) {
	// Collect rowids and IDs to handle cascaded cleanup.
	var clauses []string
	var params []any
	clauses = append(clauses, "created_at < ?")
	params = append(params, before.UTC().Format(time.RFC3339))
	if project != "" {
		clauses = append(clauses, "project = ?")
		params = append(params, project)
	}
	if category != "" {
		clauses = append(clauses, "category = ?")
		params = append(params, category)
	}
	where := " WHERE " + strings.Join(clauses, " AND ")

	rows, err := d.db.Query("SELECT id, rowid FROM memories"+where, params...) // #nosec G202 -- WHERE clause uses hardcoded column names only; values flow through ? bound parameters
	if err != nil {
		return 0, fmt.Errorf("DeleteByFilter: query: %w", err)
	}
	type entry struct {
		id    string
		rowid int64
	}
	var entries []entry
	for rows.Next() {
		var e entry
		if err := rows.Scan(&e.id, &e.rowid); err != nil {
			rows.Close()
			return 0, fmt.Errorf("DeleteByFilter: scan: %w", err)
		}
		entries = append(entries, e)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("DeleteByFilter: rows: %w", err)
	}

	for _, e := range entries {
		if _, err := d.db.Exec(`DELETE FROM memory_details WHERE memory_id = ?`, e.id); err != nil {
			return 0, fmt.Errorf("DeleteByFilter: details: %w", err)
		}
		if _, err := d.db.Exec(`DELETE FROM memories_vec WHERE rowid = ?`, e.rowid); err != nil {
			slog.Debug("DeleteByFilter: vec cleanup skipped", "err", err)
		}
		if _, err := d.db.Exec(`DELETE FROM memories WHERE id = ?`, e.id); err != nil {
			return 0, fmt.Errorf("DeleteByFilter: memory: %w", err)
		}
	}
	return len(entries), nil
}

// ReplaceMemory fully overwrites all mutable fields of an existing memory
// (prefix-matched by ID) and replaces the details body.
// Returns true if the memory was found and replaced.
func (d *DB) ReplaceMemory(id, title, what, why, impact string, tags, relatedFiles []string, category, details string) (bool, error) {
	var fullID string
	err := d.db.QueryRow(
		`SELECT id FROM memories WHERE id LIKE ?`, id+"%",
	).Scan(&fullID)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	tagsJSON, err := json.Marshal(tags)
	if err != nil {
		return false, fmt.Errorf("ReplaceMemory: marshal tags: %w", err)
	}
	filesJSON, err := json.Marshal(relatedFiles)
	if err != nil {
		return false, fmt.Errorf("ReplaceMemory: marshal files: %w", err)
	}

	_, err = d.db.Exec(`
		UPDATE memories
		SET title = ?, what = ?, why = ?, impact = ?, tags = ?,
		    related_files = ?, category = ?,
		    updated_at = ?, updated_count = updated_count + 1
		WHERE id = ?`,
		title, what, why, impact, string(tagsJSON),
		string(filesJSON), category,
		time.Now().UTC().Format(time.RFC3339), fullID,
	)
	if err != nil {
		return false, fmt.Errorf("ReplaceMemory: update: %w", err)
	}

	if details != "" {
		_, err = d.db.Exec(
			`INSERT OR REPLACE INTO memory_details (memory_id, body) VALUES (?, ?)`,
			fullID, details,
		)
	} else {
		_, err = d.db.Exec(`DELETE FROM memory_details WHERE memory_id = ?`, fullID)
	}
	if err != nil {
		return false, fmt.Errorf("ReplaceMemory: details: %w", err)
	}
	return true, nil
}

// ---------------------------------------------------------------------------
// Search
// ---------------------------------------------------------------------------

// FTSSearch performs a BM25 full-text search over memories.
func (d *DB) FTSSearch(query string, limit int, project, source string) ([]map[string]any, error) {
	if query == "" {
		return nil, nil
	}

	// Build "term1"* OR "term2"* FTS5 query.
	terms := strings.Fields(query)
	ftsParts := make([]string, len(terms))
	for i, t := range terms {
		ftsParts[i] = `"` + strings.ReplaceAll(t, `"`, `""`) + `"*`
	}
	ftsQuery := strings.Join(ftsParts, " OR ")

	where, params := buildWhere("m", project, source)
	// The FTS query already has WHERE fts.memories_fts MATCH ?; additional
	// project/source filters must be AND conditions, not a second WHERE clause.
	where = strings.Replace(where, " WHERE ", " AND ", 1)
	params = append([]any{ftsQuery}, params...)
	params = append(params, limit)

	ftsQ := `
		SELECT m.*, -fts.rank AS score,
		       EXISTS(SELECT 1 FROM memory_details WHERE memory_id = m.id) AS has_details
		FROM memories_fts fts
		JOIN memories m ON m.rowid = fts.rowid
		WHERE fts.memories_fts MATCH ?`
	q := ftsQ + where + "\n\t\tORDER BY fts.rank\n\t\tLIMIT ?" // #nosec G202 -- AND clause uses hardcoded column names only; values flow through ? bound parameters

	rows, err := d.db.Query(q, params...)
	if err != nil {
		return nil, fmt.Errorf("FTSSearch: %w", err)
	}
	defer rows.Close()
	return scanRows(rows)
}

// VectorSearch performs approximate nearest-neighbour search using sqlite-vec.
func (d *DB) VectorSearch(queryEmbedding []float32, limit int, project, source string) ([]map[string]any, error) {
	ok, err := d.HasVecTable()
	if err != nil || !ok {
		return nil, err
	}

	vecBytes := float32sToBytes(queryEmbedding)

	rows, err := d.db.Query(`
		SELECT m.*, v.distance,
		       EXISTS(SELECT 1 FROM memory_details WHERE memory_id = m.id) AS has_details
		FROM memories_vec v
		JOIN memories m ON m.rowid = v.rowid
		WHERE v.embedding MATCH ? AND k = ?
		ORDER BY v.distance`,
		vecBytes, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("VectorSearch: %w", err)
	}
	defer rows.Close()

	all, err := scanRows(rows)
	if err != nil {
		return nil, err
	}

	// Convert distance → score and post-filter by project/source.
	results := make([]map[string]any, 0, len(all))
	for _, r := range all {
		if project != "" {
			if p, _ := r["project"].(string); p != project {
				continue
			}
		}
		if source != "" {
			if s, _ := r["source"].(string); s != source {
				continue
			}
		}
		if dist, ok := r["distance"].(float64); ok {
			r["score"] = 1.0 - dist
			delete(r, "distance")
		}
		results = append(results, r)
	}
	return results, nil
}

// ListRecent returns recently created memories, newest first.
func (d *DB) ListRecent(limit int, project, source string) ([]map[string]any, error) {
	where, params := buildWhere("m", project, source)
	params = append(params, limit)

	listQ := `
		SELECT m.id, m.title, m.category, m.tags, m.project, m.source, m.created_at,
		       EXISTS(SELECT 1 FROM memory_details WHERE memory_id = m.id) AS has_details
		FROM memories m`
	listQ += where + "\n\t\tORDER BY m.created_at DESC\n\t\tLIMIT ?" // #nosec G202 -- WHERE clause uses hardcoded column names only; values flow through ? bound parameters
	rows, err := d.db.Query(listQ, params...)
	if err != nil {
		return nil, fmt.Errorf("ListRecent: %w", err)
	}
	defer rows.Close()
	return scanRows(rows)
}

// CountMemories returns the total number of memories matching optional filters.
func (d *DB) CountMemories(project, source string) (int, error) {
	where, params := buildWhere("", project, source)
	countQ := "SELECT COUNT(*) FROM memories" + where
	var n int
	err := d.db.QueryRow(countQ, params...).Scan(&n)
	return n, err
}

// ListAllForReindex returns all memories with fields needed for re-embedding.
func (d *DB) ListAllForReindex() ([]map[string]any, error) {
	rows, err := d.db.Query(
		`SELECT rowid, title, what, why, impact, tags FROM memories ORDER BY rowid`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRows(rows)
}

// ---------------------------------------------------------------------------
// Meta
// ---------------------------------------------------------------------------

// GetMeta returns the value for key, or ("", false, nil) if not set.
func (d *DB) GetMeta(key string) (string, bool, error) {
	var val string
	err := d.db.QueryRow(`SELECT value FROM meta WHERE key = ?`, key).Scan(&val)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return val, true, nil
}

// SetMeta upserts a key-value pair in the meta table.
func (d *DB) SetMeta(key, value string) error {
	_, err := d.db.Exec(
		`INSERT OR REPLACE INTO meta (key, value) VALUES (?, ?)`, key, value,
	)
	return err
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// buildWhere constructs a WHERE / AND clause for optional project and source filters.
// tableAlias is the SQL alias prefix (e.g. "m"); pass "" for unaliased queries.
func buildWhere(tableAlias, project, source string) (string, []any) {
	prefix := ""
	if tableAlias != "" {
		prefix = tableAlias + "."
	}
	var clauses []string
	var params []any
	if project != "" {
		clauses = append(clauses, prefix+"project = ?")
		params = append(params, project)
	}
	if source != "" {
		clauses = append(clauses, prefix+"source = ?")
		params = append(params, source)
	}
	if len(clauses) == 0 {
		return "", params
	}
	return " WHERE " + strings.Join(clauses, " AND "), params
}

// float32sToBytes encodes a []float32 as little-endian bytes (sqlite-vec wire format).
func float32sToBytes(floats []float32) []byte {
	b := make([]byte, len(floats)*4)
	for i, f := range floats {
		binary.LittleEndian.PutUint32(b[i*4:], math.Float32bits(f))
	}
	return b
}

// scanRows reads all rows
func scanRows(rows *sql.Rows) ([]map[string]any, error) {
	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	var results []map[string]any
	for rows.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		m := make(map[string]any, len(cols))
		for i, col := range cols {
			// Convert []byte to string for TEXT columns.
			if b, ok := vals[i].([]byte); ok {
				m[col] = string(b)
			} else {
				m[col] = vals[i]
			}
		}
		results = append(results, m)
	}
	return results, rows.Err()
}
