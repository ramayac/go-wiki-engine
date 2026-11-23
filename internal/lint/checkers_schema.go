package lint

import (
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/ramayac/go-wiki-engine/internal/config"
	"github.com/ramayac/go-wiki-engine/internal/model"
	"github.com/ramayac/go-wiki-engine/internal/schema"
)

// --- pathContainmentChecker ---

type pathContainmentChecker struct{}

func (c *pathContainmentChecker) Name() string       { return "path-containment" }
func (c *pathContainmentChecker) Group() string      { return "security" }
func (c *pathContainmentChecker) Category() Category { return CategorySnapshot }

func (c *pathContainmentChecker) Check(snap *model.WikiSnapshot, _ *config.Config) ([]Issue, error) {
	var issues []Issue
	for _, li := range snap.LoadIssues {
		issues = append(issues, Issue{
			Severity: SevError,
			Check:    c.Name(),
			File:     li.Path,
			Message:  fmt.Sprintf("%s error: %s", li.Kind, li.Message),
		})
	}
	for _, p := range snap.Pages {
		for _, lk := range p.Links {
			if lk.EscapesRepo {
				issues = append(issues, Issue{
					Severity: SevError,
					Check:    c.Name(),
					File:     p.Path,
					Line:     lk.Line,
					Message:  fmt.Sprintf("link escapes repository boundary: %s", lk.RawTarget),
				})
			}
		}
		for _, rs := range p.ResolvedSources {
			if rs.Err != nil {
				issues = append(issues, Issue{
					Severity: SevError,
					Check:    c.Name(),
					File:     p.Path,
					Line:     p.Metadata.FieldLines["sources"],
					Message:  fmt.Sprintf("source path escapes repository boundary: %s (%v)", rs.Raw.File, rs.Err),
				})
			}
		}
	}
	return issues, nil
}

// --- requiredFilesChecker ---

type requiredFile struct {
	candidates []string
	preferred  string
}

var requiredFiles = []requiredFile{
	{[]string{"README.md"}, "README.md"},
	{[]string{"index.md"}, "index.md"},
	{model.CanonicalPathCandidates["log.md"], "prologue/log.md"},
	{model.CanonicalPathCandidates["schema.md"], "prologue/schema.md"},
	{model.CanonicalPathCandidates["phases.md"], "prologue/phases.md"},
	{model.CanonicalPathCandidates["repo-map.md"], "prologue/repo-map.md"},
	{[]string{"operations/ingest.md"}, "operations/ingest.md"},
	{[]string{"operations/query.md"}, "operations/query.md"},
	{[]string{"operations/lint.md"}, "operations/lint.md"},
}

type requiredFilesChecker struct{}

func (c *requiredFilesChecker) Name() string       { return "required-files" }
func (c *requiredFilesChecker) Group() string      { return "schema" }
func (c *requiredFilesChecker) Category() Category { return CategorySnapshot }

func (c *requiredFilesChecker) Check(snap *model.WikiSnapshot, cfg *config.Config) ([]Issue, error) {
	var issues []Issue
	for _, req := range requiredFiles {
		found := false
		for _, cand := range req.candidates {
			if _, ok := snap.ByPath[cand]; ok {
				found = true
				break
			}
		}
		if !found {
			issues = append(issues, Issue{
				Severity: SevError,
				Check:    c.Name(),
				File:     filepath.ToSlash(filepath.Join(cfg.WikiDir, req.preferred)),
				Message:  fmt.Sprintf("missing required wiki file: %s/%s", cfg.WikiDir, req.preferred),
			})
		}
	}
	return issues, nil
}

// --- frontMatterChecker ---

type frontMatterChecker struct{}

func (c *frontMatterChecker) Name() string       { return "front-matter" }
func (c *frontMatterChecker) Group() string      { return "schema" }
func (c *frontMatterChecker) Category() Category { return CategorySnapshot }

