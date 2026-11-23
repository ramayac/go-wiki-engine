// Package engine implements the wiki operations: list, headings, search,
// log-tail, changed, candidates, check, lint, refresh, stats, context,
// summary, relevant, impact, diff, graph, and watch.
package engine

import (
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/ramayac/go-wiki-engine/internal/config"
	"github.com/ramayac/go-wiki-engine/internal/git"
	"github.com/ramayac/go-wiki-engine/internal/model"
	"github.com/ramayac/go-wiki-engine/internal/paths"
)

// Engine holds the runtime context for wiki operations.
type Engine struct {
	Cfg     *config.Config
	RootDir string // repo root (where .wikirc lives)
}

// New creates an Engine rooted at dir.
func New(cfg *config.Config, rootDir string) *Engine {
	return &Engine{Cfg: cfg, RootDir: rootDir}
}

// WikiPath returns the validated absolute path to the wiki directory.
// Callers that need error handling should use ResolveWikiPath or LoadSnapshot.
func (e *Engine) WikiPath() string {
	if resolved, err := paths.ResolveWikiRoot(e.RootDir, e.Cfg.WikiDir); err == nil {
		return resolved
	}
	return filepath.Join(e.RootDir, e.Cfg.WikiDir)
}

// ResolveWikiPath validates wiki_dir containment and returns the absolute wiki path.
func (e *Engine) ResolveWikiPath() (string, error) {
	return paths.ResolveWikiRoot(e.RootDir, e.Cfg.WikiDir)
}

// LoadSnapshot loads a single-read WikiSnapshot for the configured wiki.
func (e *Engine) LoadSnapshot() (*model.WikiSnapshot, error) {
	return model.LoadSnapshot(e.RootDir, e.Cfg.WikiDir)
}

// List returns all contained files in the wiki directory, sorted.
// Operational and containment errors are propagated instead of silently skipped.
func (e *Engine) List() ([]string, error) {
	snap, err := e.LoadSnapshot()
	if err != nil {
		return nil, err
	}
	if err := snap.FatalLoadError(); err != nil {
		return nil, err
	}
	return snap.AllFiles, nil
}

// HeadingEntry is a heading found in a wiki file.
type HeadingEntry struct {
	File    string `json:"file"`
	Line    int    `json:"line"`
	Heading string `json:"heading"`
}

var headingRe = regexp.MustCompile(`^#{1,6} `)

// Headings returns all Markdown headings across wiki files using one snapshot.
func (e *Engine) Headings() ([]HeadingEntry, error) {
	snap, err := e.LoadSnapshot()
	if err != nil {
		return nil, err
	}
	if err := snap.FatalLoadError(); err != nil {
		return nil, err
	}
	var entries []HeadingEntry
	for _, p := range snap.Pages {
		for _, h := range p.Headings {
			entries = append(entries, HeadingEntry{
				File:    p.Path,
				Line:    h.Line,
				Heading: h.Text,
			})
		}
	}
	return entries, nil
}

var logHeadingRe = regexp.MustCompile(`^## \[`)

// LogTail returns the most recent N log headings from log.md.
func (e *Engine) LogTail(n int) ([]string, error) {
	if n <= 0 {
		n = e.Cfg.LogLines
	}
	snap, err := e.LoadSnapshot()
	if err != nil {
		return nil, err
	}
	if err := snap.FatalLoadError(); err != nil {
		return nil, err
	}
	return logTailFromSnapshot(snap, n)
}

func logTailFromSnapshot(snap *model.WikiSnapshot, n int) ([]string, error) {
	if snap.Log == nil {
		return nil, fmt.Errorf("log.md not found in %s", snap.WikiDir)
	}
	var headings []string
	for _, line := range snap.Log.Lines {
		if logHeadingRe.MatchString(line) {
			headings = append(headings, line)
		}
	}
	if len(headings) > n {
		headings = headings[:n]
	}
	return headings, nil
}

// ChangeOptions configures branch-aware change discovery for changed/candidates.
type ChangeOptions struct {
	Base               string
	DiffRange          string
	IncludeWorkingTree bool
}

// DiscoverChanges runs the unified Git change discovery pipeline.
func (e *Engine) DiscoverChanges(opts ChangeOptions) (*git.ChangeSet, error) {
	return git.DiscoverChanges(e.RootDir, git.ChangeOptions{
		Base:               opts.Base,
		ConfigBase:         e.Cfg.BaseRef,
		DiffRange:          opts.DiffRange,
		DefaultDiff:        e.Cfg.DefaultDiff,
		IncludeWorkingTree: opts.IncludeWorkingTree,
	})
}

// Changed returns non-wiki files changed in the given git diff range (including
// working-tree and untracked changes via the unified Git pipeline).
func (e *Engine) Changed(diffRange string) ([]string, error) {
	return e.ChangedWithOptions(ChangeOptions{
		DiffRange:          diffRange,
		IncludeWorkingTree: true,
	})
}

