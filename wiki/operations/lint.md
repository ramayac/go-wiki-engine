---
id: lint
title: Lint Workflow
lifecycle: current
description: "Workflow for running the wiki linter and repairing inconsistencies."
sources:
  - file: internal/engine/engine_lint.go
  - file: cmd/wiki-engine/main.go
---
# Lint Workflow

## Goal

Keep the wiki coherent, linked, and current.

## Checks

- Pages mentioned in the index still exist.
- Important repo areas have coverage.
- Stale claims are updated when source files changed.
- Thresholds and the severity gate come from `.wikirc` (see [config.md](../prologue/config.md)).
- Exclusions still match repo reality.
- New recurring topics have a page instead of being trapped in chat history.
- Links follow the standard in [schema.md](../prologue/schema.md): relative Markdown, no wiki-style `[[Page]]` links, no unclosed parentheses, and every target exists. Links are strictly page-relative — a page in a subdirectory must use `../` to reach pages in other directories.
- `vocabulary` (part of the `schema` group) rejects page tags that are not listed in `<wiki_dir>/vocabulary.yaml`. Tags stay free-form while that file lists none. Run `wiki-engine schema --section vocabulary` for the format.
- Active pages are cross-linked per [schema.md](../prologue/schema.md): each page links to its related pages; the only intentional leaf is [log.md](../prologue/log.md).
- `markers` warns on placeholder text: a to-do, to-be-decided, or unknown label followed by a colon. It skips the open `> Rationale: not recorded.` marker and the `> ⚠️ CONTRADICTION:` flag; `wiki-engine health` reports those and lint never fails on them. Run `wiki-engine schema --section conventions` for the rules.
- `leaf-pages` surfaces active pages with no outgoing links at info severity — non-failing by default, visible as a reminder.
- `orphans` surfaces active pages not reachable from the index; `legacy` and `deprecated` pages live outside the graph and are exempt (same lifecycle rule as `leaf-pages`).

## Shell-First Checks

```bash
wiki-engine lint
wiki-engine list
wiki-engine context --active
wiki-engine search "TODO:"
make audit
```

## Repair Order

1. Fix stale or incorrect topic pages.
2. Fix [index.md](../index.md) links or summaries.
3. Append a log entry to [log.md](../prologue/log.md) if the lint changed durable content.
4. If pages moved between directories or wiki files were renamed, run `make audit` and fix strict page-relative links and stale `wiki/<path>.md` prose references.

## Log Format

Use this exact heading pattern:

```md
## [YYYY-MM-DD] lint | short summary
```

Pages touched by a lint repair should stay connected — see the [ingest workflow](ingest.md).
