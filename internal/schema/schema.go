// Package schema defines the canonical, embedded wiki schema and vocabulary
// shared across model parsing, linting, scaffolding, and the `wiki-engine schema`
// command. Go vocabulary and schema output derive from this single source of truth.
package schema

import (
	"fmt"
	"strings"
)

// SchemaVersion is the active frontmatter/wiki schema version.
const SchemaVersion = "2"

// GraphSchemaVersion is the JSON schema version for graph payloads.
const GraphSchemaVersion = "2"

// JSONEnvelopeVersion documents the global JSON envelope version.
const JSONEnvelopeVersion = "1"

// ValidPageTypes lists the canonical semantic page types.
var ValidPageTypes = []string{
	"concept",
	"entity",
	"incident",
	"decision",
	"runbook",
}

// ValidLifecycles lists the canonical authority lifecycle values in display order.
var ValidLifecycles = []string{
	"current",
	"wip",
	"implemented",
	"planned",
	"legacy",
	"superseded",
}

// LegacyStatuses lists the deprecated v1 status values mapped to lifecycle.
var LegacyStatuses = []string{
	"planned",
	"current",
	"legacy",
	"deprecated",
}

// DecisionStatuses lists valid workflow status values for type: decision.
var DecisionStatuses = []string{
	"proposed",
	"accepted",
	"superseded",
}

// IncidentStatuses lists valid workflow status values for type: incident.
var IncidentStatuses = []string{
	"active",
	"resolved",
}

// ValidReferenceTypes lists valid non-source reference types.
var ValidReferenceTypes = []string{
	"external",
	"issue",
}

// DefaultIssuePattern is the default regex for issue reference keys.
const DefaultIssuePattern = `^[A-Z][A-Z0-9]*-[0-9]+$`

// LifecyclePriority returns the sort rank for index ordering (lower comes first).
func LifecyclePriority(lifecycle string) int {
	switch lifecycle {
	case "current":
		return 0
	case "wip":
		return 1
	case "implemented":
		return 2
	case "planned":
		return 3
	case "legacy":
		return 4
	case "superseded":
		return 5
	default:
		return 6
	}
}

// IsValidPageType reports whether t is a canonical page type.
func IsValidPageType(t string) bool {
	return contains(ValidPageTypes, t)
}

// IsValidLifecycle reports whether l is a canonical lifecycle value.
func IsValidLifecycle(l string) bool {
	return contains(ValidLifecycles, l)
}

// IsValidDecisionStatus reports whether s is a valid decision workflow status.
func IsValidDecisionStatus(s string) bool {
	return contains(DecisionStatuses, s)
}

// IsValidIncidentStatus reports whether s is a valid incident workflow status.
func IsValidIncidentStatus(s string) bool {
	return contains(IncidentStatuses, s)
}

// MapLegacyStatus maps a v1 status field to a v2 lifecycle value.
// Returns ("", false) if s is not a known legacy lifecycle status.
func MapLegacyStatus(s string) (string, bool) {
	switch s {
	case "current":
		return "current", true
	case "planned":
		return "planned", true
	case "legacy":
		return "legacy", true
	case "deprecated":
		return "superseded", true
	default:
		return "", false
	}
}

// IsActiveLifecycle reports whether a lifecycle participates in the active wiki graph.
func IsActiveLifecycle(l string) bool {
	switch l {
	case "current", "implemented", "wip", "planned":
		return true
	default:
		return false
	}
}

// IsSearchDefaultLifecycle reports whether a lifecycle is included in default search
// without --include-history.
func IsSearchDefaultLifecycle(l string) bool {
	switch l {
	case "current", "implemented", "wip":
		return true
	default:
		return false
	}
}

// IsDriftCheckedLifecycle reports whether active-drift and source existence checks
// apply to pages with this lifecycle. Implemented, legacy, and superseded pages
// do not receive active-drift checks.
func IsDriftCheckedLifecycle(l string) bool {
	switch l {
	case "current", "wip", "planned":
		return true
	default:
		return false
	}
}

// IsHistoricalLifecycle reports whether a lifecycle represents historical context.
func IsHistoricalLifecycle(l string) bool {
	return l == "legacy" || l == "superseded"
}

// LifecycleTrustRank returns a multiplier for ranked search scoring.
func LifecycleTrustRank(l string) float64 {
	switch l {
	case "current":
		return 1.25
	case "implemented":
		return 1.10
	case "wip":
		return 1.00
	case "planned":
		return 0.80
	case "legacy":
		return 0.55
	case "superseded":
		return 0.40
	default:
		return 0.50
	}
}