func (c *frontMatterChecker) Check(snap *model.WikiSnapshot, _ *config.Config) ([]Issue, error) {
	var issues []Issue
	for _, p := range snap.Pages {
		if p.FMErr != nil {
			line := 1
			if fmErr, ok := p.FMErr.(*model.FrontMatterError); ok && fmErr.Line > 0 {
				line = fmErr.Line
			}
			issues = append(issues, Issue{
				Severity: SevError,
				Check:    c.Name(),
				File:     p.Path,
				Line:     line,
				Message:  fmt.Sprintf("malformed front matter: %v", p.FMErr),
			})
			continue
		}
		if p.FMState == model.FrontMatterMissing {
			issues = append(issues, Issue{
				Severity: SevWarn,
				Check:    c.Name(),
				File:     p.Path,
				Line:     1,
				Message:  "missing front matter block",
			})
			continue
		}

		m := p.Metadata
		lc := m.EffectiveLifecycle()

		// Validate lifecycle / legacy status.
		if m.Lifecycle != "" {
			if !schema.IsValidLifecycle(m.Lifecycle) {
				issues = append(issues, Issue{
					Severity: SevError,
					Check:    c.Name(),
					File:     p.Path,
					Line:     lineOrOne(m.FieldLines["lifecycle"]),
					Message:  fmt.Sprintf("invalid lifecycle value in front matter: %s (must be one of: %s)", m.Lifecycle, strings.Join(schema.ValidLifecycles, ", ")),
				})
			}
		} else if m.LegacyStatus {
			// Legacy v1 status mapped cleanly; emit an advisory notice so migration is visible.
			issues = append(issues, Issue{
				Severity: SevInfo,
				Check:    c.Name(),
				File:     p.Path,
				Line:     lineOrOne(m.FieldLines["status"]),
				Message:  fmt.Sprintf("legacy front matter field status: %s (migrate to lifecycle: %s via wiki-engine lint --fix)", m.Status, lc),
			})
		} else if m.Status != "" && m.Type != "decision" && m.Type != "incident" {
			issues = append(issues, Issue{
				Severity: SevError,
				Check:    c.Name(),
				File:     p.Path,
				Line:     lineOrOne(m.FieldLines["status"]),
				Message:  fmt.Sprintf("invalid status value in front matter: %s (must be one of: %s)", m.Status, strings.Join(schema.ValidLifecycles, ", ")),
			})
		} else {
			issues = append(issues, Issue{
				Severity: SevError,
				Check:    c.Name(),
				File:     p.Path,
				Line:     1,
				Message:  "missing lifecycle field in front matter (missing metadata cannot default to active)",
			})
		}

		if m.Description == "" {
			issues = append(issues, Issue{
				Severity: SevWarn,
				Check:    c.Name(),
				File:     p.Path,
				Line:     1,
				Message:  "missing recommended description field in front matter",
			})
		}

		// Superseded / deprecated pages must declare superseded_by pointing to an active page.
		if (lc == "superseded" || m.Status == "deprecated") && m.SupersededBy == "" {
			issues = append(issues, Issue{
				Severity: SevError,
				Check:    c.Name(),
				File:     p.Path,
				Line:     lineOrOne(m.FieldLines["lifecycle"]),
				Message:  "superseded_by is required when lifecycle is superseded (or status is deprecated)",
			})
		}

		if m.SupersededBy != "" {
			target := resolveSupersessionTarget(snap, p, m.SupersededBy)
			if target == nil {
				issues = append(issues, Issue{
					Severity: SevError,
					Check:    c.Name(),
					File:     p.Path,
					Line:     lineOrOne(m.FieldLines["superseded_by"]),
					Message:  fmt.Sprintf("superseded_by target does not exist: %s", m.SupersededBy),
				})
			} else if !target.IsActive() {
				issues = append(issues, Issue{
					Severity: SevError,
					Check:    c.Name(),
					File:     p.Path,
					Line:     lineOrOne(m.FieldLines["superseded_by"]),
					Message:  fmt.Sprintf("superseded_by target %s is not active (lifecycle: %s)", m.SupersededBy, target.EffectiveLifecycle()),
				})
			}
		}
	}
	return issues, nil
}

func resolveSupersessionTarget(snap *model.WikiSnapshot, from *model.Page, ref string) *model.Page {
	// 1. Try page-relative path inside wiki.
	pageDir := filepath.ToSlash(filepath.Dir(from.WikiRel))
	if pageDir == "." {
		pageDir = ""
	}
	cand := filepath.ToSlash(filepath.Clean(filepath.Join(pageDir, ref)))
	if p, ok := snap.ByPath[cand]; ok {
		return p
	}
	// 2. Try wiki-root-relative path or page ID via snapshot Lookup.
	if p, err := snap.Lookup(ref); err == nil {
		return p
	}
	return nil
}

// --- pageIDsChecker ---

type pageIDsChecker struct{}

func (c *pageIDsChecker) Name() string       { return "page-ids" }
func (c *pageIDsChecker) Group() string      { return "schema" }
func (c *pageIDsChecker) Category() Category { return CategorySnapshot }

