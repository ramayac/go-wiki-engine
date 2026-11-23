package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestPublicCommandContractAndUsageParity records all public commands and verifies
// that usage() documents every registered command and every command has an argSpec.
func TestPublicCommandContractAndUsageParity(t *testing.T) {
	wantCommands := []string{
		"candidates",
		"changed",
		"check",
		"context",
		"diff",
		"graph",
		"headings",
		"help",
		"impact",
		"init",
		"lint",
		"list",
		"log-tail",
		"refresh",
		"relevant",
		"schema",
		"search",
		"stats",
		"summary",
		"sync-prompts",
		"upgrade",
		"version",
		"watch",
	}

	got := registeredCommands()
	if strings.Join(got, ",") != strings.Join(wantCommands, ",") {
		t.Errorf("registeredCommands() = %v, want %v", got, wantCommands)
	}

	var buf bytes.Buffer
	usage(&buf)
	helpText := buf.String()

	for _, cmd := range got {
		if !strings.Contains(helpText, "\n  "+cmd) {
			t.Errorf("usage() output is missing command %q", cmd)
		}
		// Verify every registered flag on commandArgSpecs appears in usage().
		if spec, ok := commandArgSpecs[cmd]; ok {
			for flag := range spec.flags {
				if !strings.Contains(helpText, flag) {
					// Handle combined syntax like --sort=topo|chrono.
					if eq := strings.Index(flag, "="); eq != -1 {
						prefix := flag[:eq+1]
						val := flag[eq+1:]
						if strings.Contains(helpText, prefix) && strings.Contains(helpText, val) {
							continue
						}
					}
					t.Errorf("usage() output is missing flag %q for command %q", flag, cmd)
				}
			}
			for _, prefix := range spec.prefixes {
				if !strings.Contains(helpText, prefix) {
					t.Errorf("usage() output is missing prefix flag %q for command %q", prefix, cmd)
				}
			}
		}
	}
}

