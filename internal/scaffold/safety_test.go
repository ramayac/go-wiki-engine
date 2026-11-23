package scaffold

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// snapshotTree records every path under root with its type and content so a
// test can prove nothing changed.
func snapshotTree(t *testing.T, root string) map[string]string {
	t.Helper()
	snap := map[string]string{}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, p)
		switch {
		case d.IsDir():
			snap[rel] = "dir"
		case d.Type()&fs.ModeSymlink != 0:
			l, _ := os.Readlink(p)
			snap[rel] = "link:" + l
		default:
			b, _ := os.ReadFile(p)
			snap[rel] = "file:" + string(b)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return snap
}

func mustWrite(t *testing.T, p, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestWritesRefuseSymlinksOutsideRepo(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks need privileges on Windows")
	}
	const sentinel = "OUTSIDE DATA\n"
	cases := []struct {
		name string
		link string // path inside the repo that becomes a symlink
		file bool   // link points at an outside file, not an outside directory
		run  func(repo string) error
	}{
		{"wiki-instructions dir", ".wiki-instructions", false, syncRun},
		{"claude commands dir", ".claude/commands", false, syncRun},
		{"claude dir", ".claude", false, syncRun},
		{"github prompts dir", ".github/prompts", false, syncRun},
		{"github instructions dir", ".github/instructions", false, syncRun},
		{"github dir", ".github", false, syncRun},
		{"pi skills wiki dir", ".pi/skills/wiki", false, syncRun},
		{"pi dir", ".pi", false, syncRun},
		{"AGENTS.md sync", "AGENTS.md", true, syncRun},
		{"CLAUDE.md sync", "CLAUDE.md", true, syncRun},
		{"AGENTS.md init", "AGENTS.md", true, initRun},
		{"CLAUDE.md init", "CLAUDE.md", true, initRun},
		{".wikirc init", ".wikirc", true, initRun},
		{"wiki dir init", "wiki", false, initRun},
		{".wiki-instructions dir init", ".wiki-instructions", false, initRun},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			base := t.TempDir()
			repo := filepath.Join(base, "repo")
			outside := filepath.Join(base, "outside")
			if err := os.MkdirAll(repo, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(outside, 0o755); err != nil {
				t.Fatal(err)
			}
			var target string
			if tc.file {
				target = filepath.Join(outside, "target.txt")
				mustWrite(t, target, sentinel)
			} else {
				target = outside
				// Files the engine would write if it followed the link.
				for _, n := range []string{"ingest.md", "wiki-ingest.md", "wiki-ingest.prompt.md", "wiki-maintainer.instructions.md", "SKILL.md", "index.md"} {
					mustWrite(t, filepath.Join(outside, n), sentinel)
				}
			}
			if err := os.MkdirAll(filepath.Dir(filepath.Join(repo, tc.link)), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(target, filepath.Join(repo, tc.link)); err != nil {
				t.Fatal(err)
			}
			before := snapshotTree(t, outside)

			err := tc.run(repo)
			if err == nil {
				t.Fatal("expected an error for a symlink that leaves the repo, got nil")
			}
			if !strings.Contains(err.Error(), "outside") {
				t.Errorf("error should say the path is outside the repo: %v", err)
			}
			after := snapshotTree(t, outside)
			if len(before) != len(after) {
				t.Errorf("outside tree changed: before %v, after %v", before, after)
			}
			for k, v := range before {
				if after[k] != v {
					t.Errorf("outside %q changed: %q -> %q", k, v, after[k])
				}
			}
		})
	}
}

func TestInitRefusesDanglingSymlinkOutsideRepo(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks need privileges on Windows")
	}
	base := t.TempDir()
	repo := filepath.Join(base, "repo")
	outside := filepath.Join(base, "outside")
	for _, d := range []string{repo, outside} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	missing := filepath.Join(outside, "new.md")
	if err := os.Symlink(missing, filepath.Join(repo, "AGENTS.md")); err != nil {
		t.Fatal(err)
	}
	if err := Init(repo, "wiki"); err == nil {
		t.Fatal("expected an error, got nil")
	}
	if _, err := os.Lstat(missing); !os.IsNotExist(err) {
		t.Errorf("Init created a file outside the repo: %v", err)
	}
}

