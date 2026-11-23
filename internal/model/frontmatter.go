// Package model defines the typed wiki page, frontmatter, source reference,
// single-read snapshot, and graph models shared across all engine and lint operations.
package model

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/ramayac/go-wiki-engine/internal/schema"
	"gopkg.in/yaml.v3"
)

// FrontMatterState classifies the outcome of frontmatter parsing.
type FrontMatterState int

const (
	FrontMatterMissing FrontMatterState = iota
	FrontMatterPresent
	FrontMatterUnterminated
	FrontMatterInvalidYAML
	FrontMatterInvalidFieldType
)

// FrontMatterError describes a structured frontmatter parse failure with line info.
type FrontMatterError struct {
	State   FrontMatterState
	Line    int
	Field   string
	Message string
	Err     error
}

func (e *FrontMatterError) Error() string {
	if e.Line > 0 {
		return fmt.Sprintf("line %d: %s", e.Line, e.Message)
	}
	return e.Message
}

func (e *FrontMatterError) Unwrap() error { return e.Err }

// Reference is a declared non-source cross-reference (external URL or issue key),
// or a legacy inline reference.
type Reference struct {
	Type  string `yaml:"type" json:"type"`
	Value string `yaml:"value" json:"value"`
}

// SourceReference declares an exact repository source file or directory citation.
type SourceReference struct {
	File string `yaml:"file" json:"file"`
	Repo string `yaml:"repo,omitempty" json:"repo,omitempty"`
}

// PageMetadata holds typed metadata parsed from YAML frontmatter.
// Parsing applies NO semantic defaults: missing fields remain empty so that
// missing lifecycle/status never silently becomes active knowledge.
type PageMetadata struct {
	ID            string            `json:"id,omitempty"`
	Title         string            `json:"title,omitempty"`
	Type          string            `json:"type,omitempty"`
	Lifecycle     string            `json:"lifecycle,omitempty"`
	Status        string            `json:"status,omitempty"`
	Severity      string            `json:"severity,omitempty"`
	Description   string            `json:"description,omitempty"`
	Created       string            `json:"created,omitempty"`
	Updated       string            `json:"updated,omitempty"`
	Tags          []string          `json:"tags,omitempty"`
	Related       []string          `json:"related,omitempty"`
	DependsOn     []string          `json:"depends_on,omitempty"`
	SupersededBy  string            `json:"superseded_by,omitempty"`
	Sources       []SourceReference `json:"sources,omitempty"`
	References    []Reference       `json:"references,omitempty"`
	UnknownFields map[string]any    `json:"-"`
	FieldLines    map[string]int    `json:"-"`
	LegacyStatus  bool              `json:"-"`
	LegacyRefs    bool              `json:"-"`
}

// FrontMatterParseResult captures the parsed metadata, state, body slice, and
// any parse error.
type FrontMatterParseResult struct {
	Metadata       PageMetadata
	State          FrontMatterState
	Body           string
	BodyStartLine  int // 1-based line number where Body begins
	FrontMatterRaw string
	Err            error
}

// EffectiveLifecycle returns the page's authority lifecycle.
// If explicit `lifecycle` is set, it is returned directly.
// Otherwise, if `status` holds a v1 legacy lifecycle value (planned|current|legacy|deprecated)
// and the page is not using `status` for decision/incident workflow, it maps via
// schema.MapLegacyStatus.
// If neither is set, returns "" — NEVER defaulting to "current".
func (m PageMetadata) EffectiveLifecycle() string {
	if m.Lifecycle != "" {
		return m.Lifecycle
	}
	if m.Type == "decision" || m.Type == "incident" {
		// On typed decisions/incidents, `status` is workflow status, not authority lifecycle.
		return ""
	}
	if mapped, ok := schema.MapLegacyStatus(m.Status); ok {
		return mapped
	}
	return ""
}

// IsActive reports whether the page's effective lifecycle participates in the
// active wiki graph (current, implemented, wip, planned). Missing/invalid
// lifecycle returns false.
func (m PageMetadata) IsActive() bool {
	return schema.IsActiveLifecycle(m.EffectiveLifecycle())
}

// IsSearchDefault reports whether the page is returned by default search
// (current, implemented, wip).
func (m PageMetadata) IsSearchDefault() bool {
	return schema.IsSearchDefaultLifecycle(m.EffectiveLifecycle())
}

