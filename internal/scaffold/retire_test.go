package scaffold

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

func digestOf(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

// registerShipped adds the digest of content to the shipped digests of the
// retired path rel for the length of the test.
func registerShipped(t *testing.T, rel, content string) {
	t.Helper()
	old, ok := retiredManagedFiles[rel]
	if !ok {
		t.Fatalf("%s is not a retired path", rel)
	}
	retiredManagedFiles[rel] = append(append([]string{}, old...), digestOf(content))
	t.Cleanup(func() { retiredManagedFiles[rel] = old })
}

func TestShippedDigestsTable(t *testing.T) {
	hex64 := regexp.MustCompile(`^[0-9a-f]{64}$`)
	for _, name := range []string{"onboard", "lint", "refresh", "watch"} {
		for _, rel := range []string{
			".wiki-instructions/" + name + ".md",
			".claude/commands/wiki-" + name + ".md",
			".github/prompts/wiki-" + name + ".prompt.md",
		} {
			digests := retiredManagedFiles[rel]
			if len(digests) == 0 {
				t.Errorf("%s has no shipped digests", rel)
			}
			for _, d := range digests {
				if !hex64.MatchString(d) {
					t.Errorf("%s: %q is not a SHA-256 hex digest", rel, d)
				}
			}
		}
	}
}

func TestRetiredPromptsAreNotShipped(t *testing.T) {
	for _, name := range []string{"onboard", "lint", "refresh", "watch"} {
		for _, rel := range []string{
			"files/.wiki-instructions/" + name + ".md",
			"files/.claude/commands/wiki-" + name + ".md",
			"files/.github/prompts/wiki-" + name + ".prompt.md",
		} {
			if _, err := fs.Stat(files, rel); err == nil {
				t.Errorf("retired prompt %s is still embedded", rel)
			}
		}
	}
	want := []string{"ingest", "plan", "query", "save", "seed", "upgrade"}
	if !reflect.DeepEqual(promptWorkflows, want) {
		t.Errorf("promptWorkflows = %v, want %v", promptWorkflows, want)
	}
}

// snapshot maps every file and symlink under root to its content or link target.
func snapshot(t *testing.T, root string) map[string]string {
	t.Helper()
	snap := map[string]string{}
	err := filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(root, p)
		if info.Mode()&os.ModeSymlink != 0 {
			target, err := os.Readlink(p)
			if err != nil {
				return err
			}
			snap[filepath.ToSlash(rel)] = "-> " + target
			return nil
		}
		b, err := os.ReadFile(p)
		snap[filepath.ToSlash(rel)] = string(b)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return snap
}

func TestSyncPromptsMigratesOldLayout(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("old layout relies on symlinks")
	}
	var warn bytes.Buffer
	old := warnOut
	warnOut = &warn
	t.Cleanup(func() { warnOut = old })

	const (
		pristineOnboard = "old onboard prompt\n"
		pristineLint    = "old lint prompt\n"
		editedLint      = "old lint prompt, edited by the user\n"
		pristineRefresh = "old refresh prompt\n"
		pristineWatch   = "old watch prompt\n"
	)
	registerShipped(t, ".wiki-instructions/onboard.md", pristineOnboard)
	registerShipped(t, ".wiki-instructions/lint.md", pristineLint)
	registerShipped(t, ".wiki-instructions/refresh.md", pristineRefresh)
	registerShipped(t, ".wiki-instructions/watch.md", pristineWatch)

	repo := t.TempDir()
	canonical := map[string]string{
		"onboard": pristineOnboard,
		"lint":    editedLint,
		"refresh": pristineRefresh,
		"watch":   pristineWatch,
		"ingest":  "old ingest prompt\n",
		"query":   "old query prompt\n",
		"upgrade": "old upgrade prompt\n",
	}
	for name, content := range canonical {
		mustWrite(t, filepath.Join(repo, ".wiki-instructions", name+".md"), content)
		target := "../../.wiki-instructions/" + name + ".md"
		for _, link := range []string{".claude/commands/wiki-" + name + ".md", ".github/prompts/wiki-" + name + ".prompt.md"} {
			p := filepath.Join(repo, link)
			if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(target, p); err != nil {
				t.Fatal(err)
			}
		}
	}
	mustWrite(t, filepath.Join(repo, ".claude/commands/wiki-custom.md"), "mine\n")
	mustWrite(t, filepath.Join(repo, "wiki/prologue/repo-map.md"), "page\n")
	mustWrite(t, filepath.Join(repo, "wiki/index.md"), "index\n")

	_, removed, err := SyncPrompts(repo)
	if err != nil {
		t.Fatal(err)
	}

	exists := func(rel string) bool {
		_, err := os.Lstat(filepath.Join(repo, rel))
		return err == nil
	}
	for _, name := range []string{"onboard", "lint", "refresh", "watch"} {
		for _, rel := range []string{".claude/commands/wiki-" + name + ".md", ".github/prompts/wiki-" + name + ".prompt.md"} {
			if exists(rel) {
				t.Errorf("retired link %s was not removed", rel)
			}
		}
	}
	for _, name := range []string{"onboard", "refresh", "watch"} {
		if exists(".wiki-instructions/" + name + ".md") {
			t.Errorf("pristine %s.md was not deleted", name)
		}
		if exists(".wiki-instructions/" + name + ".md.bak") {
			t.Errorf("pristine %s.md was backed up", name)
		}
	}
	if exists(".wiki-instructions/lint.md") {
		t.Error("edited lint.md was not moved away")
	}
	if b, _ := os.ReadFile(filepath.Join(repo, ".wiki-instructions/lint.md.bak")); string(b) != editedLint {
		t.Errorf("lint.md.bak = %q, want the edited content", b)
	}
	if got := strings.Count(warn.String(), "\n"); got != 1 || !strings.Contains(warn.String(), "lint.md.bak") {
		t.Errorf("want one stderr line naming the backup, got %q", warn.String())
	}
	if len(removed) == 0 {
		t.Error("removed list is empty")
	}
	for _, name := range promptWorkflows {
		for _, link := range []string{".claude/commands/wiki-" + name + ".md", ".github/prompts/wiki-" + name + ".prompt.md"} {
			fi, err := os.Lstat(filepath.Join(repo, link))
			if err != nil || fi.Mode()&os.ModeSymlink == 0 {
				t.Errorf("%s is not a symlink (err %v)", link, err)
			}
		}
		if !exists(".wiki-instructions/" + name + ".md") {
			t.Errorf(".wiki-instructions/%s.md is missing", name)
		}
	}
	if b, _ := os.ReadFile(filepath.Join(repo, ".claude/commands/wiki-custom.md")); string(b) != "mine\n" {
		t.Errorf("custom prompt changed: %q", b)
	}
	if b, _ := os.ReadFile(filepath.Join(repo, "wiki/prologue/repo-map.md")); string(b) != "page\n" {
		t.Errorf("wiki page changed: %q", b)
	}

	// A second run changes nothing and warns about nothing.
	before := snapshot(t, repo)
	warn.Reset()
	_, removed, err = SyncPrompts(repo)
	if err != nil {
		t.Fatal(err)
	}
	if len(removed) != 0 || warn.Len() != 0 {
		t.Errorf("second run removed %v and warned %q", removed, warn.String())
	}
	if after := snapshot(t, repo); !reflect.DeepEqual(before, after) {
		t.Error("second run changed the repo")
	}
}

