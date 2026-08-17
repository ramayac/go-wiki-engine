package engine

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// --- Severity & Issue types ---

// Severity classifies the importance of a lint finding.
type Severity int

const (
	SevInfo Severity = iota
	SevWarn
	SevError
)

func (s Severity) String() string {
	switch s {
	case SevInfo:
		return "info"
	case SevWarn:
		return "warn"
	case SevError:
		return "error"
	default:
		return "unknown"
	}
}

// MarshalJSON renders severity as a lowercase string.
func (s Severity) MarshalJSON() ([]byte, error) {
	return []byte(`"` + s.String() + `"`), nil
}

// Issue is a single lint finding.
type Issue struct {
	Severity Severity `json:"severity"`
	Check    string   `json:"check"`
	File     string   `json:"file"`
	Line     int      `json:"line"`
	Message  string   `json:"message"`
}

// Checker inspects the wiki and returns a list of issues.
type Checker interface {
	Name() string
	Check(e *Engine) ([]Issue, error)
}

// LintResult holds the outcome of a wiki lint check.
type LintResult struct {
	OK       bool
	Messages []string // kept for backward compatibility with tests + Refresh()
	Issues   []Issue
}

// --- Checker implementations ---

// requiredFile is a logical wiki file required by the contract, with the
// accepted candidate paths (organized layout first, legacy flat layout for
// backward compatibility). The file is missing only when none of its
// candidates exist.
type requiredFile struct {
	name       string
	candidates []string
	preferred  string // path shown in the issue when missing
}

// requiredFiles list (used by requiredFilesChecker).
var requiredFiles = []requiredFile{
	{"README.md", []string{"README.md"}, "README.md"},
	{"index.md", []string{"index.md"}, "index.md"},
	{"log.md", canonicalPathCandidates["log.md"], "prologue/log.md"},
	{"schema.md", canonicalPathCandidates["schema.md"], "prologue/schema.md"},
	{"phases.md", canonicalPathCandidates["phases.md"], "prologue/phases.md"},
	{"repo-map.md", canonicalPathCandidates["repo-map.md"], "prologue/repo-map.md"},
	{"operations/ingest.md", []string{"operations/ingest.md"}, "operations/ingest.md"},
	{"operations/query.md", []string{"operations/query.md"}, "operations/query.md"},
	{"operations/lint.md", []string{"operations/lint.md"}, "operations/lint.md"},
}

type requiredFilesChecker struct{}

func (c *requiredFilesChecker) Name() string { return "required-files" }

func (c *requiredFilesChecker) Check(e *Engine) ([]Issue, error) {
	var issues []Issue
	wikiDir := e.WikiPath()
	for _, req := range requiredFiles {
		found := false
		for _, cand := range req.candidates {
			p := filepath.Join(wikiDir, filepath.FromSlash(cand))
			if _, err := os.Stat(p); err == nil {
				found = true
				break
			}
		}
		if found {
			continue
		}
		issues = append(issues, Issue{
			Severity: SevError,
			Check:    c.Name(),
			File:     filepath.Join(e.Cfg.WikiDir, req.preferred),
			Message:  fmt.Sprintf("missing required wiki file: %s/%s", e.Cfg.WikiDir, req.preferred),
		})
	}
	return issues, nil
}

// indexLinksChecker validates links from index.md only.
type indexLinksChecker struct{}

func (c *indexLinksChecker) Name() string { return "index-links" }

func (c *indexLinksChecker) Check(e *Engine) ([]Issue, error) {
	var issues []Issue
	wikiDir := e.WikiPath()
	indexPath := filepath.Join(wikiDir, "index.md")
	data, err := os.ReadFile(indexPath)
	if err != nil {
		return issues, nil
	}
	linkRe := regexp.MustCompile(`\]\(([^)]+)\)`)
	for _, match := range linkRe.FindAllStringSubmatch(string(data), -1) {
		target := match[1]
		if strings.HasPrefix(target, "http://") || strings.HasPrefix(target, "https://") || strings.HasPrefix(target, "#") {
			continue
		}
		baseTarget := target
		if idx := strings.Index(target, "#"); idx != -1 {
			baseTarget = target[:idx]
		}
		if baseTarget == "" {
			continue
		}
		linked := filepath.Join(wikiDir, baseTarget)
		if _, err := os.Stat(linked); os.IsNotExist(err) {
			issues = append(issues, Issue{
				Severity: SevError,
				Check:    c.Name(),
				File:     filepath.Join(e.Cfg.WikiDir, "index.md"),
				Message:  fmt.Sprintf("broken index link: %s", target),
			})
		}
	}
	return issues, nil
}

// crossPageLinksChecker validates all [text](path) links in every wiki .md file.
type crossPageLinksChecker struct{}

func (c *crossPageLinksChecker) Name() string { return "cross-page-links" }

