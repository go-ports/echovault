// Package markdown writes Obsidian-compatible session markdown files.
package markdown

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/go-ports/echovault/internal/models"
)

// RenderSection produces a single ### heading block for a memory.
func RenderSection(mem *models.Memory, details string) string {
	var sb strings.Builder
	sb.WriteString("### ")
	sb.WriteString(mem.Title)
	sb.WriteString("\n**What:** ")
	sb.WriteString(mem.What)
	if mem.Why != "" {
		sb.WriteString("\n**Why:** ")
		sb.WriteString(mem.Why)
	}
	if mem.Impact != "" {
		sb.WriteString("\n**Impact:** ")
		sb.WriteString(mem.Impact)
	}
	if mem.Source != "" {
		sb.WriteString("\n**Source:** ")
		sb.WriteString(mem.Source)
	}
	if details != "" {
		sb.WriteString("\n\n<details>\n")
		sb.WriteString(details)
		sb.WriteString("\n</details>")
	}
	return sb.String()
}

// WriteSessionMemory creates or appends to a <dateStr>-session.md file inside
// vaultProjectDir. The directory must already exist.
func WriteSessionMemory(vaultProjectDir string, mem *models.Memory, dateStr, details string) error {
	filePath := filepath.Join(vaultProjectDir, dateStr+"-session.md")
	sectionContent := RenderSection(mem, details)

	var content string
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		content = createNewSessionFile(mem, dateStr, sectionContent)
	} else {
		existing, err := os.ReadFile(filePath)
		if err != nil {
			return err
		}
		content = appendToSessionFile(string(existing), mem, sectionContent)
	}

	return os.WriteFile(filePath, []byte(content), 0o644) // #nosec G306 -- session markdown files do not contain secrets
}

// ---------------------------------------------------------------------------
// File creation
// ---------------------------------------------------------------------------

func createNewSessionFile(mem *models.Memory, dateStr, sectionContent string) string {
	now := time.Now().UTC().Format(time.RFC3339)
	tags := sortedUniq(mem.Tags)

	var sb strings.Builder
	sb.WriteString("---\n")
	sb.WriteString("project: ")
	sb.WriteString(mem.Project)
	sb.WriteString("\n")
	if mem.Source != "" {
		sb.WriteString("sources: [")
		sb.WriteString(mem.Source)
		sb.WriteString("]\n")
	} else {
		sb.WriteString("sources: []\n")
	}
	sb.WriteString("created: ")
	sb.WriteString(now)
	sb.WriteString("\n")
	sb.WriteString("tags: [")
	sb.WriteString(strings.Join(tags, ", "))
	sb.WriteString("]\n")
	sb.WriteString("---\n")
	sb.WriteString("\n# ")
	sb.WriteString(dateStr)
	sb.WriteString(" Session\n")

	if mem.Category != "" {
		heading := models.CategoryHeadings[mem.Category]
		sb.WriteString("\n## ")
		sb.WriteString(heading)
		sb.WriteString("\n")
	}

	sb.WriteString("\n")
	sb.WriteString(sectionContent)
	sb.WriteString("\n")
	return sb.String()
}

// ---------------------------------------------------------------------------
// File appending
// ---------------------------------------------------------------------------

func appendToSessionFile(content string, mem *models.Memory, sectionContent string) string {
	frontmatter, body := splitFrontmatter(content)
	updatedFM := updateFrontmatter(frontmatter, mem)
	updatedBody := insertSectionInBody(body, mem, sectionContent)
	return updatedFM + "\n" + updatedBody
}

// splitFrontmatter splits YAML front-matter from the body.
// Returns ("", content) when no front-matter is detected.
func splitFrontmatter(content string) (frontmatter, body string) {
	parts := strings.SplitN(content, "---\n", 3)
	if len(parts) >= 3 {
		return "---\n" + parts[1] + "---", parts[2]
	}
	return "", content
}

// inlineArrayRe extracts the contents of [...] on a YAML line.
var inlineArrayRe = regexp.MustCompile(`\[([^\]]*)\]`)

