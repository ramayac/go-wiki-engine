package lint

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/ramayac/go-wiki-engine/internal/config"
	"github.com/ramayac/go-wiki-engine/internal/model"
	"github.com/ramayac/go-wiki-engine/internal/paths"
	"github.com/ramayac/go-wiki-engine/internal/schema"
)

// --- indexLinksChecker ---

type indexLinksChecker struct{}

func (c *indexLinksChecker) Name() string       { return "index-links" }
func (c *indexLinksChecker) Group() string      { return "links" }
func (c *indexLinksChecker) Category() Category { return CategorySnapshot }

func (c *indexLinksChecker) Check(snap *model.WikiSnapshot, _ *config.Config) ([]Issue, error) {
	if snap.Index == nil {
		return nil, nil
	}
	var issues []Issue
	for _, lk := range snap.Index.Links {
		if lk.IsExternal || lk.IsAnchorOnly || lk.BaseTarget == "" {
			continue
		}
		if lk.IsMarkdown {
			if lk.EscapesWiki || lk.WikiRel == "" {
				issues = append(issues, Issue{
					Severity: SevError,
					Check:    c.Name(),
					File:     snap.Index.Path,
					Line:     lk.Line,
					Message:  fmt.Sprintf("broken index link: %s", lk.RawTarget),
				})
				continue
			}
			if _, ok := snap.ByPath[lk.WikiRel]; !ok {
				issues = append(issues, Issue{
					Severity: SevError,
					Check:    c.Name(),
					File:     snap.Index.Path,
					Line:     lk.Line,
					Message:  fmt.Sprintf("broken index link: %s", lk.RawTarget),
				})
			}
		} else {
			if lk.EscapesRepo || lk.RepoRel == "" {
				issues = append(issues, Issue{
					Severity: SevError,
					Check:    c.Name(),
					File:     snap.Index.Path,
					Line:     lk.Line,
					Message:  fmt.Sprintf("broken index link: %s", lk.RawTarget),
				})
				continue
			}
			if _, err := os.Stat(filepath.Join(snap.Root, filepath.FromSlash(lk.RepoRel))); err != nil {
				issues = append(issues, Issue{
					Severity: SevError,
					Check:    c.Name(),
					File:     snap.Index.Path,
					Line:     lk.Line,
					Message:  fmt.Sprintf("broken index link: %s", lk.RawTarget),
				})
			}
		}
	}
	return issues, nil
}

// --- crossPageLinksChecker ---

type crossPageLinksChecker struct{}

func (c *crossPageLinksChecker) Name() string       { return "cross-page-links" }
func (c *crossPageLinksChecker) Group() string      { return "links" }
func (c *crossPageLinksChecker) Category() Category { return CategorySnapshot }

func (c *crossPageLinksChecker) Check(snap *model.WikiSnapshot, _ *config.Config) ([]Issue, error) {
	var issues []Issue
	for _, p := range snap.Pages {
		for _, lk := range p.Links {
			if !lk.IsMarkdown || lk.IsExternal || lk.IsAnchorOnly {
				continue
			}
			if lk.EscapesWiki {
				// Allow linking to valid repo-level markdown files outside the wiki (e.g. ../../README.md)
				// as long as the file exists inside the repo; otherwise flag broken link.
				if !lk.EscapesRepo && lk.RepoRel != "" {
					if _, err := os.Stat(filepath.Join(snap.Root, filepath.FromSlash(lk.RepoRel))); err == nil {
						continue
					}
				}
				issues = append(issues, Issue{
					Severity: SevError,
					Check:    c.Name(),
					File:     p.Path,
					Line:     lk.Line,
					Message:  fmt.Sprintf("broken link: %s", lk.RawTarget),
				})
				continue
			}
			if _, ok := snap.ByPath[lk.WikiRel]; !ok {
				issues = append(issues, Issue{
					Severity: SevError,
					Check:    c.Name(),
					File:     p.Path,
					Line:     lk.Line,
					Message:  fmt.Sprintf("broken link: %s", lk.RawTarget),
				})
			}
		}
	}
	return issues, nil
}

// --- markdownFormatChecker ---

var (
	inlineCodeSpanRe = regexp.MustCompile("``[^`]+``|`[^`]*`")
	wikiLinkRe       = regexp.MustCompile(`\[\[([^\]]+)\]\]`)
	spacedLinkRe     = regexp.MustCompile(`\[([^\]]*)\]\s+\(([^)]*)\)`)
	emptyLinkRe      = regexp.MustCompile(`\[([^\]]*)\]\(\s*\)`)
	emptyTextRe      = regexp.MustCompile(`\[\s*\]\(([^)]+)\)`)
	linkOpenRe       = regexp.MustCompile(`\[([^\]]+)\]\(([^)]*)`)
	referenceLinkRe  = regexp.MustCompile(`\[[^\]]+\]\[[^\]]*\]`)
)

