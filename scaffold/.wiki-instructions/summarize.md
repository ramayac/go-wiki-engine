---
description: "Load wiki context with page summaries to avoid reading every file. Use when: the wiki has many or large pages, token budget is tight, or `context_summarize` is enabled in .wikirc. Supplements, not replaces, the standard ingest/query/refresh workflows."
name: "Wiki Summarize"
argument-hint: "none"
agent: "agent"
---

Load wiki context with page summaries for progressive disclosure.

> **Prerequisite:** `context_summarize = true` must be set in `.wikirc`. Without it, use the standard lightweight `wiki-engine context` instead.

## Why this exists

Standard `wiki-engine context` returns a one-line catalog — the agent must blindly read pages to know what's inside. For wikis with large pages (100+ lines), this wastes tokens.

This prompt teaches **progressive disclosure**:

1. **Catalog + summaries** (`wiki-engine context --summarize`) — every page's first heading, first paragraph, and line count. Typically ~2K tokens for a 10-page wiki.
2. **Preview** (`wiki-engine summary <page>`) — first heading + first paragraph for a single page. Used when a page looks borderline relevant.
3. **Full read** — only when the summary confirms the page has actionable information.

## Execution steps

1. Run `wiki-engine context --summarize`.
2. For each catalog entry, check:
   - `line_count` ≤ 50: the summary is likely sufficient; read the full page only if the summary mentions something actionable.
   - `line_count` > 50: the summary is a preview. Decide whether the page is relevant to the current task.
3. For borderline pages, run `wiki-engine summary <page>` for a richer preview before committing to a full read.
4. Read full pages only when the summary confirms they contain needed information.
5. Proceed with the standard workflow (ingest, query, or refresh) using only the pages you've actually read.

## Token budget heuristics

| Wiki size | Strategy |
|-----------|----------|
| ≤ 5 pages, all < 50 lines | Skip summaries, just read everything |
| 5–15 pages | `context --summarize` (~2K tokens), read top 3–5 relevant pages |
| 15+ pages | `context --summarize` + `wiki-engine relevant <query>` to filter, then read top pages |
| Any page > 200 lines | Always preview with `wiki-engine summary <page>` before full read |

## When NOT to use this

- Wiki has only 3–4 small pages — summaries add overhead with no benefit.
- The task explicitly requires full-wiki knowledge (e.g., a complete audit).
- `context_summarize` is `false` in `.wikirc` — use standard `wiki-engine context` instead.
