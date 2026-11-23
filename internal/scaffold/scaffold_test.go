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
	}
	for _, f := range required {
		p := filepath.Join(dest, f)
		if _, err := os.Stat(p); os.IsNotExist(err) {
			t.Errorf("missing scaffold file: %s", f)
		}
	}

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

	if _, err := os.Stat(filepath.Join(dest, ".wikirc")); os.IsNotExist(err) {
		t.Error("missing .wikirc")
	}

	assertPromptSymlinks(t, dest)
}

func TestInitReconcilesIdempotently(t *testing.T) {
	dest := t.TempDir()
	if err := Init(dest, "wiki"); err != nil {
		t.Fatalf("first Init failed: %v", err)
	}

	// Simulate user edits to index.md, log.md, a custom page, .wikirc, and AGENTS.md.
	customIndex := "---\nid: index\nlifecycle: current\ndescription: Custom Index\n---\n# Custom Index\n"
	if err := os.WriteFile(filepath.Join(dest, "wiki", "index.md"), []byte(customIndex), 0o644); err != nil {
		t.Fatal(err)
	}
	customPage := "---\nid: custom\nlifecycle: current\ndescription: Custom\n---\n# Custom\n"
	if err := os.WriteFile(filepath.Join(dest, "wiki", "custom.md"), []byte(customPage), 0o644); err != nil {
		t.Fatal(err)
	}
	// Simulate prompt drift.
	promptPath := filepath.Join(dest, ".wiki-instructions", "ingest.md")
	if err := os.WriteFile(promptPath, []byte("drifted prompt"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Re-running init reconciles prompts without overwriting user content.
	res, err := Reconcile(dest, ReconcileOptions{WikiDir: "wiki"})
	if err != nil {
		t.Fatalf("second Reconcile failed: %v", err)
	}
	if res.Mode != "reconcile" {
		t.Errorf("Mode = %q, want reconcile", res.Mode)
	}
	gotIndex, _ := os.ReadFile(filepath.Join(dest, "wiki", "index.md"))
	if string(gotIndex) != customIndex {
		t.Errorf("reconcile overwrote user index.md:\n%s", gotIndex)
	}
	gotCustom, _ := os.ReadFile(filepath.Join(dest, "wiki", "custom.md"))
	if string(gotCustom) != customPage {
		t.Errorf("reconcile overwrote user custom.md:\n%s", gotCustom)
	}
	gotPrompt, _ := os.ReadFile(promptPath)
	if string(gotPrompt) == "drifted prompt" {
		t.Error("reconcile did not repair drifted prompt")
	}
}

func TestInitRefusesNonDirTarget(t *testing.T) {
	dest := t.TempDir()
	if err := os.WriteFile(filepath.Join(dest, "wiki"), []byte("file"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Init(dest, "wiki"); err == nil {
		t.Error("Init should fail when wiki target is an existing regular file")
	}
}

func TestInitRenameAndRefuseExistingDest(t *testing.T) {
	dest := t.TempDir()
	if err := Init(dest, "wiki"); err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dest, "wiki", "custom.md"), []byte("keep me"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Rename wiki -> docs.
	res, err := Reconcile(dest, ReconcileOptions{RenameTo: "docs"})
	if err != nil {
		t.Fatalf("Rename failed: %v", err)
	}
	if res.Mode != "rename" || res.RenamedFrom != "wiki" || res.WikiDir != "docs" {
		t.Errorf("unexpected rename result: %+v", res)
	}
	if _, err := os.Stat(filepath.Join(dest, "docs", "custom.md")); err != nil {
		t.Errorf("custom page missing after rename: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dest, "wiki")); !os.IsNotExist(err) {
		t.Error("old wiki/ directory still exists after rename")
	}
	rcData, _ := os.ReadFile(filepath.Join(dest, ".wikirc"))
	if !strings.Contains(string(rcData), `wiki_dir = "docs"`) || !strings.Contains(string(rcData), `"docs/"`) {
		t.Errorf(".wikirc not updated on rename:\n%s", rcData)
	}
	agentsData, _ := os.ReadFile(filepath.Join(dest, "AGENTS.md"))
	if !strings.Contains(string(agentsData), "docs/index.md") {
		t.Errorf("AGENTS.md shim not updated on rename:\n%s", agentsData)
	}

	// Refuse rename when destination already exists.
	if err := os.MkdirAll(filepath.Join(dest, "knowledge"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := Reconcile(dest, ReconcileOptions{RenameTo: "knowledge"}); err == nil {
		t.Error("Rename should refuse when destination directory already exists")
	}
}

func TestInitPreservesExistingWikirc(t *testing.T) {
	dest := t.TempDir()
	custom := []byte("wiki_dir = \"custom\"\nlog_lines = 42\n")
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
	if !strings.Contains(string(data), "log_lines = 42") {
		t.Errorf(".wikirc user setting lost on Init:\n%s", data)
	}
}

func TestInitCustomDir(t *testing.T) {
	dest := t.TempDir()
	err := Init(dest, "docs")
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dest, "docs", "index.md")); os.IsNotExist(err) {
		t.Error("missing docs/index.md")
	}
	data, err := os.ReadFile(filepath.Join(dest, ".wikirc"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `wiki_dir = "docs"`) || !strings.Contains(string(data), `"docs/"`) {
		t.Errorf(".wikirc did not remap to docs:\n%s", data)
	}
}

func TestInitPreservesUserShimsAndAppendsFencedPointer(t *testing.T) {
	dest := t.TempDir()
	custom := "# My custom agents instructions\n\nDo not lose this line.\n"
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
	text := string(data)
	if !strings.Contains(text, "Do not lose this line.") {
		t.Errorf("Init lost user content in AGENTS.md:\n%s", text)
	}
	if !strings.Contains(text, shimStartMarker) || !strings.Contains(text, "wiki/index.md") {
		t.Errorf("Init did not append fenced wiki pointer to AGENTS.md:\n%s", text)
	}

	// Second init must be idempotent (no duplicate fenced block).
	if err := Init(dest, "wiki"); err != nil {
		t.Fatalf("second Init failed: %v", err)
	}
	data2, _ := os.ReadFile(filepath.Join(dest, "AGENTS.md"))
	if strings.Count(string(data2), shimStartMarker) != 1 {
		t.Errorf("second Init duplicated fenced block in AGENTS.md:\n%s", data2)
	}
}

func TestSyncPrompts(t *testing.T) {
	dest := t.TempDir()
	updated, _, err := SyncPrompts(dest)
	if err != nil {
		t.Fatalf("SyncPrompts failed: %v", err)
	}
	if len(updated) == 0 {
		t.Fatal("SyncPrompts returned no updated files")
	}
	for _, f := range []string{
		".wiki-instructions/ingest.md",
		".wiki-instructions/wiki-maintainer.md",
		".github/prompts/wiki-ingest.prompt.md",
		".claude/commands/wiki-ingest.md",
		".pi/skills/wiki/SKILL.md",
		"AGENTS.md",
		"CLAUDE.md",
	} {
		if _, err := os.Stat(filepath.Join(dest, f)); os.IsNotExist(err) {
			t.Errorf("SyncPrompts missing %s", f)
		}
	}
	if _, err := os.Stat(filepath.Join(dest, "wiki")); !os.IsNotExist(err) {
		t.Error("SyncPrompts should not create wiki/")
	}
	assertPromptSymlinks(t, dest)
}

func TestSyncPromptsPreservesExistingShimsAndUserFiles(t *testing.T) {
	dest := t.TempDir()
	custom := "# My custom Claude instructions\n"
	if err := os.WriteFile(filepath.Join(dest, "CLAUDE.md"), []byte(custom), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := SyncPrompts(dest); err != nil {
		t.Fatalf("SyncPrompts failed: %v", err)
	}
	data, _ := os.ReadFile(filepath.Join(dest, "CLAUDE.md"))
	if string(data) != custom {
		t.Errorf("SyncPrompts modified custom CLAUDE.md:\n%s", data)
	}

	userFile := filepath.Join(dest, ".claude", "commands", "review.md")
	if err := os.WriteFile(userFile, []byte("user"), 0o644); err != nil {
		t.Fatal(err)
	}
	retired := filepath.Join(dest, ".wiki-instructions", "migrate-shims.md")
	if err := os.WriteFile(retired, []byte("stale"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, removed, err := SyncPrompts(dest)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(userFile); err != nil {
		t.Error("SyncPrompts deleted user-owned file")
	}
	if _, err := os.Stat(retired); !os.IsNotExist(err) {
		t.Error("SyncPrompts did not remove retired file")
	}
	if len(removed) == 0 {
		t.Error("expected removed list to report retired file")
	}
}

func TestInitRejectsUnsafeWikiDirs(t *testing.T) {
	dest := t.TempDir()
	for _, bad := range []string{"", ".", "..", "../escape", "../../escape", "/absolute/path", "docs/wiki", `docs\wiki`, ".git", ".github"} {
		if err := Init(dest, bad); err == nil {
			t.Errorf("Init with wiki dir %q should fail, got nil error", bad)
		}
	}
}

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
		}
	}
}
