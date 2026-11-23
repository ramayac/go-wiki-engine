package model

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeVocab(t *testing.T, content string) string {
	t.Helper()
	wikiRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(wikiRoot, VocabularyFile), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return wikiRoot
}

func TestLoadVocabularyFreeForm(t *testing.T) {
	cases := map[string]string{
		"empty file":    "",
		"comments only": "# header\n#\n# concerns:\n#   - testing\n",
		"empty mapping": "{}\n",
		"empty list":    "[]\n",
		"null category": "concerns:\n",
	}
	for name, content := range cases {
		t.Run(name, func(t *testing.T) {
			v, err := LoadVocabulary(writeVocab(t, content))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !v.Empty() {
				t.Errorf("want free-form vocabulary, got %v", v.Tags())
			}
		})
	}
	t.Run("missing file", func(t *testing.T) {
		v, err := LoadVocabulary(t.TempDir())
		if err != nil || !v.Empty() {
			t.Fatalf("missing file must be free-form, got %v, %v", v.Tags(), err)
		}
	})
}

func TestLoadVocabularyFormats(t *testing.T) {
	t.Run("categories union", func(t *testing.T) {
		v, err := LoadVocabulary(writeVocab(t, "concerns:\n  - testing\n  - security\nareas:\n  - testing\n  - api\n"))
		if err != nil {
			t.Fatal(err)
		}
		if got := strings.Join(v.Tags(), ","); got != "api,security,testing" {
			t.Errorf("Tags() = %s", got)
		}
		if !v.Has("api") || v.Has("db") {
			t.Error("Has() wrong")
		}
	})
	t.Run("bare list", func(t *testing.T) {
		v, err := LoadVocabulary(writeVocab(t, "- one\n- two\n"))
		if err != nil {
			t.Fatal(err)
		}
		if !v.Has("one") || !v.Has("two") || v.Empty() {
			t.Errorf("bare list not loaded: %v", v.Tags())
		}
	})
}

func TestLoadVocabularyErrors(t *testing.T) {
	cases := map[string]string{
		"malformed yaml":      "concerns: [a, b\n",
		"tab indent":          "concerns:\n\t- a\n",
		"top-level scalar":    "just-a-string\n",
		"scalar category":     "concerns: testing\n",
		"non-string number":   "concerns:\n  - 2024\n",
		"non-string bool":     "concerns:\n  - true\n",
		"null item":           "concerns:\n  - a\n  -\n",
		"nested mapping":      "concerns:\n  inner:\n    - a\n",
		"bare list non-str":   "- a\n- 3\n",
		"multiple documents":  "- a\n---\n- b\n",
		"mapping item in seq": "- a:\n    - b\n",
	}
	for name, content := range cases {
		t.Run(name, func(t *testing.T) {
			v, err := LoadVocabulary(writeVocab(t, content))
			if err == nil {
				t.Fatalf("want error, got vocabulary %v", v.Tags())
			}
			if !v.Empty() {
				t.Errorf("failed load must return an empty vocabulary, got %v", v.Tags())
			}
		})
	}
}

func TestLoadVocabularySymlinkOutsideRefused(t *testing.T) {
	outside := filepath.Join(t.TempDir(), "outside.yaml")
	if err := os.WriteFile(outside, []byte("- secret\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	wikiRoot := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(wikiRoot, VocabularyFile)); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	v, err := LoadVocabulary(wikiRoot)
	if err == nil {
		t.Fatalf("want containment error, got %v", v.Tags())
	}
}

func TestVocabularySuggest(t *testing.T) {
	v, err := LoadVocabulary(writeVocab(t, "- Data-Store\n- api_gateway\n- testing\n"))
	if err != nil {
		t.Fatal(err)
	}
	cases := map[string]string{
		"data_store":  "Data-Store",
		"DATA-STORE":  "Data-Store",
		"api-gateway": "api_gateway",
		"Testing":     "testing",
		"unrelated":   "",
	}
	for in, want := range cases {
		if got := v.Suggest(in); got != want {
			t.Errorf("Suggest(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestLoadSnapshotCarriesVocabulary(t *testing.T) {
	root := t.TempDir()
	wiki := filepath.Join(root, "wiki")
	if err := os.MkdirAll(wiki, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wiki, VocabularyFile), []byte("- a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	snap, err := LoadSnapshot(root, "wiki")
	if err != nil {
		t.Fatal(err)
	}
	if snap.VocabularyErr != nil || !snap.Vocabulary.Has("a") {
		t.Errorf("snapshot vocabulary not loaded: %v %v", snap.Vocabulary.Tags(), snap.VocabularyErr)
	}
	if err := os.WriteFile(filepath.Join(wiki, VocabularyFile), []byte("a: [\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	snap, err = LoadSnapshot(root, "wiki")
	if err != nil {
		t.Fatalf("a malformed vocabulary must not fail LoadSnapshot: %v", err)
	}
	if snap.VocabularyErr == nil {
		t.Error("want VocabularyErr for malformed file")
	}
}
