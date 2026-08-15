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
- Thresholds and the severity gate come from `.wikirc` (see [config.md](../config.md)).
- Exclusions still match repo reality.
- New recurring topics have a page instead of being trapped in chat history.
- Links follow the standard in [schema.md](../schema.md): relative Markdown, no wiki-style `[[Page]]` links, no unclosed parentheses, and every target exists.

## Shell-First Checks

```bash
wiki-engine lint
wiki-engine list
wiki-engine search "TODO:"
```

## Repair Order

1. Fix stale or incorrect topic pages.
2. Fix [index.md](../index.md) links or summaries.
3. Append a log entry to [log.md](../log.md) if the lint changed durable content.

## Log Format

Use this exact heading pattern:

```md
## [YYYY-MM-DD] lint | short summary
```

Pages touched by a lint repair should stay connected — see the [ingest workflow](ingest.md).