// updateFrontmatter merges new tags and source into the existing frontmatter.
func updateFrontmatter(frontmatter string, mem *models.Memory) string { //nolint:gocognit // complexity is inherent to the two-pass (collect then rebuild) YAML front-matter merge
	lines := strings.Split(frontmatter, "\n")

	var existingTags, existingSources []string

	for _, line := range lines {
		if strings.HasPrefix(line, "tags:") { //nolint:nestif // per-line YAML front-matter parsing requires inspecting both tags and sources entries within the same loop
			if m := inlineArrayRe.FindStringSubmatch(line); m != nil && strings.TrimSpace(m[1]) != "" {
				for _, t := range strings.Split(m[1], ",") {
					if s := strings.TrimSpace(t); s != "" {
						existingTags = append(existingTags, s)
					}
				}
			}
		} else if strings.HasPrefix(line, "sources:") {
			if m := inlineArrayRe.FindStringSubmatch(line); m != nil && strings.TrimSpace(m[1]) != "" {
				for _, s := range strings.Split(m[1], ",") {
					if s := strings.TrimSpace(s); s != "" {
						existingSources = append(existingSources, s)
					}
				}
			}
		}
	}

	// Merge tags (sorted, dedup).
	allTags := sortedUniq(append(existingTags, mem.Tags...))

	// Merge source.
	allSources := existingSources
	if mem.Source != "" && !contains(allSources, mem.Source) {
		allSources = append(allSources, mem.Source)
	}

	// Rebuild lines.
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		switch {
		case strings.HasPrefix(line, "tags:"):
			out = append(out, "tags: ["+strings.Join(allTags, ", ")+"]")
		case strings.HasPrefix(line, "sources:"):
			out = append(out, "sources: ["+strings.Join(allSources, ", ")+"]")
		default:
			out = append(out, line)
		}
	}
	return strings.Join(out, "\n")
}

// ---------------------------------------------------------------------------
// Body insertion
// ---------------------------------------------------------------------------

func insertSectionInBody(body string, mem *models.Memory, sectionContent string) string {
	if mem.Category == "" {
		return strings.TrimRight(body, "\n") + "\n\n" + sectionContent + "\n"
	}

	heading := models.CategoryHeadings[mem.Category]
	h2marker := "## " + heading

	if strings.Contains(body, h2marker) {
		return appendUnderExistingCategory(body, heading, sectionContent)
	}
	return insertNewCategory(body, mem.Category, heading, sectionContent)
}

// appendUnderExistingCategory appends sectionContent after the last H3 under
// the matching H2 heading.
func appendUnderExistingCategory(body, categoryHeading, sectionContent string) string {
	target := "## " + categoryHeading
	lines := strings.Split(body, "\n")
	result := make([]string, 0, len(lines)+4)
	i := 0

	for i < len(lines) {
		line := lines[i]
		result = append(result, line)

		if line == target {
			i++
			// Copy trailing blank lines after heading.
			for i < len(lines) && strings.TrimSpace(lines[i]) == "" {
				result = append(result, lines[i])
				i++
			}
			// Copy all content until the next H2 (or EOF).
			for i < len(lines) && !strings.HasPrefix(lines[i], "## ") {
				result = append(result, lines[i])
				i++
			}
			// Append new section.
			result = append(result, "", sectionContent)
			continue
		}
		i++
	}

	return strings.Join(result, "\n") + "\n"
}

// insertNewCategory inserts a new ## heading block in ValidCategories order.
func insertNewCategory(body, category, categoryHeading, sectionContent string) string {
	targetIdx := categoryIndex(category)
	lines := strings.Split(body, "\n")
	insertPos := len(lines)

	for i, line := range lines {
		if strings.HasPrefix(line, "## ") {
			heading := strings.TrimPrefix(line, "## ")
			for _, cat := range models.ValidCategories {
				if models.CategoryHeadings[cat] == heading {
					if categoryIndex(cat) > targetIdx {
						insertPos = i
					}
					break
				}
			}
			if insertPos < len(lines) {
				break
			}
		}
	}

	newBlock := []string{"## " + categoryHeading, "", sectionContent, ""}
	merged := append(append(lines[:insertPos:insertPos], newBlock...), lines[insertPos:]...)
	return strings.TrimRight(strings.Join(merged, "\n"), "\n") + "\n"
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func categoryIndex(cat string) int {
	for i, c := range models.ValidCategories {
		if c == cat {
			return i
		}
	}
	return len(models.ValidCategories)
}

func sortedUniq(ss []string) []string {
	seen := make(map[string]bool, len(ss))
	out := make([]string, 0, len(ss))
	for _, s := range ss {
		if s != "" && !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	sort.Strings(out)
	return out
}

func contains(ss []string, s string) bool {
	for _, v := range ss {
		if v == s {
			return true
		}
	}
	return false
}
