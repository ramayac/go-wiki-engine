package model

import (
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/ramayac/go-wiki-engine/internal/paths"
)

// Heading represents a Markdown heading parsed from a page.
type Heading struct {
	Line  int    `json:"line"`
	Level int    `json:"level"`
	Text  string `json:"text"` // full line, e.g. "## Section"
	Title string `json:"title"`
}

// Link represents a Markdown link extracted from a page during snapshot load.
type Link struct {
	Line         int
	Text         string
	RawTarget    string
	BaseTarget   string // target without #fragment
	IsExternal   bool   // http:// or https:// or mailto:
	IsAnchorOnly bool   // starts with #
	IsMarkdown   bool   // BaseTarget ends with .md
	WikiRel      string // resolved wiki-relative slash path if internal .md and contained
	RepoRel      string // resolved repo-relative slash path if non-wiki repo file and contained
	EscapesWiki  bool   // true if an .md link traverses outside wikiRoot
	EscapesRepo  bool   // true if a link traverses outside repoRoot
	ResolveErr   error
}

// Page is the typed, single-read representation of one Markdown wiki page.
type Page struct {
	Path            string // repo-relative slash path, e.g. "wiki/prologue/schema.md"
	WikiRel         string // wiki-relative slash path, e.g. "prologue/schema.md"
	AbsPath         string
	ID              string // explicit Metadata.ID if present, otherwise inferred from filename stem
	ExplicitID      bool
	Metadata        PageMetadata
	FMState         FrontMatterState
	FMErr           error
	RawContent      string
	Body            string
	BodyStartLine   int
	Lines           []string
	Headings        []Heading
	Links           []Link
	ResolvedSources []ResolvedSource
	ModTime         time.Time
	LineCount       int
	HasBOM          bool // file started with a UTF-8 BOM (stripped from RawContent)
	CRLF            bool // file used CRLF line endings (RawContent uses LF)
}

// ExpectedID returns the canonical page ID derived from the page's filename
// without the ".md" extension. Root index.md always maps to "index".
func ExpectedID(wikiRel string) string {
	clean := filepath.ToSlash(filepath.Clean(wikiRel))
	if clean == "index.md" {
		return "index"
	}
	base := filepath.Base(clean)
	return strings.TrimSuffix(base, ".md")
}

// EffectiveTitle returns Metadata.Title if set, else the first H1 heading title,
// else the page ID.
func (p *Page) EffectiveTitle() string {
	if p.Metadata.Title != "" {
		return p.Metadata.Title
	}
	for _, h := range p.Headings {
		if h.Level == 1 {
			return h.Title
		}
	}
	return p.ID
}

// EffectiveLifecycle delegates to Metadata.EffectiveLifecycle.
func (p *Page) EffectiveLifecycle() string {
	return p.Metadata.EffectiveLifecycle()
}

// IsActive reports whether the page is active in the wiki graph.
func (p *Page) IsActive() bool {
	return p.Metadata.IsActive()
}

// InternalMDLinks returns deduplicated, contained wiki-relative targets of
// Markdown links from this page.
func (p *Page) InternalMDLinks() []string {
	var out []string
	seen := make(map[string]bool)
	for _, l := range p.Links {
		if l.IsMarkdown && !l.IsExternal && !l.IsAnchorOnly && !l.EscapesWiki && l.ResolveErr == nil && l.WikiRel != "" {
			if !seen[l.WikiRel] {
				seen[l.WikiRel] = true
				out = append(out, l.WikiRel)
			}
		}
	}
	return out
}

const utf8BOM = "\ufeff"

var (
	headingLevelRe = regexp.MustCompile(`^(#{1,6}) (.+)$`)
	mdLinkRe       = regexp.MustCompile(`\[([^\]]*)\]\(([^)]+)\)`)
)

