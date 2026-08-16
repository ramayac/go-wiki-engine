// Package audit verifies repo-wide wiki reference integrity as Go tests.
// Running `go test ./...` (and `make audit`) fails when wiki links, prose
// references, instruction layers, or the embedded scaffold drift. It exists
// because `wiki-engine lint` only checks markdown links inside the wiki
// directory — the gaps this package closes are wrong page-relative links,
// stale `wiki/<path>.md` references in README/prompt/scaffold files, and
// scaffold ↔ live instruction divergence.
package audit

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// repoRoot returns the repository root (two levels up from internal/audit).
func repoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd() // go test runs with the package dir as cwd
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Clean(filepath.Join(wd, "..", ".."))
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		t.Fatalf("repo root not found from %s: %v", wd, err)
	}
	return root
}

// markdown link target matcher. Group 1 is the target without anchor.
var linkTargetRe = regexp.MustCompile(`\]\(([^)#]+)(?:#[^)]*)?\)`)

// full link span matcher, used to strip links before prose scanning.
var linkSpanRe = regexp.MustCompile(`\[[^\]]*\]\([^)]*\)`)

// legacy flat-path references that the organized layout replaced.
var legacyPathRe = regexp.MustCompile(`wiki/(log|phases|schema|repo-map|todo|lessons|config|improvement-plan)\.md`)

// collectFiles walks root and returns regular files (paths relative to
// root), skipping directories whose first path element matches skipTop.
func collectFiles(root string, skipTop map[string]bool) ([]string, error) {
	var files []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if path != root {
				rel, rerr := filepath.Rel(root, path)
				if rerr == nil && skipTop[strings.SplitN(rel, string(filepath.Separator), 2)[0]] {
					return filepath.SkipDir
				}
			}
			return nil
		}
		rel, rerr := filepath.Rel(root, path)
		if rerr == nil {
			files = append(files, filepath.ToSlash(rel))
		}
		return nil
	})
	return files, err
}

// collectMDFiles walks root and returns .md files (paths relative to root),
// skipping directories whose first path element matches skipTop.
func collectMDFiles(root string, skipTop map[string]bool) ([]string, error) {
	all, err := collectFiles(root, skipTop)
	if err != nil {
		return nil, err
	}
	var md []string
	for _, f := range all {
		if strings.HasSuffix(f, ".md") {
			md = append(md, f)
		}
	}
	return md, nil
}

