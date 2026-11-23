package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/ramayac/go-wiki-engine/internal/config"
	"github.com/ramayac/go-wiki-engine/internal/engine"
	"github.com/ramayac/go-wiki-engine/internal/scaffold"
	"github.com/ramayac/go-wiki-engine/internal/schema"
	"github.com/ramayac/go-wiki-engine/internal/upgrade"
)

// argSpec describes the accepted arguments for a command: exact flags,
// prefix flags (e.g. --check=), value flags that take a following argument
// (e.g. --base <ref>), and the maximum number of positional arguments (-1 for unlimited).
type argSpec struct {
	flags      map[string]bool
	prefixes   []string
	valueFlags map[string]bool
	maxPos     int
}

var commandArgSpecs = map[string]argSpec{
	"init": {
		prefixes:   []string{"--wiki-dir="},
		valueFlags: map[string]bool{"--wiki-dir": true},
		maxPos:     1,
	},
	"sync-prompts": {maxPos: 0},
	"schema": {
		prefixes:   []string{"--type=", "--section="},
		valueFlags: map[string]bool{"--type": true, "--section": true},
		maxPos:     0,
	},
	"version": {maxPos: 0},
	"upgrade": {
		flags:  map[string]bool{"--from-source": true, "--require-attestation": true},
		maxPos: 0,
	},
	"list":     {flags: map[string]bool{"--active": true}, maxPos: 0},
	"headings": {maxPos: 0},
	"search": {
		flags: map[string]bool{
			"--include-history": true,
			"--keyword":         true,
			"--ranked":          true,
		},
		prefixes:   []string{"--tag=", "--type=", "--lifecycle=", "--limit="},
		valueFlags: map[string]bool{"--tag": true, "--type": true, "--lifecycle": true, "--limit": true},
		maxPos:     -1,
	},
	"log-tail": {maxPos: 1},
	"changed": {
		flags:      map[string]bool{"--no-working-tree": true},
		prefixes:   []string{"--base="},
		valueFlags: map[string]bool{"--base": true},
		maxPos:     1,
	},
	"candidates": {
		flags:      map[string]bool{"--no-working-tree": true},
		prefixes:   []string{"--base="},
		valueFlags: map[string]bool{"--base": true},
		maxPos:     1,
	},
	"check": {
		flags:      map[string]bool{"--no-working-tree": true},
		prefixes:   []string{"--base="},
		valueFlags: map[string]bool{"--base": true},
		maxPos:     0,
	},
	"lint": {
		flags:    map[string]bool{"--fix": true},
		prefixes: []string{"--check=", "--skip="},
		maxPos:   0,
	},
	"refresh": {maxPos: 1},
	"stats":   {maxPos: 0},
	"health":  {maxPos: 0},
	"context": {
		flags: map[string]bool{
			"--minimal": true, "--summarize": true, "--active": true,
			"--sort=topo": true, "--sort=chrono": true,
		},
		prefixes:   []string{"--max-pages=", "--max-bytes="},
		valueFlags: map[string]bool{"--max-pages": true, "--max-bytes": true},
		maxPos:     0,
	},
	"graph": {
		flags:  map[string]bool{"--strict": true, "--dot": true, "--include-sources": true},
		maxPos: 1,
	},
	"summary": {maxPos: 1},
	"relevant": {
		flags:      map[string]bool{"--include-history": true},
		prefixes:   []string{"--tag=", "--type=", "--lifecycle=", "--limit="},
		valueFlags: map[string]bool{"--tag": true, "--type": true, "--lifecycle": true, "--limit": true},
		maxPos:     2,
	},
	"impact": {
		flags:  map[string]bool{"--fuzzy": true},
		maxPos: -1,
	},
	"diff":  {maxPos: 2},
	"watch": {flags: map[string]bool{"--once": true}, maxPos: 0},
}