// LifecycleBanner returns the canonical callout banner for non-current pages.
func LifecycleBanner(lifecycle, supersededBy string) string {
	switch lifecycle {
	case "wip":
		return "> [!NOTE]\n> **Lifecycle: wip** — This page is work in progress and may be incomplete."
	case "planned":
		return "> [!NOTE]\n> **Lifecycle: planned** — This page describes intended future state, not current behavior."
	case "implemented":
		return "> [!NOTE]\n> **Lifecycle: implemented** — Historical record of a shipped plan."
	case "legacy":
		return "> [!WARNING]\n> **Lifecycle: legacy** — Historical context; no longer reflects current behavior."
	case "superseded":
		if supersededBy != "" {
			return fmt.Sprintf("> [!WARNING]\n> **Lifecycle: superseded** — Replaced by `%s`.", supersededBy)
		}
		return "> [!WARNING]\n> **Lifecycle: superseded** — This page has been replaced."
	default:
		return ""
	}
}

// TypeSpec describes a semantic page type and its template.
type TypeSpec struct {
	Name           string   `json:"name"`
	Purpose        string   `json:"purpose"`
	RequiredFields []string `json:"required_fields"`
	OptionalFields []string `json:"optional_fields"`
	Rules          []string `json:"rules"`
	Template       string   `json:"template"`
}

// LifecycleSpec describes one authority lifecycle value.
type LifecycleSpec struct {
	Name           string `json:"name"`
	Meaning        string `json:"meaning"`
	SearchDefault  bool   `json:"search_default"`
	ActiveInGraph  bool   `json:"active_in_graph"`
	DriftChecked   bool   `json:"drift_checked"`
	RequiresBanner bool   `json:"requires_banner"`
}

// Document is the complete machine-readable schema definition.
type Document struct {
	SchemaVersion       string          `json:"schema_version"`
	GraphSchemaVersion  string          `json:"graph_schema_version"`
	JSONEnvelopeVersion string          `json:"json_envelope_version"`
	RequiredFields      []string        `json:"required_fields"`
	OptionalFields      []string        `json:"optional_fields"`
	Lifecycles          []LifecycleSpec `json:"lifecycles"`
	Types               []TypeSpec      `json:"types"`
	DecisionStatuses    []string        `json:"decision_statuses"`
	IncidentStatuses    []string        `json:"incident_statuses"`
	ReferenceTypes      []string        `json:"reference_types"`
	DefaultIssuePattern string          `json:"default_issue_pattern"`
	ExitCodes           map[string]int  `json:"exit_codes"`
}

// Sections lists the valid --section values for `wiki-engine schema`.
var Sections = []string{
	"frontmatter",
	"lifecycle",
	"types",
	"sources",
	"references",
	"vocabulary",
	"graph",
	"templates",
	"conventions",
	"exit-codes",
	"json",
}

// Conventions lists the writing rules for wiki pages. The marker lines appear
// here, and not in a page template, so that a page does not copy one by default.
var Conventions = []string{
	"Rationale marker: record why, not what. When no source (the session, the commit messages, or the PR) states why something works the way it does, write `> Rationale: not recorded.` on its own line. Do not guess a reason. Do not stop to ask. When a source later states the reason, replace the marker line with it. `wiki-engine health` lists the live pages that still carry the marker.",
	"Contradiction flag: when new information contradicts existing content, flag it on its own line with `> ⚠️ CONTRADICTION: <what conflicts and where>`. Replace the flag when the conflict is resolved. `wiki-engine health` lists the live pages that still carry a flag.",
	"Open markers are advisory. A rationale marker or a contradiction flag is a valid state, so `wiki-engine lint` does not report it and `wiki-engine health` never fails a build. A marker in a code block or code span is an example, so `health` does not count it.",
	"Documentation language: write wiki pages in ASD-STE100 Simplified Technical English. Use short, direct sentences. Use active voice. Put one instruction in each sentence. Use the same term for the same thing. Do not use idioms.",
	"Data protection: never persist secrets, credentials, tokens, session IDs, personal or health identifiers, free-text personal data, or whole request or response payloads in wiki pages or in the log. Redact the value and keep only the technical fact.",
	"Log rotation: when `wiki-engine health` reports `log_needs_rotation` (the log is 128 KiB or larger), move the oldest complete entries of `log.md` into `log-archive/<YYYY>.log` next to it, until the log is below 96 KiB. Archive files keep the entry format, are append-only, and use the `.log` extension so search, graph, and lint do not read them as pages.",
}

