package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// findNulls returns the JSON paths of every null value in v.
func findNulls(v interface{}, path string) []string {
	var out []string
	switch t := v.(type) {
	case nil:
		out = append(out, path)
	case map[string]interface{}:
		for k, c := range t {
			out = append(out, findNulls(c, path+"."+k)...)
		}
	case []interface{}:
		for i, c := range t {
			out = append(out, findNulls(c, fmt.Sprintf("%s[%d]", path, i))...)
		}
	}
	return out
}

// TestJSONEmptyListsAreArrays checks that empty results emit [] and never null.
func TestJSONEmptyListsAreArrays(t *testing.T) {
	root := setupMainTestWiki(t)
	cfg, eng := mainTestEngine(t, root)

	// A wiki with no headings, no log entries, and no links.
	bare := t.TempDir()
	if err := os.MkdirAll(filepath.Join(bare, "wiki"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bare, "wiki", "index.md"), []byte("---\nstatus: current\ndescription: Index\n---\nplain text\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bare, "wiki", "log.md"), []byte("---\nstatus: current\ndescription: Log\n---\nno entries\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	bareCfg, bareEng := mainTestEngine(t, bare)

	cases := []struct {
		name  string
		bare  bool
		cmd   string
		args  []string
		allow []string // paths where null is not a list (none expected)
	}{
		{"changed", false, "changed", []string{"wiki-engine", "changed"}, nil},
		{"candidates", false, "candidates", []string{"wiki-engine", "candidates"}, nil},
		{"search no match", false, "search", []string{"wiki-engine", "search", "zzzznomatch"}, nil},
		{"search ranked no match", false, "search", []string{"wiki-engine", "search", "--ranked", "zzzznomatch"}, nil},
		{"relevant no match", false, "relevant", []string{"wiki-engine", "relevant", "zzzznomatch"}, nil},
		{"impact uncited", false, "impact", []string{"wiki-engine", "impact", "nothing-cites-this.go"}, nil},
		{"diff same ref", false, "diff", []string{"wiki-engine", "diff", "HEAD", "HEAD"}, nil},
		{"watch once", false, "watch", []string{"wiki-engine", "watch", "--once"}, nil},
		{"check", false, "check", []string{"wiki-engine", "check", "--base=main"}, nil},
		{"lint", false, "lint", []string{"wiki-engine", "lint"}, nil},
		{"lint fix", false, "lint", []string{"wiki-engine", "lint", "--fix"}, nil},
		{"list active", false, "list", []string{"wiki-engine", "list", "--active", "--"}, nil},
		{"log-tail empty", true, "log-tail", []string{"wiki-engine", "log-tail"}, nil},
		{"headings empty", true, "headings", []string{"wiki-engine", "headings"}, nil},
		{"list", true, "list", []string{"wiki-engine", "list"}, nil},
		{"stats", true, "stats", []string{"wiki-engine", "stats"}, nil},
		{"context", true, "context", []string{"wiki-engine", "context"}, nil},
		{"context minimal", true, "context", []string{"wiki-engine", "context", "--minimal"}, nil},
		{"context active", true, "context", []string{"wiki-engine", "context", "--active"}, nil},
		{"graph", true, "graph", []string{"wiki-engine", "graph"}, nil},
		{"graph neighborhood", true, "graph", []string{"wiki-engine", "graph", "index.md"}, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, e := cfg, eng
			if tc.bare {
				c, e = bareCfg, bareEng
			}
			out := captureStdout(t, func() { runEngine(tc.cmd, c, e, tc.args, true) })
			var env map[string]interface{}
			if err := json.Unmarshal([]byte(out), &env); err != nil {
				t.Fatalf("invalid JSON: %v\n%s", err, out)
			}
			if nulls := findNulls(env["data"], "data"); len(nulls) > 0 {
				t.Errorf("null values at %v:\n%s", nulls, out)
			}
		})
	}
}

func TestSyncPromptsJSONHasNoNullLists(t *testing.T) {
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(old) }()
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	out := captureStdout(t, func() { runSyncPrompts(true) })
	var env map[string]interface{}
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if nulls := findNulls(env["data"], "data"); len(nulls) > 0 {
		t.Errorf("null values at %v:\n%s", nulls, out)
	}
}
