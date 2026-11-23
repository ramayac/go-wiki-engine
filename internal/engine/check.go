package engine

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"

	"github.com/ramayac/go-wiki-engine/internal/git"
	"github.com/ramayac/go-wiki-engine/internal/model"
)

// ErrCheckSetup marks a Git or setup failure during `wiki-engine check` (exit 2).
var ErrCheckSetup = errors.New("check setup error")

// CheckOptions configures branch-aware citation and drift checking.
type CheckOptions struct {
	Base               string
	IncludeWorkingTree bool
}

// CheckFinding represents one broken citation or warning discovered by `check`.
type CheckFinding struct {
	Page    string `json:"page"`
	ID      string `json:"id,omitempty"`
	Kind    string `json:"kind"` // "missing_source", "deleted_source", "renamed_source", "invalid_commit", "unverifiable_commit", "containment", "frontmatter"
	Target  string `json:"target,omitempty"`
	Message string `json:"message"`
}

// ReviewRecommendation points to an unchanged live wiki page whose cited source
// file was modified on the branch.
type ReviewRecommendation struct {
	Page         string   `json:"page"`
	ID           string   `json:"id,omitempty"`
	ChangedFiles []string `json:"changed_files"`
	MatchReasons []string `json:"match_reasons"`
}

// CheckResult is the structured output of `wiki-engine check`.
type CheckResult struct {
	Base          string                 `json:"base"`
	MergeBase     string                 `json:"merge_base"`
	WorkingTree   bool                   `json:"working_tree"`
	ChangedFiles  []string               `json:"changed_files"`
	Untracked     []string               `json:"untracked,omitempty"`
	Broken        []CheckFinding         `json:"broken"`
	Warnings      []CheckFinding         `json:"warnings"`
	PagesToReview []ReviewRecommendation `json:"pages_to_review"`
}

// HasBreakage reports whether the branch introduced any broken citations (exit 1).
func (r *CheckResult) HasBreakage() bool {
	return len(r.Broken) > 0
}

// commitRefRe matches explicit commit citations like `commit:abcdef1` in wiki text.
var commitRefRe = regexp.MustCompile(`\bcommit:([0-9a-f]{7,40})\b`)

