// Package audit verifies repo-wide wiki reference integrity, supply-chain
// pinning, schema/docs parity, and prompt/scaffold synchronization as Go tests.
package audit

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/ramayac/go-wiki-engine/internal/model"
	"github.com/ramayac/go-wiki-engine/internal/schema"
)

func repoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Clean(filepath.Join(wd, "..", ".."))
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		t.Fatalf("repo root not found from %s: %v", wd, err)
	}
	return root
}

var (
	linkTargetRe = regexp.MustCompile(`\]\(([^)#]+)(?:#[^)]*)?\)`)
	linkSpanRe   = regexp.MustCompile(`\[[^\]]*\]\([^)]*\)`)
	legacyPathRe = regexp.MustCompile(`wiki/(log|phases|schema|repo-map|todo|lessons|config|improvement-plan)\.md`)
)

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
				continue
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

// TestScaffoldWikiLinksResolveStrictly applies strict resolution to scaffold templates.
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

// TestWikiFrontMatter checks YAML frontmatter, lifecycle, and superseded_by
// targets across all wiki pages using the real YAML parser.
func TestWikiFrontMatter(t *testing.T) {
	root := repoRoot(t)
	snap, err := model.LoadSnapshot(root, "wiki")
	if err != nil {
		t.Fatal(err)
	}
	var problems []string
	for _, p := range snap.Pages {
		if p.FMErr != nil {
			problems = append(problems, fmt.Sprintf("%s: malformed front matter: %v", p.Path, p.FMErr))
			continue
		}
		if p.FMState != model.FrontMatterPresent {
			problems = append(problems, fmt.Sprintf("%s: missing front matter", p.Path))
			continue
		}
		lc := p.EffectiveLifecycle()
		if !schema.IsValidLifecycle(lc) {
			problems = append(problems, fmt.Sprintf("%s: invalid or missing lifecycle %q", p.Path, lc))
		}
		if lc == "superseded" && p.Metadata.SupersededBy == "" {
			problems = append(problems, fmt.Sprintf("%s: superseded without superseded_by", p.Path))
		}
		if p.Metadata.SupersededBy != "" {
			pageDir := filepath.ToSlash(filepath.Dir(p.WikiRel))
			if pageDir == "." {
				pageDir = ""
			}
			cand := filepath.ToSlash(filepath.Clean(filepath.Join(pageDir, p.Metadata.SupersededBy)))
			target := snap.ByPath[cand]
			if target == nil {
				target, _ = snap.Lookup(p.Metadata.SupersededBy)
			}
			if target == nil {
				problems = append(problems, fmt.Sprintf("%s: superseded_by target missing: %s", p.Path, p.Metadata.SupersededBy))
			} else if !target.IsActive() {
				problems = append(problems, fmt.Sprintf("%s: superseded_by target %s is not active (lifecycle %s)", p.Path, p.Metadata.SupersededBy, target.EffectiveLifecycle()))
			}
		}
	}
	if len(problems) > 0 {
		t.Errorf("front matter problems:\n  %s", strings.Join(problems, "\n  "))
	}
}

// TestWikiGraphConnectivity checks reachability from index.md, duplicate IDs,
// and active leaves.
func TestWikiGraphConnectivity(t *testing.T) {
	root := repoRoot(t)
	snap, err := model.LoadSnapshot(root, "wiki")
	if err != nil {
		t.Fatal(err)
	}
	view, err := model.BuildGraph(snap, model.GraphOptions{})
	if err != nil {
		t.Fatal(err)
	}

	var problems []string
	for _, u := range view.Unlinked {
		problems = append(problems, fmt.Sprintf("unreachable active page: %s", u))
	}
	for _, iss := range view.Issues {
		problems = append(problems, fmt.Sprintf("graph issue: %s", iss))
	}
	for id, pages := range snap.ByIDMulti {
		if len(pages) > 1 {
			problems = append(problems, fmt.Sprintf("duplicate page id %q (%d files)", id, len(pages)))
		}
	}
	for _, p := range snap.Pages {
		if !p.IsActive() || model.IsCanonicalFile("log.md", p.WikiRel) {
			continue
		}
		if len(p.InternalMDLinks()) == 0 && len(p.Metadata.Related) == 0 && len(p.Metadata.DependsOn) == 0 {
			problems = append(problems, fmt.Sprintf("active leaf page: %s", p.WikiRel))
		}
	}
	if len(problems) > 0 {
		t.Errorf("graph problems:\n  %s", strings.Join(problems, "\n  "))
	}
}

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

