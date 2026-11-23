package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const brokenIndex = "---\nstatus: current\ndescription: Index\n---\n# Index\n\n- [missing.md](missing.md)\n"

// jsonEnvelope parses out as one JSON object.
func jsonEnvelope(t *testing.T, out string) map[string]json.RawMessage {
	t.Helper()
	var env map[string]json.RawMessage
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatalf("output is not one JSON object: %v\n%s", err, out)
	}
	return env
}

func brokenFixture(t *testing.T) string {
	t.Helper()
	root := setupMainTestWiki(t)
	if err := os.WriteFile(filepath.Join(root, "wiki", "index.md"), []byte(brokenIndex), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestRefreshJSONAndExitCode(t *testing.T) {
	root := setupMainTestWiki(t)
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\n\n// changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Clean wiki with a candidate: JSON object, exit 0.
	out, exit := runCLI(t, root, "--json", "refresh", "main...HEAD")
	if exit != 0 {
		t.Fatalf("refresh --json exit = %d, want 0\n%s", exit, out)
	}
	env := jsonEnvelope(t, out)
	if string(env["ok"]) != "true" {
		t.Errorf("ok = %s, want true\n%s", env["ok"], out)
	}
	var data map[string]json.RawMessage
	if err := json.Unmarshal(env["data"], &data); err != nil {
		t.Fatalf("data is not an object: %v\n%s", err, out)
	}
	for _, k := range []string{"wiki_files", "recent_log", "changed", "candidates", "lint"} {
		if _, ok := data[k]; !ok {
			t.Errorf("refresh data missing %q:\n%s", k, out)
		}
	}

	// No candidates: still a JSON object, exit 0.
	clean := setupMainTestWiki(t)
	out, exit = runCLI(t, clean, "--json", "refresh", "main...HEAD")
	if exit != 0 {
		t.Fatalf("clean refresh --json exit = %d\n%s", exit, out)
	}
	if env := jsonEnvelope(t, out); string(env["ok"]) != "true" {
		t.Errorf("clean refresh ok = %s\n%s", env["ok"], out)
	}

	// Failing lint: non-zero exit in both modes; JSON says ok=false.
	if err := os.WriteFile(filepath.Join(root, "wiki", "index.md"), []byte(brokenIndex), 0o644); err != nil {
		t.Fatal(err)
	}
	out, exit = runCLI(t, root, "refresh", "main...HEAD")
	if exit != 1 || !strings.Contains(out, "== lint ==") {
		t.Errorf("refresh with lint failure: exit=%d, want 1\n%s", exit, out)
	}
	out, exit = runCLI(t, root, "--json", "refresh", "main...HEAD")
	if exit != 1 {
		t.Errorf("refresh --json with lint failure: exit=%d, want 1\n%s", exit, out)
	}
	env = jsonEnvelope(t, out)
	if string(env["ok"]) != "false" || len(env["error"]) == 0 || len(env["data"]) == 0 {
		t.Errorf("refresh --json lint failure envelope wrong:\n%s", out)
	}
}

func TestJSONOkMatchesExitCode(t *testing.T) {
	broken := brokenFixture(t)
	for _, args := range [][]string{
		{"--json", "graph", "--strict"},
		{"--json", "watch", "--once"},
		{"--json", "lint"},
	} {
		out, exit := runCLI(t, broken, args...)
		if exit != 1 {
			t.Errorf("%v: exit = %d, want 1\n%s", args, exit, out)
			continue
		}
		env := jsonEnvelope(t, out)
		if string(env["ok"]) != "false" {
			t.Errorf("%v: ok = %s with exit 1, want false\n%s", args, env["ok"], out)
		}
		if len(env["error"]) == 0 {
			t.Errorf("%v: error field missing\n%s", args, out)
		}
		if len(env["data"]) == 0 {
			t.Errorf("%v: data field must be kept\n%s", args, out)
		}
	}

	// Healthy graph: ok stays true with --strict.
	clean := setupMainTestWiki(t)
	out, exit := runCLI(t, clean, "--json", "graph", "--strict")
	if exit != 0 || string(jsonEnvelope(t, out)["ok"]) != "true" {
		t.Errorf("clean graph --strict: exit=%d\n%s", exit, out)
	}
}

func TestUsageErrorsExitTwo(t *testing.T) {
	root := setupMainTestWiki(t)
	cases := []struct {
		name string
		args []string
	}{
		{"no command", nil},
		{"unknown command", []string{"nope"}},
		{"unknown flag", []string{"list", "--bogus"}},
		{"extra positional", []string{"list", "extra"}},
		{"flag missing value", []string{"changed", "--base"}},
		{"search without query", []string{"search"}},
		{"summary without page", []string{"summary"}},
		{"diff with one ref", []string{"diff", "HEAD"}},
		{"relevant without query", []string{"relevant"}},
		{"invalid lint selector", []string{"lint", "--check=nope"}},
		{"invalid schema type", []string{"schema", "--type=nope"}},
		{"context conflicting flags", []string{"context", "--minimal", "--active"}},
		{"graph dot with page", []string{"graph", "--dot", "index.md"}},
		{"json unknown command", []string{"--json", "nope"}},
		{"json unknown flag", []string{"--json", "list", "--bogus"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, exit := runCLI(t, root, tc.args...)
			if exit != 2 {
				t.Errorf("exit = %d, want 2\n%s", exit, out)
			}
		})
	}

	// Runtime failures and help keep their codes.
	if out, exit := runCLI(t, root, "summary", "no-such-page.md"); exit != 1 {
		t.Errorf("missing page exit = %d, want 1\n%s", exit, out)
	}
	if out, exit := runCLI(t, root, "diff", "nope1", "nope2"); exit != 1 {
		t.Errorf("bad ref exit = %d, want 1\n%s", exit, out)
	}
	if out, exit := runCLI(t, root, "list", "--help"); exit != 0 {
		t.Errorf("--help exit = %d, want 0\n%s", exit, out)
	}
	if out, exit := runCLI(t, brokenFixture(t), "lint"); exit != 1 {
		t.Errorf("lint failure exit = %d, want 1\n%s", exit, out)
	}
}

func TestTerminatorHandledUniformly(t *testing.T) {
	root := setupMainTestWiki(t)
	cases := []struct {
		name    string
		args    []string
		want    string
		notWant string
	}{
		{"summary", []string{"summary", "--", "schema.md"}, "# Schema", "not found"},
		{"diff", []string{"diff", "--", "HEAD", "HEAD"}, "no wiki changes between HEAD and HEAD", "invalid git ref"},
		{"refresh", []string{"refresh", "--", "main...HEAD"}, "diff range main...HEAD", "range --"},
		{"log-tail", []string{"log-tail", "--", "1"}, "initial scaffold", "first check"},
		{"changed", []string{"changed", "--", "main...HEAD"}, "", "invalid"},
		{"candidates", []string{"candidates", "--", "main...HEAD"}, "", "invalid"},
		{"sync-prompts", []string{"sync-prompts", "--"}, "sync-prompts:", "error"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, exit := runCLI(t, root, tc.args...)
			if exit != 0 {
				t.Errorf("exit = %d, want 0\n%s", exit, out)
			}
			if tc.want != "" && !strings.Contains(out, tc.want) {
				t.Errorf("output missing %q:\n%s", tc.want, out)
			}
			if strings.Contains(out, tc.notWant) {
				t.Errorf("output contains %q:\n%s", tc.notWant, out)
			}
		})
	}
}

func TestBadWikircDoesNotMaskUsageErrors(t *testing.T) {
	root := setupMainTestWiki(t)
	if err := os.WriteFile(filepath.Join(root, ".wikirc"), []byte("wiki_dir = ../escape\ndefault_diff = --output=x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Sanity: a known engine command reports the config error.
	if out, exit := runCLI(t, root, "list"); exit == 0 {
		t.Fatalf("fixture .wikirc is not broken enough: exit=%d\n%s", exit, out)
	}
	out, exit := runCLI(t, root, "bogus")
	if exit != 2 || !strings.Contains(out, "unknown command: bogus") || strings.Contains(out, "invalid default_diff") {
		t.Errorf("bogus with bad .wikirc: exit=%d\n%s", exit, out)
	}
	for _, args := range [][]string{{"help"}, {"--help"}, {"-h"}, {"version"}, {"list", "--help"}} {
		out, exit := runCLI(t, root, args...)
		if exit != 0 {
			t.Errorf("%v with bad .wikirc: exit=%d\n%s", args, exit, out)
		}
	}
}
