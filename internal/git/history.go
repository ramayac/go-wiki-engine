package git

import (
	"bufio"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var shortDateRe = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

// PageCommitDates returns the most recent commit date of every file under
// wikiDir in a single `git log` invocation, keyed by repo-relative slash path.
func PageCommitDates(repoRoot, wikiDir string) (map[string]time.Time, error) {
	cmd := exec.Command("git", "log", "--date=short", "--name-only", "--format=%cd", "--", wikiDir+"/")
	cmd.Dir = repoRoot
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	dates := make(map[string]time.Time)
	var current time.Time
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if shortDateRe.MatchString(line) {
			if t, err := time.Parse("2006-01-02", line); err == nil {
				current = t
			}
			continue
		}
		key := filepath.ToSlash(line)
		if _, exists := dates[key]; !exists && !current.IsZero() {
			dates[key] = current
		}
	}
	return dates, scanner.Err()
}

// CommitExists reports whether sha resolves to a valid commit object in repoRoot.
func CommitExists(repoRoot, sha string) bool {
	if len(sha) < 7 {
		return false
	}
	cmd := exec.Command("git", "cat-file", "-t", sha)
	cmd.Dir = repoRoot
	out, err := cmd.Output()
	return err == nil && strings.TrimSpace(string(out)) == "commit"
}