func (c *crossPageLinksChecker) Check(e *Engine) ([]Issue, error) {
	var issues []Issue
	files, err := e.List()
	if err != nil {
		return nil, err
	}
	linkRe := regexp.MustCompile(`\]\(([^)]+)\)`)
	for _, rel := range files {
		if !strings.HasSuffix(rel, ".md") {
			continue
		}
		abs := filepath.Join(e.RootDir, rel)
		data, err := os.ReadFile(abs)
		if err != nil {
			continue
		}
		lines := strings.Split(string(data), "\n")
		for lineNo, line := range lines {
			for _, match := range linkRe.FindAllStringSubmatch(line, -1) {
				target := match[1]
				if strings.HasPrefix(target, "http://") || strings.HasPrefix(target, "https://") || strings.HasPrefix(target, "#") {
					continue
				}
				pageDir := filepath.Dir(abs)
				baseTarget := target
				if idx := strings.Index(target, "#"); idx != -1 {
					baseTarget = target[:idx]
				}
				if baseTarget == "" {
					continue
				}
				// Only check cross-page markdown links here.
				if !strings.HasSuffix(baseTarget, ".md") {
					continue
				}
				// Links are strictly page-relative: a subdirectory page must use ../ to
				// reach siblings in other directories. No wiki-root fallback — tolerating
				// it masked broken links when pages moved into subdirectories.
				linked := filepath.Clean(filepath.Join(pageDir, baseTarget))
				if _, err := os.Stat(linked); os.IsNotExist(err) {
					issues = append(issues, Issue{
						Severity: SevError,
						Check:    c.Name(),
						File:     rel,
						Line:     lineNo + 1,
						Message:  fmt.Sprintf("broken link: %s", target),
					})
				}
			}
		}
	}
	return issues, nil
}

// inlineCodeSpanRe matches single and double backtick inline code spans.
// Double-backtick spans must be attempted first so that the empty span
// between two adjacent backticks does not win the leftmost match.
var inlineCodeSpanRe = regexp.MustCompile("``[^`]+``|`[^`]*`")

// isInsideInlineCode reports whether the byte range [start, end) overlaps an
// inline code span on the line. Span-based detection replaces the previous
// naive backtick counting, which misfired on runs of backticks.
func isInsideInlineCode(line string, start, end int) bool {
	for _, loc := range inlineCodeSpanRe.FindAllStringIndex(line, -1) {
		if start < loc[1] && end > loc[0] {
			return true
		}
	}
	return false
}

// markdownFormatChecker detects malformed markdown links or non-standard link formats (like wiki-style [[links]]).
type markdownFormatChecker struct{}

func (c *markdownFormatChecker) Name() string { return "markdown-format" }

var (
	wikiLinkRe      = regexp.MustCompile(`\[\[([^\]]+)\]\]`)
	spacedLinkRe    = regexp.MustCompile(`\[([^\]]*)\]\s+\(([^)]*)\)`)
	emptyLinkRe     = regexp.MustCompile(`\[([^\]]*)\]\(\s*\)`) // empty target like `[text]()`
	emptyTextRe     = regexp.MustCompile(`\[\s*\]\(([^)]+)\)`)  // empty link text like `[](target)`
	linkOpenRe      = regexp.MustCompile(`\[([^\]]+)\]\(([^)]*)`)
	referenceLinkRe = regexp.MustCompile(`\[[^\]]+\]\[[^\]]*\]`) // reference-style links like `[text][ref]`
)

func (c *markdownFormatChecker) Check(e *Engine) ([]Issue, error) {
	var issues []Issue
	files, err := e.List()
	if err != nil {
		return nil, err
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
		lines := strings.Split(string(data), "\n")
		inCodeBlock := false
		for lineNo, line := range lines {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "```") {
				inCodeBlock = !inCodeBlock
				continue
			}
			if inCodeBlock {
				continue
			}

			// 1. Check for wiki-style links [[...]]
			for _, loc := range wikiLinkRe.FindAllStringIndex(line, -1) {
				if isInsideInlineCode(line, loc[0], loc[1]) {
					continue
				}
				m := line[loc[0]:loc[1]]
				issues = append(issues, Issue{
					Severity: SevError,
					Check:    c.Name(),
					File:     rel,
					Line:     lineNo + 1,
					Message:  fmt.Sprintf("non-standard wiki link format: %s (use [text](path) instead)", m),
				})
			}

			// 2. Check for spaced markdown links [text] (path)
			for _, loc := range spacedLinkRe.FindAllStringIndex(line, -1) {
				if isInsideInlineCode(line, loc[0], loc[1]) {
					continue
				}
				m := line[loc[0]:loc[1]]
				issues = append(issues, Issue{
					Severity: SevError,
					Check:    c.Name(),
					File:     rel,
					Line:     lineNo + 1,
					Message:  fmt.Sprintf("malformed markdown link with spaces: %s", m),
				})
			}

			// 3. Check for empty link targets [text]()
			for _, loc := range emptyLinkRe.FindAllStringIndex(line, -1) {
				if isInsideInlineCode(line, loc[0], loc[1]) {
					continue
				}
				m := line[loc[0]:loc[1]]
				issues = append(issues, Issue{
					Severity: SevError,
					Check:    c.Name(),
					File:     rel,
					Line:     lineNo + 1,
					Message:  fmt.Sprintf("empty link target: %s", m),
				})
			}

			// 4. Check for empty link text [](target)
			for _, loc := range emptyTextRe.FindAllStringIndex(line, -1) {
				if isInsideInlineCode(line, loc[0], loc[1]) {
					continue
				}
				m := line[loc[0]:loc[1]]
				issues = append(issues, Issue{
					Severity: SevError,
					Check:    c.Name(),
					File:     rel,
					Line:     lineNo + 1,
					Message:  fmt.Sprintf("empty link text: %s", m),
				})
			}

			// 5. Check for unclosed markdown link parenthesis: [text](path
			for _, loc := range linkOpenRe.FindAllStringIndex(line, -1) {
				if isInsideInlineCode(line, loc[0], loc[1]) {
					continue
				}
				endIdx := loc[1]
				if endIdx >= len(line) || line[endIdx] != ')' {
					matchText := line[loc[0]:loc[1]]
					issues = append(issues, Issue{
						Severity: SevError,
						Check:    c.Name(),
						File:     rel,
						Line:     lineNo + 1,
						Message:  fmt.Sprintf("unclosed markdown link parenthesis: %s", matchText),
					})
				}
			}

			// 6. Check for reference-style links [text][ref]
			for _, loc := range referenceLinkRe.FindAllStringIndex(line, -1) {
				if isInsideInlineCode(line, loc[0], loc[1]) {
					continue
				}
				m := line[loc[0]:loc[1]]
				issues = append(issues, Issue{
					Severity: SevWarn,
					Check:    c.Name(),
					File:     rel,
					Line:     lineNo + 1,
					Message:  fmt.Sprintf("reference-style link: %s (use [text](path) instead)", m),
				})
			}
		}
	}
	return issues, nil
}

