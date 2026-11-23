package model

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/ramayac/go-wiki-engine/internal/paths"
)

// CanonicalPathCandidates maps a logical wiki file name to its accepted paths
// relative to the wiki directory, preferred first.
var CanonicalPathCandidates = map[string][]string{
	"log.md":      {"prologue/log.md", "log.md"},
	"phases.md":   {"prologue/phases.md", "phases.md"},
	"schema.md":   {"prologue/schema.md", "schema.md"},
	"repo-map.md": {"prologue/repo-map.md", "repo-map.md"},
	"index.md":    {"index.md"},
	"README.md":   {"README.md"},
}

// IsCanonicalFile reports whether wikiRel (slash-separated, relative to wikiDir)
// matches one of the accepted paths for the named logical file.
func IsCanonicalFile(name, wikiRel string) bool {
	for _, c := range CanonicalPathCandidates[name] {
		if wikiRel == c {
			return true
		}
	}
	return false
}

// LoadIssue records an operational or containment error encountered while
// building a WikiSnapshot (missing file, permission error, symlink escape, etc.).
type LoadIssue struct {
	Path    string `json:"path"`
	Kind    string `json:"kind"` // "permission", "containment", "io", "missing"
	Message string `json:"message"`
	Err     error  `json:"-"`
}

// WikiSnapshot is a single-read, consistent in-memory view of the entire wiki.
// Every Markdown file is read and parsed exactly once per snapshot.
type WikiSnapshot struct {
	Root       string
	WikiDir    string
	WikiRoot   string
	Pages      []*Page
	AllFiles   []string // repo-relative slash paths of all non-dir files under wiki
	Index      *Page
	Log        *Page
	Phases     *Page
	Tags       map[string]struct{}
	ByID       map[string]*Page
	ByIDMulti  map[string][]*Page
	ByPath     map[string]*Page // keyed by WikiRel (e.g. "prologue/schema.md")
	ByRepoPath map[string]*Page // keyed by Path (e.g. "wiki/prologue/schema.md")
	LoadIssues []LoadIssue
	ReadCounts map[string]int // counts disk reads per repo-relative path

	// Vocabulary is the controlled tag vocabulary from vocabulary.yaml. It is
	// empty (free-form) when the file is missing or lists no tags. VocabularyErr
	// holds the load error for a malformed or out-of-boundary file.
	Vocabulary    Vocabulary
	VocabularyErr error
}