func (c *pageIDsChecker) Check(snap *model.WikiSnapshot, _ *config.Config) ([]Issue, error) {
	var issues []Issue

	// Check global uniqueness of IDs across the wiki.
	for id, pages := range snap.ByIDMulti {
		if len(pages) > 1 {
			others := make([]string, len(pages))
			for i, p := range pages {
				others[i] = p.WikiRel
			}
			for _, p := range pages {
				issues = append(issues, Issue{
					Severity: SevError,
					Check:    c.Name(),
					File:     p.Path,
					Line:     lineOrOne(p.Metadata.FieldLines["id"]),
					Message:  fmt.Sprintf("duplicate page id %q shared by: %s", id, strings.Join(others, ", ")),
				})
			}
		}
	}

	for _, p := range snap.Pages {
		if p.FMState != model.FrontMatterPresent || p.FMErr != nil {
			continue
		}
		expected := model.ExpectedID(p.WikiRel)
		if p.ExplicitID {
			if p.Metadata.ID == "index" && p.WikiRel != "index.md" {
				issues = append(issues, Issue{
					Severity: SevError,
					Check:    c.Name(),
					File:     p.Path,
					Line:     lineOrOne(p.Metadata.FieldLines["id"]),
					Message:  `reserved page id "index" may only be used by root index.md`,
				})
			} else if p.Metadata.ID != expected {
				issues = append(issues, Issue{
					Severity: SevError,
					Check:    c.Name(),
					File:     p.Path,
					Line:     lineOrOne(p.Metadata.FieldLines["id"]),
					Message:  fmt.Sprintf("page id %q does not match filename stem %q", p.Metadata.ID, expected),
				})
			}
		} else {
			issues = append(issues, Issue{
				Severity: SevInfo,
				Check:    c.Name(),
				File:     p.Path,
				Line:     1,
				Message:  fmt.Sprintf("missing explicit id field in front matter (expected: %s; run wiki-engine lint --fix)", expected),
			})
		}
	}
	return issues, nil
}

// --- pageTypesChecker ---

type pageTypesChecker struct{}

func (c *pageTypesChecker) Name() string       { return "page-types" }
func (c *pageTypesChecker) Group() string      { return "schema" }
func (c *pageTypesChecker) Category() Category { return CategorySnapshot }

func (c *pageTypesChecker) Check(snap *model.WikiSnapshot, _ *config.Config) ([]Issue, error) {
	var issues []Issue
	for _, p := range snap.Pages {
		if p.FMState != model.FrontMatterPresent || p.FMErr != nil {
			continue
		}
		m := p.Metadata
		if m.Type == "" {
			continue // additive: untyped pages remain readable
		}
		if !schema.IsValidPageType(m.Type) {
			issues = append(issues, Issue{
				Severity: SevError,
				Check:    c.Name(),
				File:     p.Path,
				Line:     lineOrOne(m.FieldLines["type"]),
				Message:  fmt.Sprintf("invalid page type %q (must be one of: %s)", m.Type, strings.Join(schema.ValidPageTypes, ", ")),
			})
			continue
		}

		switch m.Type {
		case "decision":
			if m.Status == "" {
				issues = append(issues, Issue{
					Severity: SevError,
					Check:    c.Name(),
					File:     p.Path,
					Line:     lineOrOne(m.FieldLines["type"]),
					Message:  fmt.Sprintf("decision page requires status (%s)", strings.Join(schema.DecisionStatuses, ", ")),
				})
			} else if !schema.IsValidDecisionStatus(m.Status) {
				issues = append(issues, Issue{
					Severity: SevError,
					Check:    c.Name(),
					File:     p.Path,
					Line:     lineOrOne(m.FieldLines["status"]),
					Message:  fmt.Sprintf("invalid decision status %q (must be one of: %s)", m.Status, strings.Join(schema.DecisionStatuses, ", ")),
				})
			}
		case "incident":
			if m.Severity == "" {
				issues = append(issues, Issue{
					Severity: SevError,
					Check:    c.Name(),
					File:     p.Path,
					Line:     lineOrOne(m.FieldLines["type"]),
					Message:  "incident page requires severity field",
				})
			}
			if m.Status == "" {
				issues = append(issues, Issue{
					Severity: SevError,
					Check:    c.Name(),
					File:     p.Path,
					Line:     lineOrOne(m.FieldLines["type"]),
					Message:  fmt.Sprintf("incident page requires status (%s)", strings.Join(schema.IncidentStatuses, ", ")),
				})
			} else if !schema.IsValidIncidentStatus(m.Status) {
				issues = append(issues, Issue{
					Severity: SevError,
					Check:    c.Name(),
					File:     p.Path,
					Line:     lineOrOne(m.FieldLines["status"]),
					Message:  fmt.Sprintf("invalid incident status %q (must be one of: %s)", m.Status, strings.Join(schema.IncidentStatuses, ", ")),
				})
			}
		case "concept", "runbook":
			lc := m.EffectiveLifecycle()
			isInfra := p.WikiRel == "README.md" || p.WikiRel == "index.md" ||
				strings.HasPrefix(p.WikiRel, "prologue/") || strings.HasPrefix(p.WikiRel, "operations/")
			if !isInfra && (lc == "current" || lc == "wip") && len(m.AllSources()) == 0 {
				issues = append(issues, Issue{
					Severity: SevInfo,
					Check:    c.Name(),
					File:     p.Path,
					Line:     lineOrOne(m.FieldLines["type"]),
					Message:  fmt.Sprintf("%s page should cite source files or directories in sources", m.Type),
				})
			}
		}
	}
	return issues, nil
}

