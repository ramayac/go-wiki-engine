// Package scaffold handles `wiki-engine init`, idempotent reconciliation,
// wiki directory rename, and `wiki-engine sync-prompts`.
package scaffold

import (
	"bytes"
	"embed"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/ramayac/go-wiki-engine/internal/paths"
)

//go:embed all:files
var files embed.FS

// shimFiles are root-level agent entrypoint files managed via versioned fenced blocks.
var shimFiles = []string{"AGENTS.md", "CLAUDE.md"}

const (
	shimStartMarker = "<!-- wiki-engine:shim:v2 -->"
	shimEndMarker   = "<!-- /wiki-engine:shim -->"
)

var (
	wikiDirAssignRe   = regexp.MustCompile(`(?m)^wiki_dir\s*=\s*"[^"]*"`)
	ignoreWikiEntryRe = regexp.MustCompile(`(?m)^\s*"wiki/"`)
	ignoreBlockRe     = regexp.MustCompile(`(?ms)(ignore\s*=\s*\[)(.*?)(\])`)
	fencedShimRe      = regexp.MustCompile(`(?ms)<!-- wiki-engine:shim:v\d+ -->.*?<!-- /wiki-engine:shim -->`)
)

// promptWorkflows are the canonical workflow files in .wiki-instructions/ that
// are symlinked into the tool-specific directories.
var promptWorkflows = []string{"ingest", "lint", "onboard", "plan", "query", "refresh", "save", "seed", "upgrade", "watch"}

// toolDirLinks maps destination-relative paths to symlink targets relative to
// the directory containing the link.
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

// warnOut receives one-line notices such as backups. Tests replace it.
var warnOut io.Writer = os.Stderr

// guardWrite refuses to touch target when its real location is outside root.
// It resolves symlinks on the deepest existing ancestor of target, so a
// symlinked managed directory or file that leaves the repo is an error.
func guardWrite(root, target string) error {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	absTarget, err := filepath.Abs(target)
	if err != nil {
		return err
	}
	if err := paths.CheckSymlinkContainment(absRoot, absTarget); err != nil {
		rel, rerr := filepath.Rel(absRoot, absTarget)
		if rerr != nil {
			rel = target
		}
		return fmt.Errorf("refusing to write %s: %w", filepath.ToSlash(rel), err)
	}
	return nil
}

// backupFile copies data to the first free name in <dest>.bak, <dest>.bak.1, ...
// and never overwrites an existing backup.
func backupFile(dest string, data []byte) (string, error) {
	for i := 0; ; i++ {
		name := dest + ".bak"
		if i > 0 {
			name = fmt.Sprintf("%s.bak.%d", dest, i)
		}
		f, err := os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if os.IsExist(err) {
			continue
		}
		if err != nil {
			return "", err
		}
		_, werr := f.Write(data)
		if cerr := f.Close(); werr == nil {
			werr = cerr
		}
		return name, werr
	}
}

// writeScaffoldFile writes one managed file under root. A symlink already at
// dest is replaced, never followed. A hand-edited regular file that is about
// to become a tool-directory symlink is copied to a .bak file first.
func writeScaffoldFile(root, dest, rel string, data []byte) error {
	if err := guardWrite(root, filepath.Dir(dest)); err != nil {
		return err
	}
	info, lerr := os.Lstat(dest)
	if lerr == nil && info.Mode()&os.ModeSymlink != 0 {
		if err := os.Remove(dest); err != nil {
			return err
		}
	}
	target, isLink := toolDirLinks[rel]
	if !isLink {
		return os.WriteFile(dest, data, 0o644)
	}
	if lerr == nil && info.Mode().IsRegular() {
		existing, err := os.ReadFile(dest)
		if err != nil {
			return err
		}
		if !bytes.Equal(existing, data) {
			bak, err := backupFile(dest, existing)
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintf(warnOut, "backed up edited %s to %s\n", rel, filepath.ToSlash(filepath.Base(bak)))
		}
	}
	if err := os.Remove(dest); err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := os.Symlink(target, dest); err == nil {
		return nil
	}
	return os.WriteFile(dest, data, 0o644)
}

