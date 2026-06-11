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

func TestLoadNewKeys(t *testing.T) {
	dir := t.TempDir()
	content := `wiki_dir = "docs"
duplicate_threshold = 0.5
stale_days = 14
context_summarize = true
cache_enabled = false
cache_max_mb = 10
watch_interval = 120
fail_severity = "error"
`
	if err := os.WriteFile(filepath.Join(dir, ".wikirc"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.DuplicateThreshold != 0.5 {
		t.Errorf("DuplicateThreshold = %f, want 0.5", cfg.DuplicateThreshold)
	}
	if cfg.StaleDays != 14 {
		t.Errorf("StaleDays = %d, want 14", cfg.StaleDays)
	}
	if !cfg.ContextSummarize {
		t.Error("ContextSummarize should be true")
	}
	if cfg.CacheEnabled {
		t.Error("CacheEnabled should be false")
	}
	if cfg.CacheMaxMB != 10 {
		t.Errorf("CacheMaxMB = %d, want 10", cfg.CacheMaxMB)
	}
	if cfg.WatchInterval != 120 {
		t.Errorf("WatchInterval = %d, want 120", cfg.WatchInterval)
	}
	if cfg.FailSeverity != "error" {
		t.Errorf("FailSeverity = %q, want %q", cfg.FailSeverity, "error")
	}
}

func TestParseBool(t *testing.T) {
	tests := []struct {
		input    string
		fallback bool
		want     bool
	}{
		{"true", false, true},
		{"yes", false, true},
		{"1", false, true},
		{"false", true, false},
		{"no", true, false},
		{"0", true, false},
		{"garbage", true, true},   // fallback
		{"garbage", false, false}, // fallback
	}
	for _, tt := range tests {
		got := parseBool(tt.input, tt.fallback)
		if got != tt.want {
			t.Errorf("parseBool(%q, %v) = %v, want %v", tt.input, tt.fallback, got, tt.want)
		}
	}
}

func TestParseFloat(t *testing.T) {
	tests := []struct {
		input    string
		fallback float64
		want     float64
	}{
		{"0.7", 0.5, 0.7},
		{"1.0", 0.5, 1.0},
		{"0.0", 0.5, 0.5},     // <=0 falls back
		{"2.0", 0.5, 0.5},     // >1 falls back
		{"abc", 0.3, 0.3},     // non-numeric falls back
		{"0.555", 0.5, 0.555},
	}
	for _, tt := range tests {
		got := parseFloat(tt.input, tt.fallback)
		if got != tt.want {
			t.Errorf("parseFloat(%q, %f) = %f, want %f", tt.input, tt.fallback, got, tt.want)
		}
	}
}
