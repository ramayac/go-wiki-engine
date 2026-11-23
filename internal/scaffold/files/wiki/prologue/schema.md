---
id: schema
title: Wiki Schema
type: concept
lifecycle: current
description: "Contract and standards for wiki pages, typed models, lifecycle authority, and structure."
---
# Wiki Schema

## Goal

The wiki is the persistent knowledge layer between the chat agent and the raw repository.
Run `wiki-engine schema` (or `wiki-engine schema --type <type>` / `wiki-engine schema --section <section>`) to inspect the authoritative schema embedded in the CLI binary.

## Required Contract

Every repo that adopts this pattern should have at least these files:

- [README.md](../README.md)
- [index.md](../index.md)
- [log.md](log.md)
- `schema.md` (this file)
- [phases.md](phases.md)
- [repo-map.md](repo-map.md)
- [operations/ingest.md](../operations/ingest.md)
- [operations/query.md](../operations/query.md)
- [operations/lint.md](../operations/lint.md)

## Directory Structure

Only [index.md](../index.md) and [README.md](../README.md) live at the wiki root:

| Path | Role |
|---|---|
| `index.md` | Root catalog — reserved page ID `index`, graph BFS start point. |
| `README.md` | Wiki rules and navigation. |
| `prologue/` | Wiki-about-the-wiki files: `schema.md`, `repo-map.md`, `phases.md`, `log.md`. |
| `operations/` | Repeatable workflow pages (`ingest.md`, `query.md`, `lint.md`). |
| `decisions/`, `entities/`, `incidents/`, `architectures/`, … | Topic pages grouped by domain. |

## Frontmatter Contract (Schema v2)

Frontmatter is parsed with a real YAML parser. Missing metadata **never** defaults to active.

```yaml
---
id: main-service                # Globally unique; matches filename without .md (root index.md uses "index")
title: Main Service
type: entity                    # concept | entity | incident | decision | runbook
lifecycle: current              # current | wip | implemented | planned | legacy | superseded
status: accepted                # Workflow status (decisions: proposed|accepted|superseded; incidents: active|resolved)
severity: sev2                  # Required for type: incident
description: "One-line summary of this page's purpose"
superseded_by: ""               # Required when lifecycle is superseded
created: "2026-01-15"
updated: "2026-11-23"
tags: [service]
related: [index]                # Page IDs
depends_on: []                  # Entity page IDs only
sources:
  - file: main.go
references:
  - type: external              # http:// or https:// URL
    value: https://example.com/design
  - type: issue                 # Tracker key matching issue_pattern
    value: PROJ-123
---
```

### Legacy Compatibility

v1 frontmatter (`status: planned|current|legacy|deprecated` and inline `references: [source:..., external:..., issue:...]`) remains readable for one major release. Run `wiki-engine lint --fix` to migrate to v2 structured fields.

## Page Types

| Type | Purpose | Validation Rules |
|---|---|---|
| `concept` | Stable system knowledge | Should cite source files or directories in `sources` |
| `entity` | Component, service, store, or integration | `depends_on` targets must be `entity` pages |
| `incident` | Failure, response, and lesson | Requires `severity` and `status` (`active` \| `resolved`) |
| `decision` | Choice, alternatives, and consequences | Requires `status` (`proposed` \| `accepted` \| `superseded`) |
| `runbook` | Procedure with verification | Should cite source files or directories in `sources` |

## Authority Lifecycle

| Lifecycle | Meaning | Default Search | Active Graph | Drift Checked |
|---|---|---|---|---|
| `current` | Trusted current knowledge | Yes | Yes | Yes |
| `wip` | Partially true and in progress | Yes | Yes | Yes |
| `implemented` | A shipped plan or record | Yes | Yes | No |
| `planned` | Intended future state | No (`--include-history`) | Yes | Yes |
| `legacy` | Historical context | No (`--include-history`) | No | No |
| `superseded` | Replaced by another page (`superseded_by` required) | No (`--include-history`) | No | No |

## Exit Codes & JSON Contract

- Global JSON envelope (version `1`): `{"ok": bool, "data": ..., "error": "..."}`
- Graph JSON schema version: `2`
- Exit `0`: Success (including `wiki-engine check` with review recommendations only)
- Exit `1`: Content/lint/check breakage or usage error
- Exit `2`: Git, config, or setup error
