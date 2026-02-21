package search

// White-box testing required: the score-normalization and type-coercion helpers
// (normalizeRows, asString, asFloat, asBool, clamp) are unexported and drive
// the correctness of merged search results. Their behaviour cannot be observed
// through the public MergeResults API because that API returns only the final
// ranked list, hiding intermediate score values and conversion details.

import (
	"testing"

	qt "github.com/frankban/quicktest"
)

// ---------------------------------------------------------------------------
// normalizeRows
// ---------------------------------------------------------------------------

func TestNormalizeRows_HappyPath(t *testing.T) {
	c := qt.New(t)

	c.Run("empty slice is a no-op", func(c *qt.C) {
		rows := make([]map[string]any, 0)
		normalizeRows(rows) // must not panic
		c.Assert(rows, qt.HasLen, 0)
	})

	c.Run("single row becomes score 1.0", func(c *qt.C) {
		rows := []map[string]any{{"score": float64(5.0)}}
		normalizeRows(rows)
		c.Assert(rows[0]["score"], qt.Equals, float64(1.0))
	})

	c.Run("multiple rows divided by max", func(c *qt.C) {
		rows := []map[string]any{{"score": float64(10.0)}, {"score": float64(5.0)}}
		normalizeRows(rows)
		c.Assert(rows[0]["score"], qt.Equals, float64(1.0))
		c.Assert(rows[1]["score"], qt.Equals, float64(0.5))
	})

	c.Run("all-zero scores stay zero (maxScore becomes 1.0)", func(c *qt.C) {
		rows := []map[string]any{{"score": float64(0)}, {"score": float64(0)}}
		normalizeRows(rows)
		c.Assert(rows[0]["score"], qt.Equals, float64(0))
		c.Assert(rows[1]["score"], qt.Equals, float64(0))
	})
}

// ---------------------------------------------------------------------------
// asString
// ---------------------------------------------------------------------------

func TestAsString_HappyPath(t *testing.T) {
	c := qt.New(t)

	cases := []struct {
		name string
		in   any
		want string
	}{
		{"nil", nil, ""},
		{"string", "hello", "hello"},
		{"non-string int", 42, ""},
		{"non-string bool", true, ""},
	}

	for _, tc := range cases {
		c.Run(tc.name, func(c *qt.C) {
			c.Assert(asString(tc.in), qt.Equals, tc.want)
		})
	}
}

// ---------------------------------------------------------------------------
// asFloat
// ---------------------------------------------------------------------------

func TestAsFloat_HappyPath(t *testing.T) {
	c := qt.New(t)

	cases := []struct {
		name string
		in   any
		want float64
	}{
		{"nil", nil, 0},
		{"float64", float64(1.5), 1.5},
		{"float32", float32(2.0), 2.0},
		{"int64", int64(3), 3.0},
		{"int", int(4), 4.0},
		{"unsupported bool", true, 0},
	}

	for _, tc := range cases {
		c.Run(tc.name, func(c *qt.C) {
			c.Assert(asFloat(tc.in), qt.Equals, tc.want)
		})
	}
}

// ---------------------------------------------------------------------------
// asBool
// ---------------------------------------------------------------------------

func TestAsBool_HappyPath(t *testing.T) {
	c := qt.New(t)

	cases := []struct {
		name string
		in   any
		want bool
	}{
		{"nil", nil, false},
		{"bool true", true, true},
		{"bool false", false, false},
		{"int64 nonzero", int64(1), true},
		{"int64 zero", int64(0), false},
		{"int nonzero", int(2), true},
		{"int zero", int(0), false},
		{"unsupported string", "true", false},
	}

	for _, tc := range cases {
		c.Run(tc.name, func(c *qt.C) {
			c.Assert(asBool(tc.in), qt.Equals, tc.want)
		})
	}
}

// ---------------------------------------------------------------------------
// clamp
// ---------------------------------------------------------------------------

func TestClamp_HappyPath(t *testing.T) {
	c := qt.New(t)

	cases := []struct {
		name  string
		limit int
		n     int
		want  int
	}{
		{"limit less than n", 3, 5, 3},
		{"limit equal to n", 5, 5, 5},
		{"limit greater than n", 7, 5, 5},
		{"limit zero", 0, 5, 0},
	}

	for _, tc := range cases {
		c.Run(tc.name, func(c *qt.C) {
			c.Assert(clamp(tc.limit, tc.n), qt.Equals, tc.want)
		})
	}
}
