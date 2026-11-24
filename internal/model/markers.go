package model

import (
	"regexp"
	"strings"
)

var (
	// contradictionMarker matches the `> ⚠️ CONTRADICTION:` flag from the schema
	// conventions. The warning sign and its variation selector are optional,
	// because writers and editors drop them.
	contradictionMarker = regexp.MustCompile(`^\s*>\s*(?:⚠\x{FE0F}?\s*)?CONTRADICTION:`)
	// rationaleMarker matches `> Rationale: not recorded.`, which a page carries
	// when no source stated why the thing works the way it does.
	rationaleMarker = regexp.MustCompile(`(?i)^\s*>\s*rationale:\s*not recorded\.`)
	inlineCodeSpan  = regexp.MustCompile("`[^`\n]+`")
)

// IsOpenMarkerLine reports whether line is an open CONTRADICTION flag or a
// missing-rationale marker. It does not look at code context.
func IsOpenMarkerLine(line string) bool {
	return contradictionMarker.MatchString(line) || rationaleMarker.MatchString(line)
}

// CountMarkers counts the open CONTRADICTION flags and missing-rationale
// markers in text. A marker inside fenced or inline code is an example, not a
// flag, so it does not count.
func CountMarkers(text string) (contradictions, rationaleNotRecorded int) {
	var fence FenceTracker
	for _, line := range strings.Split(text, "\n") {
		if fence.Skip(line) || fence.InFence() {
			continue
		}
		line = inlineCodeSpan.ReplaceAllString(line, " ")
		switch {
		case contradictionMarker.MatchString(line):
			contradictions++
		case rationaleMarker.MatchString(line):
			rationaleNotRecorded++
		}
	}
	return contradictions, rationaleNotRecorded
}
