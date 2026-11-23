---
description: "Bootstrap the wiki for a brand-new project, or recover a stale/empty wiki from scratch. Use when: the wiki is empty or only has template placeholders, there has been no prior ingest, or the repo has never had a wiki. NOT for incremental updates — use Wiki Ingest for that."
name: "Wiki Onboard"
argument-hint: "Optional: specific subsystems or files to prioritise..."
agent: "agent"
---

Run a full-project wiki onboarding. This is a **cold-start** survey — do not rely on `wiki-engine changed` having output.

## Required context

- Start from active context: run `wiki-engine context --active` to check whether the wiki has been populated.
- Inspect the embedded schema templates with `wiki-engine schema` and `wiki-engine schema --type <concept|entity|incident|decision|runbook>`.
- Read [wiki/phases.md](../../wiki/prologue/phases.md) — check which phases are still `not-started`.
- Read each page's `lifecycle` before relying on it: ignore `planned` placeholders as current fact and follow `superseded_by` links.
- If wiki-engine is not installed, read [wiki/index.md](../../wiki/index.md) instead.

## Execution steps

1. **Discover the repo shape:** Run `wiki-engine candidates` or survey top-level directories, `README.md`, `Makefile`, and build manifests.
2. **Check for external knowledge to migrate:** Scan `docs/`, `AGENTS.md`, `CLAUDE.md`, `CONTRIBUTING.md`, `ARCHITECTURE.md` and migrate durable knowledge into `wiki/` (leaving stubs or reconciled fenced shims via `wiki-engine init`).
3. **Populate [wiki/repo-map.md](../../wiki/prologue/repo-map.md):** Fill in Purpose, High-Signal Areas, Generated Artifacts, Build and Run Path, and Ignored Paths.
4. **Create initial typed topic pages:** Include valid YAML frontmatter (`id`, `title`, `type`, `lifecycle: current`, `description`, and exact `sources:`) and standard relative Markdown links.
5. **Update [wiki/index.md](../../wiki/index.md)** and advance [wiki/phases.md](../../wiki/prologue/phases.md).
6. **Append to [wiki/log.md](../../wiki/prologue/log.md)** using `## [YYYY-MM-DD] ingest | initial full-repo wiki build`.
7. **Verify:** Run `wiki-engine lint`, `wiki-engine check`, and `wiki-engine graph --strict` and fix any issues before finishing.

Finish by summarising what was created, what was migrated, and what still needs human review.
