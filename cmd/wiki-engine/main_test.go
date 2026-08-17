package main

import (
	"os"
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
