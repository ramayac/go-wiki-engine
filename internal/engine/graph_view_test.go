package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeTestWiki creates a wiki directory tree from a map of relative paths
// to file contents, mirroring the fixture style of TestBuildWikiGraph.
func writeTestWiki(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for rel, content := range files {
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// graphViewFixture is a diamond-shaped wiki with a cycle:
// index -> a, index -> b, a -> c, b -> c, c -> index.
func graphViewFixture(t *testing.T) *Engine {
	t.Helper()
	root := t.TempDir()
	writeTestWiki(t, root, map[string]string{
		"wiki/index.md": `---
status: current
description: Index
---
# Index
- [a.md](a.md)
- [b.md](b.md)
`,
		"wiki/a.md": `---
status: current
description: A
---
# A
- [c.md](c.md)
`,
		"wiki/b.md": `---
status: current
description: B
---
# B
- [c.md](c.md)
`,
		"wiki/c.md": `---
status: current
description: C
---
# C
- [index.md](index.md)
`,
	})
	return newTestEngine(root)
}

func TestValidateGraph(t *testing.T) {
	eng := graphViewFixture(t)
	wikiDir := eng.WikiPath()
	nodes, edges, err := eng.BuildWikiGraph()
	if err != nil {
		t.Fatalf("BuildWikiGraph failed: %v", err)
	}

	// 1. A clean graph produces no issues.
	if issues := ValidateGraph(nodes, edges, wikiDir); len(issues) != 0 {
		t.Errorf("expected no issues for clean graph, got: %v", issues)
	}

	// 2. Self-loop detection.
	selfLoop := []WikiEdge{
		{From: "a.md", To: "a.md"},
	}
	issues := ValidateGraph(nodes, selfLoop, wikiDir)
	found := false
	for _, i := range issues {
		if strings.Contains(i, "self-loop") && strings.Contains(i, "a.md") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected self-loop issue, got: %v", issues)
	}

	// 3. Duplicate edge detection.
	dupEdges := []WikiEdge{
		{From: "a.md", To: "c.md"},
		{From: "a.md", To: "c.md"},
	}
	issues = ValidateGraph(nodes, dupEdges, wikiDir)
	found = false
	for _, i := range issues {
		if strings.Contains(i, "duplicate edge") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected duplicate edge issue, got: %v", issues)
	}

	// 4. Broken edge detection (target file does not exist on disk).
	brokenEdges := []WikiEdge{
		{From: "a.md", To: "missing.md"},
	}
	issues = ValidateGraph(nodes, brokenEdges, wikiDir)
	found = false
	for _, i := range issues {
		if strings.Contains(i, "broken edge") && strings.Contains(i, "missing.md") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected broken edge issue, got: %v", issues)
	}

	// 5. Duplicate node guard (defense in depth; impossible via BFS).
	dupNodes := append([]WikiNode{}, nodes...)
	dupNodes = append(dupNodes, nodes[0])
	issues = ValidateGraph(dupNodes, edges, wikiDir)
	found = false
	for _, i := range issues {
		if strings.Contains(i, "duplicate node") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected duplicate node issue, got: %v", issues)
	}
}

func TestComputeStats(t *testing.T) {
	nodes := []WikiNode{
		{File: "index.md", Depth: 0, Links: []string{"a.md", "b.md"}},
		{File: "a.md", Depth: 1, Links: []string{"leaf.md"}},
		{File: "b.md", Depth: 1, Links: []string{}},
		{File: "leaf.md", Depth: 2, Links: []string{}},
	}
	edges := []WikiEdge{
		{From: "index.md", To: "a.md"},
		{From: "index.md", To: "b.md"},
		{From: "a.md", To: "leaf.md"},
	}

	st := ComputeStats(nodes, edges)
	if st.NodeCount != 4 {
		t.Errorf("NodeCount = %d, want 4", st.NodeCount)
	}
	if st.EdgeCount != 3 {
		t.Errorf("EdgeCount = %d, want 3", st.EdgeCount)
	}
	if st.MaxDepth != 2 {
		t.Errorf("MaxDepth = %d, want 2", st.MaxDepth)
	}
	if len(st.Roots) != 1 || st.Roots[0] != "index.md" {
		t.Errorf("Roots = %v, want [index.md]", st.Roots)
	}
	if len(st.Leaves) != 2 || st.Leaves[0] != "b.md" || st.Leaves[1] != "leaf.md" {
		t.Errorf("Leaves = %v, want [b.md leaf.md]", st.Leaves)
	}
}

func TestRenderTree(t *testing.T) {
	eng := graphViewFixture(t)
	nodes, _, err := eng.BuildWikiGraph()
	if err != nil {
		t.Fatalf("BuildWikiGraph failed: %v", err)
	}
	SortNodes(nodes, "topo")

	tree := RenderTree(nodes)

	// Root is printed without a branch prefix.
	if !strings.HasPrefix(tree, "index.md [current] | Index\n") {
		t.Errorf("tree does not start with root line:\n%s", tree)
	}
	// Children appear under the root.
	if !strings.Contains(tree, "├── a.md") || !strings.Contains(tree, "└── b.md") {
		t.Errorf("tree missing root children:\n%s", tree)
	}
	// The diamond target c.md appears exactly once as a full node...
	if strings.Count(tree, "c.md [current]") != 1 {
		t.Errorf("c.md must appear exactly once as a full node:\n%s", tree)
	}
	// ...and the second reference is a back-reference marker.
	if !strings.Contains(tree, "↰ c.md (see above)") {
		t.Errorf("tree missing back-reference marker for c.md:\n%s", tree)
	}
	// The cycle c -> index must not hang: rendered as a marker too.
	if !strings.Contains(tree, "↰ index.md (see above)") {
		t.Errorf("tree missing back-reference marker for index.md:\n%s", tree)
	}
}

func TestRenderTreeEmpty(t *testing.T) {
	if got := RenderTree(nil); got != "" {
		t.Errorf("RenderTree(nil) = %q, want empty", got)
	}
}

func TestNeighborhood(t *testing.T) {
	eng := graphViewFixture(t)
	nodes, edges, err := eng.BuildWikiGraph()
	if err != nil {
		t.Fatalf("BuildWikiGraph failed: %v", err)
	}

	nv, err := Neighborhood(nodes, edges, "c.md")
	if err != nil {
		t.Fatalf("Neighborhood failed: %v", err)
	}
	if nv.Node.File != "c.md" {
		t.Errorf("node = %s, want c.md", nv.Node.File)
	}
	wantBacklinks := []string{"a.md", "b.md"}
	if len(nv.Backlinks) != 2 || nv.Backlinks[0] != wantBacklinks[0] || nv.Backlinks[1] != wantBacklinks[1] {
		t.Errorf("backlinks = %v, want %v", nv.Backlinks, wantBacklinks)
	}
	if len(nv.Node.Links) != 1 || nv.Node.Links[0] != "index.md" {
		t.Errorf("links = %v, want [index.md]", nv.Node.Links)
	}

	// Backlinks are sorted.
	eng2 := graphViewFixture(t)
	nodes2, edges2, err := eng2.BuildWikiGraph()
	if err != nil {
		t.Fatalf("BuildWikiGraph failed: %v", err)
	}
	// Same fixture builds edges in the same BFS order, so this exercises the
	// deterministic sort path.
	nv2, err := Neighborhood(nodes2, edges2, "c.md")
	if err != nil {
		t.Fatalf("Neighborhood failed: %v", err)
	}
	if len(nv2.Backlinks) != 2 || nv2.Backlinks[0] != "a.md" || nv2.Backlinks[1] != "b.md" {
		t.Errorf("backlinks = %v, want sorted [a.md b.md]", nv2.Backlinks)
	}

	// Unknown page errors.
	if _, err := Neighborhood(nodes, edges, "nope.md"); err == nil {
		t.Error("expected error for unknown page")
	}
}

func TestBuildGraphView(t *testing.T) {
	root := t.TempDir()
	writeTestWiki(t, root, map[string]string{
		"wiki/index.md": `---
status: current
description: Index
---
# Index
- [linked.md](linked.md)
`,
		"wiki/linked.md": `---
status: current
description: Linked
---
# Linked
`,
		// orphan.md is active but not reachable from index.md.
		"wiki/orphan.md": `---
status: current
description: Orphan
---
# Orphan
`,
	})

	eng := newTestEngine(root)
	view, err := eng.BuildGraphView()
	if err != nil {
		t.Fatalf("BuildGraphView failed: %v", err)
	}
	if len(view.Nodes) != 2 {
		t.Errorf("nodes = %d, want 2 (orphan excluded from graph body)", len(view.Nodes))
	}
	if len(view.Unlinked) != 1 || view.Unlinked[0] != "orphan.md" {
		t.Errorf("unlinked = %v, want [orphan.md]", view.Unlinked)
	}
	if view.Stats == nil {
		t.Fatal("stats missing")
	}
	if view.Stats.NodeCount != 2 || view.Stats.EdgeCount != 1 {
		t.Errorf("stats = %+v, want 2 nodes 1 edge", view.Stats)
	}
	if view.Issues == nil {
		t.Error("issues must be a non-nil slice so JSON emits [] instead of null")
	}
	if len(view.Issues) != 0 {
		t.Errorf("issues = %v, want none", view.Issues)
	}
}

func TestBrokenLinks(t *testing.T) {
	root := t.TempDir()
	writeTestWiki(t, root, map[string]string{
		"wiki/index.md": `---
status: current
description: Index
---
# Index
- [ok.md](ok.md)
- [missing.md](missing.md)
`,
		"wiki/ok.md": `---
status: current
description: OK
---
# OK
`,
	})

	eng := newTestEngine(root)
	nodes, _, err := eng.BuildWikiGraph()
	if err != nil {
		t.Fatalf("BuildWikiGraph failed: %v", err)
	}
	broken := eng.BrokenLinks(nodes)
	if len(broken) != 1 || !strings.Contains(broken[0], "index.md -> missing.md") {
		t.Errorf("broken = %v, want [index.md -> missing.md]", broken)
	}

	// The graph view must surface the dropped edge as an issue, not hide it.
	view, err := eng.BuildGraphView()
	if err != nil {
		t.Fatalf("BuildGraphView failed: %v", err)
	}
	found := false
	for _, i := range view.Issues {
		if strings.Contains(i, "missing.md") {
			found = true
		}
	}
	if !found {
		t.Errorf("issues = %v, want broken link issue for missing.md", view.Issues)
	}
}

func TestRenderDot(t *testing.T) {
	nodes := []WikiNode{
		{File: "index.md", Links: []string{"a.md"}},
		{File: "a.md", Links: []string{}},
	}
	edges := []WikiEdge{{From: "index.md", To: "a.md"}}

	dot := RenderDot(nodes, edges)
	if !strings.HasPrefix(dot, "digraph wiki {\n") {
		t.Errorf("dot output missing digraph header:\n%s", dot)
	}
	if !strings.Contains(dot, "\"index.md\" [label=\"index.md\"];") {
		t.Errorf("dot output missing node declaration:\n%s", dot)
	}
	if !strings.Contains(dot, "\"index.md\" -> \"a.md\";") {
		t.Errorf("dot output missing edge:\n%s", dot)
	}
	if !strings.HasSuffix(dot, "}\n") {
		t.Errorf("dot output missing closing brace:\n%s", dot)
	}
}
