package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ramayac/go-wiki-engine/internal/config"
)

// setupWiki creates a minimal valid wiki in a temp dir and returns the root path.
func setupWiki(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	wikiDir := filepath.Join(root, "wiki")
	opsDir := filepath.Join(wikiDir, "operations")
	os.MkdirAll(opsDir, 0o755)

	files := map[string]string{
		"wiki/README.md":            "# Wiki\n",
		"wiki/index.md":             "# Index\n\n- [schema.md](schema.md)\n- [log.md](log.md)\n- [repo-map.md](repo-map.md)\n- [phases.md](phases.md)\n- [operations/ingest.md](operations/ingest.md)\n- [operations/query.md](operations/query.md)\n- [operations/lint.md](operations/lint.md)\n",
		"wiki/log.md":               "# Log\n\n## [2026-04-16] ingest | initial scaffold\n\n- Created wiki.\n\n## [2026-04-15] lint | first check\n\n- All OK.\n",
		"wiki/schema.md":            "# Schema\n",
		"wiki/phases.md":            "# Phases\n",
		"wiki/repo-map.md":          "# Repo Map\n",
		"wiki/operations/ingest.md": "# Ingest\n",
		"wiki/operations/query.md":  "# Query\n",
		"wiki/operations/lint.md":   "# Lint\n",
	}
	for rel, content := range files {
		p := filepath.Join(root, rel)
		os.MkdirAll(filepath.Dir(p), 0o755)
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func newTestEngine(root string) *Engine {
	cfg := &config.Config{
		WikiDir:     "wiki",
		DefaultDiff: "main...HEAD",
		LogLines:    10,
		Ignore:      []string{"wiki/", "bin/", "*.log"},
	}
	return New(cfg, root)
}

func TestList(t *testing.T) {
	root := setupWiki(t)
	eng := newTestEngine(root)
	files, err := eng.List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(files) != 9 {
		t.Errorf("List returned %d files, want 9, got: %v", len(files), files)
	}
}

func TestHeadings(t *testing.T) {
	root := setupWiki(t)
	eng := newTestEngine(root)
	entries, err := eng.Headings()
	if err != nil {
		t.Fatalf("Headings failed: %v", err)
	}
	if len(entries) == 0 {
		t.Error("Headings returned no entries")
	}
	found := false
	for _, e := range entries {
		if strings.Contains(e.Heading, "# Index") {
			found = true
		}
	}
	if !found {
		t.Error("expected heading containing '# Index'")
	}
}

func TestSearch(t *testing.T) {
	root := setupWiki(t)
	eng := newTestEngine(root)

	results, err := eng.Search("Schema")
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(results) == 0 {
		t.Error("Search for 'Schema' returned no results")
	}

	// Case insensitive.
	results2, err := eng.Search("schema")
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(results2) != len(results) {
		t.Errorf("case-insensitive search mismatch: %d vs %d", len(results2), len(results))
	}
}

func TestSearchEmpty(t *testing.T) {
	root := setupWiki(t)
	eng := newTestEngine(root)
	_, err := eng.Search("")
	if err == nil {
		t.Error("expected error for empty query")
	}
}

func TestLogTail(t *testing.T) {
	root := setupWiki(t)
	eng := newTestEngine(root)

	lines, err := eng.LogTail(10)
	if err != nil {
		t.Fatalf("LogTail failed: %v", err)
	}
	if len(lines) != 2 {
		t.Errorf("LogTail returned %d lines, want 2", len(lines))
	}

	// With limit.
	lines1, err := eng.LogTail(1)
	if err != nil {
		t.Fatalf("LogTail failed: %v", err)
	}
	if len(lines1) != 1 {
		t.Errorf("LogTail(1) returned %d lines, want 1", len(lines1))
	}
}

func TestLintOK(t *testing.T) {
	root := setupWiki(t)
	eng := newTestEngine(root)
	result := eng.Lint()
	if !result.OK {
		t.Errorf("Lint failed on valid wiki: %v", result.Messages)
	}
}

func TestLintMissingFile(t *testing.T) {
	root := setupWiki(t)
	os.Remove(filepath.Join(root, "wiki", "schema.md"))
	eng := newTestEngine(root)
	result := eng.Lint()
	if result.OK {
		t.Error("Lint should fail when required file is missing")
	}
	found := false
	for _, m := range result.Messages {
		if strings.Contains(m, "schema.md") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected message about schema.md, got: %v", result.Messages)
	}
}

func TestLintBrokenLink(t *testing.T) {
	root := setupWiki(t)
	// Add a broken link to index.
	indexPath := filepath.Join(root, "wiki", "index.md")
	os.WriteFile(indexPath, []byte("# Index\n\n- [missing.md](missing.md)\n- [schema.md](schema.md)\n"), 0o644)
	eng := newTestEngine(root)
	result := eng.Lint()
	if result.OK {
		t.Error("Lint should fail with broken link")
	}
	found := false
	for _, m := range result.Messages {
		if strings.Contains(m, "broken index link") && strings.Contains(m, "missing.md") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected broken link message, got: %v", result.Messages)
	}
}

func TestLintInvalidLogHeading(t *testing.T) {
	root := setupWiki(t)
	logPath := filepath.Join(root, "wiki", "log.md")
	os.WriteFile(logPath, []byte("# Log\n\n## [2026-04-16] bad heading without pipe\n"), 0o644)
	eng := newTestEngine(root)
	result := eng.Lint()
	if result.OK {
		t.Error("Lint should fail with invalid log heading")
	}
}

func TestLintMarker(t *testing.T) {
	root := setupWiki(t)
	repoMap := filepath.Join(root, "wiki", "repo-map.md")
	os.WriteFile(repoMap, []byte("# Repo Map\n\nTODO: fill this in\n"), 0o644)
	eng := newTestEngine(root)
	result := eng.Lint()
	if result.OK {
		t.Error("Lint should fail with TODO marker")
	}
}

func TestLintMarkerInCodeBlock(t *testing.T) {
	root := setupWiki(t)
	repoMap := filepath.Join(root, "wiki", "repo-map.md")
	os.WriteFile(repoMap, []byte("# Repo Map\n\n```bash\nwiki-engine search \"TODO:\"\n```\n"), 0o644)
	eng := newTestEngine(root)
	result := eng.Lint()
	if !result.OK {
		t.Errorf("Lint should pass when markers are inside code blocks: %v", result.Messages)
	}
}

func TestIsIgnored(t *testing.T) {
	eng := newTestEngine(t.TempDir())
	tests := []struct {
		path string
		want bool
	}{
		{"wiki/index.md", true},
		{"bin/tool", true},
		{"output.log", true},
		{"src/main.go", false},
		{"README.md", false},
	}
	for _, tt := range tests {
		got := eng.isIgnored(tt.path)
		if got != tt.want {
			t.Errorf("isIgnored(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}

func TestStats(t *testing.T) {
	root := setupWiki(t)
	eng := newTestEngine(root)
	st, err := eng.Stats()
	if err != nil {
		t.Fatalf("Stats failed: %v", err)
	}
	if st.Files != 9 {
		t.Errorf("Stats Files = %d, want 9", st.Files)
	}
	if st.Headings == 0 {
		t.Error("Stats should have headings > 0")
	}
	if st.TotalLines == 0 {
		t.Error("Stats should have lines > 0")
	}
}

func TestContext(t *testing.T) {
	root := setupWiki(t)
	eng := newTestEngine(root)
	cr, err := eng.Context(false, false)
	if err != nil {
		t.Fatalf("Context failed: %v", err)
	}
	if cr.Files == 0 {
		t.Error("Context should have files > 0")
	}
	if len(cr.Catalog) == 0 {
		t.Error("Context should have catalog entries from index.md")
	}
	if len(cr.RecentLog) == 0 {
		t.Error("Context should have recent log entries")
	}
}

func TestContextMinimal(t *testing.T) {
	root := setupWiki(t)
	eng := newTestEngine(root)
	cr, err := eng.Context(true, false)
	if err != nil {
		t.Fatalf("Context(minimal) failed: %v", err)
	}
	if cr.Phase != "" {
		t.Error("Context(minimal) should not include phase")
	}
}

func TestSummary(t *testing.T) {
	root := setupWiki(t)
	eng := newTestEngine(root)
	sr, err := eng.Summary("README.md")
	if err != nil {
		t.Fatalf("Summary failed: %v", err)
	}
	if sr.FirstHeader == "" {
		t.Error("Summary should return first header")
	}
	if sr.File != "README.md" {
		t.Errorf("Summary File = %q, want README.md", sr.File)
	}
}

func TestSummaryNotFound(t *testing.T) {
	root := setupWiki(t)
	eng := newTestEngine(root)
	_, err := eng.Summary("nonexistent.md")
	if err == nil {
		t.Error("Summary should error for nonexistent page")
	}
}

func TestRelevant(t *testing.T) {
	root := setupWiki(t)
	eng := newTestEngine(root)
	results, err := eng.Relevant("schema", 3)
	if err != nil {
		t.Fatalf("Relevant failed: %v", err)
	}
	if len(results) == 0 {
		t.Error("Relevant should find matches for 'schema'")
	}
	// First result should be schema.md itself.
	if len(results) > 0 && !strings.Contains(results[0].File, "schema.md") {
		t.Logf("first result is %s (expected schema.md to rank high)", results[0].File)
	}
}

func TestRelevantEmpty(t *testing.T) {
	root := setupWiki(t)
	eng := newTestEngine(root)
	_, err := eng.Relevant("", 3)
	if err == nil {
		t.Error("Relevant should error for empty query")
	}
}

func TestLintIssues(t *testing.T) {
	root := setupWiki(t)
	eng := newTestEngine(root)
	result := eng.Lint()
	if !result.OK {
		t.Errorf("Lint failed on valid wiki: %v", result.Messages)
	}
	if result.Issues == nil {
		t.Error("Lint should populate Issues field")
	}
}

func TestLintOrphans(t *testing.T) {
	root := setupWiki(t)
	// Add an extra page not in index.md.
	extraPath := filepath.Join(root, "wiki", "extra.md")
	os.WriteFile(extraPath, []byte("# Extra\n"), 0o644)
	eng := newTestEngine(root)
	result := eng.Lint()
	found := false
	for _, iss := range result.Issues {
		if iss.Check == "orphans" && strings.Contains(iss.File, "extra.md") {
			found = true
		}
	}
	if !found {
		t.Error("Lint should detect orphan page extra.md")
	}
}

func TestLintCrossPageLink(t *testing.T) {
	root := setupWiki(t)
	// Add a broken link in a non-index page.
	phasesPath := filepath.Join(root, "wiki", "phases.md")
	os.WriteFile(phasesPath, []byte("# Phases\n\nSee [nowhere.md](nowhere.md)\n"), 0o644)
	eng := newTestEngine(root)
	result := eng.Lint()
	found := false
	for _, iss := range result.Issues {
		if iss.Check == "cross-page-links" && strings.Contains(iss.Message, "nowhere.md") {
			found = true
		}
	}
	if !found {
		t.Error("Lint should detect cross-page broken link")
	}
}

func TestLintHeadingHierarchy(t *testing.T) {
	root := setupWiki(t)
	// Create a page with a skipped heading level.
	testPath := filepath.Join(root, "wiki", "repo-map.md")
	os.WriteFile(testPath, []byte("# Title\n\n### Skipped h2\n"), 0o644)
	eng := newTestEngine(root)
	result := eng.Lint()
	found := false
	for _, iss := range result.Issues {
		if iss.Check == "heading-hierarchy" {
			found = true
		}
	}
	if !found {
		t.Error("Lint should detect heading level skip (h1 -> h3)")
	}
}

func TestLintLogChronology(t *testing.T) {
	root := setupWiki(t)
	// Write log with wrong order.
	logPath := filepath.Join(root, "wiki", "log.md")
	os.WriteFile(logPath, []byte("# Log\n\n## [2026-04-15] ingest | first\n\n## [2026-04-16] ingest | second\n"), 0o644)
	eng := newTestEngine(root)
	result := eng.Lint()
	found := false
	for _, iss := range result.Issues {
		if iss.Check == "log-chronology" {
			found = true
		}
	}
	if !found {
		t.Error("Lint should detect log entries not in descending order")
	}
}

func TestLintPhaseConsistency(t *testing.T) {
	root := setupWiki(t)
	// Write phases.md with invalid status.
	phasesPath := filepath.Join(root, "wiki", "phases.md")
	os.WriteFile(phasesPath, []byte("# Phases\n\n| 1 | Test | bad-status |\n"), 0o644)
	eng := newTestEngine(root)
	result := eng.Lint()
	found := false
	for _, iss := range result.Issues {
		if iss.Check == "phase-consistency" {
			found = true
		}
	}
	if !found {
		t.Error("Lint should detect unknown phase status")
	}
}