// IsDriftChecked reports whether active-drift and source existence checks
// apply to this page (current, wip, planned).
func (m PageMetadata) IsDriftChecked() bool {
	return schema.IsDriftCheckedLifecycle(m.EffectiveLifecycle())
}

// AllSources returns the combined list of structured `sources` and legacy
// `references` entries with type "source", deduplicated by (repo, file).
func (m PageMetadata) AllSources() []SourceReference {
	var out []SourceReference
	seen := make(map[SourceReference]bool)
	for _, s := range m.Sources {
		if s.File == "" {
			continue
		}
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	for _, r := range m.References {
		if r.Type == "source" && r.Value != "" {
			s := SourceReference{File: r.Value}
			if !seen[s] {
				seen[s] = true
				out = append(out, s)
			}
		}
	}
	return out
}

// ParseFrontMatter parses a Markdown file's YAML frontmatter block using
// gopkg.in/yaml.v3. It supports both LF and CRLF line endings, preserves field
// line numbers and unknown keys, applies no semantic defaults, and distinguishes
// missing, unterminated, invalid YAML, and scalar type errors.
func ParseFrontMatter(content string) FrontMatterParseResult {
	// Normalize CRLF to LF for deterministic line tracking while preserving content.
	norm := strings.ReplaceAll(content, "\r\n", "\n")
	lines := strings.Split(norm, "\n")

	startIdx := -1
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if trimmed == "---" {
			startIdx = i
		}
		break
	}

	if startIdx == -1 {
		return FrontMatterParseResult{
			State:         FrontMatterMissing,
			Body:          norm,
			BodyStartLine: 1,
		}
	}

	endIdx := -1
	for i := startIdx + 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			endIdx = i
			break
		}
	}

	if endIdx == -1 {
		return FrontMatterParseResult{
			State: FrontMatterUnterminated,
			Body:  "",
			Err: &FrontMatterError{
				State:   FrontMatterUnterminated,
				Line:    startIdx + 1,
				Message: "unterminated front matter block",
			},
		}
	}

	fmLines := lines[startIdx+1 : endIdx]
	fmRaw := strings.Join(fmLines, "\n")
	bodyLines := lines[endIdx+1:]
	body := strings.Join(bodyLines, "\n")
	bodyStartLine := endIdx + 2 // 1-based line number after closing ---

	// Empty front matter block (`---\n---`) is valid YAML with missing required fields.
	if strings.TrimSpace(fmRaw) == "" {
		return FrontMatterParseResult{
			Metadata:       PageMetadata{FieldLines: map[string]int{}},
			State:          FrontMatterPresent,
			Body:           body,
			BodyStartLine:  bodyStartLine,
			FrontMatterRaw: fmRaw,
		}
	}

	// Parse into yaml.Node to preserve exact line numbers and detect scalar types.
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(fmRaw), &doc); err != nil {
		lineNum := startIdx + 2
		var typeErr *yaml.TypeError
		if errors.As(err, &typeErr) {
			lineNum = startIdx + 2
		}
		return FrontMatterParseResult{
			State:          FrontMatterInvalidYAML,
			Body:           body,
			BodyStartLine:  bodyStartLine,
			FrontMatterRaw: fmRaw,
			Err: &FrontMatterError{
				State:   FrontMatterInvalidYAML,
				Line:    lineNum,
				Message: fmt.Sprintf("invalid YAML in front matter: %v", err),
				Err:     err,
			},
		}
	}

	// Also decode into map[string]any so unknown fields are preserved cleanly.
	var rawMap map[string]any
	if err := doc.Decode(&rawMap); err != nil {
		return FrontMatterParseResult{
			State:          FrontMatterInvalidYAML,
			Body:           body,
			BodyStartLine:  bodyStartLine,
			FrontMatterRaw: fmRaw,
			Err: &FrontMatterError{
				State:   FrontMatterInvalidYAML,
				Line:    startIdx + 2,
				Message: fmt.Sprintf("front matter must be a YAML mapping: %v", err),
				Err:     err,
			},
		}
	}

	if len(doc.Content) == 0 || doc.Content[0].Kind != yaml.MappingNode {
		return FrontMatterParseResult{
			State:          FrontMatterInvalidYAML,
			Body:           body,
			BodyStartLine:  bodyStartLine,
			FrontMatterRaw: fmRaw,
			Err: &FrontMatterError{
				State:   FrontMatterInvalidYAML,
				Line:    startIdx + 2,
				Message: "front matter must be a YAML key-value mapping",
			},
		}
	}

	mapping := doc.Content[0]
	lineOffset := startIdx + 1 // doc lines are 1-based relative to fmRaw (which starts after line startIdx+1)
	meta := PageMetadata{
		FieldLines: make(map[string]int),
	}

	for i := 0; i+1 < len(mapping.Content); i += 2 {
		keyNode := mapping.Content[i]
		valNode := mapping.Content[i+1]
		key := keyNode.Value
		fieldLine := keyNode.Line + lineOffset
		meta.FieldLines[key] = fieldLine

		var err error
		switch key {
		case "id":
			meta.ID, err = decodeStringScalar(key, valNode, fieldLine)
		case "title":
			meta.Title, err = decodeStringScalar(key, valNode, fieldLine)
		case "type":
			meta.Type, err = decodeStringScalar(key, valNode, fieldLine)
		case "lifecycle":
			meta.Lifecycle, err = decodeStringScalar(key, valNode, fieldLine)
		case "status":
			meta.Status, err = decodeStringScalar(key, valNode, fieldLine)
		case "severity":
			meta.Severity, err = decodeStringScalar(key, valNode, fieldLine)
		case "description":
			meta.Description, err = decodeStringScalar(key, valNode, fieldLine)
		case "created":
			meta.Created, err = decodeDateOrStringScalar(key, valNode, fieldLine)
		case "updated":
			meta.Updated, err = decodeDateOrStringScalar(key, valNode, fieldLine)
		case "superseded_by":
			meta.SupersededBy, err = decodeStringScalar(key, valNode, fieldLine)
		case "tags":
			meta.Tags, err = decodeStringList(key, valNode, fieldLine)
		case "related":
			meta.Related, err = decodeStringList(key, valNode, fieldLine)
		case "depends_on":
			meta.DependsOn, err = decodeStringList(key, valNode, fieldLine)
		case "sources":
			meta.Sources, err = decodeSources(valNode, fieldLine)
		case "references":
			var legacy bool
			meta.References, legacy, err = decodeReferences(valNode, fieldLine)
			if legacy {
				meta.LegacyRefs = true
			}
		default:
			if meta.UnknownFields == nil {
				meta.UnknownFields = make(map[string]any)
			}
			meta.UnknownFields[key] = rawMap[key]
		}
		if err != nil {
			return FrontMatterParseResult{
				Metadata:       meta,
				State:          FrontMatterInvalidFieldType,
				Body:           body,
				BodyStartLine:  bodyStartLine,
				FrontMatterRaw: fmRaw,
				Err:            err,
			}
		}
	}

	// Flag legacy status compatibility usage when lifecycle is absent and status
	// carries a v1 lifecycle keyword on a non-decision/non-incident page.
	if meta.Lifecycle == "" && meta.Status != "" && meta.Type != "decision" && meta.Type != "incident" {
		if _, ok := schema.MapLegacyStatus(meta.Status); ok {
			meta.LegacyStatus = true
		}
	}

	return FrontMatterParseResult{
		Metadata:       meta,
		State:          FrontMatterPresent,
		Body:           body,
		BodyStartLine:  bodyStartLine,
		FrontMatterRaw: fmRaw,
	}
}

