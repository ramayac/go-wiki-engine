package model

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/ramayac/go-wiki-engine/internal/paths"
)

// ResolvedSource describes the result of resolving one SourceReference against
// the local repository.
type ResolvedSource struct {
	Raw       SourceReference
	CrossRepo bool   // true if Raw.Repo names a different repository
	RepoRel   string // normalized repo-relative slash path (when !CrossRepo)
	AbsPath   string // verified contained absolute path (when !CrossRepo)
	Exists    bool   // true if target exists on disk
	IsDir     bool   // true if target is a directory (directory source coverage)
	Err       error  // containment or path error
}

// LocalRepoName derives the local repository name from the repo root directory
// (and, if go.mod exists, matches either its final module segment or directory basename).
func LocalRepoName(repoRoot string) string {
	abs, err := filepath.Abs(repoRoot)
	if err != nil {
		return filepath.Base(repoRoot)
	}
	return filepath.Base(abs)
}

// IsCrossRepo reports whether a SourceReference targets a different repository.
// An empty Repo always means the current repository.
func IsCrossRepo(sr SourceReference, repoRoot string) bool {
	if sr.Repo == "" {
		return false
	}
	local := LocalRepoName(repoRoot)
	if strings.EqualFold(sr.Repo, local) {
		return false
	}
	// Also treat "self" / "." as local repo aliases if ever used.
	if sr.Repo == "." || sr.Repo == "self" {
		return false
	}
	return true
}

// ResolveSource resolves a SourceReference through the single containment API.
// Cross-repo sources are marked CrossRepo=true and never matched against local files.
func ResolveSource(repoRoot, pageDir string, sr SourceReference) ResolvedSource {
	res := ResolvedSource{Raw: sr}
	if sr.File == "" {
		return res
	}
	if IsCrossRepo(sr, repoRoot) {
		res.CrossRepo = true
		return res
	}

	abs, rel, exists, err := paths.ResolveRepoSource(repoRoot, pageDir, sr.File)
	if err != nil {
		res.Err = err
		return res
	}
	res.AbsPath = abs
	res.RepoRel = rel
	res.Exists = exists
	if exists {
		if info, serr := os.Stat(abs); serr == nil && info.IsDir() {
			res.IsDir = true
		}
	} else if strings.HasSuffix(sr.File, "/") {
		res.IsDir = true
	}
	return res
}

// MatchesFile reports whether a resolved local source covers the given
// repo-relative slash path, returning ("source", true) for exact file match or
// ("directory", true) for directory coverage. Cross-repo sources never match.
func (rs ResolvedSource) MatchesFile(changedRepoRel string) (reason string, ok bool) {
	if rs.CrossRepo || rs.Err != nil || rs.RepoRel == "" {
		return "", false
	}
	changed := filepath.ToSlash(filepath.Clean(changedRepoRel))
	target := filepath.ToSlash(filepath.Clean(rs.RepoRel))
	if changed == target {
		return "source", true
	}
	if rs.IsDir || strings.HasSuffix(rs.Raw.File, "/") {
		prefix := strings.TrimSuffix(target, "/") + "/"
		if strings.HasPrefix(changed, prefix) {
			return "directory", true
		}
	}
	return "", false
}
