package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

var linkRe = regexp.MustCompile(`\]\(([^)]+)\)`)

// ExtractLinks parses a markdown string and returns all outgoing relative page-to-page links.
// Each returned link is a clean path relative to the wiki directory, e.g. "operations/ingest.md".
func ExtractLinks(content string, currentFileDir string, wikiDir string) []string {
	var links []string
	seen := make(map[string]bool)

	for _, match := range linkRe.FindAllStringSubmatch(content, -1) {
		target := match[1]
		// Skip external links and page anchors
		if strings.HasPrefix(target, "http://") || strings.HasPrefix(target, "https://") || strings.HasPrefix(target, "#") {
			continue
		}

		// Strip anchor if present
		if idx := strings.Index(target, "#"); idx != -1 {
			target = target[:idx]
		}
		if target == "" {
			continue
		}

		// Only local markdown links are relevant for page-to-page graph indexing
		if !strings.HasSuffix(target, ".md") {
			continue
		}

		// Resolve path relative to current file's directory
		absTarget := filepath.Clean(filepath.Join(currentFileDir, target))
		relToWiki, err := filepath.Rel(wikiDir, absTarget)
		if err != nil {
			continue
		}

		// Normalize slash format (use forward slash for compatibility)
		relToWiki = filepath.ToSlash(relToWiki)

		// Prevent duplicate entries
		if !seen[relToWiki] {
			seen[relToWiki] = true
			links = append(links, relToWiki)
		}
	}

	return links
}

// WikiNode represents a node in the active wiki graph.
type WikiNode struct {
	File        string    `json:"file"`
	Status      string    `json:"status"`
	Description string    `json:"description"`
	Created     string    `json:"created,omitempty"`
	Updated     string    `json:"updated,omitempty"`
	ModTime     time.Time `json:"-"`
	Links       []string  `json:"links"`
	Depth       int       `json:"depth"`
}

// WikiEdge represents a directed edge between active wiki pages.
type WikiEdge struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// WikiGraphJSON holds the serializable active wiki graph representation.
type WikiGraphJSON struct {
	Nodes []WikiNode `json:"nodes"`
	Edges []WikiEdge `json:"edges"`
}

// BuildWikiGraph constructs the active wiki graph starting BFS from index.md.
// It skips legacy/deprecated pages and halts traversal at their boundaries.
func (e *Engine) BuildWikiGraph() ([]WikiNode, []WikiEdge, error) {
	wikiDir := e.WikiPath()
	startFile := "index.md"
	absStart := filepath.Join(wikiDir, startFile)
	if _, err := os.Stat(absStart); os.IsNotExist(err) {
		return nil, nil, fmt.Errorf("index.md not found at %s", absStart)
	}

	type queueItem struct {
		file  string
		depth int
	}

	queue := []queueItem{{file: startFile, depth: 0}}
	visited := make(map[string]bool)
	nodesMap := make(map[string]*WikiNode)
	var rawEdges []WikiEdge

	for len(queue) > 0 {
		curr := queue[0]
		queue = queue[1:]

		cleanRel := filepath.ToSlash(filepath.Clean(curr.file))
		if visited[cleanRel] {
			continue
		}
		visited[cleanRel] = true

		absPath := filepath.Join(wikiDir, cleanRel)
		data, err := os.ReadFile(absPath)
		if err != nil {
			continue
		}

		info, err := os.Stat(absPath)
		var mtime time.Time
		if err == nil {
			mtime = info.ModTime()
		}

		content := string(data)
		fm, _, _ := ParseFrontMatter(content)

		// Traversal cutoff: do not traverse legacy/deprecated pages
		if fm.Status == "legacy" || fm.Status == "deprecated" {
			continue
		}

		links := ExtractLinks(content, filepath.Dir(absPath), wikiDir)

		nodesMap[cleanRel] = &WikiNode{
			File:        cleanRel,
			Status:      fm.Status,
			Description: fm.Description,
			Created:     fm.Created,
			Updated:     fm.Updated,
			ModTime:     mtime,
			Links:       links,
			Depth:       curr.depth,
		}

		for _, dest := range links {
			rawEdges = append(rawEdges, WikiEdge{From: cleanRel, To: dest})
			if !visited[dest] {
				queue = append(queue, queueItem{file: dest, depth: curr.depth + 1})
			}
		}
	}

	// Filter edges to ensure both From and To are active/reached nodes
	var edges []WikiEdge
	for _, edge := range rawEdges {
		if nodesMap[edge.From] != nil && nodesMap[edge.To] != nil {
			edges = append(edges, edge)
		}
	}

	// Filter node links to only reference active/reached nodes
	for _, node := range nodesMap {
		var activeLinks []string
		for _, link := range node.Links {
			if nodesMap[link] != nil {
				activeLinks = append(activeLinks, link)
			}
		}
		node.Links = activeLinks
	}

	var nodes []WikiNode
	for _, node := range nodesMap {
		nodes = append(nodes, *node)
	}

	return nodes, edges, nil
}

// SortNodes sorts a slice of WikiNodes.
// Support: "topo" (by depth, parents before children),
// "chrono" (by updated -> created -> mtime, recently updated first).
func SortNodes(nodes []WikiNode, sortBy string) {
	if sortBy == "topo" {
		sort.Slice(nodes, func(i, j int) bool {
			if nodes[i].Depth != nodes[j].Depth {
				return nodes[i].Depth < nodes[j].Depth
			}
			return nodes[i].File < nodes[j].File
		})
	} else {
		sort.Slice(nodes, func(i, j int) bool {
			timeI := resolveTime(nodes[i])
			timeJ := resolveTime(nodes[j])
			if !timeI.Equal(timeJ) {
				return timeI.After(timeJ)
			}
			return nodes[i].File < nodes[j].File
		})
	}
}

func resolveTime(node WikiNode) time.Time {
	if node.Updated != "" {
		if t, err := time.Parse("2006-01-02", node.Updated); err == nil {
			return t
		}
	}
	if node.Created != "" {
		if t, err := time.Parse("2006-01-02", node.Created); err == nil {
			return t
		}
	}
	return node.ModTime
}