// jsonContractText is the body of `wiki-engine schema --section json`.
const jsonContractText = "- `ok`: `true` when the command exits `0`; `false` for every non-zero exit.\n" +
	"- `meta`: on every success, and on a failure that keeps `data`. Keys on every response: `schema_version` (envelope version) and `command`. Command keys are in the table below.\n" +
	"- `data`: the command result. Its top-level type does not change (`search` is a list; `graph`, `check`, and `health` are objects). Kept on findings failures. Empty lists encode as `[]`, never `null`.\n" +
	"- `error`, `error_code`: on failure only. `error_code` is `usage` (invalid invocation, exit 2; also `check` git and setup errors), `findings` (the command ran and found problems, exit 1: `lint`, `check` breakage, `graph --strict`, `refresh` and `watch` lint), or `runtime` (any other exit 1).\n" +
	"- Compatibility: consumers must ignore unknown keys. New keys ship in minor releases. Removing or renaming a key, or changing its type, is a breaking change.\n\n" +
	"| Command | `meta` keys | `data` keys (item keys for lists) |\n" +
	"|---|---|---|\n" +
	"| `search` (lines) | `query`, `mode` (`lines`), `total` (hits before `--limit`), `returned`, `limit` (`0` = none), `truncated`, `include_history` | per hit: `id`, `file`, `path` (= `file`), `title`, `type`, `tags`, `lifecycle`, `line`, `text` |\n" +
	"| `search --ranked`, `relevant` | `search --ranked` sets the `search` keys with `mode` `ranked`; `relevant` sets the common keys only | per hit: `id`, `file`, `path` (= `file`), `title`, `type`, `tags`, `lifecycle`, `score`, `matched_terms`, `coverage`, `term_coverage` (= `coverage`), `section`, `start_line`, `context`, `why` |\n" +
	"| `graph` | `root` (`index.md`, or the page file in neighborhood mode), `mode` (`full` or `neighborhood`), `include_sources` | `schema_version`, `nodes`, `edges`, `unlinked`, `stats`, `issues`; neighborhood: `node`, `backlinks`. Node keys: `id`, `kind`, `file`, `title`, `type`, `tags`, `lifecycle`, `status`, `description`, `links`, `depth`, `referrals` (inbound edge count, `0` allowed) |\n" +
	"| `health` | common keys | `schema_version`, size and graph counts, `lifecycles`, `largest_pages`, `aging_work`, `updated_behind_git`, `duplicate_titles`, `index_only_pages`, `open_markers`, `thresholds` (`aging_days`, `drift_days`, `log_rotation_bytes`) |\n" +
	"| `check` | `base`, `working_tree` | `base`, `merge_base`, `working_tree`, `changed_files`, `broken`, `warnings`, `pages_to_review`, `summary` (`broken`, `warnings`, `pages_to_review`, `changed_files`: integers), `exit_code` (`0` or `1`). `broken` and `warnings` entries: `page`, `kind`, `severity` (`broken` or `warning`), `message`, and `id`, `target`, `line` when known. `pages_to_review` entries: `page`, `kind` (`source_changed`), `severity` (`review`), `changed_files`, `match_reasons`, `id` |\n" +
	"| other commands | common keys | unchanged |\n"

