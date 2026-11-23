package model

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/ramayac/go-wiki-engine/internal/schema"
)

func TestParseFrontMatterYAML(t *testing.T) {
	t.Run("missing front matter does not default to active", func(t *testing.T) {
		res := ParseFrontMatter("# Title\nBody text\n")
		if res.State != FrontMatterMissing {
			t.Fatalf("State = %v, want FrontMatterMissing", res.State)
		}
		if res.Err != nil {
			t.Fatalf("unexpected error: %v", res.Err)
		}
		if res.Metadata.EffectiveLifecycle() != "" {
			t.Errorf("EffectiveLifecycle = %q, want empty", res.Metadata.EffectiveLifecycle())
		}
		if res.Metadata.IsActive() {
			t.Error("page without front matter must not become active")
		}
	})

	t.Run("missing status and lifecycle does not default to active", func(t *testing.T) {
		res := ParseFrontMatter("---\ndescription: \"No lifecycle\"\n---\n# Title\n")
		if res.State != FrontMatterPresent {
			t.Fatalf("State = %v, want FrontMatterPresent", res.State)
		}
		if res.Metadata.Lifecycle != "" || res.Metadata.Status != "" {
			t.Errorf("expected empty lifecycle/status, got %q / %q", res.Metadata.Lifecycle, res.Metadata.Status)
		}
		if res.Metadata.IsActive() {
			t.Error("page with missing lifecycle/status must not become active")
		}
	})

	t.Run("multiline YAML values, multiline lists, quoted commas, escaped quotes, unknown fields", func(t *testing.T) {
		content := "---\n" +
			"id: architecture\n" +
			"title: \"Architecture, Core & \\\"Edges\\\"\"\n" +
			"type: concept\n" +
			"lifecycle: current\n" +
			"description: >\n" +
			"  A multiline description\n" +
			"  with folded lines.\n" +
			"tags:\n" +
			"  - core\n" +
			"  - \"alpha, beta\"\n" +
			"related:\n" +
			"  - index\n" +
			"sources:\n" +
			"  - file: internal/engine/graph.go\n" +
			"    repo: go-wiki-engine\n" +
			"references:\n" +
			"  - type: external\n" +
			"    value: https://example.com/design\n" +
			"  - type: issue\n" +
			"    value: PROJ-123\n" +
			"custom_owner: platform-team\n" +
			"---\n" +
			"# Architecture\n"

		res := ParseFrontMatter(content)
		if res.Err != nil {
			t.Fatalf("unexpected error: %v", res.Err)
		}
		m := res.Metadata
		if m.ID != "architecture" || m.Type != "concept" || m.Lifecycle != "current" {
			t.Errorf("core fields = %+v", m)
		}
		if m.Title != `Architecture, Core & "Edges"` {
			t.Errorf("Title = %q", m.Title)
		}
		if !strings.Contains(m.Description, "multiline description") {
			t.Errorf("Description = %q", m.Description)
		}
		if !reflect.DeepEqual(m.Tags, []string{"core", "alpha, beta"}) {
			t.Errorf("Tags = %#v", m.Tags)
		}
		if len(m.Sources) != 1 || m.Sources[0].File != "internal/engine/graph.go" || m.Sources[0].Repo != "go-wiki-engine" {
			t.Errorf("Sources = %+v", m.Sources)
		}
		if len(m.References) != 2 || m.References[0].Type != "external" || m.References[1].Value != "PROJ-123" {
			t.Errorf("References = %+v", m.References)
		}
		if m.UnknownFields["custom_owner"] != "platform-team" {
			t.Errorf("UnknownFields = %+v", m.UnknownFields)
		}
		if m.FieldLines["lifecycle"] != 5 {
			t.Errorf("FieldLines[lifecycle] = %d, want 5", m.FieldLines["lifecycle"])
		}
	})

	t.Run("CRLF line endings", func(t *testing.T) {
		content := "---\r\nid: crlf-page\r\nlifecycle: current\r\ndescription: \"CRLF test\"\r\n---\r\n# Title\r\n"
		res := ParseFrontMatter(content)
		if res.Err != nil {
			t.Fatalf("CRLF parse error: %v", res.Err)
		}
		if res.Metadata.ID != "crlf-page" || res.Metadata.Lifecycle != "current" {
			t.Errorf("CRLF metadata = %+v", res.Metadata)
		}
	})

	t.Run("unterminated front matter", func(t *testing.T) {
		res := ParseFrontMatter("---\nlifecycle: current\n# No closing delimiter\n")
		if res.State != FrontMatterUnterminated || res.Err == nil {
			t.Fatalf("expected FrontMatterUnterminated, got state=%v err=%v", res.State, res.Err)
		}
	})

	t.Run("invalid YAML syntax", func(t *testing.T) {
		res := ParseFrontMatter("---\nlifecycle: [unclosed\n---\n# Title\n")
		if res.State != FrontMatterInvalidYAML || res.Err == nil {
			t.Fatalf("expected FrontMatterInvalidYAML, got state=%v err=%v", res.State, res.Err)
		}
	})

	t.Run("YAML scalar type errors", func(t *testing.T) {
		cases := []string{
			"---\nlifecycle: 123\n---\n# T\n",
			"---\nlifecycle: [current]\n---\n# T\n",
			"---\ntags: not-a-list\n---\n# T\n",
			"---\ndescription: true\n---\n# T\n",
			"---\nsources: 42\n---\n# T\n",
		}
		for _, c := range cases {
			res := ParseFrontMatter(c)
			if res.State != FrontMatterInvalidFieldType || res.Err == nil {
				t.Errorf("case %q: expected FrontMatterInvalidFieldType, got state=%v err=%v", c, res.State, res.Err)
			}
			var fmErr *FrontMatterError
			if !errors.As(res.Err, &fmErr) {
				t.Errorf("case %q: expected *FrontMatterError, got %T", c, res.Err)
			}
		}
	})

	t.Run("legacy status and inline references compatibility", func(t *testing.T) {
		content := "---\nstatus: current\ndescription: \"Legacy\"\nreferences: [source:main.go, external:https://example.com, issue:JIRA-42]\n---\n# Legacy\n"
		res := ParseFrontMatter(content)
		if res.Err != nil {
			t.Fatalf("unexpected error: %v", res.Err)
		}
		if res.Metadata.EffectiveLifecycle() != "current" || !res.Metadata.LegacyStatus {
			t.Errorf("legacy status mapping failed: lifecycle=%q legacy=%v", res.Metadata.EffectiveLifecycle(), res.Metadata.LegacyStatus)
		}
		if !res.Metadata.LegacyRefs {
			t.Error("expected LegacyRefs=true")
		}
		sources := res.Metadata.AllSources()
		if len(sources) != 1 || sources[0].File != "main.go" {
			t.Errorf("AllSources = %+v, want [main.go]", sources)
		}
	})

	t.Run("decision workflow status does not masquerade as lifecycle", func(t *testing.T) {
		content := "---\nid: adr-1\ntype: decision\nstatus: accepted\ndescription: \"Decision\"\n---\n# ADR 1\n"
		res := ParseFrontMatter(content)
		if res.Err != nil {
			t.Fatalf("unexpected error: %v", res.Err)
		}
		if res.Metadata.EffectiveLifecycle() != "" {
			t.Errorf("decision without lifecycle must not infer lifecycle from status, got %q", res.Metadata.EffectiveLifecycle())
		}
	})
}

