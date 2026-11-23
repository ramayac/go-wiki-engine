package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// runCLI re-executes the test binary as wiki-engine (see TestMainReexec).
func runCLI(t *testing.T, dir string, args ...string) (string, int) {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^TestMainReexec$")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"WIKI_ENGINE_HELPER=1",
		"WIKI_ENGINE_HELPER_ARGS="+strings.Join(args, "\n"),
	)
	out, err := cmd.CombinedOutput()
	exit := 0
	if ee, ok := err.(*exec.ExitError); ok {
		exit = ee.ExitCode()
	} else if err != nil {
		t.Fatalf("helper failed to run: %v", err)
	}
	return string(out), exit
}

func victimFile(t *testing.T, root string) string {
	t.Helper()
	victim := filepath.Join(filepath.Dir(root), "victim.txt")
	if err := os.WriteFile(victim, []byte("precious"), 0o644); err != nil {
		t.Fatal(err)
	}
	return victim
}

func assertUntouched(t *testing.T, victim string) {
	t.Helper()
	got, err := os.ReadFile(victim)
	if err != nil || string(got) != "precious" {
		t.Errorf("victim file changed: %q (err %v)", got, err)
	}
}

func TestGitOptionInjectionViaArgument(t *testing.T) {
	root := setupMainTestWiki(t)
	victim := victimFile(t, root)
	for _, cmd := range []string{"changed", "candidates"} {
		out, exit := runCLI(t, root, cmd, "--", "--output=../victim.txt")
		if exit != 1 || !strings.Contains(out, "invalid diff range") {
			t.Errorf("%s: exit=%d, want 1 with invalid diff range:\n%s", cmd, exit, out)
		}
	}
	out, exit := runCLI(t, root, "refresh", "--", "--output=../victim.txt")
	if exit != 1 || !strings.Contains(out, "invalid diff range") {
		t.Errorf("refresh: exit=%d:\n%s", exit, out)
	}
	assertUntouched(t, victim)
}

func TestGitOptionInjectionViaWikirc(t *testing.T) {
	root := setupMainTestWiki(t)
	victim := victimFile(t, root)
	if err := os.WriteFile(filepath.Join(root, ".wikirc"), []byte("default_diff = \"--output=../victim.txt\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"lint"}, {"changed"}, {"candidates"}, {"refresh"}, {"watch", "--once"}} {
		out, exit := runCLI(t, root, args...)
		if exit != 1 || !strings.Contains(out, "invalid default_diff") {
			t.Errorf("%v: exit=%d, want 1 with invalid default_diff:\n%s", args, exit, out)
		}
	}
	assertUntouched(t, victim)
}

func TestDefaultDiffOnMasterBranch(t *testing.T) {
	root := setupMainTestWiki(t)
	gitCmd := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, out)
		}
	}
	gitCmd("branch", "-m", "main", "master")
	if err := os.WriteFile(filepath.Join(root, "feature.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitCmd("add", ".")
	gitCmd("commit", "-q", "-m", "feature work")

	// No .wikirc: the default must auto-detect `master`, not fail on `main`.
	for _, cmd := range []string{"changed", "candidates"} {
		out, exit := runCLI(t, root, cmd)
		if exit != 0 || !strings.Contains(out, "feature.go") {
			t.Errorf("%s on master repo: exit=%d:\n%s", cmd, exit, out)
		}
	}
	if out, exit := runCLI(t, root, "watch", "--once"); exit != 0 {
		t.Errorf("watch --once on master repo: exit=%d:\n%s", exit, out)
	}

	// A bad explicit range reports git's reason, not a generic message.
	out, exit := runCLI(t, root, "changed", "nosuch...HEAD")
	if exit != 1 || !strings.Contains(out, `unknown revision "nosuch"`) || strings.Contains(out, "exit status") {
		t.Errorf("bad range: exit=%d:\n%s", exit, out)
	}
}

func TestChangedOutsideGitRepoSaysSo(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".wikirc"), []byte("wiki_dir = \"wiki\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "wiki"), 0o755); err != nil {
		t.Fatal(err)
	}
	out, exit := runCLI(t, dir, "changed")
	if exit != 1 || !strings.Contains(out, "not a git repository") {
		t.Errorf("exit=%d, want 1 with not a git repository:\n%s", exit, out)
	}
}
