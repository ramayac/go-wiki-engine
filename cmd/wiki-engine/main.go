package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"time"

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

// argsAfterFilters returns os.Args with --json removed and whether --json was present.
func argsAfterFilters() ([]string, bool) {
	var filtered []string
	useJSON := false
	for _, a := range os.Args {
		if a == "--json" {
			useJSON = true
			continue
		}
		filtered = append(filtered, a)
	}
	return filtered, useJSON
}

// writeJSON writes the standard success envelope.
func writeJSON(data interface{}) {
	writeJSONResult(data, true, "")
}

// writeJSONResult writes the standard envelope with an explicit OK status.
// errMsg is emitted only when ok is false.
func writeJSONResult(data interface{}, ok bool, errMsg string) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	out := engine.JSONOutput{OK: ok, Data: data, Error: errMsg}
	if err := enc.Encode(out); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func main() {
	// Filter --json before command dispatch.
	args, useJSON := argsAfterFilters()

	if len(args) < 2 {
		usage()
		os.Exit(1)
	}

	cmd := args[1]

	switch cmd {
	case "init":
		runInit(args)
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
		runEngine(cmd, cfg, eng, args, useJSON)
	}
}

func runSyncPrompts() {
	dir, err := os.Getwd()
	if err != nil {
		fatal(err)
	}

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
		fmt.Fprintln(os.Stderr, "sync-prompts: no instruction files found in scaffold (unexpected)")
		return
	}
	for _, f := range updated {
		fmt.Fprintf(os.Stderr, "updated %s\n", f)
	}
	fmt.Fprintf(os.Stderr, "sync-prompts: %d file(s) updated\n", len(updated))

	if len(preExistingShims) > 0 {
		fmt.Fprintf(os.Stdout, "\ntip: %s already exist and were not modified.\n", strings.Join(preExistingShims, " and "))
		fmt.Fprintln(os.Stdout, "     Custom content in these files is preserved. Review it against wiki/README.md,")
		fmt.Fprintln(os.Stdout, "     then run wiki-engine sync-prompts again after adopting the standard redirect shims.")
	}
}

