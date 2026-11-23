// Package git provides unified Git ref resolution, branch-aware change
// discovery (merge-base, renames, deletions, working tree, untracked files),
// and batched history lookups.
package git

import (
	"fmt"
	"os/exec"
	"strings"
)

// IsRepo reports whether dir is inside a Git working tree.
func IsRepo(dir string) bool {
	cmd := exec.Command("git", "rev-parse", "--is-inside-work-tree")
	cmd.Dir = dir
	out, err := cmd.Output()
	return err == nil && strings.TrimSpace(string(out)) == "true"
}

// RefExists reports whether ref resolves to a commit in dir.
func RefExists(dir, ref string) bool {
	if strings.TrimSpace(ref) == "" {
		return false
	}
	cmd := exec.Command("git", "rev-parse", "--verify", "--quiet", ref+"^{commit}")
	cmd.Dir = dir
	out, err := cmd.Output()
	return err == nil && strings.TrimSpace(string(out)) != ""
}

// ResolveBaseRef chooses the base ref to compare HEAD against.
// Precedence: explicitBase -> configBase -> left side of defaultDiff ->
// auto-detected default branch (origin/main, main, origin/master, master, HEAD~1, HEAD).
func ResolveBaseRef(dir, explicitBase, configBase, defaultDiff string) (string, error) {
	if !IsRepo(dir) {
		return "", fmt.Errorf("not a git repository: %s", dir)
	}
	if explicitBase != "" {
		if !RefExists(dir, explicitBase) {
			return "", fmt.Errorf("base ref %q does not exist", explicitBase)
		}
		return explicitBase, nil
	}
	if configBase != "" {
		if RefExists(dir, configBase) {
			return configBase, nil
		}
		return "", fmt.Errorf("configured base_ref %q does not exist", configBase)
	}

	// Try extracting the base side from defaultDiff (e.g. "main...HEAD" -> "main").
	if defaultDiff != "" {
		base := extractBaseFromRange(defaultDiff)
		if base != "" && RefExists(dir, base) {
			return base, nil
		}
	}

	for _, cand := range []string{"origin/main", "main", "origin/master", "master", "HEAD~1", "HEAD"} {
		if RefExists(dir, cand) {
			return cand, nil
		}
	}
	return "", fmt.Errorf("could not resolve a base git ref in %s", dir)
}

func extractBaseFromRange(diffRange string) string {
	if idx := strings.Index(diffRange, "..."); idx != -1 {
		return strings.TrimSpace(diffRange[:idx])
	}
	if idx := strings.Index(diffRange, ".."); idx != -1 {
		return strings.TrimSpace(diffRange[:idx])
	}
	return strings.TrimSpace(diffRange)
}

// MergeBase computes the merge-base commit between baseRef and HEAD.
func MergeBase(dir, baseRef string) (string, error) {
	cmd := exec.Command("git", "merge-base", baseRef, "HEAD")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		// Single-commit or orphan branch fallback when baseRef is HEAD.
		if RefExists(dir, baseRef) {
			rev := exec.Command("git", "rev-parse", baseRef+"^{commit}")
			rev.Dir = dir
			if rout, rerr := rev.Output(); rerr == nil {
				return strings.TrimSpace(string(rout)), nil
			}
		}
		return "", fmt.Errorf("git merge-base %s HEAD failed: %w", baseRef, err)
	}
	return strings.TrimSpace(string(out)), nil
}