// orphansChecker detects .md files in wiki/ not linked from index.md.
type orphansChecker struct{}

func (c *orphansChecker) Name() string { return "orphans" }

func (c *orphansChecker) Check(e *Engine) ([]Issue, error) {
	var issues []Issue
	wikiDir := e.WikiPath()

	var allFiles []string
	_ = filepath.WalkDir(wikiDir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if strings.HasSuffix(path, ".md") {
			rel, _ := filepath.Rel(wikiDir, path)
			allFiles = append(allFiles, rel)
		}
		return nil
	})

	indexPath := filepath.Join(wikiDir, "index.md")
	data, err := os.ReadFile(indexPath)
	if err != nil {
		return issues, nil
	}
	linked := make(map[string]bool)
	for _, dest := range ExtractLinks(string(data), wikiDir, wikiDir) {
		linked[dest] = true
	}

	for _, f := range allFiles {
		if linked[f] {
			continue
		}
		// Core wiki infrastructure files that don't need index entries.
		switch f {
		case "index.md", "README.md":
			continue
		}
		issues = append(issues, Issue{
			Severity: SevWarn,
			Check:    c.Name(),
			File:     filepath.Join(e.Cfg.WikiDir, f),
			Message:  "orphan page: not linked from index.md",
		})
	}
	return issues, nil
}

// headingHierarchyChecker detects skipped heading levels and multiple h1s.
type headingHierarchyChecker struct{}

func (c *headingHierarchyChecker) Name() string { return "heading-hierarchy" }

var headingLevelRe = regexp.MustCompile(`^(#{1,6}) `)

func (c *headingHierarchyChecker) Check(e *Engine) ([]Issue, error) {
	var issues []Issue
	files, err := e.List()
	if err != nil {
		return nil, err
	}
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
			lastLevel := 0
			h1Count := 0
			inHTMLComment := false
			inCodeBlock := false
			for scanner.Scan() {
				lineNo++
				text := scanner.Text()
				trimmed := strings.TrimSpace(text)

				// Skip fenced code blocks: `#` lines inside ``` fences are code, not headings.
				if strings.HasPrefix(trimmed, "```") {
					inCodeBlock = !inCodeBlock
					continue
				}
				if inCodeBlock {
					continue
				}

				// Track HTML comments (<!-- ... -->).
				if strings.Contains(trimmed, "<!--") {
					inHTMLComment = true
				}
				if inHTMLComment {
					if strings.Contains(trimmed, "-->") {
						inHTMLComment = false
					}
					continue
				}

				m := headingLevelRe.FindStringSubmatch(text)
				if m == nil {
					continue
				}
				level := len(m[1])
				if level == 1 {
					h1Count++
				}
				if lastLevel > 0 && level > lastLevel+1 {
					issues = append(issues, Issue{
						Severity: SevWarn,
						Check:    c.Name(),
						File:     rel,
						Line:     lineNo,
						Message:  fmt.Sprintf("heading level skip: h%d → h%d", lastLevel, level),
					})
				}
				lastLevel = level
			}
			if h1Count > 1 {
				issues = append(issues, Issue{
					Severity: SevInfo,
					Check:    c.Name(),
					File:     rel,
					Message:  fmt.Sprintf("multiple h1 headings (%d)", h1Count),
				})
			}
			return scanner.Err()
		}()
		if err != nil {
			return nil, err
		}
	}
	return issues, nil
}

// logHeadingsChecker validates log heading format.
type logHeadingsChecker struct{}

func (c *logHeadingsChecker) Name() string { return "log-headings" }

