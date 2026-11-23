package model

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/ramayac/go-wiki-engine/internal/schema"
)

// GraphNode represents a node in the unified wiki graph (a wiki page or an
// opt-in repository source node).
type GraphNode struct {
	ID          string            `json:"id"`
	Kind        string            `json:"kind,omitempty"` // "page" (default) or "source"
	File        string            `json:"file"`
	Title       string            `json:"title,omitempty"`
	Type        string            `json:"type,omitempty"`
	Lifecycle   string            `json:"lifecycle,omitempty"`
	Status      string            `json:"status"`
	Description string            `json:"description"`
	Created     string            `json:"created,omitempty"`
	Updated     string            `json:"updated,omitempty"`
	ModTime     time.Time         `json:"-"`
	Links       []string          `json:"links"`
	Related     []string          `json:"related,omitempty"`
	DependsOn   []string          `json:"depends_on,omitempty"`
	Sources     []SourceReference `json:"sources,omitempty"`
	References  []Reference       `json:"references,omitempty"`
	Depth       int               `json:"depth"`
}

// GraphEdge represents a directed, deduplicated edge in the wiki graph.
type GraphEdge struct {
	From string `json:"from"`
	To   string `json:"to"`
	Kind string `json:"kind,omitempty"` // "link", "related", "depends_on", "source"
}

// GraphStats summarizes the graph for navigation.
type GraphStats struct {
	NodeCount int      `json:"node_count"`
	EdgeCount int      `json:"edge_count"`
	MaxDepth  int      `json:"max_depth"`
	Roots     []string `json:"roots"`
	Leaves    []string `json:"leaves"`
}

// GraphView is the complete serializable graph payload with schema versioning.
type GraphView struct {
	SchemaVersion string      `json:"schema_version"`
	Nodes         []GraphNode `json:"nodes"`
	Edges         []GraphEdge `json:"edges"`
	Unlinked      []string    `json:"unlinked"`
	Stats         *GraphStats `json:"stats,omitempty"`
	Issues        []string    `json:"issues"`
}

// GraphOptions controls graph construction.
type GraphOptions struct {
	IncludeSources bool
	IncludeHistory bool
}

