---
status: current
description: "Workflow for searching and answering questions from the wiki first."
superseded_by: ""
---
# Query Workflow

## Goal

Answer repo questions from the wiki first so the agent does not start from zero every time.

## Procedure

1. Map the wiki with `wiki-engine context --active --sort=topo`, then read [index.md](../index.md).
2. Read the latest relevant entries in [log.md](../prologue/log.md).
3. Search the wiki for the topic and follow the graph's `->` links to related pages.
4. Read only the linked pages needed to answer the question.
5. Read source files only when the wiki lacks enough evidence.
6. If the answer is durable, write it back into the wiki.

## Shell-First Search

```bash
wiki-engine context --active --sort=topo
wiki-engine search <keyword>
wiki-engine relevant <keyword>
wiki-engine list
wiki-engine headings
```

## Durable Answer Rule

File the answer back into the wiki when it is any of these:

- A stable architecture explanation.
- A repo workflow that will be reused.
- A non-obvious cross-file connection.
- A limitation, exclusion, or decision that future sessions should not rediscover.
- The page holding the answer cross-links to its related pages (see [schema.md](../prologue/schema.md)).

File the answer back using the [ingest workflow](ingest.md) and the durable knowledge rules in [schema.md](../prologue/schema.md).