var logHeadingValidRe = regexp.MustCompile(`^## \[\d{4}-\d{2}-\d{2}\] [^|]+ \| .+$`)

func (c *logHeadingsChecker) Check(e *Engine) ([]Issue, error) {
	var issues []Issue
	logRel := e.resolveWikiFile("log.md")
	logPath := filepath.Join(e.WikiPath(), filepath.FromSlash(logRel))
	f, err := os.Open(logPath)
	if err != nil {
		return issues, nil
	}
	defer func() { _ = f.Close() }()

	scanner := bufio.NewScanner(f)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := scanner.Text()
		if logHeadingRe.MatchString(line) && !logHeadingValidRe.MatchString(line) {
			issues = append(issues, Issue{
				Severity: SevError,
				Check:    c.Name(),
				File:     filepath.Join(e.Cfg.WikiDir, logRel),
				Line:     lineNo,
				Message:  fmt.Sprintf("invalid log heading format: %s", line),
			})
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return issues, nil
}

// logChronologyChecker ensures log entries are in descending date order.
type logChronologyChecker struct{}

func (c *logChronologyChecker) Name() string { return "log-chronology" }

var logDateRe = regexp.MustCompile(`^## \[(\d{4}-\d{2}-\d{2})\]`)

func (c *logChronologyChecker) Check(e *Engine) ([]Issue, error) {
	var issues []Issue
	logRel := e.resolveWikiFile("log.md")
	logPath := filepath.Join(e.WikiPath(), filepath.FromSlash(logRel))
	f, err := os.Open(logPath)
	if err != nil {
		return issues, nil
	}
	defer func() { _ = f.Close() }()

	var dates []string
	var dateLines []int
	scanner := bufio.NewScanner(f)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := scanner.Text()
		m := logDateRe.FindStringSubmatch(line)
		if m != nil {
			dates = append(dates, m[1])
			dateLines = append(dateLines, lineNo)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	for i := 1; i < len(dates); i++ {
		if dates[i] > dates[i-1] {
			issues = append(issues, Issue{
				Severity: SevWarn,
				Check:    c.Name(),
				File:     filepath.Join(e.Cfg.WikiDir, logRel),
				Line:     dateLines[i],
				Message:  fmt.Sprintf("log entries not in descending date order: %s after %s", dates[i], dates[i-1]),
			})
		}
	}
	return issues, nil
}

// markersChecker flags TODO/TBD/UNKNOWN markers outside code blocks.
type markersChecker struct{}

func (c *markersChecker) Name() string { return "markers" }

var markerRe = regexp.MustCompile(`(?i)(TODO:|TBD:|UNKNOWN:)`)

func (c *markersChecker) Check(e *Engine) ([]Issue, error) {
	var issues []Issue
	files, err := e.List()
	if err != nil {
		return nil, err
	}
	for _, rel := range files {
		if !strings.HasSuffix(rel, ".md") {
			continue
		}
		abs := filepath.Join(e.RootDir, rel)
		f, err := os.Open(abs)
		if err != nil {
			continue
		}
		defer func() { _ = f.Close() }()
		scanner := bufio.NewScanner(f)
		lineNo := 0
		inCodeBlock := false
		for scanner.Scan() {
			lineNo++
			text := scanner.Text()
			if strings.HasPrefix(strings.TrimSpace(text), "```") {
				inCodeBlock = !inCodeBlock
				continue
			}
			if inCodeBlock {
				continue
			}
			if markerRe.MatchString(text) {
				issues = append(issues, Issue{
					Severity: SevWarn,
					Check:    c.Name(),
					File:     rel,
					Line:     lineNo,
					Message:  fmt.Sprintf("marker: %s", strings.TrimSpace(text)),
				})
			}
		}
		if err := scanner.Err(); err != nil {
			return nil, err
		}
	}
	return issues, nil
}

// phaseConsistencyChecker validates the phases.md status board.
type phaseConsistencyChecker struct{}

func (c *phaseConsistencyChecker) Name() string { return "phase-consistency" }

func (c *phaseConsistencyChecker) Check(e *Engine) ([]Issue, error) {
	var issues []Issue
	phasesRel := e.resolveWikiFile("phases.md")
	phasesPath := filepath.Join(e.WikiPath(), filepath.FromSlash(phasesRel))
	f, err := os.Open(phasesPath)
	if err != nil {
		return issues, nil
	}
	defer func() { _ = f.Close() }()

	type phaseRow struct {
		num    int
		status string
		line   int
	}
	var phases []phaseRow
	phaseRowRe := regexp.MustCompile(`^\|\s*(\d+)\s*\|\s*(.+?)\s*\|\s*(\S+)\s*\|`)

	scanner := bufio.NewScanner(f)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := scanner.Text()
		m := phaseRowRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		num := 0
		_, _ = fmt.Sscanf(m[1], "%d", &num)
		phases = append(phases, phaseRow{
			num:    num,
			status: strings.TrimSpace(m[3]),
			line:   lineNo,
		})
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	validStatuses := map[string]bool{
		"not-started": true,
		"in-progress": true,
		"completed":   true,
		"blocked":     true,
	}

	completed := make(map[int]bool)
	for _, p := range phases {
		if !validStatuses[p.status] {
			issues = append(issues, Issue{
				Severity: SevWarn,
				Check:    c.Name(),
				File:     filepath.Join(e.Cfg.WikiDir, phasesRel),
				Line:     p.line,
				Message:  fmt.Sprintf("unknown phase status: %q", p.status),
			})
		}
		if p.status == "completed" {
			completed[p.num] = true
		}
	}

	// Check that completed phases have their prerequisites completed.
	for _, p := range phases {
		if p.num > 0 && p.status != "not-started" && !completed[p.num-1] {
			issues = append(issues, Issue{
				Severity: SevWarn,
				Check:    c.Name(),
				File:     filepath.Join(e.Cfg.WikiDir, phasesRel),
				Line:     p.line,
				Message:  fmt.Sprintf("phase %d is %s but phase %d is not completed", p.num, p.status, p.num-1),
			})
		}
	}

	return issues, nil
}

// externalLinksChecker validates links from wiki pages to source files in the repo.
type externalLinksChecker struct{}

func (c *externalLinksChecker) Name() string { return "external-links" }

func (c *externalLinksChecker) Check(e *Engine) ([]Issue, error) {
	var issues []Issue
	files, err := e.List()
	if err != nil {
		return nil, err
	}
	linkRe := regexp.MustCompile(`\]\(([^)]+)\)`)
	for _, rel := range files {
		if !strings.HasSuffix(rel, ".md") {
			continue
		}
		abs := filepath.Join(e.RootDir, rel)
		data, err := os.ReadFile(abs)
		if err != nil {
			continue
		}
		lines := strings.Split(string(data), "\n")
		for lineNo, line := range lines {
			for _, match := range linkRe.FindAllStringSubmatch(line, -1) {
				target := match[1]
				if strings.HasPrefix(target, "http://") || strings.HasPrefix(target, "https://") || strings.HasPrefix(target, "#") {
					continue
				}
				baseTarget := target
				if idx := strings.Index(target, "#"); idx != -1 {
					baseTarget = target[:idx]
				}
				if baseTarget == "" {
					continue
				}
				// Skip .md links (handled by cross-page checker).
				if strings.HasSuffix(baseTarget, ".md") {
					continue
				}
				// Resolve relative to the page's directory, then relative to repo root.
				pageDir := filepath.Dir(abs)
				resolved := filepath.Clean(filepath.Join(pageDir, baseTarget))
				if _, err := os.Stat(resolved); os.IsNotExist(err) {
					// Also try relative to repo root.
					resolvedRoot := filepath.Join(e.RootDir, baseTarget)
					if _, err2 := os.Stat(resolvedRoot); os.IsNotExist(err2) {
						issues = append(issues, Issue{
							Severity: SevWarn,
							Check:    c.Name(),
							File:     rel,
							Line:     lineNo + 1,
							Message:  fmt.Sprintf("broken external link: %s", target),
						})
					}
				}
			}
		}
	}
	return issues, nil
}

// duplicateContentChecker detects pages with substantially similar content.
// Threshold comes from .wikirc duplicate_threshold (default 0.7).
type duplicateContentChecker struct{}

func (c *duplicateContentChecker) Name() string { return "duplicate-content" }

func (c *duplicateContentChecker) Check(e *Engine) ([]Issue, error) {
	if e.Cfg.DuplicateThreshold <= 0 {
		return nil, nil // disabled
	}
	var issues []Issue
	files, err := e.List()
	if err != nil {
		return nil, err
	}

	// Collect .md files with their word-sets.
	type page struct {
		file  string
		words map[string]bool
	}
	var pages []page
	for _, rel := range files {
		if !strings.HasSuffix(rel, ".md") {
			continue
		}
		abs := filepath.Join(e.RootDir, rel)
		data, err := os.ReadFile(abs)
		if err != nil {
			continue
		}
		// Build word set (lowercase, min 3 chars).
		words := make(map[string]bool)
		for _, w := range strings.Fields(strings.ToLower(string(data))) {
			w = strings.Trim(w, ".,;:!?()[]{}`\"'")
			if len(w) >= 3 {
				words[w] = true
			}
		}
		if len(words) > 10 {
			pages = append(pages, page{file: rel, words: words})
		}
	}

	// Compare each pair using Jaccard similarity.
	threshold := e.Cfg.DuplicateThreshold
	for i := 0; i < len(pages); i++ {
		for j := i + 1; j < len(pages); j++ {
			sim := jaccardSimilarity(pages[i].words, pages[j].words)
			if sim >= threshold {
				issues = append(issues, Issue{
					Severity: SevWarn,
					Check:    c.Name(),
					File:     pages[i].file,
					Message:  fmt.Sprintf("%.0f%% similar to %s — consider merging", sim*100, pages[j].file),
				})
			}
		}
	}
	return issues, nil
}

// jaccardSimilarity returns the Jaccard index (intersection / union) of two word sets.
func jaccardSimilarity(a, b map[string]bool) float64 {
	if len(a) == 0 && len(b) == 0 {
		return 0
	}
	intersection := 0
	for w := range a {
		if b[w] {
			intersection++
		}
	}
	union := len(a) + len(b) - intersection
	if union == 0 {
		return 0
	}
	return float64(intersection) / float64(union)
}

// pageLastModified returns the date the page was last modified, preferring the
// last commit date from git history. This avoids the false "everything is
// stale" signal that filesystem mtimes produce after a fresh clone (all files
// share the checkout time). Falls back to mtime outside git repos or when git
// fails.
func (e *Engine) pageLastModified(rel string, info os.FileInfo) time.Time {
	gitPath := filepath.ToSlash(filepath.Join(e.Cfg.WikiDir, rel))
	cmd := exec.Command("git", "log", "-1", "--format=%cd", "--date=short", "--", gitPath)
	cmd.Dir = e.RootDir
	out, err := cmd.Output()
	if err != nil {
		return info.ModTime()
	}
	s := strings.TrimSpace(string(out))
	if s == "" {
		return info.ModTime()
	}
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return info.ModTime()
	}
	return t
}

// staleContentChecker detects wiki pages that haven't been updated recently
// despite active source-file changes. Stale threshold comes from .wikirc stale_days.
type staleContentChecker struct{}

func (c *staleContentChecker) Name() string { return "stale-content" }

func (c *staleContentChecker) Check(e *Engine) ([]Issue, error) {
	if e.Cfg.StaleDays <= 0 {
		return nil, nil // disabled
	}
	var issues []Issue
	wikiDir := e.WikiPath()
	staleThreshold := time.Now().AddDate(0, 0, -e.Cfg.StaleDays)

	// Check if there are active source changes (if not, everything is "stale" which is noisy).
	changed, _ := e.Changed(e.Cfg.DefaultDiff)
	hasSourceChanges := len(changed) > 0

	// Walk wiki .md files.
	_ = filepath.WalkDir(wikiDir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".md") {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(wikiDir, path)
		wikiRel := filepath.ToSlash(rel)

		// Skip infrastructure files.
		if wikiRel == "index.md" || wikiRel == "README.md" ||
			isCanonicalFile("log.md", wikiRel) || isCanonicalFile("phases.md", wikiRel) {
			return nil
		}

		if e.pageLastModified(rel, info).Before(staleThreshold) {
			severity := SevInfo
			msg := fmt.Sprintf("not updated in %d+ days", e.Cfg.StaleDays)
			if hasSourceChanges {
				severity = SevWarn
				msg += " (repo has active changes)"
			}
			issues = append(issues, Issue{
				Severity: severity,
				Check:    c.Name(),
				File:     filepath.Join(e.Cfg.WikiDir, rel),
				Message:  msg,
			})
		}
		return nil
	})
	return issues, nil
}

