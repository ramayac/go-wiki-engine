# Wiki Log

Append-only timeline of wiki maintenance activity.

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
