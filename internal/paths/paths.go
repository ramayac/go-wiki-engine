// Package paths provides a single, strict path containment API for wiki-engine.
// Every repository and wiki path resolution must go through this package so no
// CLI command, config file, or Markdown link can read or traverse outside the
// selected repository or wiki boundary.
package paths

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

var (
	// ErrInvalidWikiDir is returned when a configured or requested wiki_dir is
	// not a single safe path segment.
	ErrInvalidWikiDir = errors.New("invalid wiki directory")
	// ErrOutsideBoundary is returned when a resolved path escapes its allowed
	// root directory (either lexically or through a symlink).
	ErrOutsideBoundary = errors.New("path escapes allowed boundary")
)

// reservedRepoNames are repository metadata / engine-managed names that must
// never be used as a wiki_dir.
var reservedRepoNames = map[string]bool{
	".git":               true,
	".github":            true,
	".claude":            true,
	".pi":                true,
	".wiki-instructions": true,
	".wikirc":            true,
	".hg":                true,
	".svn":               true,
}

// ValidateWikiDir requires wiki_dir to be exactly one safe, relative path
// segment inside the repository root. It rejects empty values, ".", "..",
// absolute paths, path separators ("/" and "\\"), drive prefixes, NUL bytes,
// percent-encoded traversal sequences, and repository metadata names.
func ValidateWikiDir(name string) error {
	if name == "" {
		return fmt.Errorf("%w: wiki_dir cannot be empty", ErrInvalidWikiDir)
	}
	if strings.TrimSpace(name) != name {
		return fmt.Errorf("%w: wiki_dir %q must not have leading or trailing whitespace", ErrInvalidWikiDir, name)
	}
	if strings.ContainsRune(name, '\x00') {
		return fmt.Errorf("%w: wiki_dir contains NUL byte", ErrInvalidWikiDir)
	}
	// A quote or control character would break the quoted .wikirc value.
	for _, r := range name {
		if r == '"' || r < 0x20 || r == 0x7f {
			return fmt.Errorf("%w: wiki_dir must not contain quotes or control characters: %q", ErrInvalidWikiDir, name)
		}
	}
	if name == "." || name == ".." {
		return fmt.Errorf("%w: %q is not allowed", ErrInvalidWikiDir, name)
	}
	if filepath.IsAbs(name) || strings.HasPrefix(name, "/") || strings.HasPrefix(name, `\`) {
		return fmt.Errorf("%w: wiki_dir must be relative to the repository: %q", ErrInvalidWikiDir, name)
	}
	// Reject any path separator (POSIX or Windows) so wiki_dir is strictly one segment.
	if strings.ContainsAny(name, `/\`) {
		return fmt.Errorf("%w: wiki_dir must be a single path segment: %q", ErrInvalidWikiDir, name)
	}
	// Reject Windows drive-letter prefixes like "C:" even on POSIX hosts.
	if len(name) >= 2 && name[1] == ':' {
		return fmt.Errorf("%w: wiki_dir must not contain a drive prefix: %q", ErrInvalidWikiDir, name)
	}
	// A leading "-" would read as a command-line option to git and other tools.
	if strings.HasPrefix(name, "-") {
		return fmt.Errorf("%w: wiki_dir must not start with \"-\": %q", ErrInvalidWikiDir, name)
	}
	if reservedRepoNames[strings.ToLower(name)] {
		return fmt.Errorf("%w: %q is a reserved repository metadata name", ErrInvalidWikiDir, name)
	}
	// Reject percent-encoded separators or dots.
	if decoded, err := url.PathUnescape(name); err == nil && decoded != name {
		if decoded == "." || decoded == ".." || strings.ContainsAny(decoded, `/\`) {
			return fmt.Errorf("%w: %q contains unsafe encoded characters", ErrInvalidWikiDir, name)
		}
	}
	return nil
}

// DecodePath unescapes percent-encoded Markdown path components and strips any
// query/fragment if still present. Returns an error on malformed escapes or NUL
// bytes.
func DecodePath(raw string) (string, error) {
	if strings.ContainsRune(raw, '\x00') {
		return "", fmt.Errorf("%w: path contains NUL byte", ErrOutsideBoundary)
	}
	decoded, err := url.PathUnescape(raw)
	if err != nil {
		return "", fmt.Errorf("%w: malformed percent-encoding in %q: %v", ErrOutsideBoundary, raw, err)
	}
	if strings.ContainsRune(decoded, '\x00') {
		return "", fmt.Errorf("%w: decoded path contains NUL byte", ErrOutsideBoundary)
	}
	return decoded, nil
}

// ResolveWikiRoot validates wikiDir and returns the verified absolute wiki path
// contained within repoRoot (checking symlinks when present).
func ResolveWikiRoot(repoRoot, wikiDir string) (string, error) {
	if err := ValidateWikiDir(wikiDir); err != nil {
		return "", err
	}
	return ResolveWithin(repoRoot, wikiDir)
}

// ResolveWithin resolves target relative to root and guarantees the resulting
// path stays strictly inside root, both lexically and after following any
// existing symlinks. target must be relative (absolute paths are rejected).
func ResolveWithin(root, target string) (string, error) {
	decoded, err := DecodePath(target)
	if err != nil {
		return "", err
	}
	if decoded == "" {
		return "", fmt.Errorf("%w: empty path", ErrOutsideBoundary)
	}
	if filepath.IsAbs(decoded) || strings.HasPrefix(decoded, "/") || strings.HasPrefix(decoded, `\`) {
		return "", fmt.Errorf("%w: absolute path %q not allowed", ErrOutsideBoundary, target)
	}
	if len(decoded) >= 2 && decoded[1] == ':' {
		return "", fmt.Errorf("%w: drive-prefixed path %q not allowed", ErrOutsideBoundary, target)
	}

	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("resolve root %q: %w", root, err)
	}
	normalized := filepath.FromSlash(strings.ReplaceAll(decoded, `\`, "/"))
	candidate := filepath.Clean(filepath.Join(absRoot, normalized))

	if !isWithinLexical(absRoot, candidate) {
		return "", fmt.Errorf("%w: %q resolves outside %s", ErrOutsideBoundary, target, absRoot)
	}

	if err := CheckSymlinkContainment(absRoot, candidate); err != nil {
		return "", fmt.Errorf("%w: %q: %v", ErrOutsideBoundary, target, err)
	}

	return candidate, nil
}

// ResolvePageRelative resolves a Markdown link target relative to pageDir and
// guarantees the target stays strictly within wikiRoot (lexically and across
// symlinks). Returns both the absolute path and the slash-separated
// wiki-relative path.
func ResolvePageRelative(wikiRoot, pageDir, target string) (absPath string, wikiRel string, err error) {
	decoded, err := DecodePath(target)
	if err != nil {
		return "", "", err
	}
	if decoded == "" {
		return "", "", fmt.Errorf("%w: empty target", ErrOutsideBoundary)
	}
	if filepath.IsAbs(decoded) || strings.HasPrefix(decoded, "/") || strings.HasPrefix(decoded, `\`) {
		return "", "", fmt.Errorf("%w: absolute link %q not allowed inside wiki", ErrOutsideBoundary, target)
	}
	if len(decoded) >= 2 && decoded[1] == ':' {
		return "", "", fmt.Errorf("%w: drive-prefixed link %q not allowed", ErrOutsideBoundary, target)
	}

	absWiki, err := filepath.Abs(wikiRoot)
	if err != nil {
		return "", "", err
	}
	absPageDir, err := filepath.Abs(pageDir)
	if err != nil {
		return "", "", err
	}
	if !isWithinLexical(absWiki, absPageDir) {
		return "", "", fmt.Errorf("%w: page dir outside wiki root", ErrOutsideBoundary)
	}

	normalized := filepath.FromSlash(strings.ReplaceAll(decoded, `\`, "/"))
	candidate := filepath.Clean(filepath.Join(absPageDir, normalized))
	if !isWithinLexical(absWiki, candidate) {
		return "", "", fmt.Errorf("%w: link %q escapes wiki boundary", ErrOutsideBoundary, target)
	}
	if err := CheckSymlinkContainment(absWiki, candidate); err != nil {
		return "", "", fmt.Errorf("%w: link %q escapes wiki via symlink: %v", ErrOutsideBoundary, target, err)
	}

	rel, err := filepath.Rel(absWiki, candidate)
	if err != nil {
		return "", "", err
	}
	return candidate, filepath.ToSlash(rel), nil
}

// ResolveRepoSource resolves a source reference (from frontmatter or a
// Markdown link to a non-wiki repo file) and guarantees it stays inside
// repoRoot. If pageDir is non-empty, it tries pageDir-relative first when that
// file exists inside repoRoot, then falls back to repoRoot-relative.
// If the path does not exist on disk yet, it still returns the validated
// repo-relative path so callers can report "not found" vs "outside repo".
func ResolveRepoSource(repoRoot, pageDir, target string) (absPath string, repoRel string, exists bool, err error) {
	decoded, err := DecodePath(target)
	if err != nil {
		return "", "", false, err
	}
	if decoded == "" {
		return "", "", false, fmt.Errorf("%w: empty source path", ErrOutsideBoundary)
	}
	if filepath.IsAbs(decoded) || strings.HasPrefix(decoded, "/") || strings.HasPrefix(decoded, `\`) {
		return "", "", false, fmt.Errorf("%w: absolute source path %q not allowed", ErrOutsideBoundary, target)
	}
	if len(decoded) >= 2 && decoded[1] == ':' {
		return "", "", false, fmt.Errorf("%w: drive-prefixed source path %q not allowed", ErrOutsideBoundary, target)
	}

	absRepo, err := filepath.Abs(repoRoot)
	if err != nil {
		return "", "", false, err
	}
	normalized := filepath.FromSlash(strings.ReplaceAll(decoded, `\`, "/"))

	// 1. Try page-relative first if pageDir is provided and contained in repo.
	if pageDir != "" {
		absPageDir, perr := filepath.Abs(pageDir)
		if perr == nil && isWithinLexical(absRepo, absPageDir) {
			cand := filepath.Clean(filepath.Join(absPageDir, normalized))
			if isWithinLexical(absRepo, cand) {
				if serr := CheckSymlinkContainment(absRepo, cand); serr == nil {
					if _, statErr := os.Stat(cand); statErr == nil {
						rel, _ := filepath.Rel(absRepo, cand)
						return cand, filepath.ToSlash(rel), true, nil
					}
				} else if _, lerr := os.Lstat(cand); lerr == nil {
					// Symlink exists and escapes repo!
					return "", "", false, fmt.Errorf("%w: source %q escapes repository via symlink", ErrOutsideBoundary, target)
				}
			}
		}
	}

	// 2. Try repo-root-relative.
	cand := filepath.Clean(filepath.Join(absRepo, normalized))
	if !isWithinLexical(absRepo, cand) {
		return "", "", false, fmt.Errorf("%w: source %q escapes repository", ErrOutsideBoundary, target)
	}
	if err := CheckSymlinkContainment(absRepo, cand); err != nil {
		return "", "", false, fmt.Errorf("%w: source %q escapes repository via symlink: %v", ErrOutsideBoundary, target, err)
	}
	rel, err := filepath.Rel(absRepo, cand)
	if err != nil {
		return "", "", false, err
	}
	_, statErr := os.Stat(cand)
	return cand, filepath.ToSlash(rel), statErr == nil, nil
}

// isWithinLexical checks whether target is equal to or nested inside root using
// cleaned paths.
func isWithinLexical(root, target string) bool {
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return false
	}
	if rel == "." {
		return true
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// CheckSymlinkContainment resolves symlinks on root and on the deepest existing
// ancestor of target (including target itself if it exists), verifying that the
// real target stays within realRoot. Callers use it before reads and before any
// write, mkdir, remove, or symlink under a managed path.
func CheckSymlinkContainment(absRoot, absTarget string) error {
	realRoot, err := filepath.EvalSymlinks(absRoot)
	if err != nil {
		if os.IsNotExist(err) {
			// Root does not exist yet (e.g. before init); lexical check already passed.
			return nil
		}
		return err
	}

	// Walk up from absTarget until we find an existing path component so we
	// catch symlinks on parent directories even when the final file doesn't exist.
	probe := absTarget
	var tail []string
	for {
		if _, err := os.Lstat(probe); err == nil {
			realProbe, evalErr := filepath.EvalSymlinks(probe)
			if evalErr != nil {
				// Broken symlink: inspect its link target so a dangling symlink
				// pointing outside realRoot is still rejected!
				if linkDest, rerr := os.Readlink(probe); rerr == nil {
					if !filepath.IsAbs(linkDest) {
						linkDest = filepath.Join(filepath.Dir(probe), linkDest)
					}
					linkDest = filepath.Clean(linkDest)
					if !isWithinLexical(realRoot, linkDest) {
						return fmt.Errorf("broken symlink points outside root: %s", linkDest)
					}
					return nil
				}
				return evalErr
			}
			// Re-attach any non-existent tail components.
			for i := len(tail) - 1; i >= 0; i-- {
				realProbe = filepath.Join(realProbe, tail[i])
			}
			if !isWithinLexical(realRoot, realProbe) {
				return fmt.Errorf("resolved path %s is outside %s", realProbe, realRoot)
			}
			return nil
		}
		parent := filepath.Dir(probe)
		if parent == probe {
			return nil
		}
		tail = append(tail, filepath.Base(probe))
		probe = parent
	}
}
