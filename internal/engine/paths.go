package engine

import "github.com/ramayac/go-wiki-engine/internal/model"

// isCanonicalFile delegates to model.IsCanonicalFile.
func isCanonicalFile(name, wikiRel string) bool {
	return model.IsCanonicalFile(name, wikiRel)
}
