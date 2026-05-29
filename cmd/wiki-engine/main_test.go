package main

import (
	"os"
	"testing"
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
		got := parsePositiveInt(tt.input, tt.fallback)
		if got != tt.want {
			t.Errorf("parsePositiveInt(%q, %d) = %d, want %d", tt.input, tt.fallback, got, tt.want)
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
	args, useJSON = argsAfterFilters()
	if useJSON {
		t.Error("expected useJSON=false without --json")
	}
}
