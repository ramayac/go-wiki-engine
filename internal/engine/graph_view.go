package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// GraphStats summarizes the graph for navigation: size, depth, roots
// (no incoming edges) and leaves (no outgoing edges).
type GraphStats struct {
	NodeCount int      `json:"node_count"`
	EdgeCount int      `json:"edge_count"`
	MaxDepth  int      `json:"max_depth"`
	Roots     []string `json:"roots"`
	Leaves    []string `json:"leaves"`
}

// ComputeStats derives navigation statistics from the graph. Nodes are
// expected to be depth-populated (BuildWikiGraph sets Depth during BFS).
func ComputeStats(nodes []WikiNode, edges []WikiEdge) GraphStats {
	inDeg := make(map[string]int, len(nodes))
	for _, e := range edges {
		inDeg[e.To]++
	}
	st := GraphStats{
		NodeCount: len(nodes),
		EdgeCount: len(edges),
		Roots:     []string{},
		Leaves:    []string{},
	}
	for _, n := range nodes {
		if n.Depth > st.MaxDepth {
			st.MaxDepth = n.Depth
		}
		if inDeg[n.File] == 0 {
			st.Roots = append(st.Roots, n.File)
		}
		if len(n.Links) == 0 {
			st.Leaves = append(st.Leaves, n.File)
		}
	}
	sort.Strings(st.Roots)
	sort.Strings(st.Leaves)
	return st
}

// ValidateGraph returns graph-health diagnostics: duplicate edges, self-loops,
// and edges pointing at files that do not exist on disk. Duplicate nodes are
// impossible by construction (BFS visited set); the check exists as a guard.
func ValidateGraph(nodes []WikiNode, edges []WikiEdge, wikiDir string) []string {
	var issues []string
	seenEdges := make(map[WikiEdge]bool, len(edges))
	seenNodes := make(map[string]bool, len(nodes))

	for _, n := range nodes {
		if seenNodes[n.File] {
			issues = append(issues, fmt.Sprintf("duplicate node: %s", n.File))
		}
		seenNodes[n.File] = true
	}
	for _, e := range edges {
		if e.From == e.To {
			issues = append(issues, fmt.Sprintf("self-loop: %s -> %s", e.From, e.To))
		}
		if seenEdges[e] {
			issues = append(issues, fmt.Sprintf("duplicate edge: %s -> %s", e.From, e.To))
		}
		seenEdges[e] = true
		if _, err := os.Stat(filepath.Join(wikiDir, filepath.FromSlash(e.To))); os.IsNotExist(err) {
			issues = append(issues, fmt.Sprintf("broken edge: %s -> %s (target file missing)", e.From, e.To))
		}
	}
	return issues
}

// RenderTree renders the graph as an ASCII tree from index.md. The first
// visit wins: diamonds and cycles render later references as "↰ (see above)"
// markers so the output is a tree, never a loop. Nodes must be topo-sorted
// (SortNodes(nodes, "topo")) so parents appear before children.
func RenderTree(nodes []WikiNode) string {
	byFile := make(map[string]WikiNode, len(nodes))
	for _, n := range nodes {
		byFile[n.File] = n
	}
	if _, ok := byFile["index.md"]; !ok {
		return ""
	}

	children := make(map[string][]string, len(nodes))
	for _, n := range nodes {
		for _, l := range n.Links {
			if _, ok := byFile[l]; ok {
				children[n.File] = append(children[n.File], l)
			}
		}
	}

	var b strings.Builder
	root := byFile["index.md"]
	b.WriteString(root.File + " [" + root.Status + "] | " + root.Description + "\n")

	firstVisit := map[string]bool{"index.md": true}
	var walk func(file, prefix string)
	walk = func(file, prefix string) {
		kids := children[file]
		for i, k := range kids {
			last := i == len(kids)-1
			branch, indent := "├── ", "│   "
			if last {
				branch, indent = "└── ", "    "
			}
			if firstVisit[k] {
				b.WriteString(prefix + branch + "↰ " + k + " (see above)\n")
				continue
			}
			firstVisit[k] = true
			n := byFile[k]
			b.WriteString(prefix + branch + k + " [" + n.Status + "] | " + n.Description + "\n")
			walk(k, prefix+indent)
		}
	}
	walk("index.md", "")
	return b.String()
}

