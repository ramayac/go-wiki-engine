# go-wiki-engine

A global CLI tool for managing repo-local wikis. Designed to work with GitHub Copilot slash commands for feeding knowledge into the wiki.

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
# Edit wiki/repo-map.md with your project's architecture
wiki-engine lint
```

This scaffolds:
- `wiki/` — full wiki directory with required pages and operations
- `.wikirc` — per-repo configuration (wiki dir, diff base, ignore patterns)
- `.github/prompts/` — Copilot slash commands for wiki-ingest, wiki-query, wiki-refresh
- `.github/instructions/` — Copilot instruction for wiki-aware agent behavior

## Commands

```
wiki-engine init [wiki-dir]         Scaffold a new wiki into the current repo
wiki-engine list                    List all wiki files
wiki-engine headings                List all Markdown headings with file paths
wiki-engine search <query>          Case-insensitive search across wiki files
wiki-engine log-tail [n]            Show the last N log headings
wiki-engine changed [diff-range]    List non-wiki files changed in a git diff range
wiki-engine candidates [diff-range] Filter changed files to ingest-worthy candidates
wiki-engine lint                    Check wiki structure, links, and markers
wiki-engine refresh [diff-range]    Run the full maintenance snapshot
wiki-engine upgrade                 Self-upgrade to the latest version via go install
wiki-engine version                 Print the version
wiki-engine help                    Show help
```

## Configuration (`.wikirc`)

Place a `.wikirc` file in your repo root:

```toml
wiki_dir = "wiki"
default_diff = "main...HEAD"
log_lines = 10

ignore = [
  "wiki/",
  "bin/",
  "vendor/",
  "*.log",
  "*.tmp",
]
```

| Key | Default | Purpose |
|-----|---------|---------|
| `wiki_dir` | `wiki` | Directory name for the wiki |
| `default_diff` | `main...HEAD` | Default git diff range for changed/candidates/refresh |
| `log_lines` | `10` | Number of log entries shown by log-tail |
| `ignore` | see above | Paths excluded from ingest candidate filtering |

If `.wikirc` is absent, sensible defaults are used.

## How It Works with Copilot

The wiki engine is a **read-only inspection tool**. It does not modify wiki content directly — that's the agent's job. The workflow is:

1. **You** run `wiki-engine init` once to scaffold the wiki.
2. **Copilot** uses the slash commands (`/wiki-ingest`, `/wiki-query`, `/wiki-refresh`) to read, search, and update wiki content.
3. **wiki-engine** provides the plumbing: listing files, searching, detecting changes, linting structure.

The slash commands call `wiki-engine` subcommands under the hood and tell the agent what to read, what to update, and when to write back.

## Wiki Contract

Every wiki managed by this tool has at least:

```
wiki/
├── README.md
├── index.md          # Catalog of all wiki pages
├── log.md            # Append-only maintenance timeline
├── schema.md         # Required structure and rules
├── phases.md         # Rollout tracking
├── repo-map.md       # Architecture and exclusions
└── operations/
    ├── ingest.md     # How to absorb repo changes
    ├── query.md      # How to answer questions wiki-first
    └── lint.md       # How to health-check the wiki
```

## Development

```bash
make help             # Show all targets
make build            # Build to bin/wiki-engine
make test             # Run all tests
make lint             # Run go vet
make sync-scaffold    # Sync scaffold/ → internal/scaffold/files/ for embedding
make install          # go install globally
```

When editing scaffold templates in `scaffold/`, run `make sync-scaffold` before building so the embedded copies are updated.

## Self-Upgrade

```bash
wiki-engine upgrade
```

This runs `go install github.com/ramayac/go-wiki-engine/cmd/wiki-engine@latest` to pull the latest version.

## License

MIT
