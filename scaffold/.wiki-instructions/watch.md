---
description: "Monitor the repo for un-ingested changes. Use when: setting up continuous wiki maintenance, or before starting a work session to catch drift."
name: "Wiki Watch"
argument-hint: "none, or --once for a single check..."
agent: "agent"
---

Monitor the repository for changes that need wiki maintenance.

## Required context

- Read [wiki/index.md](../../wiki/index.md).
- Read [wiki/operations/ingest.md](../../wiki/operations/ingest.md).

## Execution steps

### Continuous mode

```bash
wiki-engine watch
```

This polls every N seconds (configured via `.wikirc` `watch_interval`). When changes are detected, it prints a summary of changed files, ingest candidates, and lint issues.

### One-shot mode

```bash
wiki-engine watch --once
```

Runs a single change + lint cycle and exits. Use this before starting work to check for un-ingested drift.

### Agent auto-ingest

When `wiki-engine watch` reports changed candidates:

1. Run `wiki-engine candidates` to get the filtered list.
2. For each candidate, run `wiki-engine impact <file>` to see which wiki pages are affected.
3. Read the changed source files that are relevant.
4. Update wiki pages with durable facts. Ensure all links are standard relative Markdown links (e.g., `[Text](file.md)`).
5. Append a dated entry to [wiki/log.md](../../wiki/log.md).
6. Run `wiki-engine lint`.

If no candidates have durable wiki impact, report "no wiki update needed" and move on.

## Config

Enable and tune in `.wikirc`:

```ini
watch_interval = 60   # seconds between polls (0 = disabled)
stale_days = 30        # warn on pages unchanged for N days
cache_enabled = true   # use .wiki/.cache.json for faster lookups
```
