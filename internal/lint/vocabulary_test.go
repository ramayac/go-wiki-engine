package lint

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ramayac/go-wiki-engine/internal/config"
	"github.com/ramayac/go-wiki-engine/internal/model"
)

func taggedPage(id, lifecycle, tags string) string {
	return "---\nid: " + id + "\ntype: concept\nlifecycle: " + lifecycle +
		"\ndescription: Page\ntags: " + tags + "\n---\n# Page\n\nSee [index](../index.md).\n"
}

func vocabIssues(t *testing.T, extra map[string]string, check, skip []string) Result {
	t.Helper()
	root := setupLintWiki(t, extra)
	cfg := config.DefaultConfig()
	snap, err := model.LoadSnapshot(root, cfg.WikiDir)
	if err != nil {
		t.Fatal(err)
	}
	return Run(snap, cfg, check, skip)
}

func vocabOnly(res Result) []Issue {
	var out []Issue
	for _, i := range res.Issues {
		if i.Check == "vocabulary" {
			out = append(out, i)
		}
	}
	return out
}

func TestVocabularyFreeFormWhenNoEntries(t *testing.T) {
	page := map[string]string{"wiki/concepts/a.md": taggedPage("a", "current", "[anything, Goes]")}
	cases := map[string]map[string]string{
		"absent":        page,
		"empty":         {"wiki/concepts/a.md": page["wiki/concepts/a.md"], "wiki/vocabulary.yaml": ""},
		"comments only": {"wiki/concepts/a.md": page["wiki/concepts/a.md"], "wiki/vocabulary.yaml": "# nothing\n# - x\n"},
	}
	for name, extra := range cases {
		t.Run(name, func(t *testing.T) {
			res := vocabIssues(t, extra, []string{"vocabulary"}, nil)
			if !res.OK || len(res.Issues) != 0 {
				t.Fatalf("free-form must pass, got %v", res.Messages)
			}
		})
	}
}

func TestVocabularyRejectsUnlistedTags(t *testing.T) {
	res := vocabIssues(t, map[string]string{
		"wiki/vocabulary.yaml": "concerns:\n  - testing\nareas:\n  - api\n",
		"wiki/concepts/a.md":   taggedPage("a", "current", "[testing, api, bogus]"),
		"wiki/concepts/b.md":   taggedPage("b", "legacy", "[nope]"),
	}, []string{"vocabulary"}, nil)
	if res.OK {
		t.Fatal("unlisted tags must fail lint")
	}
	got := vocabOnly(res)
	if len(got) != 2 {
		t.Fatalf("want 2 issues, got %v", res.Messages)
	}
	joined := strings.Join(res.Messages, "\n")
	for _, want := range []string{"wiki/concepts/a.md", `"bogus"`, "wiki/concepts/b.md", `"nope"`} {
		if !strings.Contains(joined, want) {
			t.Errorf("output missing %q:\n%s", want, joined)
		}
	}
	for _, i := range got {
		if i.Severity != SevError {
			t.Errorf("severity = %v, want error", i.Severity)
		}
	}
	if strings.Contains(joined, `"testing"`) || strings.Contains(joined, `"api"`) {
		t.Errorf("listed tags must not be flagged:\n%s", joined)
	}
}

func TestVocabularyBareListAccepted(t *testing.T) {
	res := vocabIssues(t, map[string]string{
		"wiki/vocabulary.yaml": "- one\n- two\n",
		"wiki/concepts/a.md":   taggedPage("a", "current", "[one, two]"),
	}, []string{"vocabulary"}, nil)
	if !res.OK {
		t.Fatalf("listed tags must pass: %v", res.Messages)
	}
}

func TestVocabularyDidYouMean(t *testing.T) {
	res := vocabIssues(t, map[string]string{
		"wiki/vocabulary.yaml": "- data-store\n- api\n",
		"wiki/concepts/a.md":   taggedPage("a", "current", "[Data_Store, API, other]"),
	}, []string{"vocabulary"}, nil)
	joined := strings.Join(res.Messages, "\n")
	if !strings.Contains(joined, `did you mean "data-store"`) || !strings.Contains(joined, `did you mean "api"`) {
		t.Errorf("missing hint:\n%s", joined)
	}
	for _, i := range vocabOnly(res) {
		if strings.Contains(i.Message, `"other"`) && strings.Contains(i.Message, "did you mean") {
			t.Errorf("unrelated tag must have no hint: %s", i.Message)
		}
	}
}

func TestVocabularyMalformedFileIsOneError(t *testing.T) {
	for name, content := range map[string]string{
		"bad yaml":   "concerns: [a\n",
		"non-string": "concerns:\n  - 12\n",
	} {
		t.Run(name, func(t *testing.T) {
			res := vocabIssues(t, map[string]string{
				"wiki/vocabulary.yaml": content,
				"wiki/concepts/a.md":   taggedPage("a", "current", "[x]"),
			}, []string{"vocabulary"}, nil)
			got := vocabOnly(res)
			if res.OK || len(got) != 1 {
				t.Fatalf("want exactly one error, got %v", res.Messages)
			}
			if got[0].File != "wiki/vocabulary.yaml" || got[0].Severity != SevError {
				t.Errorf("issue = %+v", got[0])
			}
		})
	}
}

func TestVocabularySymlinkOutsideRefused(t *testing.T) {
	outside := filepath.Join(t.TempDir(), "outside.yaml")
	if err := os.WriteFile(outside, []byte("- x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	root := setupLintWiki(t, nil)
	if err := os.Symlink(outside, filepath.Join(root, "wiki", "vocabulary.yaml")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	cfg := config.DefaultConfig()
	snap, err := model.LoadSnapshot(root, cfg.WikiDir)
	if err != nil {
		t.Fatal(err)
	}
	res := Run(snap, cfg, []string{"vocabulary"}, nil)
	if res.OK || len(vocabOnly(res)) != 1 {
		t.Fatalf("want one vocabulary error, got %v", res.Messages)
	}
}

func TestVocabularySelectors(t *testing.T) {
	extra := map[string]string{
		"wiki/vocabulary.yaml": "- ok\n",
		"wiki/concepts/a.md":   taggedPage("a", "current", "[bad]"),
	}
	if res := vocabIssues(t, extra, []string{"vocabulary"}, nil); res.OK {
		t.Error("--check=vocabulary must report")
	}
	if res := vocabIssues(t, extra, []string{"schema"}, nil); len(vocabOnly(res)) != 1 {
		t.Error("vocabulary must belong to the schema group")
	}
	if res := vocabIssues(t, extra, nil, []string{"vocabulary"}); len(vocabOnly(res)) != 0 {
		t.Error("--skip=vocabulary must silence the check")
	}
	if res := vocabIssues(t, extra, nil, []string{"schema"}); len(vocabOnly(res)) != 0 {
		t.Error("--skip=schema must silence the check")
	}
	found := false
	for _, n := range KnownCheckerNames() {
		found = found || n == "vocabulary"
	}
	if !found {
		t.Error("vocabulary missing from KnownCheckerNames")
	}
	if err := ValidateSelectors([]string{"vocabulary"}, []string{"vocabulary"}); err != nil {
		t.Errorf("ValidateSelectors: %v", err)
	}
}
