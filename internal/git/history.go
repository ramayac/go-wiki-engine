package git

import (
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var shortDateRe = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

var commitSHARe = regexp.MustCompile(`^[0-9a-fA-F]{7,40}$`)

// PageCommitDates returns the most recent commit date of every file under
// wikiDir in a single `git log` invocation, keyed by repo-relative slash path.
func PageCommitDates(repoRoot, wikiDir string) (map[string]time.Time, error) {
	// -z keeps non-ASCII names unquoted. --relative returns paths relative to
	// repoRoot, which may be a subdirectory of the git work tree.
	out, err := gitOutput(repoRoot, "log", "-z", "--relative", "--date=short", "--name-only", "--format=%x01%cd", "--", wikiDir+"/")
	if err != nil {
		return nil, err
	}

	// Output: "\x01DATE\x00\nfile1\x00file2\x00\x01DATE\x00\nfile3\x00..."
	dates := make(map[string]time.Time)
	var current time.Time
	afterDate := false
	for _, tok := range strings.Split(string(out), "\x00") {
		if strings.HasPrefix(tok, "\x01") {
			current = time.Time{}
			if shortDateRe.MatchString(tok[1:]) {
				if t, err := time.Parse("2006-01-02", tok[1:]); err == nil {
					current = t
				}
			}
			afterDate = true
			continue
		}
		if afterDate {
			tok = strings.TrimPrefix(tok, "\n")
			afterDate = false
		}
		if tok == "" {
			continue
		}
		key := filepath.ToSlash(tok)
		if _, exists := dates[key]; !exists && !current.IsZero() {
			dates[key] = current
		}
	}
	return dates, nil
}

// CommitExists reports whether sha resolves to a valid commit object in repoRoot.
func CommitExists(repoRoot, sha string) bool {
	if !commitSHARe.MatchString(sha) {
		return false
	}
	out, err := gitOutput(repoRoot, "cat-file", "-t", "--", sha)
	return err == nil && strings.TrimSpace(string(out)) == "commit"
}