// ChangedWithOptions returns non-wiki changed files according to opts.
func (e *Engine) ChangedWithOptions(opts ChangeOptions) ([]string, error) {
	cs, err := e.DiscoverChanges(opts)
	if err != nil {
		return nil, err
	}
	return cs.NonWikiChanged(e.Cfg.WikiDir), nil
}

// Candidates filters Changed output through the ignore patterns in .wikirc.
func (e *Engine) Candidates(diffRange string) ([]string, error) {
	return e.CandidatesWithOptions(ChangeOptions{
		DiffRange:          diffRange,
		IncludeWorkingTree: true,
	})
}

// CandidatesWithOptions filters ChangedWithOptions through .wikirc ignore patterns.
func (e *Engine) CandidatesWithOptions(opts ChangeOptions) ([]string, error) {
	cs, err := e.DiscoverChanges(opts)
	if err != nil {
		return nil, err
	}
	return e.candidateFiles(cs), nil
}

// candidateFiles returns the live, non-wiki changed files that no ignore
// pattern matches. Deleted files and the old side of renames are not candidates.
func (e *Engine) candidateFiles(cs *git.ChangeSet) []string {
	var filtered []string
	for _, f := range cs.LiveNonWikiChanged(e.Cfg.WikiDir) {
		if !e.isIgnored(f) {
			filtered = append(filtered, f)
		}
	}
	return filtered
}

// isIgnored reports whether rel (a slash path relative to the engine root)
// matches any `ignore` pattern from .wikirc.
func (e *Engine) isIgnored(rel string) bool {
	for _, pattern := range e.Cfg.Ignore {
		if ignoreMatches(pattern, rel) {
			return true
		}
	}
	return false
}

// ignoreMatches applies one gitignore-like pattern to a slash path:
//   - A bare name or glob (no "/") matches any path segment.
//   - A pattern with "/" is anchored at the root and matched segment by segment;
//     a "**" segment matches zero or more segments.
//   - A trailing "/" matches a directory and everything beneath it.
//
// A match on a directory also covers every path beneath it.
func ignoreMatches(pattern, rel string) bool {
	pattern = filepath.ToSlash(pattern)
	rel = filepath.ToSlash(rel)
	dirOnly := strings.HasSuffix(pattern, "/")
	anchored := strings.HasPrefix(pattern, "/")
	pattern = strings.Trim(pattern, "/")
	if pattern == "" {
		return false
	}
	segs := strings.Split(rel, "/")
	if !anchored && !dirOnly && !strings.Contains(pattern, "/") {
		for _, s := range segs {
			if ok, _ := path.Match(pattern, s); ok {
				return true
			}
		}
		return false
	}
	return matchSegments(strings.Split(pattern, "/"), segs, dirOnly)
}

// matchSegments reports whether pat matches a leading run of segs. With
// needBeneath, at least one segment must remain after the match.
func matchSegments(pat, segs []string, needBeneath bool) bool {
	if len(pat) == 0 {
		return !needBeneath || len(segs) > 0
	}
	if pat[0] == "**" {
		for i := 0; i <= len(segs); i++ {
			if matchSegments(pat[1:], segs[i:], needBeneath) {
				return true
			}
		}
		return false
	}
	if len(segs) == 0 {
		return false
	}
	if ok, _ := path.Match(pat[0], segs[0]); !ok {
		return false
	}
	return matchSegments(pat[1:], segs[1:], needBeneath)
}

// JSONOutput is the standard JSON envelope for all commands.
type JSONOutput struct {
	OK    bool        `json:"ok"`
	Data  interface{} `json:"data,omitempty"`
	Error string      `json:"error,omitempty"`
}

// StatsResult holds aggregate wiki statistics.
type StatsResult struct {
	Files       int    `json:"files"`
	Headings    int    `json:"headings"`
	TotalLines  int    `json:"total_lines"`
	LastUpdated string `json:"last_updated"`
}

// Stats returns aggregate statistics about the wiki from a single snapshot.
func (e *Engine) Stats() (*StatsResult, error) {
	snap, err := e.LoadSnapshot()
	if err != nil {
		return nil, err
	}
	if err := snap.FatalLoadError(); err != nil {
		return nil, err
	}
	return statsFromSnapshot(snap), nil
}

func statsFromSnapshot(snap *model.WikiSnapshot) *StatsResult {
	sr := &StatsResult{Files: len(snap.AllFiles)}
	var latest time.Time
	for _, p := range snap.Pages {
		sr.TotalLines += p.LineCount
		sr.Headings += len(p.Headings)
		if p.ModTime.After(latest) {
			latest = p.ModTime
		}
	}
	// Include non-markdown file mtimes in latest calculation if present.
	for _, rel := range snap.AllFiles {
		if strings.HasSuffix(strings.ToLower(rel), ".md") {
			continue
		}
		if info, err := os.Stat(filepath.Join(snap.Root, filepath.FromSlash(rel))); err == nil {
			if info.ModTime().After(latest) {
				latest = info.ModTime()
			}
		}
	}
	if !latest.IsZero() {
		sr.LastUpdated = latest.Format("2006-01-02")
	}
	return sr
}

