package engine

import (
	"os"

	"github.com/ramayac/go-wiki-engine/internal/model"
	"github.com/ramayac/go-wiki-engine/internal/paths"
)

// FrontMatter aliases model.PageMetadata so all engine callers share the real
// YAML-parsed typed metadata model without implicit active defaults.
type FrontMatter = model.PageMetadata

// Reference aliases model.Reference.
type Reference = model.Reference

// SourceReference aliases model.SourceReference.
type SourceReference = model.SourceReference

// ParseFrontMatter parses YAML front matter using gopkg.in/yaml.v3 via
// internal/model. It applies NO semantic defaults: a page without lifecycle or
// status returns empty fields and does NOT become active.
func ParseFrontMatter(content string) (FrontMatter, bool, error) {
	res := model.ParseFrontMatter(content)
	found := res.State != model.FrontMatterMissing
	return res.Metadata, found, res.Err
}

// PageFrontMatter loads and parses front matter for a repo-relative page path,
// enforcing strict repository path containment.
func (e *Engine) PageFrontMatter(relPath string) (FrontMatter, bool) {
	abs, err := paths.ResolveWithin(e.RootDir, relPath)
	if err != nil {
		return FrontMatter{}, false
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return FrontMatter{}, false
	}
	fm, found, _ := ParseFrontMatter(string(data))
	return fm, found
}
