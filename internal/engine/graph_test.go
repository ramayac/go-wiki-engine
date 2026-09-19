package engine

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

func TestBuildWikiGraph(t *testing.T) {
	root := t.TempDir()
	wikiDir := filepath.Join(root, "wiki")
	if err := os.MkdirAll(wikiDir, 0o755); err != nil {
		t.Fatal(err)
	}

	files := map[string]string{
		// index.md (active, current) -> links to current.md, legacy.md, and planned.md
		"wiki/index.md": `---
status: current
description: Index Page
---
# Index
- [current.md](current.md)
- [legacy.md](legacy.md)
- [planned.md](planned.md)
`,
		// current.md (active, current) -> links to subcurrent.md
		"wiki/current.md": `---
status: current
description: Current Page
created: 2026-06-01
updated: 2026-06-10
---
# Current
- [subcurrent.md](subcurrent.md)
`,
		// subcurrent.md (active, current) -> links back to index.md
		"wiki/subcurrent.md": `---
status: current
description: Sub-current Page
created: 2026-06-02
---
# Sub-current
- [index.md](index.md)
`,
		// legacy.md (non-active, legacy) -> links to legacy_child.md
		"wiki/legacy.md": `---
status: legacy
description: Legacy Page
---
# Legacy
- [legacy_child.md](legacy_child.md)
`,
		// legacy_child.md (active, current) but only reachable via legacy.md
		"wiki/legacy_child.md": `---
status: current
description: Legacy Child (orphan path)
---
# Legacy Child
`,
		// planned.md (active, planned) -> links to nothing
		"wiki/planned.md": `---
status: planned
description: Planned Page
created: 2026-05-15
---
# Planned
`,
	}

	for rel, content := range files {
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	eng := newTestEngine(root)
	nodes, edges, err := eng.BuildWikiGraph()
	if err != nil {
		t.Fatalf("BuildWikiGraph failed: %v", err)
	}

	// Active nodes should be: index.md, current.md, subcurrent.md, planned.md
	// legacy.md should be skipped because status=legacy
	// legacy_child.md should be skipped because it is only reachable via legacy.md (traversal cutoff)
	expectedNodes := map[string]bool{
		"index.md":      true,
		"current.md":    true,
		"subcurrent.md": true,
		"planned.md":    true,
	}

	if len(nodes) != len(expectedNodes) {
		t.Errorf("expected %d nodes, got %d: %+v", len(expectedNodes), len(nodes), nodes)
	}

	for _, n := range nodes {
		if !expectedNodes[n.File] {
			t.Errorf("unexpected node in graph: %s (status: %s)", n.File, n.Status)
		}
	}

	// Active edges should only connect active nodes:
	// index.md -> current.md
	// index.md -> planned.md (index.md -> legacy.md is removed because legacy.md is not active)
	// current.md -> subcurrent.md
	// subcurrent.md -> index.md
	expectedEdges := map[string]string{
		"index.md":      "current.md, planned.md",
		"current.md":    "subcurrent.md",
		"subcurrent.md": "index.md",
	}

	// Let's build a map of edges found
	foundEdges := make(map[string][]string)
	for _, e := range edges {
		foundEdges[e.From] = append(foundEdges[e.From], e.To)
	}

	for from, tos := range foundEdges {
		sort.Strings(tos)
		actual := strings.Join(tos, ", ")
		exp := expectedEdges[from]
		if actual != exp {
			t.Errorf("edges from %s: got %q, want %q", from, actual, exp)
		}
	}

	// Every node must serialize Links as an array, not null.
	for _, n := range nodes {
		if n.Links == nil {
			t.Errorf("node %s has nil Links; want empty slice so JSON emits []", n.File)
		}
	}
}

func TestSortNodes(t *testing.T) {
	// 1. Topological Sort (by depth)
	nodes := []WikiNode{
		{File: "subcurrent.md", Depth: 2},
		{File: "planned.md", Depth: 1},
		{File: "current.md", Depth: 1},
		{File: "index.md", Depth: 0},
	}

	SortNodes(nodes, "topo")
	if nodes[0].File != "index.md" || nodes[1].File != "current.md" || nodes[2].File != "planned.md" || nodes[3].File != "subcurrent.md" {
		t.Errorf("topological sort order incorrect: %+v", nodes)
	}

	// 2. Chronological Sort (recently updated first)
	mtime1 := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	mtime2 := time.Date(2026, 6, 2, 0, 0, 0, 0, time.UTC)

	chronoNodes := []WikiNode{
		{File: "nodeA.md", Created: "2026-05-15"},                        // May 15
		{File: "nodeB.md", Created: "2026-06-01", Updated: "2026-06-10"}, // June 10
		{File: "nodeC.md", Created: "2026-06-02", ModTime: mtime1},       // June 2 (fallback to created)
		{File: "nodeD.md", ModTime: mtime2},                              // June 2 (fallback to mtime)
	}

	SortNodes(chronoNodes, "chrono")
	// Expected order (descending):
	// 1. nodeB.md (June 10)
	// 2. nodeC.md (June 2 - created)
	// 3. nodeD.md (June 2 - modtime)
	// 4. nodeA.md (May 15)
	if chronoNodes[0].File != "nodeB.md" || chronoNodes[1].File != "nodeC.md" || chronoNodes[2].File != "nodeD.md" || chronoNodes[3].File != "nodeA.md" {
		t.Errorf("chronological sort order incorrect: %+v", chronoNodes)
	}
}
