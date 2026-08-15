package engine

import (
	"os"
	"path/filepath"
)

// canonicalPathCandidates maps a logical wiki file name to its accepted
// paths relative to the wiki directory, preferred first. The first entry is
// the organized layout (prologue/); remaining entries are legacy flat-layout
// paths kept for backward compatibility with wikis scaffolded before the
// organized structure was introduced. index.md and README.md always live at
// the wiki root.
var canonicalPathCandidates = map[string][]string{
	"log.md":      {"prologue/log.md", "log.md"},
	"phases.md":   {"prologue/phases.md", "phases.md"},
	"schema.md":   {"prologue/schema.md", "schema.md"},
	"repo-map.md": {"prologue/repo-map.md", "repo-map.md"},
	"index.md":    {"index.md"},
	"README.md":   {"README.md"},
}

// resolveWikiFile returns the wiki-relative, slash-separated path of the
// first existing candidate for the named logical file, preferring the
// organized layout. If none exists, it returns the preferred candidate so
// callers fail or report against a deterministic path.
func (e *Engine) resolveWikiFile(name string) string {
	candidates, ok := canonicalPathCandidates[name]
	if !ok {
		return name
	}
	for _, c := range candidates {
		if _, err := os.Stat(filepath.Join(e.WikiPath(), filepath.FromSlash(c))); err == nil {
			return c
		}
	}
	return candidates[0]
}

// isCanonicalFile reports whether wikiRel (slash-separated, relative to the
// wiki directory) is one of the accepted paths for the named logical file.
func isCanonicalFile(name, wikiRel string) bool {
	for _, c := range canonicalPathCandidates[name] {
		if wikiRel == c {
			return true
		}
	}
	return false
}
