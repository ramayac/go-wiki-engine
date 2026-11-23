---
description: "Answer a repository question from the wiki first. Use when: querying architecture, repo workflows, exclusions, or durable decisions before scanning source files."
name: "Wiki Query"
argument-hint: "Ask a repository question..."
agent: "agent"
---

Answer the user's repository question from the wiki first.

## Required context

- Start from active context: run `wiki-engine graph` or `wiki-engine context --active --sort=topo`. Use `wiki-engine graph <page-or-id>` to inspect one page's backlinks, outgoing links, sources, and references.
- Search with `wiki-engine search <term>` or `wiki-engine relevant <query>` (excludes historical pages by default; pass `--include-history` only when historical context is requested).
- Read each page's `lifecycle` before relying on it: **ignore `planned` content as current fact**, and **follow `superseded_by` links** whenever you encounter a `superseded` page.
- Follow the guidelines in [wiki-maintainer.md](wiki-maintainer.md).
- If wiki-engine is not installed, read [wiki/index.md](../../wiki/index.md) and [wiki/log.md](../../wiki/prologue/log.md) instead.

## Execution steps

1. Map first, then search: run `wiki-engine graph` (or `wiki-engine context --active`), locate the topic with `wiki-engine search <term>` or `wiki-engine relevant <query>`, and follow the graph's `->` links to related pages.
2. Read only the active wiki pages needed to answer the question. Do not rely on `planned`, `legacy`, or `superseded` pages as current fact; follow `superseded_by` links to the current page.
3. Use source files (starting from the page's declared `sources:`) only if the active wiki pages lack enough evidence.
4. If the answer reveals a durable repo fact that is missing or stale in the wiki, update the relevant active page (including exact `sources:`, valid `lifecycle`, and cross-links).
5. If durable wiki content changed, append a dated entry to [wiki/log.md](../../wiki/prologue/log.md) and run `wiki-engine lint` and `wiki-engine check`.

In the final response:

- Answer the question directly.
- State whether the answer came fully from the wiki or required widening to source files.
- Mention any wiki updates that were made.
