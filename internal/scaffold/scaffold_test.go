package scaffold

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestInit(t *testing.T) {
	dest := t.TempDir()
	err := Init(dest, "wiki")
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Check required wiki files exist.
	required := []string{
		"wiki/README.md",
		"wiki/index.md",
		"wiki/prologue/log.md",
		"wiki/prologue/schema.md",
		"wiki/prologue/phases.md",
		"wiki/prologue/repo-map.md",
		"wiki/operations/ingest.md",
		"wiki/operations/query.md",
		"wiki/operations/lint.md",
		"wiki/decisions/example.md",
		"wiki/architectures/example.md",
	}
	for _, f := range required {
		p := filepath.Join(dest, f)
		if _, err := os.Stat(p); os.IsNotExist(err) {
			t.Errorf("missing scaffold file: %s", f)
		}
	}

	// Check prompts and instructions.
	prompts := []string{
		".github/prompts/wiki-ingest.prompt.md",
		".github/prompts/wiki-query.prompt.md",
		".github/prompts/wiki-refresh.prompt.md",
		".github/instructions/wiki-maintainer.instructions.md",
	}
	for _, f := range prompts {
		p := filepath.Join(dest, f)
		if _, err := os.Stat(p); os.IsNotExist(err) {
			t.Errorf("missing scaffold file: %s", f)
		}
	}

	// Check .wikirc.
	if _, err := os.Stat(filepath.Join(dest, ".wikirc")); os.IsNotExist(err) {
		t.Error("missing .wikirc")
	}

	assertPromptSymlinks(t, dest)
}

func TestInitRefuses(t *testing.T) {
	dest := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dest, "wiki"), 0o755); err != nil {
		t.Fatal(err)
	}
	err := Init(dest, "wiki")
	if err == nil {
		t.Error("Init should refuse when wiki/ already exists")
	}
}

func TestInitPreservesExistingWikirc(t *testing.T) {
	dest := t.TempDir()
	custom := []byte("wiki_dir = \"custom\"\n")
	if err := os.WriteFile(filepath.Join(dest, ".wikirc"), custom, 0o644); err != nil {
		t.Fatal(err)
	}

	if err := Init(dest, "wiki"); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dest, ".wikirc"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != string(custom) {
		t.Errorf(".wikirc was overwritten by Init:\n%s", data)
	}
}

