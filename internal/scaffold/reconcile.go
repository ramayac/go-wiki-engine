package scaffold

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/ramayac/go-wiki-engine/internal/config"
	"github.com/ramayac/go-wiki-engine/internal/paths"
)

// ReconcileOptions configures idempotent setup and explicit wiki directory rename.
type ReconcileOptions struct {
	WikiDir  string // target wiki directory name (defaults to existing .wikirc or "wiki")
	RenameTo string // when set, explicitly rename the existing managed wiki to this segment
}

// ReconcileResult reports what `init` / reconciliation created, updated, preserved, or renamed.
type ReconcileResult struct {
	WikiDir        string   `json:"wiki_dir"`
	Mode           string   `json:"mode"` // "init", "reconcile", "rename"
	RenamedFrom    string   `json:"renamed_from,omitempty"`
	Created        []string `json:"created,omitempty"`
	Updated        []string `json:"updated,omitempty"`
	Removed        []string `json:"removed,omitempty"`
	Preserved      []string `json:"preserved,omitempty"`
	ShimsPreserved []string `json:"shims_preserved,omitempty"`
}

// Reconcile performs either initial bootstrap, idempotent reconciliation of an
// existing managed wiki, or an explicit wiki directory rename.
func Reconcile(destDir string, opts ReconcileOptions) (*ReconcileResult, error) {
	// Detect existing managed wiki before modifying anything.
	existingWikiDir := detectManagedWiki(destDir)

	// Handle explicit rename (`wiki-engine init --wiki-dir <name>` on an existing wiki).
	if opts.RenameTo != "" {
		if err := paths.ValidateWikiDir(opts.RenameTo); err != nil {
			return nil, err
		}
		if existingWikiDir != "" && existingWikiDir != opts.RenameTo {
			return renameAndReconcile(destDir, existingWikiDir, opts.RenameTo)
		}
		// If no existing wiki yet, treat RenameTo as the desired new wiki_dir.
		opts.WikiDir = opts.RenameTo
	}

	wikiDir := opts.WikiDir
	// A positional name must match an existing .wikirc, or lint would read the
	// old directory while init writes the new one.
	if wikiDir != "" && opts.RenameTo == "" {
		if err := paths.ValidateWikiDir(wikiDir); err != nil {
			return nil, err
		}
		if _, serr := os.Stat(filepath.Join(destDir, ".wikirc")); serr == nil {
			if cfg, lerr := config.Load(destDir); lerr == nil && cfg.WikiDir != wikiDir {
				return nil, fmt.Errorf("wiki_dir in .wikirc is %q; run init --wiki-dir=%s to rename or edit .wikirc", cfg.WikiDir, wikiDir)
			}
		}
	}
	if wikiDir == "" {
		if existingWikiDir != "" {
			wikiDir = existingWikiDir
		} else {
			wikiDir = "wiki"
		}
	}
	if err := paths.ValidateWikiDir(wikiDir); err != nil {
		return nil, err
	}

	wikiPath, err := paths.ResolveWikiRoot(destDir, wikiDir)
	if err != nil {
		return nil, err
	}

	// Refuse if target path exists as a non-directory file.
	info, statErr := os.Stat(wikiPath)
	isExisting := false
	if statErr == nil {
		if !info.IsDir() {
			return nil, fmt.Errorf("%s exists and is not a directory", wikiDir)
		}
		isExisting = true
	}

	res := &ReconcileResult{
		WikiDir: wikiDir,
		Mode:    "init",
	}
	if isExisting {
		res.Mode = "reconcile"
	}

	// 1. Walk embedded scaffold:
	//    - Wiki pages: create-only (NEVER overwrite existing user pages, index, or log).
	//    - .wikirc: create if absent; if present, preserve user values and ensure ignore contains wikiDir/.
	//    - Instruction/prompt/skill layers: reconcile cleanly via syncEmbeddedDir.
	if err := fs.WalkDir(files, "files", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel("files", path)
		rel = filepath.ToSlash(rel)
		if rel == "." {
			return nil
		}
		// Shims are reconciled separately with fenced marker semantics.
		for _, shim := range shimFiles {
			if rel == shim {
				return nil
			}
		}
		// Prompt/tool layers are reconciled via SyncPrompts below.
		if isPromptLayerPath(rel) {
			return nil
		}

		// Remap scaffold "wiki/" to wikiDir.
		if strings.HasPrefix(rel, "wiki/") {
			rel = wikiDir + rel[len("wiki"):]
		} else if rel == "wiki" {
			rel = wikiDir
		}

		dest := filepath.Join(destDir, filepath.FromSlash(rel))
		if err := guardWrite(destDir, dest); err != nil {
			return err
		}

		if d.IsDir() {
			return os.MkdirAll(dest, 0o755)
		}

		// Handle .wikirc with preserve + ignore-entry reconciliation.
		if rel == ".wikirc" {
			if _, serr := os.Stat(dest); serr == nil {
				changed, rerr := ensureWikircIgnoreEntry(dest, wikiDir)
				if rerr != nil {
					return rerr
				}
				if changed {
					res.Updated = append(res.Updated, ".wikirc")
				} else {
					res.Preserved = append(res.Preserved, ".wikirc")
				}
				return nil
			}
			data, rerr := files.ReadFile(path)
			if rerr != nil {
				return rerr
			}
			content := wikiDirAssignRe.ReplaceAllLiteralString(string(data), `wiki_dir = "`+wikiDir+`"`)
			content = ignoreWikiEntryRe.ReplaceAllLiteralString(content, `  "`+wikiDir+`/"`)
			if err := os.WriteFile(dest, []byte(content), 0o644); err != nil {
				return err
			}
			res.Created = append(res.Created, ".wikirc")
			return nil
		}

		// Wiki files: preserve any existing file! Only create missing template files.
		if _, serr := os.Stat(dest); serr == nil {
			res.Preserved = append(res.Preserved, filepath.ToSlash(rel))
			return nil
		}

		// On an existing wiki, don't re-create example placeholder files if the user deleted them.
		if isExisting && (strings.Contains(rel, "example") && (strings.Contains(rel, "decisions/") || strings.Contains(rel, "architectures/"))) {
			return nil
		}

		data, rerr := files.ReadFile(path)
		if rerr != nil {
			return rerr
		}
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return err
		}
		if err := writeScaffoldFile(destDir, dest, rel, data); err != nil {
			return err
		}
		res.Created = append(res.Created, filepath.ToSlash(rel))
		return nil
	}); err != nil {
		return nil, err
	}

	// 2. Reconcile prompt files, tool symlinks, and retired managed files.
	prefixes := syncPrefixes
	for _, prefix := range prefixes {
		if err := syncEmbeddedDir(destDir, prefix, &res.Updated); err != nil {
			return nil, err
		}
	}
	cleanOrphanedFiles(destDir, prefixes, &res.Removed)

	// 3. Reconcile root shims (AGENTS.md, CLAUDE.md): append fenced block to
	//    user-authored shims on init/reconcile without overwriting user content.
	shimUpdated, shimPreserved, err := reconcileShims(destDir, wikiDir, true)
	if err != nil {
		return nil, err
	}
	res.Updated = append(res.Updated, shimUpdated...)
	res.ShimsPreserved = shimPreserved

	// 4. Keep the temporary handoff folder out of git when the repo has a .gitignore.
	gitignoreChanged, err := ensureGitignoreEntry(destDir)
	if err != nil {
		return nil, err
	}
	if gitignoreChanged {
		res.Updated = append(res.Updated, ".gitignore")
	}

	return res, nil
}

