package engine

import (
	"path/filepath"
	"regexp"
	"strings"

	"github.com/ramayac/go-wiki-engine/internal/model"
	"github.com/ramayac/go-wiki-engine/internal/paths"
)

var linkRe = regexp.MustCompile(`\]\(([^)]+)\)`)

// ExtractLinks parses a markdown string and returns all outgoing relative
// page-to-page links that stay strictly contained inside wikiDir.
// Links attempting to escape wikiDir (via .., absolute paths, percent-encoding,
// or symlinks) are rejected.
func ExtractLinks(content string, currentFileDir string, wikiDir string) []string {
	var links []string
	seen := make(map[string]bool)

	for _, match := range linkRe.FindAllStringSubmatch(content, -1) {
		target := strings.TrimSpace(match[1])
		if strings.HasPrefix(target, "http://") || strings.HasPrefix(target, "https://") || strings.HasPrefix(target, "#") || strings.HasPrefix(target, "mailto:") {
			continue
		}
		if idx := strings.Index(target, "#"); idx != -1 {
			target = target[:idx]
		}
		if target == "" {
			continue
		}
		if !strings.HasSuffix(strings.ToLower(target), ".md") {
			continue
		}
		_, relToWiki, err := paths.ResolvePageRelative(wikiDir, currentFileDir, target)
		if err != nil {
			continue
		}
		if !seen[relToWiki] {
			seen[relToWiki] = true
			links = append(links, relToWiki)
		}
	}
	return links
}

// WikiNode aliases model.GraphNode so the CLI and graph views share one model.
type WikiNode = model.GraphNode

// WikiEdge aliases model.GraphEdge.
type WikiEdge = model.GraphEdge

// WikiGraphJSON aliases model.GraphView (includes schema_version, nodes, edges,
// unlinked, stats, issues).
type WikiGraphJSON = model.GraphView

// GraphOptions aliases model.GraphOptions.
type GraphOptions = model.GraphOptions

// BuildWikiGraph constructs the active wiki graph from a single WikiSnapshot.
func (e *Engine) BuildWikiGraph() ([]WikiNode, []WikiEdge, error) {
	snap, err := e.LoadSnapshot()
	if err != nil {
		return nil, nil, err
	}
	if err := snap.FatalLoadError(); err != nil {
		return nil, nil, err
	}
	view, err := model.BuildGraph(snap, model.GraphOptions{})
	if err != nil {
		return nil, nil, err
	}
	return view.Nodes, view.Edges, nil
}

// SortNodes sorts a slice of WikiNodes ("topo" or "chrono").
func SortNodes(nodes []WikiNode, sortBy string) {
	model.SortGraphNodes(nodes, sortBy)
}

// ActiveUnlinkedPages returns active wiki pages that are not reachable from
// index.md in the active graph, built from one WikiSnapshot.
func (e *Engine) ActiveUnlinkedPages() ([]string, error) {
	snap, err := e.LoadSnapshot()
	if err != nil {
		return nil, err
	}
	if err := snap.FatalLoadError(); err != nil {
		return nil, err
	}
	view, err := model.BuildGraph(snap, model.GraphOptions{})
	if err != nil {
		return nil, err
	}
	return view.Unlinked, nil
}

// keep filepath import clean if needed
var _ = filepath.Clean
