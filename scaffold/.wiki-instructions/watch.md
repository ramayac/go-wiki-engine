---
description: "Monitor the repo for un-ingested changes. Use when: setting up continuous wiki maintenance, or before starting a work session to catch drift."
name: "Wiki Watch"
argument-hint: "none, or --once for a single check..."
agent: "agent"
---

Monitor the repository for changes that need wiki maintenance.

## Required context

- Start from active context: run `wiki-engine context --active`.
- Check each page's `lifecycle` before relying on it (ignore `planned` as current fact; follow `superseded_by` links).
- Follow the guidelines in [wiki-maintainer.md](wiki-maintainer.md).
- Read [wiki/operations/ingest.md](../../wiki/operations/ingest.md).

## Execution steps

### Continuous & One-shot mode

```bash
wiki-engine watch --once
wiki-engine check
```

For continuous polling, set `watch_interval` in `.wikirc` and run `wiki-engine watch`.

### Agent auto-ingest

When `wiki-engine watch` or `wiki-engine check` reports changed candidates or pages to review:

1. Run `wiki-engine context --active`.
2. Run `wiki-engine candidates` and `wiki-engine check`.
3. Run `wiki-engine impact <files...>` (exact source match by default; add `--fuzzy` only if needed).
4. Read the relevant changed source files.
5. Update active wiki pages with durable facts, explicit `lifecycle`, exact `sources:`, and standard relative Markdown links; set `lifecycle: superseded` with `superseded_by:` if retiring a page.
6. Append a dated entry to [wiki/log.md](../../wiki/prologue/log.md).
7. Run `wiki-engine lint` and `wiki-engine check`.

If no candidates have durable wiki impact, report "no wiki update needed" and move on.
