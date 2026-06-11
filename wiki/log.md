---
status: current
description: "Append-only timeline of wiki maintenance activity."
superseded_by: ""
---
# Wiki Log

Append-only timeline of wiki maintenance activity.

## [2026-05-28] ingest | summarize mode, config docs, coverage, cache limit

- Added `--summarize` flag to `wiki-engine context` for progressive disclosure (opt-in via `context_summarize`)
- Added `.wiki-instructions/summarize.md` — separate agent prompt for large wikis
- Added `wiki/config.md` — full `.wikirc` configuration reference page
- Added `.wikirc.example` with all 10 config keys documented
- Added `cache_max_mb` to cap `.wiki/.cache.json` size
- Coverage: config 63→97%, engine 54→68%, scaffold 82%
- Fixed cache self-invalidation bug (`.cache.json` mtime skew)

## [2026-05-28] ingest | hardening — duplicate detection, stale content, watch mode, cache, diff, integration tests

- Added `duplicate-content` and `stale-content` lint checkers (configurable via `.wikirc`)
- Added `external-links` checker to validate wiki→source-file references
- Added `wiki-engine diff <from> <to>` — git-aware wiki change report
- Added `wiki-engine watch [--once]` — polling change detection + lint
- Added `wiki-engine impact <file...>` — maps changed files to wiki pages
- Added `.wiki/.cache.json` with mtime-based invalidation (`cache_enabled` config)
- Added `duplicate_threshold`, `stale_days`, `watch_interval`, `cache_enabled` to `.wikirc`
- Added watch agent prompt (`.wiki-instructions/watch.md`)
- Added 14-scenario integration test suite (`test/integration_test.sh`)
- Bootstrap: created `.pi/skills/wiki/SKILL.md` via `wiki-engine sync-prompts`
- Updated `wiki/repo-map.md` with new commands, config keys, architecture entries
- Fixed heading hierarchy checker to skip HTML comments
- Updated README.md for all commands, config keys, pi.dev support

## [2026-05-28] ingest | smart context, enhanced lint, pi.dev support, new commands

Major feature batch: composable lint checker system (9 checkers), four new CLI commands (stats, context, summary, relevant), pi.dev Agent Skills integration, and `--json` output support across all commands.

**Added:**
- `internal/engine/engine_lint.go` — Checker interface with 9 implementations: required-files, index-links, cross-page-links, orphans, heading-hierarchy, log-headings, log-chronology, markers, phase-consistency. `Lint()` now composes checkers, reports with severity levels (error/warn/info), and returns structured `Issue` objects.
- `internal/engine/engine.go` — new types and methods: `Stats()`, `Context()`, `Summary()`, `Relevant()`, `JSONOutput` helpers.
- `cmd/wiki-engine/main.go` — 5 new subcommands: `stats`, `context [--minimal]`, `summary <page>`, `relevant <query> [n]`. `--json` flag support on all commands.
- `scaffold/.pi/skills/wiki/SKILL.md` — pi.dev Agent Skills standard skill wrapping all wiki workflows.
- `scaffold/.wiki-instructions/*` — updated all workflow prompts to use `wiki-engine context` instead of listing 4+ files to read (reduces context tokens ~80%).
- `.github/workflows/test.yml` — CI pipeline: vet, test, build on push/PR.
- `wiki/todo.md` — 32-task improvement backlog ranked by difficulty.

**Source changes:**
- `internal/engine/engine.go` — removed monolithic `Lint()`, added `Context()`, `Summary()`, `Relevant()`, `Stats()`
- `internal/engine/engine_lint.go` (new)
- `cmd/wiki-engine/main.go` — JSON output, new commands
- `internal/scaffold/scaffold.go` — added `files/.pi` to `SyncPrompts`
- `internal/scaffold/scaffold_test.go` — pi skill verification
- `scaffold/.pi/skills/wiki/SKILL.md` (new)
- `scaffold/.wiki-instructions/*` — updated context sections

**Changed:** `Lint()` now returns `LintResult{OK, Messages, Issues}` — backward compatible via `Messages` field. Cross-page link checker strips inline code to avoid false positives.

