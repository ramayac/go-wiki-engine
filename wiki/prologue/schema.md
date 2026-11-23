---
id: schema
title: Wiki Schema
type: concept
lifecycle: current
description: "Contract and standards for wiki pages, typed models, lifecycle authority, and structure."
sources:
  - file: internal/schema/schema.go
  - file: internal/model/frontmatter.go
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
| `vocabulary.yaml` | Optional list of allowed page tags. Created by `init`, never overwritten. Format: `wiki-engine schema --section vocabulary`. |
| `prologue/` | Wiki-about-the-wiki files: `schema.md`, `repo-map.md`, `phases.md`, `log.md`, `config.md`. |
| `operations/` | Repeatable workflow pages (`ingest.md`, `query.md`, `lint.md`). |
| `decisions/`, `entities/`, `incidents/`, `architectures/`, … | Topic pages grouped by domain. |

The engine resolves canonical files (`log.md`, `phases.md`, `schema.md`, `repo-map.md`) at `prologue/<name>` first and falls back to legacy root-level paths.

## Read & Write Order

1. Start from active context (`wiki-engine context --active`) or [index.md](../index.md).
2. Read the latest entries in [log.md](log.md).
3. Check each page's `lifecycle` before relying on it; ignore `planned` content as current fact and follow `superseded_by` links.
4. Read source files only after consulting the wiki.
5. After editing pages, add exact `sources:`, update [index.md](../index.md), append to [log.md](log.md), and run `wiki-engine lint` + `wiki-engine check`.

## Frontmatter Contract (Schema v2)

Frontmatter is parsed with a real YAML parser (`gopkg.in/yaml.v3`). Missing metadata **never** defaults to active.

```yaml
---
id: main-service                # Globally unique; matches filename without .md (root index.md uses "index")
title: Main Service             # Human-readable title
type: entity                    # concept | entity | incident | decision | runbook
lifecycle: current              # current | wip | implemented | planned | legacy | superseded
status: accepted                # Workflow status (decisions: proposed|accepted|superseded; incidents: active|resolved)
severity: sev2                  # Required for type: incident
description: "One-line summary of this page's purpose"
superseded_by: ""               # Required when lifecycle is superseded
created: "2026-01-15"
updated: "2026-11-23"
tags: [service]                 # Must be listed in vocabulary.yaml once that file lists any tag
related: [architecture]         # Page IDs
depends_on: [storage]           # Entity page IDs only
sources:
  - file: internal/engine/graph.go
    repo: go-wiki-engine        # Optional for local repo; required for cross-repo sources
references:
  - type: external              # http:// or https:// URL
    value: https://example.com/design
  - type: issue                 # Tracker key matching issue_pattern (default ^[A-Z][A-Z0-9]*-[0-9]+$)
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

Page authority (`lifecycle`) is separate from decision/incident workflow (`status`):

| Lifecycle | Meaning | Default Search | Active Graph | Drift Checked | Banner & Index Badge |
|---|---|---|---|---|---|
| `current` | Trusted current knowledge | Yes | Yes | Yes | No |
| `wip` | Partially true and in progress | Yes | Yes | Yes | Yes |
| `implemented` | A shipped plan or record | Yes | Yes | No | Yes |
| `planned` | Intended future state | No (`--include-history`) | Yes | Yes | Yes |
| `legacy` | Historical context | No (`--include-history`) | No | No | Yes |
| `superseded` | Replaced by another page (`superseded_by` required) | No (`--include-history`) | No | No | Yes |

## Conventions

- **Rationale marker.** Record why, not what. When no source (the session, the commit messages, or the PR) states why something works the way it does, write `> Rationale: not recorded.` on its own line. Do not guess a reason. Do not stop to ask. When a source later states the reason, replace the marker line with it.
- **Contradiction flag.** When new information contradicts existing content, flag it on its own line with `> ⚠️ CONTRADICTION: <what conflicts and where>`. Replace the flag when the conflict is resolved.
- **Open markers are advisory.** `wiki-engine lint` does not report a rationale marker or a contradiction flag. `wiki-engine health` lists the live pages (`current`, `implemented`, `wip`, `planned`) that carry one, and never fails a build. A marker in a code block or code span is an example, so `health` does not count it. The markers appear here, and not in a page template, so that a page does not copy one by default.
- **Documentation language.** Write wiki pages in ASD-STE100 Simplified Technical English. Use short, direct sentences. Use active voice. Put one instruction in each sentence. Use the same term for the same thing. Do not use idioms.
- **Data protection.** Never persist secrets, credentials, tokens, session IDs, personal or health identifiers, free-text personal data, or whole request or response payloads in wiki pages or in the log. Redact the value and keep only the technical fact.
- **Log rotation.** When `wiki-engine health` reports `log_needs_rotation` (the log is 128 KiB or larger), move the oldest complete entries of `log.md` into `log-archive/<YYYY>.log` next to it, until the log is below 96 KiB. Archive files keep the entry format, are append-only, and use the `.log` extension so search, graph, and lint do not read them as pages.

`wiki-engine schema --section conventions` prints the same rules.

## Cross-Linking & Graph Rules

- Every active page links to related pages via relative Markdown links, `related`, or `depends_on`.
- `log.md` is the only intentional leaf.
- All wiki links are strictly contained inside `wiki_dir`; no operation can traverse outside the repo or wiki.
- Graph JSON (`schema_version: "2"`) carries `nodes`, `edges`, `unlinked`, `stats`, and `issues`; source nodes are opt-in via `wiki-engine graph --include-sources`.

## Exit Codes & JSON Contract

- Global JSON envelope (version `1`): `{"ok": bool, "meta": {...}, "data": ..., "error": "...", "error_code": "usage|findings|runtime"}`. `meta` carries `schema_version`, `command`, and command keys. Consumers must ignore unknown keys. The per-command key table is `wiki-engine schema --section json`; tests pin it in `cmd/wiki-engine/testdata/golden/`.
- Exit `0`: Success (including `wiki-engine check` with review recommendations only)
- Exit `1`: Runtime or finding failure (lint, `graph --strict`, `refresh` lint, `check` breakage, git/config/I/O errors)
- Exit `2`: Usage error (unknown command or flag, bad argument or flag value); `wiki-engine check` also uses `2` for git or setup errors (outside git, invalid base ref)
- With `--json`, `ok` is `false` whenever the exit code is non-zero; empty lists encode as `[]`, never `null`

## Related

- [config.md](config.md) — `.wikirc` configuration reference.
- [repo-map.md](repo-map.md) — repository architecture and CLI command reference.
- [operations/lint.md](../operations/lint.md) — lint checkers and mechanical fixes.
