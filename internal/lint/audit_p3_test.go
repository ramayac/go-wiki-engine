package lint

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ramayac/go-wiki-engine/internal/config"
	"github.com/ramayac/go-wiki-engine/internal/model"
)

// runChecks loads a snapshot and runs only the named checkers.
func runChecks(t *testing.T, root string, check ...string) []Issue {
	t.Helper()
	cfg := config.DefaultConfig()
	snap, err := model.LoadSnapshot(root, cfg.WikiDir)
	if err != nil {
		t.Fatal(err)
	}
	return Run(snap, cfg, check, nil).Issues
}

func issueMessages(issues []Issue) string {
	var b strings.Builder
	for _, i := range issues {
		fmt.Fprintf(&b, "%s:%d [%s] %s\n", i.File, i.Line, i.Check, i.Message)
	}
	return b.String()
}

func countCheck(issues []Issue, check string) int {
	n := 0
	for _, i := range issues {
		if i.Check == check {
			n++
		}
	}
	return n
}

func plainPage(id, body string) string {
	return "---\nid: " + id + "\ntype: concept\nlifecycle: current\ndescription: " + id + "\n---\n" + body
}

func phasesPage(rows string) string {
	return plainPage("phases", "# Phases\n\n| Phase | Name | Status | Exit |\n|---|---|---|---|\n"+rows+"\nSee [index](../index.md).\n")
}

func TestPhaseConsistencyUsesNearestLowerPhase(t *testing.T) {
	// A board numbered from 1 has no phase 0 to compare against.
	root := setupLintWiki(t, map[string]string{
		"wiki/prologue/phases.md": phasesPage("| 1 | A | completed | x |\n| 2 | B | in-progress | x |\n| 3 | C | not-started | x |\n"),
	})
	if got := countCheck(runChecks(t, root, "phase-consistency"), "phase-consistency"); got != 0 {
		t.Errorf("board from 1: want 0 issues, got %d", got)
	}

	// A gap in numbering compares with the nearest lower phase that exists.
	root = setupLintWiki(t, map[string]string{
		"wiki/prologue/phases.md": phasesPage("| 1 | A | completed | x |\n| 3 | C | in-progress | x |\n"),
	})
	if got := countCheck(runChecks(t, root, "phase-consistency"), "phase-consistency"); got != 0 {
		t.Errorf("gap in numbering: want 0 issues, got %d", got)
	}
}

func TestPhaseConsistencyStillReportsRealProblems(t *testing.T) {
	root := setupLintWiki(t, map[string]string{
		"wiki/prologue/phases.md": phasesPage("| 1 | A | not-started | x |\n| 2 | B | in-progress | x |\n| 4 | D | completed | x |\n"),
	})
	msgs := issueMessages(runChecks(t, root, "phase-consistency"))
	for _, want := range []string{
		"phase 2 is in-progress but phase 1 is not completed",
		"phase 4 is completed but phase 2 is not completed",
	} {
		if !strings.Contains(msgs, want) {
			t.Errorf("missing %q in:\n%s", want, msgs)
		}
	}
}

func TestPhaseConsistencyBlockedPhaseNeedsNoCompletedPredecessor(t *testing.T) {
	root := setupLintWiki(t, map[string]string{
		"wiki/prologue/phases.md": phasesPage("| 0 | A | in-progress | x |\n| 1 | B | blocked | x |\n"),
	})
	if got := countCheck(runChecks(t, root, "phase-consistency"), "phase-consistency"); got != 0 {
		t.Errorf("blocked phase after in-progress phase: want 0 issues, got %d", got)
	}
}

func TestMarkdownFormatTaskListMarkers(t *testing.T) {
	root := setupLintWiki(t, map[string]string{
		"wiki/tasks.md": plainPage("tasks", "# Tasks\n\n- [ ] (optional)\n- [x] (done)\n  * [X] (nested)\n"),
	})
	msgs := issueMessages(runChecks(t, root, "markdown-format"))
	if strings.Contains(msgs, "malformed markdown link") {
		t.Errorf("task-list markers flagged:\n%s", msgs)
	}
}

func TestMarkdownFormatSpacedLinkStillReported(t *testing.T) {
	root := setupLintWiki(t, map[string]string{
		"wiki/tasks.md": plainPage("tasks", "# Tasks\n\nSee [text] (url).\n\n- [ ] [text] (url)\n"),
	})
	issues := runChecks(t, root, "markdown-format")
	n := 0
	for _, i := range issues {
		if strings.Contains(i.Message, "malformed markdown link with spaces") {
			n++
		}
	}
	if n != 2 {
		t.Errorf("want 2 spaced-link issues, got %d:\n%s", n, issueMessages(issues))
	}
}

