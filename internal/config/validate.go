package config

import (
	"fmt"
	"regexp"

	"github.com/ramayac/go-wiki-engine/internal/paths"
)

// Validate checks that configuration settings satisfy containment and syntax
// invariants. In particular, wiki_dir must be a single safe segment contained
// within repoDir (when repoDir is non-empty).
func (c *Config) Validate(repoDir string) error {
	if err := paths.ValidateWikiDir(c.WikiDir); err != nil {
		return fmt.Errorf(".wikirc: %w", err)
	}
	if repoDir != "" {
		if _, err := paths.ResolveWikiRoot(repoDir, c.WikiDir); err != nil {
			return fmt.Errorf(".wikirc: %w", err)
		}
	}
	if c.IssuePattern != "" {
		if _, err := regexp.Compile(c.IssuePattern); err != nil {
			return fmt.Errorf(".wikirc: invalid issue_pattern %q: %w", c.IssuePattern, err)
		}
	}
	return nil
}