// registeredCommands returns all registered command names sorted alphabetically
// (plus help). Used by parity tests to ensure usage() and docs stay in sync.
func registeredCommands() []string {
	cmds := make([]string, 0, len(commandArgSpecs)+1)
	for k := range commandArgSpecs {
		cmds = append(cmds, k)
	}
	cmds = append(cmds, "help")
	sort.Strings(cmds)
	return cmds
}

func argsAfterFilters() ([]string, bool) {
	var filtered []string
	useJSON := false
	afterTerminator := false
	for _, a := range os.Args {
		if !afterTerminator && a == "--" {
			afterTerminator = true
		}
		if !afterTerminator && a == "--json" {
			useJSON = true
			continue
		}
		filtered = append(filtered, a)
	}
	return filtered, useJSON
}

func positionalArgs(args []string) []string {
	var out []string
	for _, a := range args {
		if a != "--" {
			out = append(out, a)
		}
	}
	return out
}

func validateCommandArgs(cmd string, args []string) error {
	spec, ok := commandArgSpecs[cmd]
	if !ok {
		return nil
	}
	positional := 0
	afterTerminator := false
	for i := 0; i < len(args); i++ {
		a := args[i]
		if afterTerminator {
			positional++
			if spec.maxPos >= 0 && positional > spec.maxPos {
				return fmt.Errorf("unexpected argument %q for %s", a, cmd)
			}
			continue
		}
		if a == "--" {
			afterTerminator = true
			continue
		}
		if strings.HasPrefix(a, "--") {
			if spec.valueFlags[a] {
				if i+1 >= len(args) {
					return fmt.Errorf("flag %q requires a value for %s", a, cmd)
				}
				i++ // consume value token
				continue
			}
			allowed := spec.flags[a]
			if !allowed {
				for _, p := range spec.prefixes {
					if strings.HasPrefix(a, p) {
						allowed = true
						break
					}
				}
			}
			if !allowed {
				return fmt.Errorf("unknown flag %q for %s", a, cmd)
			}
			continue
		}
		positional++
		if spec.maxPos >= 0 && positional > spec.maxPos {
			return fmt.Errorf("unexpected argument %q for %s", a, cmd)
		}
	}
	return nil
}

func validateLintSelectors(check, skip []string) error {
	return engine.ValidateLintSelectors(check, skip)
}

func runInit(args []string, useJSON bool) {
	dir, err := os.Getwd()
	if err != nil {
		fatal(err)
	}
	var wikiDir string
	var renameTo string
	for i := 2; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			continue
		}
		if strings.HasPrefix(a, "--wiki-dir=") {
			renameTo = strings.TrimPrefix(a, "--wiki-dir=")
			continue
		}
		if a == "--wiki-dir" && i+1 < len(args) {
			i++
			renameTo = args[i]
			continue
		}
		wikiDir = a
	}

	res, err := scaffold.Reconcile(dir, scaffold.ReconcileOptions{
		WikiDir:  wikiDir,
		RenameTo: renameTo,
	})
	if err != nil {
		fatal(err)
	}
	if useJSON {
		writeJSON(res)
		return
	}
	switch res.Mode {
	case "rename":
		fmt.Fprintf(os.Stderr, "renamed wiki directory %s/ → %s/ and reconciled .wikirc, prompts, and shims\n", res.RenamedFrom, res.WikiDir)
	case "reconcile":
		fmt.Fprintf(os.Stderr, "reconciled %s/: %d created, %d updated, %d removed, %d preserved\n",
			res.WikiDir, len(res.Created), len(res.Updated), len(res.Removed), len(res.Preserved))
	default:
		fmt.Fprintf(os.Stderr, "initialized %s/ with wiki scaffold, .wikirc, prompts, instructions, AGENTS.md/CLAUDE.md shims, .claude/commands/, and .pi/skills/\n", res.WikiDir)
		fmt.Fprintln(os.Stderr, "next steps:")
		fmt.Fprintln(os.Stderr, "  1. Edit .wikirc to set your ignore patterns")
		fmt.Fprintf(os.Stderr, "  2. Edit %s/prologue/repo-map.md with your project's architecture\n", res.WikiDir)
		fmt.Fprintln(os.Stderr, "  3. Run: wiki-engine lint")
	}
}

