package mcp

// White-box testing required: isValidCategory, truncate, parseTags,
// formatDate, and roundTwo are unexported utility functions used to validate
// incoming tool arguments and format outgoing MCP tool responses. They are
// not reachable through the public NewServer API, so direct access is
// required to achieve meaningful coverage of their edge cases.

import (
	"testing"

	qt "github.com/frankban/quicktest"
)

// ---------------------------------------------------------------------------
// isValidCategory
// ---------------------------------------------------------------------------

func TestIsValidCategory_HappyPath(t *testing.T) {
	c := qt.New(t)

	cases := []struct {
		name string
		in   string
		want bool
	}{
		{"decision", "decision", true},
		{"bug", "bug", true},
		{"pattern", "pattern", true},
		{"learning", "learning", true},
		{"context", "context", true},
		{"empty string", "", false},
		{"unknown value", "unknown", false},
		{"uppercase", "Decision", false},
	}

	for _, tc := range cases {
		c.Run(tc.name, func(c *qt.C) {
			c.Assert(isValidCategory(tc.in), qt.Equals, tc.want)
		})
	}
}

// ---------------------------------------------------------------------------
// truncate
// ---------------------------------------------------------------------------

func TestTruncate_HappyPath(t *testing.T) {
	c := qt.New(t)

	cases := []struct {
		name   string
		s      string
		maxLen int
		want   string
	}{
		{"shorter than limit", "hello", 10, "hello"},
		{"exactly limit", "hello", 5, "hello"},
		{"longer than limit", "hello world", 5, "hello"},
		{"unicode runes truncated correctly", "héllo", 3, "hél"},
		{"empty string", "", 10, ""},
		{"zero limit", "hello", 0, ""},
	}

	for _, tc := range cases {
		c.Run(tc.name, func(c *qt.C) {
			c.Assert(truncate(tc.s, tc.maxLen), qt.Equals, tc.want)
		})
	}
}

// ---------------------------------------------------------------------------
// parseTags
// ---------------------------------------------------------------------------

func TestParseTags_HappyPath(t *testing.T) {
	c := qt.New(t)

	cases := []struct {
		name string
		raw  string
		want []string
	}{
		{"valid JSON array", `["go","test"]`, []string{"go", "test"}},
		{"empty string returns empty slice", "", make([]string, 0)},
		{"null JSON returns empty slice", "null", make([]string, 0)},
		{"invalid JSON returns empty slice", "not-json", make([]string, 0)},
		{"empty JSON array", "[]", make([]string, 0)},
	}

	for _, tc := range cases {
		c.Run(tc.name, func(c *qt.C) {
			got := parseTags(tc.raw)
			c.Assert(got, qt.DeepEquals, tc.want)
		})
	}
}

// ---------------------------------------------------------------------------
// formatDate
// ---------------------------------------------------------------------------

func TestFormatDate_HappyPath(t *testing.T) {
	c := qt.New(t)

	cases := []struct {
		name    string
		dateStr string
		want    string
	}{
		{"valid ISO date", "2024-01-15", "Jan 15"},
		{"datetime string uses first 10 chars", "2024-03-07T12:00:00Z", "Mar 07"},
		{"short string returned as-is", "2024", "2024"},
		{"invalid date returned as-is", "not-a-date", "not-a-date"},
		{"empty string returned as-is", "", ""},
	}

	for _, tc := range cases {
		c.Run(tc.name, func(c *qt.C) {
			c.Assert(formatDate(tc.dateStr), qt.Equals, tc.want)
		})
	}
}

// ---------------------------------------------------------------------------
// roundTwo
// ---------------------------------------------------------------------------

func TestRoundTwo_HappyPath(t *testing.T) {
	c := qt.New(t)

	cases := []struct {
		name string
		in   float64
		want float64
	}{
		{"exact value unchanged", 1.25, 1.25},
		{"rounds down", 1.234, 1.23},
		{"rounds up", 1.235, 1.24},
		{"zero", 0.0, 0.0},
		{"whole number", 3.0, 3.0},
		{"negative value", -1.235, -1.24},
	}

	for _, tc := range cases {
		c.Run(tc.name, func(c *qt.C) {
			c.Assert(roundTwo(tc.in), qt.Equals, tc.want)
		})
	}
}
