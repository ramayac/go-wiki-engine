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
	WikiDir     string
	DefaultDiff string
	LogLines    int
	Ignore      []string
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		WikiDir:     "wiki",
		DefaultDiff: "main...HEAD",
		LogLines:    10,
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
		}
	}
	return cfg, scanner.Err()
}

func parseLogLines(s string) int {
	n := 0
	for _, c := range s {
		if c >= '0' && c <= '9' {
			n = n*10 + int(c-'0')
		}
	}
	if n == 0 {
		return 10
	}
	return n
}