func runSyncPrompts(useJSON bool) {
	dir, err := os.Getwd()
	if err != nil {
		fatal(err)
	}
	cfg, err := config.Load(dir)
	if err != nil {
		fatal(err)
	}

	var preExistingShims []string
	for _, name := range []string{"AGENTS.md", "CLAUDE.md"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
			preExistingShims = append(preExistingShims, name)
		}
	}

	updated, removed, err := scaffold.SyncPrompts(dir)
	if err != nil {
		fatal(err)
	}
	for _, f := range updated {
		_, _ = fmt.Fprintf(os.Stderr, "updated %s\n", f)
	}
	for _, f := range removed {
		_, _ = fmt.Fprintf(os.Stderr, "removed %s\n", f)
	}
	_, _ = fmt.Fprintf(os.Stderr, "sync-prompts: %d updated, %d removed\n", len(updated), len(removed))

	if len(preExistingShims) > 0 {
		_, _ = fmt.Fprintf(os.Stderr, "\ntip: %s already exist and custom content was preserved.\n", strings.Join(preExistingShims, " and "))
		_, _ = fmt.Fprintf(os.Stderr, "     Run `wiki-engine init` to reconcile fenced wiki pointers against %s/index.md.\n", cfg.WikiDir)
	}

	if useJSON {
		writeJSON(map[string]interface{}{
			"updated":         nonNil(updated),
			"removed":         nonNil(removed),
			"shims_preserved": nonNil(preExistingShims),
		})
	}
}

func runSchema(args []string, useJSON bool) {
	var typeName, section string
	for i := 2; i < len(args); i++ {
		a := args[i]
		switch {
		case strings.HasPrefix(a, "--type="):
			typeName = strings.TrimPrefix(a, "--type=")
		case a == "--type" && i+1 < len(args):
			i++
			typeName = args[i]
		case strings.HasPrefix(a, "--section="):
			section = strings.TrimPrefix(a, "--section=")
		case a == "--section" && i+1 < len(args):
			i++
			section = args[i]
		}
	}

	if useJSON {
		if typeName != "" {
			ts, err := schema.TypeByName(typeName)
			if err != nil {
				usageFatal(err)
			}
			writeJSON(ts)
			return
		}
		if section != "" {
			text, err := schema.RenderText("", section)
			if err != nil {
				usageFatal(err)
			}
			writeJSON(map[string]string{
				"schema_version": schema.SchemaVersion,
				"section":        section,
				"content":        text,
			})
			return
		}
		writeJSON(schema.Get())
		return
	}

	out, err := schema.RenderText(typeName, section)
	if err != nil {
		usageFatal(err)
	}
	fmt.Print(out)
}

