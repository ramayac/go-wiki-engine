package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ramayac/go-wiki-engine/internal/config"
	"github.com/ramayac/go-wiki-engine/internal/engine"
)

func TestGetVersion(t *testing.T) {
	v := getVersion()
	if v == "" {
		t.Error("getVersion returned empty string")
	}
}

func TestParsePositiveInt(t *testing.T) {
	tests := []struct {
		input    string
		fallback int
		want     int
	}{
		{"10", 5, 10},
		{"0", 5, 5},
		{"abc", 5, 5},
		{"25", 5, 25},
		{"-1", 5, 5},
		{"", 5, 5},
	}
	for _, tt := range tests {
		got := config.ParsePositiveInt(tt.input, tt.fallback)
		if got != tt.want {
			t.Errorf("ParsePositiveInt(%q, %d) = %d, want %d", tt.input, tt.fallback, got, tt.want)
		}
	}
}

func TestArgsAfterFilters(t *testing.T) {
	// Save and restore os.Args.
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	os.Args = []string{"wiki-engine", "--json", "list"}
	args, useJSON := argsAfterFilters()
	if !useJSON {
		t.Error("expected useJSON=true when --json is present")
	}
	if len(args) != 2 || args[1] != "list" {
		t.Errorf("expected args=[wiki-engine list], got %v", args)
	}

	os.Args = []string{"wiki-engine", "list"}
	_, useJSON = argsAfterFilters()
	if useJSON {
		t.Error("expected useJSON=false without --json")
	}

	// A "--" terminator protects a literal --json argument.
	os.Args = []string{"wiki-engine", "search", "--", "--json"}
	args, useJSON = argsAfterFilters()
	if useJSON {
		t.Error("expected useJSON=false when --json appears after --")
	}
	if len(args) != 4 || args[3] != "--json" {
		t.Errorf("expected --json preserved after --, got %v", args)
	}

	// Before the terminator, --json is still the mode switch.
	os.Args = []string{"wiki-engine", "--json", "search", "--", "x"}
	args, useJSON = argsAfterFilters()
	if !useJSON {
		t.Error("expected useJSON=true when --json appears before --")
	}
	if len(args) != 4 {
		t.Errorf("expected args=[wiki-engine search -- x], got %v", args)
	}
}

func TestWriteJSONResultTo(t *testing.T) {
	var buf bytes.Buffer
	writeJSONResultTo(&buf, map[string]string{"wiki_dir": "docs"}, true, "")

	var out map[string]json.RawMessage
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("envelope is not valid JSON: %v\n%s", err, buf.String())
	}
	if string(out["ok"]) != "true" {
		t.Errorf("ok = %s, want true", out["ok"])
	}
	if !strings.Contains(string(out["data"]), "docs") {
		t.Errorf("data = %s, want wiki_dir docs", out["data"])
	}
	if _, present := out["error"]; present {
		t.Error("error field must be omitted when ok")
	}

	// Error envelope: data omitted, error present.
	buf.Reset()
	writeJSONResultTo(&buf, nil, false, "boom")
	out = map[string]json.RawMessage{}
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("error envelope is not valid JSON: %v\n%s", err, buf.String())
	}
	if string(out["ok"]) != "false" {
		t.Errorf("ok = %s, want false", out["ok"])
	}
	if string(out["error"]) != `"boom"` {
		t.Errorf("error = %s, want \"boom\"", out["error"])
	}
	if _, present := out["data"]; present {
		t.Error("data field must be omitted on error")
	}
}

func TestValidateCommandArgs(t *testing.T) {
	tests := []struct {
		name    string
		cmd     string
		args    []string
		wantErr bool
	}{
		{"known flag", "list", []string{"--active"}, false},
		{"unknown flag", "list", []string{"--bogus"}, true},
		{"unexpected positional", "list", []string{"extra"}, true},
		{"headings no args", "headings", nil, false},
		{"search multi positional", "search", []string{"some query terms"}, false},
		{"context known flags", "context", []string{"--active", "--sort=topo"}, false},
		{"context unknown flag", "context", []string{"--sort=none"}, true},
		{"graph known flags", "graph", []string{"--strict", "--dot"}, false},
		{"graph unknown flag", "graph", []string{"--bogus"}, true},
		{"graph one positional", "graph", []string{"prologue/schema.md"}, false},
		{"graph too many positional", "graph", []string{"a.md", "b.md"}, true},
		{"lint check prefix", "lint", []string{"--check=front-matter"}, false},
		{"lint unknown flag", "lint", []string{"--quiet"}, true},
		{"impact unlimited positional", "impact", []string{"a.go", "b.go", "c.go"}, false},
		{"terminator makes flags positional", "search", []string{"--", "--check"}, false},
		{"terminator preserves flag rejection before it", "search", []string{"--bogus", "--", "x"}, true},
		{"terminator counts positionals", "list", []string{"--", "extra"}, true},
		{"terminator alone", "search", []string{"--"}, false},
		{"init one positional", "init", []string{"docs"}, false},
		{"init unknown flag", "init", []string{"--bogus"}, true},
		{"version unknown flag", "version", []string{"--bogus"}, true},
		{"upgrade unexpected positional", "upgrade", []string{"extra"}, true},
		{"sync-prompts no args", "sync-prompts", nil, false},
		{"unknown command tolerated here", "nope", []string{"--anything"}, false},
	}
	for _, tt := range tests {
		err := validateCommandArgs(tt.cmd, tt.args)
		if (err != nil) != tt.wantErr {
			t.Errorf("%s: validateCommandArgs(%q, %v) error = %v, wantErr %t", tt.name, tt.cmd, tt.args, err, tt.wantErr)
		}
	}
}

