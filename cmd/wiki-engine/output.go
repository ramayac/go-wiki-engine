package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/ramayac/go-wiki-engine/internal/engine"
)

// useJSONMode records whether --json was requested for this invocation.
var useJSONMode bool

func writeJSON(data interface{}) {
	writeJSONResultTo(os.Stdout, data, true, "")
}

func writeJSONResult(data interface{}, ok bool, errMsg string) {
	writeJSONResultTo(os.Stdout, data, ok, errMsg)
}

func writeJSONResultTo(w io.Writer, data interface{}, ok bool, errMsg string) {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	out := engine.JSONOutput{OK: ok, Data: data, Error: errMsg}
	if err := enc.Encode(out); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

// Exit codes: 0 success, 1 runtime or finding failure, 2 usage error
// (check also uses 2 for git and setup errors).
const (
	exitFailure = 1
	exitUsage   = 2
)

// nonNil returns s, or an empty slice when s is nil, so JSON encodes [] and not null.
func nonNil[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}

func fatal(err error) {
	fatalWithCode(err, exitFailure)
}

// usageFatal reports an invalid invocation (bad flag, value, or argument).
func usageFatal(err error) {
	fatalWithCode(err, exitUsage)
}

// unknownCommand reports an unknown command and exits with the usage code.
func unknownCommand(cmd string) {
	if useJSONMode {
		writeJSONResult(nil, false, fmt.Sprintf("unknown command: %s", cmd))
	} else {
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", cmd)
		usage(os.Stderr)
	}
	os.Exit(exitUsage)
}

func fatalWithCode(err error, code int) {
	if useJSONMode {
		writeJSONResult(nil, false, err.Error())
	} else {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
	}
	os.Exit(code)
}

func usageError(usageLine string) {
	if useJSONMode {
		writeJSONResult(nil, false, usageLine)
	} else {
		fmt.Fprintln(os.Stderr, usageLine)
	}
	os.Exit(exitUsage)
}

func usage(out io.Writer) {
	_, _ = fmt.Fprintln(out, `wiki-engine — repo-local wiki management tool

Usage: wiki-engine [--json] <command> [arguments]

Commands:
  init [wiki-dir] [--wiki-dir=<name>]
                           Scaffold or reconcile a wiki (or rename with --wiki-dir)
  sync-prompts             Update all tool instruction layers to the latest version
  schema [--type=<t>] [--section=<s>]
                           Print the embedded wiki schema, lifecycle rules, or page templates
  list [--active]          List all wiki files (or active pages only)
  headings                 List all Markdown headings with file paths
  search <query> [--tag=<t>] [--type=<t>] [--lifecycle=<l>] [--include-history] [--limit=<n>] [--keyword] [--ranked]
                           Search across wiki pages (excludes historical pages by default)
  log-tail [n]             Show the most recent N log headings
  changed [diff-range] [--base=<ref>] [--no-working-tree]
                           List non-wiki files changed on branch (and working tree)
  candidates [diff-range] [--base=<ref>] [--no-working-tree]
                           Filter changed files to ingest-worthy candidates
  check [--base=<ref>] [--no-working-tree]
                           Branch-aware citation and drift check (exit 0 ok/review, 1 broken, 2 setup)
  stats                    Show aggregate wiki statistics
  context [--minimal] [--summarize] [--active] [--sort=topo|chrono] [--max-pages=<n>] [--max-bytes=<n>]
                           Condensed wiki snapshot / active graph for agent context loading
  graph [page-or-id] [--strict] [--dot] [--include-sources]
                           Navigation map of the active wiki graph (tree, neighborhood, DOT export)
  summary <page-or-id>     Show first heading and paragraph of a page
  relevant <query> [n] [--tag=<t>] [--type=<t>] [--lifecycle=<l>] [--include-history] [--limit=<n>]
                           Rank wiki pages by field-weighted relevance to a query
  impact [--fuzzy] <file...>
                           Show which wiki pages cite changed files (exact by default; --fuzzy for prose)
  lint [--check=<a,b>] [--skip=<a,b>] [--fix]
                           Check wiki structure, schema, links, and sources (or apply mechanical --fix)
  diff <from> <to>         Show wiki file changes between two git refs
  watch [--once]           Poll for changes and lint issues (interval from .wikirc)
  refresh [diff-range]     Run the full maintenance snapshot
  upgrade [--from-source] [--require-attestation]
                           Fail-closed self-upgrade from GitHub Releases (checksum + provenance)
  version                  Print the version
  help                     Show this help

Add --json (accepted anywhere) for structured JSON output. On error, --json
commands emit {"ok": false, "error": "..."} and exit non-zero.
Use -- to end flag parsing, e.g. wiki-engine search -- --check.
-h/--help works after any command.`)
}