func TestBareURLAutolinkNotFlagged(t *testing.T) {
	root := setupLintWiki(t, map[string]string{
		"wiki/urls.md": plainPage("urls", "# URLs\n\nSee <https://example.com/x> for details.\n"),
	})
	if got := countCheck(runChecks(t, root, "bare-urls"), "bare-urls"); got != 0 {
		t.Errorf("autolink flagged: %d issues", got)
	}
}

func TestBareURLNakedStillFlagged(t *testing.T) {
	root := setupLintWiki(t, map[string]string{
		"wiki/urls.md": plainPage("urls", "# URLs\n\nSee https://example.com/x here.\n\nBoth <https://a.example.com> and https://b.example.com appear.\n"),
	})
	issues := runChecks(t, root, "bare-urls")
	if len(issues) != 2 {
		t.Fatalf("want 2 bare-url issues, got:\n%s", issueMessages(issues))
	}
	if !strings.Contains(issues[1].Message, "https://b.example.com") {
		t.Errorf("second issue should name b.example.com: %s", issues[1].Message)
	}
}

func TestReferenceLinkIgnoresIndexingProse(t *testing.T) {
	root := setupLintWiki(t, map[string]string{
		"wiki/math.md": plainPage("math", "# Math\n\nThe cell m[i][j] and a[0][1] and f(x)[2][3] hold values.\n\nUse `[a][b]` in code.\n"),
	})
	msgs := issueMessages(runChecks(t, root, "markdown-format"))
	if strings.Contains(msgs, "reference-style link") {
		t.Errorf("indexing prose flagged:\n%s", msgs)
	}
}

func TestReferenceLinkUndefinedStillReported(t *testing.T) {
	root := setupLintWiki(t, map[string]string{
		"wiki/refs.md": plainPage("refs", "# Refs\n\nRead [the docs][docs] now.\n\nAlso ![img][pic] here.\n"),
	})
	issues := runChecks(t, root, "markdown-format")
	n := 0
	for _, i := range issues {
		if strings.Contains(i.Message, "reference-style link") {
			n++
		}
	}
	if n != 2 {
		t.Errorf("want 2 reference-style issues, got %d:\n%s", n, issueMessages(issues))
	}
}

func identicalPages(n int) map[string]string {
	extra := map[string]string{}
	body := "# Dup\n\nalpha beta gamma delta epsilon zeta eta theta iota kappa lambda mu nu xi omicron pi rho sigma tau upsilon.\n"
	for i := 0; i < n; i++ {
		id := fmt.Sprintf("dup-%03d", i)
		extra["wiki/dups/"+id+".md"] = plainPage(id, body)
	}
	return extra
}

func TestDuplicateContentOutputIsCapped(t *testing.T) {
	root := setupLintWiki(t, identicalPages(80))
	issues := runChecks(t, root, "duplicate-content")
	// 80 pages give 3160 pairs: 50 shown plus one summary line.
	if len(issues) != 51 {
		t.Fatalf("want 51 issues, got %d", len(issues))
	}
	summary := 0
	for _, i := range issues {
		if i.Severity != SevWarn {
			t.Errorf("severity = %v, want warn", i.Severity)
		}
		if strings.Contains(i.Message, "more duplicate pair(s) not shown") {
			summary++
			want := "... and 3110 more duplicate pair(s) not shown (raise duplicate_threshold to reduce)"
			if i.Message != want {
				t.Errorf("summary = %q, want %q", i.Message, want)
			}
		}
	}
	if summary != 1 {
		t.Errorf("want 1 summary issue, got %d", summary)
	}
}

func TestDuplicateContentUnderCapIsComplete(t *testing.T) {
	root := setupLintWiki(t, identicalPages(5))
	issues := runChecks(t, root, "duplicate-content")
	if len(issues) != 10 {
		t.Fatalf("want 10 pair issues, got %d:\n%s", len(issues), issueMessages(issues))
	}
	if strings.Contains(issueMessages(issues), "not shown") {
		t.Error("summary line present although nothing was hidden")
	}
}

