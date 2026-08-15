---
status: current
description: "Append-only timeline of wiki maintenance activity."
superseded_by: ""
---
# Wiki Log

Append-only timeline of wiki maintenance activity.

## [2026-08-14] lint | document graph navigation and search in all instruction layers

- Audit found the graph feature was only taught as a health check, not as a map. Added a "Graph Navigation & Search" section to `wiki-maintainer.md`: hierarchical map (`context --active --sort=topo`), recency map (`--sort=chrono`), machine map (`--json context --active` nodes/edges/unlinked), following `->` edges, and map-then-search flow.
- Updated the query workflows (canonical prompt + live and scaffold `operations/query.md`) to map first, then search/relevant, then read along edges.
- Added `wiki-engine context --active --sort=topo` to `wiki/README.md` shell-first navigation and to the pi.dev skill quick reference.
- Propagated via `make sync-scaffold` + `wiki-engine sync-prompts`.

## [2026-08-14] lint | removed progress.md, slimmed todo.md to open backlog

- Deleted the root-level `progress.md` — its completion records are redundant with `wiki/log.md` and git history, and it was durable knowledge living outside the wiki.
- Rewrote `wiki/todo.md` to track only the 5 open items (agy integration #9, recursive .wikirc lookup #25, wiki templates #31, multi-repo aggregation #32, upgrade download-path tests #54). All completed tiers were dropped; history stays in `log.md`.
- Updated the index catalog description.

## [2026-08-14] lint | leaf-pages checker enforces wiki connectivity

- Added the `leaf-pages` checker (17th): flags active pages with no outgoing links at info severity — visible but non-failing by default (`fail_severity` defaults to `warn`). `log.md` is the only exempt leaf.
- CLI now prints info-level issues even when lint passes ("wiki lint OK (info issues above)") so non-failing reminders are not hidden.
- Cross-linked the scaffold wiki templates (README, schema, phases, repo-map, operations/\*) so a fresh `wiki-engine init` produces an already-connected wiki with zero leaf warnings.
- Documented the checker in `schema.md`, `operations/lint.md`, `repo-map.md`, `README.md`, and the canonical `.wiki-instructions/`; propagated via `make sync-scaffold` + `wiki-engine sync-prompts`.

## [2026-08-14] lint | enforce cross-linking in schema, operations, and instructions

- Added a "Cross-Linking Rules" section to `wiki/schema.md` and the scaffold template: every active page must link to its related pages, `index.md` remains the reachability root, `log.md` is the only intentional leaf, and `wiki-engine context --active` verifies connectivity after edits.
- Updated the ingest/query/lint operations docs (live + scaffold) with cross-link steps and checks so the rule is part of the repeatable procedures.
- Extended the canonical `.wiki-instructions/` maintainer checklist (rule 7) and the ingest/refresh/query/lint/watch prompts to cross-link on every write and verify the graph after linting.
- Propagated via `make sync-scaffold` + `wiki-engine sync-prompts`.

## [2026-08-14] lint | cross-linked wiki pages into a navigable graph

- Converted plain-text page references into standard relative Markdown links across `README.md`, `schema.md`, `phases.md`, `repo-map.md`, `config.md`, `lessons.md`, `todo.md`, and the three `operations/` pages.
- The active graph is no longer a star: pages now encode their conceptual relationships (`schema → operations/config`, `repo-map → config/lint/ingest`, `operations → index/log/schema/config`, etc.) so `wiki-engine context --active` gives agents real traversal paths.
- Replaced the stale `improvement-plan.md` reference in `todo.md` with links to `repo-map.md` and `lessons.md`.

## [2026-08-14] lint | reference-style link checker, improvement plan retirement

- Added a warn-level reference-style link check (`[text][ref]`) to the markdown-format checker with unit tests (improvement plan Goal 4, optional item).
- Deprecated `wiki/improvement-plan.md` (`superseded_by: repo-map.md`) after folding its Part 7 design decisions into `wiki/lessons.md`.

## [2026-08-14] ingest | 2026-08 audit fixes — release loop, cache removal, symlink distribution, git staleness

Triggered by the 2026-08-14 deep audit of the project concept, work loop, and gaps.

- `.github/workflows/release.yml` — assets packaged as `wiki-engine_<tag>_<goos>_<goarch>.{tar.gz,zip}` with a generated `checksums.txt`, matching what `wiki-engine upgrade` expects (the checksum verification path is now reachable).
- `internal/upgrade/upgrade.go` — shared HTTP client with 30s timeout.
- Removed `internal/engine/engine_cache.go` — the `.wiki/.cache.json` layer was write-only dead code; dropped `cache_enabled`/`cache_max_mb` config keys and `lint --rebuild-cache`.
- `internal/config/config.go` — `parseFloat` accepts 0 so `duplicate_threshold = 0` disables the checker; exported `ParsePositiveInt` (was duplicated in main.go).
- `cmd/wiki-engine/main.go` — `context` defaults to `context_summarize`; `watch` honors `watch_interval = 0` (exits with guidance, `--once` unaffected); `--json init` uses filtered args; lint JSON emits `ok:false` on failure; `impact` no longer blocks on an interactive stdin; usage text refreshed.
- `internal/scaffold/scaffold.go` — `init`/`sync-prompts` write tool-layer files as symlinks to `.wiki-instructions/` (regular-copy fallback where symlinks fail); `init` preserves an existing `.wikirc`; wired `/wiki-watch` into `.github/prompts/` and `.claude/commands/`.
- `internal/engine/engine_lint.go` — stale detection now uses git last-commit dates (mtime fallback outside git); heading checker skips fenced code blocks; inline-code detection is span-based instead of backtick counting; `Lint()` delegates to `LintWithOptions()`.
- `internal/engine/graph.go` — added `ActiveUnlinkedPages()`; `context --active` warns about active pages not reachable from `index.md`.
- `.github/workflows/test.yml` + `Makefile` — integration suite runs in CI (`make integration`); added a scaffold-sync drift guard.
- Wiki sweep — repo-map, config, schema, todo, progress, and the pi.dev skill refreshed; `.wikirc` templates standardized and cache keys removed.

## [2026-06-10] ingest | checksum verification, prompt restructures, bookkeeping

- Added checksum validation to self-upgrade, downloading GitHub release asset, computing SHA-256 hash, and verifying against checksums.txt.
- Completed Phase 3, restructured instructions under `.wiki-instructions/` to support `context --active`, integrated shims, and created `wiki-lint`.
- Completed Phase 4 CI check consolidation, simplifying workflows to run `make lint`.
- Completed Phase 5 code cleanup, removing dead helper functions (`jsonOK`, `jsonErr`, `cachedList`) and wrapping loop file handles with anonymous defers.

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