// Check runs branch-aware citation and impact verification:
// 1. Loads unified Git ChangeSet (merge-base + working tree + untracked unless disabled).
// 2. Validates changed live pages' source and commit citations.
// 3. Checks unchanged live pages citing changed, renamed, or deleted files.
// Returns an error wrapping ErrCheckSetup on Git/config/setup errors (exit 2).
func (e *Engine) Check(opts CheckOptions) (*CheckResult, error) {
	if _, err := e.ResolveWikiPath(); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrCheckSetup, err)
	}
	if !git.IsRepo(e.RootDir) {
		return nil, fmt.Errorf("%w: %s is not a git repository", ErrCheckSetup, e.RootDir)
	}

	cs, err := git.DiscoverChanges(e.RootDir, git.ChangeOptions{
		Base:               opts.Base,
		ConfigBase:         e.Cfg.BaseRef,
		DefaultDiff:        e.Cfg.DefaultDiff,
		IncludeWorkingTree: opts.IncludeWorkingTree,
	})
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrCheckSetup, err)
	}

	snap, err := e.LoadSnapshot()
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrCheckSetup, err)
	}
	if err := snap.FatalLoadError(); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrCheckSetup, err)
	}

	res := &CheckResult{
		Base:          cs.Base,
		MergeBase:     cs.MergeBase,
		WorkingTree:   cs.WorkingTree,
		ChangedFiles:  cs.ChangedFiles,
		Untracked:     cs.Untracked,
		Broken:        []CheckFinding{},
		Warnings:      []CheckFinding{},
		PagesToReview: []ReviewRecommendation{},
	}
	if res.ChangedFiles == nil {
		res.ChangedFiles = []string{}
	}

	changedWikiSet := make(map[string]bool)
	for _, wf := range cs.WikiChanged(e.Cfg.WikiDir) {
		changedWikiSet[filepath.ToSlash(filepath.Clean(wf))] = true
	}

	deletedSet := make(map[string]bool)
	for _, df := range cs.DeletedFiles {
		deletedSet[filepath.ToSlash(filepath.Clean(df))] = true
	}

	nonWikiChanged := cs.NonWikiChanged(e.Cfg.WikiDir)

	// A shallow clone lacks older commits, so a missing commit is not proof of a bad citation.
	shallow := git.IsShallow(e.RootDir)

	// 1. Validate changed live pages: sources, repo links, frontmatter, and commit citations.
	for _, p := range snap.Pages {
		if !changedWikiSet[p.Path] {
			continue
		}
		if p.FMErr != nil {
			res.Broken = append(res.Broken, CheckFinding{
				Page:    p.Path,
				ID:      p.ID,
				Kind:    "frontmatter",
				Message: fmt.Sprintf("invalid front matter in changed page: %v", p.FMErr),
			})
			continue
		}
		if !p.Metadata.IsDriftChecked() {
			continue
		}
		for _, rs := range p.ResolvedSources {
			if rs.CrossRepo {
				continue
			}
			if rs.Err != nil {
				res.Broken = append(res.Broken, CheckFinding{
					Page:    p.Path,
					ID:      p.ID,
					Kind:    "containment",
					Target:  rs.Raw.File,
					Message: fmt.Sprintf("source escapes repository boundary: %s", rs.Raw.File),
				})
				continue
			}
			if !rs.Exists {
				res.Broken = append(res.Broken, CheckFinding{
					Page:    p.Path,
					ID:      p.ID,
					Kind:    "missing_source",
					Target:  rs.Raw.File,
					Message: fmt.Sprintf("cited source does not exist: %s", rs.Raw.File),
				})
			}
		}
		// Validate explicit `commit:<sha>` citations in changed live pages.
		for _, m := range commitRefRe.FindAllStringSubmatch(p.Body, -1) {
			sha := m[1]
			if !git.CommitExists(e.RootDir, sha) {
				if shallow {
					res.Warnings = append(res.Warnings, CheckFinding{
						Page:    p.Path,
						ID:      p.ID,
						Kind:    "unverifiable_commit",
						Target:  sha,
						Message: fmt.Sprintf("cited commit %s cannot be verified (shallow clone)", sha),
					})
					continue
				}
				res.Broken = append(res.Broken, CheckFinding{
					Page:    p.Path,
					ID:      p.ID,
					Kind:    "invalid_commit",
					Target:  sha,
					Message: fmt.Sprintf("cited commit %s does not exist in git history", sha),
				})
			}
		}
		// Validate non-wiki repo links on changed live pages.
		for _, lk := range p.Links {
			if lk.IsExternal || lk.IsAnchorOnly || lk.IsMarkdown || lk.BaseTarget == "" {
				continue
			}
			if lk.EscapesRepo {
				res.Broken = append(res.Broken, CheckFinding{
					Page:    p.Path,
					ID:      p.ID,
					Kind:    "containment",
					Target:  lk.RawTarget,
					Message: fmt.Sprintf("link escapes repository boundary: %s", lk.RawTarget),
				})
				continue
			}
			if lk.RepoRel != "" {
				if _, err := os.Stat(filepath.Join(e.RootDir, filepath.FromSlash(lk.RepoRel))); err != nil {
					res.Warnings = append(res.Warnings, CheckFinding{
						Page:    p.Path,
						ID:      p.ID,
						Kind:    "missing_source",
						Target:  lk.RawTarget,
						Message: fmt.Sprintf("broken repo link in changed page: %s", lk.RawTarget),
					})
				}
			}
		}
	}

	// 2. Inspect unchanged live pages that cite changed, renamed, or deleted source files.
	for _, p := range snap.Pages {
		if changedWikiSet[p.Path] {
			continue
		}
		if !p.Metadata.IsDriftChecked() {
			continue
		}

		reviewFiles := make(map[string]bool)
		reviewReasons := make(map[string]bool)

		// Check declared sources against deletions, renames, and modifications.
		for _, rs := range p.ResolvedSources {
			if rs.CrossRepo || rs.Err != nil || rs.RepoRel == "" {
				continue
			}
			srcPath := filepath.ToSlash(filepath.Clean(rs.RepoRel))

			// Renamed source still cited under its old path -> broken by this branch!
			if newPath, renamed := cs.Renames[srcPath]; renamed && !rs.Exists {
				res.Broken = append(res.Broken, CheckFinding{
					Page:    p.Path,
					ID:      p.ID,
					Kind:    "renamed_source",
					Target:  srcPath,
					Message: fmt.Sprintf("cited source %s was renamed to %s", srcPath, newPath),
				})
				continue
			}

			// Deleted source still cited -> broken by this branch!
			if deletedSet[srcPath] && !rs.Exists {
				res.Broken = append(res.Broken, CheckFinding{
					Page:    p.Path,
					ID:      p.ID,
					Kind:    "deleted_source",
					Target:  srcPath,
					Message: fmt.Sprintf("cited source %s was deleted on this branch", srcPath),
				})
				continue
			}

			// Modified/added file covered by exact source or directory source -> review recommendation.
			for _, cf := range nonWikiChanged {
				if reason, ok := rs.MatchesFile(cf); ok {
					reviewFiles[cf] = true
					reviewReasons[reason] = true
				}
			}
		}

		// Also check exact Markdown links to repository files.
		for _, lk := range p.Links {
			if lk.IsExternal || lk.IsAnchorOnly || lk.EscapesRepo || lk.RepoRel == "" {
				continue
			}
			linkPath := filepath.ToSlash(filepath.Clean(lk.RepoRel))
			if newPath, renamed := cs.Renames[linkPath]; renamed {
				if _, err := os.Stat(filepath.Join(e.RootDir, filepath.FromSlash(linkPath))); err != nil {
					res.Broken = append(res.Broken, CheckFinding{
						Page:    p.Path,
						ID:      p.ID,
						Kind:    "renamed_source",
						Target:  linkPath,
						Message: fmt.Sprintf("linked repo file %s was renamed to %s", linkPath, newPath),
					})
					continue
				}
			}
			if deletedSet[linkPath] {
				if _, err := os.Stat(filepath.Join(e.RootDir, filepath.FromSlash(linkPath))); err != nil {
					res.Broken = append(res.Broken, CheckFinding{
						Page:    p.Path,
						ID:      p.ID,
						Kind:    "deleted_source",
						Target:  linkPath,
						Message: fmt.Sprintf("linked repo file %s was deleted on this branch", linkPath),
					})
					continue
				}
			}
			for _, cf := range nonWikiChanged {
				if filepath.ToSlash(filepath.Clean(cf)) == linkPath {
					reviewFiles[cf] = true
					reviewReasons["markdown_link"] = true
				}
			}
		}

		if len(reviewFiles) > 0 {
			files := sortedKeys(reviewFiles)
			reasons := sortedKeys(reviewReasons)
			res.PagesToReview = append(res.PagesToReview, ReviewRecommendation{
				Page:         p.Path,
				ID:           p.ID,
				ChangedFiles: files,
				MatchReasons: reasons,
			})
		}
	}

	sort.Slice(res.Broken, func(i, j int) bool {
		if res.Broken[i].Page != res.Broken[j].Page {
			return res.Broken[i].Page < res.Broken[j].Page
		}
		return res.Broken[i].Target < res.Broken[j].Target
	})
	sort.Slice(res.PagesToReview, func(i, j int) bool {
		return res.PagesToReview[i].Page < res.PagesToReview[j].Page
	})

	return res, nil
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// Keep model import referenced cleanly for callers inspecting page models.
var _ *model.Page