func decodeStringScalar(field string, n *yaml.Node, line int) (string, error) {
	if n.Tag == "!!null" {
		return "", nil
	}
	if n.Kind != yaml.ScalarNode {
		return "", &FrontMatterError{
			State:   FrontMatterInvalidFieldType,
			Line:    line,
			Field:   field,
			Message: fmt.Sprintf("field %q must be a string scalar", field),
		}
	}
	// Reject non-string YAML scalars (booleans, ints, floats) unless quoted.
	if n.Style == 0 && (n.Tag == "!!bool" || n.Tag == "!!int" || n.Tag == "!!float") {
		return "", &FrontMatterError{
			State:   FrontMatterInvalidFieldType,
			Line:    line,
			Field:   field,
			Message: fmt.Sprintf("field %q must be a string, got %s (%s)", field, strings.TrimPrefix(n.Tag, "!!"), n.Value),
		}
	}
	return strings.TrimSpace(n.Value), nil
}

func decodeDateOrStringScalar(field string, n *yaml.Node, line int) (string, error) {
	if n.Tag == "!!null" {
		return "", nil
	}
	if n.Kind != yaml.ScalarNode {
		return "", &FrontMatterError{
			State:   FrontMatterInvalidFieldType,
			Line:    line,
			Field:   field,
			Message: fmt.Sprintf("field %q must be a date or string scalar", field),
		}
	}
	if n.Style == 0 && (n.Tag == "!!bool" || n.Tag == "!!int" || n.Tag == "!!float") {
		return "", &FrontMatterError{
			State:   FrontMatterInvalidFieldType,
			Line:    line,
			Field:   field,
			Message: fmt.Sprintf("field %q must be a date string (YYYY-MM-DD), got %s", field, strings.TrimPrefix(n.Tag, "!!")),
		}
	}
	return strings.TrimSpace(n.Value), nil
}

