package engine

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var healthNow = time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)

// hpage builds a wiki page with frontmatter.
func hpage(id, lifecycle, updated, body string) string {
	upd := ""
	if updated != "" {
		upd = "updated: \"" + updated + "\"\n"
	}
	return fmt.Sprintf("---\nid: %s\ntitle: %s\ntype: concept\nlifecycle: %s\ndescription: %s\n%s---\n# %s\n\n%s\n", id, "Title "+id, lifecycle, id, upd, id, body)
}

// healthWiki writes an index that links every page, plus the given pages.
// Pages that need other links carry them in their own body.
func healthWiki(t *testing.T, pages map[string]string) string {
	t.Helper()
	root := t.TempDir()
	var names []string
	for rel := range pages {
		names = append(names, rel)
	}
	index := "---\nid: index\ntitle: Wiki Index\nlifecycle: current\ndescription: Index\n---\n# Index\n\n"
	for _, rel := range names {
		index += fmt.Sprintf("- [%s](%s)\n", rel, rel)
	}
	files := map[string]string{"wiki/index.md": index}
	for rel, content := range pages {
		files["wiki/"+rel] = content
	}
	writeTestWiki(t, root, files)
	return root
}

func healthOf(t *testing.T, root string) *HealthResult {
	t.Helper()
	res, err := newTestEngine(root).healthAt(healthNow)
	if err != nil {
		t.Fatalf("health failed: %v", err)
	}
	return res
}

func TestHealthCleanWiki(t *testing.T) {
	root := healthWiki(t, map[string]string{
		"a.md": hpage("a", "current", "2026-05-01", "See [b](b.md)."),
		"b.md": hpage("b", "current", "2026-05-01", "See [a](a.md)."),
	})
	res := healthOf(t, root)
	if res.SchemaVersion != "1" {
		t.Errorf("SchemaVersion = %q, want 1", res.SchemaVersion)
	}
	if res.Pages != 3 || res.WikiBytes == 0 || res.IndexBytes == 0 {
		t.Errorf("counts wrong: %+v", res)
	}
	if res.LogBytes != 0 || res.LogNeedsRotation {
		t.Errorf("no log: bytes=%d rotate=%v", res.LogBytes, res.LogNeedsRotation)
	}
	if res.Lifecycles["current"] != 3 || res.Lifecycles["wip"] != 0 {
		t.Errorf("Lifecycles = %v", res.Lifecycles)
	}
	if res.GraphNodes != 3 || res.GraphEdges == 0 {
		t.Errorf("graph counts: nodes=%d edges=%d", res.GraphNodes, res.GraphEdges)
	}
	if len(res.AgingWork)+len(res.UpdatedBehindGit)+len(res.DuplicateTitles)+len(res.IndexOnlyPages)+len(res.OpenMarkers) != 0 {
		t.Errorf("clean wiki must carry no signals: %+v", res)
	}
	if len(res.LargestPages) != 3 {
		t.Errorf("LargestPages = %d, want 3", len(res.LargestPages))
	}
}

func TestHealthLargestPagesTopFive(t *testing.T) {
	pages := map[string]string{}
	for i := 0; i < 7; i++ {
		id := fmt.Sprintf("p%d", i)
		pages[id+".md"] = hpage(id, "current", "", strings.Repeat("word ", 10*(i+1)))
	}
	res := healthOf(t, healthWiki(t, pages))
	if len(res.LargestPages) != 5 {
		t.Fatalf("LargestPages = %d, want 5", len(res.LargestPages))
	}
	if res.LargestPages[0].Path != "wiki/p6.md" || res.LargestPages[0].Words < 70 {
		t.Errorf("largest first: %+v", res.LargestPages[0])
	}
	for i := 1; i < 5; i++ {
		if res.LargestPages[i].Bytes > res.LargestPages[i-1].Bytes {
			t.Errorf("LargestPages not sorted by bytes: %+v", res.LargestPages)
		}
	}
}

