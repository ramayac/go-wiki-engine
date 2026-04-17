package config
package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.WikiDir != "wiki" {
		t.Errorf("WikiDir = %q, want %q", cfg.WikiDir, "wiki")
	}
	if cfg.DefaultDiff != "main...HEAD" {
		t.Errorf("DefaultDiff = %q, want %q", cfg.DefaultDiff, "main...HEAD")
	}
	if cfg.LogLines != 10 {
		t.Errorf("LogLines = %d, want %d", cfg.LogLines, 10)
	}
	if len(cfg.Ignore) == 0 {
		t.Error("Ignore should have default entries")
	}
}

func TestLoadMissing(t *testing.T) {
	cfg, err := Load(t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.WikiDir != "wiki" {
		t.Errorf("WikiDir = %q, want %q", cfg.WikiDir, "wiki")
	}
}

func TestLoadCustom(t *testing.T) {
	dir := t.TempDir()
	content := `wiki_dir = "docs"
default_diff = "develop...HEAD"
log_lines = 5

ignore = [
  "vendor/",
  "dist/",
  "*.bak",
]
`
	if err := os.WriteFile(filepath.Join(dir, ".wikirc"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.WikiDir != "docs" {
		t.Errorf("WikiDir = %q, want %q", cfg.WikiDir, "docs")
	}
	if cfg.DefaultDiff != "develop...HEAD" {
		t.Errorf("DefaultDiff = %q, want %q", cfg.DefaultDiff, "develop...HEAD")
	}
	if cfg.LogLines != 5 {
		t.Errorf("LogLines = %d, want %d", cfg.LogLines, 5)
	}
	wantIgnore := []string{"vendor/", "dist/", "*.bak"}
	if len(cfg.Ignore) != len(wantIgnore) {
		t.Fatalf("Ignore length = %d, want %d", len(cfg.Ignore), len(wantIgnore))
	}
	for i, v := range wantIgnore {
		if cfg.Ignore[i] != v {
			t.Errorf("Ignore[%d] = %q, want %q", i, cfg.Ignore[i], v)
		}
	}
}

func TestLoadComments(t *testing.T) {
	dir := t.TempDir()
	content := `# This is a comment
wiki_dir = "knowledge"
# Another comment

log_lines = 20
`
	if err := os.WriteFile(filepath.Join(dir, ".wikirc"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.WikiDir != "knowledge" {
		t.Errorf("WikiDir = %q, want %q", cfg.WikiDir, "knowledge")
	}
	if cfg.LogLines != 20 {
		t.Errorf("LogLines = %d, want %d", cfg.LogLines, 20)
	}
}

func TestParseLogLines(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"10", 10},
		{"5", 5},
		{"0", 10}, // fallback
		{"abc", 10},
		{"25", 25},
	}
	for _, tt := range tests {
		got := parseLogLines(tt.input)
		if got != tt.want {
			t.Errorf("parseLogLines(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}
