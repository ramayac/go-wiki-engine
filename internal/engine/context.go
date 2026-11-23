package engine

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/ramayac/go-wiki-engine/internal/model"
)

// ContextEntry is a single entry in the context catalog.
type ContextEntry struct {
	ID          string `json:"id,omitempty"`
	File        string `json:"file"`
	Type        string `json:"type,omitempty"`
	Lifecycle   string `json:"lifecycle,omitempty"`
	Status      string `json:"status"`
	Description string `json:"description"`
	Summary     string `json:"summary,omitempty"`
	LineCount   int    `json:"line_count,omitempty"`
}

// ContextOptions configures bounded context generation.
type ContextOptions struct {
	Minimal   bool
	Summarize bool
	MaxPages  int // 0 means use config default (or unlimited if <=0)
	MaxBytes  int // 0 means use config default (or unlimited if <=0)
}

// ContextResult holds a condensed, optionally bounded wiki snapshot for agent loading.
type ContextResult struct {
	Files           int            `json:"files"`
	LastUpdated     string         `json:"last_updated"`
	Phase           string         `json:"phase"`
	Catalog         []ContextEntry `json:"catalog"`
	RecentLog       []string       `json:"recent_log"`
	HeadingCount    int            `json:"heading_count"`
	Summarized      bool           `json:"summarized"`
	TotalCatalog    int            `json:"total_catalog,omitempty"`
	Truncated       bool           `json:"truncated,omitempty"`
	TruncatedReason string         `json:"truncated_reason,omitempty"`
}

// Context returns a condensed snapshot of the wiki using one WikiSnapshot.
func (e *Engine) Context(minimal, summarize bool) (*ContextResult, error) {
	return e.ContextWithOptions(ContextOptions{
		Minimal:   minimal,
		Summarize: summarize,
		MaxPages:  e.Cfg.ContextMaxPages,
		MaxBytes:  e.Cfg.ContextMaxBytes,
	})
}

// ContextWithOptions builds a condensed, bounded context snapshot from a single
// WikiSnapshot, reporting truncation when page or byte limits are reached.
func (e *Engine) ContextWithOptions(opts ContextOptions) (*ContextResult, error) {
	snap, err := e.LoadSnapshot()
	if err != nil {
		return nil, err
	}
	if err := snap.FatalLoadError(); err != nil {
		return nil, err
	}

	cr := &ContextResult{}
	var rawCatalog []ContextEntry
	if snap.Index != nil {
		rawCatalog = parseIndexCatalog(snap.Index.RawContent)
	}

	var filteredCatalog []ContextEntry
	for _, entry := range rawCatalog {
		cleanFile := filepath.ToSlash(filepath.Clean(entry.File))
		p, ok := snap.ByPath[cleanFile]
		if ok {
			lc := p.EffectiveLifecycle()
			entry.ID = p.ID
			entry.Type = p.Metadata.Type
			entry.Lifecycle = lc
			entry.Status = lc
			if entry.Status == "" {
				entry.Status = p.Metadata.Status
			}
			if opts.Summarize && !p.IsActive() {
				continue
			}
		} else if opts.Summarize {
			continue
		}
		filteredCatalog = append(filteredCatalog, entry)
	}

	cr.TotalCatalog = len(filteredCatalog)
	maxPages := opts.MaxPages
	maxBytes := opts.MaxBytes

	st := statsFromSnapshot(snap)
	cr.Files = st.Files
	cr.LastUpdated = st.LastUpdated
	cr.HeadingCount = st.Headings

	if tail, err := logTailFromSnapshot(snap, 3); err == nil {
		cr.RecentLog = tail
	}

	if !opts.Minimal {
		cr.Phase = currentPhaseFromSnapshot(snap)
	}

	if opts.Summarize {
		cr.Summarized = true
	}

	usedBytes := 0
	for _, entry := range filteredCatalog {
		if maxPages > 0 && len(cr.Catalog) >= maxPages {
			cr.Truncated = true
			cr.TruncatedReason = fmt.Sprintf("catalog truncated at %d of %d pages (max_pages=%d)", len(cr.Catalog), cr.TotalCatalog, maxPages)
			break
		}
		if opts.Summarize {
			cleanFile := filepath.ToSlash(filepath.Clean(entry.File))
			if p, ok := snap.ByPath[cleanFile]; ok {
				sr := summarizePage(p, entry.File)
				entry.Summary = pageSummaryText(sr)
				entry.LineCount = sr.LineCount
			}
		}
		entryBytes := len(entry.File) + len(entry.Description) + len(entry.Summary) + 64
		if maxBytes > 0 && usedBytes+entryBytes > maxBytes && len(cr.Catalog) > 0 {
			cr.Truncated = true
			cr.TruncatedReason = fmt.Sprintf("catalog truncated at %d bytes (%d of %d pages, max_bytes=%d)", usedBytes, len(cr.Catalog), cr.TotalCatalog, maxBytes)
			break
		}
		usedBytes += entryBytes
		cr.Catalog = append(cr.Catalog, entry)
	}

	return cr, nil
}

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

var (
	indexCatalogWithDescRe = regexp.MustCompile(`-\s+\[([^\]]+)\]\(([^)]+)\)\s*\|\s*(.+)`)
	indexCatalogSimpleRe   = regexp.MustCompile(`-\s+\[([^\]]+)\]\(([^)]+)\)`)
)

func parseIndexCatalog(content string) []ContextEntry {
	var entries []ContextEntry
	for _, match := range indexCatalogWithDescRe.FindAllStringSubmatch(content, -1) {
		if len(match) >= 4 {
			entries = append(entries, ContextEntry{
				File:        match[2],
				Description: strings.TrimSpace(match[3]),
			})
		}
	}
	seen := make(map[string]bool)
	for _, e := range entries {
		seen[e.File] = true
	}
	for _, match := range indexCatalogSimpleRe.FindAllStringSubmatch(content, -1) {
		if len(match) >= 3 && !seen[match[2]] {
			entries = append(entries, ContextEntry{
				File:        match[2],
				Description: "",
			})
		}
	}
	return entries
}

var phaseRowRe = regexp.MustCompile(`^\|\s*(\d+)\s*\|\s*(.+?)\s*\|\s*(\S+)\s*\|`)

func (e *Engine) currentPhase() string {
	snap, err := e.LoadSnapshot()
	if err != nil {
		return "unknown"
	}
	return currentPhaseFromSnapshot(snap)
}

func currentPhaseFromSnapshot(snap *model.WikiSnapshot) string {
	if snap.Phases == nil {
		return "unknown"
	}
	var last, lastInProgress, lastCompleted string
	for _, line := range snap.Phases.Lines {
		m := phaseRowRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		entry := fmt.Sprintf("Phase %s: %s — %s", m[1], strings.TrimSpace(m[2]), strings.TrimSpace(m[3]))
		last = entry
		switch strings.TrimSpace(m[3]) {
		case "in-progress":
			lastInProgress = entry
		case "completed":
			lastCompleted = entry
		}
	}
	switch {
	case lastInProgress != "":
		return lastInProgress
	case lastCompleted != "":
		return lastCompleted
	case last != "":
		return last
	default:
		return "unknown"
	}
}
