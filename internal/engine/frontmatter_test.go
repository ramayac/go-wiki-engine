package engine

import (
	"reflect"
	"testing"
)

func TestParseFrontMatter(t *testing.T) {
	tests := []struct {
		name          string
		content       string
		wantFM        FrontMatter
		wantFound     bool
		wantErrSubstr string
	}{
		{
			name: "valid front matter with status and description",
			content: `---
status: current
description: "Overview of the wiki structure"
superseded_by: ""
tags: [architecture, config]
---
# Page Title`,
			wantFM: FrontMatter{
				Status:       "current",
				Description:  "Overview of the wiki structure",
				SupersededBy: "",
				Tags:         []string{"architecture", "config"},
			},
			wantFound: true,
		},
		{
			name: "no front matter",
			content: `# Page Title
No front matter here.`,
			wantFM: FrontMatter{
				Status: "current",
			},
			wantFound: false,
		},
		{
			name: "unterminated front matter",
			content: `---
status: legacy
description: unterminated
# Page Title`,
			wantFM: FrontMatter{
				Status: "current",
			},
			wantFound:     true,
			wantErrSubstr: "unterminated front matter block",
		},
		{
			name: "quotes in fields",
			content: `---
status: 'deprecated'
description: "something 'special'"
superseded_by: "new-page.md"
created: 2026-05-28
updated: 2026-06-10
---
body`,
			wantFM: FrontMatter{
				Status:       "deprecated",
				Description:  "something 'special'",
				SupersededBy: "new-page.md",
				Created:      "2026-05-28",
				Updated:      "2026-06-10",
			},
			wantFound: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotFM, gotFound, err := ParseFrontMatter(tt.content)
			if err != nil {
				if tt.wantErrSubstr == "" {
					t.Fatalf("unexpected error: %v", err)
				}
				if !reflect.DeepEqual(gotFM, tt.wantFM) {
					t.Errorf("gotFM = %+v, want %+v", gotFM, tt.wantFM)
				}
				return
			}
			if tt.wantErrSubstr != "" {
				t.Fatalf("expected error containing %q, got nil", tt.wantErrSubstr)
			}
			if gotFound != tt.wantFound {
				t.Errorf("gotFound = %v, want %v", gotFound, tt.wantFound)
			}
			if !reflect.DeepEqual(gotFM, tt.wantFM) {
				t.Errorf("gotFM = %+v, want %+v", gotFM, tt.wantFM)
			}
		})
	}
}

func TestParseFrontMatterHashInValue(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{"hash inside quotes survives", "---\ndescription: \"C# guide\"\n---\n", "C# guide"},
		{"hash inside single quotes survives", "---\ndescription: 'base # comment'\n---\n", "base # comment"},
		{"hash after whitespace in unquoted value is a comment", "---\ndescription: base # comment\n---\n", "base"},
		{"hash mid-word in unquoted value survives", "---\ndescription: C#guide\n---\n", "C#guide"},
		{"status with trailing comment", "---\nstatus: current # active\n---\n", "current"},
	}
	for _, tt := range tests {
		fm, found, err := ParseFrontMatter(tt.content)
		if err != nil {
			t.Errorf("%s: ParseFrontMatter error: %v", tt.name, err)
			continue
		}
		if !found {
			t.Errorf("%s: front matter not found", tt.name)
			continue
		}
		if tt.name == "status with trailing comment" {
			if fm.Status != tt.want {
				t.Errorf("%s: status = %q, want %q", tt.name, fm.Status, tt.want)
			}
			continue
		}
		if fm.Description != tt.want {
			t.Errorf("%s: description = %q, want %q", tt.name, fm.Description, tt.want)
		}
	}
}
