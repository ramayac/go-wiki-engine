---
id: governance-parity
title: Governance Parity
type: decision
status: accepted
lifecycle: current
description: "Decision to port the governance features of a sibling internal project: tag vocabulary, health command, six prompts, benchmark skill, and additive JSON."
created: "2026-11-23"
updated: "2026-11-23"
sources:
  - file: internal/lint/
  - file: internal/engine/
  - file: internal/schema/schema.go
  - file: internal/scaffold/scaffold.go
  - file: internal/scaffold/retired_digests.go
  - file: cmd/wiki-engine/output.go
  - file: scaffold/.wiki-instructions/
  - file: scaffold/.claude/skills/wiki-benchmark/
related: [audit-v2-fixes, repo-map, schema]
---
# Governance Parity

## Context

A sibling internal project keeps a wiki with stricter governance than this tool enforced. It has a controlled tag list, advisory health signals, rationale and stop-condition rules, one prompt set, a benchmark, and a stable JSON contract. This tool had weaker versions of each. The goal was to give onboarded repos the same governance, without a breaking change where an additive one is enough.

## Alternatives Considered

- **Prompts: keep or align.** Keeping the old seven prompts changes nothing for users. It leaves an onboard flow that overlaps ingest, and no shared language or data-protection rules. We aligned the prompts.
- **JSON: breaking or additive.** A rewrite of the envelope gives a cleaner shape. It breaks every script that reads `--json`. We chose additive keys only, pinned by golden and compatibility tests.
- **Retired prompts: delete or back up.** Deleting every retired file is simple. It destroys user edits. Backing up every file leaves clutter. We delete a file that matches a shipped digest and back up the rest.

## Decision & Consequences

The tool now has these features. The README has the user-facing description of each.

| Feature | Name here | Where it lives |
|---|---|---|
| Controlled tag list | `vocabulary.yaml` (not `tags.yaml`) and the `vocabulary` lint check | `wiki-engine schema --section vocabulary` |
| Maintenance signals | `health`, next to `stats` (a superset, never a lint failure) | [repo-map.md](../prologue/repo-map.md) |
| Why and stop rules | `## Why` and `## Stops applying when` templates, `schema --section conventions` | [schema.md](../prologue/schema.md) |
| Prompt set | `seed`, `ingest`, `query`, `plan`, `save`, `upgrade` | `.wiki-instructions/` |
| Benchmark | `wiki-benchmark` Claude Code skill | `.claude/skills/wiki-benchmark/` |
| JSON | `meta`, `error_code`, and richer fields on `search`, `graph`, `health`, `check` | `wiki-engine schema --section json` |

The prompts `onboard`, `lint`, `refresh`, and `watch` are retired. The CLI commands `lint`, `refresh`, and `watch` stay. `/wiki-seed` replaces `/wiki-onboard`. The prompts share one language rule (STE), one data-protection rule, and the rationale marker. A cold start runs `seed`.

### Known limits

- The digests of retired files cover only versions in this repo's git history. An older edited copy is backed up, not deleted.
- `sync-prompts` backs up an unedited older skill file after an engine upgrade, because no digest history exists for the skill yet.
- Search results have no per-hit `matches` list.
- `check` findings have no `line` for source and commit findings.
- `meta` is absent on a pure error, which has no `data`.
- `search` and `graph` always carry the `type` and `title` keys. The value is an empty string when the page sets none.
- A user of `onboard` must move to `seed`.

### Upgrade notes

An onboarded repo runs these commands:

```bash
wiki-engine upgrade
wiki-engine init            # or: wiki-engine sync-prompts
wiki-engine lint --fix
wiki-engine health
```

`init` creates `vocabulary.yaml` and the benchmark skill, and adds `.wiki-engine/` to an existing `.gitignore`. Both `init` and `sync-prompts` delete a retired prompt that you did not edit and move an edited one to `<name>.bak`. Replace the example questions in `.claude/skills/wiki-benchmark/questions.md` before you run the benchmark. The exit-code and wiki-directory rules in [audit-v2-fixes.md](audit-v2-fixes.md) also apply.

## Stops applying when

A major release removes or renames one of these features, or the sibling project and this tool stop sharing a governance model.

## Related

- [audit-v2-fixes.md](audit-v2-fixes.md) - the fix PR that this work builds on.
- [CHANGELOG.md](../../CHANGELOG.md) - the full list of changes.
- [log.md](../prologue/log.md) - timeline entries for the work.