// --- relationshipsChecker ---

type relationshipsChecker struct{}

func (c *relationshipsChecker) Name() string       { return "relationships" }
func (c *relationshipsChecker) Group() string      { return "graph" }
func (c *relationshipsChecker) Category() Category { return CategorySnapshot }

func (c *relationshipsChecker) Check(snap *model.WikiSnapshot, _ *config.Config) ([]Issue, error) {
	var issues []Issue
	for _, p := range snap.Pages {
		if p.FMState != model.FrontMatterPresent || p.FMErr != nil {
			continue
		}
		for _, relID := range p.Metadata.Related {
			if relID == p.ID {
				issues = append(issues, Issue{
					Severity: SevError,
					Check:    c.Name(),
					File:     p.Path,
					Line:     lineOrOne(p.Metadata.FieldLines["related"]),
					Message:  fmt.Sprintf("related cannot reference self (%s)", relID),
				})
				continue
			}
			if _, err := snap.Lookup(relID); err != nil {
				issues = append(issues, Issue{
					Severity: SevError,
					Check:    c.Name(),
					File:     p.Path,
					Line:     lineOrOne(p.Metadata.FieldLines["related"]),
					Message:  fmt.Sprintf("related page ID not found: %s", relID),
				})
			}
		}

		for _, depID := range p.Metadata.DependsOn {
			if depID == p.ID {
				issues = append(issues, Issue{
					Severity: SevError,
					Check:    c.Name(),
					File:     p.Path,
					Line:     lineOrOne(p.Metadata.FieldLines["depends_on"]),
					Message:  fmt.Sprintf("depends_on cannot reference self (%s)", depID),
				})
				continue
			}
			target, err := snap.Lookup(depID)
			if err != nil {
				issues = append(issues, Issue{
					Severity: SevError,
					Check:    c.Name(),
					File:     p.Path,
					Line:     lineOrOne(p.Metadata.FieldLines["depends_on"]),
					Message:  fmt.Sprintf("depends_on page ID not found: %s", depID),
				})
				continue
			}
			if target.Metadata.Type != "entity" {
				issues = append(issues, Issue{
					Severity: SevError,
					Check:    c.Name(),
					File:     p.Path,
					Line:     lineOrOne(p.Metadata.FieldLines["depends_on"]),
					Message:  fmt.Sprintf("depends_on target %q must be an entity page (got type %q)", depID, target.Metadata.Type),
				})
			}
		}
	}
	return issues, nil
}

// --- indexFormatChecker ---

type indexFormatChecker struct{}

func (c *indexFormatChecker) Name() string       { return "index-format" }
func (c *indexFormatChecker) Group() string      { return "schema" }
func (c *indexFormatChecker) Category() Category { return CategorySnapshot }

var (
	bulletLinkRe = regexp.MustCompile(`^\s*[-*]\s+\[`)
	mdLinkPairRe = regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)
)