// Get returns the canonical embedded schema document.
func Get() Document {
	return Document{
		SchemaVersion:       SchemaVersion,
		GraphSchemaVersion:  GraphSchemaVersion,
		JSONEnvelopeVersion: JSONEnvelopeVersion,
		RequiredFields:      []string{"id", "lifecycle", "description"},
		OptionalFields: []string{
			"title", "type", "status", "severity", "created", "updated",
			"tags", "related", "depends_on", "superseded_by", "sources", "references",
		},
		Lifecycles: []LifecycleSpec{
			{Name: "current", Meaning: "Trusted current knowledge", SearchDefault: true, ActiveInGraph: true, DriftChecked: true, RequiresBanner: false},
			{Name: "wip", Meaning: "Partially true and in progress", SearchDefault: true, ActiveInGraph: true, DriftChecked: true, RequiresBanner: true},
			{Name: "implemented", Meaning: "A shipped plan or record", SearchDefault: true, ActiveInGraph: true, DriftChecked: false, RequiresBanner: true},
			{Name: "planned", Meaning: "Intended future state", SearchDefault: false, ActiveInGraph: true, DriftChecked: true, RequiresBanner: true},
			{Name: "legacy", Meaning: "Historical context", SearchDefault: false, ActiveInGraph: false, DriftChecked: false, RequiresBanner: true},
			{Name: "superseded", Meaning: "Replaced by another page", SearchDefault: false, ActiveInGraph: false, DriftChecked: false, RequiresBanner: true},
		},
		Types: []TypeSpec{
			{
				Name:           "concept",
				Purpose:        "Stable system knowledge",
				RequiredFields: []string{"id", "title", "type", "lifecycle", "description"},
				OptionalFields: []string{"created", "updated", "tags", "related", "sources", "references"},
				Rules:          []string{"Concepts should cite source files or source directories in sources."},
				Template: `---
id: my-concept
title: My Concept
type: concept
lifecycle: current
description: "One-line summary of this concept."
created: "2026-01-01"
updated: "2026-01-01"
tags: []
related: [index]
sources:
  - file: path/to/source.go
---
# My Concept

Explain the stable system concept here.

## Why

Why it works this way, and which options the team rejected. Take the reason from the session, the commit messages, or the PR. If no source states the reason, write the rationale marker (run wiki-engine schema --section conventions).

## Stops applying when

A condition that someone can check: a version, a config value, a metric, or a process. If you do not know the condition, write "Not known."
`,
			},
			{
				Name:           "entity",
				Purpose:        "Component, service, store, or integration",
				RequiredFields: []string{"id", "title", "type", "lifecycle", "description"},
				OptionalFields: []string{"created", "updated", "tags", "related", "depends_on", "sources", "references"},
				Rules:          []string{"depends_on entries must resolve to entity pages."},
				Template: `---
id: my-entity
title: My Entity
type: entity
lifecycle: current
description: "One-line summary of this component or service."
created: "2026-01-01"
updated: "2026-01-01"
tags: [component]
depends_on: []
related: [index]
sources:
  - file: path/to/component/
---
# My Entity

Describe responsibilities, interfaces, and dependencies.

## Why

Why it works this way, and which options the team rejected. Take the reason from the session, the commit messages, or the PR. If no source states the reason, write the rationale marker (run wiki-engine schema --section conventions).

## Stops applying when

A condition that someone can check: a version, a config value, a metric, or a process. If you do not know the condition, write "Not known."
`,
			},
			{
				Name:           "incident",
				Purpose:        "Failure, response, and lesson",
				RequiredFields: []string{"id", "title", "type", "lifecycle", "status", "severity", "description"},
				OptionalFields: []string{"created", "updated", "tags", "related", "sources", "references"},
				Rules:          []string{"Incidents require severity and status (active | resolved)."},
				Template: `---
id: incident-example
title: Incident Title
type: incident
lifecycle: current
status: resolved
severity: sev2
description: "Summary of failure, impact, and remediation."
created: "2026-01-01"
updated: "2026-01-01"
tags: [incident]
related: [index]
sources: []
---
# Incident Title

## Summary

## Root Cause

Technical explanation.

## Remediation & Lessons

What was done, and why this fix over the alternatives. Link to the PR.

## Stops applying when

A condition that someone can check: a version, a config value, a metric, or a process. If you do not know the condition, write "Not known."
`,
			},
			{
				Name:           "decision",
				Purpose:        "Choice, alternatives, and consequences",
				RequiredFields: []string{"id", "title", "type", "lifecycle", "status", "description"},
				OptionalFields: []string{"created", "updated", "tags", "related", "superseded_by", "sources", "references"},
				Rules:          []string{"Decisions require status (proposed | accepted | superseded)."},
				Template: `---
id: decision-example
title: Decision Title
type: decision
lifecycle: current
status: accepted
description: "Summary of the decision and rationale."
created: "2026-01-01"
updated: "2026-01-01"
tags: [decision]
related: [index]
sources: []
---
# Decision Title

## Context

## Alternatives Considered

## Decision & Consequences

What was chosen and why, and what this means going forward.

## Stops applying when

A condition that someone can check: a version, a config value, a metric, or a process. If you do not know the condition, write "Not known."
`,
			},
			{
				Name:           "runbook",
				Purpose:        "Procedure with verification",
				RequiredFields: []string{"id", "title", "type", "lifecycle", "description"},
				OptionalFields: []string{"created", "updated", "tags", "related", "sources", "references"},
				Rules:          []string{"Runbooks should cite source files or source directories in sources."},
				Template: `---
id: my-runbook
title: My Runbook
type: runbook
lifecycle: current
description: "Step-by-step operational procedure with verification."
created: "2026-01-01"
updated: "2026-01-01"
tags: [runbook]
related: [index]
sources:
  - file: Makefile
---
# My Runbook

## Steps

## Verification
`,
			},
		},
		DecisionStatuses:    append([]string(nil), DecisionStatuses...),
		IncidentStatuses:    append([]string(nil), IncidentStatuses...),
		ReferenceTypes:      append([]string(nil), ValidReferenceTypes...),
		DefaultIssuePattern: DefaultIssuePattern,
		ExitCodes: map[string]int{
			"ok":            0,
			"content_error": 1,
			"usage_error":   1,
			"setup_error":   2,
		},
	}
}