func isInsideInlineCode(line string, start, end int) bool {
	for _, loc := range inlineCodeSpanRe.FindAllStringIndex(line, -1) {
		if start < loc[1] && end > loc[0] {
			return true
		}
	}
	return false
}

type markdownFormatChecker struct{}

func (c *markdownFormatChecker) Name() string       { return "markdown-format" }
func (c *markdownFormatChecker) Group() string      { return "links" }
func (c *markdownFormatChecker) Category() Category { return CategorySnapshot }

func (c *markdownFormatChecker) Check(snap *model.WikiSnapshot, _ *config.Config) ([]Issue, error) {
	var issues []Issue
	for _, p := range snap.Pages {
		inCodeBlock := false
		fmEnd := p.BodyStartLine - 1
		for i, line := range p.Lines {
			lineNo := i + 1
			if p.FMState == model.FrontMatterPresent && lineNo <= fmEnd {
				continue
			}
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "```") {
				inCodeBlock = !inCodeBlock
				continue
			}
			if inCodeBlock {
				continue
			}

			for _, loc := range wikiLinkRe.FindAllStringIndex(line, -1) {
				if isInsideInlineCode(line, loc[0], loc[1]) {
					continue
				}
				issues = append(issues, Issue{
					Severity: SevError,
					Check:    c.Name(),
					File:     p.Path,
					Line:     lineNo,
					Message:  fmt.Sprintf("non-standard wiki link format: %s (use [text](path) instead)", line[loc[0]:loc[1]]),
				})
			}
			for _, loc := range spacedLinkRe.FindAllStringIndex(line, -1) {
				if isInsideInlineCode(line, loc[0], loc[1]) {
					continue
				}
				issues = append(issues, Issue{
					Severity: SevError,
					Check:    c.Name(),
					File:     p.Path,
					Line:     lineNo,
					Message:  fmt.Sprintf("malformed markdown link with spaces: %s", line[loc[0]:loc[1]]),
				})
			}
			for _, loc := range emptyLinkRe.FindAllStringIndex(line, -1) {
				if isInsideInlineCode(line, loc[0], loc[1]) {
					continue
				}
				issues = append(issues, Issue{
					Severity: SevError,
					Check:    c.Name(),
					File:     p.Path,
					Line:     lineNo,
					Message:  fmt.Sprintf("empty link target: %s", line[loc[0]:loc[1]]),
				})
			}
			for _, loc := range emptyTextRe.FindAllStringIndex(line, -1) {
				if isInsideInlineCode(line, loc[0], loc[1]) {
					continue
				}
				issues = append(issues, Issue{
					Severity: SevError,
					Check:    c.Name(),
					File:     p.Path,
					Line:     lineNo,
					Message:  fmt.Sprintf("empty link text: %s", line[loc[0]:loc[1]]),
				})
			}
			for _, loc := range linkOpenRe.FindAllStringIndex(line, -1) {
				if isInsideInlineCode(line, loc[0], loc[1]) {
					continue
				}
				endIdx := loc[1]
				if endIdx >= len(line) || line[endIdx] != ')' {
					issues = append(issues, Issue{
						Severity: SevError,
						Check:    c.Name(),
						File:     p.Path,
						Line:     lineNo,
						Message:  fmt.Sprintf("unclosed markdown link parenthesis: %s", line[loc[0]:loc[1]]),
					})
				}
			}
			for _, loc := range referenceLinkRe.FindAllStringIndex(line, -1) {
				if isInsideInlineCode(line, loc[0], loc[1]) {
					continue
				}
				issues = append(issues, Issue{
					Severity: SevWarn,
					Check:    c.Name(),
					File:     p.Path,
					Line:     lineNo,
					Message:  fmt.Sprintf("reference-style link: %s (use [text](path) instead)", line[loc[0]:loc[1]]),
				})
			}
		}
	}
	return issues, nil
}

// --- bareURLChecker ---

var (
	htmlAnchorRe   = regexp.MustCompile(`</?[aA]\b`)
	markdownLinkRe = regexp.MustCompile(`!?\[[^\]]*\]\([^)]+\)`)
	bareURLRe      = regexp.MustCompile("https?://[^\\s`<>\\)]+")
)

