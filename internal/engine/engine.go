// Package engine implements the wiki operations: list, headings, search,
// log-tail, changed, candidates, lint, and refresh.
package engine

import (
	"bufio"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/ramayac/go-wiki-engine/internal/config"
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

// WikiPath returns the absolute path to the wiki directory.
func (e *Engine) WikiPath() string {
	return filepath.Join(e.RootDir, e.Cfg.WikiDir)
}

// List returns all files in the wiki directory, sorted.
func (e *Engine) List() ([]string, error) {
	var files []string
	root := e.WikiPath()
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(e.RootDir, path)
		files = append(files, rel)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	return files, nil
}

// HeadingEntry is a heading found in a wiki file.
type HeadingEntry struct {
	File    string
	Line    int
	Heading string
}

var headingRe = regexp.MustCompile(`^#{1,6} `)

// Headings returns all Markdown headings across wiki files.
func (e *Engine) Headings() ([]HeadingEntry, error) {
	files, err := e.List()
	if err != nil {
		return nil, err
	}
	var entries []HeadingEntry
	for _, rel := range files {
		if !strings.HasSuffix(rel, ".md") {
			continue
		}
		abs := filepath.Join(e.RootDir, rel)
		f, err := os.Open(abs)
		if err != nil {
			continue
		}
		scanner := bufio.NewScanner(f)
		lineNo := 0
		for scanner.Scan() {
			lineNo++
			text := scanner.Text()
			if headingRe.MatchString(text) {
				entries = append(entries, HeadingEntry{
					File:    rel,
					Line:    lineNo,
					Heading: text,
				})
			}
		}
		f.Close()
	}
	return entries, nil
}

// SearchResult is a matching line from a wiki search.
type SearchResult struct {
	File string
	Line int
	Text string
}

// Search performs a case-insensitive fixed-string search across wiki files.
func (e *Engine) Search(query string) ([]SearchResult, error) {
	if query == "" {
		return nil, fmt.Errorf("search query is empty")
	}
	files, err := e.List()
	if err != nil {
		return nil, err
	}
	lowerQ := strings.ToLower(query)
	var results []SearchResult
	for _, rel := range files {
		abs := filepath.Join(e.RootDir, rel)
		f, err := os.Open(abs)
		if err != nil {
			continue
		}
		scanner := bufio.NewScanner(f)
		lineNo := 0
		for scanner.Scan() {
			lineNo++
			text := scanner.Text()
			if strings.Contains(strings.ToLower(text), lowerQ) {
				results = append(results, SearchResult{
					File: rel,
					Line: lineNo,
					Text: text,
				})
			}
		}
		f.Close()
	}
	return results, nil
}

var logHeadingRe = regexp.MustCompile(`^## \[`)

// LogTail returns the last N log headings from log.md.
func (e *Engine) LogTail(n int) ([]string, error) {
	if n <= 0 {
		n = e.Cfg.LogLines
	}
	logFile := filepath.Join(e.WikiPath(), "log.md")
	f, err := os.Open(logFile)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var headings []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if logHeadingRe.MatchString(line) {
			headings = append(headings, line)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	if len(headings) > n {
		headings = headings[len(headings)-n:]
	}
	return headings, nil
}

// Changed returns non-wiki files changed in the given git diff range.
func (e *Engine) Changed(diffRange string) ([]string, error) {
	if diffRange == "" {
		diffRange = e.Cfg.DefaultDiff
	}
	cmd := exec.Command("git", "diff", "--name-only", diffRange)
	cmd.Dir = e.RootDir
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git diff failed: %w", err)
	}
	var files []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		// Exclude wiki/ itself.
		if strings.HasPrefix(line, e.Cfg.WikiDir+"/") {
			continue
		}
		files = append(files, line)
	}
	return files, nil
}

// Candidates filters Changed output through the ignore patterns in .wikirc.
func (e *Engine) Candidates(diffRange string) ([]string, error) {
	changed, err := e.Changed(diffRange)
	if err != nil {
		return nil, err
	}
	var filtered []string
	for _, f := range changed {
		if !e.isIgnored(f) {
			filtered = append(filtered, f)
		}
	}
	return filtered, nil
}

func (e *Engine) isIgnored(path string) bool {
	for _, pattern := range e.Cfg.Ignore {
		// Directory prefix match.
		if strings.HasSuffix(pattern, "/") {
			if strings.HasPrefix(path, pattern) {
				return true
			}
			continue
		}
		// Glob match (e.g. *.log).
		if strings.Contains(pattern, "*") {
			matched, _ := filepath.Match(pattern, filepath.Base(path))
			if matched {
				return true
			}
			continue
		}
		// Exact match.
		if path == pattern {
			return true
		}
	}
	return false
}

