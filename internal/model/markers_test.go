package model

import "testing"

func TestCountMarkers(t *testing.T) {
	cases := []struct {
		name          string
		text          string
		contradiction int
		rationale     int
	}{
		{"none", "# Title\n\nPlain text.\n", 0, 0},
		{"contradiction with sign", "> ⚠️ CONTRADICTION: README says X.\n", 1, 0},
		{"contradiction without sign", "> CONTRADICTION: README says X.\n", 1, 0},
		{"contradiction sign without selector", "> ⚠ CONTRADICTION: README says X.\n", 1, 0},
		{"rationale", "> Rationale: not recorded.\n", 0, 1},
		{"rationale case-insensitive", "> rationale: Not Recorded.\n", 0, 1},
		{"both counted once each", "> ⚠️ CONTRADICTION: a\n\n> Rationale: not recorded.\n\n> ⚠️ CONTRADICTION: b\n", 2, 1},
		{"not a quote", "Rationale: not recorded.\n", 0, 0},
		{"mid-line is not a marker", "Write > Rationale: not recorded. on its own line.\n", 0, 0},
		{"fenced example", "```markdown\n> ⚠️ CONTRADICTION: example\n> Rationale: not recorded.\n```\n", 0, 0},
		{"inline code example", "Use `> Rationale: not recorded.` as the marker.\n", 0, 0},
		{"tilde fence example", "~~~\n> Rationale: not recorded.\n~~~\n", 0, 0},
		{"after a closed fence", "```\ncode\n```\n> Rationale: not recorded.\n", 0, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, r := CountMarkers(tc.text)
			if c != tc.contradiction || r != tc.rationale {
				t.Errorf("CountMarkers = (%d, %d), want (%d, %d)", c, r, tc.contradiction, tc.rationale)
			}
		})
	}
}

func TestIsOpenMarkerLine(t *testing.T) {
	for line, want := range map[string]bool{
		"> Rationale: not recorded.":             true,
		"> ⚠️ CONTRADICTION: docs say TODO: fix": true,
		"TODO: write this":                       false,
		"> Rationale: the cache was too slow.":   false,
	} {
		if got := IsOpenMarkerLine(line); got != want {
			t.Errorf("IsOpenMarkerLine(%q) = %v, want %v", line, got, want)
		}
	}
}
