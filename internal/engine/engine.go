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
	"time"

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
		err := func() error {
			f, err := os.Open(abs)
			if err != nil {
				return nil
			}
			defer func() { _ = f.Close() }()
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
			return scanner.Err()
		}()
		if err != nil {
			return nil, err
		}
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
		err := func() error {
			f, err := os.Open(abs)
			if err != nil {
				return nil
			}
			defer func() { _ = f.Close() }()
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
			return scanner.Err()
		}()
		if err != nil {
			return nil, err
		}
	}
	return results, nil
}

var logHeadingRe = regexp.MustCompile(`^## \[`)

// LogTail returns the last N log headings from log.md.
func (e *Engine) LogTail(n int) ([]string, error) {
	if n <= 0 {
		n = e.Cfg.LogLines
	}
	logRel := e.resolveWikiFile("log.md")
	logFile := filepath.Join(e.WikiPath(), filepath.FromSlash(logRel))
	f, err := os.Open(logFile)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

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

	// Entries are prepended (newest first) — the log-chronology checker
	// enforces descending dates. Keep the most recent n.
	if len(headings) > n {
		headings = headings[:n]
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
		return nil, fmt.Errorf("git diff failed (this command requires git and a git repository): %w", err)
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

// Stats returns aggregate statistics about the wiki.
func (e *Engine) Stats() (*StatsResult, error) {
	files, err := e.List()
	if err != nil {
		return nil, err
	}
	sr := &StatsResult{Files: len(files)}

	var latest time.Time
	for _, rel := range files {
		abs := filepath.Join(e.RootDir, rel)
		info, err := os.Stat(abs)
		if err != nil {
			continue
		}
		if info.ModTime().After(latest) {
			latest = info.ModTime()
		}
		// Count lines.
		if strings.HasSuffix(rel, ".md") {
			err := func() error {
				f, err := os.Open(abs)
				if err != nil {
					return nil
				}
				defer func() { _ = f.Close() }()
				scanner := bufio.NewScanner(f)
				for scanner.Scan() {
					sr.TotalLines++
					if headingRe.MatchString(scanner.Text()) {
						sr.Headings++
					}
				}
				return scanner.Err()
			}()
			if err != nil {
				return nil, err
			}
		}
	}
	if !latest.IsZero() {
		sr.LastUpdated = latest.Format("2006-01-02")
	}
	return sr, nil
}

// ContextEntry is a single entry in the context catalog.
type ContextEntry struct {
	File        string `json:"file"`
	Status      string `json:"status"`
	Description string `json:"description"`
	Summary     string `json:"summary,omitempty"` // first ~3 paragraphs when --summarize
	LineCount   int    `json:"line_count"`
}

// ContextResult holds a condensed wiki snapshot for agent context loading.
type ContextResult struct {
	Files        int            `json:"files"`
	LastUpdated  string         `json:"last_updated"`
	Phase        string         `json:"phase"`
	Catalog      []ContextEntry `json:"catalog"`
	RecentLog    []string       `json:"recent_log"`
	HeadingCount int            `json:"heading_count"`
	Summarized   bool           `json:"summarized"`
}

// Context returns a condensed snapshot of the wiki — enough for an agent
// to understand what's in the wiki without reading every file.
// When minimal is true, only the catalog and recent log are returned.
// When summarize is true, each catalog entry includes a page summary
// (first heading and first few paragraphs) plus line count, and
// non-active pages (status legacy/deprecated) are filtered out.
func (e *Engine) Context(minimal, summarize bool) (*ContextResult, error) {
	cr := &ContextResult{}

	// Build catalog from index.md.
	wikiDir := e.WikiPath()
	indexPath := filepath.Join(wikiDir, "index.md")
	var rawCatalog []ContextEntry
	if data, err := os.ReadFile(indexPath); err == nil {
		rawCatalog = parseIndexCatalog(string(data))
	}

	// Fetch status and filter based on active/summarize requirements
	var filteredCatalog []ContextEntry
	for _, entry := range rawCatalog {
		fm, _ := e.PageFrontMatter(filepath.Join(e.Cfg.WikiDir, entry.File))
		entry.Status = fm.Status

		isActive := fm.Status == "current" || fm.Status == "planned"

		if summarize && !isActive {
			continue
		}

		filteredCatalog = append(filteredCatalog, entry)
	}
	cr.Catalog = filteredCatalog

	// Stats.
	if st, err := e.Stats(); err == nil {
		cr.Files = st.Files
		cr.LastUpdated = st.LastUpdated
		cr.HeadingCount = st.Headings
	}

	// Recent log.
	if tail, err := e.LogTail(3); err == nil {
		cr.RecentLog = tail
	}

	// Populate summaries if requested.
	if summarize {
		cr.Summarized = true
		for i, entry := range cr.Catalog {
			sr, err := e.Summary(entry.File)
			if err != nil {
				continue
			}
			cr.Catalog[i].Summary = pageSummaryText(sr)
			cr.Catalog[i].LineCount = sr.LineCount
		}
	}

	// Phase status.
	if !minimal {
		cr.Phase = e.currentPhase()
	}

	return cr, nil
}

// pageSummaryText builds a compact summary string from a SummaryResult.
// Format: "# Heading\n\nFirst paragraph. ..."
func pageSummaryText(sr *SummaryResult) string {
	var b strings.Builder
	if sr.FirstHeader != "" {
		b.WriteString(sr.FirstHeader)
		b.WriteString("\n")
	}
	if sr.FirstPara != "" {
		b.WriteString(sr.FirstPara)
	}
	return strings.TrimSpace(b.String())
}

// parseIndexCatalog extracts page file + description from index.md content.
func parseIndexCatalog(content string) []ContextEntry {
	var entries []ContextEntry
	// Match entries with descriptions: - [text](file) | description
	catalogRe := regexp.MustCompile(`-\s+\[([^\]]+)\]\(([^)]+)\)\s*\|\s*(.+)`)
	for _, match := range catalogRe.FindAllStringSubmatch(content, -1) {
		if len(match) >= 4 {
			entries = append(entries, ContextEntry{
				File:        match[2],
				Description: strings.TrimSpace(match[3]),
			})
		}
	}
	// Also match entries without descriptions: - [text](file)
	simpleRe := regexp.MustCompile(`-\s+\[([^\]]+)\]\(([^)]+)\)`)
	seen := make(map[string]bool)
	for _, e := range entries {
		seen[e.File] = true
	}
	for _, match := range simpleRe.FindAllStringSubmatch(content, -1) {
		if len(match) >= 3 && !seen[match[2]] {
			entries = append(entries, ContextEntry{
				File:        match[2],
				Description: "",
			})
		}
	}
	return entries
}

// currentPhase reads the active phase status from phases.md.
func (e *Engine) currentPhase() string {
	phasesRel := e.resolveWikiFile("phases.md")
	phasesPath := filepath.Join(e.WikiPath(), filepath.FromSlash(phasesRel))
	f, err := os.Open(phasesPath)
	if err != nil {
		return "unknown"
	}
	defer func() { _ = f.Close() }()

	phaseRowRe := regexp.MustCompile(`^\|\s*(\d+)\s*\|\s*(.+?)\s*\|\s*(\S+)\s*\|`)
	var last string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		m := phaseRowRe.FindStringSubmatch(scanner.Text())
		if m != nil {
			last = fmt.Sprintf("Phase %s: %s — %s", m[1], strings.TrimSpace(m[2]), strings.TrimSpace(m[3]))
		}
	}
	if err := scanner.Err(); err != nil {
		return "unknown"
	}
	if last == "" {
		return "unknown"
	}
	return last
}

// SummaryResult holds a concise page preview.
type SummaryResult struct {
	File        string `json:"file"`
	FirstHeader string `json:"first_header"`
	FirstPara   string `json:"first_para"`
	Preview     string `json:"preview"` // first heading + up to 3 paragraphs
	LineCount   int    `json:"line_count"`
}

// Summary returns a preview of a single wiki page: its first heading,
// first paragraph, and line count. Useful for agents to preview a page
// before loading it fully.
func (e *Engine) Summary(page string) (*SummaryResult, error) {
	wikiDir := e.WikiPath()
	abs := filepath.Join(wikiDir, page)
	// Containment guard: reject paths that escape the wiki directory.
	if rel, err := filepath.Rel(wikiDir, abs); err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return nil, fmt.Errorf("page not found: %s", page)
	}
	f, err := os.Open(abs)
	if err != nil {
		return nil, fmt.Errorf("page not found: %s", page)
	}
	defer func() { _ = f.Close() }()

	sr := &SummaryResult{File: page}
	scanner := bufio.NewScanner(f)
	lineNo := 0
	paragraphCount := 0
	var previewLines []string
	inPara := false

	inFM := false
	firstLine := true

	for scanner.Scan() {
		lineNo++
		text := scanner.Text()
		trimmed := strings.TrimSpace(text)

		if firstLine {
			if trimmed == "" {
				continue
			}
			firstLine = false
			if trimmed == "---" {
				inFM = true
				continue
			}
		}

		if inFM {
			if trimmed == "---" {
				inFM = false
			}
			continue
		}

		// Capture first heading.
		if sr.FirstHeader == "" && headingRe.MatchString(text) {
			sr.FirstHeader = text
			previewLines = append(previewLines, text)
			continue
		}

		// Skip frontmatter, headings, tables, code blocks.
		if trimmed == "" || strings.HasPrefix(trimmed, "---") ||
			strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "|") ||
			strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			if inPara && trimmed == "" {
				inPara = false
			}
			continue
		}

		// Capture paragraphs (up to 3).
		if paragraphCount < 3 && !strings.HasPrefix(trimmed, ">") && !strings.HasPrefix(trimmed, "-") && !strings.HasPrefix(trimmed, "*") {
			if sr.FirstPara == "" {
				sr.FirstPara = text
			}
			inPara = true
			previewLines = append(previewLines, text)
			// Count paragraphs when we hit a blank line after content.
			paragraphCount++
			// Skip ahead to next blank line to avoid capturing same paragraph multiple times.
			for scanner.Scan() {
				lineNo++
				n := scanner.Text()
				if strings.TrimSpace(n) == "" {
					break
				}
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	sr.LineCount = lineNo
	sr.Preview = strings.TrimSpace(strings.Join(previewLines, "\n"))
	return sr, nil
}

// RelevanceResult is a ranked page match for a query.
type RelevanceResult struct {
	File  string  `json:"file"`
	Score float64 `json:"score"`
	Why   string  `json:"why"`
}

// Relevant ranks wiki pages by relevance to a query. It scores each page
// by heading-match count plus body-match count, weighted by heading
// proximity (matches near headings score higher).
func (e *Engine) Relevant(query string, topN int) ([]RelevanceResult, error) {
	if query == "" {
		return nil, fmt.Errorf("query is empty")
	}
	if topN <= 0 {
		topN = 5
	}

	files, err := e.List()
	if err != nil {
		return nil, err
	}

	lowerQ := strings.ToLower(query)
	var results []RelevanceResult

	for _, rel := range files {
		if !strings.HasSuffix(rel, ".md") {
			continue
		}
		abs := filepath.Join(e.RootDir, rel)
		var score float64
		headingHits := 0
		bodyHits := 0
		var matchedHeadings []string

		err := func() error {
			f, err := os.Open(abs)
			if err != nil {
				return nil
			}
			defer func() { _ = f.Close() }()

			scanner := bufio.NewScanner(f)
			for scanner.Scan() {
				text := scanner.Text()
				lower := strings.ToLower(text)

				if headingRe.MatchString(text) {
					if strings.Contains(lower, lowerQ) {
						headingHits++
						matchedHeadings = append(matchedHeadings, text)
					}
					continue
				}

				if strings.Contains(lower, lowerQ) {
					bodyHits++
				}
			}
			return scanner.Err()
		}()
		if err != nil {
			return nil, err
		}

		// Score: heading matches are worth 3× body matches.
		score = float64(headingHits)*3.0 + float64(bodyHits)

		if score > 0 {
			why := fmt.Sprintf("%d heading match(es), %d body match(es)", headingHits, bodyHits)
			if len(matchedHeadings) > 0 {
				why += fmt.Sprintf(" under %s", strings.Join(matchedHeadings, ", "))
			}
			results = append(results, RelevanceResult{
				File:  rel,
				Score: score,
				Why:   why,
			})
		}
	}

	// Sort descending by score.
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	if len(results) > topN {
		results = results[:topN]
	}
	return results, nil
}

// ImpactResult reports which wiki pages mention a changed file.
type ImpactResult struct {
	ChangedFile string   `json:"changed_file"`
	WikiPages   []string `json:"wiki_pages"`
}

// Impact maps changed files to wiki pages that mention them.
// For each changed file, it searches wiki content for the file's basename
// and returns which wiki pages are likely impacted.
func (e *Engine) Impact(changedFiles []string) ([]ImpactResult, error) {
	files, err := e.List()
	if err != nil {
		return nil, err
	}

	// Build a map of basename → changed file(s).
	basenameToFiles := make(map[string][]string)
	for _, cf := range changedFiles {
		base := filepath.Base(cf)
		basenameToFiles[base] = append(basenameToFiles[base], cf)
	}

	// For each changed file, search wiki content.
	var results []ImpactResult
	seen := make(map[string]map[string]bool) // changed file → set of wiki pages

	for _, cf := range changedFiles {
		seen[cf] = make(map[string]bool)
	}

	for _, rel := range files {
		if !strings.HasSuffix(rel, ".md") {
			continue
		}
		abs := filepath.Join(e.RootDir, rel)
		data, err := os.ReadFile(abs)
		if err != nil {
			continue
		}
		content := string(data)
		lower := strings.ToLower(content)

		for base, cfs := range basenameToFiles {
			if strings.Contains(lower, strings.ToLower(base)) {
				for _, cf := range cfs {
					seen[cf][rel] = true
				}
			}
		}
	}

	for _, cf := range changedFiles {
		var pages []string
		for p := range seen[cf] {
			pages = append(pages, p)
		}
		sort.Strings(pages)
		results = append(results, ImpactResult{
			ChangedFile: cf,
			WikiPages:   pages,
		})
	}
	return results, nil
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
	dr := &DiffResult{From: from, To: to}

	// List wiki files at <from>.
	fromFiles, _ := e.filesAtRef(from)
	toFiles, _ := e.filesAtRef(to)

	fromSet := make(map[string]bool)
	toSet := make(map[string]bool)
	for _, f := range fromFiles {
		fromSet[f] = true
	}
	for _, f := range toFiles {
		toSet[f] = true
	}

	// Added in <to> but not in <from>.
	for _, f := range toFiles {
		if !fromSet[f] {
			dr.Added = append(dr.Added, f)
		}
	}
	// Removed from <from>.
	for _, f := range fromFiles {
		if !toSet[f] {
			dr.Removed = append(dr.Removed, f)
		}
	}
	// Changed (present in both, but modified).
	changedOut, err := e.changedWikiFiles(from + ".." + to)
	if err == nil {
		dr.Changed = changedOut
	}

	return dr, nil
}

// filesAtRef lists wiki files at a given git ref.
func (e *Engine) filesAtRef(ref string) ([]string, error) {
	cmd := exec.Command("git", "ls-tree", "-r", "--name-only", ref, e.Cfg.WikiDir+"/")
	cmd.Dir = e.RootDir
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	var files []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line != "" {
			files = append(files, line)
		}
	}
	return files, nil
}

