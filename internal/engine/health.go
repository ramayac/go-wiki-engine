package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/ramayac/go-wiki-engine/internal/git"
	"github.com/ramayac/go-wiki-engine/internal/model"
	"github.com/ramayac/go-wiki-engine/internal/schema"
)

// HealthSchemaVersion is the JSON schema version of the health payload.
const HealthSchemaVersion = "1"

const (
	healthLargestPageCount = 5
	healthAgingWorkDays    = 90
	healthLogRotateBytes   = 128 * 1024
	healthGitDriftDays     = 7
	healthDateLayout       = "2006-01-02"
)

// unsetLifecycle is the Lifecycles key for pages without a lifecycle.
const unsetLifecycle = "unset"

// HealthPage describes one page in the largest-pages signal.
type HealthPage struct {
	ID        string `json:"id"`
	Path      string `json:"path"`
	Title     string `json:"title"`
	Lifecycle string `json:"lifecycle"`
	Bytes     int    `json:"bytes"`
	Words     int    `json:"words"`
}

// HealthAging describes a planned or wip page that is older than the threshold.
type HealthAging struct {
	Path      string `json:"path"`
	Lifecycle string `json:"lifecycle"`
	Updated   string `json:"updated"`
	AgeDays   int    `json:"age_days"`
}

// HealthDrift describes a page whose `updated` field lags its last git commit.
type HealthDrift struct {
	Path       string `json:"path"`
	Updated    string `json:"updated"`
	LastCommit string `json:"last_commit"`
	DriftDays  int    `json:"drift_days"`
}

// HealthDuplicateTitle groups pages that share one normalized title.
type HealthDuplicateTitle struct {
	Title string   `json:"title"`
	Paths []string `json:"paths"`
}

// HealthMarker counts the open markers on one live page: CONTRADICTION flags
// and "Rationale: not recorded." markers (see model.CountMarkers).
type HealthMarker struct {
	Path                 string `json:"path"`
	Contradictions       int    `json:"contradictions"`
	RationaleNotRecorded int    `json:"rationale_not_recorded"`
}

// HealthThresholds lists the numeric thresholds behind the health signals, so a
// consumer can interpret them.
type HealthThresholds struct {
	AgingDays        int `json:"aging_days"`
	DriftDays        int `json:"drift_days"`
	LogRotationBytes int `json:"log_rotation_bytes"`
}

// HealthResult holds the advisory maintenance signals of `wiki-engine health`.
// Every signal is advisory: none of them is a lint failure.
type HealthResult struct {
	SchemaVersion    string                 `json:"schema_version"`
	Pages            int                    `json:"pages"`
	WikiBytes        int                    `json:"wiki_bytes"`
	IndexBytes       int                    `json:"index_bytes"`
	LogBytes         int                    `json:"log_bytes"`
	LogNeedsRotation bool                   `json:"log_needs_rotation"`
	GraphNodes       int                    `json:"graph_nodes"`
	GraphEdges       int                    `json:"graph_edges"`
	Lifecycles       map[string]int         `json:"lifecycles"`
	LargestPages     []HealthPage           `json:"largest_pages"`
	AgingWork        []HealthAging          `json:"aging_work"`
	UpdatedBehindGit []HealthDrift          `json:"updated_behind_git"`
	DuplicateTitles  []HealthDuplicateTitle `json:"duplicate_titles"`
	// IndexOnlyPages lists live pages that no other page links to, so only
	// index.md reaches them. Leaf pages are normal; the list is a prompt to
	// consider cross-links, not a defect report.
	IndexOnlyPages []string `json:"index_only_pages"`
	// OpenMarkers lists live pages that still carry a CONTRADICTION flag or a
	// missing-rationale marker. It is a prompt to resolve them, not a lint
	// failure.
	OpenMarkers []HealthMarker `json:"open_markers"`
	// Thresholds lists the limits that decide aging_work, updated_behind_git,
	// and log_needs_rotation.
	Thresholds HealthThresholds `json:"thresholds"`
}

// Health computes the advisory maintenance signals from one snapshot.
func (e *Engine) Health() (*HealthResult, error) {
	return e.healthAt(time.Now())
}

