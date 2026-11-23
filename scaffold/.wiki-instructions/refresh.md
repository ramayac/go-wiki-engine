---
description: "Run the repo wiki maintenance cycle. Use when: refreshing wiki state, reviewing repo changes, updating wiki pages, or running a wiki health check."
name: "Wiki Refresh"
argument-hint: "Optional focus area, file, or subsystem..."
agent: "agent"
---

Run the repository wiki refresh workflow.

## Required context

- Start from active context: run `wiki-engine context --active`.
- Check each page's `lifecycle` before relying on it; ignore `planned` content as current fact and follow `superseded_by` links.
- Use `wiki-engine refresh` and `wiki-engine check` for the full maintenance and citation report.
- Follow the guidelines in [wiki-maintainer.md](wiki-maintainer.md).
- If wiki-engine is not installed, read [wiki/index.md](../../wiki/index.md) and [wiki/repo-map.md](../../wiki/prologue/repo-map.md).

## Execution steps

1. Run `wiki-engine refresh` and `wiki-engine check`.
2. If there are no ingest candidates, no broken citations, and no pages to review, stop and explain that no wiki update is needed.
3. Review `wiki-engine changed`, `wiki-engine candidates`, and `wiki-engine impact`.
4. Update the relevant active pages under `wiki/` (with typed YAML frontmatter, explicit `lifecycle`, exact `sources:`, and cross-links).
5. Update [wiki/index.md](../../wiki/index.md) if coverage or lifecycle changed.
6. Append a dated entry to [wiki/log.md](../../wiki/prologue/log.md).
7. Run `wiki-engine lint` and `wiki-engine check` before finishing.
8. Summarize what changed, which source files drove the change, and any remaining gaps.