func TestHealthLogRotationThreshold(t *testing.T) {
	logPage := func(size int) string {
		head := "---\nid: log\nlifecycle: current\ndescription: Log\n---\n# Log\n\n"
		return head + strings.Repeat("x", size-len(head))
	}
	for _, tc := range []struct {
		name string
		rel  string
		size int
		want bool
	}{
		{"below threshold", "prologue/log.md", healthLogRotateBytes - 1, false},
		{"at threshold", "prologue/log.md", healthLogRotateBytes, true},
		{"root fallback path", "log.md", healthLogRotateBytes + 10, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := healthWiki(t, map[string]string{tc.rel: logPage(tc.size)})
			res := healthOf(t, root)
			if res.LogBytes != tc.size {
				t.Errorf("LogBytes = %d, want %d", res.LogBytes, tc.size)
			}
			if res.LogNeedsRotation != tc.want {
				t.Errorf("LogNeedsRotation = %v, want %v", res.LogNeedsRotation, tc.want)
			}
		})
	}
}

func TestHealthAgingWork(t *testing.T) {
	root := healthWiki(t, map[string]string{
		"old-planned.md": hpage("old-planned", "planned", "2026-01-01", "Intended."),     // 151 days
		"old-wip.md":     hpage("old-wip", "wip", "2026-02-01", "In progress."),          // 120 days
		"new-wip.md":     hpage("new-wip", "wip", "2026-05-01", "In progress."),          // 31 days
		"edge.md":        hpage("edge", "planned", "2026-03-03", "Exactly 90 days old."), // 90 days: not over
		"old-current.md": hpage("old-current", "current", "2025-01-01", "Fine."),
		"no-date.md":     hpage("no-date", "wip", "", "No updated field."),
	})
	res := healthOf(t, root)
	if len(res.AgingWork) != 2 {
		t.Fatalf("AgingWork = %+v, want 2 entries", res.AgingWork)
	}
	if res.AgingWork[0].Path != "wiki/old-planned.md" || res.AgingWork[0].AgeDays != 151 {
		t.Errorf("first aging entry = %+v, want old-planned 151 days", res.AgingWork[0])
	}
	if res.AgingWork[1].Path != "wiki/old-wip.md" || res.AgingWork[1].AgeDays != 120 {
		t.Errorf("second aging entry = %+v", res.AgingWork[1])
	}
}

func TestHealthDuplicateTitles(t *testing.T) {
	two := func(id, title string) string {
		return strings.Replace(hpage(id, "current", "", "Body."), "title: Title "+id, "title: "+title, 1)
	}
	root := healthWiki(t, map[string]string{
		"a.md": two("a", "Auth Flow"),
		"b.md": two("b", "auth  flow!"),
		"c.md": two("c", "Other"),
	})
	res := healthOf(t, root)
	if len(res.DuplicateTitles) != 1 {
		t.Fatalf("DuplicateTitles = %+v, want 1 group", res.DuplicateTitles)
	}
	d := res.DuplicateTitles[0]
	if d.Title != "auth flow" || strings.Join(d.Paths, ",") != "wiki/a.md,wiki/b.md" {
		t.Errorf("duplicate group = %+v", d)
	}
}

func TestHealthIndexOnlyPages(t *testing.T) {
	root := healthWiki(t, map[string]string{
		"hub.md":    hpage("hub", "current", "", "See [linked](linked.md)."),
		"linked.md": hpage("linked", "current", "", "Leaf."),
		"lonely.md": hpage("lonely", "current", "", "Nobody links here."),
		"old.md":    "---\nid: old\ntitle: Old\nlifecycle: legacy\ndescription: Old\n---\n# Old\n",
	})
	res := healthOf(t, root)
	want := "wiki/hub.md,wiki/lonely.md"
	if got := strings.Join(res.IndexOnlyPages, ","); got != want {
		t.Errorf("IndexOnlyPages = %q, want %q", got, want)
	}
}

