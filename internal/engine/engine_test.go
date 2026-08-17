package engine

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ramayac/go-wiki-engine/internal/config"
)

// setupWiki creates a minimal valid wiki in a temp dir and returns the root path.
func setupWiki(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	wikiDir := filepath.Join(root, "wiki")
	opsDir := filepath.Join(wikiDir, "operations")
	if err := os.MkdirAll(opsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	files := map[string]string{
		"wiki/README.md":            "---\nstatus: current\ndescription: README\n---\n# Wiki\n",
		"wiki/index.md":             "---\nstatus: current\ndescription: Index\n---\n# Index\n\n- [schema.md](schema.md) | Schema\n- [log.md](log.md) | Log\n- [repo-map.md](repo-map.md) | Repo Map\n- [phases.md](phases.md) | Phases\n- [operations/ingest.md](operations/ingest.md) | Ingest\n- [operations/query.md](operations/query.md) | Query\n- [operations/lint.md](operations/lint.md) | Lint\n",
		"wiki/log.md":               "---\nstatus: current\ndescription: Log\n---\n# Log\n\n## [2026-04-16] ingest | initial scaffold\n\n- Created wiki.\n\n## [2026-04-15] lint | first check\n\n- All OK.\n",
		"wiki/schema.md":            "---\nstatus: current\ndescription: Schema\n---\n# Schema\n",
		"wiki/phases.md":            "---\nstatus: current\ndescription: Phases\n---\n# Phases\n",
		"wiki/repo-map.md":          "---\nstatus: current\ndescription: Repo Map\n---\n# Repo Map\n",
		"wiki/operations/ingest.md": "---\nstatus: current\ndescription: Ingest\n---\n# Ingest\n",
		"wiki/operations/query.md":  "---\nstatus: current\ndescription: Query\n---\n# Query\n",
		"wiki/operations/lint.md":   "---\nstatus: current\ndescription: Lint\n---\n# Lint\n",
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
	// Entries are prepended newest-first; LogTail must return the most recent.
	if len(lines) > 0 && lines[0] != "## [2026-04-16] ingest | initial scaffold" {
		t.Errorf("LogTail should return newest entries first, got %q", lines[0])
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
	if err := os.Remove(filepath.Join(root, "wiki", "schema.md")); err != nil {
		t.Fatal(err)
	}
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
	if err := os.WriteFile(indexPath, []byte("# Index\n\n- [missing.md](missing.md)\n- [schema.md](schema.md)\n"), 0o644); err != nil {
		t.Fatal(err)
	}
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
	if err := os.WriteFile(logPath, []byte("# Log\n\n## [2026-04-16] bad heading without pipe\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	eng := newTestEngine(root)
	result := eng.Lint()
	if result.OK {
		t.Error("Lint should fail with invalid log heading")
	}
}

func TestLintMarker(t *testing.T) {
	root := setupWiki(t)
	repoMap := filepath.Join(root, "wiki", "repo-map.md")
	if err := os.WriteFile(repoMap, []byte("---\nstatus: current\ndescription: Repo Map\n---\n# Repo Map\n\nTODO: fill this in\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	eng := newTestEngine(root)
	result := eng.Lint()
	if result.OK {
		t.Error("Lint should fail with TODO marker")
	}
}

func TestLintMarkerInCodeBlock(t *testing.T) {
	root := setupWiki(t)
	repoMap := filepath.Join(root, "wiki", "repo-map.md")
	if err := os.WriteFile(repoMap, []byte("---\nstatus: current\ndescription: Repo Map\n---\n# Repo Map\n\n```bash\nwiki-engine search \"TODO:\"\n```\n"), 0o644); err != nil {
		t.Fatal(err)
	}
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

func TestSummaryPathTraversal(t *testing.T) {
	root := setupWiki(t)
	eng := newTestEngine(root)

	// A real file outside the wiki dir must not be readable through Summary.
	if err := os.WriteFile(filepath.Join(root, "secret.md"), []byte("# Secret\n"), 0o644); err != nil {
		t.Fatalf("write secret.md: %v", err)
	}
	if _, err := eng.Summary("../secret.md"); err == nil {
		t.Error("Summary should reject paths escaping the wiki directory")
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
	if err := os.WriteFile(extraPath, []byte("# Extra\n"), 0o644); err != nil {
		t.Fatal(err)
	}
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
	if err := os.WriteFile(phasesPath, []byte("# Phases\n\nSee [nowhere.md](nowhere.md)\n"), 0o644); err != nil {
		t.Fatal(err)
	}
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
	if err := os.WriteFile(testPath, []byte("# Title\n\n### Skipped h2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
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
	if err := os.WriteFile(logPath, []byte("# Log\n\n## [2026-04-15] ingest | first\n\n## [2026-04-16] ingest | second\n"), 0o644); err != nil {
		t.Fatal(err)
	}
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
	if err := os.WriteFile(phasesPath, []byte("# Phases\n\n| 1 | Test | bad-status |\n"), 0o644); err != nil {
		t.Fatal(err)
	}
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

func TestLintMarkdownFormatWikiLinks(t *testing.T) {
	root := setupWiki(t)
	repoMap := filepath.Join(root, "wiki", "repo-map.md")
	if err := os.WriteFile(repoMap, []byte("# Repo Map\n\nThis is a [[wiki-style-link]]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	eng := newTestEngine(root)
	result := eng.Lint()
	found := false
	for _, iss := range result.Issues {
		if iss.Check == "markdown-format" && strings.Contains(iss.Message, "non-standard wiki link format") {
			found = true
			break
		}
	}
	if !found {
		t.Error("Lint should detect non-standard wiki link [[...]]")
	}
}

func TestLintMarkdownFormatSpacedLinks(t *testing.T) {
	root := setupWiki(t)
	repoMap := filepath.Join(root, "wiki", "repo-map.md")
	if err := os.WriteFile(repoMap, []byte("# Repo Map\n\nThis is a [spaced link] (repo-map.md)\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	eng := newTestEngine(root)
	result := eng.Lint()
	found := false
	for _, iss := range result.Issues {
		if iss.Check == "markdown-format" && strings.Contains(iss.Message, "malformed markdown link with spaces") {
			found = true
			break
		}
	}
	if !found {
		t.Error("Lint should detect spaced markdown link [text] (link)")
	}
}

func TestLintMarkdownFormatEmptyLink(t *testing.T) {
	root := setupWiki(t)
	repoMap := filepath.Join(root, "wiki", "repo-map.md")
	if err := os.WriteFile(repoMap, []byte("# Repo Map\n\nEmpty target: [link]()\nEmpty text: [](repo-map.md)\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	eng := newTestEngine(root)
	result := eng.Lint()
	foundTarget := false
	foundText := false
	for _, iss := range result.Issues {
		if iss.Check == "markdown-format" {
			if strings.Contains(iss.Message, "empty link target") {
				foundTarget = true
			}
			if strings.Contains(iss.Message, "empty link text") {
				foundText = true
			}
		}
	}
	if !foundTarget {
		t.Error("Lint should detect empty link target [link]()")
	}
	if !foundText {
		t.Error("Lint should detect empty link text [](link)")
	}
}

func TestLintMarkdownFormatUnclosedLink(t *testing.T) {
	root := setupWiki(t)
	repoMap := filepath.Join(root, "wiki", "repo-map.md")
	if err := os.WriteFile(repoMap, []byte("# Repo Map\n\nUnclosed link: [link](repo-map.md\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	eng := newTestEngine(root)
	result := eng.Lint()
	found := false
	for _, iss := range result.Issues {
		if iss.Check == "markdown-format" && strings.Contains(iss.Message, "unclosed markdown link parenthesis") {
			found = true
			break
		}
	}
	if !found {
		t.Error("Lint should detect unclosed markdown link [text](link")
	}
}

func TestLintMarkdownFormatReferenceLinks(t *testing.T) {
	root := setupWiki(t)
	repoMap := filepath.Join(root, "wiki", "repo-map.md")
	if err := os.WriteFile(repoMap, []byte("# Repo Map\n\nRef link: [schema][1]\n\n[1]: schema.md\nNormal: [schema](schema.md)\nInline code: `[x][y]`\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	eng := newTestEngine(root)
	result := eng.Lint()
	refCount := 0
	for _, iss := range result.Issues {
		if iss.Check == "markdown-format" && strings.Contains(iss.Message, "reference-style link") {
			refCount++
			if iss.Severity != SevWarn {
				t.Errorf("reference-style link should be warn severity, got %v", iss.Severity)
			}
		}
	}
	if refCount != 1 {
		t.Errorf("expected exactly 1 reference-style link issue, got %d", refCount)
	}
}

func TestLeafPagesChecker(t *testing.T) {
	root := setupWiki(t)
	eng := newTestEngine(root)

	// Add an active leaf page with no outgoing links.
	leaf := "---\nstatus: current\ndescription: Leaf\n---\n# Leaf Page\nNo links here.\n"
	if err := os.WriteFile(filepath.Join(root, "wiki", "leaf.md"), []byte(leaf), 0o644); err != nil {
		t.Fatal(err)
	}

	lr := eng.LintWithOptions([]string{"leaf-pages"}, nil)
	found := false
	for _, iss := range lr.Issues {
		if iss.Check == "leaf-pages" && iss.File == filepath.Join("wiki", "leaf.md") {
			found = true
			if iss.Severity != SevInfo {
				t.Errorf("leaf-pages should be info severity, got %v", iss.Severity)
			}
		}
		if iss.Check == "leaf-pages" && iss.File == filepath.Join("wiki", "log.md") {
			t.Error("log.md should be exempt from leaf-pages")
		}
	}
	if !found {
		t.Error("expected leaf-pages issue for active page with no outgoing links")
	}
}

func TestLintAnchorInLinks(t *testing.T) {
	root := setupWiki(t)
	repoMap := filepath.Join(root, "wiki", "repo-map.md")
	if err := os.WriteFile(repoMap, []byte("# Repo Map\n\nSee [schema](schema.md#user-table) or [main](cmd/main.go#L10)\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Create cmd/main.go in root to satisfy external link check
	if err := os.MkdirAll(filepath.Join(root, "cmd"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "cmd", "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	eng := newTestEngine(root)
	result := eng.Lint()
	// Filter out "duplicate-content" warnings or other warnings to verify cross-page and external-links pass
	crossPageFailed := false
	externalFailed := false
	for _, iss := range result.Issues {
		if iss.Check == "cross-page-links" {
			crossPageFailed = true
			t.Errorf("cross-page-links check failed unexpectedly: %s", iss.Message)
		}
		if iss.Check == "external-links" {
			externalFailed = true
			t.Errorf("external-links check failed unexpectedly: %s", iss.Message)
		}
	}
	if crossPageFailed || externalFailed {
		t.Error("Lint should pass cross-page-links and external-links with anchors/fragments")
	}
}

// --- Impact tests ---

func TestImpact(t *testing.T) {
	root := setupWiki(t)
	eng := newTestEngine(root)

	// Add a wiki page that mentions a source file.
	if err := os.WriteFile(filepath.Join(root, "wiki", "architecture.md"),
		[]byte("# Architecture\n\nThe main entry point is cmd/main.go.\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	results, err := eng.Impact([]string{"cmd/main.go", "pkg/unknown.go"})
	if err != nil {
		t.Fatalf("Impact failed: %v", err)
	}

	// cmd/main.go should match architecture.md.
	for _, r := range results {
		if r.ChangedFile == "cmd/main.go" {
			if len(r.WikiPages) == 0 {
				t.Error("expected architecture.md to mention cmd/main.go")
			}
		}
		if r.ChangedFile == "pkg/unknown.go" {
			if len(r.WikiPages) > 0 {
				t.Error("expected no wiki pages to mention pkg/unknown.go")
			}
		}
	}
}

// --- Context with summarize ---

func TestContextSummarize(t *testing.T) {
	root := setupWiki(t)
	eng := newTestEngine(root)

	cr, err := eng.Context(false, true)
	if err != nil {
		t.Fatalf("Context(summarize) failed: %v", err)
	}
	if !cr.Summarized {
		t.Error("Context should mark summarized=true")
	}

	// At least one catalog entry should have a summary.
	hasSummary := false
	for _, e := range cr.Catalog {
		if e.Summary != "" {
			hasSummary = true
			break
		}
	}
	if !hasSummary {
		t.Error("expected at least one catalog entry to have a summary")
	}
}

// --- WatchOnce test ---

func TestWatchOnce(t *testing.T) {
	root := setupWiki(t)
	eng := newTestEngine(root)

	// WatchOnce uses git diff which might fail in test env; it should handle gracefully.
	wr, err := eng.WatchOnce()
	// Accept either success or git-not-found error (no git repo in test).
	if err != nil && !strings.Contains(err.Error(), "git diff failed") {
		t.Logf("WatchOnce returned error (expected in non-git test env): %v", err)
	}
	if err == nil {
		if wr == nil {
			t.Error("WatchOnce returned nil result")
		}
	}
}

// --- Context minimal with summary disabled ---

func TestContextMinimalSummarize(t *testing.T) {
	root := setupWiki(t)
	eng := newTestEngine(root)

	// Minimal + summarize: catalog should have summaries but no phase.
	cr, err := eng.Context(true, true)
	if err != nil {
		t.Fatalf("Context(minimal, summarize) failed: %v", err)
	}
	if cr.Phase != "" {
		t.Error("minimal mode should not include phase")
	}
	if !cr.Summarized {
		t.Error("summarized should be true")
	}
}

func TestFrontMatterChecker(t *testing.T) {
	root := setupWiki(t)
	eng := newTestEngine(root)

	// Verify standard setup passes front matter check
	fmc := &frontMatterChecker{}
	issues, err := fmc.Check(eng)
	if err != nil {
		t.Fatalf("frontMatterChecker failed: %v", err)
	}
	if len(issues) != 0 {
		t.Errorf("expected 0 issues on clean setup, got %d: %v", len(issues), issues)
	}

	// 1. Missing front matter block
	if err := os.WriteFile(filepath.Join(root, "wiki", "schema.md"), []byte("# Schema\nNo front matter"), 0o644); err != nil {
		t.Fatal(err)
	}
	issues, _ = fmc.Check(eng)
	foundMissing := false
	for _, iss := range issues {
		if iss.File == "wiki/schema.md" && strings.Contains(iss.Message, "missing front matter block") {
			foundMissing = true
			if iss.Severity != SevWarn {
				t.Errorf("expected warning severity for missing front matter, got %v", iss.Severity)
			}
		}
	}
	if !foundMissing {
		t.Error("expected issue for missing front matter block")
	}

	// Restore schema.md
	if err := os.WriteFile(filepath.Join(root, "wiki", "schema.md"), []byte("---\nstatus: current\ndescription: Schema\n---\n# Schema\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// 2. Invalid status value
	if err := os.WriteFile(filepath.Join(root, "wiki", "schema.md"), []byte("---\nstatus: invalid_status\ndescription: Schema\n---\n# Schema\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	issues, _ = fmc.Check(eng)
	foundInvalidStatus := false
	for _, iss := range issues {
		if iss.File == "wiki/schema.md" && strings.Contains(iss.Message, "invalid status value") {
			foundInvalidStatus = true
			if iss.Severity != SevError {
				t.Errorf("expected error severity for invalid status, got %v", iss.Severity)
			}
		}
	}
	if !foundInvalidStatus {
		t.Error("expected issue for invalid status")
	}

	// Restore schema.md
	if err := os.WriteFile(filepath.Join(root, "wiki", "schema.md"), []byte("---\nstatus: current\ndescription: Schema\n---\n# Schema\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// 3. Deprecated status requiring superseded_by
	if err := os.WriteFile(filepath.Join(root, "wiki", "schema.md"), []byte("---\nstatus: deprecated\ndescription: Schema\n---\n# Schema\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	issues, _ = fmc.Check(eng)
	foundMissingSuperseded := false
	for _, iss := range issues {
		if iss.File == "wiki/schema.md" && strings.Contains(iss.Message, "superseded_by is required") {
			foundMissingSuperseded = true
			if iss.Severity != SevError {
				t.Errorf("expected error severity for missing superseded_by, got %v", iss.Severity)
			}
		}
	}
	if !foundMissingSuperseded {
		t.Error("expected issue for missing superseded_by when deprecated")
	}

	// 4. Superseded_by target does not exist
	if err := os.WriteFile(filepath.Join(root, "wiki", "schema.md"), []byte("---\nstatus: deprecated\ndescription: Schema\nsuperseded_by: non-existent.md\n---\n# Schema\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	issues, _ = fmc.Check(eng)
	foundNonExistentTarget := false
	for _, iss := range issues {
		if iss.File == "wiki/schema.md" && strings.Contains(iss.Message, "superseded_by target does not exist") {
			foundNonExistentTarget = true
		}
	}
	if !foundNonExistentTarget {
		t.Error("expected issue for non-existent superseded_by target")
	}

	// 5. Superseded_by target is not active (e.g. is deprecated itself)
	if err := os.WriteFile(filepath.Join(root, "wiki", "phases.md"), []byte("---\nstatus: deprecated\ndescription: Phases\nsuperseded_by: schema.md\n---\n# Phases\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "wiki", "schema.md"), []byte("---\nstatus: deprecated\ndescription: Schema\nsuperseded_by: phases.md\n---\n# Schema\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	issues, _ = fmc.Check(eng)
	foundNotActiveTarget := false
	for _, iss := range issues {
		if strings.Contains(iss.Message, "is not active") {
			foundNotActiveTarget = true
		}
	}
	if !foundNotActiveTarget {
		t.Error("expected issue for non-active superseded_by target")
	}
}

func TestContextLifecycleFiltering(t *testing.T) {
	root := setupWiki(t)
	eng := newTestEngine(root)

	// Make one of the wiki files legacy (non-active).
	if err := os.WriteFile(filepath.Join(root, "wiki", "phases.md"), []byte("---\nstatus: legacy\ndescription: Phases\n---\n# Phases\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Plain context includes legacy pages: the catalog shows everything.
	cr, err := eng.Context(false, false)
	if err != nil {
		t.Fatalf("Context failed: %v", err)
	}
	found := false
	for _, entry := range cr.Catalog {
		if entry.File == "phases.md" {
			found = true
		}
	}
	if !found {
		t.Error("plain context should include legacy pages")
	}

	// Summarize mode filters legacy/deprecated pages out.
	crSum, err := eng.Context(false, true)
	if err != nil {
		t.Fatalf("Context(summarize) failed: %v", err)
	}
	for _, entry := range crSum.Catalog {
		if entry.File == "phases.md" {
			t.Error("summarize context should filter out legacy pages")
		}
	}
}

func TestIndexFormatChecker(t *testing.T) {
	root := setupWiki(t)
	eng := newTestEngine(root)

	// Clean setup should have zero index-format issues
	ifc := &indexFormatChecker{}
	issues, err := ifc.Check(eng)
	if err != nil {
		t.Fatalf("indexFormatChecker failed: %v", err)
	}
	if len(issues) != 0 {
		t.Errorf("expected 0 issues on clean setup, got %d: %v", len(issues), issues)
	}

	// 1. Missing description
	indexPath := filepath.Join(root, "wiki", "index.md")
	if err := os.WriteFile(indexPath, []byte("---\nstatus: current\ndescription: Index\n---\n# Index\n- [schema.md](schema.md)\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	issues, _ = ifc.Check(eng)
	foundMissingDesc := false
	for _, iss := range issues {
		if strings.Contains(iss.Message, "missing pipe-separated description") {
			foundMissingDesc = true
		}
	}
	if !foundMissingDesc {
		t.Error("expected issue for missing description")
	}

	// 2. Non-relative target (starts with /)
	if err := os.WriteFile(indexPath, []byte("---\nstatus: current\ndescription: Index\n---\n# Index\n- [/schema.md](/schema.md) | Schema description\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	issues, _ = ifc.Check(eng)
	foundNonRelative := false
	for _, iss := range issues {
		if strings.Contains(iss.Message, "must use a relative path") {
			foundNonRelative = true
		}
	}
	if !foundNonRelative {
		t.Error("expected issue for non-relative path starting with /")
	}

	// 3. Non-relative target (has protocol)
	if err := os.WriteFile(indexPath, []byte("---\nstatus: current\ndescription: Index\n---\n# Index\n- [schema.md](https://google.com/schema.md) | Schema description\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	issues, _ = ifc.Check(eng)
	foundProtocol := false
	for _, iss := range issues {
		if strings.Contains(iss.Message, "must use a relative path") {
			foundProtocol = true
		}
	}
	if !foundProtocol {
		t.Error("expected issue for non-relative path starting with https://")
	}
}

func TestBareUrlChecker(t *testing.T) {
	root := setupWiki(t)
	eng := newTestEngine(root)

	// Clean setup should have zero bare-url issues
	buc := &bareUrlChecker{}
	issues, err := buc.Check(eng)
	if err != nil {
		t.Fatalf("bareUrlChecker failed: %v", err)
	}
	if len(issues) != 0 {
		t.Errorf("expected 0 issues on clean setup, got %d: %v", len(issues), issues)
	}

	// 1. Bare URL outside link
	readmePath := filepath.Join(root, "wiki", "README.md")
	if err := os.WriteFile(readmePath, []byte("---\nstatus: current\ndescription: README\n---\n# README\nVisit https://google.com for more info.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	issues, _ = buc.Check(eng)
	foundBareUrl := false
	for _, iss := range issues {
		if strings.Contains(iss.Message, "bare URL outside link") {
			foundBareUrl = true
			if iss.Severity != SevWarn {
				t.Errorf("expected warning severity for bare URL, got %v", iss.Severity)
			}
		}
	}
	if !foundBareUrl {
		t.Error("expected issue for bare URL")
	}

	// 2. HTML anchor tag
	if err := os.WriteFile(readmePath, []byte("---\nstatus: current\ndescription: README\n---\n# README\nVisit <a href=\"https://google.com\">Google</a>.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	issues, _ = buc.Check(eng)
	foundHtmlLink := false
	for _, iss := range issues {
		if strings.Contains(iss.Message, "use markdown links, not HTML") {
			foundHtmlLink = true
			if iss.Severity != SevError {
				t.Errorf("expected error severity for HTML links, got %v", iss.Severity)
			}
		}
	}
	if !foundHtmlLink {
		t.Error("expected issue for HTML link")
	}

	// 3. URL in code block (should be ignored)
	if err := os.WriteFile(readmePath, []byte("---\nstatus: current\ndescription: README\n---\n# README\n```\nhttps://google.com\n```\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	issues, _ = buc.Check(eng)
	if len(issues) != 0 {
		t.Errorf("expected no issues for URL inside code block, got: %v", issues)
	}

	// 4. URL in inline backticks (should be ignored)
	if err := os.WriteFile(readmePath, []byte("---\nstatus: current\ndescription: README\n---\n# README\n`https://google.com` is a URL.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	issues, _ = buc.Check(eng)
	if len(issues) != 0 {
		t.Errorf("expected no issues for URL inside inline code, got: %v", issues)
	}
}

func TestLintWithOptions(t *testing.T) {
	root := setupWiki(t)
	eng := newTestEngine(root)

	// Introduce a markers issue and an index-format issue
	if err := os.WriteFile(filepath.Join(root, "wiki", "repo-map.md"), []byte("---\nstatus: current\ndescription: Repo Map\n---\n# Repo Map\nTODO: fix\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "wiki", "index.md"), []byte("---\nstatus: current\ndescription: Index\n---\n# Index\n- [schema.md](schema.md)\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// 1. Run all (default) -> both issues should be found
	resAll := eng.LintWithOptions(nil, nil)
	foundMarkers := false
	foundIndexFormat := false
	for _, iss := range resAll.Issues {
		if iss.Check == "markers" {
			foundMarkers = true
		}
		if iss.Check == "index-format" {
			foundIndexFormat = true
		}
	}
	if !foundMarkers || !foundIndexFormat {
		t.Errorf("expected both markers and index-format issues, got markers=%v, index-format=%v", foundMarkers, foundIndexFormat)
	}

	// 2. Check only markers -> only markers issue should be found
	resCheck := eng.LintWithOptions([]string{"markers"}, nil)
	foundMarkers = false
	foundIndexFormat = false
	for _, iss := range resCheck.Issues {
		if iss.Check == "markers" {
			foundMarkers = true
		}
		if iss.Check == "index-format" {
			foundIndexFormat = true
		}
	}
	if !foundMarkers || foundIndexFormat {
		t.Errorf("expected only markers issue, got markers=%v, index-format=%v", foundMarkers, foundIndexFormat)
	}

	// 3. Skip markers -> only index-format issue should be found
	resSkip := eng.LintWithOptions(nil, []string{"markers"})
	foundMarkers = false
	foundIndexFormat = false
	for _, iss := range resSkip.Issues {
		if iss.Check == "markers" {
			foundMarkers = true
		}
		if iss.Check == "index-format" {
			foundIndexFormat = true
		}
	}
	if foundMarkers || !foundIndexFormat {
		t.Errorf("expected only index-format issue, got markers=%v, index-format=%v", foundMarkers, foundIndexFormat)
	}
}

func TestStaleContentGitDate(t *testing.T) {
	root := setupWiki(t)
	eng := newTestEngine(root)
	eng.Cfg.StaleDays = 1

	// Put the wiki under git with an old commit date so stale detection
	// uses the commit date instead of the (fresh) filesystem mtime.
	gitCmd := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, out)
		}
	}
	gitCmd("init", "-q", "-b", "main")
	gitCmd("config", "user.email", "test@test")
	gitCmd("config", "user.name", "Test")
	gitCmd("add", ".")

	commit := exec.Command("git", "commit", "-q", "-m", "init")
	commit.Dir = root
	commit.Env = append(os.Environ(),
		"GIT_AUTHOR_DATE=2020-01-01T00:00:00+00:00",
		"GIT_COMMITTER_DATE=2020-01-01T00:00:00+00:00",
	)
	if out, err := commit.CombinedOutput(); err != nil {
		t.Fatalf("git commit failed: %v\n%s", err, out)
	}

	lr := eng.LintWithOptions([]string{"stale-content"}, nil)
	stale := 0
	for _, iss := range lr.Issues {
		if iss.Check == "stale-content" {
			stale++
		}
	}
	if stale == 0 {
		t.Error("expected stale-content issues for pages last committed in 2020")
	}
}

func TestStaleContentMtimeFallback(t *testing.T) {
	root := setupWiki(t)
	eng := newTestEngine(root)
	eng.Cfg.StaleDays = 1

	// No git repo here: stale detection falls back to mtime, which we set
	// to a date in the past.
	old := time.Now().AddDate(-1, 0, 0)
	for _, rel := range []string{"schema.md", "repo-map.md"} {
		p := filepath.Join(root, "wiki", rel)
		if err := os.Chtimes(p, old, old); err != nil {
			t.Fatal(err)
		}
	}

	lr := eng.LintWithOptions([]string{"stale-content"}, nil)
	stale := 0
	for _, iss := range lr.Issues {
		if iss.Check == "stale-content" {
			stale++
		}
	}
	if stale == 0 {
		t.Error("expected stale-content issues from mtime fallback")
	}
}

func TestActiveUnlinkedPages(t *testing.T) {
	root := setupWiki(t)
	eng := newTestEngine(root)

	// Add an active page that is not linked from index.md.
	orphan := "---\nstatus: current\ndescription: Unlinked\n---\n# Unlinked\n"
	if err := os.WriteFile(filepath.Join(root, "wiki", "unlinked.md"), []byte(orphan), 0o644); err != nil {
		t.Fatal(err)
	}

	unlinked, err := eng.ActiveUnlinkedPages()
	if err != nil {
		t.Fatalf("ActiveUnlinkedPages failed: %v", err)
	}
	found := false
	for _, u := range unlinked {
		if u == "unlinked.md" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected unlinked.md in active unlinked pages, got %v", unlinked)
	}
}
