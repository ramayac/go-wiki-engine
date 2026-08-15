---
status: current
description: "Contract and standards for wiki pages, lifecycle, and structure."
superseded_by: ""
---
# Wiki Schema

## Goal

The wiki is the persistent knowledge layer between the chat agent and the raw repository.

It should reduce repeated repo rediscovery by storing stable summaries, operating procedures, and durable answers in plain Markdown.

## Required Contract

Every repo that adopts this pattern should have at least these files:

- [README.md](../README.md)
- [index.md](../index.md)
- [log.md](log.md)
- `schema.md` (this file)
- [phases.md](phases.md)
- [repo-map.md](repo-map.md)
- [operations/ingest.md](../operations/ingest.md)
- [operations/query.md](../operations/query.md)
- [operations/lint.md](../operations/lint.md)

## Directory Structure

The wiki is organized into subdirectories. Only [index.md](../index.md) and
[README.md](../README.md) live at the root:

| Path | Role |
|---|---|
| `index.md` | Root catalog — the graph BFS start point. |
| `README.md` | Wiki rules and navigation. |
| `prologue/` | Wiki-about-the-wiki files: `schema.md`, `repo-map.md`, `phases.md`, `log.md`, `config.md`. |
| `operations/` | Repeatable workflow pages (`ingest.md`, `query.md`, `lint.md`). |
| `decisions/`, `architectures/`, … | Topic pages grouped by domain. Create new category directories as needed. |

The engine resolves the canonical files (`log.md`, `phases.md`, `schema.md`,
`repo-map.md`) at `prologue/<name>` first and falls back to the legacy
root-level paths, so older flat wikis keep working.

## Read Order

1. Read [index.md](../index.md).
2. Read the latest entries in [log.md](log.md).
3. Read the relevant [operations](../operations/ingest.md) page.
4. Read only the linked topic pages needed for the task.
5. Read source files only after the wiki has been consulted.

## Write Order

1. Update the topic page that changed.
2. Update [index.md](../index.md) if a page was added or its role changed.
3. Append a dated entry to [log.md](log.md).

## File Style

- Use plain Markdown.
- Prefer stable filenames over timestamped filenames, except for the log headings.
- Use grep-friendly headings and short lists.
- Prefer explicit relative links.
- Avoid generated JSON, vector indexes, or tool-specific metadata unless there is a clear need.

## Durable Knowledge Rules

- Put repeatable procedures in [operations/](../operations/ingest.md).
- Put repo facts in [repo-map.md](repo-map.md) or another topic page referenced by [index.md](../index.md).
- Put longer-lived decisions or answers into the wiki instead of leaving them only in chat history.
- Keep [log.md](log.md) append-only.

## Repo-Specific Exclusions

Each repo should document high-noise or user-authored areas that should not be routinely ingested in `.wikirc` under the `ignore` list — see [config.md](config.md).

## Cross-Linking Rules

The wiki is a navigable graph, not a pile of pages:

- Every active page links to the pages it depends on or extends (concepts, workflows, configuration).
- `index.md` is the root catalog; every page must be reachable from it.
- Reference other pages with standard relative Markdown links instead of bare filenames.
- `log.md` is the only intentional leaf — it is append-only: pages link to it, it never links back.
- After creating or updating pages, run `wiki-engine context --active` and confirm the page is connected; fix any pages reported as unlinked.
- `wiki-engine lint` reminds you via the `leaf-pages` check (info severity) when an active page has no outgoing links.

## Page Lifecycle & Front Matter

Every wiki page must include YAML front matter specifying its status and a brief description:

```yaml
---
status: current          # planned | current | legacy | deprecated
description: "One-line summary of this page's purpose"
superseded_by: ""        # (Conditional) path to the replacing page if status is deprecated
created: "2026-01-15"     # (Optional) creation date used by chronological graph sorting
updated: "2026-08-14"     # (Optional) last-updated date used by chronological graph sorting
tags: ["foo", "bar"]     # (Optional) single-line tag list
---
```

Optional fields:
- `created` / `updated` — `YYYY-MM-DD` dates used by `wiki-engine context --active --sort=chrono`; filesystem mtime is the fallback when absent.
- `tags` — single-line bracket list; reserved for future filtering.

### Status Definitions

- **`planned`**: The page is a placeholder or has been proposed but the content/implementation is not yet written. Agents can write to these.
- **`current`**: The page contains active, valid, and up-to-date documentation. Agents can read and write to these.
- **`legacy`**: The page is outdated but still kept for historical context. Agents should not read this page unless requested with `--all`.
- **`deprecated`**: The page is fully obsolete and has been superseded. If a page is deprecated, it must define `superseded_by` pointing to the new replacement page. Agents skip these entirely.

### Agent Visibility

| Status | Agent Reads? | Agent Writes? | Shows in `context`? |
|--------|--------------|---------------|---------------------|
| `planned` | ✅ Yes | ✅ Yes (to fill in) | ✅ Marked as planned |
| `current` | ✅ Yes | ✅ Yes | ✅ Default (active) |
| `legacy` | ❌ No | ❌ No | ⚠️ Only in plain `context` (excluded from `--active` and `--summarize`) |
| `deprecated` | ❌ No | ❌ No | ⚠️ Only in plain `context` (excluded from `--active` and `--summarize`) |