// LintResult holds the outcome of a wiki lint check.
type LintResult struct {
	OK       bool
	Messages []string
}

var requiredFiles = []string{
	"README.md",
	"index.md",
	"log.md",
	"schema.md",
	"phases.md",
	"repo-map.md",
	"operations/ingest.md",
	"operations/query.md",
	"operations/lint.md",
}

var logHeadingValidRe = regexp.MustCompile(`^## \[\d{4}-\d{2}-\d{2}\] [^|]+ \| .+$`)
var markerRe = regexp.MustCompile(`(?i)(TODO:|TBD:|UNKNOWN:)`)

// Lint checks the wiki for structural issues.
func (e *Engine) Lint() LintResult {
	var msgs []string
	wikiDir := e.WikiPath()

	// Check required files.
	for _, req := range requiredFiles {
		p := filepath.Join(wikiDir, req)
		if _, err := os.Stat(p); os.IsNotExist(err) {
			msgs = append(msgs, fmt.Sprintf("missing required wiki file: %s/%s", e.Cfg.WikiDir, req))
		}
	}

	// Check index links.
	indexPath := filepath.Join(wikiDir, "index.md")
	if data, err := os.ReadFile(indexPath); err == nil {
		linkRe := regexp.MustCompile(`\]\(([^)]+)\)`)
		for _, match := range linkRe.FindAllStringSubmatch(string(data), -1) {
			target := match[1]
			if strings.HasPrefix(target, "http://") || strings.HasPrefix(target, "https://") || strings.HasPrefix(target, "#") {
				continue
			}
			linked := filepath.Join(wikiDir, target)
			if _, err := os.Stat(linked); os.IsNotExist(err) {
				msgs = append(msgs, fmt.Sprintf("broken index link: %s", target))
			}
		}
	}

	// Check log heading format.
	logPath := filepath.Join(wikiDir, "log.md")
	if f, err := os.Open(logPath); err == nil {
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := scanner.Text()
			if logHeadingRe.MatchString(line) && !logHeadingValidRe.MatchString(line) {
				msgs = append(msgs, fmt.Sprintf("invalid log heading: %s", line))
			}
		}
		f.Close()
	}

	// Check for markers.
	files, _ := e.List()
	for _, rel := range files {
		if !strings.HasSuffix(rel, ".md") {
			continue
		}
		abs := filepath.Join(e.RootDir, rel)
		f, err := os.Open(abs)
		if err != nil {
			continue
		}
		scanner := bufio.NewScanner(f)
		lineNo := 0
		inCodeBlock := false
		for scanner.Scan() {
			lineNo++
			text := scanner.Text()
			// Track fenced code blocks.
			if strings.HasPrefix(strings.TrimSpace(text), "```") {
				inCodeBlock = !inCodeBlock
				continue
			}
			if inCodeBlock {
				continue
			}
			if markerRe.MatchString(text) {
				msgs = append(msgs, fmt.Sprintf("marker in %s:%d: %s", rel, lineNo, strings.TrimSpace(text)))
			}
		}
		f.Close()
	}

	return LintResult{
		OK:       len(msgs) == 0,
		Messages: msgs,
	}
}

// Refresh runs the full maintenance snapshot and returns a formatted report.
func (e *Engine) Refresh(diffRange string) (string, error) {
	candidates, err := e.Candidates(diffRange)
	if err != nil {
		return "", err
	}
	if len(candidates) == 0 {
		return fmt.Sprintf("no wiki refresh needed: no ingest candidates for diff range %s", diffRange), nil
	}

	var b strings.Builder

	// Wiki files.
	b.WriteString("== wiki files ==\n")
	files, _ := e.List()
	for _, f := range files {
		b.WriteString(f + "\n")
	}

	// Recent log.
	b.WriteString("\n== recent log ==\n")
	tail, _ := e.LogTail(0)
	for _, h := range tail {
		b.WriteString(h + "\n")
	}

	// Changed files.
	b.WriteString("\n== changed files ==\n")
	changed, _ := e.Changed(diffRange)
	for _, f := range changed {
		b.WriteString(f + "\n")
	}

	// Ingest candidates.
	b.WriteString("\n== ingest candidates ==\n")
	for _, f := range candidates {
		b.WriteString(f + "\n")
	}

	// Lint.
	b.WriteString("\n== lint ==\n")
	lint := e.Lint()
	if lint.OK {
		b.WriteString("wiki lint OK\n")
	} else {
		for _, m := range lint.Messages {
			b.WriteString(m + "\n")
		}
	}

	return b.String(), nil
}