// DiffResult holds the output of wiki-engine diff.
type DiffResult struct {
	From    string   `json:"from"`
	To      string   `json:"to"`
	Added   []string `json:"added"`
	Removed []string `json:"removed"`
	Changed []string `json:"changed"`
}

// Diff shows wiki file changes between two git refs using git diff.
func (e *Engine) Diff(from, to string) (*DiffResult, error) {
	if _, err := e.ResolveWikiPath(); err != nil {
		return nil, err
	}
	dr := &DiffResult{From: from, To: to}

	fromFiles, err := e.filesAtRef(from)
	if err != nil {
		return nil, fmt.Errorf("listing wiki files at %q: %w", from, err)
	}
	toFiles, err := e.filesAtRef(to)
	if err != nil {
		return nil, fmt.Errorf("listing wiki files at %q: %w", to, err)
	}

	fromSet := make(map[string]bool)
	toSet := make(map[string]bool)
	for _, f := range fromFiles {
		fromSet[f] = true
	}
	for _, f := range toFiles {
		toSet[f] = true
	}

	if len(fromFiles) == 0 && len(toFiles) == 0 {
		return dr, nil
	}

	for _, f := range toFiles {
		if !fromSet[f] {
			dr.Added = append(dr.Added, f)
		}
	}
	for _, f := range fromFiles {
		if !toSet[f] {
			dr.Removed = append(dr.Removed, f)
		}
	}

	changedOut, err := e.changedWikiFiles(from + ".." + to)
	if err != nil {
		return nil, fmt.Errorf("computing changed wiki files between %q and %q: %w", from, to, err)
	}
	dr.Changed = changedOut

	return dr, nil
}

func (e *Engine) filesAtRef(ref string) ([]string, error) {
	if !git.RefExists(e.RootDir, ref) {
		return nil, fmt.Errorf("invalid git ref %q", ref)
	}
	cmd := exec.Command("git", "ls-tree", "-r", "-z", "--name-only", ref, "--", e.Cfg.WikiDir+"/")
	cmd.Dir = e.RootDir
	out, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok && strings.Contains(string(ee.Stderr), "did not match") {
			return nil, nil
		}
		return nil, err
	}
	return splitNUL(out), nil
}

// splitNUL splits NUL-terminated git output into non-empty entries.
func splitNUL(out []byte) []string {
	var files []string
	for _, f := range strings.Split(string(out), "\x00") {
		if f != "" {
			files = append(files, f)
		}
	}
	return files
}

func (e *Engine) changedWikiFiles(diffRange string) ([]string, error) {
	if err := git.ValidateDiffRange(e.RootDir, diffRange); err != nil {
		return nil, fmt.Errorf("invalid diff range: %w", err)
	}
	cmd := exec.Command("git", "diff", "-z", "--name-only", "--relative", "--end-of-options", diffRange, "--", e.Cfg.WikiDir+"/")
	cmd.Dir = e.RootDir
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	return splitNUL(out), nil
}

// Refresh runs the full maintenance snapshot and returns a formatted report.
func (e *Engine) Refresh(diffRange string) (string, error) {
	opts := ChangeOptions{DiffRange: diffRange, IncludeWorkingTree: true}
	cs, err := e.DiscoverChanges(opts)
	if err != nil {
		return "", err
	}
	changed := cs.NonWikiChanged(e.Cfg.WikiDir)
	candidates := e.candidateFiles(cs)
	if len(candidates) == 0 {
		label := diffRange
		if label == "" {
			label = cs.Base
		}
		return fmt.Sprintf("no wiki refresh needed: no ingest candidates for diff range %s", label), nil
	}

	snap, err := e.LoadSnapshot()
	if err != nil {
		return "", err
	}
	if err := snap.FatalLoadError(); err != nil {
		return "", err
	}

	var b strings.Builder

	b.WriteString("== wiki files ==\n")
	for _, f := range snap.AllFiles {
		b.WriteString(f)
		b.WriteString("\n")
	}

	b.WriteString("\n== recent log ==\n")
	tail, err := logTailFromSnapshot(snap, e.Cfg.LogLines)
	if err != nil {
		return "", err
	}
	for _, h := range tail {
		b.WriteString(h)
		b.WriteString("\n")
	}

	b.WriteString("\n== changed files ==\n")
	for _, f := range changed {
		b.WriteString(f)
		b.WriteString("\n")
	}

	b.WriteString("\n== ingest candidates ==\n")
	for _, f := range candidates {
		b.WriteString(f)
		b.WriteString("\n")
	}

	b.WriteString("\n== lint ==\n")
	lintRes := e.lintSnapshot(snap, nil, nil)
	if lintRes.OK {
		b.WriteString("wiki lint OK\n")
	} else {
		for _, m := range lintRes.Messages {
			b.WriteString(m)
			b.WriteString("\n")
		}
	}

	return b.String(), nil
}
