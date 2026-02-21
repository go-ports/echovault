package db_test

import (
	"path/filepath"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"

	"github.com/go-ports/echovault/internal/db"
	"github.com/go-ports/echovault/internal/models"
)

// newMemAt returns a *models.Memory with the given created/updated timestamps.
func newMemAt(id, title, project string, createdAt time.Time) *models.Memory {
	return &models.Memory{
		ID:        id,
		Title:     title,
		What:      "what about " + title,
		Project:   project,
		FilePath:  "/vault/" + project + "/2024-01-15-session.md",
		CreatedAt: createdAt,
		UpdatedAt: createdAt,
	}
}

// openTestDB opens a fresh SQLite database in a temp directory and registers
// t.Cleanup to close it.
func openTestDB(t *testing.T) *db.DB {
	t.Helper()
	d, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("openTestDB: %v", err)
	}
	t.Cleanup(func() { _ = d.Close() })
	return d
}

// newMem returns a minimal *models.Memory with a unique ID.
func newMem(id, title, project string) *models.Memory {
	now := time.Now().UTC()
	return &models.Memory{
		ID:        id,
		Title:     title,
		What:      "what about " + title,
		Project:   project,
		FilePath:  "/vault/" + project + "/2024-01-15-session.md",
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// ---------------------------------------------------------------------------
// Open
// ---------------------------------------------------------------------------

func TestOpen_HappyPath(t *testing.T) {
	c := qt.New(t)
	d := openTestDB(t)
	c.Assert(d, qt.IsNotNil)
}

// ---------------------------------------------------------------------------
// InsertMemory / GetMemory
// ---------------------------------------------------------------------------

func TestInsertAndGetMemory_HappyPath(t *testing.T) {
	c := qt.New(t)

	c.Run("inserted memory is retrievable by exact ID", func(c *qt.C) {
		d := openTestDB(t)
		mem := newMem("id-abc", "Alpha", "myproject")
		mem.Tags = []string{"go", "test"}
		mem.Category = "decision"
		mem.Why = "because reasons"

		_, err := d.InsertMemory(mem, "")
		c.Assert(err, qt.IsNil)

		got, found, err := d.GetMemory("id-abc")
		c.Assert(err, qt.IsNil)
		c.Assert(found, qt.IsTrue)
		c.Assert(got["id"], qt.Equals, "id-abc")
		c.Assert(got["title"], qt.Equals, "Alpha")
		c.Assert(got["project"], qt.Equals, "myproject")
		c.Assert(got["category"], qt.Equals, "decision")
	})

	c.Run("unknown ID returns not-found", func(c *qt.C) {
		d := openTestDB(t)
		_, found, err := d.GetMemory("nonexistent")
		c.Assert(err, qt.IsNil)
		c.Assert(found, qt.IsFalse)
	})

	c.Run("has_details is false when no details provided", func(c *qt.C) {
		d := openTestDB(t)
		_, err := d.InsertMemory(newMem("id-1", "T", "p"), "")
		c.Assert(err, qt.IsNil)

		got, _, err := d.GetMemory("id-1")
		c.Assert(err, qt.IsNil)
		// has_details is stored as int64 0 in SQLite
		c.Assert(got["has_details"], qt.Equals, int64(0))
	})

	c.Run("has_details is true when details provided", func(c *qt.C) {
		d := openTestDB(t)
		_, err := d.InsertMemory(newMem("id-2", "T", "p"), "some details body")
		c.Assert(err, qt.IsNil)

		got, _, err := d.GetMemory("id-2")
		c.Assert(err, qt.IsNil)
		c.Assert(got["has_details"], qt.Equals, int64(1))
	})
}

// ---------------------------------------------------------------------------
// GetDetails
// ---------------------------------------------------------------------------

func TestGetDetails_HappyPath(t *testing.T) {
	c := qt.New(t)

	c.Run("returns details body for memory with details", func(c *qt.C) {
		d := openTestDB(t)
		_, err := d.InsertMemory(newMem("id-d", "Title", "proj"), "the full context")
		c.Assert(err, qt.IsNil)

		detail, err := d.GetDetails("id-d")
		c.Assert(err, qt.IsNil)
		c.Assert(detail, qt.IsNotNil)
		c.Assert(detail.Body, qt.Equals, "the full context")
		c.Assert(detail.MemoryID, qt.Equals, "id-d")
	})

	c.Run("returns nil for memory without details", func(c *qt.C) {
		d := openTestDB(t)
		_, err := d.InsertMemory(newMem("id-nd", "Title", "proj"), "")
		c.Assert(err, qt.IsNil)

		detail, err := d.GetDetails("id-nd")
		c.Assert(err, qt.IsNil)
		c.Assert(detail, qt.IsNil)
	})

	c.Run("prefix match works", func(c *qt.C) {
		d := openTestDB(t)
		_, err := d.InsertMemory(newMem("abcdef-1234", "T", "p"), "prefix body")
		c.Assert(err, qt.IsNil)

		detail, err := d.GetDetails("abcdef")
		c.Assert(err, qt.IsNil)
		c.Assert(detail, qt.IsNotNil)
		c.Assert(detail.Body, qt.Equals, "prefix body")
	})
}

// ---------------------------------------------------------------------------
// UpdateMemory
// ---------------------------------------------------------------------------

func TestUpdateMemory_HappyPath(t *testing.T) {
	c := qt.New(t)

	c.Run("updates what and why fields", func(c *qt.C) {
		d := openTestDB(t)
		_, err := d.InsertMemory(newMem("upd-1", "Original", "p"), "")
		c.Assert(err, qt.IsNil)

		ok, err := d.UpdateMemory("upd-1", "new what", "new why", "", nil, "")
		c.Assert(err, qt.IsNil)
		c.Assert(ok, qt.IsTrue)

		got, found, err := d.GetMemory("upd-1")
		c.Assert(err, qt.IsNil)
		c.Assert(found, qt.IsTrue)
		c.Assert(got["what"], qt.Equals, "new what")
		c.Assert(got["why"], qt.Equals, "new why")
	})

	c.Run("non-existent ID returns false", func(c *qt.C) {
		d := openTestDB(t)
		ok, err := d.UpdateMemory("ghost", "w", "", "", nil, "")
		c.Assert(err, qt.IsNil)
		c.Assert(ok, qt.IsFalse)
	})

	c.Run("appends to existing details", func(c *qt.C) {
		d := openTestDB(t)
		_, err := d.InsertMemory(newMem("upd-2", "T", "p"), "original details")
		c.Assert(err, qt.IsNil)

		_, err = d.UpdateMemory("upd-2", "", "", "", nil, "appended")
		c.Assert(err, qt.IsNil)

		detail, err := d.GetDetails("upd-2")
		c.Assert(err, qt.IsNil)
		c.Assert(detail, qt.IsNotNil)
		c.Assert(detail.Body, qt.Contains, "original details")
		c.Assert(detail.Body, qt.Contains, "appended")
	})
}

// ---------------------------------------------------------------------------
// DeleteMemory
// ---------------------------------------------------------------------------

func TestDeleteMemory_HappyPath(t *testing.T) {
	c := qt.New(t)

	c.Run("deletes existing memory", func(c *qt.C) {
		d := openTestDB(t)
		_, err := d.InsertMemory(newMem("del-1", "T", "p"), "")
		c.Assert(err, qt.IsNil)

		ok, err := d.DeleteMemory("del-1")
		c.Assert(err, qt.IsNil)
		c.Assert(ok, qt.IsTrue)

		_, found, err := d.GetMemory("del-1")
		c.Assert(err, qt.IsNil)
		c.Assert(found, qt.IsFalse)
	})

	c.Run("deleting non-existent returns false", func(c *qt.C) {
		d := openTestDB(t)
		ok, err := d.DeleteMemory("ghost")
		c.Assert(err, qt.IsNil)
		c.Assert(ok, qt.IsFalse)
	})

	c.Run("prefix delete resolves correctly", func(c *qt.C) {
		d := openTestDB(t)
		_, err := d.InsertMemory(newMem("prefix-abc123", "T", "p"), "")
		c.Assert(err, qt.IsNil)

		ok, err := d.DeleteMemory("prefix-abc")
		c.Assert(err, qt.IsNil)
		c.Assert(ok, qt.IsTrue)
	})
}

// ---------------------------------------------------------------------------
// GetMeta / SetMeta
// ---------------------------------------------------------------------------

func TestGetMeta_SetMeta_HappyPath(t *testing.T) {
	c := qt.New(t)

	c.Run("set and get a key-value pair", func(c *qt.C) {
		d := openTestDB(t)
		err := d.SetMeta("mykey", "myvalue")
		c.Assert(err, qt.IsNil)

		val, found, err := d.GetMeta("mykey")
		c.Assert(err, qt.IsNil)
		c.Assert(found, qt.IsTrue)
		c.Assert(val, qt.Equals, "myvalue")
	})

	c.Run("get missing key returns not-found", func(c *qt.C) {
		d := openTestDB(t)
		_, found, err := d.GetMeta("absent")
		c.Assert(err, qt.IsNil)
		c.Assert(found, qt.IsFalse)
	})

	c.Run("upsert overwrites existing value", func(c *qt.C) {
		d := openTestDB(t)
		c.Assert(d.SetMeta("k", "v1"), qt.IsNil)
		c.Assert(d.SetMeta("k", "v2"), qt.IsNil)

		val, _, err := d.GetMeta("k")
		c.Assert(err, qt.IsNil)
		c.Assert(val, qt.Equals, "v2")
	})
}

// ---------------------------------------------------------------------------
// GetEmbeddingDim / SetEmbeddingDim / EnsureVecTable
// ---------------------------------------------------------------------------

func TestEmbeddingDim_HappyPath(t *testing.T) {
	c := qt.New(t)

	c.Run("no dim stored returns not-found", func(c *qt.C) {
		d := openTestDB(t)
		_, found, err := d.GetEmbeddingDim()
		c.Assert(err, qt.IsNil)
		c.Assert(found, qt.IsFalse)
	})

	c.Run("set and get dim round-trips", func(c *qt.C) {
		d := openTestDB(t)
		c.Assert(d.SetEmbeddingDim(768), qt.IsNil)

		dim, found, err := d.GetEmbeddingDim()
		c.Assert(err, qt.IsNil)
		c.Assert(found, qt.IsTrue)
		c.Assert(dim, qt.Equals, 768)
	})

	c.Run("EnsureVecTable creates table on first call", func(c *qt.C) {
		d := openTestDB(t)
		err := d.EnsureVecTable(384)
		c.Assert(err, qt.IsNil)

		ok, err := d.HasVecTable()
		c.Assert(err, qt.IsNil)
		c.Assert(ok, qt.IsTrue)
	})

	c.Run("EnsureVecTable returns ErrDimensionMismatch on mismatch", func(c *qt.C) {
		d := openTestDB(t)
		c.Assert(d.EnsureVecTable(384), qt.IsNil)

		err := d.EnsureVecTable(512)
		c.Assert(err, qt.ErrorIs, db.ErrDimensionMismatch)
	})
}

// ---------------------------------------------------------------------------
// CountMemories
// ---------------------------------------------------------------------------

func TestCountMemories_HappyPath(t *testing.T) {
	c := qt.New(t)

	c.Run("empty DB returns zero", func(c *qt.C) {
		d := openTestDB(t)
		n, err := d.CountMemories("", "")
		c.Assert(err, qt.IsNil)
		c.Assert(n, qt.Equals, 0)
	})

	c.Run("counts all memories across projects", func(c *qt.C) {
		d := openTestDB(t)
		_, _ = d.InsertMemory(newMem("c1", "T1", "proj-a"), "")
		_, _ = d.InsertMemory(newMem("c2", "T2", "proj-b"), "")

		n, err := d.CountMemories("", "")
		c.Assert(err, qt.IsNil)
		c.Assert(n, qt.Equals, 2)
	})

	c.Run("project filter narrows count", func(c *qt.C) {
		d := openTestDB(t)
		_, _ = d.InsertMemory(newMem("f1", "T1", "proj-a"), "")
		_, _ = d.InsertMemory(newMem("f2", "T2", "proj-b"), "")

		n, err := d.CountMemories("proj-a", "")
		c.Assert(err, qt.IsNil)
		c.Assert(n, qt.Equals, 1)
	})
}

// ---------------------------------------------------------------------------
// FTSSearch
// ---------------------------------------------------------------------------

func TestFTSSearch_HappyPath(t *testing.T) {
	c := qt.New(t)

	c.Run("empty query returns nil", func(c *qt.C) {
		d := openTestDB(t)
		rows, err := d.FTSSearch("", 10, "", "")
		c.Assert(err, qt.IsNil)
		c.Assert(rows, qt.IsNil)
	})

	c.Run("search finds inserted memory by title term", func(c *qt.C) {
		d := openTestDB(t)
		mem := newMem("fts-1", "Golang channels explained", "proj")
		_, err := d.InsertMemory(mem, "")
		c.Assert(err, qt.IsNil)

		rows, err := d.FTSSearch("golang", 10, "", "")
		c.Assert(err, qt.IsNil)
		c.Assert(rows, qt.HasLen, 1)
		c.Assert(rows[0]["id"], qt.Equals, "fts-1")
	})

	c.Run("project filter excludes other projects", func(c *qt.C) {
		d := openTestDB(t)
		_, _ = d.InsertMemory(newMem("p1", "Refactoring tips", "proj-a"), "")
		_, _ = d.InsertMemory(newMem("p2", "Refactoring guide", "proj-b"), "")

		rows, err := d.FTSSearch("refactoring", 10, "proj-a", "")
		c.Assert(err, qt.IsNil)
		c.Assert(rows, qt.HasLen, 1)
		c.Assert(rows[0]["id"], qt.Equals, "p1")
	})

	c.Run("limit is respected", func(c *qt.C) {
		d := openTestDB(t)
		_, _ = d.InsertMemory(newMem("l1", "SQLite performance tuning", "p"), "")
		_, _ = d.InsertMemory(newMem("l2", "SQLite WAL mode explained", "p"), "")
		_, _ = d.InsertMemory(newMem("l3", "SQLite FTS5 full text search", "p"), "")

		rows, err := d.FTSSearch("sqlite", 2, "", "")
		c.Assert(err, qt.IsNil)
		c.Assert(len(rows) <= 2, qt.IsTrue)
	})
}

// ---------------------------------------------------------------------------
// DeleteByFilter
// ---------------------------------------------------------------------------

func TestDeleteByFilter_HappyPath(t *testing.T) {
	c := qt.New(t)

	old := time.Now().UTC().Add(-48 * time.Hour)
	recent := time.Now().UTC()
	future := time.Now().UTC().Add(24 * time.Hour)

	c.Run("deletes memories older than cutoff", func(c *qt.C) {
		d := openTestDB(t)
		_, _ = d.InsertMemory(newMemAt("old-1", "Old", "proj", old), "")
		_, _ = d.InsertMemory(newMemAt("new-1", "New", "proj", recent), "")

		count, err := d.DeleteByFilter("", "", future)
		c.Assert(err, qt.IsNil)
		c.Assert(count, qt.Equals, 2)

		n, err := d.CountMemories("", "")
		c.Assert(err, qt.IsNil)
		c.Assert(n, qt.Equals, 0)
	})

	c.Run("project filter deletes only matching project", func(c *qt.C) {
		d := openTestDB(t)
		_, _ = d.InsertMemory(newMemAt("a1", "T", "proj-a", old), "")
		_, _ = d.InsertMemory(newMemAt("b1", "T", "proj-b", old), "")

		count, err := d.DeleteByFilter("proj-a", "", future)
		c.Assert(err, qt.IsNil)
		c.Assert(count, qt.Equals, 1)

		n, err := d.CountMemories("", "")
		c.Assert(err, qt.IsNil)
		c.Assert(n, qt.Equals, 1)
	})

	c.Run("category filter deletes only matching category", func(c *qt.C) {
		d := openTestDB(t)
		mDecision := newMemAt("cat-dec", "T", "proj", old)
		mDecision.Category = "decision"
		mPattern := newMemAt("cat-pat", "T", "proj", old)
		mPattern.Category = "pattern"
		_, _ = d.InsertMemory(mDecision, "")
		_, _ = d.InsertMemory(mPattern, "")

		count, err := d.DeleteByFilter("", "decision", future)
		c.Assert(err, qt.IsNil)
		c.Assert(count, qt.Equals, 1)

		n, err := d.CountMemories("", "")
		c.Assert(err, qt.IsNil)
		c.Assert(n, qt.Equals, 1)
	})

	c.Run("newer memories are not deleted", func(c *qt.C) {
		d := openTestDB(t)
		_, _ = d.InsertMemory(newMemAt("keep-1", "Keep", "proj", recent), "")

		cutoff := time.Now().UTC().Add(-1 * time.Hour)
		count, err := d.DeleteByFilter("", "", cutoff)
		c.Assert(err, qt.IsNil)
		c.Assert(count, qt.Equals, 0)

		n, err := d.CountMemories("", "")
		c.Assert(err, qt.IsNil)
		c.Assert(n, qt.Equals, 1)
	})

	c.Run("also deletes associated memory_details", func(c *qt.C) {
		d := openTestDB(t)
		_, _ = d.InsertMemory(newMemAt("det-1", "T", "proj", old), "the details body")

		_, err := d.DeleteByFilter("", "", future)
		c.Assert(err, qt.IsNil)

		detail, err := d.GetDetails("det-1")
		c.Assert(err, qt.IsNil)
		c.Assert(detail, qt.IsNil)
	})
}

func TestDeleteByFilter_FailurePath(t *testing.T) {
	c := qt.New(t)

	c.Run("no matching records returns zero without error", func(c *qt.C) {
		d := openTestDB(t)
		cutoff := time.Now().UTC().Add(-30 * 24 * time.Hour)
		count, err := d.DeleteByFilter("", "", cutoff)
		c.Assert(err, qt.IsNil)
		c.Assert(count, qt.Equals, 0)
	})
}

// ---------------------------------------------------------------------------
// ReplaceMemory
// ---------------------------------------------------------------------------

func TestReplaceMemory_HappyPath(t *testing.T) {
	c := qt.New(t)

	c.Run("replaces all mutable fields", func(c *qt.C) {
		d := openTestDB(t)
		_, _ = d.InsertMemory(newMem("rep-1", "Original Title", "proj"), "original details")

		ok, err := d.ReplaceMemory("rep-1", "New Title", "new what", "new why", "new impact",
			[]string{"tag1"}, []string{"file.go"}, "decision", "new details")
		c.Assert(err, qt.IsNil)
		c.Assert(ok, qt.IsTrue)

		got, found, err := d.GetMemory("rep-1")
		c.Assert(err, qt.IsNil)
		c.Assert(found, qt.IsTrue)
		c.Assert(got["title"], qt.Equals, "New Title")
		c.Assert(got["what"], qt.Equals, "new what")
		c.Assert(got["why"], qt.Equals, "new why")
		c.Assert(got["category"], qt.Equals, "decision")
	})

	c.Run("replaces details body entirely", func(c *qt.C) {
		d := openTestDB(t)
		_, _ = d.InsertMemory(newMem("rep-2", "T", "p"), "old details")

		_, err := d.ReplaceMemory("rep-2", "T", "w", "", "",
			make([]string, 0), make([]string, 0), "context", "completely new details")
		c.Assert(err, qt.IsNil)

		detail, err := d.GetDetails("rep-2")
		c.Assert(err, qt.IsNil)
		c.Assert(detail, qt.IsNotNil)
		c.Assert(detail.Body, qt.Equals, "completely new details")
	})

	c.Run("empty details removes existing details", func(c *qt.C) {
		d := openTestDB(t)
		_, _ = d.InsertMemory(newMem("rep-3", "T", "p"), "some details")

		_, err := d.ReplaceMemory("rep-3", "T", "w", "", "",
			make([]string, 0), make([]string, 0), "context", "")
		c.Assert(err, qt.IsNil)

		detail, err := d.GetDetails("rep-3")
		c.Assert(err, qt.IsNil)
		c.Assert(detail, qt.IsNil)
	})

	c.Run("prefix match works", func(c *qt.C) {
		d := openTestDB(t)
		_, _ = d.InsertMemory(newMem("prefix-xyz-789", "T", "p"), "")

		ok, err := d.ReplaceMemory("prefix-xyz", "Updated", "w", "", "",
			make([]string, 0), make([]string, 0), "pattern", "")
		c.Assert(err, qt.IsNil)
		c.Assert(ok, qt.IsTrue)

		got, found, err := d.GetMemory("prefix-xyz-789")
		c.Assert(err, qt.IsNil)
		c.Assert(found, qt.IsTrue)
		c.Assert(got["title"], qt.Equals, "Updated")
	})
}

func TestReplaceMemory_FailurePath(t *testing.T) {
	c := qt.New(t)

	c.Run("non-existent ID returns false without error", func(c *qt.C) {
		d := openTestDB(t)
		ok, err := d.ReplaceMemory("ghost", "T", "w", "", "",
			make([]string, 0), make([]string, 0), "context", "")
		c.Assert(err, qt.IsNil)
		c.Assert(ok, qt.IsFalse)
	})
}
