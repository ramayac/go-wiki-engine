package engine

import (
	"fmt"
	"sort"
	"strings"

	"github.com/ramayac/go-wiki-engine/internal/model"
	"github.com/ramayac/go-wiki-engine/internal/schema"
)

// SearchResult is a matching line from an exact-line wiki search.
type SearchResult struct {
	ID        string `json:"id,omitempty"`
	File      string `json:"file"`
	Lifecycle string `json:"lifecycle,omitempty"`
	Line      int    `json:"line"`
	Text      string `json:"text"`
}

// SearchOptions filters and configures both exact-line and ranked search.
type SearchOptions struct {
	Tag            string
	Type           string
	Lifecycle      string
	IncludeHistory bool
	Limit          int
	Keyword        bool // exact keyword/line mode
}

// RankedSearchResult is a field-weighted ranked match for a query.
type RankedSearchResult struct {
	ID           string   `json:"id"`
	File         string   `json:"file"`
	Title        string   `json:"title,omitempty"`
	Type         string   `json:"type,omitempty"`
	Lifecycle    string   `json:"lifecycle"`
	Score        float64  `json:"score"`
	MatchedTerms []string `json:"matched_terms"`
	Coverage     float64  `json:"coverage"`
	Section      string   `json:"section,omitempty"`
	StartLine    int      `json:"start_line"`
	Context      string   `json:"context"`
	Why          string   `json:"why"`
}

// RelevanceResult is kept as a compatibility type for `relevant`.
type RelevanceResult = RankedSearchResult

// Search performs a case-insensitive exact-line search across active wiki pages
// (excluding historical legacy/superseded pages by default).
func (e *Engine) Search(query string) ([]SearchResult, error) {
	return e.SearchLines(query, SearchOptions{})
}

// SearchLines performs filtered exact-line search from one WikiSnapshot.
func (e *Engine) SearchLines(query string, opts SearchOptions) ([]SearchResult, error) {
	if strings.TrimSpace(query) == "" {
		return nil, fmt.Errorf("search query is empty")
	}
	snap, err := e.LoadSnapshot()
	if err != nil {
		return nil, err
	}
	if err := snap.FatalLoadError(); err != nil {
		return nil, err
	}
	return searchLinesFromSnapshot(snap, query, opts), nil
}

func searchLinesFromSnapshot(snap *model.WikiSnapshot, query string, opts SearchOptions) []SearchResult {
	lowerQ := strings.ToLower(query)
	results := []SearchResult{}
	for _, p := range snap.Pages {
		if !matchesSearchFilters(p, opts) {
			continue
		}
		for i, text := range p.Lines {
			if strings.Contains(strings.ToLower(text), lowerQ) {
				results = append(results, SearchResult{
					ID:        p.ID,
					File:      p.Path,
					Lifecycle: p.EffectiveLifecycle(),
					Line:      i + 1,
					Text:      text,
				})
				if opts.Limit > 0 && len(results) >= opts.Limit {
					return results
				}
			}
		}
	}
	return results
}

// SearchRanked scores wiki pages using field-weighted ranking:
// title, page ID, tags, section headings, body, exact phrase hits,
// lifecycle trust rank, and unique query-term coverage.
func (e *Engine) SearchRanked(query string, opts SearchOptions) ([]RankedSearchResult, error) {
	if strings.TrimSpace(query) == "" {
		return nil, fmt.Errorf("query is empty")
	}
	snap, err := e.LoadSnapshot()
	if err != nil {
		return nil, err
	}
	if err := snap.FatalLoadError(); err != nil {
		return nil, err
	}
	return SearchRankedFromSnapshot(snap, query, opts), nil
}