func TestValidateLintSelectors(t *testing.T) {
	tests := []struct {
		name    string
		check   []string
		skip    []string
		wantErr bool
	}{
		{"valid checker", []string{"front-matter"}, nil, false},
		{"all in check", []string{"all"}, nil, false},
		{"unknown check", []string{"bogus"}, nil, true},
		{"unknown skip", nil, []string{"bogus"}, true},
		{"skip all rejected", nil, []string{"all"}, true},
		{"empty strings tolerated", []string{""}, []string{""}, false},
		{"mixed valid", []string{"orphans", "leaf-pages"}, []string{"markers"}, false},
	}
	for _, tt := range tests {
		err := validateLintSelectors(tt.check, tt.skip)
		if (err != nil) != tt.wantErr {
			t.Errorf("%s: validateLintSelectors(%v, %v) error = %v, wantErr %t", tt.name, tt.check, tt.skip, err, tt.wantErr)
		}
	}
}

func TestPositionalArgs(t *testing.T) {
	tests := []struct {
		in   []string
		want []string
	}{
		{[]string{"--", "--check"}, []string{"--check"}},
		{[]string{"--check"}, []string{"--check"}},
		{[]string{"a.go", "--", "b.go"}, []string{"a.go", "b.go"}},
		{[]string{"--", "--", "--"}, []string{}},
		{[]string{}, []string{}},
	}
	for _, tt := range tests {
		got := positionalArgs(tt.in)
		if len(got) != len(tt.want) {
			t.Errorf("positionalArgs(%v) = %v, want %v", tt.in, got, tt.want)
			continue
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("positionalArgs(%v) = %v, want %v", tt.in, got, tt.want)
				break
			}
		}
	}
}

// --- CLI-level tests: runEngine dispatch, init/sync, watch cycle, main() ---

// setupMainTestWiki mirrors the engine package's lint-clean fixture: a
// minimal wiki that passes full lint. It is placed under git with a feature
// branch checked out so diff-based commands (changed, candidates, refresh,
// watch --once) work in-process.
func setupMainTestWiki(t *testing.T) (root string) {
	t.Helper()
	root = t.TempDir()
	opsDir := filepath.Join(root, "wiki", "operations")
	if err := os.MkdirAll(opsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"wiki/README.md":            "---\nstatus: current\ndescription: README\n---\n# Wiki\n",
		"wiki/index.md":             "---\nstatus: current\ndescription: Index\n---\n# Index\n\n- [README.md](README.md) | Overview\n- [schema.md](schema.md) | Schema\n- [log.md](log.md) | Log\n- [repo-map.md](repo-map.md) | Repo Map\n- [phases.md](phases.md) | Phases\n- [operations/ingest.md](operations/ingest.md) | Ingest\n- [operations/query.md](operations/query.md) | Query\n- [operations/lint.md](operations/lint.md) | Lint\n",
		"wiki/log.md":               "---\nstatus: current\ndescription: Log\n---\n# Log\n\n## [2026-04-16] ingest | initial scaffold\n\n- Created wiki.\n\n## [2026-04-15] lint | first check\n\n- All OK.\n",
		"wiki/schema.md":            "---\nstatus: current\ndescription: Schema\n---\n# Schema\n\nSee main.go for the entry point.\n",
		"wiki/phases.md":            "---\nstatus: current\ndescription: Phases\n---\n# Phases\n",
		"wiki/repo-map.md":          "---\nstatus: current\ndescription: Repo Map\n---\n# Repo Map\n",
		"wiki/operations/ingest.md": "---\nstatus: current\ndescription: Ingest\n---\n# Ingest\n",
		"wiki/operations/query.md":  "---\nstatus: current\ndescription: Query\n---\n# Query\n",
		"wiki/operations/lint.md":   "---\nstatus: current\ndescription: Lint\n---\n# Lint\n",
		"main.go":                   "package main\n",
	}
	for rel, content := range files {
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	gitCmd := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, out)
		}
	}
	gitCmd("init", "-q", "-b", "main")
	gitCmd("config", "user.email", "test@test")
	gitCmd("config", "user.name", "Test")
	gitCmd("add", ".")
	gitCmd("commit", "-q", "-m", "init")
	gitCmd("checkout", "-q", "-b", "feature")
	return root
}

