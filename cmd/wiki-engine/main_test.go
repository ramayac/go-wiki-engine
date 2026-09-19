package main

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/ramayac/go-wiki-engine/internal/config"
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