func TestInitCustomDir(t *testing.T) {
	dest := t.TempDir()
	err := Init(dest, "docs")
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	// The wiki files should be under docs/ since we remapped the dir.
	if _, err := os.Stat(filepath.Join(dest, "docs", "index.md")); os.IsNotExist(err) {
		t.Error("missing docs/index.md")
	}
	if _, err := os.Stat(filepath.Join(dest, ".wikirc")); os.IsNotExist(err) {
		t.Error("missing .wikirc")
	}

	// The scaffolded .wikirc must point at the remapped dir, not the default.
	data, err := os.ReadFile(filepath.Join(dest, ".wikirc"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `wiki_dir = "docs"`) {
		t.Errorf(".wikirc should remap wiki_dir to docs, got:\n%s", data)
	}
	if !strings.Contains(string(data), `"docs/"`) {
		t.Errorf(".wikirc should remap the ignore-list wiki/ entry to docs/, got:\n%s", data)
	}
}

func TestSyncPrompts(t *testing.T) {
	dest := t.TempDir()

	// SyncPrompts should work even on a repo that has never had init run.
	updated, err := SyncPrompts(dest)
	if err != nil {
		t.Fatalf("SyncPrompts failed: %v", err)
	}
	if len(updated) == 0 {
		t.Fatal("SyncPrompts returned no updated files")
	}

	// Verify canonical instruction files were written.
	canonical := []string{
		".wiki-instructions/ingest.md",
		".wiki-instructions/query.md",
		".wiki-instructions/refresh.md",
		".wiki-instructions/onboard.md",
		".wiki-instructions/lint.md",
		".wiki-instructions/wiki-maintainer.md",
	}
	for _, f := range canonical {
		p := filepath.Join(dest, f)
		if _, err := os.Stat(p); os.IsNotExist(err) {
			t.Errorf("SyncPrompts missing canonical file: %s", f)
		}
	}

	// Verify Copilot prompt files were written.
	copilot := []string{
		".github/prompts/wiki-ingest.prompt.md",
		".github/prompts/wiki-query.prompt.md",
		".github/prompts/wiki-refresh.prompt.md",
		".github/prompts/wiki-onboard.prompt.md",
		".github/prompts/wiki-lint.prompt.md",
		".github/instructions/wiki-maintainer.instructions.md",
	}
	for _, f := range copilot {
		p := filepath.Join(dest, f)
		if _, err := os.Stat(p); os.IsNotExist(err) {
			t.Errorf("SyncPrompts missing Copilot file: %s", f)
		}
	}

	// Verify Claude Code command files were written.
	claude := []string{
		".claude/commands/wiki-ingest.md",
		".claude/commands/wiki-query.md",
		".claude/commands/wiki-refresh.md",
		".claude/commands/wiki-onboard.md",
		".claude/commands/wiki-lint.md",
	}
	for _, f := range claude {
		p := filepath.Join(dest, f)
		if _, err := os.Stat(p); os.IsNotExist(err) {
			t.Errorf("SyncPrompts missing Claude Code file: %s", f)
		}
	}

	// Verify pi.dev skill files were written.
	piSkills := []string{
		".pi/skills/wiki/SKILL.md",
	}
	for _, f := range piSkills {
		p := filepath.Join(dest, f)
		if _, err := os.Stat(p); os.IsNotExist(err) {
			t.Errorf("SyncPrompts missing pi.dev skill file: %s", f)
		}
	}

	// Wiki content and .wikirc should NOT have been created.
	if _, err := os.Stat(filepath.Join(dest, "wiki")); !os.IsNotExist(err) {
		t.Error("SyncPrompts should not create wiki/")
	}
	if _, err := os.Stat(filepath.Join(dest, ".wikirc")); !os.IsNotExist(err) {
		t.Error("SyncPrompts should not create .wikirc")
	}

	assertPromptSymlinks(t, dest)
}

func TestSyncPromptsOverwrites(t *testing.T) {
	dest := t.TempDir()

	// Write a stale version of the ingest prompt.
	stale := filepath.Join(dest, ".github", "prompts", "wiki-ingest.prompt.md")
	if err := os.MkdirAll(filepath.Dir(stale), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(stale, []byte("old content"), 0o644); err != nil {
		t.Fatal(err)
	}

	// SyncPrompts should overwrite it.
	if _, err := SyncPrompts(dest); err != nil {
		t.Fatalf("SyncPrompts failed: %v", err)
	}

	data, err := os.ReadFile(stale)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) == "old content" {
		t.Error("SyncPrompts did not overwrite stale file")
	}
}

// assertPromptSymlinks verifies that the tool-layer files are real symlinks
// pointing into .wiki-instructions/. Symlinks may not be available on all
// platforms (e.g. Windows without developer mode), so the check is skipped
// there — those platforms fall back to regular copies.
func assertPromptSymlinks(t *testing.T, dest string) {
	if runtime.GOOS == "windows" {
		return
	}
	for _, f := range []string{
		".github/prompts/wiki-ingest.prompt.md",
		".github/prompts/wiki-watch.prompt.md",
		".claude/commands/wiki-watch.md",
		".github/instructions/wiki-maintainer.instructions.md",
	} {
		p := filepath.Join(dest, f)
		fi, err := os.Lstat(p)
		if err != nil {
			t.Errorf("missing %s: %v", f, err)
			continue
		}
		if fi.Mode()&os.ModeSymlink == 0 {
			t.Errorf("%s should be a symlink, got mode %s", f, fi.Mode())
			continue
		}
		target, err := os.Readlink(p)
		if err != nil {
			t.Errorf("readlink %s: %v", f, err)
			continue
		}
		if !strings.Contains(target, ".wiki-instructions/") {
			t.Errorf("%s symlink target %q does not point into .wiki-instructions/", f, target)
		}
	}
}

func TestInitCreatesShims(t *testing.T) {
	dest := t.TempDir()
	if err := Init(dest, "wiki"); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	for _, name := range []string{"AGENTS.md", "CLAUDE.md"} {
		p := filepath.Join(dest, name)
		if _, err := os.Stat(p); os.IsNotExist(err) {
			t.Errorf("Init did not create %s", name)
		}
	}
}

func TestInitPreservesExistingShims(t *testing.T) {
	dest := t.TempDir()

	// Write a user-customised AGENTS.md before init.
	custom := "# My custom agents instructions\n"
	if err := os.WriteFile(filepath.Join(dest, "AGENTS.md"), []byte(custom), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := Init(dest, "wiki"); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dest, "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != custom {
		t.Error("Init overwrote existing AGENTS.md — should preserve user content")
	}
}

func TestSyncPromptsCreatesShims(t *testing.T) {
	dest := t.TempDir()

	updated, err := SyncPrompts(dest)
	if err != nil {
		t.Fatalf("SyncPrompts failed: %v", err)
	}

	for _, name := range []string{"AGENTS.md", "CLAUDE.md"} {
		p := filepath.Join(dest, name)
		if _, err := os.Stat(p); os.IsNotExist(err) {
			t.Errorf("SyncPrompts did not create %s", name)
		}
		// The created filename should appear in the returned list.
		found := false
		for _, u := range updated {
			if u == name {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("SyncPrompts did not report %s in updated list", name)
		}
	}
}

func TestSyncPromptsPreservesExistingShims(t *testing.T) {
	dest := t.TempDir()

	// Write a user-customised CLAUDE.md before syncing.
	custom := "# My custom Claude instructions\n"
	if err := os.WriteFile(filepath.Join(dest, "CLAUDE.md"), []byte(custom), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := SyncPrompts(dest); err != nil {
		t.Fatalf("SyncPrompts failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dest, "CLAUDE.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != custom {
		t.Error("SyncPrompts overwrote existing CLAUDE.md — should preserve user content")
	}
}

func TestSyncPromptsRemovesOrphans(t *testing.T) {
	dest := t.TempDir()

	// First sync to populate the destination.
	if _, err := SyncPrompts(dest); err != nil {
		t.Fatalf("first SyncPrompts failed: %v", err)
	}

	// Write an orphan file that simulates a removed prompt.
	orphanPath := filepath.Join(dest, ".wiki-instructions", "migrate-shims.md")
	if err := os.WriteFile(orphanPath, []byte("stale content"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Second sync should remove the orphan.
	updated, err := SyncPrompts(dest)
	if err != nil {
		t.Fatalf("second SyncPrompts failed: %v", err)
	}

	if _, err := os.Stat(orphanPath); !os.IsNotExist(err) {
		t.Error("SyncPrompts did not remove orphaned file migrate-shims.md")
	}

	// Verify the removal was reported.
	found := false
	for _, u := range updated {
		if u == "removed .wiki-instructions/migrate-shims.md" {
			found = true
			break
		}
	}
	if !found {
		t.Error("SyncPrompts did not report orphan removal in updated list")
	}
}