func TestSyncReplacesSymlinkedManagedFilesWithoutFollowing(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks need privileges on Windows")
	}
	base := t.TempDir()
	repo := filepath.Join(base, "repo")
	outsideFile := filepath.Join(base, "outside", "keep.md")
	mustWrite(t, outsideFile, "OUTSIDE DATA\n")
	for _, rel := range []string{".claude/commands/wiki-ingest.md", ".wiki-instructions/ingest.md"} {
		p := filepath.Join(repo, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(outsideFile, p); err != nil {
			t.Fatal(err)
		}
	}
	if _, _, err := SyncPrompts(repo); err != nil {
		t.Fatalf("SyncPrompts: %v", err)
	}
	if b, _ := os.ReadFile(outsideFile); string(b) != "OUTSIDE DATA\n" {
		t.Errorf("outside file changed: %q", b)
	}
	for _, rel := range []string{".claude/commands/wiki-ingest.md", ".wiki-instructions/ingest.md"} {
		if got, _ := os.Readlink(filepath.Join(repo, rel)); got == outsideFile {
			t.Errorf("%s still points outside the repo", rel)
		}
	}
	if b, _ := os.ReadFile(filepath.Join(repo, ".claude/commands/wiki-ingest.md")); !bytes.Contains(b, []byte("#")) {
		t.Error("wiki-ingest.md should resolve to the embedded prompt")
	}
}

func syncRun(repo string) error {
	_, _, err := SyncPrompts(repo)
	return err
}

func initRun(repo string) error { return Init(repo, "wiki") }

func TestSyncPromptsKeepsUserFilesAndRemovesOnlyOwnedOnes(t *testing.T) {
	repo := t.TempDir()
	if _, _, err := SyncPrompts(repo); err != nil {
		t.Fatal(err)
	}
	user := map[string]string{
		".claude/commands/wiki-custom.md":                         "mine",
		".github/prompts/wiki-mine.prompt.md":                     "mine",
		".github/instructions/wiki-x.instructions.md":             "mine",
		".wiki-instructions/wiki-notes.md":                        "mine",
		".pi/skills/wiki/mine.md":                                 "mine",
		".claude/commands/review.md":                              "mine",
		".github/prompts/wiki-keep.prompt.md":                     "mine",
		".wiki-instructions/notes.md":                             "mine",
		".claude/commands/wiki-link-elsewhere.md.regular":         "mine",
		".claude/commands/wiki-not-engine-link.md":                "", // symlink made below
		".claude/commands/wiki-migrate-shims.md":                  "retired",
		".github/prompts/wiki-migrate-shims.prompt.md":            "retired",
		".wiki-instructions/migrate-shims.md":                     "retired",
		".wiki-instructions/summarize.md":                         "retired",
		".claude/commands/wiki-dangling-engine-link.md":           "", // symlink made below
		".github/prompts/wiki-dangling-engine-link.prompt.md":     "", // symlink made below
		".github/instructions/wiki-dangling-link.instructions.md": "",
	}
	for rel, content := range user {
		if content != "" {
			mustWrite(t, filepath.Join(repo, rel), content)
		}
	}
	// A symlink that is not the engine's: it points at a user file.
	mustWrite(t, filepath.Join(repo, "docs", "mine.md"), "mine")
	if runtime.GOOS != "windows" {
		if err := os.Symlink("../../docs/mine.md", filepath.Join(repo, ".claude/commands/wiki-not-engine-link.md")); err != nil {
			t.Fatal(err)
		}
		for _, rel := range []string{".claude/commands/wiki-dangling-engine-link.md", ".github/prompts/wiki-dangling-engine-link.prompt.md"} {
			if err := os.Symlink("../../.wiki-instructions/gone.md", filepath.Join(repo, rel)); err != nil {
				t.Fatal(err)
			}
		}
	}

	_, removed, err := SyncPrompts(repo)
	if err != nil {
		t.Fatal(err)
	}
	exists := func(rel string) bool {
		_, err := os.Lstat(filepath.Join(repo, rel))
		return err == nil
	}
	for _, rel := range []string{
		".claude/commands/wiki-custom.md",
		".github/prompts/wiki-mine.prompt.md",
		".github/instructions/wiki-x.instructions.md",
		".wiki-instructions/wiki-notes.md",
		".pi/skills/wiki/mine.md",
		".claude/commands/review.md",
		".wiki-instructions/notes.md",
	} {
		if !exists(rel) {
			t.Errorf("user file %s was deleted", rel)
		}
	}
	gone := []string{
		".claude/commands/wiki-migrate-shims.md",
		".github/prompts/wiki-migrate-shims.prompt.md",
		".wiki-instructions/migrate-shims.md",
		".wiki-instructions/summarize.md",
	}
	if runtime.GOOS != "windows" {
		if !exists(".claude/commands/wiki-not-engine-link.md") {
			t.Error("a symlink to a user file was deleted")
		}
		gone = append(gone, ".claude/commands/wiki-dangling-engine-link.md", ".github/prompts/wiki-dangling-engine-link.prompt.md")
	}
	for _, rel := range gone {
		if exists(rel) {
			t.Errorf("engine-owned file %s was not removed", rel)
		}
	}
	if len(removed) != len(gone) {
		t.Errorf("removed = %v, want %d entries", removed, len(gone))
	}
}

func TestSyncPromptsBacksUpEditedManagedFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink replacement is not used on Windows")
	}
	var warn bytes.Buffer
	old := warnOut
	warnOut = &warn
	t.Cleanup(func() { warnOut = old })

	repo := t.TempDir()
	if _, _, err := SyncPrompts(repo); err != nil {
		t.Fatal(err)
	}
	embedded, err := os.ReadFile(filepath.Join(repo, ".claude/commands/wiki-ingest.md"))
	if err != nil {
		t.Fatal(err)
	}

	// Edited regular file: backed up, replaced by the link, one stderr line.
	dest := filepath.Join(repo, ".claude/commands/wiki-ingest.md")
	if err := os.Remove(dest); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, dest, "hand written\n")
	mustWrite(t, dest+".bak", "older backup\n")
	if _, _, err := SyncPrompts(repo); err != nil {
		t.Fatal(err)
	}
	if fi, _ := os.Lstat(dest); fi.Mode()&os.ModeSymlink == 0 {
		t.Error("wiki-ingest.md was not replaced by a symlink")
	}
	if b, _ := os.ReadFile(dest + ".bak"); string(b) != "older backup\n" {
		t.Errorf("existing .bak was overwritten: %q", b)
	}
	if b, _ := os.ReadFile(dest + ".bak.1"); string(b) != "hand written\n" {
		t.Errorf(".bak.1 = %q, want the hand-written content", b)
	}
	if got := strings.Count(warn.String(), "\n"); got != 1 || !strings.Contains(warn.String(), "wiki-ingest.md.bak.1") {
		t.Errorf("want one stderr line naming the backup, got %q", warn.String())
	}

	// Identical regular file: replaced silently, no backup.
	warn.Reset()
	if err := os.Remove(dest); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dest, embedded, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := SyncPrompts(repo); err != nil {
		t.Fatal(err)
	}
	if warn.Len() != 0 {
		t.Errorf("identical file should be replaced silently, got %q", warn.String())
	}
	if _, err := os.Lstat(dest + ".bak.2"); !os.IsNotExist(err) {
		t.Error("identical file was backed up")
	}
}

func TestInitWikiDirArguments(t *testing.T) {
	t.Run("dollar sign is literal", func(t *testing.T) {
		repo := t.TempDir()
		if err := Init(repo, "a$b"); err != nil {
			t.Fatal(err)
		}
		if err := Init(repo, "a$b"); err != nil {
			t.Fatal(err)
		}
		agents, _ := os.ReadFile(filepath.Join(repo, "AGENTS.md"))
		if !strings.Contains(string(agents), "a$b/index.md") {
			t.Errorf("AGENTS.md lost the dollar sign after a second Init:\n%s", agents)
		}
		rc, _ := os.ReadFile(filepath.Join(repo, ".wikirc"))
		if !strings.Contains(string(rc), `wiki_dir = "a$b"`) || !strings.Contains(string(rc), `"a$b/"`) {
			t.Errorf(".wikirc lost the dollar sign:\n%s", rc)
		}
	})
	t.Run("leading dash is rejected", func(t *testing.T) {
		repo := t.TempDir()
		for _, bad := range []string{"-x", "--wiki-dir", "-"} {
			if err := Init(repo, bad); err == nil {
				t.Errorf("Init(%q) should fail", bad)
			}
		}
		// The name error wins over a .wikirc mismatch error.
		if err := Init(repo, "wiki"); err != nil {
			t.Fatal(err)
		}
		if _, err := Reconcile(repo, ReconcileOptions{WikiDir: "-x"}); err == nil || !strings.Contains(err.Error(), "must not start with") {
			t.Errorf("want the name error, got %v", err)
		}
		_ = os.RemoveAll(repo)
		_ = os.MkdirAll(repo, 0o755)
		if entries, _ := os.ReadDir(repo); len(entries) != 0 {
			t.Errorf("rejected init created files: %v", entries)
		}
	})
	t.Run("positional name must match .wikirc", func(t *testing.T) {
		repo := t.TempDir()
		if err := Init(repo, "wiki"); err != nil {
			t.Fatal(err)
		}
		_, err := Reconcile(repo, ReconcileOptions{WikiDir: "docs"})
		if err == nil || !strings.Contains(err.Error(), `wiki_dir in .wikirc is "wiki"`) {
			t.Fatalf("want wiki_dir mismatch error, got %v", err)
		}
		if _, serr := os.Stat(filepath.Join(repo, "docs")); !os.IsNotExist(serr) {
			t.Error("docs/ was created despite the error")
		}
		if _, err := Reconcile(repo, ReconcileOptions{WikiDir: "wiki"}); err != nil {
			t.Errorf("matching name should work: %v", err)
		}
	})
}
