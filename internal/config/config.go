// Package config loads and parses the .wikirc configuration file.
package config

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// Config holds the parsed .wikirc settings.
type Config struct {
	WikiDir            string
	DefaultDiff        string
	LogLines           int
	Ignore             []string
	DuplicateThreshold float64 // 0.0-1.0, similarity above which pages are flagged as duplicates
	StaleDays          int     // days after which an unchanged wiki page is flagged as stale
	ContextSummarize   bool    // enable --summarize on wiki-engine context
	WatchInterval      int     // seconds between watch polls (0 = disabled)
	FailSeverity       string  // severity above/equal to which lint fails: error, warn, info
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
		WatchInterval:      0, // disabled by default
		FailSeverity:       "warn",
		Ignore: []string{
			"wiki/",
			"bin/",
			"*.log",
			"*.tmp",
		},
	}
}

// Load reads a .wikirc file from the given directory. If the file does not
// exist, it returns DefaultConfig with no error.
func Load(dir string) (*Config, error) {
	path := filepath.Join(dir, ".wikirc")
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultConfig(), nil
		}
		return nil, err
	}
	defer f.Close()

	cfg := DefaultConfig()
	var inIgnore bool
	cfg.Ignore = nil // reset to collect from file

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
			// Strip quotes and trailing comma.
			val := strings.TrimRight(line, ",")
			val = strings.Trim(val, `"`)
			val = strings.TrimSpace(val)
			if val != "" {
				cfg.Ignore = append(cfg.Ignore, val)
			}
			continue
		}

		// Start of ignore array.
		if strings.HasPrefix(line, "ignore") && strings.Contains(line, "[") {
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
		val = strings.Trim(val, `"`)

		switch key {
		case "wiki_dir":
			cfg.WikiDir = val
		case "default_diff":
			cfg.DefaultDiff = val
		case "log_lines":
			cfg.LogLines = parseLogLines(val)
		case "duplicate_threshold":
			cfg.DuplicateThreshold = parseFloat(val, 0.7)
		case "stale_days":
			cfg.StaleDays = parseInt(val, 30)
		case "context_summarize":
			cfg.ContextSummarize = parseBool(val, false)
		case "watch_interval":
			cfg.WatchInterval = ParsePositiveInt(val, 0)
		case "fail_severity":
			cfg.FailSeverity = strings.ToLower(strings.TrimSpace(val))
		}
	}
	return cfg, scanner.Err()
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

func parseInt(s string, fallback int) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return fallback
	}
	if s == "0" {
		return 0
	}
	n := 0
	hasDigit := false
	for _, c := range s {
		if c >= '0' && c <= '9' {
			n = n*10 + int(c-'0')
			hasDigit = true
		}
	}
	if !hasDigit {
		return fallback
	}
	return n
}

func parseFloat(s string, fallback float64) float64 {
	// Simple parser: extract digits and one decimal point.
	var result float64
	decimal := false
	divisor := 1.0
	hasDigit := false
	for _, c := range s {
		if c == '.' && !decimal {
			decimal = true
			continue
		}
		if c >= '0' && c <= '9' {
			hasDigit = true
			d := float64(c - '0')
			if decimal {
				divisor *= 10
				result += d / divisor
			} else {
				result = result*10 + d
			}
		}
	}
	if !hasDigit {
		return fallback
	}
	if result < 0 || result > 1 {
		return fallback
	}
	return result
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
