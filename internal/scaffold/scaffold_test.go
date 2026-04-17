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