**Needs human review:** agy CLI integration (deferred — needs research on agy's command format).

Triggered by a gap analysis that revealed the intelligence layer (prompts, instructions) was Copilot-specific. Claude Code users had no guided workflows.

**Added:**
- `scaffold/.wiki-instructions/` — canonical workflow definitions. Six files: ingest, query, refresh, onboard, migrate-shims, wiki-maintainer.
- `scaffold/.claude/commands/` — Claude Code custom slash commands (symlinks to `.wiki-instructions/`).
- `syncEmbeddedDir()` helper in `internal/scaffold/scaffold.go` — extracted from `SyncPrompts()`, now syncs `.wiki-instructions/`, `.github/`, and `.claude/commands/`.
- `Makefile` — `sync-scaffold` now uses `cp -rL` to dereference symlinks (go:embed resolves symlinks at build time, but `cp -rL` makes the embedded FS self-contained).
- Tests updated: `TestInit` and `TestSyncPrompts` now verify `.wiki-instructions/` and `.claude/commands/` files.

**Source changes that drove this entry:**
- `scaffold/.wiki-instructions/*` (new), `scaffold/.claude/commands/*` (new)
- `scaffold/.github/prompts/*` — regular files → symlinks to `.wiki-instructions/`
- `scaffold/.github/instructions/*` — regular file → symlink
- `scaffold/AGENTS.md`, `scaffold/CLAUDE.md` — updated shim text to reference both tools
- `internal/scaffold/scaffold.go` — `SyncPrompts()` refactored
- `internal/scaffold/scaffold_test.go` — expanded checks
- `cmd/wiki-engine/main.go` — CLI messages updated
- `internal/upgrade/upgrade.go` — post-upgrade message updated
- `internal/config/config_test.go` — fixed duplicate `package config` bug
- `README.md` — documented new structure and supported tools table

**Changed:**
- `.github/prompts/` and `.github/instructions/` are now symlinks in both scaffold/ and live project.
- `cp -rL` replaces `cp -r` in make sync-scaffold.
- `SyncPrompts` scope expanded from `.github/` only to all three instruction layers.

**Key design principle:** Use symlinks for DRY, not duplicated files. `go:embed` follows symlinks at compile time so no runtime changes needed. Tool-specific directories are symlink shims pointing to a single canonical source.

**Needs human review:** Verify that `make build` and `wiki-engine init` still produce correct output in a clean target repo.

## [2026-04-17] ingest | AI entrypoint gap (shim files)

Triggered by recognising that non-Copilot AI tools (Claude Code, cursor, etc.) have no path to the wiki without AGENTS.md/CLAUDE.md shims.

**Added:**
- `scaffold/AGENTS.md` and `scaffold/CLAUDE.md` — shim templates that redirect to `wiki/index.md`.
- `syncShims()` internal helper in `internal/scaffold/scaffold.go` — create-only semantics, called by both `Init()` and `SyncPrompts()`.
- 4 new tests in `internal/scaffold/scaffold_test.go` (9 total, all passing).
- `wiki/lessons.md` — 4th entry: AI entrypoint gap.
- `wiki/repo-map.md` — updated scaffold section, high-signal area, and init subcommand description.

**Source changes that drove this entry:**
- `scaffold/AGENTS.md` (new), `scaffold/CLAUDE.md` (new)
- `internal/scaffold/scaffold.go` — `syncShims()` added, wired into `Init()` and `SyncPrompts()`
- `cmd/wiki-engine/main.go` — init success message updated

**Changed:** nothing removed.

**Needs human review:** Run `wiki-engine sync-prompts` in Mana-world-shift to install the new shims there (AGENTS.md already exists as a stub — it will be preserved).

## [2026-04-16] ingest | sync-prompts gap, cold-start gap, external-docs gap

Triggered by a full onboarding session on Mana-world-shift and the resulting improvements fed back into go-wiki-engine.

**Added:**
- `wiki/lessons.md` (new) — three design insights: prompt-upgrade gap, cold-start/incremental confusion, external-docs visibility.
- `wiki/index.md` — added lessons.md entry; updated repo-map.md description.
- `wiki/repo-map.md` — added `sync-prompts` subcommand row; updated `SyncPrompts()` in High-Signal Areas; added step 5 to the Copilot workflow (run sync-prompts after upgrade).

**Source changes that drove this entry:**
- `internal/scaffold/scaffold.go` — new `SyncPrompts()` function
- `cmd/wiki-engine/main.go` — new `sync-prompts` subcommand
- `internal/upgrade/upgrade.go` — post-upgrade reminder message
- `scaffold/.github/prompts/wiki-onboard.prompt.md` — new cold-start prompt
- `scaffold/.github/prompts/wiki-ingest.prompt.md` — cold-start detection hint
- `scaffold/.github/instructions/wiki-maintainer.instructions.md` — expanded guidance
- `scaffold/wiki/operations/ingest.md` — added cold-start and external-docs steps
- `scaffold/wiki/repo-map.md` — improved placeholder hints

**Changed:** nothing removed.

**Needs human review:** nothing.



- Bootstrapped wiki scaffold via `wiki-engine init`.
- Wrote `wiki/repo-map.md` with full architecture: subcommand inventory, Copilot integration model (prompts vs instructions vs CLI), .wikirc config table, build and release path, and exclusion rules.
- Corrected `.wikirc` default diff base from `main...HEAD` to `master...HEAD` to match the repo's default branch.
- Updated `wiki/phases.md` status board: phases 0–2 now marked completed.
- Source: `README.md`, `AGENTS.md`, `cmd/wiki-engine/main.go`, `internal/engine/engine.go`, `internal/scaffold/scaffold.go`, `internal/config/config.go`.
