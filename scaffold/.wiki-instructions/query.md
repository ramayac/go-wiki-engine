---
description: "Answer a repository question from the wiki first. Use when: querying architecture, repo workflows, exclusions, or durable decisions before scanning source files."
name: "Wiki Query"
argument-hint: "Ask a repository question..."
agent: "agent"
---

Answer the user's repository question from the wiki first.

## Required context

- Run `wiki-engine context` to get the current wiki snapshot.
- Search the wiki with `wiki-engine search <term>` or `wiki-engine relevant <term>`.
- Read only the wiki pages needed to answer the question.
- If wiki-engine is not installed, read [wiki/index.md](../../wiki/index.md) and [wiki/log.md](../../wiki/log.md) instead.

## Execution steps

1. Search the wiki using `wiki-engine search <term>` or equivalent targeted reads.
2. Read only the wiki pages needed to answer the question.
3. Use source files only if the wiki lacks enough evidence.
4. If the answer reveals a durable repo fact that is missing or stale in the wiki, update the relevant page.
5. If durable wiki content changed, append a dated entry to [wiki/log.md](../../wiki/log.md) and run `wiki-engine lint`.

In the final response:

- Answer the question directly.
- State whether the answer came fully from the wiki or required widening to source files.
- Mention any wiki updates that were made.