type bareURLChecker struct{}

func (c *bareURLChecker) Name() string       { return "bare-urls" }
func (c *bareURLChecker) Group() string      { return "links" }
func (c *bareURLChecker) Category() Category { return CategorySnapshot }

func (c *bareURLChecker) Check(snap *model.WikiSnapshot, _ *config.Config) ([]Issue, error) {
	var issues []Issue
	for _, p := range snap.Pages {
		inCodeBlock := false
		fmEnd := p.BodyStartLine - 1
		for i, line := range p.Lines {
			lineNo := i + 1
			if p.FMState == model.FrontMatterPresent && lineNo <= fmEnd {
				continue
			}
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "```") {
				inCodeBlock = !inCodeBlock
				continue
			}
			if inCodeBlock {
				continue
			}
			cleaned := inlineCodeSpanRe.ReplaceAllString(line, "")
			cleaned = markdownLinkRe.ReplaceAllString(cleaned, "")
			if htmlAnchorRe.MatchString(cleaned) {
				issues = append(issues, Issue{
					Severity: SevError,
					Check:    c.Name(),
					File:     p.Path,
					Line:     lineNo,
					Message:  "use markdown links, not HTML",
				})
			}
			if match := bareURLRe.FindString(cleaned); match != "" {
				issues = append(issues, Issue{
					Severity: SevWarn,
					Check:    c.Name(),
					File:     p.Path,
					Line:     lineNo,
					Message:  fmt.Sprintf("bare URL outside link: %s (use [text](url) format)", match),
				})
			}
		}
	}
	return issues, nil
}

// --- externalLinksChecker ---

type externalLinksChecker struct{}

func (c *externalLinksChecker) Name() string       { return "external-links" }
func (c *externalLinksChecker) Group() string      { return "links" }
func (c *externalLinksChecker) Category() Category { return CategoryFilesystem }

func (c *externalLinksChecker) Check(snap *model.WikiSnapshot, _ *config.Config) ([]Issue, error) {
	var issues []Issue
	for _, p := range snap.Pages {
		pageDir := filepath.Dir(p.AbsPath)
		for _, lk := range p.Links {
			if lk.IsExternal || lk.IsAnchorOnly || lk.IsMarkdown || lk.BaseTarget == "" {
				continue
			}
			_, _, exists, err := paths.ResolveRepoSource(snap.Root, pageDir, lk.BaseTarget)
			if err != nil || !exists {
				issues = append(issues, Issue{
					Severity: SevWarn,
					Check:    c.Name(),
					File:     p.Path,
					Line:     lk.Line,
					Message:  fmt.Sprintf("broken external link: %s", lk.RawTarget),
				})
			}
		}
	}
	return issues, nil
}

// --- referencesChecker ---

type referencesChecker struct{}

func (c *referencesChecker) Name() string       { return "references" }
func (c *referencesChecker) Group() string      { return "links" }
func (c *referencesChecker) Category() Category { return CategoryFilesystem }

func (c *referencesChecker) Check(snap *model.WikiSnapshot, cfg *config.Config) ([]Issue, error) {
	pattern := cfg.IssuePattern
	if pattern == "" {
		pattern = schema.DefaultIssuePattern
	}
	issueRe, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("invalid issue_pattern %q: %w", pattern, err)
	}

	var issues []Issue
	for _, p := range snap.Pages {
		if p.FMState != model.FrontMatterPresent || p.FMErr != nil || len(p.Metadata.References) == 0 {
			continue
		}
		if !p.Metadata.IsDriftChecked() {
			continue
		}
		pageDir := filepath.Dir(p.AbsPath)
		line := lineOrOne(p.Metadata.FieldLines["references"])
		for _, r := range p.Metadata.References {
			switch r.Type {
			case "source":
				_, _, exists, rerr := paths.ResolveRepoSource(snap.Root, pageDir, r.Value)
				if rerr != nil || !exists {
					issues = append(issues, Issue{
						Severity: SevWarn,
						Check:    c.Name(),
						File:     p.Path,
						Line:     line,
						Message:  fmt.Sprintf("reference source not found: %s", r.Value),
					})
				}
			case "external":
				if !strings.HasPrefix(r.Value, "http://") && !strings.HasPrefix(r.Value, "https://") {
					issues = append(issues, Issue{
						Severity: SevWarn,
						Check:    c.Name(),
						File:     p.Path,
						Line:     line,
						Message:  fmt.Sprintf("reference external URL missing scheme: %s", r.Value),
					})
				}
			case "issue":
				if !issueRe.MatchString(r.Value) {
					issues = append(issues, Issue{
						Severity: SevWarn,
						Check:    c.Name(),
						File:     p.Path,
						Line:     line,
						Message:  fmt.Sprintf("reference issue key malformed: %s", r.Value),
					})
				}
			default:
				issues = append(issues, Issue{
					Severity: SevWarn,
					Check:    c.Name(),
					File:     p.Path,
					Line:     line,
					Message:  fmt.Sprintf("reference with unknown type %q: %s", r.Type, r.Value),
				})
			}
		}
	}
	return issues, nil
}

