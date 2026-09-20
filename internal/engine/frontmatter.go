// Package engine implements the wiki operations and structures.
package engine

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// FrontMatter holds metadata parsed from markdown YAML front matter.
type FrontMatter struct {
	Status       string      `json:"status"`
	SupersededBy string      `json:"superseded_by"`
	Description  string      `json:"description"`
	Created      string      `json:"created,omitempty"`
	Updated      string      `json:"updated,omitempty"`
	Tags         []string    `json:"tags,omitempty"`
	References   []Reference `json:"references,omitempty"`
}

// Reference is a declared cross-reference from a wiki page to something
// outside the wiki, written as "type:value" in the front matter
// `references` list. Supported types: source (repo file path),
// external (http/https URL), issue (tracker key like JIRA-42).
type Reference struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

// DefaultFrontMatter returns the fallback front matter if none is present.
func DefaultFrontMatter() FrontMatter {
	return FrontMatter{
		Status: "current",
	}
}

// ParseFrontMatter parses YAML front matter from the start of a markdown file.
// It returns the parsed FrontMatter, a boolean indicating if front matter block was found,
// and an error if the front matter was malformed (e.g. unterminated).
func ParseFrontMatter(content string) (FrontMatter, bool, error) {
	scanner := bufio.NewScanner(strings.NewReader(content))
	var lines []string
	inFrontMatter := false
	started := false
	found := false

	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		if !started {
			// Skip empty lines before front matter starts
			if trimmed == "" {
				continue
			}
			if trimmed == "---" {
				inFrontMatter = true
				started = true
				found = true
				continue
			} else {
				// No front matter at the start of the file
				break
			}
		}

		if inFrontMatter {
			if trimmed == "---" {
				inFrontMatter = false
				break
			}
			lines = append(lines, line)
		}
	}

	if started && inFrontMatter {
		return DefaultFrontMatter(), true, fmt.Errorf("unterminated front matter block")
	}

	if !found {
		return DefaultFrontMatter(), false, nil
	}

	fm := DefaultFrontMatter()
	for _, line := range lines {
		// Ignore comments or empty lines inside front matter
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		parts := strings.SplitN(line, ":", 2)
		if len(parts) < 2 {
			continue // ignore malformed line or lines without key/value
		}

		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])

		// Strip inline comments: YAML treats # as a comment only outside
		// quotes and only after whitespace (or at the start of the value).
		// Values like `description: "C# guide"` must survive intact.
		if !strings.HasPrefix(val, `"`) && !strings.HasPrefix(val, `'`) {
			if idx := strings.Index(val, "#"); idx != -1 && (idx == 0 || val[idx-1] == ' ' || val[idx-1] == '\t') {
				val = strings.TrimSpace(val[:idx])
			}
		}

		// Strip quotes if present
		val = trimQuotes(val)

		switch key {
		case "status":
			fm.Status = val
		case "superseded_by":
			fm.SupersededBy = val
		case "description":
			fm.Description = val
		case "created":
			fm.Created = val
		case "updated":
			fm.Updated = val
		case "tags":
			fm.Tags = parseTags(val)
		case "references":
			fm.References = parseReferences(val)
		}
	}

	return fm, true, nil
}

func trimQuotes(s string) string {
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
}

func parseTags(s string) []string {
	s = strings.TrimPrefix(s, "[")
	s = strings.TrimSuffix(s, "]")
	if s == "" {
		return nil
	}
	raw := strings.Split(s, ",")
	var tags []string
	for _, t := range raw {
		t = strings.TrimSpace(t)
		t = trimQuotes(t)
		if t != "" {
			tags = append(tags, t)
		}
	}
	return tags
}

// parseReferences parses an inline `references: [...]` list into typed
// Reference entries. Each item is "type:value" — split on the FIRST colon
// so URLs inside values survive. Items without a prefix keep type "unknown"
// (the references lint checker flags them).
func parseReferences(s string) []Reference {
	s = strings.TrimPrefix(s, "[")
	s = strings.TrimSuffix(s, "]")
	if s == "" {
		return nil
	}
	raw := strings.Split(s, ",")
	var refs []Reference
	for _, r := range raw {
		r = strings.TrimSpace(r)
		r = trimQuotes(r)
		if r == "" {
			continue
		}
		typ, val := "unknown", r
		if idx := strings.Index(r, ":"); idx != -1 {
			prefix := strings.TrimSpace(r[:idx])
			switch prefix {
			case "source", "external", "issue":
				typ = prefix
				val = strings.TrimSpace(r[idx+1:])
			}
		}
		if val == "" {
			continue
		}
		refs = append(refs, Reference{Type: typ, Value: val})
	}
	return refs
}

// PageFrontMatter loads and parses front matter for a page path relative to repo root.
func (e *Engine) PageFrontMatter(relPath string) (FrontMatter, bool) {
	abs := filepath.Join(e.RootDir, relPath)
	data, err := os.ReadFile(abs)
	if err != nil {
		return DefaultFrontMatter(), false
	}
	fm, found, _ := ParseFrontMatter(string(data))
	return fm, found
}
