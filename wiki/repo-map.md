# Repo Map

## Purpose

go-wiki-engine is a global CLI tool for managing repo-local wikis. It provides the plumbing layer (file listing, search, diff-driven change detection, structure linting, and scaffolding) while GitHub Copilot slash commands handle the intelligence layer (reading context, deciding what to update, writing wiki content).

It ships as a single statically-compiled Go binary with no external dependencies, distributed via `go install`.

## Architecture

```
cmd/wiki-engine/        CLI entry point — dispatches all subcommands
internal/config/        .wikirc parser — DefaultConfig(), Load(dir)
internal/engine/        Core operations: List, Headings, Search, LogTail,
                        Changed, Candidates, Lint, Refresh
internal/scaffold/      Init command — copies go:embed scaffold into a target repo
  files/                go:embed source (mirror of scaffold/)
internal/upgrade/       Self-upgrade via `go install @latest`
scaffold/               Human-readable reference copy of embedded templates
  wiki/                 Wiki pages: README, index, log, schema, phases, repo-map, operations/
  .github/prompts/      VS Code slash command prompts: wiki-ingest, wiki-query, wiki-refresh
  .github/instructions/ Copilot instruction: wiki-maintainer.instructions.md
  .wikirc               Default config template
```

## High-Signal Areas

- `cmd/wiki-engine/main.go` — CLI dispatcher; version injected via `-ldflags`
- `internal/engine/engine.go` — all read-only wiki operations; `Lint` tracks fenced code blocks to avoid false positives
- `internal/scaffold/scaffold.go` — `Init()` walks the embedded FS and remaps `wiki/` to the user-specified dir name
- `internal/config/config.go` — parses `.wikirc` (key=value + array format, no external deps); returns defaults when file is absent
- `scaffold/` — source of truth for scaffold templates; `make sync-scaffold` copies it to `internal/scaffold/files/`

## Subcommands

| Command | What it does |
|---|---|
| `init [wiki-dir]` | Scaffold wiki, .wikirc, prompts, and instructions into the current repo |
| `list` | List all files under `wiki_dir` |
| `headings` | List all Markdown headings across wiki files |
| `search <query>` | Case-insensitive full-text search across wiki files |
| `log-tail [n]` | Show last N log headings from `log.md` |
| `changed [diff]` | `git diff --name-only` filtered to non-wiki, non-ignored files |
| `candidates [diff]` | Same as changed, further filtered by `.wikirc` ignore rules |
| `lint` | Check required files, broken index links, log heading format, open markers |
| `refresh [diff]` | Run list + log-tail + changed + candidates + lint as a maintenance snapshot |
| `upgrade` | Re-runs `go install github.com/ramayac/go-wiki-engine/cmd/wiki-engine@latest` |
| `version` | Print the version set by -ldflags at build time |

## How the Copilot Integration Works

The CLI is **read-only plumbing**. It never writes wiki content.

`wiki-engine init` copies three VS Code prompt files into `.github/prompts/`:

| File | VS Code slash command | Purpose |
|---|---|---|
| `wiki-ingest.prompt.md` | `/wiki-ingest` | Absorb recent repo changes into the wiki |
| `wiki-refresh.prompt.md` | `/wiki-refresh` | Run the full maintenance snapshot |
| `wiki-query.prompt.md` | `/wiki-query` | Answer questions from the wiki first |

It also copies `.github/instructions/wiki-maintainer.instructions.md`, which VS Code Copilot picks up automatically (via `applyTo: "**"`) and injects as persistent agent context — telling the agent to read the wiki before broad analysis and to write durable findings back into it.

The workflow is:
1. `wiki-engine init` — run once to scaffold
2. Developer customizes `wiki/repo-map.md` and `.wikirc`
3. Agent (via `/wiki-ingest` or `/wiki-refresh`) calls `wiki-engine changed` + `wiki-engine candidates` to discover what changed, then reads and writes wiki content itself
4. Agent calls `wiki-engine lint` to validate hygiene before finishing

The prompts tell the agent *which subcommands to call* and in what order. The agent does all reading and writing; `wiki-engine` only surfaces facts.

## Configuration — .wikirc

| Key | Default | Purpose |
|---|---|---|
| `wiki_dir` | `wiki` | Directory name for the wiki |
| `default_diff` | `main...HEAD` | Git diff range for changed/candidates/refresh |
| `log_lines` | `10` | Number of log entries shown by log-tail |
| `ignore` | `["wiki/", "bin/", "*.log", "*.tmp"]` | Paths excluded from candidate filtering |

## Generated Artifacts

- `bin/wiki-engine` — compiled binary (gitignored)
- `internal/scaffold/files/` — synced from `scaffold/` via `make sync-scaffold`; embedded into the binary

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
- `bin/` is gitignored.
- The wiki itself (`wiki/`) is excluded from candidate filtering.