// LoadSnapshot walks the wiki directory once, validates containment for every
// entry, reads every Markdown page exactly once, and builds all lookup maps.
// Operational errors on the wiki root itself return an error immediately;
// per-file read or containment failures are both recorded in LoadIssues and
// returned via SnapshotError when strict propagation is requested.
func LoadSnapshot(repoRoot, wikiDir string) (*WikiSnapshot, error) {
	wikiRoot, err := paths.ResolveWikiRoot(repoRoot, wikiDir)
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(wikiRoot); err != nil {
		return nil, fmt.Errorf("wiki directory not found: %s (%w)", wikiDir, err)
	}

	snap := &WikiSnapshot{
		Root:       repoRoot,
		WikiDir:    wikiDir,
		WikiRoot:   wikiRoot,
		Tags:       make(map[string]struct{}),
		ByID:       make(map[string]*Page),
		ByIDMulti:  make(map[string][]*Page),
		ByPath:     make(map[string]*Page),
		ByRepoPath: make(map[string]*Page),
		ReadCounts: make(map[string]int),
	}

	snap.Vocabulary, snap.VocabularyErr = LoadVocabulary(wikiRoot)

	var mdPaths []string
	walkErr := filepath.WalkDir(wikiRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			rel, rerr := filepath.Rel(repoRoot, path)
			if rerr != nil {
				rel = path
			}
			kind := "io"
			if os.IsPermission(err) {
				kind = "permission"
			}
			snap.LoadIssues = append(snap.LoadIssues, LoadIssue{
				Path:    filepath.ToSlash(rel),
				Kind:    kind,
				Message: fmt.Sprintf("cannot access %s: %v", filepath.ToSlash(rel), err),
				Err:     err,
			})
			return nil
		}
		if d.IsDir() {
			return nil
		}
		wikiRelOS, err := filepath.Rel(wikiRoot, path)
		if err != nil {
			return err
		}
		wikiRel := filepath.ToSlash(wikiRelOS)
		repoRel := filepath.ToSlash(filepath.Join(wikiDir, wikiRelOS))

		// Verify strict symlink containment before including or reading any file.
		if _, cerr := paths.ResolveWithin(wikiRoot, wikiRel); cerr != nil {
			snap.LoadIssues = append(snap.LoadIssues, LoadIssue{
				Path:    repoRel,
				Kind:    "containment",
				Message: fmt.Sprintf("path escapes wiki boundary: %v", cerr),
				Err:     cerr,
			})
			return nil
		}

		snap.AllFiles = append(snap.AllFiles, repoRel)
		if strings.HasSuffix(strings.ToLower(wikiRel), ".md") {
			mdPaths = append(mdPaths, wikiRel)
		}
		return nil
	})
	if walkErr != nil {
		return nil, walkErr
	}

	sort.Strings(snap.AllFiles)
	sort.Strings(mdPaths)

	for _, wikiRel := range mdPaths {
		absPath := filepath.Join(wikiRoot, filepath.FromSlash(wikiRel))
		repoRel := filepath.ToSlash(filepath.Join(wikiDir, filepath.FromSlash(wikiRel)))

		info, statErr := os.Stat(absPath)
		if statErr != nil {
			kind := "io"
			if os.IsPermission(statErr) {
				kind = "permission"
			} else if os.IsNotExist(statErr) {
				kind = "missing"
			}
			snap.LoadIssues = append(snap.LoadIssues, LoadIssue{
				Path:    repoRel,
				Kind:    kind,
				Message: fmt.Sprintf("cannot stat %s: %v", repoRel, statErr),
				Err:     statErr,
			})
			continue
		}

		data, readErr := os.ReadFile(absPath)
		snap.ReadCounts[repoRel]++
		if readErr != nil {
			kind := "io"
			if os.IsPermission(readErr) {
				kind = "permission"
			}
			snap.LoadIssues = append(snap.LoadIssues, LoadIssue{
				Path:    repoRel,
				Kind:    kind,
				Message: fmt.Sprintf("cannot read %s: %v", repoRel, readErr),
				Err:     readErr,
			})
			continue
		}

		page := ParsePage(repoRoot, wikiRoot, wikiDir, wikiRel, absPath, data, info.ModTime())
		snap.Pages = append(snap.Pages, page)
		snap.ByPath[page.WikiRel] = page
		snap.ByRepoPath[page.Path] = page
		snap.ByIDMulti[page.ID] = append(snap.ByIDMulti[page.ID], page)
		if _, exists := snap.ByID[page.ID]; !exists {
			snap.ByID[page.ID] = page
		}
		for _, tag := range page.Metadata.Tags {
			snap.Tags[tag] = struct{}{}
		}
	}

	snap.Index = snap.ByPath["index.md"]
	snap.Log = snap.ResolveCanonical("log.md")
	snap.Phases = snap.ResolveCanonical("phases.md")

	return snap, nil
}

// ResolveCanonical returns the Page for a canonical logical file (e.g. "log.md"),
// preferring the organized prologue/ layout before legacy root fallback.
func (s *WikiSnapshot) ResolveCanonical(name string) *Page {
	for _, cand := range CanonicalPathCandidates[name] {
		if p, ok := s.ByPath[cand]; ok {
			return p
		}
	}
	return nil
}

// Lookup resolves a page identifier with deterministic precedence:
// 1. Exact wiki-relative path (e.g. "prologue/schema.md")
// 2. Exact repo-relative path (e.g. "wiki/prologue/schema.md")
// 3. Exact page ID (e.g. "schema")
// Returns an error if the identifier is ambiguous or not found.
func (s *WikiSnapshot) Lookup(ref string) (*Page, error) {
	clean := filepath.ToSlash(filepath.Clean(strings.TrimSpace(ref)))
	if p, ok := s.ByPath[clean]; ok {
		return p, nil
	}
	if p, ok := s.ByRepoPath[clean]; ok {
		return p, nil
	}
	prefix := s.WikiDir + "/"
	if strings.HasPrefix(clean, prefix) {
		stripped := strings.TrimPrefix(clean, prefix)
		if p, ok := s.ByPath[stripped]; ok {
			return p, nil
		}
	}
	if matches := s.ByIDMulti[clean]; len(matches) == 1 {
		return matches[0], nil
	} else if len(matches) > 1 {
		paths := make([]string, len(matches))
		for i, m := range matches {
			paths[i] = m.WikiRel
		}
		return nil, fmt.Errorf("ambiguous page ID %q matches multiple pages: %s", ref, strings.Join(paths, ", "))
	}
	return nil, fmt.Errorf("page not found: %s", ref)
}

// FatalLoadError returns the first operational LoadIssue as an error, or nil
// when every wiki file loaded cleanly.
func (s *WikiSnapshot) FatalLoadError() error {
	if len(s.LoadIssues) == 0 {
		return nil
	}
	first := s.LoadIssues[0]
	return fmt.Errorf("%s (%s): %s", first.Path, first.Kind, first.Message)
}