// frontMatterChecker validates YAML front matter on all wiki pages.
type frontMatterChecker struct{}

func (c *frontMatterChecker) Name() string { return "front-matter" }

func (c *frontMatterChecker) Check(e *Engine) ([]Issue, error) {
	var issues []Issue
	wikiDir := e.WikiPath()
	files, err := e.List()
	if err != nil {
		return nil, err
	}

	validStatuses := map[string]bool{
		"planned":    true,
		"current":    true,
		"legacy":     true,
		"deprecated": true,
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
		fm, found, err := ParseFrontMatter(content)

		if err != nil {
			issues = append(issues, Issue{
				Severity: SevError,
				Check:    c.Name(),
				File:     rel,
				Line:     1,
				Message:  fmt.Sprintf("malformed front matter: %v", err),
			})
			continue
		}

		if !found {
			issues = append(issues, Issue{
				Severity: SevWarn,
				Check:    c.Name(),
				File:     rel,
				Line:     1,
				Message:  "missing front matter block",
			})
			continue
		}

		// Check status
		if fm.Status == "" {
			issues = append(issues, Issue{
				Severity: SevError,
				Check:    c.Name(),
				File:     rel,
				Line:     1,
				Message:  "missing status field in front matter",
			})
		} else if !validStatuses[fm.Status] {
			issues = append(issues, Issue{
				Severity: SevError,
				Check:    c.Name(),
				File:     rel,
				Line:     1,
				Message:  fmt.Sprintf("invalid status value in front matter: %s (must be one of: planned, current, legacy, deprecated)", fm.Status),
			})
		}

		// Check description
		if fm.Description == "" {
			issues = append(issues, Issue{
				Severity: SevWarn,
				Check:    c.Name(),
				File:     rel,
				Line:     1,
				Message:  "missing recommended description field in front matter",
			})
		}

		// Check superseded_by
		if fm.Status == "deprecated" && fm.SupersededBy == "" {
			issues = append(issues, Issue{
				Severity: SevError,
				Check:    c.Name(),
				File:     rel,
				Line:     1,
				Message:  "superseded_by is required when status is deprecated",
			})
		}

		if fm.SupersededBy != "" {
			targetRel := fm.SupersededBy
			var targetAbs string

			// Try relative to current file's directory
			dir := filepath.Dir(abs)
			cand1 := filepath.Clean(filepath.Join(dir, targetRel))
			// Try relative to wiki root
			cand2 := filepath.Clean(filepath.Join(wikiDir, targetRel))

			if _, err1 := os.Stat(cand1); err1 == nil {
				targetAbs = cand1
			} else if _, err2 := os.Stat(cand2); err2 == nil {
				targetAbs = cand2
			}

			if targetAbs == "" {
				issues = append(issues, Issue{
					Severity: SevError,
					Check:    c.Name(),
					File:     rel,
					Line:     1,
					Message:  fmt.Sprintf("superseded_by target does not exist: %s", targetRel),
				})
			} else {
				// Parse target front matter to check status
				tData, err := os.ReadFile(targetAbs)
				if err == nil {
					tfm, _, _ := ParseFrontMatter(string(tData))
					if tfm.Status != "current" && tfm.Status != "planned" {
						issues = append(issues, Issue{
							Severity: SevError,
							Check:    c.Name(),
							File:     rel,
							Line:     1,
							Message:  fmt.Sprintf("superseded_by target %s is not active (status: %s)", targetRel, tfm.Status),
						})
					}
				}
			}
		}
	}

	return issues, nil
}

