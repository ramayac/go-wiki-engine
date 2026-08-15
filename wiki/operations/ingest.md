---
status: current
description: "Workflow for incrementally absorbing repository changes into the wiki."
superseded_by: ""
---
# Ingest Workflow

## Goal

Absorb a repo change into the wiki without rediscovering the entire codebase.

## Procedure

1. Read [index.md](../index.md) and the latest entries in [log.md](../prologue/log.md).
2. Inspect the changed files first.
3. Ignore repo-specific excluded paths from `.wikirc` (see [config.md](../prologue/config.md)).
4. Decide whether the change updates an existing page or needs a new page.
5. Update the relevant wiki page with the durable facts only — and cross-link it to its related pages per [schema.md](../prologue/schema.md).
6. Update [index.md](../index.md) if page coverage changed.
7. Append an entry to [log.md](../prologue/log.md).

## Shell-First Inputs

```bash
wiki-engine changed
wiki-engine candidates
wiki-engine refresh
```

## Page Decision Rule

- Update an existing page when the change fits an existing concern.
- Create a new page when the change introduces a new subsystem, workflow, or recurring question.
- Follow the durable knowledge rules in [schema.md](../prologue/schema.md) when writing.
- Cross-link the updated page: add outgoing links to its related pages and link back from them where useful.

## Log Format

Use this exact heading pattern:

```md
## [YYYY-MM-DD] ingest | short summary
```

After updating pages, validate with the [lint workflow](lint.md).