func runUpgrade(args []string, useJSON bool) {
	opts := upgrade.Options{CurrentVersion: getVersion()}
	for _, a := range args[2:] {
		switch a {
		case "--from-source":
			opts.FromSource = true
		case "--require-attestation":
			opts.RequireAttestation = true
		}
	}
	report, err := upgrade.RunWithOptions(opts)
	if err != nil {
		fatal(err)
	}
	if useJSON {
		writeJSON(report)
	}
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
	if err := validateCommandArgs(cmd, args[2:]); err != nil {
		usageFatal(err)
	}
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
				fm, found := eng.PageFrontMatter(f)
				if found && fm.IsActive() {
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
		opts, pos, ranked := parseSearchArgs(args[2:])
		query := strings.Join(pos, " ")
		if strings.TrimSpace(query) == "" {
			usageError("usage: wiki-engine search <query>")
		}
		if ranked && !opts.Keyword {
			results, err := eng.SearchRanked(query, opts)
			if err != nil {
				fatal(err)
			}
			if useJSON {
				writeJSON(results)
				return
			}
			for _, r := range results {
				fmt.Printf("%.1f\t%s\t[%s]\t%s\n", r.Score, r.File, r.Lifecycle, r.Why)
			}
			return
		}
		results, err := eng.SearchLines(query, opts)
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
		if pos := positionalArgs(args[2:]); len(pos) > 0 {
			n = config.ParsePositiveInt(pos[0], cfg.LogLines)
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
		opts := parseChangeArgs(args[2:], cfg.DefaultDiff)
		files, err := eng.ChangedWithOptions(opts)
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
		opts := parseChangeArgs(args[2:], cfg.DefaultDiff)
		files, err := eng.CandidatesWithOptions(opts)
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

	case "check":
		checkOpts := engine.CheckOptions{IncludeWorkingTree: true}
		for i := 2; i < len(args); i++ {
			a := args[i]
			switch {
			case a == "--no-working-tree":
				checkOpts.IncludeWorkingTree = false
			case strings.HasPrefix(a, "--base="):
				checkOpts.Base = strings.TrimPrefix(a, "--base=")
			case a == "--base" && i+1 < len(args):
				i++
				checkOpts.Base = args[i]
			}
		}
		res, err := eng.Check(checkOpts)
		if err != nil {
			if errors.Is(err, engine.ErrCheckSetup) {
				fatalWithCode(err, 2)
			}
			fatal(err)
		}
		if useJSON {
			errMsg := ""
			if res.HasBreakage() {
				errMsg = fmt.Sprintf("%d broken citation(s) introduced on branch", len(res.Broken))
			}
			writeJSONResult(res, !res.HasBreakage(), errMsg)
			if res.HasBreakage() {
				os.Exit(1)
			}
			return
		}
		fmt.Printf("base: %s (merge-base: %s, working-tree: %t)\n", res.Base, res.MergeBase, res.WorkingTree)
		if len(res.Broken) > 0 {
			fmt.Println("== broken ==")
			for _, b := range res.Broken {
				fmt.Printf("  ! %s [%s]: %s\n", b.Page, b.Kind, b.Message)
			}
		}
		if len(res.Warnings) > 0 {
			fmt.Println("== warnings ==")
			for _, w := range res.Warnings {
				fmt.Printf("  ? %s [%s]: %s\n", w.Page, w.Kind, w.Message)
			}
		}
		if len(res.PagesToReview) > 0 {
			fmt.Println("== pages to review ==")
			for _, r := range res.PagesToReview {
				fmt.Printf("  ~ %s (%s) <- %s\n", r.Page, strings.Join(r.MatchReasons, ","), strings.Join(r.ChangedFiles, ", "))
			}
		}
		if !res.HasBreakage() && len(res.PagesToReview) == 0 && len(res.Warnings) == 0 {
			fmt.Println("wiki check OK")
		}
		if res.HasBreakage() {
			os.Exit(1)
		}

	case "lint":
		var check []string
		var skip []string
		doFix := false
		for _, a := range args[2:] {
			if a == "--fix" {
				doFix = true
			} else if strings.HasPrefix(a, "--check=") {
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
		if err := validateLintSelectors(check, skip); err != nil {
			usageFatal(err)
		}
		if doFix {
			fixRes, err := eng.LintFix()
			if err != nil {
				fatal(err)
			}
			if useJSON {
				writeJSON(fixRes)
				return
			}
			if fixRes.TotalFixes == 0 {
				fmt.Println("wiki lint --fix: no mechanical fixes needed")
				return
			}
			for _, f := range fixRes.Files {
				fmt.Printf("%s: %d fix(es) (%s)\n", f.File, f.Count, strings.Join(f.Fixes, "; "))
			}
			fmt.Printf("wiki lint --fix: applied %d fix(es) across %d file(s)\n", fixRes.TotalFixes, len(fixRes.Files))
			return
		}

		result := eng.LintWithOptions(check, skip)
		if useJSON {
			errMsg := ""
			if !result.OK {
				errMsg = "lint issues found"
				if result.Reason != "" {
					errMsg = result.Reason
				}
			}
			writeJSONResult(result.Issues, result.OK, errMsg)
			if !result.OK {
				os.Exit(1)
			}
			return
		}
		if result.OK {
			if len(result.Messages) > 0 {
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
		if pos := positionalArgs(args[2:]); len(pos) > 0 {
			diff = pos[0]
		}
		res, err := eng.RefreshSnapshot(diff)
		if err != nil {
			fatal(err)
		}
		if useJSON {
			errMsg := ""
			if !res.Lint.OK {
				errMsg = fmt.Sprintf("refresh lint failed: %d issue(s)", len(res.Lint.Issues))
			}
			writeJSONResult(res, res.Lint.OK, errMsg)
		} else {
			fmt.Print(res.Text())
		}
		if !res.Lint.OK {
			os.Exit(exitFailure)
		}

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

	case "health":
		res, err := eng.Health()
		if err != nil {
			fatal(err)
		}
		if useJSON {
			writeJSON(res)
			return
		}
		fmt.Print(res.Text())

	case "context":
		minimal := false
		summarize := cfg.ContextSummarize
		explicitSummarize := false
		active := false
		sortBy := "chrono"
		explicitSort := false
		maxPages := cfg.ContextMaxPages
		maxBytes := cfg.ContextMaxBytes
		for i := 2; i < len(args); i++ {
			a := args[i]
			switch {
			case a == "--minimal":
				minimal = true
			case a == "--summarize":
				summarize = true
				explicitSummarize = true
			case a == "--active":
				active = true
			case a == "--sort=topo":
				sortBy = "topo"
				explicitSort = true
			case a == "--sort=chrono":
				sortBy = "chrono"
				explicitSort = true
			case strings.HasPrefix(a, "--max-pages="):
				maxPages = config.ParsePositiveInt(strings.TrimPrefix(a, "--max-pages="), maxPages)
			case a == "--max-pages" && i+1 < len(args):
				i++
				maxPages = config.ParsePositiveInt(args[i], maxPages)
			case strings.HasPrefix(a, "--max-bytes="):
				maxBytes = config.ParsePositiveInt(strings.TrimPrefix(a, "--max-bytes="), maxBytes)
			case a == "--max-bytes" && i+1 < len(args):
				i++
				maxBytes = config.ParsePositiveInt(args[i], maxBytes)
			}
		}

		if active {
			if explicitSummarize {
				usageFatal(fmt.Errorf("--summarize cannot be combined with --active; run wiki-engine context --summarize for page previews"))
			}
			if minimal {
				usageFatal(fmt.Errorf("--minimal cannot be combined with --active"))
			}
			view, err := eng.BuildGraphView()
			if err != nil {
				fatal(err)
			}
			engine.SortNodes(view.Nodes, sortBy)

			if useJSON {
				writeJSON(view)
				return
			}

			fmt.Println("== active wiki graph ==")
			for _, n := range view.Nodes {
				fmt.Printf("%s [%s] | %s\n", n.File, n.Status, n.Description)
				for _, dest := range n.Links {
					fmt.Printf("  -> %s\n", dest)
				}
				fmt.Println()
			}
			if len(view.Unlinked) > 0 {
				fmt.Println("== warning: active pages unreachable from index.md ==")
				for _, u := range view.Unlinked {
					fmt.Printf("  %s\n", u)
				}
			}
			return
		}

		if explicitSort {
			usageFatal(fmt.Errorf("--sort=topo|chrono requires --active (it orders the active wiki graph)"))
		}

		cr, err := eng.ContextWithOptions(engine.ContextOptions{
			Minimal:   minimal,
			Summarize: summarize,
			MaxPages:  maxPages,
			MaxBytes:  maxBytes,
		})
		if err != nil {
			fatal(err)
		}
		if useJSON {
			writeJSON(cr)
			return
		}
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
			if cr.Summarized && c.Summary != "" {
				fmt.Printf("    %s\n", strings.ReplaceAll(c.Summary, "\n", "\n    "))
				fmt.Printf("    (lines: %d)\n", c.LineCount)
			}
		}
		if cr.Truncated {
			fmt.Printf("\n(truncated: %s)\n", cr.TruncatedReason)
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

	case "graph":
		strict := false
		dot := false
		includeSources := false
		var posArgs []string
		afterTerminator := false
		for _, a := range args[2:] {
			if a == "--" {
				afterTerminator = true
				continue
			}
			if !afterTerminator {
				switch a {
				case "--strict":
					strict = true
					continue
				case "--dot":
					dot = true
					continue
				case "--include-sources":
					includeSources = true
					continue
				}
			}
			posArgs = append(posArgs, a)
		}
		page := ""
		if len(posArgs) > 0 {
			page = posArgs[0]
		}
		if dot {
			if page != "" {
				usageFatal(fmt.Errorf("--dot cannot be combined with a page argument; run wiki-engine graph --dot or wiki-engine graph <page>"))
			}
			if useJSON {
				usageFatal(fmt.Errorf("--dot cannot be combined with --json"))
			}
		}

		view, err := eng.BuildGraphViewWithOptions(engine.GraphOptions{IncludeSources: includeSources})
		if err != nil {
			fatal(err)
		}
		unhealthy := len(view.Unlinked) > 0 || len(view.Issues) > 0

		var nv *engine.NodeView
		if page != "" {
			nv, err = engine.NeighborhoodForWiki(view.Nodes, view.Edges, cfg.WikiDir, page)
			if err != nil {
				fatal(err)
			}
		}

		if useJSON {
			var data interface{} = view
			if nv != nil {
				data = nv
			}
			if strict && unhealthy {
				writeJSONResult(data, false, fmt.Sprintf("graph strict check failed: %d unlinked page(s), %d graph issue(s)", len(view.Unlinked), len(view.Issues)))
				os.Exit(exitFailure)
			}
			writeJSON(data)
			return
		}

		if dot {
			fmt.Print(engine.RenderDot(view.Nodes, view.Edges))
			if unhealthy {
				fmt.Fprintf(os.Stderr, "warning: %d unlinked page(s), %d graph issue(s); run wiki-engine graph for details\n", len(view.Unlinked), len(view.Issues))
			}
			if strict && unhealthy {
				os.Exit(1)
			}
			return
		}

		if nv != nil {
			fmt.Printf("%s [%s] | %s\n", nv.Node.File, nv.Node.Status, nv.Node.Description)
			fmt.Println()
			fmt.Println("== backlinks ==")
			for _, b := range nv.Backlinks {
				fmt.Printf("  <- %s\n", b)
			}
			fmt.Println()
			fmt.Println("== links ==")
			for _, l := range nv.Node.Links {
				fmt.Printf("  -> %s\n", l)
			}
			if len(nv.Node.Sources) > 0 {
				fmt.Println()
				fmt.Println("== sources ==")
				for _, s := range nv.Node.Sources {
					if s.Repo != "" {
						fmt.Printf("  %s (%s)\n", s.File, s.Repo)
					} else {
						fmt.Printf("  %s\n", s.File)
					}
				}
			}
			if len(nv.Node.References) > 0 {
				fmt.Println()
				fmt.Println("== references ==")
				for _, r := range nv.Node.References {
					fmt.Printf("  %s: %s\n", r.Type, r.Value)
				}
			}
			if strict && unhealthy {
				os.Exit(1)
			}
			return
		}

		fmt.Println("== wiki graph ==")
		fmt.Print(engine.RenderTree(view.Nodes))
		fmt.Println()
		fmt.Printf("stats: %d nodes, %d edges, max depth %d\n", view.Stats.NodeCount, view.Stats.EdgeCount, view.Stats.MaxDepth)
		if len(view.Stats.Roots) > 1 {
			fmt.Printf("roots: %s\n", strings.Join(view.Stats.Roots, ", "))
		}
		if len(view.Unlinked) > 0 {
			fmt.Println()
			fmt.Println("== warning: active pages unreachable from index.md ==")
			for _, u := range view.Unlinked {
				fmt.Printf("  %s\n", u)
			}
		}
		if len(view.Issues) > 0 {
			fmt.Println()
			fmt.Println("== graph issues ==")
			for _, i := range view.Issues {
				fmt.Printf("  ! %s\n", i)
			}
		}
		if strict && unhealthy {
			os.Exit(1)
		}

	case "summary":
		pos := positionalArgs(args[2:])
		if len(pos) < 1 {
			usageError("usage: wiki-engine summary <page>")
		}
		page := pos[0]
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
		opts, pos, _ := parseSearchArgs(args[2:])
		if len(pos) < 1 {
			usageError("usage: wiki-engine relevant <query> [topN]")
		}
		query := pos[0]
		if opts.Limit <= 0 {
			opts.Limit = 5
		}
		if len(pos) > 1 {
			opts.Limit = config.ParsePositiveInt(pos[1], opts.Limit)
		}
		results, err := eng.SearchRanked(query, opts)
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
		fuzzy := false
		var changedFiles []string
		afterTerminator := false
		for _, a := range args[2:] {
			if a == "--" && !afterTerminator {
				afterTerminator = true
				continue
			}
			if !afterTerminator && a == "--fuzzy" {
				fuzzy = true
				continue
			}
			changedFiles = append(changedFiles, a)
		}
		if len(changedFiles) == 0 {
			if fi, err := os.Stdin.Stat(); err == nil && fi.Mode()&os.ModeCharDevice != 0 {
				usageError("usage: wiki-engine impact [--fuzzy] <file...>  (or pipe from wiki-engine changed)")
			}
			scanner := bufio.NewScanner(os.Stdin)
			for scanner.Scan() {
				line := strings.TrimSpace(scanner.Text())
				if line != "" {
					changedFiles = append(changedFiles, line)
				}
			}
			if err := scanner.Err(); err != nil {
				fatal(err)
			}
		}
		if len(changedFiles) == 0 {
			usageError("usage: wiki-engine impact [--fuzzy] <file...>  (or pipe from wiki-engine changed)")
		}
		results, err := eng.ImpactWithOptions(changedFiles, engine.ImpactOptions{Fuzzy: fuzzy})
		if err != nil {
			fatal(err)
		}
		if useJSON {
			writeJSON(results)
			return
		}
		for _, r := range results {
			if len(r.WikiPages) == 0 {
				fmt.Printf("%s → (no wiki pages cite this file)\n", r.ChangedFile)
			} else {
				fmt.Printf("%s → %s\n", r.ChangedFile, strings.Join(r.WikiPages, ", "))
			}
		}

	case "diff":
		pos := positionalArgs(args[2:])
		if len(pos) < 2 {
			usageError("usage: wiki-engine diff <from-ref> <to-ref>")
		}
		from, to := pos[0], pos[1]
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
			if runWatchCycle(eng, useJSON) {
				os.Exit(1)
			}
			return
		}
		interval := cfg.WatchInterval
		if interval <= 0 {
			usageError("watch_interval is 0 in .wikirc — continuous watch is disabled. Set watch_interval to a positive number of seconds to enable, or run: wiki-engine watch --once")
		}
		fmt.Fprintf(os.Stderr, "watching wiki (interval: %ds, diff: %s)...\n", interval, cfg.DefaultDiff)
		ticker := time.NewTicker(time.Duration(interval) * time.Second)
		defer ticker.Stop()

		watchLoop(eng, useJSON, ticker.C)

	default:
		unknownCommand(cmd)
	}
}

func parseSearchArgs(args []string) (opts engine.SearchOptions, pos []string, ranked bool) {
	afterTerminator := false
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" && !afterTerminator {
			afterTerminator = true
			continue
		}
		if !afterTerminator && strings.HasPrefix(a, "--") {
			switch {
			case a == "--include-history":
				opts.IncludeHistory = true
			case a == "--keyword":
				opts.Keyword = true
			case a == "--ranked":
				ranked = true
			case strings.HasPrefix(a, "--tag="):
				opts.Tag = strings.TrimPrefix(a, "--tag=")
			case a == "--tag" && i+1 < len(args):
				i++
				opts.Tag = args[i]
			case strings.HasPrefix(a, "--type="):
				opts.Type = strings.TrimPrefix(a, "--type=")
			case a == "--type" && i+1 < len(args):
				i++
				opts.Type = args[i]
			case strings.HasPrefix(a, "--lifecycle="):
				opts.Lifecycle = strings.TrimPrefix(a, "--lifecycle=")
			case a == "--lifecycle" && i+1 < len(args):
				i++
				opts.Lifecycle = args[i]
			case strings.HasPrefix(a, "--limit="):
				opts.Limit = config.ParsePositiveInt(strings.TrimPrefix(a, "--limit="), 0)
			case a == "--limit" && i+1 < len(args):
				i++
				opts.Limit = config.ParsePositiveInt(args[i], 0)
			}
			continue
		}
		pos = append(pos, a)
	}
	return opts, pos, ranked
}

func parseChangeArgs(args []string, defaultDiff string) engine.ChangeOptions {
	opts := engine.ChangeOptions{
		DiffRange:          defaultDiff,
		IncludeWorkingTree: true,
	}
	afterTerminator := false
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case afterTerminator:
			opts.DiffRange = a
		case a == "--":
			afterTerminator = true
		case a == "--no-working-tree":
			opts.IncludeWorkingTree = false
		case strings.HasPrefix(a, "--base="):
			opts.Base = strings.TrimPrefix(a, "--base=")
			opts.DiffRange = ""
		case a == "--base" && i+1 < len(args):
			i++
			opts.Base = args[i]
			opts.DiffRange = ""
		default:
			opts.DiffRange = a
		}
	}
	return opts
}

