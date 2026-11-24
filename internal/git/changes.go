package git

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// LegacyDefaultDiff is the historical default_diff value. When it is the
// configured range and its base ref does not exist, DiscoverChanges falls back
// to base auto-detection instead of failing.
const LegacyDefaultDiff = "main...HEAD"

// ChangeSet captures all repository file changes across branch history and
// optionally the working tree (staged, unstaged, and untracked files).
type ChangeSet struct {
	Base         string            `json:"base"`
	MergeBase    string            `json:"merge_base"`
	ChangedFiles []string          `json:"changed_files"`
	DeletedFiles []string          `json:"deleted_files,omitempty"`
	Renames      map[string]string `json:"renames,omitempty"` // oldPath -> newPath
	Untracked    []string          `json:"untracked,omitempty"`
	WorkingTree  bool              `json:"working_tree"`
}

// ChangeOptions controls how DiscoverChanges computes the ChangeSet.
type ChangeOptions struct {
	Base               string // explicit base ref (e.g. "main", "origin/main")
	ConfigBase         string // fallback base_ref from .wikirc
	DiffRange          string // explicit diff range (e.g. "HEAD~1..HEAD" or "main...HEAD")
	DefaultDiff        string // default_diff from .wikirc
	IncludeWorkingTree bool   // include staged, unstaged, and untracked files
}

// DiscoverChanges runs the unified Git change discovery pipeline.
// When DiffRange is explicitly provided (and Base is empty), it respects that
// range directly while still optionally merging working-tree changes when
// IncludeWorkingTree is true. Otherwise it resolves a base ref, computes its
// merge-base with HEAD, tracks renames and deletions, and merges working-tree
// and untracked files when enabled.
func DiscoverChanges(dir string, opts ChangeOptions) (*ChangeSet, error) {
	if _, err := exec.LookPath("git"); err != nil {
		return nil, fmt.Errorf("git not found in PATH (this command requires git)")
	}
	if !IsRepo(dir) {
		return nil, fmt.Errorf("not a git repository: %s", dir)
	}

	cs := &ChangeSet{
		Renames:     make(map[string]string),
		WorkingTree: opts.IncludeWorkingTree,
	}

	changedSet := make(map[string]bool)
	deletedSet := make(map[string]bool)

	diffRange := opts.DiffRange
	// The scaffolded default names `main`. On repositories without that
	// branch, auto-detect the base the same way `check` does.
	if diffRange == LegacyDefaultDiff && diffRange == opts.DefaultDiff && opts.Base == "" &&
		!RefExists(dir, extractBaseFromRange(diffRange)) {
		diffRange = ""
	}

	if diffRange != "" && opts.Base == "" {
		if err := ValidateDiffRange(dir, diffRange); err != nil {
			return nil, fmt.Errorf("invalid diff range: %w", err)
		}
		cs.Base = diffRange
		// If it's a triple-dot range `a...b`, record merge-base for metadata when resolvable.
		if base := extractBaseFromRange(diffRange); base != "" && RefExists(dir, base) {
			if mb, err := MergeBase(dir, base); err == nil {
				cs.MergeBase = mb
			}
		}
		if err := collectNameStatus(dir, []string{"diff", "-z", "--name-status", "-M", "--relative", "--end-of-options", diffRange, "--"}, changedSet, deletedSet, cs.Renames); err != nil {
			return nil, fmt.Errorf("git diff failed: %w", err)
		}
	} else {
		baseRef, err := ResolveBaseRef(dir, opts.Base, opts.ConfigBase, opts.DefaultDiff)
		if err != nil {
			return nil, fmt.Errorf("git diff failed: %w", err)
		}
		mb, err := MergeBase(dir, baseRef)
		if err != nil {
			return nil, fmt.Errorf("git diff failed: %w", err)
		}
		cs.Base = baseRef
		cs.MergeBase = mb

		if err := collectNameStatus(dir, []string{"diff", "-z", "--name-status", "-M", "--relative", "--end-of-options", mb, "HEAD", "--"}, changedSet, deletedSet, cs.Renames); err != nil {
			return nil, fmt.Errorf("git diff failed: %w", err)
		}
	}

	if opts.IncludeWorkingTree {
		// Staged changes (index vs HEAD).
		if err := collectNameStatus(dir, []string{"diff", "-z", "--cached", "--name-status", "-M", "--relative"}, changedSet, deletedSet, cs.Renames); err != nil {
			return nil, fmt.Errorf("git diff --cached failed: %w", err)
		}
		// Unstaged changes (worktree vs index).
		if err := collectNameStatus(dir, []string{"diff", "-z", "--name-status", "-M", "--relative"}, changedSet, deletedSet, cs.Renames); err != nil {
			return nil, fmt.Errorf("git diff working tree failed: %w", err)
		}
		// Untracked files (respecting .gitignore).
		untracked, err := listUntracked(dir)
		if err != nil {
			return nil, fmt.Errorf("git ls-files untracked failed: %w", err)
		}
		cs.Untracked = untracked
		for _, u := range untracked {
			changedSet[u] = true
			delete(deletedSet, u)
		}
	}

	// Build sorted slices.
	for f := range changedSet {
		cs.ChangedFiles = append(cs.ChangedFiles, f)
	}
	sort.Strings(cs.ChangedFiles)

	for f := range deletedSet {
		cs.DeletedFiles = append(cs.DeletedFiles, f)
	}
	sort.Strings(cs.DeletedFiles)

	if len(cs.Renames) == 0 {
		cs.Renames = nil
	}
	return cs, nil
}