// NodeView is the neighborhood navigation payload: one node with the pages
// that link to it and the pages it links to.
type NodeView struct {
	Node      WikiNode `json:"node"`
	Backlinks []string `json:"backlinks"`
}

// Neighborhood returns one node with its backlinks (computed by inverting
// the edge list) and outgoing links.
func Neighborhood(nodes []WikiNode, edges []WikiEdge, page string) (*NodeView, error) {
	page = filepath.ToSlash(filepath.Clean(page))
	var node *WikiNode
	for i := range nodes {
		if nodes[i].File == page {
			node = &nodes[i]
			break
		}
	}
	if node == nil {
		return nil, fmt.Errorf("page not found in active graph: %s", page)
	}
	var backlinks []string
	for _, e := range edges {
		if e.To == page {
			backlinks = append(backlinks, e.From)
		}
	}
	sort.Strings(backlinks)
	return &NodeView{Node: *node, Backlinks: backlinks}, nil
}

// BrokenLinks returns link targets referenced from active graph pages that
// do not exist on disk. The BFS silently drops those edges (an unreadable
// target is never enqueued), so they are re-derived here from the active
// nodes and surfaced as graph issues instead of vanishing.
func (e *Engine) BrokenLinks(nodes []WikiNode) []string {
	wikiDir := e.WikiPath()
	var broken []string
	for _, n := range nodes {
		absPath := filepath.Join(wikiDir, filepath.FromSlash(n.File))
		data, err := os.ReadFile(absPath)
		if err != nil {
			continue
		}
		for _, l := range ExtractLinks(string(data), filepath.Dir(absPath), wikiDir) {
			target := filepath.Join(wikiDir, filepath.FromSlash(l))
			if _, err := os.Stat(target); os.IsNotExist(err) {
				broken = append(broken, fmt.Sprintf("broken edge: %s -> %s (target file missing)", n.File, l))
			}
		}
	}
	return broken
}

// BuildGraphView assembles the navigation payload by reusing the existing
// graph builder and orphan detection: topo-sorted nodes, edges, orphaned
// active pages, navigation stats, and graph-health diagnostics.
func (e *Engine) BuildGraphView() (*WikiGraphJSON, error) {
	nodes, edges, err := e.BuildWikiGraph()
	if err != nil {
		return nil, err
	}
	SortNodes(nodes, "topo")

	unlinked, err := e.ActiveUnlinkedPages()
	if err != nil {
		return nil, err
	}
	issues := ValidateGraph(nodes, edges, e.WikiPath())
	if broken := e.BrokenLinks(nodes); len(broken) > 0 {
		issues = append(issues, broken...)
	}
	if issues == nil {
		issues = []string{}
	}
	stats := ComputeStats(nodes, edges)

	return &WikiGraphJSON{
		Nodes:    nodes,
		Edges:    edges,
		Unlinked: unlinked,
		Stats:    &stats,
		Issues:   issues,
	}, nil
}

// RenderDot renders the graph in Graphviz DOT format for external
// visualization, e.g. `wiki-engine graph --dot | dot -Tsvg > wiki.svg`.
func RenderDot(nodes []WikiNode, edges []WikiEdge) string {
	var b strings.Builder
	b.WriteString("digraph wiki {\n")
	b.WriteString("  rankdir=LR;\n")
	for _, n := range nodes {
		fmt.Fprintf(&b, "  %s [label=%s];\n", dotQuote(n.File), dotQuote(n.File))
	}
	for _, e := range edges {
		fmt.Fprintf(&b, "  %s -> %s;\n", dotQuote(e.From), dotQuote(e.To))
	}
	b.WriteString("}\n")
	return b.String()
}

// dotQuote escapes a string for use as a double-quoted DOT identifier.
func dotQuote(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return `"` + s + `"`
}