func (c *indexFormatChecker) Check(snap *model.WikiSnapshot, _ *config.Config) ([]Issue, error) {
	if snap.Index == nil {
		return nil, nil
	}
	var issues []Issue
	for i, line := range snap.Index.Lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if bulletLinkRe.MatchString(line) {
			parts := strings.SplitN(line, "|", 2)
			match := mdLinkPairRe.FindStringSubmatch(parts[0])
			if len(match) < 3 {
				issues = append(issues, Issue{
					Severity: SevWarn,
					Check:    c.Name(),
					File:     snap.Index.Path,
					Line:     i + 1,
					Message:  "malformed link in index entry",
				})
				continue
			}
			target := match[2]
			if strings.HasPrefix(target, "/") || strings.Contains(target, "://") {
				issues = append(issues, Issue{
					Severity: SevError,
					Check:    c.Name(),
					File:     snap.Index.Path,
					Line:     i + 1,
					Message:  fmt.Sprintf("index entry must use a relative path: %s", target),
				})
			}
			if len(parts) < 2 {
				issues = append(issues, Issue{
					Severity: SevWarn,
					Check:    c.Name(),
					File:     snap.Index.Path,
					Line:     i + 1,
					Message:  "index entry missing pipe-separated description",
				})
			} else if strings.TrimSpace(parts[1]) == "" {
				issues = append(issues, Issue{
					Severity: SevWarn,
					Check:    c.Name(),
					File:     snap.Index.Path,
					Line:     i + 1,
					Message:  "index entry has empty description",
				})
			}
		}
	}
	return issues, nil
}

// --- headingHierarchyChecker ---

type headingHierarchyChecker struct{}

func (c *headingHierarchyChecker) Name() string       { return "heading-hierarchy" }
func (c *headingHierarchyChecker) Group() string      { return "schema" }
func (c *headingHierarchyChecker) Category() Category { return CategorySnapshot }

func (c *headingHierarchyChecker) Check(snap *model.WikiSnapshot, _ *config.Config) ([]Issue, error) {
	var issues []Issue
	for _, p := range snap.Pages {
		lastLevel := 0
		h1Count := 0
		for _, h := range p.Headings {
			if h.Level == 1 {
				h1Count++
			}
			if lastLevel > 0 && h.Level > lastLevel+1 {
				issues = append(issues, Issue{
					Severity: SevWarn,
					Check:    c.Name(),
					File:     p.Path,
					Line:     h.Line,
					Message:  fmt.Sprintf("heading level skip: h%d → h%d", lastLevel, h.Level),
				})
			}
			lastLevel = h.Level
		}
		if h1Count > 1 {
			issues = append(issues, Issue{
				Severity: SevInfo,
				Check:    c.Name(),
				File:     p.Path,
				Message:  fmt.Sprintf("multiple h1 headings (%d)", h1Count),
			})
		}
	}
	return issues, nil
}

// --- logHeadingsChecker & logChronologyChecker ---

var (
	logHeadingRe      = regexp.MustCompile(`^## \[`)
	logHeadingValidRe = regexp.MustCompile(`^## \[\d{4}-\d{2}-\d{2}\] [^|]+ \| .+$`)
	logDateRe         = regexp.MustCompile(`^## \[(\d{4}-\d{2}-\d{2})\]`)
)

type logHeadingsChecker struct{}

func (c *logHeadingsChecker) Name() string       { return "log-headings" }
func (c *logHeadingsChecker) Group() string      { return "schema" }
func (c *logHeadingsChecker) Category() Category { return CategorySnapshot }

func (c *logHeadingsChecker) Check(snap *model.WikiSnapshot, _ *config.Config) ([]Issue, error) {
	if snap.Log == nil {
		return nil, nil
	}
	var issues []Issue
	for i, line := range snap.Log.Lines {
		if logHeadingRe.MatchString(line) && !logHeadingValidRe.MatchString(line) {
			issues = append(issues, Issue{
				Severity: SevError,
				Check:    c.Name(),
				File:     snap.Log.Path,
				Line:     i + 1,
				Message:  fmt.Sprintf("invalid log heading format: %s", line),
			})
		}
	}
	return issues, nil
}

type logChronologyChecker struct{}

func (c *logChronologyChecker) Name() string       { return "log-chronology" }
func (c *logChronologyChecker) Group() string      { return "schema" }
func (c *logChronologyChecker) Category() Category { return CategorySnapshot }

func (c *logChronologyChecker) Check(snap *model.WikiSnapshot, _ *config.Config) ([]Issue, error) {
	if snap.Log == nil {
		return nil, nil
	}
	var dates []string
	var dateLines []int
	for i, line := range snap.Log.Lines {
		if m := logDateRe.FindStringSubmatch(line); m != nil {
			dates = append(dates, m[1])
			dateLines = append(dateLines, i+1)
		}
	}
	var issues []Issue
	for i := 1; i < len(dates); i++ {
		if dates[i] > dates[i-1] {
			issues = append(issues, Issue{
				Severity: SevWarn,
				Check:    c.Name(),
				File:     snap.Log.Path,
				Line:     dateLines[i],
				Message:  fmt.Sprintf("log entries not in descending date order: %s after %s", dates[i], dates[i-1]),
			})
		}
	}
	return issues, nil
}

