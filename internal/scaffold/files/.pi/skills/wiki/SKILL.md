---
name: wiki
description: >
  Maintain a repo-local wiki for durable project knowledge.
  Use for ingesting repo changes, querying architecture, refreshing
  stale docs, onboarding cold-start projects, upgrading the wiki-engine
  CLI, monitoring drift, and linting wiki health.
  Works with the wiki-engine CLI for filesystem-level inspection.
license: MIT
compatibility: "Requires wiki-engine CLI (go install github.com/ramayac/go-wiki-engine/cmd/wiki-engine@latest)"
---

# Wiki Maintainer

Maintain the repository wiki — the persistent knowledge layer that survives
beyond a single chat session.

## Quick Reference — wiki-engine CLI

```bash
wiki-engine context --active                    # Active-page graph from index.md
wiki-engine context --active --sort=topo        # Hierarchical map: parents before children
wiki-engine context --summarize                 # Bounded snapshot with previews
wiki-engine graph [page-or-id] [--strict] [--dot] [--include-sources]
wiki-engine schema [--type=<t>] [--section=<s>] # Authoritative embedded schema & templates
wiki-engine search <term> [--tag=<t>] [--type=<t>] [--lifecycle=<l>] [--include-history] [--ranked]
wiki-engine relevant <query> [n]                # Field-weighted ranked search
wiki-engine list [--active]                     # List wiki files
wiki-engine headings                            # All Markdown headings
wiki-engine log-tail [n]                        # Last N log entries
wiki-engine changed [--base=<ref>] [--no-working-tree]
wiki-engine candidates [--base=<ref>] [--no-working-tree]
wiki-engine check [--base=<ref>] [--no-working-tree]  # Branch citation & drift check
wiki-engine impact [--fuzzy] <file...>          # Exact source impact (or --fuzzy prose)
wiki-engine lint [--check=<a,b>] [--skip=<a,b>] [--fix]
wiki-engine refresh [diff-range]                # Combined maintenance snapshot
wiki-engine stats                               # Aggregate wiki statistics
wiki-engine health                              # Advisory maintenance signals (never fails CI)
wiki-engine diff <from> <to>                    # Wiki changes between two git refs
wiki-engine watch [--once]                      # Poll for changes + lint issues
wiki-engine init [--wiki-dir=<name>]            # Idempotent bootstrap/reconcile or rename
wiki-engine upgrade [--from-source]             # Fail-closed verified upgrade
```

## Page Lifecycle & Schema Rules

Every wiki page has YAML frontmatter parsed with a real YAML parser. Missing metadata never defaults to active.

```yaml
---
id: page-stem
title: Page Title
type: concept            # concept | entity | incident | decision | runbook
lifecycle: current       # current | wip | implemented | planned | legacy | superseded
description: "One-line summary"
superseded_by: ""        # required when lifecycle is superseded
sources:
  - file: internal/engine/graph.go
---
```

- Start from active context (`wiki-engine context --active`).
- Always check `lifecycle` before relying on a page.
- **Ignore `planned` content as current fact.**
- **Follow `superseded_by` links** to active replacements.
- Add exact `sources:` to changed pages.
- Run `wiki-engine lint` after writes and `wiki-engine check` after ingest or save.

## Workflows

1. **Ingest (`/wiki-ingest`):** `context --active` → `changed` / `candidates` / `check` / `impact` → update pages with exact `sources:` → update `index.md` + `log.md` → `lint` + `check`.
2. **Query (`/wiki-query`):** `graph` / `context --active --sort=topo` → `search` / `relevant` → read active pages (ignore `planned`, follow `superseded_by`) → widen to `sources:` only if needed.
3. **Refresh (`/wiki-refresh`):** `refresh` + `check` → update impacted pages → `lint` + `check`.
4. **Onboard (`/wiki-onboard`):** `schema` → survey repo → migrate external docs → populate `repo-map.md` + typed pages → `lint` + `check` + `graph --strict`.
5. **Lint (`/wiki-lint`):** `lint --fix` → `lint --json` + `check --json` → resolve remaining issues → `lint` + `check` + `graph --strict`.
6. **Upgrade (`/wiki-upgrade`):** `upgrade` → `init` (reconcile prompts) → `lint` + `check`.
7. **Watch (`/wiki-watch`):** `watch --once` + `check` → `impact` → ingest → `lint` + `check`.
