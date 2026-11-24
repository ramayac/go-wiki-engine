---
id: index
title: Wiki Index
lifecycle: current
description: "Catalog of all wiki pages with one-line descriptions."
---
# Wiki Index

## Prologue

- [README.md](README.md) | Purpose, rules, and shell-first navigation.
- [prologue/schema.md](prologue/schema.md) | Required structure and maintenance contract.
- [prologue/phases.md](prologue/phases.md) | Phase rollout plan.
- [prologue/repo-map.md](prologue/repo-map.md) | Current repo architecture, subcommands, multi-tool integration model, config, build path.
- [prologue/log.md](prologue/log.md) | Append-only timeline of wiki maintenance.
- [prologue/config.md](prologue/config.md) | Full `.wikirc` configuration reference.

## Decisions

- [decisions/lessons.md](decisions/lessons.md) | Design insights and gap post-mortems from real usage sessions.
- [decisions/todo.md](decisions/todo.md) | Open improvement backlog ranked by difficulty.
- [decisions/parity-improvement-plan.md](decisions/parity-improvement-plan.md) | Phased engineering parity plan covering containment, YAML typed model, single-read lint, citation checks, fail-closed upgrade, and CI hardening.
- [decisions/audit-v2-fixes.md](decisions/audit-v2-fixes.md) | Intentional contract changes from the second audit fix PR, and the limits left in place.
- [decisions/governance-parity.md](decisions/governance-parity.md) | Decision to port the governance features of a sibling project: tag vocabulary, `health`, six prompts, benchmark skill, additive JSON. Known limits and upgrade notes.
- [decisions/improvement-plan.md](decisions/improvement-plan.md) | [superseded] Deprecated — completed hardening roadmap; design decisions archived in lessons.md.

## Operations

- [operations/ingest.md](operations/ingest.md) | How to absorb a repo change into the wiki.
- [operations/query.md](operations/query.md) | How to answer questions from the wiki first.
- [operations/lint.md](operations/lint.md) | How to health-check and repair wiki drift.
- [operations/release.md](operations/release.md) | Release runbook: versioning policy and cut-and-verify steps.

## Prompt Workflows

The slash-command prompts live outside the wiki in `.wiki-instructions/` (canonical source, symlinked into `.github/prompts/` and `.claude/commands/`). The set is `/wiki-seed`, `/wiki-ingest`, `/wiki-query`, `/wiki-plan`, `/wiki-save`, and `/wiki-upgrade`. `/wiki-ingest` and `/wiki-query` mirror the workflows under [operations/](operations/ingest.md). `lint`, `refresh`, and `watch` are CLI commands, not prompts; see the subcommand table in [prologue/repo-map.md](prologue/repo-map.md). The README has the purpose of each prompt.