func mainTestEngine(t *testing.T, root string) (*config.Config, *engine.Engine) {
	t.Helper()
	cfg := &config.Config{
		WikiDir:     "wiki",
		DefaultDiff: "main...HEAD",
		LogLines:    10,
		Ignore:      []string{"wiki/", "bin/", "*.log"},
	}
	return cfg, engine.New(cfg, root)
}

// captureStdout runs fn with os.Stdout redirected to a pipe and returns
// everything written. Stderr is untouched.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	done := make(chan string, 1)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		done <- buf.String()
	}()
	fn()
	_ = w.Close()
	os.Stdout = old
	return <-done
}

func TestRunEngineCommands(t *testing.T) {
	root := setupMainTestWiki(t)
	cfg, eng := mainTestEngine(t, root)

	cases := []struct {
		name    string
		cmd     string
		args    []string
		useJSON bool
		want    []string
	}{
		{"list", "list", []string{"wiki-engine", "list"}, false, []string{"wiki/index.md", "wiki/schema.md"}},
		{"list active", "list", []string{"wiki-engine", "list", "--active"}, false, []string{"wiki/index.md"}},
		{"headings", "headings", []string{"wiki-engine", "headings"}, false, []string{"# Schema"}},
		{"search", "search", []string{"wiki-engine", "search", "schema"}, false, []string{"wiki/schema.md"}},
		{"log-tail", "log-tail", []string{"wiki-engine", "log-tail", "1"}, false, []string{"## [2026-04-16] ingest | initial scaffold"}},
		{"stats", "stats", []string{"wiki-engine", "stats"}, false, []string{"files:", "headings:"}},
		{"context catalog", "context", []string{"wiki-engine", "context"}, false, []string{"== wiki status ==", "schema.md [current]"}},
		{"context active", "context", []string{"wiki-engine", "context", "--active"}, false, []string{"== active wiki graph ==", "index.md [current]"}},
		{"context active json", "context", []string{"wiki-engine", "context", "--active"}, true, []string{`"nodes"`, `"edges"`}},
		{"summary", "summary", []string{"wiki-engine", "summary", "schema.md"}, false, []string{"# Schema"}},
		{"relevant", "relevant", []string{"wiki-engine", "relevant", "schema"}, false, []string{"wiki/schema.md"}},
		{"impact", "impact", []string{"wiki-engine", "impact", "main.go"}, false, []string{"wiki/schema.md"}},
		{"graph tree", "graph", []string{"wiki-engine", "graph"}, false, []string{"== wiki graph ==", "index.md [current]", "stats:"}},
		{"graph neighborhood", "graph", []string{"wiki-engine", "graph", "schema.md"}, false, []string{"== backlinks ==", "== links =="}},
		{"graph dot", "graph", []string{"wiki-engine", "graph", "--dot"}, false, []string{"digraph wiki {"}},
		{"graph json", "graph", []string{"wiki-engine", "graph"}, true, []string{`"stats"`, `"nodes"`}},
		{"graph neighborhood json", "graph", []string{"wiki-engine", "graph", "schema.md"}, true, []string{`"backlinks"`}},
		{"lint ok", "lint", []string{"wiki-engine", "lint"}, false, []string{"wiki lint OK"}},
		{"lint json ok", "lint", []string{"wiki-engine", "lint"}, true, []string{`"ok": true`}},
		{"changed clean", "changed", []string{"wiki-engine", "changed"}, false, nil},
		{"candidates clean", "candidates", []string{"wiki-engine", "candidates"}, false, nil},
		{"refresh clean", "refresh", []string{"wiki-engine", "refresh"}, false, []string{"no wiki refresh needed"}},
		{"watch once clean", "watch", []string{"wiki-engine", "watch", "--once"}, false, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out := captureStdout(t, func() {
				runEngine(tc.cmd, cfg, eng, tc.args, tc.useJSON)
			})
			for _, want := range tc.want {
				if !strings.Contains(out, want) {
					t.Errorf("%s output missing %q:\n%s", tc.name, want, out)
				}
			}
		})
	}
}