func renderShimBlock(name, wikiDir string) string {
	heading := "# AI Agent Instructions"
	if name == "CLAUDE.md" {
		heading = "# Claude Instructions"
	}
	return fmt.Sprintf(`%s
%s

This project uses a structured wiki for all documentation and agent context.

Start here: **[%s/index.md](%s/index.md)**

The wiki covers architecture, conventions, active phases, and the project change log.
To update or query the wiki, use the tool-specific workflows in
`+"`.github/prompts/`"+` (GitHub Copilot) or `+"`.claude/commands/`"+` (Claude Code).
%s`, shimStartMarker, heading, wikiDir, wikiDir, shimEndMarker)
}

// isPristineLegacyShim reports whether data matches the unedited v1 scaffold shim.
func isPristineLegacyShim(name string, data []byte) bool {
	embedded, err := files.ReadFile("files/" + name)
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(data)) == strings.TrimSpace(string(embedded))
}

// reconcileShims ensures AGENTS.md and CLAUDE.md contain a pointer to wikiDir
// while preserving any user-authored content:
//   - Absent: write pristine generated shim with versioned marker.
//   - Pristine v1 shim: refresh to versioned v2 shim.
//   - Contains versioned marker block: refresh ONLY the fenced block, preserving surrounding user edits.
//   - Existing user-authored file without marker: append fenced pointer block (unless it already references wikiDir/index.md).
func reconcileShims(destDir, wikiDir string, appendToUser bool) (updated []string, preserved []string, err error) {
	for _, name := range shimFiles {
		dest := filepath.Join(destDir, name)
		block := renderShimBlock(name, wikiDir)
		if err := guardWrite(destDir, dest); err != nil {
			return updated, preserved, err
		}

		existing, rerr := os.ReadFile(dest)
		if os.IsNotExist(rerr) {
			if err := os.WriteFile(dest, []byte(block+"\n"), 0o644); err != nil {
				return updated, preserved, err
			}
			updated = append(updated, name)
			continue
		}
		if rerr != nil {
			return updated, preserved, rerr
		}

		text := string(existing)
		switch {
		case fencedShimRe.MatchString(text):
			// Update only the fenced wiki-engine block; keep any user content outside untouched.
			replaced := fencedShimRe.ReplaceAllLiteralString(text, block)
			if replaced != text {
				if err := os.WriteFile(dest, []byte(replaced), 0o644); err != nil {
					return updated, preserved, err
				}
				updated = append(updated, name)
			} else {
				preserved = append(preserved, name)
			}
		case isPristineLegacyShim(name, existing):
			if err := os.WriteFile(dest, []byte(block+"\n"), 0o644); err != nil {
				return updated, preserved, err
			}
			updated = append(updated, name)
		default:
			preserved = append(preserved, name)
			if appendToUser && !strings.Contains(text, wikiDir+"/index.md") {
				sep := "\n\n"
				if strings.HasSuffix(text, "\n\n") {
					sep = ""
				} else if strings.HasSuffix(text, "\n") {
					sep = "\n"
				}
				appended := text + sep + block + "\n"
				if err := os.WriteFile(dest, []byte(appended), 0o644); err != nil {
					return updated, preserved, err
				}
				updated = append(updated, name)
			}
		}
	}
	return updated, preserved, nil
}

// Init scaffolds a new wiki or idempotently reconciles an existing managed wiki
// in destDir. Existing user pages, index, log, .wikirc values, and user-authored
// AGENTS.md/CLAUDE.md content are preserved.
func Init(destDir, wikiDir string) error {
	if err := paths.ValidateWikiDir(wikiDir); err != nil {
		return err
	}
	_, err := Reconcile(destDir, ReconcileOptions{WikiDir: wikiDir})
	return err
}

