package engine

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/ramayac/go-wiki-engine/internal/config"
)

func gitIn(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func TestIgnoreMatches(t *testing.T) {
	cases := []struct {
		pattern, path string
		want          bool
	}{
		{"docs/*.md", "docs/a.md", true},
		{"docs/*.md", "other/a.md", false},
		{"docs/*.md", "docs/sub/a.md", false},
		{"docs/**/*.md", "docs/sub/deep/a.md", true},
		{"docs/**/*.md", "docs/a.md", true},
		{"**/gen/", "pkg/gen/x.go", true},
		{"vendor", "vendor/x/b.go", true},
		{"vendor", "src/vendor/b.go", true},
		{"vendor", "vendored/b.go", false},
		{"*.log", "a.log", true},
		{"*.log", "logs/a.log", true},
		{"*.log", "a.txt", false},
		{"wiki/", "wiki/index.md", true},
		{"wiki/", "wiki", false},
		{"wiki/", "docs/wiki/index.md", false},
		{"bin/tool", "bin/tool", true},
		{"bin/tool", "bin/other", false},
		{"/top.go", "top.go", true},
		{"/top.go", "sub/top.go", false},
		{"", "a.go", false},
	}
	for _, tc := range cases {
		if got := ignoreMatches(tc.pattern, tc.path); got != tc.want {
			t.Errorf("ignoreMatches(%q, %q) = %v, want %v", tc.pattern, tc.path, got, tc.want)
		}
	}
}

func TestCandidatesSubdirectoryProject(t *testing.T) {
	repo := t.TempDir()
	gitIn(t, repo, "init", "-q", "-b", "main")
	gitIn(t, repo, "config", "user.email", "test@test")
	gitIn(t, repo, "config", "user.name", "Test")
	proj := filepath.Join(repo, "proj")
	write := func(rel, content string) {
		p := filepath.Join(proj, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("wiki/index.md", "x\n")
	write("main.go", "package main\n")
	write("café.go", "package main\n")
	write("old.go", "package main\nfunc old() int { return 42 }\n")
	write("dead.go", "package main\n")
	write("docs/a.md", "a\n")
	gitIn(t, repo, "add", ".")
	gitIn(t, repo, "commit", "-q", "-m", "init")
	gitIn(t, repo, "checkout", "-q", "-b", "feature")
	gitIn(t, proj, "mv", "old.go", "new.go")
	gitIn(t, proj, "rm", "-q", "dead.go")
	write("café.go", "package main // edited\n")
	write("docs/a.md", "a2\n")
	write("wiki/index.md", "y\n")

	eng := New(&config.Config{
		WikiDir:     "wiki",
		DefaultDiff: "main...HEAD",
		Ignore:      []string{"wiki/", "docs/*.md"},
	}, proj)

	cands, err := eng.Candidates("main...HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"café.go", "new.go"}; !slices.Equal(cands, want) {
		t.Errorf("Candidates = %q, want %q", cands, want)
	}

	// `changed` keeps deleted files and the old side of renames for ingest.
	changed, err := eng.Changed("main...HEAD")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"dead.go", "old.go", "new.go", "café.go", "docs/a.md"} {
		if !slices.Contains(changed, want) {
			t.Errorf("Changed missing %q: %q", want, changed)
		}
	}
	for _, f := range changed {
		if strings.HasPrefix(f, "wiki/") || strings.HasPrefix(f, "proj/") || strings.HasPrefix(f, `"`) {
			t.Errorf("Changed has unexpected path %q", f)
		}
	}
}

func TestDiffRejectsDashPrefixedWikiDir(t *testing.T) {
	root := t.TempDir()
	eng := New(&config.Config{WikiDir: "-x"}, root)
	if _, err := eng.Diff("HEAD", "HEAD"); err == nil {
		t.Fatal("Diff with a dash-prefixed wiki_dir should fail validation")
	}
}

func TestDiffRejectsOptionLikeRefs(t *testing.T) {
	root := gitFixtureWiki(t)
	victim := filepath.Join(t.TempDir(), "victim.txt")
	if err := os.WriteFile(victim, []byte("precious"), 0o644); err != nil {
		t.Fatal(err)
	}
	eng := newTestEngine(root)
	if _, err := eng.Diff("--output="+victim, "HEAD"); err == nil {
		t.Error("Diff with option-like ref: want error")
	}
	if _, err := eng.Diff("HEAD", "--output="+victim); err == nil {
		t.Error("Diff with option-like ref: want error")
	}
	if got, _ := os.ReadFile(victim); string(got) != "precious" {
		t.Errorf("victim modified: %q", got)
	}
}

// shallowCloneWithWikiPage returns a depth-1 clone whose wiki page cites a
// commit that exists upstream but not in the clone.
func shallowCloneWithWikiPage(t *testing.T) (clone, missingSHA string) {
	t.Helper()
	origin := setupWiki(t)
	gitIn(t, origin, "init", "-q", "-b", "main")
	gitIn(t, origin, "config", "user.email", "test@test")
	gitIn(t, origin, "config", "user.name", "Test")
	gitIn(t, origin, "add", ".")
	gitIn(t, origin, "commit", "-q", "-m", "one")
	missingSHA = gitIn(t, origin, "rev-parse", "HEAD")
	if err := os.WriteFile(filepath.Join(origin, "extra.go"), []byte("package x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitIn(t, origin, "add", ".")
	gitIn(t, origin, "commit", "-q", "-m", "two")

	clone = filepath.Join(t.TempDir(), "clone")
	gitIn(t, origin, "clone", "-q", "--depth", "1", "file://"+origin, clone)
	gitIn(t, clone, "config", "user.email", "test@test")
	gitIn(t, clone, "config", "user.name", "Test")
	return clone, missingSHA
}

func TestCheckShallowCloneUnverifiableCommitIsWarning(t *testing.T) {
	clone, sha := shallowCloneWithWikiPage(t)
	page := filepath.Join(clone, "wiki", "operations", "ingest.md")
	body := "---\nid: ingest\ntype: concept\nlifecycle: current\ndescription: Ingest\n---\n# Ingest\n\nSee commit:" + sha + " for context.\n"
	if err := os.WriteFile(page, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	eng := newTestEngine(clone)
	res, err := eng.Check(CheckOptions{IncludeWorkingTree: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, b := range res.Broken {
		if b.Kind == "invalid_commit" {
			t.Errorf("shallow clone must not report invalid_commit BROKEN: %+v", b)
		}
	}
	found := false
	for _, w := range res.Warnings {
		if w.Kind == "unverifiable_commit" && w.Target == sha && strings.Contains(w.Message, "shallow clone") {
			found = true
		}
	}
	if !found {
		t.Errorf("want unverifiable_commit warning, got warnings %+v broken %+v", res.Warnings, res.Broken)
	}
}

func TestCheckFullCloneMissingCommitStaysBroken(t *testing.T) {
	root := gitFixtureWiki(t)
	page := filepath.Join(root, "wiki", "operations", "ingest.md")
	body := "---\nid: ingest\ntype: concept\nlifecycle: current\ndescription: Ingest\n---\n# Ingest\n\nSee commit:deadbeefdeadbeef for context.\n"
	if err := os.WriteFile(page, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := newTestEngine(root).Check(CheckOptions{Base: "main", IncludeWorkingTree: true})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, b := range res.Broken {
		if b.Kind == "invalid_commit" {
			found = true
		}
	}
	if !found {
		t.Errorf("want invalid_commit BROKEN on a full repo, got %+v", res)
	}
}

func TestStaleContentSkipsShallowClone(t *testing.T) {
	clone, _ := shallowCloneWithWikiPage(t)
	eng := newTestEngine(clone)
	eng.Cfg.StaleDays = 1
	old := time.Now().AddDate(-1, 0, 0)
	for _, rel := range []string{"schema.md", "repo-map.md"} {
		if err := os.Chtimes(filepath.Join(clone, "wiki", rel), old, old); err != nil {
			t.Fatal(err)
		}
	}
	lr := eng.LintWithOptions([]string{"stale-content"}, nil)
	var got []Issue
	for _, iss := range lr.Issues {
		if iss.Check == "stale-content" {
			got = append(got, iss)
		}
	}
	if len(got) != 1 || got[0].Severity != SevInfo || !strings.Contains(got[0].Message, "shallow clone") {
		t.Errorf("want one info issue saying the check was skipped for a shallow clone, got %+v", got)
	}
}
