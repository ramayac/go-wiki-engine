---
name: wiki
description: >
  Maintain a repo-local wiki for durable project knowledge.
  Use for ingesting repo changes, querying architecture, refreshing
  stale docs, onboarding cold-start projects, and linting wiki health.
  Works with the wiki-engine CLI for filesystem-level inspection.
license: MIT
compatibility: "Requires wiki-engine CLI (go install github.com/ramayac/go-wiki-engine/cmd/wiki-engine@latest)"
---

# Wiki Maintainer

Maintain the repository wiki — the persistent knowledge layer that survives
beyond a single chat session. This skill provides shell-first plumbing
commands and repeatable workflows for wiki operations.

## Quick Reference — wiki-engine CLI

```bash
wiki-engine context           # Snapshot: catalog, recent log, phase status
wiki-engine list              # List all wiki files
wiki-engine search <term>     # Case-insensitive search across wiki
wiki-engine headings          # All Markdown headings with file paths
wiki-engine log-tail [n]      # Last N log entries
wiki-engine changed           # Non-wiki files changed in git diff
wiki-engine candidates        # Changed files worth ingesting (respects .wikirc ignore)
wiki-engine lint              # Full wiki health check (structure, links, markers, orphans)
wiki-engine refresh           # Combined snapshot: files + log + changed + candidates + lint
wiki-engine stats             # File count, heading count, last-updated
wiki-engine relevant <query>  # Rank pages by relevance to a query
```

## Workflows

### 1. Ingest Changes (`/wiki-ingest`)
Absorb repo changes into the wiki.
1. Run `wiki-engine context` and `wiki-engine candidates`.
2. Read only the changed source files relevant to the ingest.
3. Update or create wiki pages with durable facts.
4. Update `wiki/index.md` if page coverage changed.
5. Append a dated entry to `wiki/log.md`.
6. Run `wiki-engine lint`.

### 2. Query the Repo (`/wiki-query`)
Answer questions from the wiki first.
1. Run `wiki-engine search <term>` for the topic.
2. Read only the wiki pages with matches.
3. Use source files only if the wiki lacks evidence.
4. File durable answers back into the wiki.

### 3. Refresh Wiki (`/wiki-refresh`)
Periodic maintenance cycle.
1. Run `wiki-engine refresh`.
2. If no ingest candidates, stop — no update needed.
3. Otherwise, update stale pages, index, and log.
4. Run `wiki-engine lint`.
5. Summarize what changed and what still needs review.

### 4. Onboard Cold Start (`/wiki-onboard`)
Bootstrap a wiki for a brand-new project or empty wiki.
1. Run `wiki-engine candidates` (fall back to manual repo survey).
2. Check for external docs outside `wiki/` to migrate.
3. Fill in `wiki/repo-map.md` completely.
4. Create initial topic pages (architecture, data-model, api, etc.).
5. Update `wiki/index.md` and `wiki/phases.md`.
6. Append an ingest entry to `wiki/log.md`.
7. Run `wiki-engine lint`.

## Canonicial Instructions

Full workflow definitions live in `.wiki-instructions/` — edit those files
to update prompts for all supported tools (GitHub Copilot, Claude Code, pi).
Run `wiki-engine sync-prompts` after upgrading the wiki-engine binary.

## What Makes a Good Wiki Page

- **Durable over ephemeral.** Facts that survive 10+ commits.
- **One concern per file.** Split when a page covers two unrelated subsystems.
- **Grep-friendly headings.** Terms that appear in source code.
- **Link, don't duplicate.** Reference `repo-map.md` rather than repeating.