// ParsePage parses raw Markdown bytes into a typed Page in one pass.
func ParsePage(repoRoot, wikiRoot, wikiDir, wikiRel, absPath string, raw []byte, modTime time.Time) *Page {
	text, hasBOM := strings.CutPrefix(string(raw), utf8BOM)
	crlf := strings.Count(text, "\r\n")
	norm := strings.ReplaceAll(text, "\r\n", "\n")
	fmRes := ParseFrontMatter(norm)
	lines := strings.Split(norm, "\n")
	// Do not count a trailing empty element caused by a terminating newline as an extra line.
	lineCount := len(lines)
	if lineCount > 0 && lines[lineCount-1] == "" {
		lineCount--
	}

	inferredID := ExpectedID(wikiRel)
	id := inferredID
	explicitID := false
	if fmRes.Metadata.ID != "" {
		id = fmRes.Metadata.ID
		explicitID = true
	}

	page := &Page{
		Path:          filepath.ToSlash(filepath.Join(wikiDir, filepath.FromSlash(wikiRel))),
		WikiRel:       filepath.ToSlash(wikiRel),
		AbsPath:       absPath,
		ID:            id,
		ExplicitID:    explicitID,
		Metadata:      fmRes.Metadata,
		FMState:       fmRes.State,
		FMErr:         fmRes.Err,
		RawContent:    norm,
		Body:          fmRes.Body,
		BodyStartLine: fmRes.BodyStartLine,
		Lines:         lines,
		ModTime:       modTime,
		LineCount:     lineCount,
		HasBOM:        hasBOM,
		CRLF:          crlf > 0 && crlf > strings.Count(norm, "\n")-crlf,
	}

	pageDir := filepath.Dir(absPath)

	// Extract headings and links in one pass over lines (skipping frontmatter and fenced code blocks for headings,
	// and skipping frontmatter for links so YAML reference values are not parsed as markdown links).
	var fence FenceTracker
	inHTMLComment := false
	fmEndLine := fmRes.BodyStartLine - 1 // 1-based inclusive end of frontmatter (0 if none)

	for i, line := range lines {
		lineNo := i + 1
		trimmed := strings.TrimSpace(line)
		if fmRes.State == FrontMatterPresent && lineNo <= fmEndLine {
			continue
		}
		if fence.Skip(line) {
			continue
		}
		if !fence.InFence() {
			if strings.Contains(trimmed, "<!--") {
				inHTMLComment = true
			}
			if !inHTMLComment {
				if m := headingLevelRe.FindStringSubmatch(line); m != nil {
					page.Headings = append(page.Headings, Heading{
						Line:  lineNo,
						Level: len(m[1]),
						Text:  line,
						Title: strings.TrimSpace(m[2]),
					})
				}
			}
			if inHTMLComment && strings.Contains(trimmed, "-->") {
				inHTMLComment = false
			}
		}

		// Extract markdown links (skip fenced code blocks to avoid false positives in examples).
		if fence.InFence() {
			continue
		}
		for _, m := range mdLinkRe.FindAllStringSubmatch(line, -1) {
			linkText := m[1]
			rawTarget := strings.TrimSpace(m[2])
			// Strip optional markdown link title: path "title"
			if sp := strings.IndexAny(rawTarget, " \t"); sp != -1 && !strings.HasPrefix(rawTarget, "http") {
				rawTarget = rawTarget[:sp]
			}
			lk := Link{
				Line:      lineNo,
				Text:      linkText,
				RawTarget: rawTarget,
			}
			switch {
			case strings.HasPrefix(rawTarget, "http://") || strings.HasPrefix(rawTarget, "https://") || strings.HasPrefix(rawTarget, "mailto:"):
				lk.IsExternal = true
			case strings.HasPrefix(rawTarget, "#"):
				lk.IsAnchorOnly = true
			default:
				base := rawTarget
				if idx := strings.Index(rawTarget, "#"); idx != -1 {
					base = rawTarget[:idx]
				}
				lk.BaseTarget = base
				if base == "" {
					lk.IsAnchorOnly = true
				} else if strings.HasSuffix(strings.ToLower(base), ".md") {
					lk.IsMarkdown = true
					_, wRel, err := paths.ResolvePageRelative(wikiRoot, pageDir, base)
					if err != nil {
						lk.EscapesWiki = true
						lk.ResolveErr = err
						// Still check if it resolves cleanly inside the repo (e.g., linking to root README.md).
						if _, rRel, _, rerr := paths.ResolveRepoSource(repoRoot, pageDir, base); rerr == nil {
							lk.RepoRel = rRel
						} else {
							lk.EscapesRepo = true
						}
					} else {
						lk.WikiRel = wRel
					}
				} else {
					_, rRel, _, rerr := paths.ResolveRepoSource(repoRoot, pageDir, base)
					if rerr != nil {
						lk.EscapesRepo = true
						lk.ResolveErr = rerr
					} else {
						lk.RepoRel = rRel
					}
				}
			}
			page.Links = append(page.Links, lk)
		}
	}

	// Resolve all declared sources once.
	for _, sr := range page.Metadata.AllSources() {
		page.ResolvedSources = append(page.ResolvedSources, ResolveSource(repoRoot, pageDir, sr))
	}

	return page
}

// FenceTracker follows fenced code blocks (``` and ~~~) line by line. A fence
// closes on a line of the same character that is at least as long as the
// opening fence and has no text after it.
type FenceTracker struct {
	char byte
	size int
}

// InFence reports whether the line last passed to Skip is inside a fenced block.
func (f *FenceTracker) InFence() bool { return f.size > 0 }

// Skip updates the tracker with one line. It returns true when the line is a
// fence delimiter, so callers can skip it as well as lines inside the fence.
func (f *FenceTracker) Skip(line string) bool {
	t := strings.TrimSpace(line)
	if len(t) < 3 || (t[0] != '`' && t[0] != '~') {
		return false
	}
	n := 0
	for n < len(t) && t[n] == t[0] {
		n++
	}
	if n < 3 {
		return false
	}
	if f.size == 0 {
		// A backtick fence cannot have a backtick in its info string.
		if t[0] == '`' && strings.Contains(t[n:], "`") {
			return false
		}
		f.char, f.size = t[0], n
		return true
	}
	if t[0] == f.char && n >= f.size && strings.TrimSpace(t[n:]) == "" {
		f.size = 0
		return true
	}
	return false
}