// collectNameStatus executes `git <args...>` (which must use -z --name-status)
// and merges modified/added/renamed/deleted paths into the provided sets.
// Paths are relative to dir.
func collectNameStatus(dir string, args []string, changedSet, deletedSet map[string]bool, renames map[string]string) error {
	out, err := gitOutput(dir, args...)
	if err != nil {
		return err
	}
	tokens := strings.Split(string(out), "\x00")
	for i := 0; i < len(tokens); {
		status := tokens[i]
		i++
		if status == "" {
			continue
		}
		pathCount := 1
		if strings.HasPrefix(status, "R") || strings.HasPrefix(status, "C") {
			pathCount = 2
		}
		if i+pathCount > len(tokens) {
			break
		}
		fields := tokens[i : i+pathCount]
		i += pathCount
		switch {
		case strings.HasPrefix(status, "R"):
			oldPath := filepath.ToSlash(fields[0])
			newPath := filepath.ToSlash(fields[1])
			renames[oldPath] = newPath
			changedSet[newPath] = true
			changedSet[oldPath] = true
			delete(deletedSet, newPath)
		case strings.HasPrefix(status, "C"):
			newPath := filepath.ToSlash(fields[1])
			changedSet[newPath] = true
		case status == "D":
			p := filepath.ToSlash(fields[0])
			deletedSet[p] = true
			changedSet[p] = true
		default:
			p := filepath.ToSlash(fields[0])
			changedSet[p] = true
			delete(deletedSet, p)
		}
	}
	return nil
}

func listUntracked(dir string) ([]string, error) {
	out, err := gitOutput(dir, "ls-files", "-z", "--others", "--exclude-standard")
	if err != nil {
		return nil, err
	}
	var files []string
	for _, f := range strings.Split(string(out), "\x00") {
		if f != "" {
			files = append(files, filepath.ToSlash(f))
		}
	}
	sort.Strings(files)
	return files, nil
}

// NonWikiChanged returns ChangedFiles excluding files under wikiDir/.
func (cs *ChangeSet) NonWikiChanged(wikiDir string) []string {
	prefix := strings.TrimSuffix(filepath.ToSlash(wikiDir), "/") + "/"
	var out []string
	for _, f := range cs.ChangedFiles {
		if strings.HasPrefix(f, prefix) {
			continue
		}
		out = append(out, f)
	}
	return out
}

// LiveNonWikiChanged returns NonWikiChanged without deleted files and without
// the old path of renames, so it lists only files that exist after the change.
func (cs *ChangeSet) LiveNonWikiChanged(wikiDir string) []string {
	gone := make(map[string]bool, len(cs.DeletedFiles)+len(cs.Renames))
	for _, f := range cs.DeletedFiles {
		gone[f] = true
	}
	for oldPath := range cs.Renames {
		gone[oldPath] = true
	}
	var out []string
	for _, f := range cs.NonWikiChanged(wikiDir) {
		if !gone[f] {
			out = append(out, f)
		}
	}
	return out
}

// WikiChanged returns ChangedFiles that live under wikiDir/.
func (cs *ChangeSet) WikiChanged(wikiDir string) []string {
	prefix := strings.TrimSuffix(filepath.ToSlash(wikiDir), "/") + "/"
	var out []string
	for _, f := range cs.ChangedFiles {
		if strings.HasPrefix(f, prefix) {
			out = append(out, f)
		}
	}
	return out
}
