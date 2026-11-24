// Package git provides unified Git ref resolution, branch-aware change
// discovery (merge-base, renames, deletions, working tree, untracked files),
// and batched history lookups.
package git

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"unicode"
)

// gitOutput runs `git <args...>` in dir. On failure it returns an error that
// carries the first line of git's stderr instead of a bare exit status.
func gitOutput(dir string, args ...string) ([]byte, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			for _, line := range strings.Split(string(bytes.TrimSpace(ee.Stderr)), "\n") {
				if line = strings.TrimSpace(line); line != "" {
					return nil, errors.New(line)
				}
			}
		}
		return nil, err
	}
	return out, nil
}

// IsShallow reports whether dir is inside a shallow clone (truncated history).
func IsShallow(dir string) bool {
	out, err := gitOutput(dir, "rev-parse", "--is-shallow-repository")
	return err == nil && strings.TrimSpace(string(out)) == "true"
}

// IsRepo reports whether dir is inside a Git working tree.
func IsRepo(dir string) bool {
	cmd := exec.Command("git", "rev-parse", "--is-inside-work-tree")
	cmd.Dir = dir
	out, err := cmd.Output()
	return err == nil && strings.TrimSpace(string(out)) == "true"
}

// RefExists reports whether ref resolves to a commit in dir.
func RefExists(dir, ref string) bool {
	// A leading "-" would be read by git as an option, never as a revision.
	if strings.TrimSpace(ref) == "" || strings.HasPrefix(ref, "-") {
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
	if strings.HasPrefix(baseRef, "-") {
		return "", fmt.Errorf("invalid base ref %q: must not start with \"-\"", baseRef)
	}
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

// ValidateDiffRange checks that r is a revision, `a..b`, or `a...b` of existing
// revisions. It rejects option-like parts, whitespace, and control characters,
// so r is safe to pass to git as a revision argument.
func ValidateDiffRange(dir, r string) error {
	if r == "" {
		return errors.New("range is empty")
	}
	for _, c := range r {
		if unicode.IsSpace(c) || unicode.IsControl(c) {
			return fmt.Errorf("%q must not contain whitespace or control characters", r)
		}
	}
	if strings.HasPrefix(r, "-") {
		return fmt.Errorf("%q must not start with \"-\"", r)
	}
	parts := strings.Split(r, "...")
	if len(parts) == 1 {
		parts = strings.Split(r, "..")
	}
	if len(parts) > 2 {
		return fmt.Errorf("%q has more than one range operator", r)
	}
	named := 0
	for _, p := range parts {
		if p == "" {
			continue
		}
		named++
		if strings.HasPrefix(p, "-") {
			return fmt.Errorf("revision %q must not start with \"-\"", p)
		}
		if !RefExists(dir, p) {
			return fmt.Errorf("unknown revision %q", p)
		}
	}
	if named == 0 {
		return fmt.Errorf("%q names no revision", r)
	}
	return nil
}