// changedWikiFiles returns wiki files changed in a diff range.
func (e *Engine) changedWikiFiles(diffRange string) ([]string, error) {
	cmd := exec.Command("git", "diff", "--name-only", diffRange, "--", e.Cfg.WikiDir+"/")
	cmd.Dir = e.RootDir
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	var files []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line != "" {
			files = append(files, line)
		}
	}
	return files, nil
}

// WatchResult holds the output of one watch cycle.
type WatchResult struct {
	Changed    []string `json:"changed"`
	Candidates []string `json:"candidates"`
	LintOK     bool     `json:"lint_ok"`
	LintIssues []Issue  `json:"lint_issues,omitempty"`
}

// WatchOnce runs a single watch cycle: changed + candidates + lint.
func (e *Engine) WatchOnce() (*WatchResult, error) {
	wr := &WatchResult{}

	changed, err := e.Changed(e.Cfg.DefaultDiff)
	if err != nil {
		return nil, err
	}
	wr.Changed = changed

	candidates, err := e.Candidates(e.Cfg.DefaultDiff)
	if err != nil {
		return nil, err
	}
	wr.Candidates = candidates

	lint := e.Lint()
	wr.LintOK = lint.OK
	wr.LintIssues = lint.Issues

	return wr, nil
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
		b.WriteString(f)
		b.WriteString("\n")
	}

	// Recent log.
	b.WriteString("\n== recent log ==\n")
	tail, _ := e.LogTail(0)
	for _, h := range tail {
		b.WriteString(h)
		b.WriteString("\n")
	}

	// Changed files.
	b.WriteString("\n== changed files ==\n")
	changed, _ := e.Changed(diffRange)
	for _, f := range changed {
		b.WriteString(f)
		b.WriteString("\n")
	}

	// Ingest candidates.
	b.WriteString("\n== ingest candidates ==\n")
	for _, f := range candidates {
		b.WriteString(f)
		b.WriteString("\n")
	}

	// Lint.
	b.WriteString("\n== lint ==\n")
	lint := e.Lint()
	if lint.OK {
		b.WriteString("wiki lint OK\n")
	} else {
		for _, m := range lint.Messages {
			b.WriteString(m)
			b.WriteString("\n")
		}
	}

	return b.String(), nil
}
