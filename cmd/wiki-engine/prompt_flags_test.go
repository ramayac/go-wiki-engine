package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var (
	promptFenceRe   = regexp.MustCompile("(?s)```.*?```")
	promptSpanRe    = regexp.MustCompile("`([^`\n]+)`")
	promptCommandRe = regexp.MustCompile(`wiki-engine ([a-z][a-z-]*)([^\n]*)`)
	promptFlagRe    = regexp.MustCompile(`--[a-z][a-z-]*(?:=[^\s\]|>,;)]*)?`)
)

// codeSnippets returns the text of each fenced block line and each inline code span.
func codeSnippets(text string) []string {
	var out []string
	for _, block := range promptFenceRe.FindAllString(text, -1) {
		out = append(out, strings.Split(block, "\n")...)
	}
	rest := promptFenceRe.ReplaceAllString(text, "")
	for _, m := range promptSpanRe.FindAllStringSubmatch(rest, -1) {
		out = append(out, m[1])
	}
	return out
}

func flagAllowed(spec argSpec, token string) bool {
	if token == "--json" || spec.flags[token] {
		return true
	}
	name, _, hasValue := strings.Cut(token, "=")
	if spec.valueFlags[name] && !hasValue {
		return true
	}
	for _, p := range spec.prefixes {
		if hasValue && p == name+"=" {
			return true
		}
	}
	return false
}

// TestShippedPromptsUseRealCommandsAndFlags guards against invented CLI usage:
// every `wiki-engine <command> --flag` written in a shipped prompt, the
// maintainer guidelines, or the pi skill must exist in the CLI's argument specs.
func TestShippedPromptsUseRealCommandsAndFlags(t *testing.T) {
	root := filepath.Join("..", "..", "scaffold")
	var files []string
	for _, pattern := range []string{
		filepath.Join(root, ".wiki-instructions", "*.md"),
		filepath.Join(root, ".pi", "skills", "wiki", "SKILL.md"),
	} {
		m, err := filepath.Glob(pattern)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, m...)
	}
	if len(files) < 10 {
		t.Fatalf("found only %d shipped prompt files, want at least 10", len(files))
	}

	checked := 0
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, snippet := range codeSnippets(string(data)) {
			for _, m := range promptCommandRe.FindAllStringSubmatch(snippet, -1) {
				cmd, rest := m[1], m[2]
				spec, known := commandArgSpecs[cmd]
				if !known && cmd != "help" {
					t.Errorf("%s: unknown command %q in %q", filepath.Base(f), cmd, snippet)
					continue
				}
				checked++
				for _, flag := range promptFlagRe.FindAllString(rest, -1) {
					if !flagAllowed(spec, flag) {
						t.Errorf("%s: wiki-engine %s does not accept %q (in %q)", filepath.Base(f), cmd, flag, snippet)
					}
				}
			}
		}
	}
	if checked < 50 {
		t.Errorf("checked only %d wiki-engine command mentions, want at least 50 (parser regression?)", checked)
	}
}

// TestPromptFlagCheckRejectsInventedFlags proves the guard itself works.
func TestPromptFlagCheckRejectsInventedFlags(t *testing.T) {
	if flagAllowed(commandArgSpecs["graph"], "--from") {
		t.Error("graph --from must not be accepted")
	}
	if flagAllowed(commandArgSpecs["lint"], "--repo") {
		t.Error("lint --repo must not be accepted")
	}
	if !flagAllowed(commandArgSpecs["search"], "--limit=10") || !flagAllowed(commandArgSpecs["context"], "--sort=topo") {
		t.Error("real flags must be accepted")
	}
}
