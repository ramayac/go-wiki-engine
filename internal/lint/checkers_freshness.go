package lint

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/ramayac/go-wiki-engine/internal/config"
	"github.com/ramayac/go-wiki-engine/internal/git"
	"github.com/ramayac/go-wiki-engine/internal/model"
)

// --- markersChecker ---

var markerRe = regexp.MustCompile(`(?i)(TODO:|TBD:|UNKNOWN:)`)

type markersChecker struct{}

func (c *markersChecker) Name() string       { return "markers" }
func (c *markersChecker) Group() string      { return "freshness" }
func (c *markersChecker) Category() Category { return CategorySnapshot }

func (c *markersChecker) Check(snap *model.WikiSnapshot, _ *config.Config) ([]Issue, error) {
	var issues []Issue
	for _, p := range snap.Pages {
		inCodeBlock := false
		fmEnd := p.BodyStartLine - 1
		for i, line := range p.Lines {
			lineNo := i + 1
			if p.FMState == model.FrontMatterPresent && lineNo <= fmEnd {
				continue
			}
			if strings.HasPrefix(strings.TrimSpace(line), "```") {
				inCodeBlock = !inCodeBlock
				continue
			}
			if inCodeBlock {
				continue
			}
			if markerRe.MatchString(line) {
				issues = append(issues, Issue{
					Severity: SevWarn,
					Check:    c.Name(),
					File:     p.Path,
					Line:     lineNo,
					Message:  fmt.Sprintf("marker: %s", strings.TrimSpace(line)),
				})
			}
		}
	}
	return issues, nil
}

// --- duplicateContentChecker ---

type duplicateContentChecker struct{}

func (c *duplicateContentChecker) Name() string       { return "duplicate-content" }
func (c *duplicateContentChecker) Group() string      { return "freshness" }
func (c *duplicateContentChecker) Category() Category { return CategorySnapshot }

func (c *duplicateContentChecker) Check(snap *model.WikiSnapshot, cfg *config.Config) ([]Issue, error) {
	if cfg.DuplicateThreshold <= 0 {
		return nil, nil
	}

	type pageWords struct {
		file  string
		words map[string]struct{}
	}
	var pages []pageWords
	// Inverted index word -> page indices so we only evaluate candidate pairs
	// that actually share vocabulary (avoiding naive O(n^2) on large wikis).
	inverted := make(map[string][]int)

	for _, p := range snap.Pages {
		words := extractBodyWordSet(p)
		if len(words) <= 10 {
			continue
		}
		idx := len(pages)
		pages = append(pages, pageWords{file: p.Path, words: words})
		for w := range words {
			inverted[w] = append(inverted[w], idx)
		}
	}

	// Count shared words per pair using the inverted index.
	type pair struct{ i, j int }
	shared := make(map[pair]int)
	for _, postings := range inverted {
		for a := 0; a < len(postings); a++ {
			for b := a + 1; b < len(postings); b++ {
				shared[pair{postings[a], postings[b]}]++
			}
		}
	}

	threshold := cfg.DuplicateThreshold
	var issues []Issue
	for i := 0; i < len(pages); i++ {
		for j := i + 1; j < len(pages); j++ {
			inter := shared[pair{i, j}]
			if inter == 0 {
				continue
			}
			union := len(pages[i].words) + len(pages[j].words) - inter
			if union == 0 {
				continue
			}
			sim := float64(inter) / float64(union)
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

// extractBodyWordSet builds a normalized word set excluding YAML frontmatter,
// fenced code blocks, and HTML template comments (Phase 9).
func extractBodyWordSet(p *model.Page) map[string]struct{} {
	words := make(map[string]struct{})
	inCodeBlock := false
	inComment := false
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
		if strings.Contains(trimmed, "<!--") {
			inComment = true
		}
		if inComment {
			if strings.Contains(trimmed, "-->") {
				inComment = false
			}
			continue
		}
		// Skip lifecycle banners (template boilerplate).
		if strings.HasPrefix(trimmed, "> **Lifecycle:") || strings.HasPrefix(trimmed, "> [!") {
			continue
		}
		for _, w := range strings.Fields(strings.ToLower(line)) {
			w = strings.Trim(w, ".,;:!?()[]{}`\"'*_#|-")
			if len(w) >= 3 {
				words[w] = struct{}{}
			}
		}
	}
	return words
}

// --- staleContentChecker ---

type staleContentChecker struct{}

func (c *staleContentChecker) Name() string       { return "stale-content" }
func (c *staleContentChecker) Group() string      { return "freshness" }
func (c *staleContentChecker) Category() Category { return CategoryGit }

func (c *staleContentChecker) Check(snap *model.WikiSnapshot, cfg *config.Config) ([]Issue, error) {
	if cfg.StaleDays <= 0 {
		return nil, nil
	}
	staleThreshold := time.Now().AddDate(0, 0, -cfg.StaleDays)

	// Unified git change discovery (best-effort; outside git repo hasSourceChanges=false).
	hasSourceChanges := false
	if cs, err := git.DiscoverChanges(snap.Root, git.ChangeOptions{
		ConfigBase:         cfg.BaseRef,
		DiffRange:          cfg.DefaultDiff,
		DefaultDiff:        cfg.DefaultDiff,
		IncludeWorkingTree: true,
	}); err == nil {
		hasSourceChanges = len(cs.NonWikiChanged(cfg.WikiDir)) > 0
	}

	// Single git invocation for all wiki page commit dates.
	commitDates, _ := git.PageCommitDates(snap.Root, cfg.WikiDir)

	var issues []Issue
	for _, p := range snap.Pages {
		if p.WikiRel == "index.md" || p.WikiRel == "README.md" ||
			model.IsCanonicalFile("log.md", p.WikiRel) || model.IsCanonicalFile("phases.md", p.WikiRel) {
			continue
		}
		// Implemented, legacy, and superseded pages do not receive active-drift checks.
		if p.FMState == model.FrontMatterPresent && !p.Metadata.IsDriftChecked() {
			continue
		}
		lastMod := p.ModTime
		if commitDates != nil {
			if t, ok := commitDates[p.Path]; ok && !t.IsZero() {
				lastMod = t
			}
		}
		if lastMod.Before(staleThreshold) {
			severity := SevInfo
			msg := fmt.Sprintf("not updated in %d+ days", cfg.StaleDays)
			if hasSourceChanges {
				severity = SevWarn
				msg += " (repo has active changes)"
			}
			issues = append(issues, Issue{
				Severity: severity,
				Check:    c.Name(),
				File:     p.Path,
				Message:  msg,
			})
		}
	}
	return issues, nil
}
