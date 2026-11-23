// Package lint implements single-read, snapshot-based wiki validation and
// deterministic mechanical fixes.
package lint

import (
	"fmt"
	"sort"
	"strings"

	"github.com/ramayac/go-wiki-engine/internal/config"
	"github.com/ramayac/go-wiki-engine/internal/model"
)

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

// Category separates pure snapshot checkers from filesystem and Git checkers.
type Category string

const (
	CategorySnapshot   Category = "snapshot"
	CategoryFilesystem Category = "filesystem"
	CategoryGit        Category = "git"
)

// Issue is a single lint finding.
type Issue struct {
	Severity Severity `json:"severity"`
	Check    string   `json:"check"`
	File     string   `json:"file"`
	Line     int      `json:"line"`
	Message  string   `json:"message"`

	// linkKey names the link behind a broken-link issue (file, line, target) so
	// that two checkers do not report the same link twice. It is not serialised.
	linkKey string
}

// Checker inspects a single WikiSnapshot and returns findings.
type Checker interface {
	Name() string
	Group() string
	Category() Category
	Check(snap *model.WikiSnapshot, cfg *config.Config) ([]Issue, error)
}

// Result holds the aggregated outcome of a lint run.
type Result struct {
	OK       bool
	Messages []string
	Issues   []Issue
	Reason   string
}

// GroupNames lists the supported grouped selector names in display order.
var GroupNames = []string{
	"schema",
	"links",
	"graph",
	"freshness",
	"security",
}

// AllCheckers returns all registered checkers in priority order.
func AllCheckers() []Checker {
	return []Checker{
		&pathContainmentChecker{},
		&requiredFilesChecker{},
		&frontMatterChecker{},
		&pageIDsChecker{},
		&pageTypesChecker{},
		&relationshipsChecker{},
		&vocabularyChecker{},
		&indexFormatChecker{},
		&bareURLChecker{},
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
		&referencesChecker{},
		&sourcesChecker{},
		&duplicateContentChecker{},
		&staleContentChecker{},
	}
}

// KnownCheckerNames returns the names of all registered individual checkers.
func KnownCheckerNames() []string {
	checkers := AllCheckers()
	names := make([]string, 0, len(checkers))
	for _, c := range checkers {
		names = append(names, c.Name())
	}
	return names
}

// KnownGroups returns a map from group name to its member checker names.
func KnownGroups() map[string][]string {
	groups := make(map[string][]string, len(GroupNames))
	for _, g := range GroupNames {
		groups[g] = nil
	}
	for _, c := range AllCheckers() {
		groups[c.Group()] = append(groups[c.Group()], c.Name())
	}
	return groups
}

// ValidateSelectors validates --check and --skip selector lists (accepting
// individual checker names and group names, and rejecting --skip=all).
func ValidateSelectors(check, skip []string) error {
	knownCheckers := make(map[string]bool)
	for _, n := range KnownCheckerNames() {
		knownCheckers[n] = true
	}
	knownGroups := make(map[string]bool)
	for _, g := range GroupNames {
		knownGroups[g] = true
	}
	available := strings.Join(append(append([]string(nil), KnownCheckerNames()...), GroupNames...), ", ")

	for _, n := range check {
		if n == "" || n == "all" {
			continue
		}
		if !knownCheckers[n] && !knownGroups[n] {
			return fmt.Errorf("unknown checker %q for lint --check (available: %s)", n, available)
		}
	}
	for _, n := range skip {
		if n == "" {
			continue
		}
		if n == "all" {
			return fmt.Errorf("--skip=all would disable every checker; run --check=<names> instead")
		}
		if !knownCheckers[n] && !knownGroups[n] {
			return fmt.Errorf("unknown checker %q for lint --skip (available: %s)", n, available)
		}
	}
	return nil
}

// Run executes selected checkers against one already-loaded WikiSnapshot.
func Run(snap *model.WikiSnapshot, cfg *config.Config, check, skip []string) Result {
	groups := KnownGroups()
	expand := func(list []string) (all bool, set map[string]bool) {
		set = make(map[string]bool)
		for _, item := range list {
			if item == "" {
				continue
			}
			if item == "all" {
				all = true
				continue
			}
			if members, isGroup := groups[item]; isGroup {
				for _, m := range members {
					set[m] = true
				}
				continue
			}
			set[item] = true
		}
		return all, set
	}

	checkAll, checkMap := expand(check)
	if len(checkMap) == 0 {
		checkAll = true
	}
	_, skipMap := expand(skip)

	var active []Checker
	for _, c := range AllCheckers() {
		name := c.Name()
		if skipMap[name] {
			continue
		}
		if !checkAll && !checkMap[name] {
			continue
		}
		active = append(active, c)
	}

	allIssues := make([]Issue, 0)
	for _, c := range active {
		issues, err := c.Check(snap, cfg)
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

	allIssues = dropDuplicateIndexLinks(allIssues)

	sort.SliceStable(allIssues, func(i, j int) bool {
		if allIssues[i].File != allIssues[j].File {
			return allIssues[i].File < allIssues[j].File
		}
		return allIssues[i].Line < allIssues[j].Line
	})

	var msgs []string
	for _, iss := range allIssues {
		if iss.Line > 0 {
			msgs = append(msgs, fmt.Sprintf("%s:%d: [%s] %s", iss.File, iss.Line, iss.Check, iss.Message))
		} else {
			msgs = append(msgs, fmt.Sprintf("%s: [%s] %s", iss.File, iss.Check, iss.Message))
		}
	}

	failThreshold := SevWarn
	switch strings.ToLower(cfg.FailSeverity) {
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

	return Result{
		OK:       !hasFailure,
		Messages: msgs,
		Issues:   allIssues,
	}
}

// dropDuplicateIndexLinks removes cross-page-links issues for links that
// index-links already reported at the same file, line, and target. When
// index-links did not run, nothing is dropped.
func dropDuplicateIndexLinks(issues []Issue) []Issue {
	seen := make(map[string]bool)
	for _, iss := range issues {
		if iss.Check == "index-links" && iss.linkKey != "" {
			seen[iss.linkKey] = true
		}
	}
	if len(seen) == 0 {
		return issues
	}
	out := issues[:0]
	for _, iss := range issues {
		if iss.Check == "cross-page-links" && seen[iss.linkKey] {
			continue
		}
		out = append(out, iss)
	}
	return out
}

func brokenLinkKey(file string, line int, target string) string {
	return fmt.Sprintf("%s:%d:%s", file, line, target)
}