func TestHealthOpenMarkersLiveVersusHistorical(t *testing.T) {
	marked := "> Rationale: not recorded.\n\n> ⚠️ CONTRADICTION: A says X.\n\n> ⚠️ CONTRADICTION: B says Y.\n"
	pages := map[string]string{
		"cur.md":  hpage("cur", "current", "", marked),
		"imp.md":  hpage("imp", "implemented", "", "> Rationale: not recorded.\n"),
		"wip.md":  hpage("wip", "wip", "", "> ⚠️ CONTRADICTION: C.\n"),
		"plan.md": hpage("plan", "planned", "", "> Rationale: not recorded.\n"),
		"leg.md":  hpage("leg", "legacy", "", marked),
		"sup.md":  hpage("sup", "superseded", "", marked),
		"ex.md":   hpage("ex", "current", "", "Example: `> Rationale: not recorded.`\n\n```\n> ⚠️ CONTRADICTION: x\n```\n"),
	}
	res := healthOf(t, healthWiki(t, pages))
	got := map[string][2]int{}
	for _, m := range res.OpenMarkers {
		got[m.Path] = [2]int{m.Contradictions, m.RationaleNotRecorded}
	}
	want := map[string][2]int{
		"wiki/cur.md":  {2, 1},
		"wiki/imp.md":  {0, 1},
		"wiki/wip.md":  {1, 0},
		"wiki/plan.md": {0, 1},
	}
	if len(got) != len(want) {
		t.Fatalf("OpenMarkers = %v, want %v", got, want)
	}
	for p, w := range want {
		if got[p] != w {
			t.Errorf("%s markers = %v, want %v", p, got[p], w)
		}
	}
	for i := 1; i < len(res.OpenMarkers); i++ {
		if res.OpenMarkers[i-1].Path > res.OpenMarkers[i].Path {
			t.Errorf("OpenMarkers not sorted: %+v", res.OpenMarkers)
		}
	}
}

func commitAt(t *testing.T, root, date, msg string) {
	t.Helper()
	gitIn(t, root, "add", ".")
	cmd := exec.Command("git", "commit", "-q", "-m", msg)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "GIT_COMMITTER_DATE="+date+"T12:00:00Z", "GIT_AUTHOR_DATE="+date+"T12:00:00Z")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git commit failed: %v\n%s", err, out)
	}
}

func gitRepoOf(t *testing.T, root string) {
	t.Helper()
	gitIn(t, root, "init", "-q", "-b", "main")
	gitIn(t, root, "config", "user.email", "test@test")
	gitIn(t, root, "config", "user.name", "Test")
}

func driftFixture(t *testing.T) string {
	t.Helper()
	root := healthWiki(t, map[string]string{
		"late.md":   hpage("late", "current", "2026-01-01", "See [fresh](fresh.md)."),
		"fresh.md":  hpage("fresh", "current", "2026-01-28", "See [late](late.md)."),
		"frozen.md": hpage("frozen", "implemented", "2026-01-01", "Shipped. See [late](late.md)."),
		"edge.md":   hpage("edge", "current", "2026-01-25", "See [late](late.md)."),
	})
	gitRepoOf(t, root)
	commitAt(t, root, "2026-02-01", "pages")
	return root
}

func TestHealthUpdatedBehindGit(t *testing.T) {
	res := healthOf(t, driftFixture(t))
	if len(res.UpdatedBehindGit) != 1 {
		t.Fatalf("UpdatedBehindGit = %+v, want only late.md", res.UpdatedBehindGit)
	}
	d := res.UpdatedBehindGit[0]
	if d.Path != "wiki/late.md" || d.Updated != "2026-01-01" || d.LastCommit != "2026-02-01" || d.DriftDays != 31 {
		t.Errorf("drift entry = %+v", d)
	}
}