func (e *Engine) healthAt(now time.Time) (*HealthResult, error) {
	snap, err := e.LoadSnapshot()
	if err != nil {
		return nil, err
	}
	if err := snap.FatalLoadError(); err != nil {
		return nil, err
	}

	res := &HealthResult{
		SchemaVersion:    HealthSchemaVersion,
		Pages:            len(snap.Pages),
		Lifecycles:       map[string]int{unsetLifecycle: 0},
		LargestPages:     []HealthPage{},
		AgingWork:        []HealthAging{},
		UpdatedBehindGit: []HealthDrift{},
		DuplicateTitles:  []HealthDuplicateTitle{},
		IndexOnlyPages:   []string{},
		OpenMarkers:      []HealthMarker{},
		Thresholds: HealthThresholds{
			AgingDays:        healthAgingWorkDays,
			DriftDays:        healthGitDriftDays,
			LogRotationBytes: healthLogRotateBytes,
		},
	}
	for _, lc := range schema.ValidLifecycles {
		res.Lifecycles[lc] = 0
	}

	// One git call dates every page. A wiki outside git gives no drift data, and
	// so does a shallow clone, whose commit dates are truncated. Neither is an
	// error.
	var commitDates map[string]time.Time
	if !git.IsShallow(snap.Root) {
		commitDates, _ = git.PageCommitDates(snap.Root, snap.WikiDir)
	}
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	titlePaths := map[string][]string{}

	for _, p := range snap.Pages {
		lifecycle := p.EffectiveLifecycle()
		if schema.IsValidLifecycle(lifecycle) {
			res.Lifecycles[lifecycle]++
		} else {
			res.Lifecycles[unsetLifecycle]++
		}

		size := len(p.RawContent)
		if info, err := os.Stat(p.AbsPath); err == nil {
			size = int(info.Size())
		}
		res.LargestPages = append(res.LargestPages, HealthPage{
			ID:        p.ID,
			Path:      p.Path,
			Title:     p.EffectiveTitle(),
			Lifecycle: lifecycle,
			Bytes:     size,
			Words:     len(strings.Fields(p.RawContent)),
		})

		updated, updatedErr := time.Parse(healthDateLayout, p.Metadata.Updated)
		if updatedErr == nil && (lifecycle == "planned" || lifecycle == "wip") {
			if age := int(today.Sub(updated).Hours() / 24); age > healthAgingWorkDays {
				res.AgingWork = append(res.AgingWork, HealthAging{Path: p.Path, Lifecycle: lifecycle, Updated: p.Metadata.Updated, AgeDays: age})
			}
		}
		// Drift asks "did the content move without the author refreshing the
		// knowledge". Implemented, legacy, and superseded pages are fixed
		// records, so they are left out, as in the stale-content check.
		if updatedErr == nil && p.Metadata.IsDriftChecked() {
			if commit, ok := commitDates[p.Path]; ok {
				if drift := int(commit.Sub(updated).Hours() / 24); drift > healthGitDriftDays {
					res.UpdatedBehindGit = append(res.UpdatedBehindGit, HealthDrift{
						Path:       p.Path,
						Updated:    p.Metadata.Updated,
						LastCommit: commit.Format(healthDateLayout),
						DriftDays:  drift,
					})
				}
			}
		}
		if p.IsActive() {
			if c, r := model.CountMarkers(p.Body); c+r > 0 {
				res.OpenMarkers = append(res.OpenMarkers, HealthMarker{Path: p.Path, Contradictions: c, RationaleNotRecorded: r})
			}
		}
		if key := normalizeTitle(pageTitle(p)); key != "" {
			titlePaths[key] = append(titlePaths[key], p.Path)
		}
	}

	if snap.Index != nil {
		res.IndexBytes = fileSize(snap.Index.AbsPath)
	}
	if snap.Log != nil {
		res.LogBytes = fileSize(snap.Log.AbsPath)
		res.LogNeedsRotation = res.LogBytes >= healthLogRotateBytes
	}
	for _, rel := range snap.AllFiles {
		res.WikiBytes += fileSize(filepath.Join(snap.Root, filepath.FromSlash(rel)))
	}

	// Without index.md there is no graph; the graph signals stay at zero.
	if view, err := model.BuildGraph(snap, model.GraphOptions{}); err == nil {
		res.GraphNodes = len(view.Nodes)
		res.GraphEdges = len(view.Edges)
		res.IndexOnlyPages = indexOnlyPages(snap, view)
	}

	for title, paths := range titlePaths {
		if len(paths) > 1 {
			sort.Strings(paths)
			res.DuplicateTitles = append(res.DuplicateTitles, HealthDuplicateTitle{Title: title, Paths: paths})
		}
	}
	sort.Slice(res.DuplicateTitles, func(i, j int) bool { return res.DuplicateTitles[i].Title < res.DuplicateTitles[j].Title })
	sort.Slice(res.OpenMarkers, func(i, j int) bool { return res.OpenMarkers[i].Path < res.OpenMarkers[j].Path })
	sort.Slice(res.LargestPages, func(i, j int) bool {
		if res.LargestPages[i].Bytes != res.LargestPages[j].Bytes {
			return res.LargestPages[i].Bytes > res.LargestPages[j].Bytes
		}
		return res.LargestPages[i].Path < res.LargestPages[j].Path
	})
	if len(res.LargestPages) > healthLargestPageCount {
		res.LargestPages = res.LargestPages[:healthLargestPageCount]
	}
	sort.Slice(res.AgingWork, func(i, j int) bool {
		if res.AgingWork[i].AgeDays != res.AgingWork[j].AgeDays {
			return res.AgingWork[i].AgeDays > res.AgingWork[j].AgeDays
		}
		return res.AgingWork[i].Path < res.AgingWork[j].Path
	})
	sort.Slice(res.UpdatedBehindGit, func(i, j int) bool {
		if res.UpdatedBehindGit[i].DriftDays != res.UpdatedBehindGit[j].DriftDays {
			return res.UpdatedBehindGit[i].DriftDays > res.UpdatedBehindGit[j].DriftDays
		}
		return res.UpdatedBehindGit[i].Path < res.UpdatedBehindGit[j].Path
	})
	return res, nil
}

