# Wiki Log

Append-only timeline of wiki maintenance activity.

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
