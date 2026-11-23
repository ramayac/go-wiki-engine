package git

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

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
	if !IsRepo(dir) {
		return nil, fmt.Errorf("git diff failed (this command requires git and a git repository)")
	}

	cs := &ChangeSet{
		Renames:     make(map[string]string),
		WorkingTree: opts.IncludeWorkingTree,
	}

	changedSet := make(map[string]bool)
	deletedSet := make(map[string]bool)

	if opts.DiffRange != "" && opts.Base == "" {
		cs.Base = opts.DiffRange
		// If it's a triple-dot range `a...b`, record merge-base for metadata when resolvable.
		if base := extractBaseFromRange(opts.DiffRange); base != "" && RefExists(dir, base) {
			if mb, err := MergeBase(dir, base); err == nil {
				cs.MergeBase = mb
			}
		}
		if err := collectNameStatus(dir, []string{"diff", "--name-status", "-M", opts.DiffRange}, changedSet, deletedSet, cs.Renames); err != nil {
			return nil, fmt.Errorf("git diff failed (this command requires git and a git repository): %w", err)
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

		if err := collectNameStatus(dir, []string{"diff", "--name-status", "-M", mb, "HEAD"}, changedSet, deletedSet, cs.Renames); err != nil {
			return nil, fmt.Errorf("git diff failed: %w", err)
		}
	}

	if opts.IncludeWorkingTree {
		// Staged changes (index vs HEAD).
		if err := collectNameStatus(dir, []string{"diff", "--cached", "--name-status", "-M"}, changedSet, deletedSet, cs.Renames); err != nil {
			return nil, fmt.Errorf("git diff --cached failed: %w", err)
		}
		// Unstaged changes (worktree vs index).
		if err := collectNameStatus(dir, []string{"diff", "--name-status", "-M"}, changedSet, deletedSet, cs.Renames); err != nil {
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

// collectNameStatus executes `git <args...>` and merges modified/added/renamed/deleted
// paths into the provided sets.
func collectNameStatus(dir string, args []string, changedSet, deletedSet map[string]bool, renames map[string]string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return err
	}
	for _, line := range strings.Split(strings.TrimRight(string(out), "\r\n"), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) < 2 {
			continue
		}
		status := fields[0]
		switch {
		case strings.HasPrefix(status, "R") && len(fields) >= 3:
			oldPath := filepath.ToSlash(fields[1])
			newPath := filepath.ToSlash(fields[2])
			renames[oldPath] = newPath
			changedSet[newPath] = true
			changedSet[oldPath] = true
			delete(deletedSet, newPath)
		case strings.HasPrefix(status, "C") && len(fields) >= 3:
			newPath := filepath.ToSlash(fields[2])
			changedSet[newPath] = true
		case status == "D":
			p := filepath.ToSlash(fields[1])
			deletedSet[p] = true
			changedSet[p] = true
		default:
			p := filepath.ToSlash(fields[1])
			changedSet[p] = true
			delete(deletedSet, p)
		}
	}
	return nil
}

func listUntracked(dir string) ([]string, error) {
	cmd := exec.Command("git", "ls-files", "--others", "--exclude-standard")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	var files []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			files = append(files, filepath.ToSlash(line))
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
