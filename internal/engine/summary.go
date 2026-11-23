package engine

import (
	"fmt"
	"strings"

	"github.com/ramayac/go-wiki-engine/internal/model"
	"github.com/ramayac/go-wiki-engine/internal/paths"
)

// SummaryResult holds a concise page preview.
type SummaryResult struct {
	ID          string `json:"id,omitempty"`
	File        string `json:"file"`
	Lifecycle   string `json:"lifecycle,omitempty"`
	FirstHeader string `json:"first_header"`
	FirstPara   string `json:"first_para"`
	Preview     string `json:"preview"` // first heading + up to 3 paragraphs
	LineCount   int    `json:"line_count"`
}

// Summary returns a preview of a single wiki page (accepting either a
// wiki-relative path, repo-relative path, or stable page ID) while enforcing
// strict wiki path containment.
func (e *Engine) Summary(pageRef string) (*SummaryResult, error) {
	wikiRoot, err := e.ResolveWikiPath()
	if err != nil {
		return nil, err
	}
	// Reject explicit path traversal attempts before snapshot lookup.
	if strings.Contains(pageRef, "/") || strings.Contains(pageRef, `\`) || strings.HasPrefix(pageRef, ".") || strings.HasSuffix(pageRef, ".md") {
		if _, cerr := paths.ResolveWithin(wikiRoot, pageRef); cerr != nil {
			// Allow repo-relative path prefixed with wikiDir/ if it stays within wikiRoot.
			stripped := strings.TrimPrefix(pageRef, e.Cfg.WikiDir+"/")
			if stripped == pageRef {
				return nil, fmt.Errorf("page not found: %s (%w)", pageRef, cerr)
			}
			if _, cerr2 := paths.ResolveWithin(wikiRoot, stripped); cerr2 != nil {
				return nil, fmt.Errorf("page not found: %s (%w)", pageRef, cerr2)
			}
		}
	}

	snap, err := e.LoadSnapshot()
	if err != nil {
		return nil, err
	}
	if err := snap.FatalLoadError(); err != nil {
		return nil, err
	}
	p, err := snap.Lookup(pageRef)
	if err != nil {
		return nil, err
	}
	return summarizePage(p, p.WikiRel), nil
}

func summarizePage(p *model.Page, displayFile string) *SummaryResult {
	sr := &SummaryResult{
		ID:        p.ID,
		File:      displayFile,
		Lifecycle: p.EffectiveLifecycle(),
		LineCount: p.LineCount,
	}

	paragraphCount := 0
	var previewLines []string
	var fence model.FenceTracker
	fmEnd := p.BodyStartLine - 1

	for i := 0; i < len(p.Lines); i++ {
		lineNo := i + 1
		if p.FMState == model.FrontMatterPresent && lineNo <= fmEnd {
			continue
		}
		text := p.Lines[i]
		trimmed := strings.TrimSpace(text)

		if fence.Skip(text) || fence.InFence() {
			continue
		}

		if sr.FirstHeader == "" && headingRe.MatchString(text) {
			sr.FirstHeader = text
			previewLines = append(previewLines, text)
			continue
		}

		if trimmed == "" || strings.HasPrefix(trimmed, "---") ||
			strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "|") ||
			(strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]")) {
			continue
		}

		if paragraphCount < 3 && !strings.HasPrefix(trimmed, ">") && !strings.HasPrefix(trimmed, "-") && !strings.HasPrefix(trimmed, "*") {
			if sr.FirstPara == "" {
				sr.FirstPara = text
			}
			previewLines = append(previewLines, text)
			paragraphCount++
			for i+1 < len(p.Lines) {
				i++
				if strings.TrimSpace(p.Lines[i]) == "" {
					break
				}
			}
		}
	}

	sr.Preview = strings.TrimSpace(strings.Join(previewLines, "\n"))
	return sr
}