// BuildGraph constructs the wiki graph from a single WikiSnapshot starting BFS
// at index.md. Edges combine validated Markdown links, `related`, `depends_on`,
// and (when opts.IncludeSources is true) local repository source nodes.
// Historical pages (legacy, superseded) and pages with missing lifecycle do not
// connect active traversal paths unless opts.IncludeHistory is true.
func BuildGraph(snap *WikiSnapshot, opts GraphOptions) (*GraphView, error) {
	if snap.Index == nil {
		return nil, fmt.Errorf("index.md not found at %s", filepath.Join(snap.WikiRoot, "index.md"))
	}

	allowPage := func(p *Page) bool {
		if opts.IncludeHistory {
			return true
		}
		return p.IsActive()
	}

	type qItem struct {
		page  *Page
		depth int
	}

	queue := []qItem{{page: snap.Index, depth: 0}}
	visited := make(map[string]bool)
	nodesMap := make(map[string]*GraphNode)
	var issues []string

	type edgeKey struct {
		from string
		to   string
		kind string
	}
	seenEdges := make(map[edgeKey]bool)
	var rawEdges []GraphEdge

	for len(queue) > 0 {
		curr := queue[0]
		queue = queue[1:]

		p := curr.page
		if visited[p.WikiRel] {
			continue
		}
		visited[p.WikiRel] = true

		if !allowPage(p) {
			continue
		}

		lc := p.EffectiveLifecycle()
		statusDisplay := lc
		if p.Metadata.Status != "" && (p.Metadata.Type == "decision" || p.Metadata.Type == "incident") {
			statusDisplay = p.Metadata.Status
		} else if statusDisplay == "" {
			statusDisplay = p.Metadata.Status
		}

		gn := &GraphNode{
			ID:          p.ID,
			Kind:        "page",
			File:        p.WikiRel,
			Title:       p.EffectiveTitle(),
			Type:        p.Metadata.Type,
			Lifecycle:   lc,
			Status:      statusDisplay,
			Description: p.Metadata.Description,
			Created:     p.Metadata.Created,
			Updated:     p.Metadata.Updated,
			ModTime:     p.ModTime,
			Links:       []string{},
			Related:     p.Metadata.Related,
			DependsOn:   p.Metadata.DependsOn,
			Sources:     p.Metadata.Sources,
			References:  p.Metadata.References,
			Depth:       curr.depth,
		}
		nodesMap[p.WikiRel] = gn

		addPageEdge := func(destWikiRel, kind string) {
			if destWikiRel == p.WikiRel {
				// Reject self-edges and record diagnostic.
				issues = append(issues, fmt.Sprintf("self-loop: %s -> %s", p.WikiRel, destWikiRel))
				return
			}
			destPage, ok := snap.ByPath[destWikiRel]
			if !ok {
				issues = append(issues, fmt.Sprintf("broken edge: %s -> %s (target file missing)", p.WikiRel, destWikiRel))
				return
			}
			if !allowPage(destPage) {
				return
			}
			ek := edgeKey{from: p.WikiRel, to: destWikiRel, kind: kind}
			if seenEdges[ek] {
				return
			}
			seenEdges[ek] = true
			rawEdges = append(rawEdges, GraphEdge{From: p.WikiRel, To: destWikiRel, Kind: kind})
			if !containsStr(gn.Links, destWikiRel) {
				gn.Links = append(gn.Links, destWikiRel)
			}
			if !visited[destWikiRel] {
				queue = append(queue, qItem{page: destPage, depth: curr.depth + 1})
			}
		}

		// 1. Validated Markdown links (strictly contained in wiki).
		for _, lk := range p.Links {
			if !lk.IsMarkdown || lk.IsExternal || lk.IsAnchorOnly {
				continue
			}
			if lk.EscapesWiki {
				// Only flag as a graph issue if it doesn't validly point to an external repo file.
				if lk.EscapesRepo {
					issues = append(issues, fmt.Sprintf("escaped link: %s -> %s", p.WikiRel, lk.RawTarget))
				}
				continue
			}
			if lk.WikiRel != "" {
				addPageEdge(lk.WikiRel, "link")
			}
		}

		// 2. Frontmatter `related` IDs.
		for _, relID := range p.Metadata.Related {
			if target, err := snap.Lookup(relID); err == nil {
				addPageEdge(target.WikiRel, "related")
			} else {
				issues = append(issues, fmt.Sprintf("broken related edge: %s -> %s (%v)", p.WikiRel, relID, err))
			}
		}

		// 3. Frontmatter `depends_on` IDs.
		for _, depID := range p.Metadata.DependsOn {
			if target, err := snap.Lookup(depID); err == nil {
				addPageEdge(target.WikiRel, "depends_on")
			} else {
				issues = append(issues, fmt.Sprintf("broken depends_on edge: %s -> %s (%v)", p.WikiRel, depID, err))
			}
		}
	}

	// Filter edges to reached nodes only and deduplicate across kinds for node.Links.
	var edges []GraphEdge
	for _, e := range rawEdges {
		if nodesMap[e.From] != nil && nodesMap[e.To] != nil {
			edges = append(edges, e)
		}
	}

	// Optional source nodes (opt-in via --include-sources).
	if opts.IncludeSources {
		sourceNodes := make(map[string]*GraphNode)
		for _, p := range snap.Pages {
			gn := nodesMap[p.WikiRel]
			if gn == nil {
				continue
			}
			for _, rs := range p.ResolvedSources {
				if rs.CrossRepo || rs.Err != nil || rs.RepoRel == "" {
					continue
				}
				srcID := rs.RepoRel
				if _, exists := sourceNodes[srcID]; !exists {
					sourceNodes[srcID] = &GraphNode{
						ID:          srcID,
						Kind:        "source",
						File:        srcID,
						Status:      "source",
						Description: "Repository source file",
						Links:       []string{},
						Depth:       gn.Depth + 1,
					}
				}
				ek := edgeKey{from: p.WikiRel, to: srcID, kind: "source"}
				if !seenEdges[ek] {
					seenEdges[ek] = true
					edges = append(edges, GraphEdge{From: p.WikiRel, To: srcID, Kind: "source"})
				}
			}
		}
		for _, sn := range sourceNodes {
			nodesMap[sn.File] = sn
		}
	}

	// Compute active unlinked pages (active pages not reached from index.md).
	unlinked := []string{}
	for _, p := range snap.Pages {
		if nodesMap[p.WikiRel] != nil {
			continue
		}
		if p.IsActive() {
			unlinked = append(unlinked, p.WikiRel)
		}
	}
	sort.Strings(unlinked)

	nodes := make([]GraphNode, 0, len(nodesMap))
	for _, gn := range nodesMap {
		nodes = append(nodes, *gn)
	}
	SortGraphNodes(nodes, "topo")

	if issues == nil {
		issues = []string{}
	}
	sort.Strings(issues)
	if edges == nil {
		edges = []GraphEdge{}
	}

	stats := ComputeGraphStats(nodes, edges)
	return &GraphView{
		SchemaVersion: schema.GraphSchemaVersion,
		Nodes:         nodes,
		Edges:         edges,
		Unlinked:      unlinked,
		Stats:         &stats,
		Issues:        issues,
	}, nil
}