func TestRetiredRegularFileRules(t *testing.T) {
	var warn bytes.Buffer
	old := warnOut
	warnOut = &warn
	t.Cleanup(func() { warnOut = old })
	registerShipped(t, ".wiki-instructions/watch.md", "shipped watch\n")

	repo := t.TempDir()
	target := filepath.Join(repo, ".wiki-instructions/watch.md")
	mustWrite(t, target, "user edit\n")
	mustWrite(t, target+".bak", "older backup\n")

	if _, _, err := SyncPrompts(repo); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(target); !os.IsNotExist(err) {
		t.Error("edited retired file was not moved")
	}
	if b, _ := os.ReadFile(target + ".bak"); string(b) != "older backup\n" {
		t.Errorf("existing .bak was overwritten: %q", b)
	}
	if b, _ := os.ReadFile(target + ".bak.1"); string(b) != "user edit\n" {
		t.Errorf(".bak.1 = %q, want the edited content", b)
	}

	// A symlink at a retired path that points outside .wiki-instructions/ stays.
	mustWrite(t, filepath.Join(repo, "docs/mine.md"), "mine\n")
	link := filepath.Join(repo, ".claude/commands/wiki-watch.md")
	if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" {
		if err := os.Symlink("../../docs/mine.md", link); err != nil {
			t.Fatal(err)
		}
		if _, _, err := SyncPrompts(repo); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Lstat(link); err != nil {
			t.Error("a symlink to a user file was removed")
		}
	}
}
