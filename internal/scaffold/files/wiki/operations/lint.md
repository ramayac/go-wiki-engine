---
status: current
description: "Workflow for running the wiki linter and repairing inconsistencies."
superseded_by: ""
---
# Lint Workflow

## Goal

Keep the wiki coherent, linked, and current.

## Checks

- Pages mentioned in the index still exist.
- Important repo areas have coverage.
- Stale claims are updated when source files changed.
- Exclusions still match repo reality.
- New recurring topics have a page instead of being trapped in chat history.
- Links are standard relative Markdown format, are not malformed (e.g., no wiki-style links `[[Page]]` or unclosed parentheses), and are correctly mapped to files that exist.
- Active pages are cross-linked: each page links to its related pages; the only intentional leaf is `log.md`.
- `leaf-pages` surfaces active pages with no outgoing links at info severity — non-failing by default, visible as a reminder.

## Shell-First Checks

```bash
wiki-engine lint
wiki-engine list
wiki-engine context --active
wiki-engine search "TODO:"
```

## Repair Order

1. Fix stale or incorrect topic pages.
2. Fix [index.md](../index.md) links or summaries.
3. Append a log entry to [log.md](../prologue/log.md) if the lint changed durable content.

## Log Format

Use this exact heading pattern:

```md
## [YYYY-MM-DD] lint | short summary
```

Pages touched by a lint repair should stay connected — see the [ingest workflow](ingest.md).
