package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHealthCommandJSONEnvelope(t *testing.T) {
	root := setupMainTestWiki(t)
	out, exit := runCLI(t, root, "--json", "health")
	if exit != 0 {
		t.Fatalf("health --json exit = %d, want 0\n%s", exit, out)
	}
	env := jsonEnvelope(t, out)
	if string(env["ok"]) != "true" {
		t.Errorf("ok = %s, want true\n%s", env["ok"], out)
	}
	var data map[string]json.RawMessage
	if err := json.Unmarshal(env["data"], &data); err != nil {
		t.Fatalf("data is not an object: %v\n%s", err, out)
	}
	if string(data["schema_version"]) != `"1"` {
		t.Errorf("schema_version = %s, want \"1\"", data["schema_version"])
	}
	for _, k := range []string{"pages", "wiki_bytes", "log_needs_rotation", "lifecycles", "largest_pages"} {
		if _, ok := data[k]; !ok {
			t.Errorf("health data missing %q:\n%s", k, out)
		}
	}
	for _, k := range []string{"aging_work", "updated_behind_git", "duplicate_titles", "open_markers"} {
		if string(data[k]) != "[]" {
			t.Errorf("%s = %s, want []", k, data[k])
		}
	}
}

func TestHealthCommandTextSmoke(t *testing.T) {
	root := setupMainTestWiki(t)
	out, exit := runCLI(t, root, "health")
	if exit != 0 {
		t.Fatalf("health exit = %d, want 0\n%s", exit, out)
	}
	for _, want := range []string{"Wiki health:", "Lifecycle:", "Largest pages:"} {
		if !strings.Contains(out, want) {
			t.Errorf("health output missing %q:\n%s", want, out)
		}
	}
	// The `--` terminator is accepted, like on every other command.
	if out, exit := runCLI(t, root, "health", "--"); exit != 0 {
		t.Errorf("health -- exit = %d\n%s", exit, out)
	}
}

func TestHealthCommandUsageErrorsExitTwo(t *testing.T) {
	root := setupMainTestWiki(t)
	for _, args := range [][]string{
		{"health", "--bogus"},
		{"health", "extra"},
		{"--json", "health", "--bogus"},
	} {
		if out, exit := runCLI(t, root, args...); exit != 2 {
			t.Errorf("%v: exit = %d, want 2\n%s", args, exit, out)
		}
	}
}

// Open markers are advisory: health reports them and still exits 0.
func TestHealthCommandExitsZeroWithSignals(t *testing.T) {
	root := setupMainTestWiki(t)
	page := "---\nid: marked\ntitle: Marked\ntype: concept\nlifecycle: current\ndescription: Marked\n---\n# Marked\n\n> Rationale: not recorded.\n\n> ⚠️ CONTRADICTION: A says X.\n"
	if err := os.WriteFile(filepath.Join(root, "wiki", "marked.md"), []byte(page), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"health"}, {"--json", "health"}} {
		out, exit := runCLI(t, root, args...)
		if exit != 0 {
			t.Errorf("%v exit = %d, want 0\n%s", args, exit, out)
		}
		if !strings.Contains(out, "wiki/marked.md") {
			t.Errorf("%v output does not list the marked page:\n%s", args, out)
		}
	}
}

func TestHealthCommandWithoutWikiIsRuntimeError(t *testing.T) {
	out, exit := runCLI(t, t.TempDir(), "health")
	if exit != 1 {
		t.Errorf("health without wiki: exit = %d, want 1\n%s", exit, out)
	}
}

func TestSchemaConventionsSectionCLI(t *testing.T) {
	root := setupMainTestWiki(t)
	out, exit := runCLI(t, root, "schema", "--section", "conventions")
	if exit != 0 || !strings.Contains(out, "## Conventions") || !strings.Contains(out, "> Rationale: not recorded.") {
		t.Errorf("schema --section conventions: exit=%d\n%s", exit, out)
	}
	out, exit = runCLI(t, root, "--json", "schema", "--section=conventions")
	if exit != 0 || !strings.Contains(out, "ASD-STE100") {
		t.Errorf("--json schema --section conventions: exit=%d\n%s", exit, out)
	}
}