func runWatchCycle(eng *engine.Engine, useJSON bool) bool {
	return watchCycle(eng, useJSON, nil)
}

// watchLoop runs one cycle at once, then one per tick until ticks closes.
// It prints only when the result differs from the last printed result.
func watchLoop(eng *engine.Engine, useJSON bool, ticks <-chan time.Time) {
	var last string
	watchCycle(eng, useJSON, &last)
	for range ticks {
		watchCycle(eng, useJSON, &last)
	}
}

// watchCycle runs one watch cycle and reports whether lint failed. When last
// is not nil, it skips printing a result equal to the one stored in last.
func watchCycle(eng *engine.Engine, useJSON bool, last *string) bool {
	wr, err := eng.WatchOnce()
	if err != nil {
		if shouldPrintWatch(last, "error: "+err.Error()) {
			if useJSON {
				writeJSONResult(nil, false, err.Error())
			} else {
				fmt.Fprintf(os.Stderr, "watch error: %v\n", err)
			}
		}
		return true
	}
	fingerprint, _ := json.Marshal(wr)
	if !shouldPrintWatch(last, string(fingerprint)) {
		return !wr.LintOK
	}
	if useJSON {
		errMsg := ""
		if !wr.LintOK {
			errMsg = fmt.Sprintf("watch lint failed: %d issue(s)", len(wr.LintIssues))
		}
		writeJSONResult(wr, wr.LintOK, errMsg)
		return !wr.LintOK
	}
	if len(wr.Changed) > 0 {
		fmt.Printf("\n=== [%s] ===\n", time.Now().Format("15:04:05"))
		fmt.Printf("changed: %d file(s)\n", len(wr.Changed))
		if len(wr.Candidates) > 0 {
			fmt.Printf("candidates: %d file(s)\n", len(wr.Candidates))
			for _, f := range wr.Candidates {
				fmt.Printf("  %s\n", f)
			}
		}
	}
	if !wr.LintOK {
		fmt.Println("lint: ISSUES FOUND")
		for _, iss := range wr.LintIssues {
			fmt.Printf("  [%s] %s: %s\n", iss.Severity, iss.File, iss.Message)
		}
	}
	return !wr.LintOK
}

// shouldPrintWatch reports whether a watch result is new. A nil last always prints.
func shouldPrintWatch(last *string, fingerprint string) bool {
	if last == nil {
		return true
	}
	if *last == fingerprint {
		return false
	}
	*last = fingerprint
	return true
}