// TypeByName returns the TypeSpec for a given type name, or an error if unknown.
func TypeByName(name string) (TypeSpec, error) {
	doc := Get()
	for _, ts := range doc.Types {
		if ts.Name == name {
			return ts, nil
		}
	}
	return TypeSpec{}, fmt.Errorf("unknown page type %q (valid: %s)", name, strings.Join(ValidPageTypes, ", "))
}

// RenderText formats the schema document (or a filtered type/section) as readable text.
func RenderText(typeName, section string) (string, error) {
	doc := Get()
	if typeName != "" {
		ts, err := TypeByName(typeName)
		if err != nil {
			return "", err
		}
		var b strings.Builder
		fmt.Fprintf(&b, "# Page Type: %s\n\n", ts.Name)
		fmt.Fprintf(&b, "Purpose: %s\n\n", ts.Purpose)
		fmt.Fprintf(&b, "Required fields: %s\n", strings.Join(ts.RequiredFields, ", "))
		fmt.Fprintf(&b, "Optional fields: %s\n\n", strings.Join(ts.OptionalFields, ", "))
		if len(ts.Rules) > 0 {
			b.WriteString("## Rules\n\n")
			for _, r := range ts.Rules {
				fmt.Fprintf(&b, "- %s\n", r)
			}
			b.WriteString("\n")
		}
		b.WriteString("## Template\n\n```markdown\n")
		b.WriteString(ts.Template)
		b.WriteString("```\n")
		return b.String(), nil
	}

	if section != "" {
		return renderSection(doc, section)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "# Wiki Schema (v%s)\n\n", doc.SchemaVersion)
	for _, sec := range Sections {
		part, err := renderSection(doc, sec)
		if err != nil {
			return "", err
		}
		b.WriteString(part)
		b.WriteString("\n")
	}
	return b.String(), nil
}