// SearchRankedFromSnapshot runs field-weighted search directly against a loaded
// WikiSnapshot (enabling shared-snapshot workflows and benchmarks).
func SearchRankedFromSnapshot(snap *model.WikiSnapshot, query string, opts SearchOptions) []RankedSearchResult {
	lowerQ := strings.ToLower(strings.TrimSpace(query))
	terms := uniqueTerms(lowerQ)
	if len(terms) == 0 {
		return []RankedSearchResult{}
	}

	results := []RankedSearchResult{}
	for _, p := range snap.Pages {
		if !matchesSearchFilters(p, opts) {
			continue
		}

		titleLower := strings.ToLower(p.EffectiveTitle())
		idLower := strings.ToLower(p.ID)
		tagsLower := make([]string, len(p.Metadata.Tags))
		for i, t := range p.Metadata.Tags {
			tagsLower[i] = strings.ToLower(t)
		}

		matchedTermSet := make(map[string]bool)
		var score float64
		headingHits := 0
		bodyHits := 0
		bestSection := ""
		bestLine := 1
		bestLineScore := -1.0

		// Title, ID, and tag scoring.
		if strings.Contains(titleLower, lowerQ) {
			score += 12.0
		}
		if idLower == lowerQ || strings.Contains(idLower, lowerQ) {
			score += 10.0
		}
		for _, term := range terms {
			if strings.Contains(titleLower, term) {
				score += 8.0
				matchedTermSet[term] = true
			}
			if strings.Contains(idLower, term) {
				score += 7.0
				matchedTermSet[term] = true
			}
			for _, tag := range tagsLower {
				if tag == term || strings.Contains(tag, term) {
					score += 6.0
					matchedTermSet[term] = true
				}
			}
		}

		// Scan body lines tracking current section heading.
		currentSection := p.EffectiveTitle()
		fmEnd := p.BodyStartLine - 1
		for i, line := range p.Lines {
			lineNo := i + 1
			if p.FMState == model.FrontMatterPresent && lineNo <= fmEnd {
				continue
			}
			lower := strings.ToLower(line)
			isHeading := headingRe.MatchString(line)
			if isHeading {
				currentSection = strings.TrimSpace(strings.TrimLeft(line, "#"))
			}

			lineScore := 0.0
			if len(terms) > 1 && strings.Contains(lower, lowerQ) {
				lineScore += 5.0
			}
			for _, term := range terms {
				if strings.Contains(lower, term) {
					matchedTermSet[term] = true
					if isHeading {
						headingHits++
						lineScore += 4.0
					} else {
						bodyHits++
						lineScore += 1.0
					}
				}
			}
			if lineScore > 0 {
				score += lineScore
				if lineScore > bestLineScore {
					bestLineScore = lineScore
					bestLine = lineNo
					bestSection = currentSection
				}
			}
		}

		if score <= 0 || len(matchedTermSet) == 0 {
			continue
		}

		coverage := float64(len(matchedTermSet)) / float64(len(terms))
		// Reward unique query-term coverage and apply lifecycle trust multiplier.
		lc := p.EffectiveLifecycle()
		finalScore := score * (0.5 + 0.5*coverage) * schema.LifecycleTrustRank(lc)

		matchedTerms := make([]string, 0, len(matchedTermSet))
		for t := range matchedTermSet {
			matchedTerms = append(matchedTerms, t)
		}
		sort.Strings(matchedTerms)

		ctx := extractBoundedContext(p.Lines, bestLine, 2, 240)
		why := fmt.Sprintf("lifecycle=%s, coverage=%.0f%%, %d heading, %d body", lc, coverage*100, headingHits, bodyHits)
		if bestSection != "" {
			why += fmt.Sprintf(" under %s", bestSection)
		}

		results = append(results, RankedSearchResult{
			ID:           p.ID,
			File:         p.Path,
			Title:        p.EffectiveTitle(),
			Type:         p.Metadata.Type,
			Lifecycle:    lc,
			Score:        finalScore,
			MatchedTerms: matchedTerms,
			Coverage:     coverage,
			Section:      bestSection,
			StartLine:    bestLine,
			Context:      ctx,
			Why:          why,
		})
	}

	sort.Slice(results, func(i, j int) bool {
		if results[i].Score != results[j].Score {
			return results[i].Score > results[j].Score
		}
		return results[i].File < results[j].File
	})

	if opts.Limit > 0 && len(results) > opts.Limit {
		results = results[:opts.Limit]
	}
	return results
}

// Relevant is a compatibility alias for ranked search.
func (e *Engine) Relevant(query string, topN int) ([]RelevanceResult, error) {
	if topN <= 0 {
		topN = 5
	}
	return e.SearchRanked(query, SearchOptions{Limit: topN})
}

func matchesSearchFilters(p *model.Page, opts SearchOptions) bool {
	lc := p.EffectiveLifecycle()
	if opts.Lifecycle != "" {
		if lc != opts.Lifecycle {
			return false
		}
	} else if !opts.IncludeHistory {
		// Default search includes current, implemented, and wip.
		// Exclude planned, legacy, superseded, and pages with missing lifecycle.
		if !schema.IsSearchDefaultLifecycle(lc) {
			return false
		}
	}
	if opts.Type != "" && p.Metadata.Type != opts.Type {
		return false
	}
	if opts.Tag != "" {
		found := false
		for _, t := range p.Metadata.Tags {
			if strings.EqualFold(t, opts.Tag) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func uniqueTerms(lowerQuery string) []string {
	seen := make(map[string]bool)
	var out []string
	for _, f := range strings.Fields(lowerQuery) {
		f = strings.Trim(f, ".,;:!?()[]{}\"'`")
		if f == "" || seen[f] {
			continue
		}
		seen[f] = true
		out = append(out, f)
	}
	return out
}

func extractBoundedContext(lines []string, centerLine, radius, maxChars int) string {
	if len(lines) == 0 {
		return ""
	}
	start := centerLine - 1 - radius
	if start < 0 {
		start = 0
	}
	end := centerLine - 1 + radius + 1
	if end > len(lines) {
		end = len(lines)
	}
	snippet := strings.TrimSpace(strings.Join(lines[start:end], "\n"))
	if maxChars > 0 && len(snippet) > maxChars {
		snippet = snippet[:maxChars-3] + "..."
	}
	return snippet
}
