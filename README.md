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
# Edit wiki/repo-map.md with your project's architecture
wiki-engine lint
```

This scaffolds:

| What | Purpose |
|------|---------|
| `wiki/` | Required pages + operations docs |
| `.wikirc` | Per-repo config (wiki dir, diff base, ignore patterns, detection thresholds) |
| `.wiki-instructions/` | **Canonical workflow definitions** — single source of truth for all tools |
| `.pi/skills/wiki/` | pi.dev Agent Skills standard skill |
| `.claude/commands/` | Claude Code slash commands |
| `.github/prompts/` | GitHub Copilot slash commands (`/wiki-ingest`, `/wiki-query`, `/wiki-refresh`, `/wiki-onboard`, `/wiki-lint`) |
| `.github/instructions/` | Copilot instruction file |
| `AGENTS.md` / `CLAUDE.md` | Redirect shims pointing AI tools to `wiki/index.md` |

## Slash Commands

The primary interface is slash commands in your AI tool. All share canonical definitions in `.wiki-instructions/`:

| Command | When to use |
|---------|-------------|
| `/wiki-ingest` | Absorb a feature branch or batch of commits into the wiki |
| `/wiki-query` | Answer architecture questions from the wiki first |
| `/wiki-refresh` | Run the full maintenance cycle (changed → ingest → lint) |
| `/wiki-onboard` | Bootstrap a wiki for a brand-new project or empty wiki |
| `/wiki-lint` | Run the wiki linter and automatically fix structural errors, formatting, or metadata |
| `/wiki-watch` | Monitor for un-ingested changes and trigger auto-ingest |

**Workflow:** You type `/wiki-ingest` (or `/wiki-lint`) → the agent inspects the repo with `wiki-engine` CLI commands → reads changed source files → writes durable facts to `wiki/` → runs `wiki-engine lint` to validate.

## Commands

```
wiki-engine [--json] <command> [arguments]
```

### Inspection

| Command | Description |
|---------|-------------|
| `list [--active]` | List all wiki files (optionally filtering by active lifecycle status) |
| `headings` | List all Markdown headings with file paths |
| `search <query>` | Case-insensitive search across wiki files |
| `log-tail [n]` | Show the last N log headings |
| `changed [diff-range]` | List non-wiki files changed in a git diff range |
| `candidates [diff-range]` | Filter changed files to ingest-worthy candidates |
| `context [--minimal] [--active] [--sort=topo\|chrono] [--summarize]` | Condensed wiki snapshot for agent context loading |
| `summary <page>` | Show first heading and paragraph of a page |
| `relevant <query> [n]` | Rank wiki pages by relevance to a query |
| `impact <file...>` | Show which wiki pages mention changed files |
| `stats` | Aggregate wiki statistics (files, headings, lines) |

### Validation

| Command | Description |
|---------|-------------|
| `lint [--check=<checkers>] [--skip=<checkers>] [--rebuild-cache]` | Full health check — front-matter, index-format, bare-urls, structure, links, markers, orphans, duplicates, stale content |
| `watch [--once]` | Poll for changes and lint issues (interval from `.wikirc`) |
| `diff <from> <to>` | Show wiki file changes between two git refs |

### Maintenance

| Command | Description |
|---------|-------------|
| `init [wiki-dir]` | Scaffold a new wiki into the current repo |
| `sync-prompts` | Update all tool instruction layers to the latest version |
| `refresh [diff-range]` | Run the full maintenance snapshot |
| `upgrade` | Self-upgrade to the latest version via `go install` |
| `version` | Print the version |

Add `--json` before any command for structured output: `wiki-engine --json lint`.

## Configuration (`.wikirc`)

Place a `.wikirc` file in your repo root:

```ini
wiki_dir = "wiki"
default_diff = "main...HEAD"
log_lines = 10
fail_severity = "warn"        # minimum severity to exit 1: error | warn | info

# Detection thresholds
duplicate_threshold = 0.7   # 0.0-1.0, similarity above which pages are flagged as duplicates
stale_days = 30              # days before an unchanged page is flagged as stale

# Watch mode
watch_interval = 0           # seconds between watch polls (0 = disabled)

# Performance
cache_enabled = true         # use .wiki/.cache.json for faster lookups
cache_max_mb = 10            # max cache size in MB (0 = unlimited)
context_summarize = false    # default context command to --summarize mode

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
| `fail_severity` | `warn` | Minimum severity level that causes linter to fail with exit code 1 (`error`, `warn`, or `info`) |
| `duplicate_threshold` | `0.7` | Similarity threshold for duplicate page detection (0 disables) |
| `stale_days` | `30` | Days before an unchanged page is flagged as stale (0 disables) |
| `watch_interval` | `0` | Seconds between watch polls (0 disables) |
| `cache_enabled` | `true` | Use `.wiki/.cache.json` to speed up searches and context |
| `cache_max_mb` | `0` | Maximum cache size in MB (0 is unlimited) |
| `context_summarize` | `false` | Default the `context` command to progressive summary disclosure mode |
| `ignore` | see above | Paths excluded from ingest candidate filtering |

If `.wikirc` is absent, sensible defaults are used.

## How It Works

The wiki engine is a **read-only inspection and validation tool**. It never modifies wiki content — that's the agent's job.

`wiki-engine init` scaffolds multiple tool layers into your repo, all sharing a single canonical source in `.wiki-instructions/`:

| Layer | Tool | Format |
|-------|------|--------|
| `.wiki-instructions/` | All tools | Canonical workflow definitions (edit here) |
| `.github/prompts/` | GitHub Copilot | Slash commands (symlinks) |
| `.github/instructions/` | GitHub Copilot | Agent instructions (symlink) |
| `.claude/commands/` | Claude Code | Custom slash commands (symlinks) |
| `.pi/skills/wiki/` | pi.dev | Agent Skills standard skill |
| `AGENTS.md` / `CLAUDE.md` | All AI tools | Redirect shims → `wiki/index.md` |

**Typical workflow:**

1. You run `wiki-engine init` once, then customize `wiki/repo-map.md` and `.wikirc`.
2. You type `/wiki-ingest`, `/wiki-query`, or `/wiki-refresh` in your AI tool.
3. The agent runs `wiki-engine context` to get a lightweight snapshot, then `wiki-engine changed` + `wiki-engine candidates` to see what changed.
4. The agent reads affected source files, writes durable facts into `wiki/`, and appends to `wiki/log.md`.
5. The agent runs `wiki-engine lint` to validate before finishing.

`wiki-engine` provides the plumbing. The slash commands provide the intelligence.

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
make test             # Run all unit tests
make lint             # Run go vet
make sync-scaffold    # Sync scaffold/ → internal/scaffold/files/ for embedding
make install          # go install globally

# Integration tests
bash test/integration_test.sh
```

When editing scaffold templates in `scaffold/`, run `make sync-scaffold` before building so the embedded copies are updated.

## Self-Upgrade

```bash
wiki-engine upgrade
wiki-engine sync-prompts   # Update prompts in each repo after upgrading
```

## License

MIT
