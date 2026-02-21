package checkers_test

import (
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/go-ports/echovault/internal/checkers"
)

var nopNoteFn = func(string, any) {}

func TestJSONPathEquals(t *testing.T) {
	t.Run("Test case 1: JSON value matches the expected value at the JSON path expression (string JSON)", func(t *testing.T) {
		c := qt.New(t)
		jsonData := `{"name": "John", "age": 30}`
		result := checkers.JSONPathEquals("$.name").Check(jsonData, []any{"John"}, nopNoteFn)
		c.Assert(result, qt.IsNil)
	})

	t.Run("Test case 2: JSON value does not match the expected value at the JSON path expression (string JSON)", func(t *testing.T) {
		c := qt.New(t)
		jsonData := `{"name": "John", "age": 30}`
		result := checkers.JSONPathEquals("$.name").Check(jsonData, []any{"Jane"}, nopNoteFn)
		c.Assert(result, qt.IsNotNil)
	})

	t.Run("Test case 3: JSON value matches the expected value at the JSON path expression ([]any)", func(t *testing.T) {
		c := qt.New(t)
		jsonData := []any{"name", "John", "age", 30}
		result := checkers.JSONPathEquals("$[1]").Check(jsonData, []any{"John"}, nopNoteFn)
		c.Assert(result, qt.IsNil)
	})

	t.Run("Test case 4: JSON value does not match the expected value at the JSON path expression ([]any)", func(t *testing.T) {
		c := qt.New(t)
		jsonData := []any{"name", "John", "age", 30}
		result := checkers.JSONPathEquals("$[1]").Check(jsonData, []any{"Jane"}, nopNoteFn)
		c.Assert(result, qt.IsNotNil)
	})

	t.Run("Test case 5: JSON value matches the expected value at the JSON path expression (map[string]any)", func(t *testing.T) {
		c := qt.New(t)
		jsonData := map[string]any{"name": "John", "age": 30}
		result := checkers.JSONPathEquals("$.name").Check(jsonData, []any{"John"}, nopNoteFn)
		c.Assert(result, qt.IsNil)
	})

	t.Run("Test case 6: JSON value does not match the expected value at the JSON path expression (map[string]any)", func(t *testing.T) {
		c := qt.New(t)
		jsonData := map[string]any{"name": "John", "age": 30}
		result := checkers.JSONPathEquals("$.name").Check(jsonData, []any{"Jane"}, nopNoteFn)
		c.Assert(result, qt.IsNotNil)
	})

	t.Run("Test case 7: JSON value matches the expected value at the JSON path expression ([]byte)", func(t *testing.T) {
		c := qt.New(t)
		jsonData := []byte(`{"name": "John", "age": 30}`)
		result := checkers.JSONPathEquals("$.name").Check(jsonData, []any{"John"}, nopNoteFn)
		c.Assert(result, qt.IsNil)
	})

	t.Run("Test case 8: JSON value does not match the expected value at the JSON path expression ([]byte)", func(t *testing.T) {
		c := qt.New(t)
		jsonData := []byte(`{"name": "John", "age": 30}`)
		result := checkers.JSONPathEquals("$.name").Check(jsonData, []any{"Jane"}, nopNoteFn)
		c.Assert(result, qt.IsNotNil)
	})

	t.Run("Test case 9: JSON value matches the expected value at the JSON path expression (different input type)", func(t *testing.T) {
		c := qt.New(t)
		jsonData := 42 // Example of a different input type (int)
		result := checkers.JSONPathEquals("$.name").Check(jsonData, []any{42}, nopNoteFn)
		c.Assert(result, qt.IsNotNil)
	})
}

func TestJSONPathMatches(t *testing.T) {
	t.Run("Test case 1: JSON value matches the expected value using a custom checker at the JSON path expression", func(t *testing.T) {
		c := qt.New(t)
		jsonData := `{"name": "John", "age": 30}`
		result := checkers.JSONPathMatches("$.age", qt.Satisfies).Check(jsonData, []any{func(v float64) bool {
			return v > 20
		}}, nopNoteFn)
		c.Assert(result, qt.IsNil)
	})

	t.Run("Test case 2: JSON value does not match the expected value using a custom checker at the JSON path expression", func(t *testing.T) {
		c := qt.New(t)
		jsonData := `{"name": "John", "age": 30}`
		result := checkers.JSONPathMatches("$.age", qt.Equals).Check(jsonData, []any{40}, nopNoteFn)
		c.Assert(result, qt.IsNotNil)
	})
}

func TestJSONPathMatchesWithUnmarshalError(t *testing.T) {
	t.Run("Test case: JSON unmarshaling error occurs", func(t *testing.T) {
		c := qt.New(t)
		jsonData := "invalid-json-data"
		result := checkers.JSONPathMatches("$.age", qt.Equals).Check(jsonData, []any{30}, nopNoteFn)
		c.Assert(result, qt.IsNotNil)
	})
}

func TestJSONPathMatchesWithEvaluateError(t *testing.T) {
	t.Run("Test case: JSON path expression evaluation error occurs", func(t *testing.T) {
		c := qt.New(t)
		jsonData := `{"name": "John", "age": 30}`
		result := checkers.JSONPathMatches("$.unknown", qt.Equals).Check(jsonData, []any{"John"}, nopNoteFn)
		c.Assert(result, qt.IsNotNil)
	})
}

func TestJSONPathMatchesWithInvalidValue(t *testing.T) {
	t.Run("Test case: Invalid JSON value is passed", func(t *testing.T) {
		c := qt.New(t)
		jsonData := make(chan int) // Invalid JSON value
		result := checkers.JSONPathMatches("$.age", qt.Equals).Check(jsonData, []any{30}, nopNoteFn)
		c.Assert(result, qt.IsNotNil)
	})
}

func TestJSONPathMatchesWithNilValue(t *testing.T) {
	t.Run("Test case: Nil JSON value is passed", func(t *testing.T) {
		c := qt.New(t)
		var jsonData any // Nil JSON value
		result := checkers.JSONPathMatches("$.age", qt.Equals).Check(jsonData, []any{30}, nopNoteFn)
		c.Assert(result, qt.IsNotNil)
	})
}
