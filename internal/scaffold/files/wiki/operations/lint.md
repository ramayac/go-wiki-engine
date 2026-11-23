---
id: lint
title: Lint Workflow
type: runbook
lifecycle: current
description: "Workflow for running the wiki linter and repairing inconsistencies."
---
# Lint Workflow

## Goal

Keep the wiki coherent, linked, cited, and current.

## Shell-First Checks

```bash
wiki-engine lint
wiki-engine lint --fix
wiki-engine check
wiki-engine graph --strict
```

## Repair Order

1. Run `wiki-engine lint --fix` for deterministic mechanical fixes (legacy frontmatter migration, resolvable wiki links, lifecycle banners, index badges, section ordering, and inline reference migration).
2. Fix remaining structural, relationship (`related` / `depends_on`), or broken citation issues reported by `wiki-engine lint` and `wiki-engine check`.
3. Update [index.md](../index.md) and append a dated entry to [log.md](../prologue/log.md) if durable content changed.
4. Re-run `wiki-engine lint`, `wiki-engine check`, and `wiki-engine graph --strict`.

Pages touched by a lint repair should stay connected — see the [ingest workflow](ingest.md).
