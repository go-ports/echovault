// Package models defines the core data types for the memory system.
package models

import (
	"crypto/rand"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// ValidCategories lists the accepted memory category values.
var ValidCategories = []string{"decision", "pattern", "bug", "context", "learning"}

// CategoryHeadings maps category keys to Markdown heading text.
var CategoryHeadings = map[string]string{
	"decision": "Decisions",
	"pattern":  "Patterns",
	"bug":      "Bugs Fixed",
	"context":  "Context",
	"learning": "Learnings",
}

// RawMemoryInput is the caller-supplied data before redaction and ID generation.
type RawMemoryInput struct {
	Title        string
	What         string
	Why          string // optional
	Impact       string // optional
	Tags         []string
	Category     string // optional; one of ValidCategories
	RelatedFiles []string
	Details      string // optional; extended body
	Source       string // optional; agent name e.g. "claude-code"
}

// Memory is a fully processed memory record.
type Memory struct {
	ID            string
	Title         string
	What          string
	Why           string
	Impact        string
	Tags          []string
	Category      string
	Project       string
	Source        string
	RelatedFiles  []string
	FilePath      string
	SectionAnchor string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// FromRaw constructs a Memory from a RawMemoryInput, assigning a new UUID,
// generating the section anchor, and stamping creation/update times.
func FromRaw(raw *RawMemoryInput, project, filePath string) *Memory {
	now := time.Now().UTC()
	return &Memory{
		ID:            newUUID(),
		Title:         raw.Title,
		What:          raw.What,
		Why:           raw.Why,
		Impact:        raw.Impact,
		Tags:          raw.Tags,
		Category:      raw.Category,
		Project:       project,
		Source:        raw.Source,
		RelatedFiles:  raw.RelatedFiles,
		FilePath:      filePath,
		SectionAnchor: sectionAnchor(raw.Title),
		CreatedAt:     now,
		UpdatedAt:     now,
	}
}

// MemoryDetail holds the extended body text for a memory.
type MemoryDetail struct {
	MemoryID string
	Body     string
}

// SearchResult is a single hit returned from hybrid search.
type SearchResult struct {
	ID         string
	Title      string
	What       string
	Why        string
	Impact     string
	Category   string
	Tags       []string
	Project    string
	Source     string
	Score      float64
	HasDetails bool
	FilePath   string
	CreatedAt  string
}

// SaveResult is returned from Service.Save.
type SaveResult struct {
	ID       string
	FilePath string
	Action   string // "created" or "updated"
	Warnings []string
}

// ReindexResult is returned from Service.Reindex.
type ReindexResult struct {
	Count int
	Dim   int
	Model string
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

var nonAlnum = regexp.MustCompile(`[^a-z0-9]+`)

// sectionAnchor converts a title to a lowercase hyphenated anchor.
func sectionAnchor(title string) string {
	s := strings.ToLower(title)
	s = nonAlnum.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	return s
}

// newUUID generates a random UUID v4 using crypto/rand without external deps.
func newUUID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic("echovault: crypto/rand unavailable: " + err.Error())
	}
	// Set version 4 and variant bits (RFC 4122).
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf(
		"%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:],
	)
}
