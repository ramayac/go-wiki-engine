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
		return os.WriteFile(dest, data, 0o644)
	}); err != nil {
		return err
	}

	_, err := syncShims(destDir)
	return err
}

// SyncPrompts overwrites the .github/prompts/ and .github/instructions/
// files in destDir with the current embedded versions. It does not touch
// wiki/ content or .wikirc. Safe to run after a wiki-engine upgrade to pick
// up new or changed prompts and instructions.
func SyncPrompts(destDir string) ([]string, error) {
	var updated []string

	err := fs.WalkDir(files, "files/.github", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			rel, _ := filepath.Rel("files", path)
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
		if err := os.WriteFile(dest, data, 0o644); err != nil {
			return err
		}
		updated = append(updated, rel)
		return nil
	})
	if err != nil {
		return updated, err
	}

	shims, err := syncShims(destDir)
	updated = append(updated, shims...)
	return updated, err
}