func decodeStringList(field string, n *yaml.Node, line int) ([]string, error) {
	if n.Tag == "!!null" {
		return nil, nil
	}
	if n.Kind != yaml.SequenceNode {
		return nil, &FrontMatterError{
			State:   FrontMatterInvalidFieldType,
			Line:    line,
			Field:   field,
			Message: fmt.Sprintf("field %q must be a YAML list of strings", field),
		}
	}
	if len(n.Content) == 0 {
		return nil, nil
	}
	out := make([]string, 0, len(n.Content))
	for _, item := range n.Content {
		if item.Kind != yaml.ScalarNode || item.Tag == "!!null" {
			return nil, &FrontMatterError{
				State:   FrontMatterInvalidFieldType,
				Line:    line,
				Field:   field,
				Message: fmt.Sprintf("field %q items must be strings", field),
			}
		}
		if item.Style == 0 && (item.Tag == "!!bool" || item.Tag == "!!int" || item.Tag == "!!float") {
			return nil, &FrontMatterError{
				State:   FrontMatterInvalidFieldType,
				Line:    line,
				Field:   field,
				Message: fmt.Sprintf("field %q items must be strings, got %s (%s)", field, strings.TrimPrefix(item.Tag, "!!"), item.Value),
			}
		}
		val := strings.TrimSpace(item.Value)
		if val != "" {
			out = append(out, val)
		}
	}
	if len(out) == 0 {
		return nil, nil
	}
	return out, nil
}

func decodeSources(n *yaml.Node, line int) ([]SourceReference, error) {
	if n.Tag == "!!null" {
		return nil, nil
	}
	if n.Kind != yaml.SequenceNode {
		return nil, &FrontMatterError{
			State:   FrontMatterInvalidFieldType,
			Line:    line,
			Field:   "sources",
			Message: `field "sources" must be a YAML list of source entries`,
		}
	}
	if len(n.Content) == 0 {
		return nil, nil
	}
	var out []SourceReference
	for _, item := range n.Content {
		switch item.Kind {
		case yaml.ScalarNode:
			val := strings.TrimSpace(item.Value)
			if val != "" {
				out = append(out, SourceReference{File: val})
			}
		case yaml.MappingNode:
			var sr SourceReference
			for i := 0; i+1 < len(item.Content); i += 2 {
				k := item.Content[i].Value
				v := item.Content[i+1]
				if v.Kind != yaml.ScalarNode {
					return nil, &FrontMatterError{
						State:   FrontMatterInvalidFieldType,
						Line:    line,
						Field:   "sources",
						Message: fmt.Sprintf("sources entry field %q must be a string", k),
					}
				}
				switch k {
				case "file":
					sr.File = strings.TrimSpace(v.Value)
				case "repo":
					sr.Repo = strings.TrimSpace(v.Value)
				}
			}
			if sr.File != "" || sr.Repo != "" {
				out = append(out, sr)
			}
		default:
			return nil, &FrontMatterError{
				State:   FrontMatterInvalidFieldType,
				Line:    line,
				Field:   "sources",
				Message: `sources entries must be mappings ({file, repo}) or file path strings`,
			}
		}
	}
	if len(out) == 0 {
		return nil, nil
	}
	return out, nil
}

