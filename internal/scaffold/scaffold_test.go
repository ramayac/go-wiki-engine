package scaffold

import (
	"os"
	"path/filepath"
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
		"wiki/log.md",
		"wiki/schema.md",
		"wiki/phases.md",
		"wiki/repo-map.md",
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
}

func TestInitRefuses(t *testing.T) {
	dest := t.TempDir()
	os.MkdirAll(filepath.Join(dest, "wiki"), 0o755)
	err := Init(dest, "wiki")
	if err == nil {
		t.Error("Init should refuse when wiki/ already exists")
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

	// Verify the canonical prompt files were written.
	required := []string{
		".github/prompts/wiki-ingest.prompt.md",
		".github/prompts/wiki-query.prompt.md",
		".github/prompts/wiki-refresh.prompt.md",
		".github/prompts/wiki-onboard.prompt.md",
		".github/instructions/wiki-maintainer.instructions.md",
	}
	for _, f := range required {
		p := filepath.Join(dest, f)
		if _, err := os.Stat(p); os.IsNotExist(err) {
			t.Errorf("SyncPrompts missing expected file: %s", f)
		}
	}

	// Wiki content and .wikirc should NOT have been created.
	if _, err := os.Stat(filepath.Join(dest, "wiki")); !os.IsNotExist(err) {
		t.Error("SyncPrompts should not create wiki/")
	}
	if _, err := os.Stat(filepath.Join(dest, ".wikirc")); !os.IsNotExist(err) {
		t.Error("SyncPrompts should not create .wikirc")
	}
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
