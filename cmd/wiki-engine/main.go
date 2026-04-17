package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"

	"github.com/ramayac/go-wiki-engine/internal/config"
	"github.com/ramayac/go-wiki-engine/internal/engine"
	"github.com/ramayac/go-wiki-engine/internal/scaffold"
	"github.com/ramayac/go-wiki-engine/internal/upgrade"
)

// Set by -ldflags at build time. Falls back to embedded module version when
// installed via `go install` without ldflags (e.g. after `wiki-engine upgrade`).
var version = "dev"

func getVersion() string {
	if version != "dev" {
		return version
	}
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return version
}

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	cmd := os.Args[1]

	switch cmd {
	case "init":
		runInit()
	case "sync-prompts":
		runSyncPrompts()
	case "version":
		fmt.Println(getVersion())
	case "upgrade":
		if err := upgrade.Run(); err != nil {
			fatal(err)
		}
	case "help", "-h", "--help":
		usage()
	default:
		// All other commands need a loaded config and engine.
		cfg, eng := loadEngine()
		runEngine(cmd, cfg, eng)
	}
}

func runSyncPrompts() {
	dir, _ := os.Getwd()

	// Capture which shim files already exist BEFORE syncing. syncShims uses
	// create-only semantics, so any shim not yet present will be created fresh
	// (already the standard template) and does not need a migration reminder.
	var preExistingShims []string
	for _, name := range []string{"AGENTS.md", "CLAUDE.md"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
			preExistingShims = append(preExistingShims, name)
		}
	}

	updated, err := scaffold.SyncPrompts(dir)
	if err != nil {
		fatal(err)
	}
	if len(updated) == 0 {
		fmt.Fprintln(os.Stderr, "sync-prompts: no .github files found in scaffold (unexpected)")
		return
	}
	for _, f := range updated {
		fmt.Fprintf(os.Stderr, "updated %s\n", f)
	}
	fmt.Fprintf(os.Stderr, "sync-prompts: %d file(s) updated\n", len(updated))

	if len(preExistingShims) > 0 {
		fmt.Fprintf(os.Stdout, "\ntip: %s already exist and were not modified.\n", strings.Join(preExistingShims, " and "))
		fmt.Fprintln(os.Stdout, "     If they contain custom instructions, run /wiki-migrate-shims to migrate")
		fmt.Fprintln(os.Stdout, "     that content into the wiki and replace the files with standard redirect shims.")
	}
}

func runInit() {
	dir, _ := os.Getwd()
	wikiDir := "wiki"
	if len(os.Args) > 2 {
		wikiDir = os.Args[2]
	}
	if err := scaffold.Init(dir, wikiDir); err != nil {
		fatal(err)
	}
	fmt.Fprintf(os.Stderr, "initialized %s/ with wiki scaffold, .wikirc, prompts, instructions, and AGENTS.md/CLAUDE.md shims\n", wikiDir)
	fmt.Fprintln(os.Stderr, "next steps:")
	fmt.Fprintln(os.Stderr, "  1. Edit .wikirc to set your ignore patterns")
	fmt.Fprintln(os.Stderr, "  2. Edit wiki/repo-map.md with your project's architecture")
	fmt.Fprintln(os.Stderr, "  3. Run: wiki-engine lint")
}

func loadEngine() (*config.Config, *engine.Engine) {
	dir, err := os.Getwd()
	if err != nil {
		fatal(err)
	}
	cfg, err := config.Load(dir)
	if err != nil {
		fatal(err)
	}
	return cfg, engine.New(cfg, dir)
}

func runEngine(cmd string, cfg *config.Config, eng *engine.Engine) {
	switch cmd {
	case "list":
		files, err := eng.List()
		if err != nil {
			fatal(err)
		}
		for _, f := range files {
			fmt.Println(f)
		}

	case "headings":
		entries, err := eng.Headings()
		if err != nil {
			fatal(err)
		}
		for _, e := range entries {
			fmt.Printf("%s:%d:%s\n", e.File, e.Line, e.Heading)
		}

	case "search":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "usage: wiki-engine search <query>")
			os.Exit(1)
		}
		query := strings.Join(os.Args[2:], " ")
		results, err := eng.Search(query)
		if err != nil {
			fatal(err)
		}
		for _, r := range results {
			fmt.Printf("%s:%d:%s\n", r.File, r.Line, r.Text)
		}

	case "log-tail":
		n := cfg.LogLines
		if len(os.Args) > 2 {
			n = parsePositiveInt(os.Args[2], cfg.LogLines)
		}
		lines, err := eng.LogTail(n)
		if err != nil {
			fatal(err)
		}
		for _, l := range lines {
			fmt.Println(l)
		}

	case "changed":
		diff := cfg.DefaultDiff
		if len(os.Args) > 2 {
			diff = os.Args[2]
		}
		files, err := eng.Changed(diff)
		if err != nil {
			fatal(err)
		}
		for _, f := range files {
			fmt.Println(f)
		}

	case "candidates":
		diff := cfg.DefaultDiff
		if len(os.Args) > 2 {
			diff = os.Args[2]
		}
		files, err := eng.Candidates(diff)
		if err != nil {
			fatal(err)
		}
		for _, f := range files {
			fmt.Println(f)
		}

	case "lint":
		result := eng.Lint()
		if result.OK {
			fmt.Println("wiki lint OK")
		} else {
			for _, m := range result.Messages {
				fmt.Fprintln(os.Stderr, m)
			}
			os.Exit(1)
		}

	case "refresh":
		diff := cfg.DefaultDiff
		if len(os.Args) > 2 {
			diff = os.Args[2]
		}
		out, err := eng.Refresh(diff)
		if err != nil {
			fatal(err)
		}
		fmt.Print(out)

	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", cmd)
		usage()
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, `wiki-engine — repo-local wiki management tool

Usage: wiki-engine <command> [arguments]

Commands:
  init [wiki-dir]         Scaffold a new wiki into the current repo
  sync-prompts            Update .github/prompts/ and .github/instructions/ to the latest version
  list                    List all wiki files
  headings                List all Markdown headings with file paths
  search <query>          Case-insensitive search across wiki files
  log-tail [n]            Show the last N log headings
  changed [diff-range]    List non-wiki files changed in a git diff range
  candidates [diff-range] Filter changed files to ingest-worthy candidates
  lint                    Check wiki structure, links, and markers
  refresh [diff-range]    Run the full maintenance snapshot
  upgrade                 Self-upgrade to the latest version via go install
  version                 Print the version
  help                    Show this help`)
}

func fatal(err error) {
	fmt.Fprintf(os.Stderr, "error: %v\n", err)
	os.Exit(1)
}

func parsePositiveInt(s string, fallback int) int {
	n := 0
	for _, c := range s {
		if c >= '0' && c <= '9' {
			n = n*10 + int(c-'0')
		}
	}
	if n <= 0 {
		return fallback
	}
	return n
}
