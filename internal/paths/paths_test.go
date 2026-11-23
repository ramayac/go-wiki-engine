package paths

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestValidateWikiDir(t *testing.T) {
	valid := []string{"wiki", "docs", "knowledge-base", "wiki_v2", "日本語wiki"}
	for _, v := range valid {
		if err := ValidateWikiDir(v); err != nil {
			t.Errorf("ValidateWikiDir(%q) unexpected error: %v", v, err)
		}
	}

	invalid := []string{
		"",
		" ",
		" wiki",
		"wiki ",
		".",
		"..",
		"../wiki",
		"../../wiki",
		"/tmp/wiki",
		`\tmp\wiki`,
		"wiki/child",
		`wiki\child`,
		"C:wiki",
		".git",
		".github",
		".claude",
		".pi",
		".wiki-instructions",
		".wikirc",
		"wiki\x00bad",
		"%2e%2e",
		"wiki%2fchild",
		"-x",
		"--output=x",
		`my"wiki`,
		"my\nwiki",
		"my\twiki",
	}
	for _, bad := range invalid {
		err := ValidateWikiDir(bad)
		if err == nil {
			t.Errorf("ValidateWikiDir(%q) = nil, want error", bad)
		}
		if !errors.Is(err, ErrInvalidWikiDir) {
			t.Errorf("ValidateWikiDir(%q) error = %v, want ErrInvalidWikiDir", bad, err)
		}
	}
}

func TestResolveWithin(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()

	// Valid nested file.
	got, err := ResolveWithin(root, "wiki/prologue/schema.md")
	if err != nil {
		t.Fatalf("ResolveWithin valid path failed: %v", err)
	}
	want := filepath.Join(root, "wiki", "prologue", "schema.md")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}

	// Percent-encoded valid filename.
	got, err = ResolveWithin(root, "wiki/my%20page.md")
	if err != nil {
		t.Fatalf("ResolveWithin percent-encoded valid path failed: %v", err)
	}
	if filepath.Base(got) != "my page.md" {
		t.Errorf("percent-encoded base = %q, want 'my page.md'", filepath.Base(got))
	}

	// Traversal attempts.
	for _, bad := range []string{
		"../secret.md",
		"wiki/../../secret.md",
		"/etc/passwd",
		`..\secret.md`,
		"%2e%2e/secret.md",
		"wiki/%2e%2e/%2e%2e/secret.md",
		"",
	} {
		if _, err := ResolveWithin(root, bad); err == nil {
			t.Errorf("ResolveWithin(%q) succeeded, want ErrOutsideBoundary", bad)
		}
	}

	// Symlink escape.
	if runtime.GOOS != "windows" {
		secretFile := filepath.Join(outside, "secret.md")
		if err := os.WriteFile(secretFile, []byte("# Secret"), 0o644); err != nil {
			t.Fatal(err)
		}
		linkPath := filepath.Join(root, "escape.md")
		if err := os.Symlink(secretFile, linkPath); err != nil {
			t.Fatal(err)
		}
		if _, err := ResolveWithin(root, "escape.md"); err == nil {
			t.Error("ResolveWithin allowed symlink escape to outside file")
		}

		// Symlinked directory escape.
		dirLink := filepath.Join(root, "escape-dir")
		if err := os.Symlink(outside, dirLink); err != nil {
			t.Fatal(err)
		}
		if _, err := ResolveWithin(root, "escape-dir/nonexistent.md"); err == nil {
			t.Error("ResolveWithin allowed symlink directory escape for non-existent child")
		}
	}
}

func TestResolvePageRelative(t *testing.T) {
	root := t.TempDir()
	wikiRoot := filepath.Join(root, "wiki")
	subDir := filepath.Join(wikiRoot, "decisions")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Valid sibling and parent within wiki.
	_, rel, err := ResolvePageRelative(wikiRoot, subDir, "../index.md")
	if err != nil || rel != "index.md" {
		t.Fatalf("ResolvePageRelative(../index.md) = (%q, %v), want (index.md, nil)", rel, err)
	}

	// Escaping wiki to repo root must fail.
	if _, _, err := ResolvePageRelative(wikiRoot, subDir, "../../README.md"); err == nil {
		t.Error("ResolvePageRelative should reject link escaping wiki root")
	}

	// Percent-encoded traversal must fail.
	if _, _, err := ResolvePageRelative(wikiRoot, wikiRoot, "%2e%2e/README.md"); err == nil {
		t.Error("ResolvePageRelative should reject percent-encoded traversal")
	}

	// Absolute path must fail.
	if _, _, err := ResolvePageRelative(wikiRoot, wikiRoot, "/etc/passwd"); err == nil {
		t.Error("ResolvePageRelative should reject absolute path")
	}
}

func TestResolveRepoSource(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	wikiDir := filepath.Join(root, "wiki")
	if err := os.MkdirAll(wikiDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Valid repo-root-relative source.
	_, rel, exists, err := ResolveRepoSource(root, wikiDir, "main.go")
	if err != nil || !exists || rel != "main.go" {
		t.Fatalf("ResolveRepoSource(main.go) = (%q, %v, %v), want (main.go, true, nil)", rel, exists, err)
	}

	// Valid page-relative link to repo-root file.
	_, rel, exists, err = ResolveRepoSource(root, wikiDir, "../main.go")
	if err != nil || !exists || rel != "main.go" {
		t.Fatalf("ResolveRepoSource(../main.go) = (%q, %v, %v), want (main.go, true, nil)", rel, exists, err)
	}

	// Non-existent file inside repo returns exists=false, err=nil.
	_, rel, exists, err = ResolveRepoSource(root, wikiDir, "missing.go")
	if err != nil || exists || rel != "missing.go" {
		t.Fatalf("ResolveRepoSource(missing.go) = (%q, %v, %v), want (missing.go, false, nil)", rel, exists, err)
	}

	// Traversal outside repo must fail.
	if _, _, _, err := ResolveRepoSource(root, wikiDir, "../../outside.go"); err == nil {
		t.Error("ResolveRepoSource should reject path escaping repo")
	}

	if runtime.GOOS != "windows" {
		outFile := filepath.Join(outside, "secret.go")
		if err := os.WriteFile(outFile, []byte("secret"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(outFile, filepath.Join(root, "linked.go")); err != nil {
			t.Fatal(err)
		}
		if _, _, _, err := ResolveRepoSource(root, wikiDir, "linked.go"); err == nil {
			t.Error("ResolveRepoSource should reject symlink escaping repo")
		}
	}
}