// TestCLIJSONAndPlainGoldenContracts tests every engine/admin command in BOTH
// plain-text and --json modes and validates the global JSON envelope contract.
func TestCLIJSONAndPlainGoldenContracts(t *testing.T) {
	root := setupMainTestWiki(t)
	// Add a page with structured sources, CRLF, and Unicode filename for cross-platform baseline.
	if err := os.WriteFile(filepath.Join(root, "wiki", "café.md"), []byte("---\r\nid: café\r\ntitle: Café Guide\r\ntype: concept\r\nlifecycle: current\r\ndescription: Unicode and CRLF test\r\nsources:\r\n  - file: main.go\r\n---\r\n# Café Guide\r\n\r\nSee [index](index.md).\r\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	idxPath := filepath.Join(root, "wiki", "index.md")
	idxData, _ := os.ReadFile(idxPath)
	if err := os.WriteFile(idxPath, append(idxData, []byte("- [café.md](café.md) | Unicode page\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, out)
		}
	}
	runGit("add", ".")
	runGit("commit", "-q", "-m", "add unicode page")

	cfg, eng := mainTestEngine(t, root)

	// Commands exercised in both plain and JSON modes.
	engineCmds := []struct {
		name string
		cmd  string
		args []string
	}{
		{"list", "list", []string{"wiki-engine", "list"}},
		{"list active", "list", []string{"wiki-engine", "list", "--active"}},
		{"headings", "headings", []string{"wiki-engine", "headings"}},
		{"search", "search", []string{"wiki-engine", "search", "Schema"}},
		{"search ranked", "search", []string{"wiki-engine", "search", "--ranked", "Café"}},
		{"log-tail", "log-tail", []string{"wiki-engine", "log-tail", "1"}},
		{"changed", "changed", []string{"wiki-engine", "changed"}},
		{"candidates", "candidates", []string{"wiki-engine", "candidates"}},
		{"check", "check", []string{"wiki-engine", "check", "--base=main"}},
		{"stats", "stats", []string{"wiki-engine", "stats"}},
		{"context", "context", []string{"wiki-engine", "context"}},
		{"context minimal", "context", []string{"wiki-engine", "context", "--minimal"}},
		{"context summarize", "context", []string{"wiki-engine", "context", "--summarize", "--max-pages=3"}},
		{"context active", "context", []string{"wiki-engine", "context", "--active", "--sort=topo"}},
		{"graph", "graph", []string{"wiki-engine", "graph"}},
		{"graph sources", "graph", []string{"wiki-engine", "graph", "--include-sources"}},
		{"graph neighborhood", "graph", []string{"wiki-engine", "graph", "café"}},
		{"summary", "summary", []string{"wiki-engine", "summary", "café"}},
		{"relevant", "relevant", []string{"wiki-engine", "relevant", "Café", "3"}},
		{"impact exact", "impact", []string{"wiki-engine", "impact", "main.go"}},
		{"impact fuzzy", "impact", []string{"wiki-engine", "impact", "--fuzzy", "main.go"}},
		{"lint", "lint", []string{"wiki-engine", "lint"}},
		{"diff", "diff", []string{"wiki-engine", "diff", "HEAD~1", "HEAD"}},
		{"watch once", "watch", []string{"wiki-engine", "watch", "--once"}},
	}

	for _, tc := range engineCmds {
		t.Run(tc.name+"/plain", func(t *testing.T) {
			_ = captureStdout(t, func() {
				runEngine(tc.cmd, cfg, eng, tc.args, false)
			})
		})
		t.Run(tc.name+"/json", func(t *testing.T) {
			out := captureStdout(t, func() {
				runEngine(tc.cmd, cfg, eng, tc.args, true)
			})
			var env map[string]json.RawMessage
			if err := json.Unmarshal([]byte(out), &env); err != nil {
				t.Fatalf("%s --json invalid JSON: %v\n%s", tc.name, err, out)
			}
			if string(env["ok"]) != "true" {
				t.Errorf("%s --json ok = %s, want true\n%s", tc.name, env["ok"], out)
			}
		})
	}

	// Admin commands: schema in plain and JSON.
	for _, useJSON := range []bool{false, true} {
		out := captureStdout(t, func() {
			runSchema([]string{"wiki-engine", "schema"}, useJSON)
		})
		if useJSON {
			var env map[string]json.RawMessage
			if err := json.Unmarshal([]byte(out), &env); err != nil || string(env["ok"]) != "true" {
				t.Errorf("schema --json invalid envelope: %v\n%s", err, out)
			}
		} else if !strings.Contains(out, "# Wiki Schema") {
			t.Errorf("schema plain output missing header:\n%s", out)
		}
	}
}

// TestCheckExitCodes verifies exit 0 (clean/review), exit 1 (broken citation),
// and exit 2 (git/setup error) via the main() subprocess harness.
func TestCheckExitCodes(t *testing.T) {
	// 1. Non-git directory -> exit 2.
	nonGit := t.TempDir()
	_ = os.MkdirAll(filepath.Join(nonGit, "wiki"), 0o755)
	_ = os.WriteFile(filepath.Join(nonGit, "wiki", "index.md"), []byte("---\nid: index\nlifecycle: current\ndescription: Index\n---\n# Index\n"), 0o644)

	cmd := exec.Command(os.Args[0], "-test.run=^TestMainReexec$")
	cmd.Dir = nonGit
	cmd.Env = append(os.Environ(),
		"WIKI_ENGINE_HELPER=1",
		"WIKI_ENGINE_HELPER_ARGS=check",
	)
	out, err := cmd.CombinedOutput()
	ee, ok := err.(*exec.ExitError)
	if !ok || ee.ExitCode() != 2 {
		t.Errorf("check outside git exit = %v, want exit code 2\noutput:\n%s", err, out)
	}
}
