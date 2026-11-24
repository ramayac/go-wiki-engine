package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

func gitT(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func writeT(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// newRepoOnBranch creates a repo with one commit on the given branch.
func newRepoOnBranch(t *testing.T, branch string) string {
	t.Helper()
	dir := t.TempDir()
	gitT(t, dir, "init", "-q", "-b", branch)
	gitT(t, dir, "config", "user.email", "test@test")
	gitT(t, dir, "config", "user.name", "Test")
	writeT(t, filepath.Join(dir, "a.go"), "package a\n")
	gitT(t, dir, "add", ".")
	gitT(t, dir, "commit", "-q", "-m", "one")
	return dir
}

func TestValidateDiffRange(t *testing.T) {
	dir := newRepoOnBranch(t, "main")
	gitT(t, dir, "checkout", "-q", "-b", "feature")
	valid := []string{"HEAD", "main..feature", "main...feature", "HEAD~0..HEAD", "main..", "..HEAD"}
	for _, r := range valid {
		if err := ValidateDiffRange(dir, r); err != nil {
			t.Errorf("ValidateDiffRange(%q) = %v, want nil", r, err)
		}
	}
	invalid := []string{
		"", "--output=../victim.txt", "-x", "main..--output=x", "main...-x", "--output=x..HEAD",
		"main feature", "main..\nHEAD", "main\x00..HEAD", "nosuch..HEAD", "..", "a..b..c",
	}
	for _, r := range invalid {
		if err := ValidateDiffRange(dir, r); err == nil {
			t.Errorf("ValidateDiffRange(%q) = nil, want error", r)
		}
	}
}

func TestDiscoverChangesRejectsOptionInjection(t *testing.T) {
	root := t.TempDir()
	repo := filepath.Join(root, "repo")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	gitT(t, repo, "init", "-q", "-b", "main")
	gitT(t, repo, "config", "user.email", "test@test")
	gitT(t, repo, "config", "user.name", "Test")
	writeT(t, filepath.Join(repo, "a.go"), "package a\n")
	gitT(t, repo, "add", ".")
	gitT(t, repo, "commit", "-q", "-m", "one")

	victim := filepath.Join(root, "victim.txt")
	writeT(t, victim, "precious")

	for _, r := range []string{"--output=../victim.txt", "main..--output=../victim.txt", "-p"} {
		_, err := DiscoverChanges(repo, ChangeOptions{DiffRange: r, DefaultDiff: r})
		if err == nil || !strings.Contains(err.Error(), "invalid diff range") {
			t.Errorf("DiscoverChanges(%q) error = %v, want invalid diff range", r, err)
		}
	}
	_, err := DiscoverChanges(repo, ChangeOptions{Base: "--output=../victim.txt"})
	if err == nil {
		t.Error("DiscoverChanges with option-like Base: want error")
	}
	if got, _ := os.ReadFile(victim); string(got) != "precious" {
		t.Errorf("victim file was modified: %q", got)
	}
}

func TestDiscoverChangesErrorsAreSpecific(t *testing.T) {
	dir := newRepoOnBranch(t, "main")
	_, err := DiscoverChanges(dir, ChangeOptions{DiffRange: "nosuch...HEAD"})
	if err == nil || !strings.Contains(err.Error(), `unknown revision "nosuch"`) {
		t.Errorf("bad ref error = %v, want unknown revision", err)
	}
	_, err = DiscoverChanges(t.TempDir(), ChangeOptions{DiffRange: "main...HEAD"})
	if err == nil || !strings.Contains(err.Error(), "not a git repository") {
		t.Errorf("non-repo error = %v, want not a git repository", err)
	}
	if err != nil && strings.Contains(err.Error(), "exit status") {
		t.Errorf("error should not be a bare exit status: %v", err)
	}
}

func TestDiscoverChangesLegacyDefaultFallsBackToDetectedBase(t *testing.T) {
	dir := newRepoOnBranch(t, "master")
	gitT(t, dir, "checkout", "-q", "-b", "feature")
	writeT(t, filepath.Join(dir, "b.go"), "package b\n")
	gitT(t, dir, "add", ".")
	gitT(t, dir, "commit", "-q", "-m", "two")

	cs, err := DiscoverChanges(dir, ChangeOptions{
		DiffRange:   LegacyDefaultDiff,
		DefaultDiff: LegacyDefaultDiff,
	})
	if err != nil {
		t.Fatalf("DiscoverChanges with legacy default on master repo: %v", err)
	}
	if cs.Base != "master" || !slices.Equal(cs.ChangedFiles, []string{"b.go"}) {
		t.Errorf("Base=%q ChangedFiles=%v, want master and [b.go]", cs.Base, cs.ChangedFiles)
	}

	// An explicit range that differs from the legacy default stays authoritative.
	if _, err := DiscoverChanges(dir, ChangeOptions{DiffRange: "develop...HEAD", DefaultDiff: "develop...HEAD"}); err == nil {
		t.Error("explicit missing range must fail, not fall back")
	}
}

func TestDiscoverChangesSubdirectoryProject(t *testing.T) {
	repo := t.TempDir()
	gitT(t, repo, "init", "-q", "-b", "main")
	gitT(t, repo, "config", "user.email", "test@test")
	gitT(t, repo, "config", "user.name", "Test")
	proj := filepath.Join(repo, "proj")
	writeT(t, filepath.Join(proj, "wiki", "p.md"), "p\n")
	writeT(t, filepath.Join(proj, "main.go"), "package main\n")
	writeT(t, filepath.Join(proj, "café.go"), "package main\n")
	writeT(t, filepath.Join(proj, "old.go"), "package main\nfunc old() int { return 42 }\n")
	writeT(t, filepath.Join(proj, "dead.go"), "package main\n")
	writeT(t, filepath.Join(repo, "outside.go"), "package outside\n")
	gitT(t, repo, "add", ".")
	gitT(t, repo, "commit", "-q", "-m", "init")
	gitT(t, repo, "checkout", "-q", "-b", "feature")

	gitT(t, proj, "mv", "old.go", "new.go")
	gitT(t, proj, "rm", "-q", "dead.go")
	writeT(t, filepath.Join(proj, "café.go"), "package main // edited\n")
	writeT(t, filepath.Join(proj, "wiki", "p.md"), "p2\n")
	writeT(t, filepath.Join(repo, "outside.go"), "package outside // edited\n")
	gitT(t, repo, "add", ".")
	gitT(t, repo, "commit", "-q", "-m", "change")
	writeT(t, filepath.Join(proj, "naïve.go"), "package main\n")

	cs, err := DiscoverChanges(proj, ChangeOptions{DiffRange: "main...HEAD", DefaultDiff: "main...HEAD", IncludeWorkingTree: true})
	if err != nil {
		t.Fatal(err)
	}
	gotLive := cs.LiveNonWikiChanged("wiki")
	wantLive := []string{"café.go", "naïve.go", "new.go"}
	if !slices.Equal(gotLive, wantLive) {
		t.Errorf("LiveNonWikiChanged = %q, want %q", gotLive, wantLive)
	}
	for _, f := range cs.ChangedFiles {
		if strings.HasPrefix(f, "proj/") || strings.Contains(f, `"`) || f == "outside.go" {
			t.Errorf("ChangedFiles has non-root-relative or quoted path: %q", f)
		}
	}
	if !slices.Contains(cs.DeletedFiles, "dead.go") || cs.Renames["old.go"] != "new.go" {
		t.Errorf("DeletedFiles=%v Renames=%v", cs.DeletedFiles, cs.Renames)
	}
	if !slices.Equal(cs.WikiChanged("wiki"), []string{"wiki/p.md"}) {
		t.Errorf("WikiChanged = %v", cs.WikiChanged("wiki"))
	}
}

func TestPageCommitDatesNonASCIIAndSubdirectory(t *testing.T) {
	repo := t.TempDir()
	gitT(t, repo, "init", "-q", "-b", "main")
	gitT(t, repo, "config", "user.email", "test@test")
	gitT(t, repo, "config", "user.name", "Test")
	proj := filepath.Join(repo, "proj")
	writeT(t, filepath.Join(proj, "wiki", "é.md"), "x\n")
	writeT(t, filepath.Join(proj, "wiki", "plain.md"), "x\n")
	gitT(t, repo, "add", ".")
	cmd := exec.Command("git", "commit", "-q", "-m", "init")
	cmd.Dir = repo
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_DATE=2020-01-02T00:00:00+00:00", "GIT_COMMITTER_DATE=2020-01-02T00:00:00+00:00")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("commit: %v\n%s", err, out)
	}

	dates, err := PageCommitDates(proj, "wiki")
	if err != nil {
		t.Fatal(err)
	}
	want := time.Date(2020, 1, 2, 0, 0, 0, 0, time.UTC)
	for _, k := range []string{"wiki/é.md", "wiki/plain.md"} {
		if got, ok := dates[k]; !ok || !got.Equal(want) {
			t.Errorf("dates[%q] = %v (ok=%v), want %v; all: %v", k, got, ok, want, dates)
		}
	}
}

func TestCommitExistsRejectsNonHex(t *testing.T) {
	dir := newRepoOnBranch(t, "main")
	sha := gitT(t, dir, "rev-parse", "HEAD")
	if !CommitExists(dir, sha) || !CommitExists(dir, sha[:7]) {
		t.Error("CommitExists should accept full and short SHA")
	}
	for _, s := range []string{"--batch-all-objects", "-p", "HEAD", "abc"} {
		if CommitExists(dir, s) {
			t.Errorf("CommitExists(%q) = true, want false", s)
		}
	}
}

func TestShallowClone(t *testing.T) {
	origin := newRepoOnBranch(t, "main")
	first := gitT(t, origin, "rev-parse", "HEAD")
	writeT(t, filepath.Join(origin, "b.go"), "package b\n")
	gitT(t, origin, "add", ".")
	gitT(t, origin, "commit", "-q", "-m", "two")

	clone := filepath.Join(t.TempDir(), "clone")
	gitT(t, origin, "clone", "-q", "--depth", "1", "file://"+origin, clone)

	if IsShallow(origin) {
		t.Error("origin must not be shallow")
	}
	if !IsShallow(clone) {
		t.Error("clone must be shallow")
	}
	if CommitExists(clone, first) {
		t.Error("first commit must be absent from the depth-1 clone")
	}
}
