---
id: query
title: Query Workflow
type: runbook
lifecycle: current
description: "Workflow for searching and answering questions from the wiki first."
---
# Query Workflow

## Goal

Answer repo questions from the wiki first so the agent does not start from zero every time.

## Procedure

1. Start from active context: `wiki-engine graph` or `wiki-engine context --active --sort=topo`, then inspect [index.md](../index.md).
2. Search with `wiki-engine search <term>` or `wiki-engine relevant <query>` (add `--include-history` only when historical context is needed).
3. Check each page's `lifecycle` before relying on it. Ignore `planned` content as current fact, and follow `superseded_by` links to the active replacement.
4. Read source files only when the wiki lacks enough evidence.
5. File durable answers back into the wiki using the [ingest workflow](ingest.md) and [schema.md](../prologue/schema.md), then run `wiki-engine lint` and `wiki-engine check`.

## Shell-First Search

```bash
wiki-engine graph
wiki-engine graph <page-or-id>
wiki-engine context --active --sort=topo
wiki-engine search <keyword>
wiki-engine relevant <query>
```
