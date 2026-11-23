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