// wikiFileSet maps wiki-relative slash paths of every .md file in root/wiki.
func wikiFileSet(t *testing.T, root string) map[string]bool {
	t.Helper()
	set := make(map[string]bool)
	err := filepath.WalkDir(filepath.Join(root, "wiki"), func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		if strings.HasSuffix(path, ".md") {
			rel, rerr := filepath.Rel(filepath.Join(root, "wiki"), path)
			if rerr == nil {
				set[filepath.ToSlash(rel)] = true
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return set
}

// statusOf reads the front matter status of a wiki page; unknown defaults to "".
func statusOf(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(string(data), "\n") {
		m := regexp.MustCompile(`^\s*status:\s*(\S+)`).FindStringSubmatch(line)
		if m != nil {
			return m[1]
		}
	}
	return ""
}

// strictMDLinkTargets returns markdown link targets (without anchors) that
// point at other .md files. http/https/mailto/anchor-only links are skipped.
func strictMDLinkTargets(text string) []string {
	var targets []string
	for _, m := range linkTargetRe.FindAllStringSubmatch(text, -1) {
		target := strings.TrimSpace(m[1])
		if target == "" || strings.HasPrefix(target, "#") ||
			strings.HasPrefix(target, "http://") || strings.HasPrefix(target, "https://") ||
			strings.HasPrefix(target, "mailto:") {
			continue
		}
		if !strings.HasSuffix(target, ".md") {
			continue
		}
		targets = append(targets, target)
	}
	return targets
}

// TestWikiLinksResolveStrictly verifies every .md link in wiki/ resolves
// relative to the page's own directory — no wiki-root fallback.
func TestWikiLinksResolveStrictly(t *testing.T) {
	root := repoRoot(t)
	wikiRoot := filepath.Join(root, "wiki")
	set := wikiFileSet(t, root)

	files, err := collectMDFiles(wikiRoot, nil)
	if err != nil {
		t.Fatal(err)
	}
	var problems []string
	for _, rel := range files {
		data, err := os.ReadFile(filepath.Join(wikiRoot, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatal(err)
		}
		pageDir := filepath.Dir(filepath.Join(wikiRoot, filepath.FromSlash(rel)))
		for _, target := range strictMDLinkTargets(string(data)) {
			resolved := filepath.Clean(filepath.Join(pageDir, filepath.FromSlash(target)))
			wikiRel, rerr := filepath.Rel(wikiRoot, resolved)
			if rerr != nil || strings.HasPrefix(wikiRel, "..") {
				continue // link to a file outside the wiki — external-links checker domain
			}
			if !set[filepath.ToSlash(wikiRel)] {
				problems = append(problems, fmt.Sprintf("wiki/%s -> %s", rel, target))
			}
		}
	}
	if len(problems) > 0 {
		t.Errorf("broken page-relative wiki links:\n  %s", strings.Join(problems, "\n  "))
	}
}

// TestScaffoldWikiLinksResolveStrictly applies the same strict resolution to
// the scaffold templates.
func TestScaffoldWikiLinksResolveStrictly(t *testing.T) {
	root := repoRoot(t)
	scaffoldWiki := filepath.Join(root, "scaffold", "wiki")

	files, err := collectMDFiles(scaffoldWiki, nil)
	if err != nil {
		t.Fatal(err)
	}
	var problems []string
	for _, rel := range files {
		data, err := os.ReadFile(filepath.Join(scaffoldWiki, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatal(err)
		}
		pageDir := filepath.Dir(filepath.Join(scaffoldWiki, filepath.FromSlash(rel)))
		for _, target := range strictMDLinkTargets(string(data)) {
			resolved := filepath.Clean(filepath.Join(pageDir, filepath.FromSlash(target)))
			if _, err := os.Stat(resolved); err != nil {
				problems = append(problems, fmt.Sprintf("scaffold/wiki/%s -> %s", rel, target))
			}
		}
	}
	if len(problems) > 0 {
		t.Errorf("broken scaffold wiki links:\n  %s", strings.Join(problems, "\n  "))
	}
}

// TestWikiFrontMatter checks front matter, statuses, and superseded_by
// targets across all wiki pages.
func TestWikiFrontMatter(t *testing.T) {
	root := repoRoot(t)
	wikiRoot := filepath.Join(root, "wiki")
	set := wikiFileSet(t, root)

	files, err := collectMDFiles(wikiRoot, nil)
	if err != nil {
		t.Fatal(err)
	}
	valid := map[string]bool{"planned": true, "current": true, "legacy": true, "deprecated": true}
	var problems []string
	for _, rel := range files {
		abs := filepath.Join(wikiRoot, filepath.FromSlash(rel))
		data, err := os.ReadFile(abs)
		if err != nil {
			t.Fatal(err)
		}
		text := string(data)
		if !strings.HasPrefix(strings.TrimSpace(text), "---") {
			problems = append(problems, fmt.Sprintf("wiki/%s: missing front matter", rel))
			continue
		}
		status := statusOf(t, abs)
		if !valid[status] {
			problems = append(problems, fmt.Sprintf("wiki/%s: invalid or missing status %q", rel, status))
		}
		var superseded string
		for _, line := range strings.Split(text, "\n") {
			if m := regexp.MustCompile(`^\s*superseded_by:\s*"([^"]*)"`).FindStringSubmatch(line); m != nil {
				superseded = m[1]
				break // first occurrence — later examples in the body are irrelevant
			}
		}
		if status == "deprecated" && superseded == "" {
			problems = append(problems, fmt.Sprintf("wiki/%s: deprecated without superseded_by", rel))
		}
		if superseded != "" {
			target := filepath.Clean(filepath.Join(filepath.Dir(abs), filepath.FromSlash(superseded)))
			targetRel, rerr := filepath.Rel(wikiRoot, target)
			targetKey := filepath.ToSlash(targetRel)
			if rerr != nil || strings.HasPrefix(targetRel, "..") || !set[targetKey] {
				problems = append(problems, fmt.Sprintf("wiki/%s: superseded_by target missing: %s", rel, superseded))
			} else if st := statusOf(t, target); st != "current" && st != "planned" {
				problems = append(problems, fmt.Sprintf("wiki/%s: superseded_by target %s is not active (status %s)", rel, superseded, st))
			}
		}
	}
	if len(problems) > 0 {
		t.Errorf("front matter problems:\n  %s", strings.Join(problems, "\n  "))
	}
}

// TestWikiGraphConnectivity checks reachability from index.md, duplicate
// basenames, and active leaves.
func TestWikiGraphConnectivity(t *testing.T) {
	root := repoRoot(t)
	wikiRoot := filepath.Join(root, "wiki")
	set := wikiFileSet(t, root)

	files, err := collectMDFiles(wikiRoot, nil)
	if err != nil {
		t.Fatal(err)
	}
	graph := make(map[string][]string)
	for _, rel := range files {
		abs := filepath.Join(wikiRoot, filepath.FromSlash(rel))
		data, err := os.ReadFile(abs)
		if err != nil {
			t.Fatal(err)
		}
		pageDir := filepath.Dir(abs)
		for _, target := range strictMDLinkTargets(string(data)) {
			resolved := filepath.Clean(filepath.Join(pageDir, filepath.FromSlash(target)))
			rel2, rerr := filepath.Rel(wikiRoot, resolved)
			if rerr != nil || strings.HasPrefix(rel2, "..") {
				continue
			}
			graph[rel] = append(graph[rel], filepath.ToSlash(rel2))
		}
	}

	// BFS from index.md.
	visited := map[string]bool{"index.md": true}
	queue := []string{"index.md"}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, nxt := range graph[cur] {
			if !visited[nxt] && set[nxt] {
				visited[nxt] = true
				queue = append(queue, nxt)
			}
		}
	}

	canonicalLog := map[string]bool{"prologue/log.md": true, "log.md": true}
	basenames := map[string]int{}
	var problems []string
	for _, rel := range files {
		abs := filepath.Join(wikiRoot, filepath.FromSlash(rel))
		basenames[filepath.Base(rel)]++
		status := statusOf(t, abs)
		if status != "current" && status != "planned" {
			continue
		}
		if !visited[rel] {
			problems = append(problems, fmt.Sprintf("unreachable active page: %s", rel))
		}
		if len(graph[rel]) == 0 && !canonicalLog[rel] {
			problems = append(problems, fmt.Sprintf("active leaf page: %s", rel))
		}
	}
	for name, n := range basenames {
		if n > 1 {
			problems = append(problems, fmt.Sprintf("duplicate page basename %q (%d files)", name, n))
		}
	}
	if len(problems) > 0 {
		t.Errorf("graph problems:\n  %s", strings.Join(problems, "\n  "))
	}
}

// nonWikiMDFiles returns repo .md files outside wiki/, the embedded mirror,
// and test fixtures.
func nonWikiMDFiles(t *testing.T, root string) []string {
	t.Helper()
	files, err := collectMDFiles(root, map[string]bool{
		"wiki": true, "internal": true, "test": true, ".git": true,
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

// TestExternalMarkdownReferencesExist verifies every markdown link target in
// non-wiki .md files that points into wiki/ references a real wiki file.
func TestExternalMarkdownReferencesExist(t *testing.T) {
	root := repoRoot(t)
	set := wikiFileSet(t, root)

	var problems []string
	for _, rel := range nonWikiMDFiles(t, root) {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatal(err)
		}
		for _, target := range strictMDLinkTargets(string(data)) {
			idx := strings.Index(target, "wiki/")
			if idx == -1 {
				continue
			}
			wikiPath := strings.TrimPrefix(filepath.ToSlash(filepath.Clean(target[idx:])), "wiki/")
			// Placeholder used by the external-docs migration template in
			// wiki-maintainer.md — not a concrete reference.
			if wikiPath == "<name>.md" {
				continue
			}
			if !set[wikiPath] {
				problems = append(problems, fmt.Sprintf("%s -> %s", rel, target))
			}
		}
	}
	if len(problems) > 0 {
		t.Errorf("markdown references to missing wiki files:\n  %s", strings.Join(problems, "\n  "))
	}
}

// TestLegacyFlatPathReferencesAbsent scans prose in non-wiki .md files (link
// spans stripped) for references to the legacy flat canonical paths.
func TestLegacyFlatPathReferencesAbsent(t *testing.T) {
	root := repoRoot(t)

	var problems []string
	for _, rel := range nonWikiMDFiles(t, root) {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatal(err)
		}
		prose := linkSpanRe.ReplaceAllString(string(data), "")
		for _, m := range legacyPathRe.FindAllString(prose, -1) {
			problems = append(problems, fmt.Sprintf("%s: %s", rel, m))
		}
	}
	if len(problems) > 0 {
		t.Errorf("legacy flat-path references in non-wiki files:\n  %s", strings.Join(problems, "\n  "))
	}
}

// TestInstructionLayersIdentical ensures the live instruction files and the
// pi.dev skill stay identical to their scaffold copies (we edit both).
func TestInstructionLayersIdentical(t *testing.T) {
	root := repoRoot(t)
	files := []string{"ingest.md", "lint.md", "onboard.md", "query.md", "refresh.md", "upgrade.md", "watch.md", "wiki-maintainer.md"}
	var problems []string
	for _, f := range files {
		live := filepath.Join(root, ".wiki-instructions", f)
		scaffold := filepath.Join(root, "scaffold", ".wiki-instructions", f)
		a, err1 := os.ReadFile(live)
		b, err2 := os.ReadFile(scaffold)
		if err1 != nil || err2 != nil || string(a) != string(b) {
			problems = append(problems, ".wiki-instructions/"+f+" differs from scaffold copy")
		}
	}
	a, err1 := os.ReadFile(filepath.Join(root, ".pi", "skills", "wiki", "SKILL.md"))
	b, err2 := os.ReadFile(filepath.Join(root, "scaffold", ".pi", "skills", "wiki", "SKILL.md"))
	if err1 != nil || err2 != nil || string(a) != string(b) {
		problems = append(problems, ".pi/skills/wiki/SKILL.md differs from scaffold copy")
	}
	if len(problems) > 0 {
		t.Errorf("instruction layer drift:\n  %s", strings.Join(problems, "\n  "))
	}
}

// TestEmbeddedScaffoldSynced ensures internal/scaffold/files matches scaffold/
// (equivalent to the CI make sync-scaffold drift guard, but runs in go test).
func TestEmbeddedScaffoldSynced(t *testing.T) {
	root := repoRoot(t)

	read := func(base, rel string) (string, error) {
		data, err := os.ReadFile(filepath.Join(base, filepath.FromSlash(rel)))
		if err != nil {
			return "", err
		}
		return string(data), nil
	}

	scaffoldFiles, err := collectFiles(filepath.Join(root, "scaffold"), nil)
	if err != nil {
		t.Fatal(err)
	}
	embeddedFiles, err := collectFiles(filepath.Join(root, "internal", "scaffold", "files"), nil)
	if err != nil {
		t.Fatal(err)
	}

	sort.Strings(scaffoldFiles)
	sort.Strings(embeddedFiles)
	if strings.Join(scaffoldFiles, "\n") != strings.Join(embeddedFiles, "\n") {
		t.Errorf("scaffold/ and internal/scaffold/files file sets differ — run make sync-scaffold\nscaffold-only: %v\nembedded-only: %v",
			diffStrings(scaffoldFiles, embeddedFiles), diffStrings(embeddedFiles, scaffoldFiles))
	}
	for _, rel := range scaffoldFiles {
		a, err := read(filepath.Join(root, "scaffold"), rel)
		if err != nil {
			t.Fatalf("read scaffold/%s: %v", rel, err)
		}
		b, err := read(filepath.Join(root, "internal", "scaffold", "files"), rel)
		if err != nil {
			t.Fatalf("read internal/scaffold/files/%s: %v", rel, err)
		}
		if a != b {
			t.Errorf("content drift for %s — run make sync-scaffold", rel)
		}
	}
}

func diffStrings(a, b []string) []string {
	inB := make(map[string]bool, len(b))
	for _, s := range b {
		inB[s] = true
	}
	var out []string
	for _, s := range a {
		if !inB[s] {
			out = append(out, s)
		}
	}
	return out
}