// indexOnlyPages returns the live pages of the active graph whose only inbound
// link comes from index.md. index.md itself and log.md, the documented leaf,
// are left out.
func indexOnlyPages(snap *model.WikiSnapshot, view *model.GraphView) []string {
	inbound := map[string]int{}
	for _, edge := range view.Edges {
		if edge.From != "index.md" && edge.From != edge.To {
			inbound[edge.To]++
		}
	}
	out := []string{}
	for _, n := range view.Nodes {
		if n.File == "index.md" || model.IsCanonicalFile("log.md", n.File) || inbound[n.File] > 0 {
			continue
		}
		if p, ok := snap.ByPath[n.File]; ok {
			out = append(out, p.Path)
		}
	}
	sort.Strings(out)
	return out
}

func fileSize(path string) int {
	if info, err := os.Stat(path); err == nil {
		return int(info.Size())
	}
	return 0
}

// pageTitle returns the frontmatter title, else the first H1, else "".
func pageTitle(p *model.Page) string {
	if p.Metadata.Title != "" {
		return p.Metadata.Title
	}
	for _, h := range p.Headings {
		if h.Level == 1 {
			return h.Title
		}
	}
	return ""
}

// normalizeTitle lowercases a title and joins its letter and digit runs with
// single spaces, so case and punctuation do not hide a duplicate.
func normalizeTitle(title string) string {
	return strings.Join(strings.FieldsFunc(strings.ToLower(title), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	}), " ")
}

// Text renders the health signals as concise human-readable text.
func (r *HealthResult) Text() string {
	var b strings.Builder
	fmt.Fprintf(&b, "Wiki health: %d pages, %d bytes, %d graph nodes, %d graph edges\n", r.Pages, r.WikiBytes, r.GraphNodes, r.GraphEdges)
	fmt.Fprintf(&b, "Index: %d bytes   Log: %d bytes\n", r.IndexBytes, r.LogBytes)
	b.WriteString("Lifecycle:")
	for _, lc := range schema.ValidLifecycles {
		fmt.Fprintf(&b, " %s=%d", lc, r.Lifecycles[lc])
	}
	if n := r.Lifecycles[unsetLifecycle]; n > 0 {
		fmt.Fprintf(&b, " %s=%d", unsetLifecycle, n)
	}
	b.WriteString("\n")
	if len(r.DuplicateTitles) > 0 {
		fmt.Fprintf(&b, "Duplicate normalized titles: %d\n", len(r.DuplicateTitles))
		for _, d := range r.DuplicateTitles {
			fmt.Fprintf(&b, "  %s: %s\n", d.Title, strings.Join(d.Paths, ", "))
		}
	}
	if len(r.AgingWork) > 0 {
		fmt.Fprintf(&b, "Planned/WIP pages older than %d days: %d\n", healthAgingWorkDays, len(r.AgingWork))
		for _, a := range r.AgingWork {
			fmt.Fprintf(&b, "  %s: %d days\n", a.Path, a.AgeDays)
		}
	}
	if len(r.UpdatedBehindGit) > 0 {
		fmt.Fprintf(&b, "Pages whose 'updated' lags git history by more than %d days: %d\n", healthGitDriftDays, len(r.UpdatedBehindGit))
		for _, d := range r.UpdatedBehindGit {
			fmt.Fprintf(&b, "  %s: updated %s, last commit %s (%d days)\n", d.Path, d.Updated, d.LastCommit, d.DriftDays)
		}
	}
	if r.LogNeedsRotation {
		fmt.Fprintf(&b, "Log exceeds rotation threshold: %d bytes\n", healthLogRotateBytes)
	}
	if len(r.IndexOnlyPages) > 0 {
		fmt.Fprintf(&b, "Live pages linked only from index.md: %d\n", len(r.IndexOnlyPages))
		for _, p := range r.IndexOnlyPages {
			fmt.Fprintf(&b, "  %s\n", p)
		}
	}
	if len(r.OpenMarkers) > 0 {
		fmt.Fprintf(&b, "Pages with open markers: %d\n", len(r.OpenMarkers))
		for _, m := range r.OpenMarkers {
			fmt.Fprintf(&b, "  %s: %d contradiction(s), %d rationale not recorded\n", m.Path, m.Contradictions, m.RationaleNotRecorded)
		}
	}
	b.WriteString("Largest pages:\n")
	for _, p := range r.LargestPages {
		fmt.Fprintf(&b, "  %s: %d bytes, %d words\n", p.Path, p.Bytes, p.Words)
	}
	return b.String()
}
