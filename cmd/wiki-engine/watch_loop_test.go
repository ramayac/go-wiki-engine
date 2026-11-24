package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestWatchLoopPrintsOnlyOnChange(t *testing.T) {
	root := brokenFixture(t)
	_, eng := mainTestEngine(t, root)

	ticks := make(chan time.Time, 3)
	for i := 0; i < 3; i++ {
		ticks <- time.Now()
	}
	close(ticks)
	out := captureStdout(t, func() { watchLoop(eng, false, ticks) })
	if got := strings.Count(out, "lint: ISSUES FOUND"); got != 1 {
		t.Errorf("unchanged issues printed %d times, want 1:\n%s", got, out)
	}

	// A different issue set prints again; the same set does not.
	var last string
	out = captureStdout(t, func() {
		watchCycle(eng, false, &last)
		watchCycle(eng, false, &last)
		if err := os.WriteFile(filepath.Join(root, "wiki", "index.md"), []byte(brokenIndex+"- [other.md](other.md)\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		watchCycle(eng, false, &last)
		watchCycle(eng, false, &last)
	})
	if got := strings.Count(out, "lint: ISSUES FOUND"); got != 2 {
		t.Errorf("printed %d times, want 2 (first tick + one change):\n%s", got, out)
	}
}