// indexFormatChecker validates the formatting of index.md entries.
type indexFormatChecker struct{}

func (c *indexFormatChecker) Name() string { return "index-format" }

func (c *indexFormatChecker) Check(e *Engine) ([]Issue, error) {
	var issues []Issue
	wikiDir := e.WikiPath()
	indexPath := filepath.Join(wikiDir, "index.md")
	data, err := os.ReadFile(indexPath)
	if err != nil {
		return nil, nil // Handled by required-files checker
	}

	lines := strings.Split(string(data), "\n")
	bulletLinkRe := regexp.MustCompile(`^\s*[-*]\s+\[`)
	linkRe := regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)

	for lineNo, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		if bulletLinkRe.MatchString(line) {
			parts := strings.SplitN(line, "|", 2)

			match := linkRe.FindStringSubmatch(parts[0])
			if len(match) < 3 {
				issues = append(issues, Issue{
					Severity: SevWarn,
					Check:    c.Name(),
					File:     filepath.Join(e.Cfg.WikiDir, "index.md"),
					Line:     lineNo + 1,
					Message:  "malformed link in index entry",
				})
				continue
			}

			target := match[2]
			if strings.HasPrefix(target, "/") || strings.Contains(target, "://") {
				issues = append(issues, Issue{
					Severity: SevError,
					Check:    c.Name(),
					File:     filepath.Join(e.Cfg.WikiDir, "index.md"),
					Line:     lineNo + 1,
					Message:  fmt.Sprintf("index entry must use a relative path: %s", target),
				})
			}

			if len(parts) < 2 {
				issues = append(issues, Issue{
					Severity: SevWarn,
					Check:    c.Name(),
					File:     filepath.Join(e.Cfg.WikiDir, "index.md"),
					Line:     lineNo + 1,
					Message:  "index entry missing pipe-separated description",
				})
			} else {
				desc := strings.TrimSpace(parts[1])
				if desc == "" {
					issues = append(issues, Issue{
						Severity: SevWarn,
						Check:    c.Name(),
						File:     filepath.Join(e.Cfg.WikiDir, "index.md"),
						Line:     lineNo + 1,
						Message:  "index entry has empty description",
					})
				}
			}
		}
	}

	return issues, nil
}

