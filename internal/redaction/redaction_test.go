package redaction_test

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/go-ports/echovault/internal/redaction"
)

func TestRedact_PlainText(t *testing.T) {
	c := qt.New(t)
	got := redaction.Redact("hello world", nil)
	c.Assert(got, qt.Equals, "hello world")
}

func TestRedact_ExplicitTags_HappyPath(t *testing.T) {
	c := qt.New(t)

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "single tag pair replaced",
			input: "before <redacted>sensitive</redacted> after",
			want:  "before [REDACTED] after",
		},
		{
			name:  "multiple tag pairs replaced",
			input: "<redacted>a</redacted> and <redacted>b</redacted>",
			want:  "[REDACTED] and [REDACTED]",
		},
		{
			name:  "multiline content replaced",
			input: "start <redacted>line1\nline2</redacted> end",
			want:  "start [REDACTED] end",
		},
		{
			name:  "orphaned opening tag stripped",
			input: "before <redacted> after",
			want:  "before  after",
		},
		{
			name:  "orphaned closing tag stripped",
			input: "before </redacted> after",
			want:  "before  after",
		},
		{
			name:  "no tags leaves text unchanged",
			input: "nothing sensitive here",
			want:  "nothing sensitive here",
		},
	}

	for _, tt := range tests {
		c.Run(tt.name, func(c *qt.C) {
			got := redaction.Redact(tt.input, nil)
			c.Assert(got, qt.Equals, tt.want)
		})
	}
}

func TestRedact_BuiltinPatterns_HappyPath(t *testing.T) {
	c := qt.New(t)

	tests := []struct {
		name  string
		input string
	}{
		{name: "stripe live key", input: "key=sk_live_abcdef1234567890"},
		{name: "stripe test key", input: "key=sk_test_abcdef1234567890"},
		{name: "github PAT", input: "token=ghp_abcdefghijklmnopqrst12345"},
		{name: "aws access key ID", input: "access=AKIAIOSFODNN7EXAMPLE"}, // #nosec G101 -- test data, not real credentials
		{name: "slack bot token", input: "token=xoxb-some-slack-token"},
		{name: "rsa private key header", input: "-----BEGIN RSA PRIVATE KEY-----"}, // #nosec G101 -- test data, not real credentials
		{name: "generic private key header", input: "-----BEGIN PRIVATE KEY-----"},
		{name: "password assignment", input: "password=mysecret123"},
		{name: "secret assignment", input: "secret=topsecret"},
		{name: "api_key assignment", input: "api_key=abcdef"},
		{name: "api-key assignment", input: "api-key=abcdef"},
		{name: "jwt token", input: "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJ1c2VyMTIzIn0"},
	}

	for _, tt := range tests {
		c.Run(tt.name, func(c *qt.C) {
			got := redaction.Redact(tt.input, nil)
			c.Assert(got, qt.Contains, "[REDACTED]")
		})
	}
}

func TestRedact_ExtraPatterns_HappyPath(t *testing.T) {
	c := qt.New(t)

	extra := []*regexp.Regexp{
		regexp.MustCompile(`mycompany-[a-z0-9]+`),
	}

	c.Run("custom pattern matches and redacts", func(c *qt.C) {
		got := redaction.Redact("token=mycompany-abc123", extra)
		c.Assert(got, qt.Contains, "[REDACTED]")
	})

	c.Run("unrelated text is not redacted", func(c *qt.C) {
		got := redaction.Redact("hello world", extra)
		c.Assert(got, qt.Equals, "hello world")
	})

	c.Run("nil extra patterns does not alter plain text", func(c *qt.C) {
		got := redaction.Redact("plain text", nil)
		c.Assert(got, qt.Equals, "plain text")
	})
}

func TestLoadMemoryIgnore_HappyPath(t *testing.T) {
	c := qt.New(t)

	c.Run("non-existent file returns nil patterns and no error", func(c *qt.C) {
		patterns, err := redaction.LoadMemoryIgnore("/nonexistent/.memoryignore")
		c.Assert(err, qt.IsNil)
		c.Assert(patterns, qt.IsNil)
	})

	c.Run("valid patterns file returns compiled regexps", func(c *qt.C) {
		tmp := t.TempDir()
		path := filepath.Join(tmp, ".memoryignore")
		err := os.WriteFile(path, []byte("foo-[0-9]+\nbar[a-z]+\n"), 0o600)
		c.Assert(err, qt.IsNil)

		patterns, err := redaction.LoadMemoryIgnore(path)
		c.Assert(err, qt.IsNil)
		c.Assert(patterns, qt.HasLen, 2)
	})

	c.Run("blank lines and comments are skipped", func(c *qt.C) {
		tmp := t.TempDir()
		path := filepath.Join(tmp, ".memoryignore")
		err := os.WriteFile(path, []byte("\n# comment\n\n# another comment\n"), 0o600)
		c.Assert(err, qt.IsNil)

		patterns, err := redaction.LoadMemoryIgnore(path)
		c.Assert(err, qt.IsNil)
		c.Assert(patterns, qt.IsNil)
	})

	c.Run("loaded patterns apply in Redact", func(c *qt.C) {
		tmp := t.TempDir()
		path := filepath.Join(tmp, ".memoryignore")
		err := os.WriteFile(path, []byte("internal-[0-9a-f]+\n"), 0o600)
		c.Assert(err, qt.IsNil)

		patterns, err := redaction.LoadMemoryIgnore(path)
		c.Assert(err, qt.IsNil)

		got := redaction.Redact("ref=internal-cafebabe", patterns)
		c.Assert(got, qt.Contains, "[REDACTED]")
	})
}

func TestLoadMemoryIgnore_FailurePath(t *testing.T) {
	c := qt.New(t)

	c.Run("invalid regexp returns error and nil patterns", func(c *qt.C) {
		tmp := t.TempDir()
		path := filepath.Join(tmp, ".memoryignore")
		err := os.WriteFile(path, []byte("valid-[0-9]+\n[invalid\n"), 0o600)
		c.Assert(err, qt.IsNil)

		patterns, err := redaction.LoadMemoryIgnore(path)
		c.Assert(err, qt.IsNotNil)
		c.Assert(patterns, qt.IsNil)
	})
}