// handoffDir is the temporary folder that the Wiki Plan prompt writes to.
const handoffDir = ".wiki-engine"

// ensureGitignoreEntry appends handoffDir to an existing .gitignore that does
// not ignore it yet. It never creates a .gitignore. It reports whether it wrote.
func ensureGitignoreEntry(destDir string) (bool, error) {
	path := filepath.Join(destDir, ".gitignore")
	if _, err := os.Lstat(path); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	if err := guardWrite(destDir, path); err != nil {
		return false, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	text := string(data)
	for _, line := range strings.Split(text, "\n") {
		switch strings.TrimSpace(line) {
		case handoffDir, handoffDir + "/", "/" + handoffDir, "/" + handoffDir + "/":
			return false, nil
		}
	}
	if text != "" && !strings.HasSuffix(text, "\n") {
		text += "\n"
	}
	text += handoffDir + "/\n"
	return true, os.WriteFile(path, []byte(text), 0o644)
}

func renameAndReconcile(destDir, oldWikiDir, newWikiDir string) (*ReconcileResult, error) {
	oldAbs, err := paths.ResolveWikiRoot(destDir, oldWikiDir)
	if err != nil {
		return nil, err
	}
	newAbs, err := paths.ResolveWikiRoot(destDir, newWikiDir)
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(oldAbs); err != nil {
		return nil, fmt.Errorf("existing wiki directory %q not found: %w", oldWikiDir, err)
	}
	if _, err := os.Stat(newAbs); err == nil {
		return nil, fmt.Errorf("refusing to rename wiki: destination %q already exists", newWikiDir)
	}

	if err := os.Rename(oldAbs, newAbs); err != nil {
		return nil, fmt.Errorf("rename %s -> %s: %w", oldWikiDir, newWikiDir, err)
	}

	// Update .wikirc wiki_dir and ignore entry.
	wikircPath := filepath.Join(destDir, ".wikirc")
	if data, err := os.ReadFile(wikircPath); err == nil {
		if err := guardWrite(destDir, wikircPath); err != nil {
			return nil, err
		}
		content := string(data)
		if wikiDirAssignRe.MatchString(content) {
			content = wikiDirAssignRe.ReplaceAllLiteralString(content, `wiki_dir = "`+newWikiDir+`"`)
		} else {
			content = fmt.Sprintf("wiki_dir = %q\n%s", newWikiDir, content)
		}
		content = strings.ReplaceAll(content, `"`+oldWikiDir+`/"`, `"`+newWikiDir+`/"`)
		if err := os.WriteFile(wikircPath, []byte(content), 0o644); err != nil {
			return nil, err
		}
	}

	// Reconcile prompts, ignore entries, and shims against the new wikiDir.
	res, err := Reconcile(destDir, ReconcileOptions{WikiDir: newWikiDir})
	if err != nil {
		return nil, err
	}
	res.Mode = "rename"
	res.RenamedFrom = oldWikiDir
	return res, nil
}

func detectManagedWiki(destDir string) string {
	// 1. Check .wikirc first if it exists.
	if _, err := os.Stat(filepath.Join(destDir, ".wikirc")); err == nil {
		if cfg, lerr := config.Load(destDir); lerr == nil && cfg.WikiDir != "" {
			if info, serr := os.Stat(filepath.Join(destDir, cfg.WikiDir)); serr == nil && info.IsDir() {
				return cfg.WikiDir
			}
		}
	}
	// 2. Fallback: check default "wiki/index.md".
	if _, err := os.Stat(filepath.Join(destDir, "wiki", "index.md")); err == nil {
		return "wiki"
	}
	return ""
}

func detectWikiDir(destDir, fallback string) string {
	if existing := detectManagedWiki(destDir); existing != "" {
		return existing
	}
	return fallback
}

func isPromptLayerPath(rel string) bool {
	s := filepath.ToSlash(rel)
	for _, p := range []string{
		".wiki-instructions",
		".github/prompts",
		".github/instructions",
		".claude/commands",
		".pi/skills",
		skillDir,
	} {
		if s == p || strings.HasPrefix(s, p+"/") {
			return true
		}
	}
	return false
}

// ensureWikircIgnoreEntry ensures that if .wikirc defines an `ignore = [...]`
// block, it contains "<wikiDir>/" without clobbering any existing user keys.
func ensureWikircIgnoreEntry(wikircPath, wikiDir string) (bool, error) {
	data, err := os.ReadFile(wikircPath)
	if err != nil {
		return false, err
	}
	text := string(data)
	needle := `"` + wikiDir + `/"`
	if strings.Contains(text, needle) {
		return false, nil
	}
	if !ignoreBlockRe.MatchString(text) {
		return false, nil
	}
	updated := ignoreBlockRe.ReplaceAllStringFunc(text, func(block string) string {
		idx := strings.Index(block, "[")
		if idx == -1 {
			return block
		}
		if strings.Contains(block, "\n") {
			return block[:idx+1] + "\n  " + needle + "," + block[idx+1:]
		}
		inner := strings.TrimSpace(block[idx+1 : len(block)-1])
		if inner == "" {
			return block[:idx+1] + needle + "]"
		}
		return block[:idx+1] + needle + ", " + inner + "]"
	})
	if updated == text {
		return false, nil
	}
	return true, os.WriteFile(wikircPath, []byte(updated), 0o644)
}