func decodeReferences(n *yaml.Node, line int) (refs []Reference, legacy bool, err error) {
	if n.Tag == "!!null" {
		return nil, false, nil
	}
	if n.Kind != yaml.SequenceNode {
		return nil, false, &FrontMatterError{
			State:   FrontMatterInvalidFieldType,
			Line:    line,
			Field:   "references",
			Message: `field "references" must be a YAML list`,
		}
	}
	if len(n.Content) == 0 {
		return nil, false, nil
	}
	for _, item := range n.Content {
		switch item.Kind {
		case yaml.ScalarNode:
			// Legacy inline string form: "source:...", "external:...", "issue:..."
			raw := strings.TrimSpace(item.Value)
			if raw == "" {
				continue
			}
			legacy = true
			refs = append(refs, parseInlineReference(raw))
		case yaml.MappingNode:
			// Structured form: {type: external, value: https://...}
			var r Reference
			for i := 0; i+1 < len(item.Content); i += 2 {
				k := item.Content[i].Value
				v := item.Content[i+1]
				if v.Kind != yaml.ScalarNode {
					return nil, false, &FrontMatterError{
						State:   FrontMatterInvalidFieldType,
						Line:    line,
						Field:   "references",
						Message: fmt.Sprintf("reference entry field %q must be a string", k),
					}
				}
				switch k {
				case "type":
					r.Type = strings.TrimSpace(v.Value)
				case "value":
					r.Value = strings.TrimSpace(v.Value)
				}
			}
			if r.Type == "" {
				r.Type = "unknown"
			}
			if r.Value != "" {
				refs = append(refs, r)
			}
		default:
			return nil, false, &FrontMatterError{
				State:   FrontMatterInvalidFieldType,
				Line:    line,
				Field:   "references",
				Message: `references entries must be mappings ({type, value}) or "type:value" strings`,
			}
		}
	}
	if len(refs) == 0 {
		return nil, legacy, nil
	}
	return refs, legacy, nil
}

func parseInlineReference(r string) Reference {
	typ, val := "unknown", r
	if idx := strings.Index(r, ":"); idx != -1 {
		prefix := strings.TrimSpace(r[:idx])
		switch prefix {
		case "source", "external", "issue":
			typ = prefix
			val = strings.TrimSpace(r[idx+1:])
		}
	}
	return Reference{Type: typ, Value: val}
}

// SerializeFrontMatter renders PageMetadata as clean YAML frontmatter delimited by ---.
func SerializeFrontMatter(m PageMetadata) string {
	var b strings.Builder
	b.WriteString("---\n")
	writeScalar := func(k, v string, quote bool) {
		if v == "" {
			return
		}
		if quote || needsYAMLQuotes(v) {
			b.WriteString(k + ": " + strconv.Quote(v) + "\n")
		} else {
			b.WriteString(k + ": " + v + "\n")
		}
	}
	writeList := func(k string, items []string) {
		if len(items) == 0 {
			return
		}
		quoted := make([]string, len(items))
		for i, it := range items {
			if needsYAMLQuotes(it) {
				quoted[i] = strconv.Quote(it)
			} else {
				quoted[i] = it
			}
		}
		b.WriteString(k + ": [" + strings.Join(quoted, ", ") + "]\n")
	}

	writeScalar("id", m.ID, false)
	writeScalar("title", m.Title, false)
	writeScalar("type", m.Type, false)
	writeScalar("lifecycle", m.Lifecycle, false)
	writeScalar("status", m.Status, false)
	writeScalar("severity", m.Severity, false)
	writeScalar("description", m.Description, true)
	writeScalar("superseded_by", m.SupersededBy, false)
	writeScalar("created", m.Created, true)
	writeScalar("updated", m.Updated, true)
	writeList("tags", m.Tags)
	writeList("related", m.Related)
	writeList("depends_on", m.DependsOn)

	if len(m.Sources) > 0 {
		b.WriteString("sources:\n")
		for _, s := range m.Sources {
			b.WriteString("  - file: " + formatYAMLVal(s.File) + "\n")
			if s.Repo != "" {
				b.WriteString("    repo: " + formatYAMLVal(s.Repo) + "\n")
			}
		}
	}
	if len(m.References) > 0 {
		b.WriteString("references:\n")
		for _, r := range m.References {
			b.WriteString("  - type: " + formatYAMLVal(r.Type) + "\n")
			b.WriteString("    value: " + formatYAMLVal(r.Value) + "\n")
		}
	}
	b.WriteString("---\n")
	return b.String()
}

func formatYAMLVal(v string) string {
	if needsYAMLQuotes(v) {
		return strconv.Quote(v)
	}
	return v
}

func needsYAMLQuotes(s string) bool {
	if s == "" {
		return true
	}
	lower := strings.ToLower(s)
	switch lower {
	case "true", "false", "yes", "no", "null", "~":
		return true
	}
	for _, ch := range []string{":", "#", "[", "]", "{", "}", ",", `"`, "'", "\n", "\t", "!", "&", "*", "?"} {
		if strings.Contains(s, ch) {
			return true
		}
	}
	if strings.HasPrefix(s, " ") || strings.HasSuffix(s, " ") || strings.HasPrefix(s, "-") {
		return true
	}
	return false
}
