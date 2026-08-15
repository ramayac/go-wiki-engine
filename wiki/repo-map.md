---
status: current
description: "Overview of the repository architecture, high-signal areas, and build instructions."
superseded_by: ""
---
# Repo Map

## Purpose

go-wiki-engine is a global CLI tool for managing repo-local wikis. It provides the plumbing layer (file listing, search, diff-driven change detection, structure linting, and scaffolding) while AI agent slash commands handle the intelligence layer (reading context, deciding what to update, writing wiki content). Supports GitHub Copilot and Claude Code out of the box.

It ships as a single statically-compiled Go binary with no external dependencies, distributed via `go install`.

## Architecture

```
cmd/wiki-engine/        CLI entry point — dispatches all subcommands
internal/config/        .wikirc parser — DefaultConfig(), Load(dir)
internal/engine/        Core operations: List, Headings, Search, LogTail,
                        Changed, Candidates, Lint, Refresh, Stats,
                        Context, Summary, Relevant, Impact, Diff, Watch,
                        BuildWikiGraph
internal/engine/engine_lint.go   Composable Checker interface + 16 implementations
internal/scaffold/      Init command — copies go:embed scaffold into a target repo
  files/                go:embed source (mirror of scaffold/)
internal/upgrade/       Self-upgrade via `go install @latest`
scaffold/               Human-readable reference copy of embedded templates
  wiki/                 Wiki pages: README, index, log, schema, phases, repo-map, operations/
  .wiki-instructions/   Canonical workflow definitions (single source of truth for all tools)
  .github/prompts/      GitHub Copilot slash commands → symlinks to .wiki-instructions/
  .github/instructions/ Copilot instructions → symlink to .wiki-instructions/
  .claude/commands/     Claude Code custom slash commands → symlinks to .wiki-instructions/
  .wikirc               Default config template
  AGENTS.md             Shim — redirects AI agent tools to wiki/index.md (create-only)
  CLAUDE.md             Shim — redirects Claude Code to wiki/index.md (create-only)
```

## High-Signal Areas

- `cmd/wiki-engine/main.go` — CLI dispatcher; version injected via `-ldflags`; `--json` flag support on all commands
- `internal/engine/engine.go` — all read-only wiki operations plus Diff, Watch, Impact; `Lint` delegates to composable checkers
- `internal/engine/engine_lint.go` — `Checker` interface + 16 implementations: required-files, front-matter, index-format, bare-urls, index-links, cross-page-links, markdown-format, orphans, heading-hierarchy, log-headings, log-chronology, markers, phase-consistency, external-links, duplicate-content, stale-content
- `internal/scaffold/scaffold.go` — `Init()` walks the embedded FS and remaps `wiki/` to the user-specified dir name; `SyncPrompts()` overwrites `.wiki-instructions/`, `.github/`, `.claude/commands/`, and `.pi/skills/` via `syncEmbeddedDir()` helper; tool-layer files are written as symlinks to `.wiki-instructions/` with a regular-copy fallback on platforms without symlink support; `syncShims()` creates `AGENTS.md`/`CLAUDE.md` only when absent (never overwrites user content); `Init()` preserves an existing `.wikirc`
- `internal/config/config.go` — parses `.wikirc` (key=value + array format, no external deps); returns defaults when file is absent
- `scaffold/` — source of truth for scaffold templates; `make sync-scaffold` copies it to `internal/scaffold/files/`

## Subcommands

| Command | What it does |
|---|---|
| `init [wiki-dir]` | Scaffold wiki, .wikirc, prompts for all tools, instructions, and AGENTS.md/CLAUDE.md shims into the current repo |
| `sync-prompts` | Overwrite `.wiki-instructions/`, `.github/`, and `.claude/commands/` with current embedded versions (safe to run after upgrade) |
| `list` | List all files under `wiki_dir` |
| `headings` | List all Markdown headings across wiki files |
| `search <query>` | Case-insensitive full-text search across wiki files |
| `log-tail [n]` | Show last N log headings from `log.md` |
| `changed [diff]` | `git diff --name-only` filtered to non-wiki, non-ignored files |
| `candidates [diff]` | Same as changed, further filtered by `.wikirc` ignore rules (see [config.md](config.md)) |
| `lint [--check=<a,b>] [--skip=<a,b>]` | Check required files, front matter, index format, bare URLs, broken links (index + cross-page), log heading format and chronology, open markers, orphans, heading hierarchy, phase consistency, external links to source files, duplicate content, stale content — repair guide: [operations/lint.md](operations/lint.md) |
| `stats` | Aggregate statistics: file count, heading count, total lines, last-updated date |
| `context [--minimal] [--active] [--sort=topo\|chrono] [--summarize]` | Condensed wiki snapshot, or the active-page graph from `index.md` with `--active` (`--sort=topo` by depth, default chronological) |
| `summary <page>` | First heading + first paragraph preview of a page |
| `relevant <query> [n]` | Rank wiki pages by relevance to a query |
| `impact <file...>` | Show which wiki pages mention changed source files (or pipe from `changed`) |
| `diff <from> <to>` | Show wiki files added/removed/changed between two git refs |
| `watch [--once]` | Poll for changes + lint issues at interval from `.wikirc`; exits with guidance when `watch_interval` is 0; `--once` for one-shot check |
| `refresh [diff]` | Run list + log-tail + changed + candidates + lint as a maintenance snapshot |
| `upgrade` | Re-runs `go install github.com/ramayac/go-wiki-engine/cmd/wiki-engine@latest` |
| `version` | Print the version set by -ldflags at build time |