// --- phaseConsistencyChecker ---

type phaseConsistencyChecker struct{}

func (c *phaseConsistencyChecker) Name() string       { return "phase-consistency" }
func (c *phaseConsistencyChecker) Group() string      { return "schema" }
func (c *phaseConsistencyChecker) Category() Category { return CategorySnapshot }

var phaseRowRe = regexp.MustCompile(`^\|\s*(\d+)\s*\|\s*(.+?)\s*\|\s*(\S+)\s*\|`)

func (c *phaseConsistencyChecker) Check(snap *model.WikiSnapshot, _ *config.Config) ([]Issue, error) {
	if snap.Phases == nil {
		return nil, nil
	}
	type phaseRow struct {
		num    int
		status string
		line   int
	}
	var phases []phaseRow
	for i, line := range snap.Phases.Lines {
		m := phaseRowRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		num := 0
		_, _ = fmt.Sscanf(m[1], "%d", &num)
		phases = append(phases, phaseRow{
			num:    num,
			status: strings.TrimSpace(m[3]),
			line:   i + 1,
		})
	}
	validStatuses := map[string]bool{
		"not-started": true,
		"in-progress": true,
		"completed":   true,
		"blocked":     true,
	}
	var issues []Issue
	completed := make(map[int]bool)
	for _, p := range phases {
		if !validStatuses[p.status] {
			issues = append(issues, Issue{
				Severity: SevWarn,
				Check:    c.Name(),
				File:     snap.Phases.Path,
				Line:     p.line,
				Message:  fmt.Sprintf("unknown phase status: %q", p.status),
			})
		}
		if p.status == "completed" {
			completed[p.num] = true
		}
	}
	// A phase may start only after the nearest lower phase on the board is
	// completed. Boards can start at 1 or skip numbers. A blocked phase can
	// wait on an open predecessor, so only in-progress and completed phases count.
	nums := make([]int, 0, len(phases))
	for _, p := range phases {
		nums = append(nums, p.num)
	}
	sort.Ints(nums)
	for _, p := range phases {
		if p.status != "in-progress" && p.status != "completed" {
			continue
		}
		prev := -1
		for _, n := range nums {
			if n >= p.num {
				break
			}
			prev = n
		}
		if prev >= 0 && !completed[prev] {
			issues = append(issues, Issue{
				Severity: SevWarn,
				Check:    c.Name(),
				File:     snap.Phases.Path,
				Line:     p.line,
				Message:  fmt.Sprintf("phase %d is %s but phase %d is not completed", p.num, p.status, prev),
			})
		}
	}
	return issues, nil
}

func lineOrOne(l int) int {
	if l > 0 {
		return l
	}
	return 1
}

// --- vocabularyChecker ---

type vocabularyChecker struct{}

func (c *vocabularyChecker) Name() string       { return "vocabulary" }
func (c *vocabularyChecker) Group() string      { return "schema" }
func (c *vocabularyChecker) Category() Category { return CategorySnapshot }

func (c *vocabularyChecker) Check(snap *model.WikiSnapshot, _ *config.Config) ([]Issue, error) {
	file := filepath.ToSlash(filepath.Join(snap.WikiDir, model.VocabularyFile))
	if snap.VocabularyErr != nil {
		return []Issue{{
			Severity: SevError,
			Check:    c.Name(),
			File:     file,
			Message:  snap.VocabularyErr.Error(),
		}}, nil
	}
	if snap.Vocabulary.Empty() {
		return nil, nil
	}
	var issues []Issue
	// Pages of every lifecycle are checked: tags are metadata, not claims.
	for _, p := range snap.Pages {
		if p.FMErr != nil {
			continue
		}
		for _, tag := range p.Metadata.Tags {
			if snap.Vocabulary.Has(tag) {
				continue
			}
			msg := fmt.Sprintf("tag %q is not in %s", tag, file)
			if hint := snap.Vocabulary.Suggest(tag); hint != "" {
				msg += fmt.Sprintf(` (did you mean "%s"?)`, hint)
			}
			issues = append(issues, Issue{
				Severity: SevError,
				Check:    c.Name(),
				File:     p.Path,
				Line:     p.Metadata.FieldLines["tags"],
				Message:  msg,
			})
		}
	}
	return issues, nil
}