func brokenIndexWiki(t *testing.T) string {
	t.Helper()
	root := setupLintWiki(t, nil)
	p := filepath.Join(root, "wiki", "index.md")
	data, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	data = append(data, []byte("- [Gone](gone.md) | Missing page\n")...)
	if err := os.WriteFile(p, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestBrokenIndexLinkReportedOnce(t *testing.T) {
	root := brokenIndexWiki(t)
	issues := runChecks(t, root, "index-links", "cross-page-links")
	if len(issues) != 1 || issues[0].Check != "index-links" {
		t.Fatalf("want exactly one index-links issue, got:\n%s", issueMessages(issues))
	}
	// A full run reports it once as well.
	if got := strings.Count(issueMessages(runChecks(t, root)), "gone.md"); got != 1 {
		t.Errorf("full run reports gone.md %d times, want 1", got)
	}
}

func TestBrokenIndexLinkStillReportedByCrossPageAlone(t *testing.T) {
	root := brokenIndexWiki(t)
	issues := runChecks(t, root, "cross-page-links")
	if len(issues) != 1 || issues[0].Check != "cross-page-links" {
		t.Fatalf("want exactly one cross-page-links issue, got:\n%s", issueMessages(issues))
	}
}

func TestBrokenPageLinkStillReportedWithIndexLinks(t *testing.T) {
	root := setupLintWiki(t, map[string]string{
		"wiki/other.md": plainPage("other", "# Other\n\nSee [gone](gone.md).\n"),
	})
	issues := runChecks(t, root, "index-links", "cross-page-links")
	if countCheck(issues, "cross-page-links") != 1 {
		t.Errorf("non-index broken link must still be reported:\n%s", issueMessages(issues))
	}
}

func crlf(s string) string { return strings.ReplaceAll(s, "\n", "\r\n") }

func TestFixPreservesCRLF(t *testing.T) {
	root := setupLintWiki(t, map[string]string{
		"wiki/crlf.md": crlf(plainPage("crlf", "# CRLF\n\nSee [[schema]] for details.\n\nTail line.\n")),
		"wiki/lf.md":   plainPage("lf", "# LF\n\nSee [[schema]] for details.\n\nTail line.\n"),
	})
	res, err := Fix(root, "wiki")
	if err != nil {
		t.Fatal(err)
	}
	if res.TotalFixes == 0 {
		t.Fatal("expected fixes")
	}
	got, _ := os.ReadFile(filepath.Join(root, "wiki", "crlf.md"))
	s := string(got)
	if strings.Contains(s, "[[schema]]") {
		t.Fatalf("link was not converted:\n%q", s)
	}
	if strings.Count(s, "\n") != strings.Count(s, "\r\n") || !strings.Contains(s, "\r\n") {
		t.Errorf("CRLF file not 100%% CRLF: %q", s)
	}
	got, _ = os.ReadFile(filepath.Join(root, "wiki", "lf.md"))
	if strings.Contains(string(got), "\r") {
		t.Errorf("LF file gained CR: %q", got)
	}
}

func TestFixPreservesCRLFInIndex(t *testing.T) {
	root := setupLintWiki(t, map[string]string{
		"wiki/old.md": "---\nid: old\ntype: concept\nlifecycle: legacy\ndescription: Old\n---\n# Old\n\nText.\n",
	})
	idx := filepath.Join(root, "wiki", "index.md")
	data, _ := os.ReadFile(idx)
	data = append(data, []byte("- [old.md](old.md) | Old doc\n")...)
	if err := os.WriteFile(idx, []byte(crlf(string(data))), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Fix(root, "wiki"); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(idx)
	s := string(got)
	if !strings.Contains(s, "[legacy] Old doc") {
		t.Fatalf("index badge not added:\n%q", s)
	}
	if strings.Count(s, "\n") != strings.Count(s, "\r\n") {
		t.Errorf("index.md not 100%% CRLF: %q", s)
	}
}

func TestBOMPageHasFrontMatterAndFixKeepsBOM(t *testing.T) {
	const bom = "\xef\xbb\xbf"
	root := setupLintWiki(t, map[string]string{
		"wiki/bom.md": bom + plainPage("bom", "# BOM\n\nSee [[schema]] for details.\n"),
	})
	msgs := issueMessages(runChecks(t, root, "front-matter"))
	if strings.Contains(msgs, "missing front matter") {
		t.Errorf("BOM page reported as missing front matter:\n%s", msgs)
	}
	if _, err := Fix(root, "wiki"); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(filepath.Join(root, "wiki", "bom.md"))
	s := string(got)
	if !strings.HasPrefix(s, bom+"---") || strings.Count(s, "\xef\xbb\xbf") != 1 {
		t.Errorf("BOM not preserved exactly once: %q", s)
	}
	if strings.Contains(s, "[[schema]]") {
		t.Errorf("link not converted: %q", s)
	}
}

func TestReferenceLinkDefinedStillReported(t *testing.T) {
	root := setupLintWiki(t, map[string]string{
		"wiki/refs.md": plainPage("refs", "# Refs\n\nRead [the docs][docs] now.\n\n[docs]: https://example.com\n"),
	})
	if !strings.Contains(issueMessages(runChecks(t, root, "markdown-format")), "reference-style link: [the docs][docs]") {
		t.Error("defined reference-style link no longer reported")
	}
}