// bareUrlChecker flags bare URLs and HTML anchor tags outside code blocks.
type bareUrlChecker struct{}

func (c *bareUrlChecker) Name() string { return "bare-urls" }

func (c *bareUrlChecker) Check(e *Engine) ([]Issue, error) {
	var issues []Issue
	files, err := e.List()
	if err != nil {
		return nil, err
	}

	htmlAnchorRe := regexp.MustCompile(`</?[aA]\b`)
	inlineCodeRe := regexp.MustCompile("``[^`]+``|`[^`]*`")
	markdownLinkRe := regexp.MustCompile(`!?\[[^\]]*\]\([^)]+\)`)
	bareUrlRe := regexp.MustCompile("https?://[^\\s`<>\\)]+")

	for _, rel := range files {
		if !strings.HasSuffix(rel, ".md") {
			continue
		}
		abs := filepath.Join(e.RootDir, rel)
		data, err := os.ReadFile(abs)
		if err != nil {
			continue
		}

		lines := strings.Split(string(data), "\n")
		inCodeBlock := false

		for lineNo, line := range lines {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "```") {
				inCodeBlock = !inCodeBlock
				continue
			}
			if inCodeBlock {
				continue
			}

			cleaned := inlineCodeRe.ReplaceAllString(line, "")
			cleaned = markdownLinkRe.ReplaceAllString(cleaned, "")

			if htmlAnchorRe.MatchString(cleaned) {
				issues = append(issues, Issue{
					Severity: SevError,
					Check:    c.Name(),
					File:     rel,
					Line:     lineNo + 1,
					Message:  "use markdown links, not HTML",
				})
			}

			if match := bareUrlRe.FindString(cleaned); match != "" {
				issues = append(issues, Issue{
					Severity: SevWarn,
					Check:    c.Name(),
					File:     rel,
					Line:     lineNo + 1,
					Message:  fmt.Sprintf("bare URL outside link: %s (use [text](url) format)", match),
				})
			}
		}
	}

	return issues, nil
}

