package engine

import (
	"path/filepath"
	"sort"
	"strings"
)

// ImpactOptions controls how source-to-wiki impact matching operates.
type ImpactOptions struct {
	Fuzzy bool // opt-in basename text matching for pages without exact citations
}

// ImpactMatch describes one matched wiki page and the exact reason it matched.
type ImpactMatch struct {
	Page          string `json:"page"`
	ID            string `json:"id,omitempty"`
	Reason        string `json:"reason"` // "source", "directory", "markdown_link", "fuzzy_basename"
	MatchedSource string `json:"matched_source,omitempty"`
}

// ImpactResult reports which wiki pages are impacted by a changed file.
type ImpactResult struct {
	ChangedFile string        `json:"changed_file"`
	WikiPages   []string      `json:"wiki_pages"`
	Matches     []ImpactMatch `json:"matches,omitempty"`
}

// Impact maps changed files to wiki pages using exact references (structured
// sources, directory coverage, and exact Markdown repo links). Basename text
// matching is opt-in via ImpactWithOptions(files, ImpactOptions{Fuzzy: true}).
func (e *Engine) Impact(changedFiles []string) ([]ImpactResult, error) {
	return e.ImpactWithOptions(changedFiles, ImpactOptions{Fuzzy: false})
}

// ImpactWithOptions maps changed files to impacted wiki pages using one
// WikiSnapshot and reports the match reason for every hit.
func (e *Engine) ImpactWithOptions(changedFiles []string, opts ImpactOptions) ([]ImpactResult, error) {
	snap, err := e.LoadSnapshot()
	if err != nil {
		return nil, err
	}
	if err := snap.FatalLoadError(); err != nil {
		return nil, err
	}

	type hitMap map[string]ImpactMatch // pagePath -> best match
	byChanged := make(map[string]hitMap, len(changedFiles))
	for _, cf := range changedFiles {
		byChanged[cf] = make(hitMap)
	}

	record := func(cf string, m ImpactMatch) {
		hm := byChanged[cf]
		if existing, ok := hm[m.Page]; ok {
			if impactReasonRank(m.Reason) >= impactReasonRank(existing.Reason) {
				return
			}
		}
		hm[m.Page] = m
	}

	for _, p := range snap.Pages {
		hasDeclaredSources := len(p.ResolvedSources) > 0

		for _, cf := range changedFiles {
			cfClean := filepath.ToSlash(filepath.Clean(cf))

			// 1. Exact declared sources (and directory source coverage).
			// Cross-repo sources are skipped automatically by MatchesFile.
			matchedBySource := false
			for _, rs := range p.ResolvedSources {
				if reason, ok := rs.MatchesFile(cfClean); ok {
					record(cf, ImpactMatch{
						Page:          p.Path,
						ID:            p.ID,
						Reason:        reason,
						MatchedSource: rs.RepoRel,
					})
					matchedBySource = true
				}
			}
			if matchedBySource {
				continue
			}

			// 2. Exact Markdown links to repository files.
			matchedByLink := false
			for _, lk := range p.Links {
				if lk.IsExternal || lk.IsAnchorOnly || lk.EscapesRepo || lk.RepoRel == "" {
					continue
				}
				linkClean := filepath.ToSlash(filepath.Clean(lk.RepoRel))
				if linkClean == cfClean {
					record(cf, ImpactMatch{
						Page:          p.Path,
						ID:            p.ID,
						Reason:        "markdown_link",
						MatchedSource: linkClean,
					})
					matchedByLink = true
					break
				}
			}
			if matchedByLink {
				continue
			}

			// 3. Opt-in fuzzy basename scan (only behind --fuzzy, and never for pages
			// that already declare exact sources).
			if opts.Fuzzy && !hasDeclaredSources {
				base := filepath.Base(cfClean)
				if base != "" && strings.Contains(strings.ToLower(p.RawContent), strings.ToLower(base)) {
					record(cf, ImpactMatch{
						Page:          p.Path,
						ID:            p.ID,
						Reason:        "fuzzy_basename",
						MatchedSource: base,
					})
				}
			}
		}
	}

	results := make([]ImpactResult, 0, len(changedFiles))
	for _, cf := range changedFiles {
		hm := byChanged[cf]
		var matches []ImpactMatch
		var pages []string
		for _, m := range hm {
			matches = append(matches, m)
			pages = append(pages, m.Page)
		}
		sort.Strings(pages)
		sort.Slice(matches, func(i, j int) bool {
			return matches[i].Page < matches[j].Page
		})
		results = append(results, ImpactResult{
			ChangedFile: cf,
			WikiPages:   pages,
			Matches:     matches,
		})
	}
	return results, nil
}

func impactReasonRank(reason string) int {
	switch reason {
	case "source":
		return 0
	case "directory":
		return 1
	case "markdown_link":
		return 2
	case "fuzzy_basename":
		return 3
	default:
		return 9
	}
}
