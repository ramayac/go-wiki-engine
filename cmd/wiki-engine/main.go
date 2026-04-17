package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/ramayac/go-wiki-engine/internal/config"
	"github.com/ramayac/go-wiki-engine/internal/engine"
	"github.com/ramayac/go-wiki-engine/internal/scaffold"
	"github.com/ramayac/go-wiki-engine/internal/upgrade"
)

// Set by -ldflags at build time.
var version = "dev"

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	cmd := os.Args[1]

	switch cmd {
	case "init":
		runInit()
	case "version":
		fmt.Println(version)
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

func runInit() {
	dir, _ := os.Getwd()
	wikiDir := "wiki"
	if len(os.Args) > 2 {
		wikiDir = os.Args[2]
	}
	if err := scaffold.Init(dir, wikiDir); err != nil {
		fatal(err)
	}
	fmt.Fprintf(os.Stderr, "initialized %s/ with wiki scaffold, .wikirc, prompts, and instructions\n", wikiDir)
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