// --- sourcesChecker ---

type sourcesChecker struct{}

func (c *sourcesChecker) Name() string       { return "sources" }
func (c *sourcesChecker) Group() string      { return "links" }
func (c *sourcesChecker) Category() Category { return CategoryFilesystem }

func (c *sourcesChecker) Check(snap *model.WikiSnapshot, _ *config.Config) ([]Issue, error) {
	var issues []Issue
	for _, p := range snap.Pages {
		if p.FMState != model.FrontMatterPresent || p.FMErr != nil || len(p.Metadata.Sources) == 0 {
			continue
		}
		if !p.Metadata.IsDriftChecked() {
			continue
		}
		line := lineOrOne(p.Metadata.FieldLines["sources"])
		pageDir := filepath.Dir(p.AbsPath)
		for _, sr := range p.Metadata.Sources {
			if sr.File == "" {
				issues = append(issues, Issue{
					Severity: SevError,
					Check:    c.Name(),
					File:     p.Path,
					Line:     line,
					Message:  "source entry missing required file path",
				})
				continue
			}
			rs := model.ResolveSource(snap.Root, pageDir, sr)
			if rs.CrossRepo {
				continue // cross-repo source carries explicit repo and is not checked locally
			}
			if rs.Err != nil {
				// Containment violation already reported by path-containment checker.
				continue
			}
			if !rs.Exists {
				issues = append(issues, Issue{
					Severity: SevWarn,
					Check:    c.Name(),
					File:     p.Path,
					Line:     line,
					Message:  fmt.Sprintf("source not found: %s", sr.File),
				})
			}
		}
	}
	return issues, nil
}

// --- orphansChecker ---

type orphansChecker struct{}

func (c *orphansChecker) Name() string       { return "orphans" }
func (c *orphansChecker) Group() string      { return "graph" }
func (c *orphansChecker) Category() Category { return CategorySnapshot }

func (c *orphansChecker) Check(snap *model.WikiSnapshot, _ *config.Config) ([]Issue, error) {
	if snap.Index == nil {
		return nil, nil
	}
	linked := make(map[string]bool)
	for _, dest := range snap.Index.InternalMDLinks() {
		linked[dest] = true
	}
	for _, relID := range snap.Index.Metadata.Related {
		if t, err := snap.Lookup(relID); err == nil {
			linked[t.WikiRel] = true
		}
	}

	var issues []Issue
	for _, p := range snap.Pages {
		if linked[p.WikiRel] {
			continue
		}
		if p.WikiRel == "index.md" || p.WikiRel == "README.md" {
			continue
		}
		lc := p.EffectiveLifecycle()
		// Historical pages (legacy, superseded) are intentionally outside the active graph.
		if schema.IsHistoricalLifecycle(lc) {
			continue
		}
		issues = append(issues, Issue{
			Severity: SevWarn,
			Check:    c.Name(),
			File:     p.Path,
			Message:  "orphan page: not linked from index.md",
		})
	}
	return issues, nil
}

// --- leafPagesChecker ---

type leafPagesChecker struct{}

func (c *leafPagesChecker) Name() string       { return "leaf-pages" }
func (c *leafPagesChecker) Group() string      { return "graph" }
func (c *leafPagesChecker) Category() Category { return CategorySnapshot }

func (c *leafPagesChecker) Check(snap *model.WikiSnapshot, _ *config.Config) ([]Issue, error) {
	var issues []Issue
	for _, p := range snap.Pages {
		if model.IsCanonicalFile("log.md", p.WikiRel) {
			continue
		}
		if !p.IsActive() {
			continue
		}
		if len(p.InternalMDLinks()) == 0 && len(p.Metadata.Related) == 0 && len(p.Metadata.DependsOn) == 0 {
			issues = append(issues, Issue{
				Severity: SevInfo,
				Check:    c.Name(),
				File:     p.Path,
				Message:  "leaf page: no outgoing links to related pages (log.md is the only intentional leaf)",
			})
		}
	}
	return issues, nil
}
