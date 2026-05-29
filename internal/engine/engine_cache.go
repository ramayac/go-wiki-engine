package engine

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// wikiCache is the on-disk cache stored at .wiki/.cache.json.
type wikiCache struct {
	Built   string            `json:"built"`   // ISO timestamp of last build
	Files   []string          `json:"files"`   // sorted list of wiki files (relative to wiki dir)
	MTimes  map[string]int64  `json:"mtimes"`  // file → modtime (Unix nano)
	Lines   map[string]int    `json:"lines"`   // file → line count
}

// cachePath returns the path to the cache file.
func (e *Engine) cachePath() string {
	return filepath.Join(e.WikiPath(), ".cache.json")
}

// loadCache reads and validates the cache. Returns nil if invalid or missing.
func (e *Engine) loadCache() *wikiCache {
	if !e.Cfg.CacheEnabled {
		return nil
	}
	path := e.cachePath()
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var c wikiCache
	if err := json.Unmarshal(data, &c); err != nil {
		return nil
	}
	if !e.cacheValid(&c) {
		return nil
	}
	return &c
}

// saveCache rebuilds and writes the cache.
func (e *Engine) saveCache() error {
	c := &wikiCache{
		Built:  time.Now().UTC().Format(time.RFC3339),
		MTimes: make(map[string]int64),
		Lines:  make(map[string]int),
	}

	wikiDir := e.WikiPath()
	err := filepath.WalkDir(wikiDir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(wikiDir, path)
		c.Files = append(c.Files, rel)
		c.MTimes[rel] = info.ModTime().UnixNano()

		// Count lines for .md files.
		if strings.HasSuffix(rel, ".md") {
			data, err := os.ReadFile(path)
			if err == nil {
				c.Lines[rel] = len(strings.Split(string(data), "\n"))
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	sort.Strings(c.Files)

	// Ensure .wiki directory exists for the cache file.
	os.MkdirAll(filepath.Dir(e.cachePath()), 0o755)

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(e.cachePath(), data, 0o644)
}

// cacheValid checks whether the on-disk cache matches current filesystem state.
func (e *Engine) cacheValid(c *wikiCache) bool {
	wikiDir := e.WikiPath()
	currentFiles := make(map[string]bool)

	walkErr := filepath.WalkDir(wikiDir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(wikiDir, path)
		currentFiles[rel] = true

		info, err := d.Info()
		if err != nil {
			return nil
		}
		cachedMT, ok := c.MTimes[rel]
		if !ok || cachedMT != info.ModTime().UnixNano() {
			return errFileChanged
		}
		return nil
	})

	if walkErr != nil {
		return false
	}

	// Check that cached files still exist.
	for _, f := range c.Files {
		if !currentFiles[f] {
			return false
		}
	}
	return true
}

var errFileChanged = filepath.SkipAll // sentinel to abort walk early

// cachedList returns file list from cache, falling back to filesystem walk.
func (e *Engine) cachedList() ([]string, error) {
	if c := e.loadCache(); c != nil {
		return c.Files, nil
	}
	return e.List()
}

// RebuildCache saves a fresh cache if the current one is stale
// or missing. Called by lint --rebuild-cache.
func (e *Engine) RebuildCache() error {
	return e.saveCache()
}