// SyncPrompts overwrites the .wiki-instructions/, .github/prompts/,
// .github/instructions/, .claude/commands/, and .pi/skills/ files in
// destDir with the current embedded versions. It does not touch wiki/
// content or .wikirc.
func SyncPrompts(destDir string) (updated, removed []string, err error) {
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
			return updated, removed, err
		}
	}

	cleanOrphanedFiles(destDir, prefixes, &removed)

	// In sync-prompts, create missing shims and refresh fenced/pristine shims
	// without appending to custom user-edited shims (preserving exact backward compat).
	wikiDir := detectWikiDir(destDir, "wiki")
	shims, _, err := reconcileShims(destDir, wikiDir, false)
	updated = append(updated, shims...)
	return updated, removed, err
}

// retiredManagedFiles are files earlier releases shipped in the sync
// directories that the current embed no longer has. Paths are relative to the
// repo root, with forward slashes. The engine deletes these and no other
// regular file.
var retiredManagedFiles = map[string]bool{
	".claude/commands/wiki-migrate-shims.md":       true,
	".github/prompts/wiki-migrate-shims.prompt.md": true,
	".wiki-instructions/migrate-shims.md":          true,
	".wiki-instructions/summarize.md":              true,
}

// isEngineOwned reports whether the engine may delete path. It owns a retired
// file, and a symlink that points into .wiki-instructions/ (its own tool links).
func isEngineOwned(destDir, path, rel string, info os.FileInfo) bool {
	if retiredManagedFiles[rel] {
		return true
	}
	if info.Mode()&os.ModeSymlink == 0 {
		return false
	}
	link, err := os.Readlink(path)
	if err != nil {
		return false
	}
	if !filepath.IsAbs(link) {
		link = filepath.Join(filepath.Dir(path), link)
	}
	inRel, err := filepath.Rel(filepath.Join(destDir, ".wiki-instructions"), link)
	return err == nil && inRel != "." && inRel != ".." && !strings.HasPrefix(inRel, ".."+string(filepath.Separator))
}

func cleanOrphanedFiles(destDir string, embedPrefixes []string, cleaned *[]string) {
	known := make(map[string]bool)
	for _, prefix := range embedPrefixes {
		_ = fs.WalkDir(files, prefix, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			rel, _ := filepath.Rel("files", path)
			known[filepath.ToSlash(rel)] = true
			return nil
		})
	}

	for _, prefix := range embedPrefixes {
		relRoot, _ := filepath.Rel("files", prefix)
		walkRoot := filepath.Join(destDir, relRoot)
		if guardWrite(destDir, walkRoot) != nil {
			continue
		}
		_ = filepath.Walk(walkRoot, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return err
			}
			rel, _ := filepath.Rel(destDir, path)
			relSlash := filepath.ToSlash(rel)
			if known[relSlash] || !isEngineOwned(destDir, path, relSlash, info) {
				return nil
			}
			if err := os.Remove(path); err == nil {
				*cleaned = append(*cleaned, relSlash)
			}
			return nil
		})
	}
}

func syncEmbeddedDir(destDir, embedRoot string, updated *[]string) error {
	return fs.WalkDir(files, embedRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			rel, _ := filepath.Rel("files", path)
			rel = filepath.ToSlash(rel)
			if rel == "files" || rel == "." {
				return nil
			}
			dir := filepath.Join(destDir, filepath.FromSlash(rel))
			if err := guardWrite(destDir, dir); err != nil {
				return err
			}
			return os.MkdirAll(dir, 0o755)
		}

		rel, _ := filepath.Rel("files", path)
		rel = filepath.ToSlash(rel)
		dest := filepath.Join(destDir, filepath.FromSlash(rel))

		data, err := files.ReadFile(path)
		if err != nil {
			return err
		}

		if err := guardWrite(destDir, filepath.Dir(dest)); err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return err
		}
		if err := writeScaffoldFile(destDir, dest, rel, data); err != nil {
			return err
		}
		*updated = append(*updated, rel)
		return nil
	})
}