func TestUsage(t *testing.T) {
	var buf bytes.Buffer
	usage(&buf)
	out := buf.String()
	for _, want := range []string{"Commands:", "graph [page] [--strict] [--dot]", "context [--minimal]"} {
		if !strings.Contains(out, want) {
			t.Errorf("usage output missing %q", want)
		}
	}
}

func TestRunInit(t *testing.T) {
	dir := t.TempDir()
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(old) }()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

	runInit([]string{"wiki-engine", "init"}, false)

	for _, want := range []string{"wiki/index.md", ".wikirc", ".github/prompts/wiki-ingest.prompt.md", ".wiki-instructions/wiki-maintainer.md"} {
		if _, err := os.Stat(filepath.Join(dir, want)); err != nil {
			t.Errorf("init did not create %s: %v", want, err)
		}
	}

	// Custom wiki dir name.
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	runInit([]string{"wiki-engine", "init", "docs"}, false)
	if _, err := os.Stat(filepath.Join(dir, "docs", "index.md")); err != nil {
		t.Errorf("init docs did not create docs/index.md: %v", err)
	}
}

func TestRunSyncPrompts(t *testing.T) {
	dir := t.TempDir()
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(old) }()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

	runSyncPrompts(false)

	for _, want := range []string{".wiki-instructions/wiki-maintainer.md", ".github/prompts/wiki-ingest.prompt.md", ".pi/skills/wiki/SKILL.md"} {
		if _, err := os.Stat(filepath.Join(dir, want)); err != nil {
			t.Errorf("sync-prompts did not create %s: %v", want, err)
		}
	}
}

func TestRunWatchCycle(t *testing.T) {
	root := setupMainTestWiki(t)
	_, eng := mainTestEngine(t, root)

	// Clean wiki: gate passes.
	if runWatchCycle(eng, false) {
		t.Error("clean wiki should pass the watch lint gate")
	}

	// Broken link: gate fails.
	if err := os.WriteFile(filepath.Join(root, "wiki", "index.md"), []byte("---\nstatus: current\ndescription: Index\n---\n# Index\n\n- [missing.md](missing.md)\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !runWatchCycle(eng, false) {
		t.Error("broken link should fail the watch lint gate")
	}
}

// TestMainReexec drives main() in a subprocess (the standard Go pattern for
// code paths that call os.Exit) and asserts exit codes and output.
func TestMainReexec(t *testing.T) {
	if os.Getenv("WIKI_ENGINE_HELPER") == "1" {
		os.Args = append([]string{"wiki-engine"}, strings.Split(os.Getenv("WIKI_ENGINE_HELPER_ARGS"), "\n")...)
		main()
		os.Exit(0)
	}

	root := setupMainTestWiki(t)
	broken := setupMainTestWiki(t)
	if err := os.WriteFile(filepath.Join(broken, "wiki", "index.md"), []byte("---\nstatus: current\ndescription: Index\n---\n# Index\n\n- [missing.md](missing.md)\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name     string
		dir      string
		args     []string
		wantExit int
		wantOut  string
	}{
		{"no args shows usage", root, nil, 1, "Usage:"},
		{"unknown command", root, []string{"nope"}, 1, "unknown command"},
		{"json unknown command envelope", root, []string{"--json", "nope"}, 1, `"ok": false`},
		{"unknown flag rejected", root, []string{"list", "--bogus"}, 1, "unknown flag"},
		{"version", root, []string{"version"}, 0, "dev"},
		{"list works", root, []string{"list"}, 0, "wiki/index.md"},
		{"graph strict clean passes", root, []string{"graph", "--strict"}, 0, "== wiki graph =="},
		{"graph strict broken fails", broken, []string{"graph", "--strict"}, 1, "== graph issues =="},
		{"watch once clean", root, []string{"watch", "--once"}, 0, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cmd := exec.Command(os.Args[0], "-test.run=^TestMainReexec$")
			cmd.Dir = tc.dir
			cmd.Env = append(os.Environ(),
				"WIKI_ENGINE_HELPER=1",
				"WIKI_ENGINE_HELPER_ARGS="+strings.Join(tc.args, "\n"),
			)
			out, err := cmd.CombinedOutput()
			exit := 0
			if ee, ok := err.(*exec.ExitError); ok {
				exit = ee.ExitCode()
			} else if err != nil {
				t.Fatalf("helper failed to run: %v", err)
			}
			if exit != tc.wantExit {
				t.Errorf("exit = %d, want %d\noutput:\n%s", exit, tc.wantExit, out)
			}
			if tc.wantOut != "" && !strings.Contains(string(out), tc.wantOut) {
				t.Errorf("output missing %q:\n%s", tc.wantOut, out)
			}
		})
	}
}
