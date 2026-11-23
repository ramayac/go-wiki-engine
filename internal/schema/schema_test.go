package schema

import (
	"strings"
	"testing"
)

func TestSchemaVocabulary(t *testing.T) {
	doc := Get()
	if doc.SchemaVersion != SchemaVersion {
		t.Errorf("SchemaVersion = %q, want %q", doc.SchemaVersion, SchemaVersion)
	}
	if len(doc.Types) != len(ValidPageTypes) {
		t.Errorf("doc.Types count = %d, want %d", len(doc.Types), len(ValidPageTypes))
	}
	if len(doc.Lifecycles) != len(ValidLifecycles) {
		t.Errorf("doc.Lifecycles count = %d, want %d", len(doc.Lifecycles), len(ValidLifecycles))
	}
	for _, typ := range ValidPageTypes {
		if !IsValidPageType(typ) {
			t.Errorf("IsValidPageType(%q) = false", typ)
		}
		ts, err := TypeByName(typ)
		if err != nil {
			t.Errorf("TypeByName(%q) error: %v", typ, err)
		}
		if ts.Template == "" {
			t.Errorf("type %q missing template", typ)
		}
	}
	if _, err := TypeByName("bogus"); err == nil {
		t.Error("TypeByName(bogus) should error")
	}
}

func TestRenderText(t *testing.T) {
	full, err := RenderText("", "")
	if err != nil {
		t.Fatalf("RenderText full failed: %v", err)
	}
	for _, want := range []string{"# Wiki Schema", "## Lifecycle", "## Page Types", "## Sources", "## Exit Codes"} {
		if !strings.Contains(full, want) {
			t.Errorf("full schema missing %q", want)
		}
	}

	for _, sec := range Sections {
		out, err := RenderText("", sec)
		if err != nil || out == "" {
			t.Errorf("RenderText section %q failed: %v", sec, err)
		}
	}

	for _, typ := range ValidPageTypes {
		out, err := RenderText(typ, "")
		if err != nil || !strings.Contains(out, "type: "+typ) {
			t.Errorf("RenderText type %q missing template header: %v", typ, err)
		}
	}

	if _, err := RenderText("", "nonexistent"); err == nil {
		t.Error("unknown section should error")
	}
}

func TestVocabularySection(t *testing.T) {
	out, err := RenderText("", "vocabulary")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"## Vocabulary", "vocabulary.yaml", "free-form", "controlled", "lint", "bare list", "did you mean"} {
		if !strings.Contains(out, want) {
			t.Errorf("vocabulary section missing %q:\n%s", want, out)
		}
	}
	fm, err := RenderText("", "frontmatter")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(fm, "schema --section vocabulary") {
		t.Errorf("frontmatter section must point at the vocabulary section:\n%s", fm)
	}
}

func TestTemplatesCarryWhyAndStopsApplyingSections(t *testing.T) {
	const marker = "> Rationale: not recorded."
	want := map[string]struct{ why, stops bool }{
		"concept":  {true, true},
		"entity":   {true, true},
		"incident": {false, true},
		"decision": {false, true},
		"runbook":  {false, false},
	}
	for typ, w := range want {
		ts, err := TypeByName(typ)
		if err != nil {
			t.Fatal(err)
		}
		if got := strings.Contains(ts.Template, "\n## Why\n"); got != w.why {
			t.Errorf("%s template has ## Why = %v, want %v", typ, got, w.why)
		}
		if got := strings.Contains(ts.Template, "\n## Stops applying when\n"); got != w.stops {
			t.Errorf("%s template has ## Stops applying when = %v, want %v", typ, got, w.stops)
		}
		if w.stops && !strings.Contains(ts.Template, `"Not known."`) {
			t.Errorf("%s template must say to write \"Not known.\" when the condition is unknown", typ)
		}
		// The marker belongs in the conventions, never inside a template.
		if strings.Contains(ts.Template, marker) {
			t.Errorf("%s template must not contain the rationale marker", typ)
		}
	}
}

func TestConventionsSection(t *testing.T) {
	out, err := RenderText("", "conventions")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"## Conventions",
		"`> Rationale: not recorded.`",
		"on its own line",
		"Do not guess",
		"Do not stop to ask",
		"replace the marker",
		"`> ⚠️ CONTRADICTION: ",
		"ASD-STE100 Simplified Technical English",
		"active voice",
		"secrets, credentials, tokens, session IDs",
		"personal or health identifiers",
		"whole request or response payloads",
		"wiki-engine health",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("conventions section missing %q:\n%s", want, out)
		}
	}
	full, err := RenderText("", "")
	if err != nil || !strings.Contains(full, "## Conventions") {
		t.Errorf("full schema must include the conventions section (err %v)", err)
	}
}