func TestSchemaTemplatesMatchTypedModel(t *testing.T) {
	doc := schema.Get()
	for _, ts := range doc.Types {
		res := ParseFrontMatter(ts.Template)
		if res.Err != nil {
			t.Errorf("template for type %q failed to parse: %v", ts.Name, res.Err)
			continue
		}
		m := res.Metadata
		if m.Type != ts.Name {
			t.Errorf("template %q has type %q", ts.Name, m.Type)
		}
		if m.ID == "" || m.Title == "" || m.Lifecycle == "" || m.Description == "" {
			t.Errorf("template %q missing core required fields: %+v", ts.Name, m)
		}
		if ts.Name == "decision" && !schema.IsValidDecisionStatus(m.Status) {
			t.Errorf("decision template status %q invalid", m.Status)
		}
		if ts.Name == "incident" && (!schema.IsValidIncidentStatus(m.Status) || m.Severity == "") {
			t.Errorf("incident template status=%q severity=%q invalid", m.Status, m.Severity)
		}
	}
}

func TestResolveSourceAndDirectoryCoverage(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "internal", "engine"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "internal", "engine", "graph.go"), []byte("package engine\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Exact file match.
	rs := ResolveSource(root, filepath.Join(root, "wiki"), SourceReference{File: "internal/engine/graph.go"})
	if rs.Err != nil || !rs.Exists || rs.CrossRepo {
		t.Fatalf("exact source failed: %+v", rs)
	}
	if reason, ok := rs.MatchesFile("internal/engine/graph.go"); !ok || reason != "source" {
		t.Errorf("MatchesFile exact = (%q, %v), want (source, true)", reason, ok)
	}

	// Directory coverage.
	rsDir := ResolveSource(root, filepath.Join(root, "wiki"), SourceReference{File: "internal/engine"})
	if rsDir.Err != nil || !rsDir.Exists || !rsDir.IsDir {
		t.Fatalf("dir source failed: %+v", rsDir)
	}
	if reason, ok := rsDir.MatchesFile("internal/engine/graph.go"); !ok || reason != "directory" {
		t.Errorf("MatchesFile dir = (%q, %v), want (directory, true)", reason, ok)
	}

	// Cross-repo source never matches local files.
	rsCross := ResolveSource(root, filepath.Join(root, "wiki"), SourceReference{File: "internal/engine/graph.go", Repo: "other-repo"})
	if !rsCross.CrossRepo {
		t.Error("expected CrossRepo=true")
	}
	if _, ok := rsCross.MatchesFile("internal/engine/graph.go"); ok {
		t.Error("cross-repo source must not match local files")
	}
}

