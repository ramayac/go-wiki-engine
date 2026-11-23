package engine

import (
	"strings"
	"testing"
)

const fencedPage = "---\nid: deploy\ntitle: Deploy Guide\ntype: runbook\nlifecycle: current\ndescription: Deploy\n---\n# Deploy Guide\n\n## Real Section\n\nIntro text.\n\n%s\n# frobnicate widgets\nmake frobnicate\n%s\n\nTail text.\n"

func rankedFor(t *testing.T, open, close, query string) RankedSearchResult {
	t.Helper()
	root := setupWiki(t)
	page := strings.Replace(strings.Replace(fencedPage, "%s", open, 1), "%s", close, 1)
	writeTestWiki(t, root, map[string]string{"wiki/deploy.md": page})
	res, err := newTestEngine(root).SearchRanked(query, SearchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range res {
		if r.ID == "deploy" {
			return r
		}
	}
	t.Fatalf("deploy page not found for %q", query)
	return RankedSearchResult{}
}

func TestSearchRankedIgnoresFencedComments(t *testing.T) {
	cases := map[string][2]string{
		"backticks": {"```bash", "```"},
		"tildes":    {"~~~", "~~~"},
		"long":      {"````", "````"},
	}
	for name, f := range cases {
		t.Run(name, func(t *testing.T) {
			r := rankedFor(t, f[0], f[1], "frobnicate")
			if r.Section != "Real Section" {
				t.Errorf("Section = %q, want %q", r.Section, "Real Section")
			}
			if !strings.Contains(r.Why, "0 heading") {
				t.Errorf("Why = %q, want 0 heading hits", r.Why)
			}
		})
	}
}

func TestSearchRankedCountsRealHeadings(t *testing.T) {
	root := setupWiki(t)
	writeTestWiki(t, root, map[string]string{
		"wiki/deploy.md": "---\nid: deploy\ntitle: Deploy Guide\ntype: runbook\nlifecycle: current\ndescription: Deploy\n---\n# Deploy Guide\n\n## Frobnicate widgets\n\nText.\n\n```bash\n# unrelated comment\n```\n",
	})
	res, err := newTestEngine(root).SearchRanked("frobnicate", SearchOptions{})
	if err != nil || len(res) == 0 {
		t.Fatalf("search failed: %v %v", res, err)
	}
	if res[0].Section != "Frobnicate widgets" || !strings.Contains(res[0].Why, "1 heading") {
		t.Errorf("real heading not counted: section=%q why=%q", res[0].Section, res[0].Why)
	}
}

func TestHeadingsIgnoreTildeFences(t *testing.T) {
	root := setupWiki(t)
	writeTestWiki(t, root, map[string]string{
		"wiki/deploy.md": "---\nid: deploy\ntype: runbook\nlifecycle: current\ndescription: Deploy\n---\n# Deploy\n\n~~~\n# not a heading\n~~~\n\n## After\n",
	})
	entries, err := newTestEngine(root).Headings()
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, e := range entries {
		if strings.HasSuffix(e.File, "deploy.md") {
			got = append(got, e.Heading)
		}
	}
	if strings.Join(got, "|") != "# Deploy|## After" {
		t.Errorf("headings = %v", got)
	}
}
