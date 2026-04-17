# go-wiki-engine — Agent Instructions

## Project Overview

go-wiki-engine is a global CLI tool for managing repo-local wikis. It provides the plumbing layer (list, search, lint, change detection) while GitHub Copilot slash commands handle the intelligence layer (reading, deciding, writing wiki content).

## Architecture

```
cmd/
  wiki-engine/        # CLI entry point with all subcommands
internal/
  config/             # .wikirc parser
  engine/             # Core operations: list, headings, search, log-tail, changed, candidates, lint, refresh
  scaffold/           # `init` command: embeds and copies wiki templates + prompts + instructions
    files/            # go:embed source — mirror of scaffold/
  upgrade/            # Self-upgrade via `go install`
scaffold/             # Human-readable reference copy of embedded templates
  wiki/               # Wiki scaffold files (README, index, log, schema, phases, repo-map, operations/)
  .github/            # Copilot prompts and instructions
  .wikirc             # Default configuration
```

## Key Design Decisions

- **No database, no server.** Pure CLI tool that operates on the filesystem and git.
- **No API keys.** Wiki content is managed by Copilot slash commands, not by calling LLM APIs directly.
- **go:embed for scaffold.** The `init` command copies embedded files — no network fetch needed.
- **Self-upgrade via `go install`.** No custom update mechanism.
- **.wikirc is the only per-repo config.** Simple key=value + array format, no TOML/YAML dependency.

## Coding Conventions

- **Go 1.24+ required.**
- **No external dependencies.** Standard library only.
- **Tests live in the same package** (white-box style).
- **Build with:** `make build` or `go build -ldflags '...' ./cmd/wiki-engine`
- **Version is injected via -ldflags** at build time.

## Developer Workflow

```bash
make build            # Build binary to bin/
make test             # Run all tests
make lint             # Run go vet
make sync-scaffold    # Copy scaffold/ → internal/scaffold/files/ after editing templates
make install          # go install globally
```

## What NOT to Do

- Do not add external dependencies without strong justification.
- Do not add LLM API integration — that's the Copilot slash command's job.
- Do not modify wiki content from the CLI tool — it only inspects and reports.
- When editing scaffold templates, always run `make sync-scaffold` before building.
