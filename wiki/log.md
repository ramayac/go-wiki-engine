# Wiki Log

Append-only timeline of wiki maintenance activity.

## [2026-04-16] bootstrap | initial repo-map and wiki baseline

- Bootstrapped wiki scaffold via `wiki-engine init`.
- Wrote `wiki/repo-map.md` with full architecture: subcommand inventory, Copilot integration model (prompts vs instructions vs CLI), .wikirc config table, build and release path, and exclusion rules.
- Corrected `.wikirc` default diff base from `main...HEAD` to `master...HEAD` to match the repo's default branch.
- Updated `wiki/phases.md` status board: phases 0–2 now marked completed.
- Source: `README.md`, `AGENTS.md`, `cmd/wiki-engine/main.go`, `internal/engine/engine.go`, `internal/scaffold/scaffold.go`, `internal/config/config.go`.
