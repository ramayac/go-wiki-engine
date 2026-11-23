package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func initTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, out)
		}
	}
	run("init", "-q", "-b", "main")
	run("config", "user.email", "test@test")
	run("config", "user.name", "Test")
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "old.go"), []byte("package main\nfunc helper() int { return 42 }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "dead.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", ".")
	run("commit", "-q", "-m", "initial")
	run("checkout", "-q", "-b", "feature")
	return dir
}

func TestDiscoverChangesMergeBaseRenamesAndWorkingTree(t *testing.T) {
	dir := initTestRepo(t)
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, out)
		}
	}

	// Commit a rename and a deletion on the feature branch.
	run("mv", "old.go", "renamed.go")
	run("rm", "dead.go")
	run("commit", "-q", "-m", "rename and delete")

	// Add an unstaged modification and an untracked file.
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\nfunc main() { println(1) }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "untracked.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Working-tree enabled: sees rename, delete, unstaged edit, and untracked.
	cs, err := DiscoverChanges(dir, ChangeOptions{
		Base:               "main",
		IncludeWorkingTree: true,
	})
	if err != nil {
		t.Fatalf("DiscoverChanges failed: %v", err)
	}
	if cs.Base != "main" || cs.MergeBase == "" {
		t.Errorf("expected base=main and non-empty merge_base, got %+v", cs)
	}
	if cs.Renames["old.go"] != "renamed.go" {
		t.Errorf("expected rename old.go -> renamed.go, got %+v", cs.Renames)
	}
	hasDeleted := false
	for _, d := range cs.DeletedFiles {
		if d == "dead.go" {
			hasDeleted = true
		}
	}
	if !hasDeleted {
		t.Errorf("expected dead.go in DeletedFiles, got %v", cs.DeletedFiles)
	}
	hasUntracked := false
	for _, u := range cs.Untracked {
		if u == "untracked.go" {
			hasUntracked = true
		}
	}
	if !hasUntracked {
		t.Errorf("expected untracked.go in Untracked, got %v", cs.Untracked)
	}

	// Reproducible CI mode (IncludeWorkingTree=false): excludes unstaged/untracked.
	csCI, err := DiscoverChanges(dir, ChangeOptions{
		Base:               "main",
		IncludeWorkingTree: false,
	})
	if err != nil {
		t.Fatalf("DiscoverChanges CI mode failed: %v", err)
	}
	for _, f := range csCI.ChangedFiles {
		if f == "main.go" || f == "untracked.go" {
			t.Errorf("CI mode should exclude working-tree file %s", f)
		}
	}
	if len(csCI.Untracked) != 0 {
		t.Errorf("CI mode should have no untracked files, got %v", csCI.Untracked)
	}
}
