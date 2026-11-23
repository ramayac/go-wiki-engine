package scaffold

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

const (
	skillRel      = ".claude/skills/wiki-benchmark"
	templateMark  = "<!-- TEMPLATE: replace these example questions -->"
	editedMarkers = "MY EDIT\n"
)

var skillFiles = []string{
	"SKILL.md",
	"prepare.sh",
	"questions.md",
	"headless/README.md",
	"headless/bench.py",
	"headless/test_bench.py",
}

func skillPath(repo, name string) string {
	return filepath.Join(repo, filepath.FromSlash(skillRel), filepath.FromSlash(name))
}

func captureWarnings(t *testing.T) *bytes.Buffer {
	t.Helper()
	var warn bytes.Buffer
	old := warnOut
	warnOut = &warn
	t.Cleanup(func() { warnOut = old })
	return &warn
}

func TestInitInstallsBenchmarkSkill(t *testing.T) {
	repo := t.TempDir()
	if err := Init(repo, "wiki"); err != nil {
		t.Fatal(err)
	}
	for _, name := range skillFiles {
		if _, err := os.Stat(skillPath(repo, name)); err != nil {
			t.Errorf("init did not create %s/%s: %v", skillRel, name, err)
		}
	}
	if runtime.GOOS != "windows" {
		fi, err := os.Stat(skillPath(repo, "prepare.sh"))
		if err != nil {
			t.Fatal(err)
		}
		if fi.Mode().Perm() != 0o755 {
			t.Errorf("prepare.sh mode = %o, want 755", fi.Mode().Perm())
		}
	}
	qs, _ := os.ReadFile(skillPath(repo, "questions.md"))
	if !strings.Contains(string(qs), templateMark) {
		t.Error("a new questions.md must carry the template marker")
	}
}

func TestInitNeverOverwritesEditedQuestions(t *testing.T) {
	repo := t.TempDir()
	if err := Init(repo, "wiki"); err != nil {
		t.Fatal(err)
	}
	q := skillPath(repo, "questions.md")
	if err := os.WriteFile(q, []byte(editedMarkers), 0o644); err != nil {
		t.Fatal(err)
	}
	warn := captureWarnings(t)
	if err := Init(repo, "wiki"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := SyncPrompts(repo); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(q); string(got) != editedMarkers {
		t.Errorf("questions.md was overwritten: %q", got)
	}
	if _, err := os.Lstat(q + ".bak"); !os.IsNotExist(err) {
		t.Error("questions.md is user-owned: it needs no backup")
	}
	if warn.Len() != 0 {
		t.Errorf("unexpected warnings: %s", warn.String())
	}

	// A deleted questions.md is a missing file, so init writes the template again.
	if err := os.Remove(q); err != nil {
		t.Fatal(err)
	}
	if err := Init(repo, "wiki"); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(q); !strings.Contains(string(got), templateMark) {
		t.Error("init should recreate a missing questions.md from the template")
	}
}

func TestInitLeavesSymlinkedQuestionsAlone(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks need privileges on Windows")
	}
	base := t.TempDir()
	repo := filepath.Join(base, "repo")
	outside := filepath.Join(base, "outside.md")
	mustWrite(t, outside, "OUTSIDE DATA\n")
	if err := os.MkdirAll(filepath.Dir(skillPath(repo, "questions.md")), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, skillPath(repo, "questions.md")); err != nil {
		t.Fatal(err)
	}
	if err := Init(repo, "wiki"); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(outside); string(b) != "OUTSIDE DATA\n" {
		t.Errorf("init wrote through the questions.md symlink: %q", b)
	}
}

func TestSyncPromptsRefreshesSkillAndBacksUpEditedFile(t *testing.T) {
	repo := t.TempDir()
	if err := Init(repo, "wiki"); err != nil {
		t.Fatal(err)
	}
	embedded, err := files.ReadFile("files/" + skillRel + "/SKILL.md")
	if err != nil {
		t.Fatal(err)
	}
	skill := skillPath(repo, "SKILL.md")
	if err := os.WriteFile(skill, []byte(editedMarkers), 0o644); err != nil {
		t.Fatal(err)
	}
	warn := captureWarnings(t)
	updated, _, err := SyncPrompts(repo)
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(skill); !bytes.Equal(got, embedded) {
		t.Error("sync-prompts did not restore SKILL.md")
	}
	bak, err := os.ReadFile(skill + ".bak")
	if err != nil || string(bak) != editedMarkers {
		t.Errorf("edited SKILL.md was not backed up: %q, %v", bak, err)
	}
	if !strings.Contains(warn.String(), skillRel+"/SKILL.md") {
		t.Errorf("no backup notice: %q", warn.String())
	}
	reported := false
	for _, u := range updated {
		reported = reported || u == skillRel+"/SKILL.md"
	}
	if !reported {
		t.Errorf("SKILL.md not in the updated list: %v", updated)
	}

	// A second sync finds identical bytes and makes no new backup.
	if _, _, err := SyncPrompts(repo); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(skill + ".bak.1"); !os.IsNotExist(err) {
		t.Error("an unchanged SKILL.md must not be backed up again")
	}
}

func TestSyncPromptsRestoresScriptMode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("mode bits do not apply on Windows")
	}
	repo := t.TempDir()
	if err := Init(repo, "wiki"); err != nil {
		t.Fatal(err)
	}
	script := skillPath(repo, "prepare.sh")
	if err := os.Chmod(script, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := SyncPrompts(repo); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(script)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != 0o755 {
		t.Errorf("prepare.sh mode = %o after sync-prompts, want 755", fi.Mode().Perm())
	}
}

func TestSyncPromptsKeepsUserFilesInSkillDir(t *testing.T) {
	repo := t.TempDir()
	if _, _, err := SyncPrompts(repo); err != nil {
		t.Fatal(err)
	}
	mine := skillPath(repo, "notes.md")
	mustWrite(t, mine, "mine")
	if _, removed, err := SyncPrompts(repo); err != nil {
		t.Fatal(err)
	} else if len(removed) != 0 {
		t.Errorf("sync-prompts removed %v", removed)
	}
	if _, err := os.Stat(mine); err != nil {
		t.Error("sync-prompts deleted a user file in the skill directory")
	}
}

func TestSyncReplacesSymlinkedSkillFileWithoutFollowing(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks need privileges on Windows")
	}
	base := t.TempDir()
	repo := filepath.Join(base, "repo")
	outside := filepath.Join(base, "outside.md")
	mustWrite(t, outside, "OUTSIDE DATA\n")
	if err := os.MkdirAll(filepath.Dir(skillPath(repo, "SKILL.md")), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, skillPath(repo, "SKILL.md")); err != nil {
		t.Fatal(err)
	}
	if _, _, err := SyncPrompts(repo); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(outside); string(b) != "OUTSIDE DATA\n" {
		t.Errorf("outside file changed: %q", b)
	}
	if fi, err := os.Lstat(skillPath(repo, "SKILL.md")); err != nil || !fi.Mode().IsRegular() {
		t.Errorf("SKILL.md should be a regular file now: %v, %v", fi, err)
	}
}
