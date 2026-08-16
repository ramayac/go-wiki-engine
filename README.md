# go-wiki-engine

A global CLI tool for managing repo-local wikis. Provides the plumbing layer (inspection, validation, change detection) while **slash commands** in your AI tool of choice handle the intelligence layer — reading, deciding, and writing wiki content.

Works with **GitHub Copilot**, **Claude Code**, and **pi.dev**.

## Install

```bash
go install github.com/ramayac/go-wiki-engine/cmd/wiki-engine@latest
```

Or build from source:

```bash
git clone https://github.com/ramayac/go-wiki-engine.git
cd go-wiki-engine
make build
# Binary is at bin/wiki-engine
```

## Quick Start

```bash
cd your-repo
wiki-engine init
# Edit .wikirc to set your ignore patterns
# Edit wiki/prologue/repo-map.md with your project's architecture
wiki-engine lint
```

The two files to customize after `init` are [`.wikirc`](wiki/prologue/config.md) — full reference in the wiki — and `wiki/prologue/repo-map.md`.

It scaffolds the [wiki structure](#wiki-contract), the canonical `.wiki-instructions/` workflows, and tool-specific layers for Copilot, Claude Code, and pi.dev — see [How It Works](#how-it-works).

## Slash Commands

The primary interface is slash commands in your AI tool, all sharing canonical definitions in `.wiki-instructions/`: `/wiki-ingest`, `/wiki-query`, `/wiki-refresh`, `/wiki-onboard`, `/wiki-lint`, `/wiki-upgrade`, `/wiki-watch`. The multi-tool integration model is documented in [wiki/prologue/repo-map.md](wiki/prologue/repo-map.md).

## Commands

Run `wiki-engine help` for the full list, or `wiki-engine --json <command>` for structured output. Command-by-command details: [wiki/prologue/repo-map.md](wiki/prologue/repo-map.md).

Core commands: `init`, `list`, `search`, `context --active`, `summary`, `log-tail`, `changed`, `candidates`, `lint`, `watch --once`, `diff`, `refresh`, `sync-prompts`, `upgrade`.

## Configuration (`.wikirc`)

Config lives in a `.wikirc` file at the repo root (sensible defaults apply when absent). Full reference: [wiki/prologue/config.md](wiki/prologue/config.md).

## How It Works

The wiki engine is a **read-only inspection and validation tool**. It never modifies wiki content — that's the agent's job.

`wiki-engine init` scaffolds tool layers for GitHub Copilot, Claude Code, and pi.dev — one canonical source in `.wiki-instructions/` with symlinks in each tool directory (regular-copy fallback where symlinks fail). The full multi-tool integration model: [wiki/prologue/repo-map.md](wiki/prologue/repo-map.md).

**Typical workflow:**

1. You run `wiki-engine init` once, then customize [`.wikirc`](wiki/prologue/config.md) and [wiki/prologue/repo-map.md](wiki/prologue/repo-map.md).
2. You type `/wiki-ingest`, `/wiki-query`, or `/wiki-refresh` in your AI tool.
3. The agent runs `wiki-engine context` to get a lightweight snapshot, then `wiki-engine changed` + `wiki-engine candidates` to see what changed.
4. The agent reads affected source files, writes durable facts into `wiki/`, and appends to `wiki/prologue/log.md`.
5. The agent runs `wiki-engine lint` to validate before finishing.

## Wiki Contract

Every wiki managed by this tool has at least:

```
wiki/
├── README.md
├── index.md              # Catalog of all wiki pages
├── prologue/
│   ├── log.md            # Append-only maintenance timeline
│   ├── schema.md         # Required structure and rules
│   ├── phases.md         # Rollout tracking
│   └── repo-map.md       # Architecture and exclusions
├── decisions/            # Topic pages grouped by domain
└── operations/
    ├── ingest.md         # How to absorb repo changes
    ├── query.md          # How to answer questions wiki-first
    └── lint.md           # How to health-check the wiki
```

Category directories such as `decisions/` and `architectures/` are conventions, not requirements — the scaffold ships placeholder examples in both. The full contract and directory convention: [wiki/prologue/schema.md](wiki/prologue/schema.md).

## Development

```bash
make help             # Show all targets
make build            # Build to bin/wiki-engine
make test             # Run all unit tests
make lint             # Run go vet + wiki-engine lint
make audit            # Repo-wide wiki reference integrity audit
make sync-scaffold    # Sync scaffold/ → internal/scaffold/files/ for embedding
make integration      # End-to-end integration test suite
make install          # go install globally
```

When editing scaffold templates in `scaffold/`, run `make sync-scaffold` before building so the embedded copies are updated.

## Self-Upgrade

```bash
wiki-engine upgrade
wiki-engine sync-prompts   # Update prompts in each repo after upgrading
```

## License

MIT
