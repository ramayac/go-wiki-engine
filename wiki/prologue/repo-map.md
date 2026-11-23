---
id: repo-map
title: Repo Map
type: concept
lifecycle: current
description: "Overview of the repository architecture, high-signal areas, CLI commands, and build instructions."
sources:
  - file: cmd/wiki-engine/
  - file: internal/paths/
  - file: internal/config/
  - file: internal/model/
  - file: internal/git/
  - file: internal/engine/
  - file: internal/lint/
  - file: internal/scaffold/
  - file: internal/schema/
  - file: internal/upgrade/
---
# Repo Map

## Purpose

go-wiki-engine is a deterministic Go CLI tool for managing repo-local wikis. It provides the plumbing layer (path-contained inspection, real YAML frontmatter parsing, single-read linting, branch-aware citation checks, graph navigation, and idempotent scaffolding) while AI agent slash commands handle the intelligence layer for GitHub Copilot, Claude Code, and pi.dev.

## Architecture

```text
cmd/wiki-engine/        CLI entrypoint split into main.go, commands.go, output.go
internal/paths/         Single strict path containment API (single-segment wiki_dir, symlink checks)
internal/config/        .wikirc parser & validator (config.go, validate.go)
internal/schema/        Authoritative embedded schema v2 vocabulary & page templates
internal/model/         Typed page, YAML frontmatter (gopkg.in/yaml.v3), structured sources,
                        single-read WikiSnapshot, and unified Graph model
internal/git/           Unified Git change pipeline (merge-base, renames, working tree, untracked,
                        batched history, commit lookup)
internal/engine/        Focused operations: context, search, summary, impact, check, graph, watch,
                        diff, stats, refresh
internal/lint/          Single-read snapshot lint engine, grouped checkers, and idempotent lint --fix
internal/scaffold/      Idempotent init/reconciliation, wiki rename, versioned root shims, sync-prompts
  files/                go:embed mirror of scaffold/
internal/upgrade/       Fail-closed self-upgrade (streamed size-capped downloads, SHA-256 checksums,
                        provenance attestation verification, safe archive extraction, --from-source)
internal/audit/         Repo-wide reference, action-pin SHA, schema, prompt, and scaffold parity tests
```

## Subcommands

| Command | What it does |
|---|---|
| `init [wiki-dir] [--wiki-dir=<name>]` | Scaffold a new wiki, idempotently reconcile an existing managed wiki, or safely rename with `--wiki-dir` |
| `sync-prompts` | Compatibility command to update `.wiki-instructions/`, `.github/`, `.claude/commands/`, and `.pi/skills/` |
| `schema [--type=<t>] [--section=<s>]` | Print the embedded v2 wiki schema, lifecycle rules, or page templates |
| `list [--active]` | List all contained wiki files (or active pages only) |
| `headings` | List all Markdown headings across wiki files |
| `search <query> [--tag=<t>] [--type=<t>] [--lifecycle=<l>] [--include-history] [--limit=<n>] [--keyword] [--ranked]` | Search wiki pages (excludes historical pages by default; `--ranked` enables field-weighted ranker) |
| `log-tail [n]` | Show the most recent N log headings from `log.md` |
| `changed [diff-range] [--base=<ref>] [--no-working-tree]` | Branch-aware non-wiki changed files (merge-base + working tree + untracked unless disabled) |
| `candidates [diff-range] [--base=<ref>] [--no-working-tree]` | Changed files filtered by `.wikirc` ignore patterns |
| `check [--base=<ref>] [--no-working-tree]` | Branch-aware citation and drift check (exit 0 ok/review, 1 breakage, 2 Git/setup error) |
| `lint [--check=<a,b>] [--skip=<a,b>] [--fix]` | Single-read wiki lint with individual or grouped selectors (`schema`, `links`, `graph`, `freshness`, `security`) and idempotent `--fix` |
| `stats` | Aggregate statistics: file count, heading count, total lines, last-updated date |
| `context [--minimal] [--summarize] [--active] [--sort=topo\|chrono] [--max-pages=<n>] [--max-bytes=<n>]` | Bounded wiki snapshot or active graph for agent context loading |
| `graph [page-or-id] [--strict] [--dot] [--include-sources]` | Active wiki graph (ASCII tree, ID/path neighborhood, DOT export, opt-in source nodes, `schema_version: "2"`) |
| `summary <page-or-id>` | First heading, first paragraph, and bounded preview of a page |
| `relevant <query> [n] [--tag=<t>] [--type=<t>] [--lifecycle=<l>] [--include-history] [--limit=<n>]` | Compatibility alias for field-weighted ranked search |
| `impact [--fuzzy] <file...>` | Exact source/directory/link impact with match reasons in JSON (`--fuzzy` opts in to basename prose scan) |
| `diff <from> <to>` | Show wiki files added/removed/changed between two Git refs |
| `watch [--once]` | Poll for changes + lint issues (`--once` exits 1 when lint fails) |
| `refresh [diff-range]` | Full maintenance snapshot: files, log, changed, candidates, lint (exits 1 when lint fails; honors `--json`) |
| `upgrade [--from-source] [--require-attestation]` | Fail-closed self-upgrade with SHA-256 checksum, `gh attestation verify` provenance check, tag validation, and downgrade refusal (explicit `--from-source` for pinned `go install`) |
| `version` | Print build version |

## JSON & Exit-Code Contracts

- Global JSON envelope (version `1`): `{"ok": bool, "data": ..., "error": "..."}`
- Graph JSON schema version: `2`
- Frontmatter schema version: `2`
- Exit codes: `0` success (including `check` review recommendations), `1` runtime or finding failure (lint, `graph --strict`, `refresh` lint, `check` breakage), `2` usage error (`check` also uses `2` for Git/setup errors). See [README.md](../../README.md#contracts--compatibility-policy).

## Build and Release Path

```bash
make build           # Compile to bin/wiki-engine
make test            # Run all unit tests
make lint            # go vet + wiki-engine lint
make audit           # Repo-wide reference, action-pin, and schema parity audit
make golangci-lint   # golangci-lint v2
make govulncheck     # Pinned govulncheck vulnerability scan
make sync-scaffold   # Copy scaffold/ → internal/scaffold/files/
make integration     # End-to-end integration test suite
```

Go module: `github.com/ramayac/go-wiki-engine` (Go stdlib + `gopkg.in/yaml.v3`).

## Related Pages

- [config.md](config.md) — full `.wikirc` reference.
- [operations/lint.md](../operations/lint.md) — checker-by-checker repair guide.
- [operations/ingest.md](../operations/ingest.md) — how architecture facts get updated.
- [operations/release.md](../operations/release.md) — how releases are cut, attested, and verified.
- [schema.md](schema.md) — the v2 schema contract this page satisfies.