// ComputeGraphStats computes navigation statistics from a graph.
func ComputeGraphStats(nodes []GraphNode, edges []GraphEdge) GraphStats {
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
		if len(n.Links) == 0 && n.Kind != "source" {
			st.Leaves = append(st.Leaves, n.File)
		}
	}
	sort.Strings(st.Roots)
	sort.Strings(st.Leaves)
	return st
}

// SortGraphNodes sorts nodes in "topo" (depth then path) or "chrono" (recency then path) order.
func SortGraphNodes(nodes []GraphNode, sortBy string) {
	if sortBy == "topo" {
		sort.Slice(nodes, func(i, j int) bool {
			if nodes[i].Depth != nodes[j].Depth {
				return nodes[i].Depth < nodes[j].Depth
			}
			return nodes[i].File < nodes[j].File
		})
		return
	}
	sort.Slice(nodes, func(i, j int) bool {
		ti := resolveNodeTime(nodes[i])
		tj := resolveNodeTime(nodes[j])
		if !ti.Equal(tj) {
			return ti.After(tj)
		}
		return nodes[i].File < nodes[j].File
	})
}

func resolveNodeTime(n GraphNode) time.Time {
	if n.Updated != "" {
		if t, err := time.Parse("2006-01-02", n.Updated); err == nil {
			return t
		}
	}
	if n.Created != "" {
		if t, err := time.Parse("2006-01-02", n.Created); err == nil {
			return t
		}
	}
	return n.ModTime
}

func containsStr(list []string, target string) bool {
	for _, s := range list {
		if s == target {
			return true
		}
	}
	return false
}

// LookupGraphNode resolves a page ID or wiki/repo-relative path against graph nodes.
func LookupGraphNode(nodes []GraphNode, wikiDir, query string) (*GraphNode, error) {
	clean := filepath.ToSlash(filepath.Clean(strings.TrimSpace(query)))
	stripped := strings.TrimPrefix(clean, wikiDir+"/")
	// 1. Exact file match.
	for i := range nodes {
		if nodes[i].File == clean || nodes[i].File == stripped {
			return &nodes[i], nil
		}
	}
	// 2. Exact ID match.
	var matches []*GraphNode
	for i := range nodes {
		if nodes[i].ID == clean {
			matches = append(matches, &nodes[i])
		}
	}
	if len(matches) == 1 {
		return matches[0], nil
	}
	if len(matches) > 1 {
		return nil, fmt.Errorf("ambiguous page ID in graph: %s", query)
	}
	return nil, fmt.Errorf("page not found in active graph: %s", query)
}
