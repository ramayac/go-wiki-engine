// Package config loads, parses, and validates the .wikirc configuration file.
package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/ramayac/go-wiki-engine/internal/schema"
)

// Config holds the parsed .wikirc settings.
type Config struct {
	WikiDir            string
	DefaultDiff        string
	BaseRef            string // optional base ref fallback for branch-aware checks
	LogLines           int
	Ignore             []string
	DuplicateThreshold float64 // 0.0-1.0, similarity above which pages are flagged as duplicates
	StaleDays          int     // days after which an unchanged wiki page is flagged as stale
	ContextSummarize   bool    // enable --summarize on wiki-engine context
	ContextMaxPages    int     // max catalog pages in bounded context output (0 = unlimited)
	ContextMaxBytes    int     // max byte budget in bounded context output (0 = unlimited)
	WatchInterval      int     // seconds between watch polls (0 = disabled)
	FailSeverity       string  // severity above/equal to which lint fails: error, warn, info
	IssuePattern       string  // optional regex for validating issue reference keys
	SchemaVersion      string  // optional schema version override (default "2")
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		WikiDir:            "wiki",
		DefaultDiff:        "main...HEAD",
		LogLines:           10,
		DuplicateThreshold: 0.7,
		StaleDays:          30,
		ContextSummarize:   false,
		ContextMaxPages:    100,
		ContextMaxBytes:    65536,
		WatchInterval:      0, // disabled by default
		FailSeverity:       "warn",
		IssuePattern:       schema.DefaultIssuePattern,
		SchemaVersion:      schema.SchemaVersion,
		Ignore: []string{
			"wiki/",
			"bin/",
			"*.log",
			"*.tmp",
		},
	}
}

// Load reads and validates a .wikirc file from the given directory. If the file
// does not exist, it returns DefaultConfig with no error. Invalid path settings
// such as an escaping wiki_dir return an error instead of silently accepting
// an unsafe value.
func Load(dir string) (*Config, error) {
	path := filepath.Join(dir, ".wikirc")
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultConfig(), nil
		}
		return nil, err
	}
	defer func() { _ = f.Close() }()

	cfg := DefaultConfig()
	var inIgnore bool
	hasIgnore := false

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip comments and blank lines.
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// End of ignore array.
		if inIgnore {
			if line == "]" {
				inIgnore = false
				continue
			}
			// Strip quotes, trailing comma, and a ] that ends the entry
			// list on the same line (e.g. `"wiki/"]`).
			val := strings.TrimRight(line, ",")
			val = strings.TrimSuffix(val, "]")
			val = strings.Trim(val, `"`)
			val = strings.TrimSpace(val)
			if val != "" {
				cfg.Ignore = append(cfg.Ignore, val)
			}
			continue
		}

		// Start of ignore array.
		if strings.HasPrefix(line, "ignore") && strings.Contains(line, "[") {
			if !hasIgnore {
				cfg.Ignore = nil
				hasIgnore = true
			}
			// Single-line form: ignore = ["wiki/", "bin/"]
			open := strings.Index(line, "[")
			closeIdx := strings.Index(line, "]")
			if closeIdx > open {
				for _, entry := range strings.Split(line[open+1:closeIdx], ",") {
					entry = strings.Trim(strings.TrimSpace(entry), `"`)
					if entry != "" {
						cfg.Ignore = append(cfg.Ignore, entry)
					}
				}
				continue
			}
			inIgnore = true
			continue
		}

		// Key = value pairs.
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		// Strip inline comments outside quotes.
		if !strings.HasPrefix(val, `"`) && !strings.HasPrefix(val, `'`) {
			if idx := strings.Index(val, "#"); idx != -1 && (idx == 0 || val[idx-1] == ' ' || val[idx-1] == '\t') {
				val = strings.TrimSpace(val[:idx])
			}
		}
		val = strings.Trim(val, `"`)

		switch key {
		case "wiki_dir":
			cfg.WikiDir = val
		case "default_diff":
			cfg.DefaultDiff = val
		case "base_ref":
			cfg.BaseRef = val
		case "log_lines":
			cfg.LogLines = parseLogLines(val)
		case "duplicate_threshold":
			cfg.DuplicateThreshold = parseFloat(val, 0.7)
		case "stale_days":
			cfg.StaleDays = parseInt(val, 30)
		case "context_summarize":
			cfg.ContextSummarize = parseBool(val, false)
		case "context_max_pages":
			cfg.ContextMaxPages = parseNonNegativeInt(val, 100)
		case "context_max_bytes":
			cfg.ContextMaxBytes = parseNonNegativeInt(val, 65536)
		case "watch_interval":
			cfg.WatchInterval = ParsePositiveInt(val, 0)
		case "fail_severity":
			v := strings.ToLower(strings.TrimSpace(val))
			switch v {
			case "error", "warn", "info":
				cfg.FailSeverity = v
			default:
				fmt.Fprintf(os.Stderr, "warning: invalid fail_severity %q (expected error, warn, or info); using %q\n", val, cfg.FailSeverity)
			}
		case "issue_pattern":
			if val != "" {
				cfg.IssuePattern = val
			}
		case "schema_version":
			if val != "" {
				cfg.SchemaVersion = val
			}
		default:
			fmt.Fprintf(os.Stderr, "warning: unknown .wikirc key %q ignored\n", key)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if err := cfg.Validate(dir); err != nil {
		return nil, err
	}
	return cfg, nil
}

func parseLogLines(s string) int {
	return ParsePositiveInt(s, 10)
}

// ParsePositiveInt parses a positive integer, returning fallback for
// invalid, zero, or negative input. Zero is deliberately treated as
// invalid here; callers that want zero (e.g. watch_interval) pass 0
// as the fallback so "0" maps to 0.
func ParsePositiveInt(s string, fallback int) int {
	if strings.HasPrefix(strings.TrimSpace(s), "-") {
		return fallback
	}
	n := parseInt(s, fallback)
	if n <= 0 {
		return fallback
	}
	return n
}

func parseNonNegativeInt(s string, fallback int) int {
	if strings.HasPrefix(strings.TrimSpace(s), "-") {
		return fallback
	}
	n := parseInt(s, fallback)
	if n < 0 {
		return fallback
	}
	return n
}

func parseInt(s string, fallback int) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return fallback
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: invalid integer value %q; using %d\n", s, fallback)
		return fallback
	}
	return n
}

func parseFloat(s string, fallback float64) float64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return fallback
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil || f < 0 || f > 1 {
		fmt.Fprintf(os.Stderr, "warning: invalid value %q; using %v\n", s, fallback)
		return fallback
	}
	return f
}

func parseBool(s string, fallback bool) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "true", "yes", "1":
		return true
	case "false", "no", "0":
		return false
	default:
		return fallback
	}
}
