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

Page templates (`wiki-engine schema --type <type>`) ask `concept` and `entity` pages for a `## Why` section (the reason and the rejected options). They ask `concept`, `entity`, `incident`, and `decision` pages for a `## Stops applying when` section (a condition that someone can check, or "Not known."). A `decision` records its reason in its decision section, and an `incident` in its root cause. A `runbook` has neither section.

## Authority Lifecycle

| Lifecycle | Meaning | Default Search | Active Graph | Drift Checked |
|---|---|---|---|---|
| `current` | Trusted current knowledge | Yes | Yes | Yes |
| `wip` | Partially true and in progress | Yes | Yes | Yes |
| `implemented` | A shipped plan or record | Yes | Yes | No |
| `planned` | Intended future state | No (`--include-history`) | Yes | Yes |
| `legacy` | Historical context | No (`--include-history`) | No | No |
| `superseded` | Replaced by another page (`superseded_by` required) | No (`--include-history`) | No | No |

## Conventions

- **Rationale marker.** Record why, not what. When no source (the session, the commit messages, or the PR) states why something works the way it does, write `> Rationale: not recorded.` on its own line. Do not guess a reason. Do not stop to ask. When a source later states the reason, replace the marker line with it.
- **Contradiction flag.** When new information contradicts existing content, flag it on its own line with `> ⚠️ CONTRADICTION: <what conflicts and where>`. Replace the flag when the conflict is resolved.
- **Open markers are advisory.** `wiki-engine lint` does not report a rationale marker or a contradiction flag. `wiki-engine health` lists the live pages (`current`, `implemented`, `wip`, `planned`) that carry one, and never fails a build. A marker in a code block or code span is an example, so `health` does not count it. The markers appear here, and not in a page template, so that a page does not copy one by default.
- **Documentation language.** Write wiki pages in ASD-STE100 Simplified Technical English. Use short, direct sentences. Use active voice. Put one instruction in each sentence. Use the same term for the same thing. Do not use idioms.
- **Data protection.** Never persist secrets, credentials, tokens, session IDs, personal or health identifiers, free-text personal data, or whole request or response payloads in wiki pages or in the log. Redact the value and keep only the technical fact.
- **Log rotation.** When `wiki-engine health` reports `log_needs_rotation` (the log is 128 KiB or larger), move the oldest complete entries of `log.md` into `log-archive/<YYYY>.log` next to it, until the log is below 96 KiB. Archive files keep the entry format, are append-only, and use the `.log` extension so search, graph, and lint do not read them as pages.

`wiki-engine schema --section conventions` prints the same rules.

## Exit Codes & JSON Contract

- Global JSON envelope (version `1`): `{"ok": bool, "data": ..., "error": "..."}`
- Graph JSON schema version: `2`
- Exit `0`: Success (including `wiki-engine check` with review recommendations only)
- Exit `1`: Content/lint/check breakage or usage error
- Exit `2`: Git, config, or setup error