func TestSnapshotSingleReadAndLookup(t *testing.T) {
	root := t.TempDir()
	wikiDir := filepath.Join(root, "wiki")
	if err := os.MkdirAll(filepath.Join(wikiDir, "prologue"), 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"wiki/index.md":           "---\nid: index\nlifecycle: current\ndescription: Index\n---\n# Index\n- [schema](prologue/schema.md) | Schema\n",
		"wiki/prologue/schema.md": "---\nid: schema\nlifecycle: current\ndescription: Schema\n---\n# Schema\n",
	}
	for rel, content := range files {
		if err := os.WriteFile(filepath.Join(root, rel), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	snap, err := LoadSnapshot(root, "wiki")
	if err != nil {
		t.Fatalf("LoadSnapshot failed: %v", err)
	}
	for path, count := range snap.ReadCounts {
		if count != 1 {
			t.Errorf("file %s read %d times, want 1", path, count)
		}
	}
	// Deterministic lookup by wiki-rel, repo-rel, and ID.
	for _, query := range []string{"prologue/schema.md", "wiki/prologue/schema.md", "schema"} {
		p, err := snap.Lookup(query)
		if err != nil || p.WikiRel != "prologue/schema.md" {
			t.Errorf("Lookup(%q) = (%v, %v), want prologue/schema.md", query, p, err)
		}
	}
}

func TestParseFrontMatterStripsBOM(t *testing.T) {
	res := ParseFrontMatter("\ufeff---\nid: a\ndescription: x\n---\n# A\n")
	if res.State != FrontMatterPresent || res.Metadata.ID != "a" {
		t.Fatalf("BOM page: state=%v id=%q", res.State, res.Metadata.ID)
	}
	if res := ParseFrontMatter("\ufeff# Title\n"); res.State != FrontMatterMissing {
		t.Errorf("BOM page without front matter: state=%v", res.State)
	}
}

func TestParsePageRecordsEncoding(t *testing.T) {
	parse := func(raw string) *Page {
		return ParsePage("/r", "/r/wiki", "wiki", "a.md", "/r/wiki/a.md", []byte(raw), time.Time{})
	}
	p := parse("\ufeff---\r\nid: a\r\n---\r\n# A\r\n")
	if !p.HasBOM || !p.CRLF || p.FMState != FrontMatterPresent || strings.ContainsAny(p.RawContent, "\r\ufeff") {
		t.Errorf("BOM+CRLF page: %+v", p)
	}
	p = parse("---\nid: a\n---\n# A\n")
	if p.HasBOM || p.CRLF {
		t.Errorf("LF page flagged: bom=%v crlf=%v", p.HasBOM, p.CRLF)
	}
	// Majority rule: two LF lines against one CRLF line stay LF.
	if p = parse("a\r\nb\nc\n"); p.CRLF {
		t.Error("mostly-LF page flagged as CRLF")
	}
}

func TestFenceTracker(t *testing.T) {
	var f FenceTracker
	step := func(line string) (skip, in bool) {
		skip = f.Skip(line)
		return skip, f.InFence()
	}
	for _, c := range []struct {
		line     string
		skip, in bool
	}{
		{"text", false, false},
		{"~~~", true, true},
		{"```", false, true}, // other fence char does not close
		{"# in tilde fence", false, true},
		{"~~~", true, false},
		{"````sh", true, true},
		{"```", false, true}, // shorter fence does not close
		{"````", true, false},
		{"```a`b", false, false}, // backtick in info string is not a fence
		{"# heading", false, false},
	} {
		if skip, in := step(c.line); skip != c.skip || in != c.in {
			t.Errorf("%q: skip=%v in=%v, want %v %v", c.line, skip, in, c.skip, c.in)
		}
	}
}