func runInit(args []string) {
	dir, err := os.Getwd()
	if err != nil {
		fatal(err)
	}
	wikiDir := "wiki"
	if len(args) > 2 {
		wikiDir = args[2]
	}
	if err := scaffold.Init(dir, wikiDir); err != nil {
		fatal(err)
	}
	fmt.Fprintf(os.Stderr, "initialized %s/ with wiki scaffold, .wikirc, prompts, instructions, AGENTS.md/CLAUDE.md shims, .claude/commands/, and .pi/skills/\n", wikiDir)
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

func runEngine(cmd string, cfg *config.Config, eng *engine.Engine, args []string, useJSON bool) {
	switch cmd {
	case "list":
		activeOnly := false
		for _, a := range args[2:] {
			if a == "--active" {
				activeOnly = true
			}
		}
		files, err := eng.List()
		if err != nil {
			fatal(err)
		}
		if activeOnly {
			var filtered []string
			for _, f := range files {
				fm, _ := eng.PageFrontMatter(f)
				if fm.Status == "current" || fm.Status == "planned" {
					filtered = append(filtered, f)
				}
			}
			files = filtered
		}
		if useJSON {
			writeJSON(files)
			return
		}
		for _, f := range files {
			fmt.Println(f)
		}

	case "headings":
		entries, err := eng.Headings()
		if err != nil {
			fatal(err)
		}
		if useJSON {
			writeJSON(entries)
			return
		}
		for _, e := range entries {
			fmt.Printf("%s:%d:%s\n", e.File, e.Line, e.Heading)
		}

	case "search":
		if len(args) < 3 {
			fmt.Fprintln(os.Stderr, "usage: wiki-engine search <query>")
			os.Exit(1)
		}
		query := strings.Join(args[2:], " ")
		results, err := eng.Search(query)
		if err != nil {
			fatal(err)
		}
		if useJSON {
			writeJSON(results)
			return
		}
		for _, r := range results {
			fmt.Printf("%s:%d:%s\n", r.File, r.Line, r.Text)
		}

	case "log-tail":
		n := cfg.LogLines
		if len(args) > 2 {
			n = config.ParsePositiveInt(args[2], cfg.LogLines)
		}
		lines, err := eng.LogTail(n)
		if err != nil {
			fatal(err)
		}
		if useJSON {
			writeJSON(lines)
			return
		}
		for _, l := range lines {
			fmt.Println(l)
		}

	case "changed":
		diff := cfg.DefaultDiff
		if len(args) > 2 {
			diff = args[2]
		}
		files, err := eng.Changed(diff)
		if err != nil {
			fatal(err)
		}
		if useJSON {
			writeJSON(files)
			return
		}
		for _, f := range files {
			fmt.Println(f)
		}

	case "candidates":
		diff := cfg.DefaultDiff
		if len(args) > 2 {
			diff = args[2]
		}
		files, err := eng.Candidates(diff)
		if err != nil {
			fatal(err)
		}
		if useJSON {
			writeJSON(files)
			return
		}
		for _, f := range files {
			fmt.Println(f)
		}

	case "lint":
		var check []string
		var skip []string
		for _, a := range args[2:] {
			if strings.HasPrefix(a, "--check=") {
				val := strings.TrimPrefix(a, "--check=")
				if val != "" {
					check = strings.Split(val, ",")
				}
			} else if strings.HasPrefix(a, "--skip=") {
				val := strings.TrimPrefix(a, "--skip=")
				if val != "" {
					skip = strings.Split(val, ",")
				}
			}
		}
		result := eng.LintWithOptions(check, skip)
		if useJSON {
			errMsg := ""
			if !result.OK {
				errMsg = "lint issues found"
			}
			writeJSONResult(result.Issues, result.OK, errMsg)
			if !result.OK {
				os.Exit(1)
			}
			return
		}
		if result.OK {
			if len(result.Messages) > 0 {
				// Info-only issues pass the gate but should still be visible.
				for _, m := range result.Messages {
					fmt.Fprintln(os.Stderr, m)
				}
				fmt.Println("wiki lint OK (info issues above)")
			} else {
				fmt.Println("wiki lint OK")
			}
		} else {
			for _, m := range result.Messages {
				fmt.Fprintln(os.Stderr, m)
			}
			os.Exit(1)
		}

	case "refresh":
		diff := cfg.DefaultDiff
		if len(args) > 2 {
			diff = args[2]
		}
		out, err := eng.Refresh(diff)
		if err != nil {
			fatal(err)
		}
		fmt.Print(out)

	case "stats":
		st, err := eng.Stats()
		if err != nil {
			fatal(err)
		}
		if useJSON {
			writeJSON(st)
			return
		}
		fmt.Printf("files: %d\n", st.Files)
		fmt.Printf("headings: %d\n", st.Headings)
		fmt.Printf("lines: %d\n", st.TotalLines)
		if st.LastUpdated != "" {
			fmt.Printf("last_updated: %s\n", st.LastUpdated)
		}

	case "context":
		minimal := false
		summarize := cfg.ContextSummarize
		active := false
		sortBy := "chrono"
		for _, a := range args[2:] {
			switch {
			case a == "--minimal":
				minimal = true
			case a == "--summarize":
				summarize = true
			case a == "--active":
				active = true
			case a == "--sort=topo":
				sortBy = "topo"
			case a == "--sort=chrono":
				sortBy = "chrono"
			}
		}

		if active {
			nodes, edges, err := eng.BuildWikiGraph()
			if err != nil {
				fatal(err)
			}
			engine.SortNodes(nodes, sortBy)

			unlinked, uerr := eng.ActiveUnlinkedPages()
			if uerr != nil {
				unlinked = nil
			}

			if useJSON {
				writeJSON(engine.WikiGraphJSON{Nodes: nodes, Edges: edges, Unlinked: unlinked})
				return
			}

			fmt.Println("== active wiki graph ==")
			for _, n := range nodes {
				fmt.Printf("%s [%s] | %s\n", n.File, n.Status, n.Description)
				for _, dest := range n.Links {
					fmt.Printf("  -> %s\n", dest)
				}
				fmt.Println()
			}
			if len(unlinked) > 0 {
				fmt.Println("== warning: active pages not linked from index.md ==")
				for _, u := range unlinked {
					fmt.Printf("  %s\n", u)
				}
			}
			return
		}

		cr, err := eng.Context(minimal, summarize, false)
		if err != nil {
			fatal(err)
		}
		if useJSON {
			writeJSON(cr)
			return
		}
		// Plain text snapshot format.
		fmt.Printf("== wiki status ==\n")
		fmt.Printf("files: %d\n", cr.Files)
		if cr.LastUpdated != "" {
			fmt.Printf("last_updated: %s\n", cr.LastUpdated)
		}
		if cr.Phase != "" {
			fmt.Printf("phase: %s\n", cr.Phase)
		}
		fmt.Println()
		fmt.Println("== catalog ==")
		for _, c := range cr.Catalog {
			fmt.Printf("%s [%s] | %s\n", c.File, c.Status, c.Description)
		}
		if len(cr.RecentLog) > 0 {
			fmt.Println()
			fmt.Println("== recent activity ==")
			for _, h := range cr.RecentLog {
				fmt.Println(h)
			}
		}
		if cr.Phase != "" {
			fmt.Println()
			fmt.Println("== active phase ==")
			fmt.Println(cr.Phase)
		}

	case "summary":
		if len(args) < 3 {
			fmt.Fprintln(os.Stderr, "usage: wiki-engine summary <page>")
			os.Exit(1)
		}
		page := args[2]
		sr, err := eng.Summary(page)
		if err != nil {
			fatal(err)
		}
		if useJSON {
			writeJSON(sr)
			return
		}
		fmt.Println(sr.FirstHeader)
		fmt.Println()
		fmt.Println(sr.FirstPara)

	case "relevant":
		if len(args) < 3 {
			fmt.Fprintln(os.Stderr, "usage: wiki-engine relevant <query> [topN]")
			os.Exit(1)
		}
		query := args[2]
		topN := 5
		if len(args) > 3 {
			topN = config.ParsePositiveInt(args[3], 5)
		}
		results, err := eng.Relevant(query, topN)
		if err != nil {
			fatal(err)
		}
		if useJSON {
			writeJSON(results)
			return
		}
		for _, r := range results {
			fmt.Printf("%.0f\t%s\t%s\n", r.Score, r.File, r.Why)
		}

	case "impact":
		// Read changed files from args or stdin.
		var changedFiles []string
		if len(args) > 2 {
			changedFiles = args[2:]
		} else {
			// If stdin is an interactive terminal, there is nothing to read —
			// show usage instead of blocking.
			if fi, err := os.Stdin.Stat(); err == nil && fi.Mode()&os.ModeCharDevice != 0 {
				fmt.Fprintln(os.Stderr, "usage: wiki-engine impact <file...>  (or pipe from wiki-engine changed)")
				os.Exit(1)
			}
			// Read from stdin (pipe from wiki-engine changed).
			scanner := bufio.NewScanner(os.Stdin)
			for scanner.Scan() {
				line := strings.TrimSpace(scanner.Text())
				if line != "" {
					changedFiles = append(changedFiles, line)
				}
			}
		}
		if len(changedFiles) == 0 {
			fmt.Fprintln(os.Stderr, "usage: wiki-engine impact <file...>  (or pipe from wiki-engine changed)")
			os.Exit(1)
		}
		results, err := eng.Impact(changedFiles)
		if err != nil {
			fatal(err)
		}
		if useJSON {
			writeJSON(results)
			return
		}
		for _, r := range results {
			if len(r.WikiPages) == 0 {
				fmt.Printf("%s → (no wiki pages mention this file)\n", r.ChangedFile)
			} else {
				fmt.Printf("%s → %s\n", r.ChangedFile, strings.Join(r.WikiPages, ", "))
			}
		}

	case "diff":
		if len(args) < 4 {
			fmt.Fprintln(os.Stderr, "usage: wiki-engine diff <from-ref> <to-ref>")
			os.Exit(1)
		}
		from, to := args[2], args[3]
		dr, err := eng.Diff(from, to)
		if err != nil {
			fatal(err)
		}
		if useJSON {
			writeJSON(dr)
			return
		}
		if len(dr.Added) > 0 {
			fmt.Println("== added ==")
			for _, f := range dr.Added {
				fmt.Println("+", f)
			}
		}
		if len(dr.Removed) > 0 {
			fmt.Println("== removed ==")
			for _, f := range dr.Removed {
				fmt.Println("-", f)
			}
		}
		if len(dr.Changed) > 0 {
			fmt.Println("== changed ==")
			for _, f := range dr.Changed {
				fmt.Println("~", f)
			}
		}
		if len(dr.Added)+len(dr.Removed)+len(dr.Changed) == 0 {
			fmt.Printf("no wiki changes between %s and %s\n", from, to)
		}

	case "watch":
		once := false
		for _, a := range args[2:] {
			if a == "--once" {
				once = true
			}
		}

		if once {
			runWatchCycle(eng, useJSON)
			return
		}

		interval := cfg.WatchInterval
		if interval <= 0 {
			fmt.Fprintln(os.Stderr, "watch_interval is 0 in .wikirc — continuous watch is disabled.")
			fmt.Fprintln(os.Stderr, "Set watch_interval to a positive number of seconds to enable, or run: wiki-engine watch --once")
			os.Exit(1)
		}

		// Continuous polling.
		fmt.Fprintf(os.Stderr, "watching wiki (interval: %ds, diff: %s)...\n", interval, cfg.DefaultDiff)
		ticker := time.NewTicker(time.Duration(interval) * time.Second)
		defer ticker.Stop()

		// Run first cycle immediately.
		runWatchCycle(eng, useJSON)

		for range ticker.C {
			runWatchCycle(eng, useJSON)
		}

	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", cmd)
		usage()
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, `wiki-engine — repo-local wiki management tool

Usage: wiki-engine [--json] <command> [arguments]

Commands:
  init [wiki-dir]         Scaffold a new wiki into the current repo
  sync-prompts            Update all tool instruction layers to the latest version
  list                    List all wiki files
  headings                List all Markdown headings with file paths
  search <query>          Case-insensitive search across wiki files
  log-tail [n]            Show the last N log headings
  changed [diff-range]    List non-wiki files changed in a git diff range
  candidates [diff-range] Filter changed files to ingest-worthy candidates
  stats                   Show aggregate wiki statistics
  context [--minimal] [--summarize] [--active] [--sort=topo|chrono]
                           Condensed wiki snapshot / active graph for agent context loading
  summary <page>          Show first heading and paragraph of a page
  relevant <query> [n]    Rank wiki pages by relevance to a query
  impact <file...>        Show which wiki pages mention changed files (or pipe from changed)
  lint [--check=<a,b>] [--skip=<a,b>]  Check wiki structure, links, and markers
  diff <from> <to>        Show wiki file changes between two git refs
  watch [--once]          Poll for changes and lint issues (interval from .wikirc)
  refresh [diff-range]    Run the full maintenance snapshot
  upgrade                 Self-upgrade to the latest version via go install
  version                 Print the version
  help                    Show this help

Add --json before the command for structured JSON output.`)
}

func fatal(err error) {
	fmt.Fprintf(os.Stderr, "error: %v\n", err)
	os.Exit(1)
}

func runWatchCycle(eng *engine.Engine, useJSON bool) {
	wr, err := eng.WatchOnce()
	if err != nil {
		fmt.Fprintf(os.Stderr, "watch error: %v\n", err)
		return
	}
	if useJSON {
		writeJSON(wr)
		return
	}
	if len(wr.Changed) == 0 {
		return // no changes, silent
	}
	fmt.Printf("\n=== [%s] ===\n", time.Now().Format("15:04:05"))
	fmt.Printf("changed: %d file(s)\n", len(wr.Changed))
	if len(wr.Candidates) > 0 {
		fmt.Printf("candidates: %d file(s)\n", len(wr.Candidates))
		for _, f := range wr.Candidates {
			fmt.Printf("  %s\n", f)
		}
	}
	if !wr.LintOK {
		fmt.Println("lint: ISSUES FOUND")
		for _, iss := range wr.LintIssues {
			fmt.Printf("  [%s] %s: %s\n", iss.Severity, iss.File, iss.Message)
		}
	}
}