// leafPagesChecker flags active pages with no outgoing links to other wiki
// pages. A connected wiki is a graph, not a star: every active page should
// reference its related pages. log.md is the only intentional leaf because it
// is append-only. Severity is info so violations surface without failing the
// default lint gate (fail_severity defaults to warn).
type leafPagesChecker struct{}

func (c *leafPagesChecker) Name() string { return "leaf-pages" }

func (c *leafPagesChecker) Check(e *Engine) ([]Issue, error) {
	var issues []Issue
	wikiDir := e.WikiPath()
	files, err := e.List()
	if err != nil {
		return nil, err
	}
	for _, rel := range files {
		if !strings.HasSuffix(rel, ".md") {
			continue
		}
		wikiRel, err := filepath.Rel(wikiDir, filepath.Join(e.RootDir, rel))
		if err != nil {
			continue
		}
		wikiRel = filepath.ToSlash(wikiRel)

		// The append-only timeline is the one intentional leaf.
		if isCanonicalFile("log.md", wikiRel) {
			continue
		}

		abs := filepath.Join(e.RootDir, rel)
		data, err := os.ReadFile(abs)
		if err != nil {
			continue
		}
		fm, _, _ := ParseFrontMatter(string(data))

		// Only active pages participate in the graph.
		if fm.Status == "legacy" || fm.Status == "deprecated" {
			continue
		}

		links := ExtractLinks(string(data), filepath.Dir(abs), wikiDir)
		if len(links) == 0 {
			issues = append(issues, Issue{
				Severity: SevInfo,
				Check:    c.Name(),
				File:     rel,
				Message:  "leaf page: no outgoing links to related pages (log.md is the only intentional leaf)",
			})
		}
	}
	return issues, nil
}

// allCheckers returns the default set of lint checkers, ordered by priority.
func allCheckers() []Checker {
	return []Checker{
		&requiredFilesChecker{},
		&frontMatterChecker{},
		&indexFormatChecker{},
		&bareUrlChecker{},
		&indexLinksChecker{},
		&crossPageLinksChecker{},
		&markdownFormatChecker{},
		&orphansChecker{},
		&leafPagesChecker{},
		&headingHierarchyChecker{},
		&logHeadingsChecker{},
		&logChronologyChecker{},
		&markersChecker{},
		&phaseConsistencyChecker{},
		&externalLinksChecker{},
		&duplicateContentChecker{},
		&staleContentChecker{},
	}
}

// Lint runs all registered checkers and aggregates the results.
func (e *Engine) Lint() LintResult {
	return e.LintWithOptions(nil, nil)
}

// LintWithOptions runs specified checkers, skipping any listed in skip.
func (e *Engine) LintWithOptions(check []string, skip []string) LintResult {
	checkAll := true
	checkMap := make(map[string]bool)
	for _, c := range check {
		if c == "all" {
			checkAll = true
			break
		}
		if c != "" {
			checkAll = false
			checkMap[c] = true
		}
	}

	skipMap := make(map[string]bool)
	for _, s := range skip {
		if s != "" {
			skipMap[s] = true
		}
	}

	var activeCheckers []Checker
	for _, c := range allCheckers() {
		name := c.Name()
		if skipMap[name] {
			continue
		}
		if !checkAll && !checkMap[name] {
			continue
		}
		activeCheckers = append(activeCheckers, c)
	}

	var allIssues []Issue
	allIssues = make([]Issue, 0)
	for _, c := range activeCheckers {
		issues, err := c.Check(e)
		if err != nil {
			allIssues = append(allIssues, Issue{
				Severity: SevError,
				Check:    c.Name(),
				Message:  fmt.Sprintf("checker %s failed: %v", c.Name(), err),
			})
			continue
		}
		allIssues = append(allIssues, issues...)
	}

	// Stable sort by file then line.
	sort.SliceStable(allIssues, func(i, j int) bool {
		if allIssues[i].File != allIssues[j].File {
			return allIssues[i].File < allIssues[j].File
		}
		return allIssues[i].Line < allIssues[j].Line
	})

	// Build backward-compatible Messages.
	var msgs []string
	for _, iss := range allIssues {
		if iss.Line > 0 {
			msgs = append(msgs, fmt.Sprintf("%s:%d: [%s] %s", iss.File, iss.Line, iss.Check, iss.Message))
		} else {
			msgs = append(msgs, fmt.Sprintf("%s: [%s] %s", iss.File, iss.Check, iss.Message))
		}
	}

	// Parse fail threshold
	failThreshold := SevWarn // default fallback
	switch strings.ToLower(e.Cfg.FailSeverity) {
	case "info":
		failThreshold = SevInfo
	case "warn":
		failThreshold = SevWarn
	case "error":
		failThreshold = SevError
	}

	hasFailure := false
	for _, iss := range allIssues {
		if iss.Severity >= failThreshold {
			hasFailure = true
			break
		}
	}

	return LintResult{
		OK:       !hasFailure,
		Messages: msgs,
		Issues:   allIssues,
	}
}
