package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// setupNestedWiki creates a wiki using the organized layout: index.md and
// README.md at the root, prologue files in prologue/, operations in
// operations/. Returns the repo root path.
func setupNestedWiki(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{
		"wiki/README.md": "---\nstatus: current\ndescription: README\n---\n# Wiki\n\n- [index.md](index.md)\n",
		"wiki/index.md": "---\nstatus: current\ndescription: Index\n---\n# Index\n\n" +
			"- [prologue/schema.md](prologue/schema.md) | Schema\n" +
			"- [prologue/log.md](prologue/log.md) | Log\n" +
			"- [prologue/repo-map.md](prologue/repo-map.md) | Repo Map\n" +
			"- [prologue/phases.md](prologue/phases.md) | Phases\n" +
			"- [operations/ingest.md](operations/ingest.md) | Ingest\n" +
			"- [operations/query.md](operations/query.md) | Query\n" +
			"- [operations/lint.md](operations/lint.md) | Lint\n",
		"wiki/prologue/log.md": "---\nstatus: current\ndescription: Log\n---\n# Log\n\n" +
			"## [2026-04-16] ingest | initial scaffold\n\n- Created wiki.\n\n" +
			"## [2026-04-15] lint | first check\n\n- All OK.\n",
		"wiki/prologue/schema.md": "---\nstatus: current\ndescription: Schema\n---\n# Schema\n\nSee [log.md](log.md).\n",
		"wiki/prologue/phases.md": "---\nstatus: current\ndescription: Phases\n---\n# Phases\n\n" +
			"| 0 | Bootstrap | completed | Required files exist |\n" +
			"| 1 | Populate | in-progress | Architecture recorded |\n\n" +
			"Related: [../index.md](../index.md)\n",
		"wiki/prologue/repo-map.md": "---\nstatus: current\ndescription: Repo Map\n---\n# Repo Map\n\nSee [schema.md](schema.md).\n",
		"wiki/operations/ingest.md": "---\nstatus: current\ndescription: Ingest\n---\n# Ingest\n\nSee [../prologue/log.md](../prologue/log.md).\n",
		"wiki/operations/query.md":  "---\nstatus: current\ndescription: Query\n---\n# Query\n\nSee [ingest.md](ingest.md).\n",
		"wiki/operations/lint.md":   "---\nstatus: current\ndescription: Lint\n---\n# Lint\n\nSee [query.md](query.md).\n",
	}
	for rel, content := range files {
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestNestedLayoutLogTail(t *testing.T) {
	root := setupNestedWiki(t)
	eng := newTestEngine(root)

	lines, err := eng.LogTail(10)
	if err != nil {
		t.Fatalf("LogTail failed on nested layout: %v", err)
	}
	if len(lines) != 2 {
		t.Errorf("LogTail returned %d lines, want 2", len(lines))
	}
}

func TestNestedLayoutLintOK(t *testing.T) {
	root := setupNestedWiki(t)
	eng := newTestEngine(root)
	result := eng.Lint()
	if !result.OK {
		t.Errorf("Lint failed on organized-layout wiki: %v", result.Messages)
	}
	// The nested log must not be reported as a leaf page.
	for _, iss := range result.Issues {
		if iss.Check == "leaf-pages" && strings.Contains(iss.File, "log.md") {
			t.Errorf("prologue/log.md should be exempt from leaf-pages, got: %v", iss)
		}
	}
}

func TestNestedLayoutLogCheckers(t *testing.T) {
	root := setupNestedWiki(t)
	logPath := filepath.Join(root, "wiki", "prologue", "log.md")
	if err := os.WriteFile(logPath, []byte("# Log\n\n## [2026-04-16] bad heading without pipe\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	eng := newTestEngine(root)
	result := eng.Lint()
	foundHeading := false
	for _, iss := range result.Issues {
		if iss.Check == "log-headings" {
			foundHeading = true
			if !strings.Contains(iss.File, filepath.Join("wiki", "prologue", "log.md")) {
				t.Errorf("log-headings should report prologue/log.md, got %s", iss.File)
			}
		}
	}
	if !foundHeading {
		t.Error("Lint should detect invalid log heading in prologue/log.md")
	}

	// Chronology violation in the nested log.
	if err := os.WriteFile(logPath, []byte("# Log\n\n## [2026-04-15] a | first\n\n## [2026-04-16] b | second\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	result = eng.Lint()
	foundChron := false
	for _, iss := range result.Issues {
		if iss.Check == "log-chronology" {
			foundChron = true
		}
	}
	if !foundChron {
		t.Error("Lint should detect log chronology violation in prologue/log.md")
	}
}

func TestNestedLayoutPhaseConsistency(t *testing.T) {
	root := setupNestedWiki(t)
	phasesPath := filepath.Join(root, "wiki", "prologue", "phases.md")
	if err := os.WriteFile(phasesPath, []byte("# Phases\n\n| 1 | Test | bad-status |\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	eng := newTestEngine(root)
	result := eng.Lint()
	found := false
	for _, iss := range result.Issues {
		if iss.Check == "phase-consistency" {
			found = true
			if !strings.Contains(iss.File, filepath.Join("wiki", "prologue", "phases.md")) {
				t.Errorf("phase-consistency should report prologue/phases.md, got %s", iss.File)
			}
		}
	}
	if !found {
		t.Error("Lint should detect unknown phase status in prologue/phases.md")
	}
}

func TestNestedLayoutCurrentPhase(t *testing.T) {
	root := setupNestedWiki(t)
	eng := newTestEngine(root)
	phase := eng.currentPhase()
	if !strings.Contains(phase, "Phase 1") || !strings.Contains(phase, "in-progress") {
		t.Errorf("currentPhase = %q, want latest phase row", phase)
	}
}

func TestRequiredFilesLegacyFallback(t *testing.T) {
	// Organized layout with log.md moved back to the flat legacy path must
	// still pass required-files.
	root := setupNestedWiki(t)
	nested := filepath.Join(root, "wiki", "prologue", "log.md")
	flat := filepath.Join(root, "wiki", "log.md")
	if err := os.Rename(nested, flat); err != nil {
		t.Fatal(err)
	}
	eng := newTestEngine(root)
	result := eng.Lint()
	for _, iss := range result.Issues {
		if iss.Check == "required-files" {
			t.Errorf("legacy flat log.md should satisfy required-files, got: %v", iss)
		}
	}
}

func TestRequiredFilesMissingBothCandidates(t *testing.T) {
	root := setupNestedWiki(t)
	// Neither prologue/schema.md nor schema.md exists after removal.
	if err := os.Remove(filepath.Join(root, "wiki", "prologue", "schema.md")); err != nil {
		t.Fatal(err)
	}
	eng := newTestEngine(root)
	result := eng.Lint()
	found := false
	for _, iss := range result.Issues {
		if iss.Check == "required-files" && strings.Contains(iss.Message, "schema.md") {
			found = true
		}
	}
	if !found {
		t.Errorf("required-files should report missing schema.md when both candidates are absent, got: %v", result.Messages)
	}
}

// TestLintSubdirLinkMustBePageRelative guards the strict cross-page link
// resolution: a page inside a subdirectory must use ../ to reach siblings in
// other directories. The removed wiki-root fallback used to mask this class
// of broken link.
func TestLintSubdirLinkMustBePageRelative(t *testing.T) {
	root := setupNestedWiki(t)
	repoMap := filepath.Join(root, "wiki", "prologue", "repo-map.md")
	// links operations/lint.md WITHOUT ../ — resolves to prologue/operations/lint.md.
	if err := os.WriteFile(repoMap, []byte("---\nstatus: current\ndescription: Repo Map\n---\n# Repo Map\n\nSee [Lint](operations/lint.md).\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	eng := newTestEngine(root)
	result := eng.LintWithOptions([]string{"cross-page-links"}, nil)
	found := false
	for _, iss := range result.Issues {
		if iss.Check == "cross-page-links" && strings.Contains(iss.Message, "operations/lint.md") {
			found = true
		}
	}
	if !found {
		t.Error("cross-page-links should flag a subdir page linking a sibling without ../")
	}

	// The corrected ../ link must pass.
	if err := os.WriteFile(repoMap, []byte("---\nstatus: current\ndescription: Repo Map\n---\n# Repo Map\n\nSee [Lint](../operations/lint.md).\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	result = eng.LintWithOptions([]string{"cross-page-links"}, nil)
	for _, iss := range result.Issues {
		if iss.Check == "cross-page-links" {
			t.Errorf("correct ../ link should pass, got issue: %v", iss)
		}
	}
}

func TestIsCanonicalFile(t *testing.T) {
	tests := []struct {
		name    string
		wikiRel string
		want    bool
	}{
		{"log.md", "prologue/log.md", true},
		{"log.md", "log.md", true},
		{"log.md", "decisions/log.md", false},
		{"phases.md", "prologue/phases.md", true},
		{"phases.md", "phases.md", true},
		{"index.md", "index.md", true},
		{"index.md", "prologue/index.md", false},
		{"README.md", "README.md", true},
	}
	for _, tt := range tests {
		if got := isCanonicalFile(tt.name, tt.wikiRel); got != tt.want {
			t.Errorf("isCanonicalFile(%q, %q) = %v, want %v", tt.name, tt.wikiRel, got, tt.want)
		}
	}
}