## How the Multi-Tool Integration Works

The CLI is **read-only plumbing**. It never writes wiki content.

Workflows are defined once in `.wiki-instructions/` (canonical). Tool-specific directories contain symlinks:

| Canonical source | Copilot path | Claude Code path | pi.dev path |
|---|---|---|---|
| `.wiki-instructions/ingest.md` | `.github/prompts/wiki-ingest.prompt.md` | `.claude/commands/wiki-ingest.md` | (included in wiki skill) |
| `.wiki-instructions/query.md` | `.github/prompts/wiki-query.prompt.md` | `.claude/commands/wiki-query.md` | (included in wiki skill) |
| `.wiki-instructions/refresh.md` | `.github/prompts/wiki-refresh.prompt.md` | `.claude/commands/wiki-refresh.md` | (included in wiki skill) |
| `.wiki-instructions/onboard.md` | `.github/prompts/wiki-onboard.prompt.md` | `.claude/commands/wiki-onboard.md` | (included in wiki skill) |
| `.wiki-instructions/lint.md` | `.github/prompts/wiki-lint.prompt.md` | `.claude/commands/wiki-lint.md` | (included in wiki skill) |
| `.wiki-instructions/upgrade.md` | `.github/prompts/wiki-upgrade.prompt.md` | `.claude/commands/wiki-upgrade.md` | (included in wiki skill) |
| `.wiki-instructions/watch.md` | `.github/prompts/wiki-watch.prompt.md` | `.claude/commands/wiki-watch.md` | (included in wiki skill) |
| `.wiki-instructions/wiki-maintainer.md` | `.github/instructions/wiki-maintainer.instructions.md` | — | — |

In user repos, `wiki-engine init` and `wiki-engine sync-prompts` write these as real symlinks so edits to `.wiki-instructions/` propagate to every tool. On platforms where symlink creation fails (e.g. Windows without developer mode), regular copies are written instead — re-run `sync-prompts` after editing canonical files there.

The pi.dev integration uses an Agent Skills standard `SKILL.md` at `.pi/skills/wiki/SKILL.md` — a self-contained skill file that bundles all workflows into one entrypoint invoked via `/skill:wiki`.

Frontmatter is compatible: both tools use `description`. Copilot-specific fields (`name`, `argument-hint`, `agent`) are ignored by Claude Code.

The workflow is:
1. `wiki-engine init` — run once to scaffold
2. Developer customizes `wiki/repo-map.md` and `.wikirc`
3. Agent (via `/wiki-ingest`, `/wiki-refresh`, or `/wiki-onboard`) calls `wiki-engine changed` + `wiki-engine candidates` to discover what changed, then reads and writes wiki content itself
4. Agent calls `wiki-engine lint` to validate hygiene before finishing
5. After a binary upgrade, run `wiki-engine sync-prompts` in each repo to pull in new or updated prompts and instructions for all tools

`go:embed` follows symlinks at build time, so the embedded FS contains regular files. `make sync-scaffold` uses `cp -rL` to dereference symlinks when copying the scaffold into `internal/scaffold/files/`.

## Configuration — .wikirc

Full key reference: [config.md](config.md).

| Key | Default | Purpose |
|---|---|---|
| `wiki_dir` | `wiki` | Directory name for the wiki |
| `default_diff` | `main...HEAD` | Git diff range for changed/candidates/refresh |
| `log_lines` | `10` | Number of log entries shown by log-tail |
| `duplicate_threshold` | `0.7` | Similarity above which pages are flagged as duplicates (0.0-1.0, 0 disables) |
| `stale_days` | `30` | Days since a page's last git commit before it is flagged as stale (0 disables; falls back to file mtime outside git) |
| `watch_interval` | `0` | Seconds between watch polls (0 disables continuous watch; `--once` still works) |
| `context_summarize` | `false` | Default `wiki-engine context` to `--summarize` mode |
| `ignore` | `["wiki/", "bin/", "*.log", "*.tmp"]` | Paths excluded from candidate filtering |

## Generated Artifacts

- `bin/wiki-engine` — compiled binary (gitignored)
- `internal/scaffold/files/` — synced from `scaffold/` via `make sync-scaffold`; embedded into the binary
- `test/integration_test.sh` — end-to-end test suite (run via `make integration`, also executed in CI)

## Build and Release Path

```bash
make build           # Compile to bin/wiki-engine (version=dev)
make test            # Run all tests
make lint            # go vet
make sync-scaffold   # Copy scaffold/ → internal/scaffold/files/
make install         # go install globally
```

Releases are cross-compiled by `.github/workflows/release.yml` on `release: published` and uploaded as binary assets. Version is injected via `-ldflags "-X main.version=vX.Y.Z"`.

Go module: `github.com/ramayac/go-wiki-engine`. No external dependencies — standard library only.

## Exclusions

- `scaffold/` is human-readable reference; only `internal/scaffold/files/` is embedded. Always run `make sync-scaffold` after editing templates.
- `.wiki-instructions/` is the canonical source — edit here, not in the symlinked tool directories.
- `bin/` is gitignored.
- The wiki itself (`wiki/`) is excluded from candidate filtering.

## Related Pages

- [config.md](config.md) — full `.wikirc` reference.
- [operations/lint.md](operations/lint.md) — checker-by-checker repair guide.
- [operations/ingest.md](operations/ingest.md) — how architecture facts get updated.
- [schema.md](schema.md) — the contract this page must satisfy.
