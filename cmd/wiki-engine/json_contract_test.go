package main

import (
	"encoding/json"
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// update rewrites the golden files: go test ./cmd/wiki-engine -run TestJSONGolden -update
var update = flag.Bool("update", false, "rewrite the golden files in testdata/golden")

const goldenDir = "testdata/golden"

const fixedDate = "2026-01-01T12:00:00Z"

// pageFM builds YAML front matter for a fixture page.
func pageFM(id, title, lifecycle string, extra string) string {
	return "---\nid: " + id + "\ntitle: " + title + "\ntype: concept\nlifecycle: " + lifecycle +
		"\ndescription: " + title + " page\nupdated: 2026-01-01\n" + extra + "---\n# " + title + "\n"
}

// contractFixture builds a git repo with a stable wiki on branch "feature"
// (base "main"). orphan adds an active page that nothing links to.
func contractFixture(t *testing.T, orphan bool) string {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{
		"wiki/index.md": pageFM("index", "Index", "current", "") +
			"\n- [Authentication](auth.md)\n- [Token Validation](tokens.md)\n- [Log](log.md)\n",
		"wiki/log.md": pageFM("log", "Log", "current", "") +
			"\n## [2026-01-01] ingest | initial\n\n- Created.\n",
		"wiki/auth.md": pageFM("auth", "Authentication", "current", "tags: [security, api]\nsources:\n  - file: src/main.go\n") +
			"\nAuthentication checks every token. See [Token Validation](tokens.md).\n",
		"wiki/tokens.md": pageFM("tokens", "Token Validation", "current", "tags: [security]\n") +
			"\n## Rules\n\nA token expires after one hour.\nA token carries the user ID.\n",
		"wiki/old.md": pageFM("old", "Old Tokens", "legacy", "") +
			"\n> Historical.\n\nThe old token format is gone.\n",
		"src/main.go": "package main\n",
	}
	if orphan {
		files["wiki/orphan.md"] = pageFM("orphan", "Orphan", "current", "") + "\nNothing links here.\n"
	}
	for rel, content := range files {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	git := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_DATE="+fixedDate, "GIT_COMMITTER_DATE="+fixedDate,
			"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	git("init", "-q", "-b", "main")
	git("config", "user.email", "test@test")
	git("config", "user.name", "Test")
	git("add", ".")
	git("commit", "-q", "-m", "init")
	git("checkout", "-q", "-b", "feature")
	return root
}

// dirtyFixture adds a broken citation, a broken repo link, and a changed
// source file that an unchanged page cites.
func dirtyFixture(t *testing.T, root string) {
	t.Helper()
	page := pageFM("extra", "Extra", "current", "sources:\n  - file: src/missing.go\n") +
		"\nSee [gone](src/gone.go).\n"
	if err := os.WriteFile(filepath.Join(root, "wiki", "extra.md"), []byte(page), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "src", "main.go"), []byte("package main\n\n// changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

var (
	shaRe  = regexp.MustCompile(`\b[0-9a-f]{40}\b`)
	timeRe = regexp.MustCompile(`\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}[0-9:.+Z-]*`)
)

// normalize replaces temp paths, commit hashes, and timestamps.
func normalize(root, out string) string {
	roots := []string{root}
	if real, err := filepath.EvalSymlinks(root); err == nil && real != root {
		roots = append(roots, real)
	}
	// Longest first: the real path ("/private/var/...") contains the short one.
	for i := len(roots) - 1; i >= 0; i-- {
		out = strings.ReplaceAll(out, roots[i], "<ROOT>")
	}
	out = shaRe.ReplaceAllString(out, "<SHA>")
	return timeRe.ReplaceAllString(out, "<TIME>")
}

// checkGolden compares got with testdata/golden/<name>.json, or rewrites it with -update.
func checkGolden(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join(goldenDir, name+".json")
	if *update {
		if err := os.MkdirAll(goldenDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("missing golden file %s (run with -update): %v", path, err)
	}
	if string(want) != got {
		t.Errorf("%s differs from golden %s (run with -update after review)\n--- got ---\n%s\n--- want ---\n%s", name, path, got, want)
	}
}

type goldenCase struct {
	name     string
	args     []string
	wantExit int
	orphan   bool // fixture has an unlinked active page
	dirty    bool // fixture has a broken citation
	noRepo   bool // run in an empty directory
}

var goldenCases = []goldenCase{
	{name: "search_lines", args: []string{"--json", "search", "token", "--limit=3"}},
	{name: "search_ranked", args: []string{"--json", "search", "token", "--ranked"}},
	{name: "graph_full", args: []string{"--json", "graph"}},
	{name: "graph_neighborhood", args: []string{"--json", "graph", "auth"}},
	{name: "health", args: []string{"--json", "health"}},
	{name: "check_clean", args: []string{"--json", "check", "--base=main"}},
	{name: "check_broken", args: []string{"--json", "check", "--base=main"}, wantExit: 1, dirty: true},
	{name: "error_usage", args: []string{"--json", "search"}, wantExit: 2},
	{name: "error_findings", args: []string{"--json", "graph", "--strict"}, wantExit: 1, orphan: true},
	{name: "error_runtime", args: []string{"--json", "health"}, wantExit: 1, noRepo: true},
}

// runGolden runs one case and returns the normalized output.
func runGolden(t *testing.T, tc goldenCase) string {
	t.Helper()
	root := t.TempDir()
	if !tc.noRepo {
		root = contractFixture(t, tc.orphan)
		if tc.dirty {
			dirtyFixture(t, root)
		}
	}
	out, exit := runCLI(t, root, tc.args...)
	if exit != tc.wantExit {
		t.Fatalf("%s: exit = %d, want %d\n%s", tc.name, exit, tc.wantExit, out)
	}
	return normalize(root, out)
}

func TestJSONGolden(t *testing.T) {
	for _, tc := range goldenCases {
		t.Run(tc.name, func(t *testing.T) {
			checkGolden(t, tc.name, runGolden(t, tc))
		})
	}
}

// TestJSONErrorCodes pins the error_code of each failure class, and that
// success carries meta and no error_code.
func TestJSONErrorCodes(t *testing.T) {
	want := map[string]string{"error_usage": "usage", "error_findings": "findings", "error_runtime": "runtime"}
	for _, tc := range goldenCases {
		env := map[string]json.RawMessage{}
		if err := json.Unmarshal([]byte(runGolden(t, tc)), &env); err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		code, failed := want[tc.name]
		if !failed {
			if _, ok := env["error_code"]; ok && tc.wantExit == 0 {
				t.Errorf("%s: success response has error_code", tc.name)
			}
			if _, ok := env["meta"]; !ok {
				t.Errorf("%s: meta missing", tc.name)
			}
			continue
		}
		if string(env["error_code"]) != `"`+code+`"` || string(env["ok"]) != "false" {
			t.Errorf("%s: ok=%s error_code=%s, want false/%s", tc.name, env["ok"], env["error_code"], code)
		}
	}
}

// jsonType names the JSON type of v.
func jsonType(v interface{}) string {
	switch v.(type) {
	case nil:
		return "null"
	case bool:
		return "bool"
	case float64:
		return "number"
	case string:
		return "string"
	case []interface{}:
		return "array"
	default:
		return "object"
	}
}

// shape maps key -> JSON type for one object.
type shape map[string]string

// previousShapes lists, per golden file, the keys and types of the JSON before
// the additive meta/field changes. A key listed here must stay with its type.
// "[]" selects the first item of a list, a key selects a nested object or list.
var previousShapes = map[string]map[string]shape{
	"search_lines": {"": {"id": "string", "file": "string", "lifecycle": "string", "line": "number", "text": "string"}},
	"search_ranked": {"": {"id": "string", "file": "string", "title": "string", "lifecycle": "string", "score": "number",
		"matched_terms": "array", "coverage": "number", "section": "string", "start_line": "number", "context": "string"}},
	"graph_full": {
		".": {"schema_version": "string", "nodes": "array", "edges": "array", "unlinked": "array", "stats": "object", "issues": "array"},
		"nodes": {"id": "string", "kind": "string", "file": "string", "title": "string", "lifecycle": "string",
			"status": "string", "description": "string", "links": "array", "depth": "number"},
		"edges": {"from": "string", "to": "string", "kind": "string"},
		"stats": {"node_count": "number", "edge_count": "number", "max_depth": "number", "roots": "array", "leaves": "array"},
	},
	"graph_neighborhood": {
		".": {"node": "object", "backlinks": "array"},
		"node": {"id": "string", "kind": "string", "file": "string", "title": "string", "lifecycle": "string",
			"status": "string", "description": "string", "links": "array", "depth": "number"},
	},
	"health": {".": {"schema_version": "string", "pages": "number", "wiki_bytes": "number", "index_bytes": "number",
		"log_bytes": "number", "log_needs_rotation": "bool", "graph_nodes": "number", "graph_edges": "number",
		"lifecycles": "object", "largest_pages": "array", "aging_work": "array", "updated_behind_git": "array",
		"duplicate_titles": "array", "index_only_pages": "array", "open_markers": "array"}},
	"check_clean": {".": {"base": "string", "merge_base": "string", "working_tree": "bool", "changed_files": "array",
		"broken": "array", "warnings": "array", "pages_to_review": "array"}},
	"check_broken": {
		".":               {"base": "string", "merge_base": "string", "working_tree": "bool", "changed_files": "array", "broken": "array", "warnings": "array", "pages_to_review": "array"},
		"broken":          {"page": "string", "id": "string", "kind": "string", "target": "string", "message": "string"},
		"warnings":        {"page": "string", "id": "string", "kind": "string", "target": "string", "message": "string"},
		"pages_to_review": {"page": "string", "id": "string", "changed_files": "array", "match_reasons": "array"},
	},
	"error_usage":    {"envelope": {"ok": "bool", "error": "string"}},
	"error_runtime":  {"envelope": {"ok": "bool", "error": "string"}},
	"error_findings": {"envelope": {"ok": "bool", "error": "string", "data": "object"}},
}

// TestJSONCompat loads the goldens and asserts that every key of the previous
// shape is still present with the same JSON type. It guards against removals,
// renames, and type changes, which are breaking changes.
func TestJSONCompat(t *testing.T) {
	for name, parts := range previousShapes {
		raw, err := os.ReadFile(filepath.Join(goldenDir, name+".json"))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		var env map[string]interface{}
		if err := json.Unmarshal(raw, &env); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		for path, want := range parts {
			var obj interface{}
			switch path {
			case "envelope":
				obj = env
			case "":
				// The data is a list: check its first item.
				list, ok := env["data"].([]interface{})
				if !ok || len(list) == 0 {
					t.Fatalf("%s: data is not a non-empty list", name)
				}
				obj = list[0]
			case ".":
				obj = env["data"]
			default:
				data, _ := env["data"].(map[string]interface{})
				obj = data[path]
				// A list key: check its first item.
				if list, ok := obj.([]interface{}); ok && len(list) > 0 {
					obj = list[0]
				}
			}
			m, ok := obj.(map[string]interface{})
			if !ok {
				t.Errorf("%s[%q]: not an object (%T)", name, path, obj)
				continue
			}
			for key, typ := range want {
				v, present := m[key]
				if !present {
					t.Errorf("%s[%q]: key %q removed or renamed", name, path, key)
					continue
				}
				if got := jsonType(v); got != typ {
					t.Errorf("%s[%q].%s: type %s, was %s", name, path, key, got, typ)
				}
			}
		}
	}
}
