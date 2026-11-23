---
id: README
title: Wiki
type: concept
lifecycle: current
description: "Overview of the wiki structure, rules, and navigation."
---
# Wiki

This directory is the repository-local knowledge base.

The wiki is the working memory for repo analysis, architectural summaries, operating procedures, and durable answers that should survive beyond a single chat session. It is meant to be updated incrementally instead of regenerated from zero.

## Rules

- Start from active context (`wiki-engine context --active`) before doing broad repo analysis.
- Check each page's `lifecycle` before relying on it; do not treat `planned`, `legacy`, or `superseded` pages as current fact.
- Keep files plain Markdown with stable filenames, unique `id` fields, and grep-friendly headings.
- Cite exact source files or directories in frontmatter `sources:` whenever documenting code.
- Append dated entries to [prologue/log.md](prologue/log.md) for ingest, query, and lint activity.

## Required Files

- [index.md](index.md) tracks the catalog.
- [prologue/log.md](prologue/log.md) tracks chronology.
- [prologue/schema.md](prologue/schema.md) defines the contract.
- [prologue/phases.md](prologue/phases.md) tracks rollout.
- [prologue/repo-map.md](prologue/repo-map.md) records the current repo model.
- [operations/](operations/ingest.md) holds repeatable workflows.

## Shell-First Navigation

```bash
wiki-engine context --active
wiki-engine graph
wiki-engine search <term>
wiki-engine relevant <query>
wiki-engine check
wiki-engine lint
```

## Update Loop

1. Read [index.md](index.md) or run `wiki-engine context --active`.
2. Read the last few entries in [prologue/log.md](prologue/log.md).
3. Read the relevant page under [operations/](operations/ingest.md).
4. Read only the topic pages needed for the task.
5. Read source files when the wiki lacks detail or needs verification.
6. Write durable findings back into the wiki, then run `wiki-engine lint` and `wiki-engine check`.