func TestInstructionLayersIdentical(t *testing.T) {
	root := repoRoot(t)
	files := []string{"ingest.md", "plan.md", "query.md", "save.md", "seed.md", "upgrade.md", "wiki-maintainer.md"}
	var problems []string
	for _, f := range files {
		live := filepath.Join(root, ".wiki-instructions", f)
		scaf := filepath.Join(root, "scaffold", ".wiki-instructions", f)
		a, err1 := os.ReadFile(live)
		b, err2 := os.ReadFile(scaf)
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

// TestBenchmarkSkillParity keeps the dogfooded .claude/skills/wiki-benchmark/
// layer in step with the scaffold copy. Every file must match byte for byte,
// except questions.md, which a repo owner edits after install. The scaffold
// questions.md must stay a template, and prepare.sh must stay executable.
func TestBenchmarkSkillParity(t *testing.T) {
	root := repoRoot(t)
	const skill = ".claude/skills/wiki-benchmark"
	const userOwned = "questions.md"
	const marker = "<!-- TEMPLATE: replace these example questions -->"
	scafDir := filepath.Join(root, "scaffold", filepath.FromSlash(skill))
	liveDir := filepath.Join(root, filepath.FromSlash(skill))

	scafFiles, err := collectFiles(scafDir, nil)
	if err != nil {
		t.Fatal(err)
	}
	liveFiles, err := collectFiles(liveDir, nil)
	if err != nil {
		t.Fatalf("root skill layer missing: %v", err)
	}
	sort.Strings(scafFiles)
	sort.Strings(liveFiles)
	if len(scafFiles) < 6 {
		t.Fatalf("scaffold skill has %d files, expected SKILL.md, prepare.sh, questions.md and the headless files: %v", len(scafFiles), scafFiles)
	}
	if strings.Join(scafFiles, "\n") != strings.Join(liveFiles, "\n") {
		t.Errorf("%s file set differs from scaffold\nscaffold-only: %v\nroot-only: %v",
			skill, diffStrings(scafFiles, liveFiles), diffStrings(liveFiles, scafFiles))
	}
	for _, rel := range scafFiles {
		a, err := os.ReadFile(filepath.Join(scafDir, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatal(err)
		}
		b, err := os.ReadFile(filepath.Join(liveDir, filepath.FromSlash(rel)))
		if err != nil {
			continue // already reported by the file-set check
		}
		if rel != userOwned && string(a) != string(b) {
			t.Errorf("%s/%s differs from scaffold copy", skill, rel)
		}
	}

	for _, rel := range []string{userOwned, "SKILL.md"} {
		data, err := os.ReadFile(filepath.Join(scafDir, rel))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(data), marker) {
			t.Errorf("scaffold %s/%s must carry the template marker", skill, rel)
		}
	}

	if runtime.GOOS != "windows" {
		for _, dir := range []string{scafDir, liveDir} {
			fi, err := os.Stat(filepath.Join(dir, "prepare.sh"))
			if err != nil {
				t.Fatal(err)
			}
			if fi.Mode()&0o100 == 0 {
				t.Errorf("%s/prepare.sh is not executable (mode %s)", dir, fi.Mode())
			}
		}
	}
}

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

// TestGitHubActionsPinnedBySHA enforces the Phase 7.1 supply-chain policy:
// every external GitHub Action in .github/workflows/*.yml must be pinned to a
// 40-character commit SHA with a `# v...` version comment, and every workflow
// must declare minimal permissions.
func TestGitHubActionsPinnedBySHA(t *testing.T) {
	root := repoRoot(t)
	wfDir := filepath.Join(root, ".github", "workflows")
	entries, err := os.ReadDir(wfDir)
	if err != nil {
		t.Fatalf("read workflows dir: %v", err)
	}
	usesRe := regexp.MustCompile(`uses:\s*([^\s#]+)(.*)$`)
	shaPinnedRe := regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+(?:/[A-Za-z0-9_.-]+)*@[0-9a-f]{40}$`)
	commentVersionRe := regexp.MustCompile(`#\s*v\d+`)

	for _, e := range entries {
		if e.IsDir() || (!strings.HasSuffix(e.Name(), ".yml") && !strings.HasSuffix(e.Name(), ".yaml")) {
			continue
		}
		path := filepath.Join(wfDir, e.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		text := string(data)
		if !strings.Contains(text, "permissions:") {
			t.Errorf("%s: missing explicit minimal permissions block", e.Name())
		}
		for i, line := range strings.Split(text, "\n") {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "#") {
				continue
			}
			m := usesRe.FindStringSubmatch(trimmed)
			if m == nil {
				continue
			}
			ref := m[1]
			comment := m[2]
			if strings.HasPrefix(ref, "./") {
				continue // local composite action
			}
			if !shaPinnedRe.MatchString(ref) {
				t.Errorf("%s:%d: action %q must be pinned to a 40-character immutable commit SHA", e.Name(), i+1, ref)
			}
			if !commentVersionRe.MatchString(comment) {
				t.Errorf("%s:%d: pinned action %q should keep a '# vX' version comment", e.Name(), i+1, ref)
			}
		}
	}
}

// TestSchemaParityWithDocs enforces Phase 8.3: the Go schema vocabulary equals
// the documented schema in both wiki/prologue/schema.md and scaffold/wiki/prologue/schema.md.
func TestSchemaParityWithDocs(t *testing.T) {
	root := repoRoot(t)
	doc := schema.Get()
	targets := []string{
		filepath.Join(root, "wiki", "prologue", "schema.md"),
		filepath.Join(root, "scaffold", "wiki", "prologue", "schema.md"),
	}
	for _, path := range targets {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		text := string(data)
		for _, typ := range schema.ValidPageTypes {
			if !strings.Contains(text, "`"+typ+"`") {
				t.Errorf("%s missing page type %q", path, typ)
			}
		}
		for _, lc := range schema.ValidLifecycles {
			if !strings.Contains(text, "`"+lc+"`") {
				t.Errorf("%s missing lifecycle value %q", path, lc)
			}
		}
		for _, st := range doc.DecisionStatuses {
			if !strings.Contains(text, st) {
				t.Errorf("%s missing decision status %q", path, st)
			}
		}
		for _, st := range doc.IncidentStatuses {
			if !strings.Contains(text, st) {
				t.Errorf("%s missing incident status %q", path, st)
			}
		}
	}
}

// TestSchemaDocsCarryConventions keeps the human-readable schema docs in step
// with `wiki-engine schema --section conventions` and the page templates.
func TestSchemaDocsCarryConventions(t *testing.T) {
	root := repoRoot(t)
	for _, rel := range []string{"wiki/prologue/schema.md", "scaffold/wiki/prologue/schema.md"} {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatal(err)
		}
		text := string(data)
		for _, want := range []string{
			"## Conventions",
			"`> Rationale: not recorded.`",
			"`> ⚠️ CONTRADICTION: ",
			"ASD-STE100 Simplified Technical English",
			"secrets, credentials, tokens, session IDs, personal or health identifiers, free-text personal data, or whole request or response payloads",
			"## Why",
			"## Stops applying when",
			"log_needs_rotation",
		} {
			if !strings.Contains(text, want) {
				t.Errorf("%s missing %q", rel, want)
			}
		}
	}
}

// TestWikiCommandDocsParity verifies that wiki/prologue/repo-map.md documents
// every public CLI subcommand.
func TestWikiCommandDocsParity(t *testing.T) {
	root := repoRoot(t)
	data, err := os.ReadFile(filepath.Join(root, "wiki", "prologue", "repo-map.md"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	commands := []string{
		"init", "sync-prompts", "schema", "list", "headings", "search",
		"log-tail", "changed", "candidates", "check", "lint", "stats",
		"context", "graph", "summary", "relevant", "impact", "diff",
		"health", "watch", "refresh", "upgrade", "version",
	}
	for _, cmd := range commands {
		if !strings.Contains(text, "`"+cmd) {
			t.Errorf("wiki/prologue/repo-map.md missing command documentation for %q", cmd)
		}
	}
}

// TestPromptLifecycleRulesParity ensures canonical agent workflows teach the
// Phase 8.2 lifecycle, source citation, lint, and check rules.
func TestPromptLifecycleRulesParity(t *testing.T) {
	root := repoRoot(t)
	checks := map[string][]string{
		".wiki-instructions/wiki-maintainer.md": {"lifecycle", "planned", "superseded_by", "sources:", "wiki-engine lint", "wiki-engine check"},
		".wiki-instructions/ingest.md":          {"context --active", "lifecycle", "superseded_by", "sources:", "wiki-engine lint", "wiki-engine check"},
		".wiki-instructions/query.md":           {"lifecycle", "planned", "superseded_by", "wiki-engine check"},
		".wiki-instructions/seed.md":            {"lifecycle", "sources:", "## Open questions", "wiki-engine lint", "wiki-engine check", "wiki-engine health"},
		".wiki-instructions/save.md":            {"lifecycle", "superseded_by", "sources:", "wiki-engine lint", "wiki-engine check", "wiki-engine health"},
		".wiki-instructions/plan.md":            {".wiki-engine/handoff.md"},
		".pi/skills/wiki/SKILL.md":              {"lifecycle", "superseded_by", "sources:", "wiki-engine check", "wiki-engine schema"},
	}
	for rel, wants := range checks {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatalf("read %s: %v", rel, err)
		}
		text := string(data)
		for _, want := range wants {
			if !strings.Contains(text, want) {
				t.Errorf("%s missing required lifecycle/workflow guidance %q", rel, want)
			}
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