func renderSection(doc Document, section string) (string, error) {
	var b strings.Builder
	switch section {
	case "frontmatter":
		b.WriteString("## Frontmatter\n\n")
		fmt.Fprintf(&b, "Schema version: %s\n", doc.SchemaVersion)
		fmt.Fprintf(&b, "Required fields: %s\n", strings.Join(doc.RequiredFields, ", "))
		fmt.Fprintf(&b, "Optional fields: %s\n", strings.Join(doc.OptionalFields, ", "))
		b.WriteString("Tags: free-form unless the wiki lists them in `vocabulary.yaml`. Run `wiki-engine schema --section vocabulary`.\n")
		b.WriteString("Legacy compatibility: v1 `status: planned|current|legacy|deprecated` maps to `lifecycle` with a deprecation notice. Missing lifecycle never defaults to active.\n")
	case "lifecycle":
		b.WriteString("## Lifecycle\n\n")
		b.WriteString("| Lifecycle | Meaning | Search Default | Active Graph | Drift Checked |\n")
		b.WriteString("|---|---|---|---|---|\n")
		for _, l := range doc.Lifecycles {
			fmt.Fprintf(&b, "| `%s` | %s | %t | %t | %t |\n", l.Name, l.Meaning, l.SearchDefault, l.ActiveInGraph, l.DriftChecked)
		}
		fmt.Fprintf(&b, "\nDecision workflow statuses: %s\n", strings.Join(doc.DecisionStatuses, ", "))
		fmt.Fprintf(&b, "Incident workflow statuses: %s (also requires `severity`)\n", strings.Join(doc.IncidentStatuses, ", "))
	case "types":
		b.WriteString("## Page Types\n\n")
		b.WriteString("| Type | Purpose | Rules |\n")
		b.WriteString("|---|---|---|\n")
		for _, ts := range doc.Types {
			fmt.Fprintf(&b, "| `%s` | %s | %s |\n", ts.Name, ts.Purpose, strings.Join(ts.Rules, "; "))
		}
	case "sources":
		b.WriteString("## Sources\n\n")
		b.WriteString("Structured source citations:\n```yaml\nsources:\n  - file: internal/engine/graph.go\n    repo: go-wiki-engine # optional; cross-repo sources require repo\n```\n")
		b.WriteString("Legacy inline `references: [source:path/to/file]` remains supported during migration.\n")
	case "references":
		b.WriteString("## References\n\n")
		fmt.Fprintf(&b, "Structured reference types: %s\n", strings.Join(doc.ReferenceTypes, ", "))
		fmt.Fprintf(&b, "Default issue key pattern: `%s`\n", doc.DefaultIssuePattern)
		b.WriteString("External references must use `http://` or `https://`.\n")
	case "vocabulary":
		b.WriteString("## Vocabulary\n\n")
		b.WriteString("`<wiki_dir>/vocabulary.yaml` is an optional, user-owned list of allowed page tags. `wiki-engine init` creates it with no entries and never overwrites it.\n")
		b.WriteString("Format: YAML with any top-level keys (category names, for people only), each holding a list of tag strings. A bare list is also valid. The allowed set is the union of all listed tags.\n")
		b.WriteString("```yaml\nconcerns:\n  - testing\n  - security\nareas:\n  - api\n```\n")
		b.WriteString("Mode free-form: a missing, empty, or comments-only file lists no tags, and `lint` accepts any tag.\n")
		b.WriteString("Mode controlled: once any tag is listed, the `vocabulary` check reports an error for every page tag that is not listed, on pages of every lifecycle. If a listed tag differs only by case or `-` versus `_`, the message adds a \"did you mean\" hint.\n")
		b.WriteString("A malformed file, a non-string value, or a file that resolves outside the repository gives one error on `vocabulary.yaml`. It never falls back to free-form.\n")
		b.WriteString("Select the check with `wiki-engine lint --check=vocabulary` or skip it with `--skip=vocabulary`. It belongs to the `schema` group.\n")
	case "graph":
		b.WriteString("## Graph\n\n")
		fmt.Fprintf(&b, "Graph JSON schema version: %s\n", doc.GraphSchemaVersion)
		b.WriteString("Edges combine validated Markdown links, `related`, `depends_on`, and opt-in source nodes (`--include-sources`).\n")
		b.WriteString("Page IDs match filename without `.md` (`index.md` has reserved ID `index`).\n")
	case "templates":
		b.WriteString("## Templates\n\n")
		b.WriteString("Run `wiki-engine schema --type <type>` to print a ready-to-use page template.\n")
	case "conventions":
		b.WriteString("## Conventions\n\n")
		for _, c := range Conventions {
			fmt.Fprintf(&b, "- %s\n", c)
		}
	case "exit-codes":
		b.WriteString("## Exit Codes\n\n")
		b.WriteString("- `0`: Success (including `check` with review recommendations only)\n")
		b.WriteString("- `1`: Runtime or finding failure (lint, `graph --strict`, `refresh` lint, `check` breakage, git, config, or I/O errors)\n")
		b.WriteString("- `2`: Usage error (unknown command or flag, bad argument or flag value); `check` also uses `2` for git and setup errors (e.g., outside git or invalid base ref)\n")
	case "json":
		b.WriteString("## JSON Contract\n\n")
		fmt.Fprintf(&b, "Envelope version: %s\n", doc.JSONEnvelopeVersion)
		b.WriteString("`{\"ok\": bool, \"meta\": {...}, \"data\": ..., \"error\": \"...\", \"error_code\": \"...\"}`\n")
		fmt.Fprintf(&b, "Graph schema version: %s (the `schema_version` key inside graph `data`; additive node keys do not change it)\n\n", doc.GraphSchemaVersion)
		b.WriteString(jsonContractText)
	default:
		return "", fmt.Errorf("unknown schema section %q (valid: %s)", section, strings.Join(Sections, ", "))
	}
	return b.String(), nil
}

func contains(list []string, v string) bool {
	for _, s := range list {
		if s == v {
			return true
		}
	}
	return false
}
