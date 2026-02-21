// Package redaction implements multi-layer secret redaction for memory text fields.
package redaction

import (
	"bufio"
	"os"
	"regexp"
	"strings"
)

// sensitivePatterns are compiled once at package init and applied in layer 2.
var sensitivePatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)sk_live_[a-zA-Z0-9]+`),             // Stripe live keys
	regexp.MustCompile(`(?i)sk_test_[a-zA-Z0-9]+`),             // Stripe test keys
	regexp.MustCompile(`ghp_[a-zA-Z0-9]+`),                     // GitHub PATs
	regexp.MustCompile(`AKIA[0-9A-Z]{16}`),                     // AWS access key IDs
	regexp.MustCompile(`xoxb-[a-zA-Z0-9-]+`),                   // Slack bot tokens
	regexp.MustCompile(`-----BEGIN (?:RSA )?PRIVATE KEY-----`), // Private keys
	regexp.MustCompile(`eyJ[a-zA-Z0-9_-]+\.eyJ[a-zA-Z0-9_-]+`), // JWT tokens
	regexp.MustCompile(`(?i)password\s*[:=]\s*["']?.+`),        // password = ...
	regexp.MustCompile(`(?i)secret\s*[:=]\s*["']?.+`),          // secret = ...
	regexp.MustCompile(`(?i)api[_-]?key\s*[:=]\s*["']?.+`),     // api_key = ...
}

// redactedTagRe matches explicit <redacted>…</redacted> pairs (including multiline).
var redactedTagRe = regexp.MustCompile(`(?s)<redacted>.*?</redacted>`)

const replacement = "[REDACTED]"

// Redact applies a three-layer pipeline to text:
//
//  1. Explicit <redacted>…</redacted> tags — replaced with [REDACTED] until
//     no pairs remain; orphaned opening/closing tags are then stripped.
//  2. Built-in sensitive patterns (API keys, tokens, passwords, …).
//  3. Caller-supplied extraPatterns (e.g. from LoadMemoryIgnore).
func Redact(text string, extraPatterns []*regexp.Regexp) string {
	// Layer 1: explicit tags — loop until stable.
	for {
		next := redactedTagRe.ReplaceAllString(text, replacement)
		if next == text {
			break
		}
		text = next
	}
	// Strip any remaining orphaned tags.
	text = strings.ReplaceAll(text, "<redacted>", "")
	text = strings.ReplaceAll(text, "</redacted>", "")

	// Layer 2: built-in patterns.
	for _, re := range sensitivePatterns {
		text = re.ReplaceAllString(text, replacement)
	}

	// Layer 3: caller-supplied patterns.
	for _, re := range extraPatterns {
		text = re.ReplaceAllString(text, replacement)
	}

	return text
}

// LoadMemoryIgnore reads a .memoryignore file and compiles each non-blank,
// non-comment line as a regular expression.
// Returns nil (no error) if the file does not exist.
func LoadMemoryIgnore(path string) ([]*regexp.Regexp, error) {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var patterns []*regexp.Regexp
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		re, err := regexp.Compile(line)
		if err != nil {
			return nil, err
		}
		patterns = append(patterns, re)
	}
	return patterns, scanner.Err()
}
