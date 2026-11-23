package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/ramayac/go-wiki-engine/internal/model"
)

// GraphStats aliases model.GraphStats.
type GraphStats = model.GraphStats

// ComputeStats derives navigation statistics from the graph.
func ComputeStats(nodes []WikiNode, edges []WikiEdge) GraphStats {
	return model.ComputeGraphStats(nodes, edges)
}

// ValidateGraph returns graph-health diagnostics: duplicate nodes, self-loops,
// duplicate edges, and edges pointing at missing files.
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
		if e.Kind == "source" {
			continue
		}
		if _, err := os.Stat(filepath.Join(wikiDir, filepath.FromSlash(e.To))); os.IsNotExist(err) {
			issues = append(issues, fmt.Sprintf("broken edge: %s -> %s (target file missing)", e.From, e.To))
		}
	}
	return issues
}

// RenderTree renders the graph as an ASCII tree from index.md.
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

// NodeView is the neighborhood navigation payload: one node with its backlinks
// and outgoing links.
type NodeView struct {
	Node      WikiNode `json:"node"`
	Backlinks []string `json:"backlinks"`
}

// Neighborhood returns one node (looked up by wiki-relative path, repo-relative
// path, or stable page ID) with its direct backlinks and outgoing links.
func Neighborhood(nodes []WikiNode, edges []WikiEdge, page string) (*NodeView, error) {
	node, err := model.LookupGraphNode(nodes, "wiki", page)
	if err != nil {
		return nil, err
	}
	seenBack := make(map[string]bool)
	backlinks := []string{}
	for _, e := range edges {
		if e.To == node.File && !seenBack[e.From] {
			seenBack[e.From] = true
			backlinks = append(backlinks, e.From)
		}
	}
	sort.Strings(backlinks)
	return &NodeView{Node: *node, Backlinks: backlinks}, nil
}

// NeighborhoodForWiki resolves page using the configured wikiDir prefix.
func NeighborhoodForWiki(nodes []WikiNode, edges []WikiEdge, wikiDir, page string) (*NodeView, error) {
	node, err := model.LookupGraphNode(nodes, wikiDir, page)
	if err != nil {
		return nil, err
	}
	seenBack := make(map[string]bool)
	backlinks := []string{}
	for _, e := range edges {
		if e.To == node.File && !seenBack[e.From] {
			seenBack[e.From] = true
			backlinks = append(backlinks, e.From)
		}
	}
	sort.Strings(backlinks)
	return &NodeView{Node: *node, Backlinks: backlinks}, nil
}

// BrokenLinks returns link targets referenced from active graph pages that do
// not exist on disk, computed from a single snapshot.
func (e *Engine) BrokenLinks(nodes []WikiNode) []string {
	snap, err := e.LoadSnapshot()
	if err != nil {
		return nil
	}
	activeSet := make(map[string]bool, len(nodes))
	for _, n := range nodes {
		activeSet[n.File] = true
	}
	var broken []string
	for _, p := range snap.Pages {
		if !activeSet[p.WikiRel] {
			continue
		}
		for _, dest := range p.InternalMDLinks() {
			if _, ok := snap.ByPath[dest]; !ok {
				broken = append(broken, fmt.Sprintf("broken edge: %s -> %s (target file missing)", p.WikiRel, dest))
			}
		}
	}
	return broken
}

// BuildGraphView assembles the full navigation payload from ONE WikiSnapshot.
func (e *Engine) BuildGraphView() (*WikiGraphJSON, error) {
	return e.BuildGraphViewWithOptions(GraphOptions{})
}

// BuildGraphViewWithOptions builds the graph view with optional source nodes
// and historical inclusion from one WikiSnapshot.
func (e *Engine) BuildGraphViewWithOptions(opts GraphOptions) (*WikiGraphJSON, error) {
	snap, err := e.LoadSnapshot()
	if err != nil {
		return nil, err
	}
	if err := snap.FatalLoadError(); err != nil {
		return nil, err
	}
	return model.BuildGraph(snap, opts)
}

// RenderDot renders the graph in Graphviz DOT format.
func RenderDot(nodes []WikiNode, edges []WikiEdge) string {
	var b strings.Builder
	b.WriteString("digraph wiki {\n")
	b.WriteString("  rankdir=LR;\n")
	for _, n := range nodes {
		if n.Kind == "source" {
			fmt.Fprintf(&b, "  %s [label=%s, shape=box, style=dashed];\n", dotQuote(n.File), dotQuote(n.File))
		} else {
			fmt.Fprintf(&b, "  %s [label=%s];\n", dotQuote(n.File), dotQuote(n.File))
		}
	}
	for _, e := range edges {
		switch e.Kind {
		case "depends_on":
			fmt.Fprintf(&b, "  %s -> %s [label=\"depends_on\"];\n", dotQuote(e.From), dotQuote(e.To))
		case "source":
			fmt.Fprintf(&b, "  %s -> %s [style=dashed];\n", dotQuote(e.From), dotQuote(e.To))
		default:
			fmt.Fprintf(&b, "  %s -> %s;\n", dotQuote(e.From), dotQuote(e.To))
		}
	}
	b.WriteString("}\n")
	return b.String()
}

func dotQuote(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return `"` + s + `"`
}