func TestHealthDriftSkippedOutsideGitAndInShallowClone(t *testing.T) {
	// Not a git repo: no drift data, no error.
	root := healthWiki(t, map[string]string{
		"late.md": hpage("late", "current", "2026-01-01", "Body."),
	})
	if res := healthOf(t, root); len(res.UpdatedBehindGit) != 0 {
		t.Errorf("non-git wiki must report no drift: %+v", res.UpdatedBehindGit)
	}

	// Shallow clone: truncated history, so drift is skipped.
	src := driftFixture(t)
	clone := filepath.Join(t.TempDir(), "clone")
	cmd := exec.Command("git", "clone", "-q", "--depth=1", "file://"+src, clone)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("shallow clone failed: %v\n%s", err, out)
	}
	if got := gitIn(t, clone, "rev-parse", "--is-shallow-repository"); got != "true" {
		t.Skipf("clone is not shallow (%s)", got)
	}
	if res := healthOf(t, clone); len(res.UpdatedBehindGit) != 0 {
		t.Errorf("shallow clone must report no drift: %+v", res.UpdatedBehindGit)
	}
}

func TestHealthJSONShape(t *testing.T) {
	res := healthOf(t, healthWiki(t, map[string]string{
		"a.md": hpage("a", "current", "2026-05-01", "See [b](b.md)."),
		"b.md": hpage("b", "current", "2026-05-01", "See [a](a.md)."),
	}))
	data, err := json.Marshal(res)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{
		"schema_version", "pages", "wiki_bytes", "index_bytes", "log_bytes", "log_needs_rotation",
		"graph_nodes", "graph_edges", "lifecycles", "largest_pages", "aging_work",
		"updated_behind_git", "duplicate_titles", "index_only_pages", "open_markers",
	} {
		if _, ok := m[k]; !ok {
			t.Errorf("health JSON missing key %q:\n%s", k, data)
		}
	}
	if string(m["schema_version"]) != `"1"` {
		t.Errorf("schema_version = %s, want \"1\"", m["schema_version"])
	}
	for _, k := range []string{"aging_work", "updated_behind_git", "duplicate_titles", "index_only_pages", "open_markers"} {
		if string(m[k]) != "[]" {
			t.Errorf("%s = %s, want []", k, m[k])
		}
	}
	for _, k := range []string{"current", "wip", "implemented", "planned", "legacy", "superseded", "unset"} {
		var lc map[string]int
		_ = json.Unmarshal(m["lifecycles"], &lc)
		if _, ok := lc[k]; !ok {
			t.Errorf("lifecycles missing key %q: %s", k, m["lifecycles"])
		}
	}

	// Age and drift fields are present even when zero.
	aging, _ := json.Marshal(HealthAging{})
	drift, _ := json.Marshal(HealthDrift{})
	for _, k := range []string{`"age_days":0`, `"updated":""`} {
		if !strings.Contains(string(aging), k) {
			t.Errorf("aging entry JSON lacks %s: %s", k, aging)
		}
	}
	if !strings.Contains(string(drift), `"drift_days":0`) {
		t.Errorf("drift entry JSON lacks drift_days: %s", drift)
	}
}

func TestHealthTextOutput(t *testing.T) {
	root := healthWiki(t, map[string]string{
		"old.md": hpage("old", "planned", "2026-01-01", "> Rationale: not recorded."),
	})
	text := healthOf(t, root).Text()
	for _, want := range []string{
		"Wiki health: 2 pages",
		"Lifecycle: current=1",
		"Planned/WIP pages older than 90 days: 1",
		"Live pages linked only from index.md: 1",
		"Pages with open markers: 1",
		"1 rationale not recorded",
		"Largest pages:",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("health text missing %q:\n%s", want, text)
		}
	}
}

func TestHealthRequiresWiki(t *testing.T) {
	if _, err := newTestEngine(t.TempDir()).Health(); err == nil {
		t.Error("health without a wiki directory must return an error")
	}
}
