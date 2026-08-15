---
status: current
description: "Catalog of all wiki pages with one-line descriptions."
superseded_by: ""
---
# Wiki Index

## Core

- [README.md](README.md) | Purpose, rules, and shell-first navigation.
- [schema.md](schema.md) | Required structure and maintenance contract.
- [phases.md](phases.md) | Phase rollout plan.
- [repo-map.md](repo-map.md) | Current repo architecture, subcommands, multi-tool integration model, config, build path.
- [log.md](log.md) | Append-only timeline of wiki maintenance.
- [lessons.md](lessons.md) | Design insights and gap post-mortems from real usage sessions.
- [todo.md](todo.md) | Open improvement backlog ranked by difficulty.
- [improvement-plan.md](improvement-plan.md) | Deprecated — completed hardening roadmap; design decisions archived in lessons.md.
- [config.md](config.md) | Full `.wikirc` configuration reference.

## Operations

- [operations/ingest.md](operations/ingest.md) | How to absorb a repo change into the wiki.
- [operations/query.md](operations/query.md) | How to answer questions from the wiki first.
- [operations/lint.md](operations/lint.md) | How to health-check and repair wiki drift.

## Prompt Workflows

The slash-command prompts live outside the wiki in `.wiki-instructions/` (canonical source, symlinked into `.github/prompts/` and `.claude/commands/`). `/wiki-ingest`, `/wiki-query`, and `/wiki-lint` mirror the workflows under [operations/](operations/ingest.md); `/wiki-onboard`, `/wiki-refresh`, `/wiki-watch`, and `/wiki-upgrade` are documented in [repo-map.md](repo-map.md).
