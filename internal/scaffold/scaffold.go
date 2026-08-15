// Package scaffold handles the `wiki-engine init` command by copying
// embedded template files into a target repository.
package scaffold

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

//go:embed all:files
var files embed.FS

// shimFiles are root-level files created with create-only semantics: written
// on init and sync-prompts only when they do not already exist. This prevents
// overwriting user-customised entrypoint files.
var shimFiles = []string{"AGENTS.md", "CLAUDE.md"}

// promptWorkflows are the canonical workflow files in .wiki-instructions/ that
// are symlinked into the tool-specific directories.
var promptWorkflows = []string{"ingest", "lint", "onboard", "query", "refresh", "upgrade", "watch"}

// toolDirLinks maps destination-relative paths to symlink targets relative to
// the directory containing the link. init and sync-prompts create these as
// symlinks so that edits to the canonical .wiki-instructions/ files propagate
// to every tool layer. On platforms where symlink creation fails (e.g.
// Windows without developer mode), a regular copy is written instead.
var toolDirLinks = buildToolDirLinks()

func buildToolDirLinks() map[string]string {
	links := map[string]string{
		".github/instructions/wiki-maintainer.instructions.md": "../../.wiki-instructions/wiki-maintainer.md",
	}
	for _, w := range promptWorkflows {
		target := "../../.wiki-instructions/" + w + ".md"
		links[".github/prompts/wiki-"+w+".prompt.md"] = target
		links[".claude/commands/wiki-"+w+".md"] = target
	}
	return links
}

// writeScaffoldFile writes an embedded scaffold file to dest. Files that have
// a symlink mapping in toolDirLinks are written as symlinks to the canonical
// .wiki-instructions/ source, falling back to a regular copy when symlink
// creation fails.
func writeScaffoldFile(dest, rel string, data []byte) error {
	target, isLink := toolDirLinks[rel]
	if !isLink {
		return os.WriteFile(dest, data, 0o644)
	}

	// Replace any existing file (or stale symlink) at the destination.
	if err := os.Remove(dest); err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := os.Symlink(target, dest); err == nil {
		return nil
	}
	return os.WriteFile(dest, data, 0o644)
}

// syncShims copies the shim files (AGENTS.md, CLAUDE.md) into destDir only
// when they do not already exist. Returns the names of files written.
func syncShims(destDir string) ([]string, error) {
	var created []string
	for _, name := range shimFiles {
		dest := filepath.Join(destDir, name)
		if _, err := os.Stat(dest); err == nil {
			continue // already exists — never overwrite user content
		}
		data, err := files.ReadFile("files/" + name)
		if err != nil {
			continue // not in embedded FS — skip silently
		}
		if err := os.WriteFile(dest, data, 0o644); err != nil {
			return created, err
		}
		created = append(created, name)
	}
	return created, nil
}

// Init copies the scaffold into destDir. It refuses to overwrite an existing
// wiki directory.
func Init(destDir, wikiDir string) error {
	wikiPath := filepath.Join(destDir, wikiDir)
	if _, err := os.Stat(wikiPath); err == nil {
		return fmt.Errorf("%s already exists; refusing to overwrite", wikiDir)
	}

	if err := fs.WalkDir(files, "files", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Strip the "files/" prefix to get the relative destination path.
		rel, _ := filepath.Rel("files", path)
		if rel == "." {
			return nil
		}

		// Shim files are handled separately with create-only semantics.
		for _, shim := range shimFiles {
			if rel == shim {
				return nil
			}
		}

		// Remap scaffold "wiki/" to the requested wikiDir name.
		if strings.HasPrefix(rel, "wiki/") {
			rel = wikiDir + rel[len("wiki"):]
		} else if rel == "wiki" {
			rel = wikiDir
		}

		dest := filepath.Join(destDir, rel)

		// .wikirc is user-authored — never overwrite an existing one,
		// matching the create-only semantics used for AGENTS.md/CLAUDE.md.
		if rel == ".wikirc" {
			if _, err := os.Stat(dest); err == nil {
				return nil
			}
		}

		if d.IsDir() {
			return os.MkdirAll(dest, 0o755)
		}

		data, err := files.ReadFile(path)
		if err != nil {
			return err
		}

		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return err
		}
		return writeScaffoldFile(dest, rel, data)
	}); err != nil {
		return err
	}

	_, err := syncShims(destDir)
	return err
}

// SyncPrompts overwrites the .wiki-instructions/, .github/prompts/,
// .github/instructions/, .claude/commands/, and .pi/skills/ files in
// destDir with the current embedded versions. It does not touch wiki/
// content or .wikirc. Safe to run after a wiki-engine upgrade to pick
// up new or changed prompts and instructions for all supported AI tools.
func SyncPrompts(destDir string) ([]string, error) {
	var updated []string

	// Sync each instruction layer prefix. The embedded FS dereferences
	// symlinks, so .github/prompts/ and .claude/commands/ contain regular
	// file copies of the canonical .wiki-instructions/ files.
	prefixes := []string{
		"files/.wiki-instructions",
		"files/.github/prompts",
		"files/.github/instructions",
		"files/.claude/commands",
		"files/.pi/skills",
	}
	for _, prefix := range prefixes {
		err := syncEmbeddedDir(destDir, prefix, &updated)
		if err != nil {
			return updated, err
		}
	}

	// Remove destination files that no longer exist in the embedded
	// FS. This cleans up prompts that were removed from the scaffold
	// (e.g. migrate-shims.md, summarize.md).
	cleanOrphanedFiles(destDir, prefixes, &updated)

	shims, err := syncShims(destDir)
	updated = append(updated, shims...)
	return updated, err
}

// cleanOrphanedFiles removes files in the destination sync directories
// that no longer exist in the embedded FS. Only files within the known
// sync prefixes are considered — wiki/ and .wikirc are never touched.
func cleanOrphanedFiles(destDir string, embedPrefixes []string, cleaned *[]string) {
	// Build the set of all known embedded paths (relative to destDir).
	known := make(map[string]bool)
	for _, prefix := range embedPrefixes {
		fs.WalkDir(files, prefix, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			rel, _ := filepath.Rel("files", path)
			known[rel] = true
			return nil
		})
	}

	// Walk each destination prefix and remove files not in known.
	for _, prefix := range embedPrefixes {
		relRoot, _ := filepath.Rel("files", prefix)
		walkRoot := filepath.Join(destDir, relRoot)
		filepath.Walk(walkRoot, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return err
			}
			rel, _ := filepath.Rel(destDir, path)
			if !known[rel] {
				if err := os.Remove(path); err == nil {
					*cleaned = append(*cleaned, "removed "+rel)
				}
			}
			return nil
		})
	}
}

// syncEmbeddedDir walks an embedded directory and writes all files
// (including those in subdirectories) to the corresponding destination
// path, overwriting any existing files. Directory structure is created
// as needed. Updated file rel paths are appended to updated.
func syncEmbeddedDir(destDir, embedRoot string, updated *[]string) error {
	return fs.WalkDir(files, embedRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			rel, _ := filepath.Rel("files", path)
			if rel == "files" {
				return nil
			}
			return os.MkdirAll(filepath.Join(destDir, rel), 0o755)
		}

		rel, _ := filepath.Rel("files", path)
		dest := filepath.Join(destDir, rel)

		data, err := files.ReadFile(path)
		if err != nil {
			return err
		}

		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return err
		}
		if err := writeScaffoldFile(dest, rel, data); err != nil {
			return err
		}
		*updated = append(*updated, rel)
		return nil
	})
}
