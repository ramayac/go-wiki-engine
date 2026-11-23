---
id: ingest
title: Ingest Workflow
type: runbook
lifecycle: current
description: "Workflow for incrementally absorbing repository changes into the wiki."
---
# Ingest Workflow

## Goal

Absorb a repo change into the wiki without rediscovering the entire codebase.

## Procedure

1. Run `wiki-engine context --active` and read the latest entries in [log.md](../prologue/log.md).
2. If `wiki-engine changed` returns no output **and** `wiki/prologue/log.md` has no prior ingest entries, this is a cold start — run the full onboarding survey instead of an incremental ingest.
3. Inspect the changed files (`wiki-engine changed`, `wiki-engine candidates`, and `wiki-engine impact`).
4. Ignore repo-specific excluded paths from `.wikirc`.
5. Update or create topic pages with typed frontmatter (`id`, `title`, `type`, `lifecycle`, `description`, exact `sources:`) and cross-link related pages per [schema.md](../prologue/schema.md).
6. Update [index.md](../index.md) if page coverage or lifecycle changed.
7. Advance [phases.md](../prologue/phases.md) if phases were completed during this ingest.
8. Append a dated entry to [log.md](../prologue/log.md).
9. Run `wiki-engine lint` and `wiki-engine check` to verify structure and citations.

## Shell-First Inputs

```bash
wiki-engine changed
wiki-engine candidates
wiki-engine check
wiki-engine refresh
```

## Log Format

```md
## [YYYY-MM-DD] ingest | short summary
```
