# go-wiki-engine

A global CLI tool for managing repo-local wikis. Provides the deterministic plumbing layer (inspection, single-read validation, branch-aware citation checks, graph navigation, and idempotent scaffolding) while **slash commands** in your AI tool of choice handle the intelligence layer — reading, deciding, and writing wiki content.

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

### Prerequisites

- **Go 1.24+** — to build from source or install via `go install`.
- **git** 2.24 or newer — required by the diff-driven commands: `changed`, `candidates`, `check`, `refresh`, `watch`, `diff`, and `impact`.
- **Prebuilt binaries** — linux amd64/arm64, darwin amd64/arm64, and windows amd64 on [GitHub Releases](https://github.com/ramayac/go-wiki-engine/releases) with SHA-256 checksums and build-provenance attestations.

```bash
cd your-repo
wiki-engine init
# Edit .wikirc to set your ignore patterns
# Edit wiki/prologue/repo-map.md with your project's architecture
wiki-engine lint
wiki-engine check
```

Re-running `wiki-engine init` is idempotent: it reconciles prompts, symlinks, ignore entries, and versioned root shims while preserving user-authored pages, `.wikirc`, and `AGENTS.md`/`CLAUDE.md` content. Pass `wiki-engine init --wiki-dir <name>` to rename an existing wiki directory safely. `init <name>` fails when `.wikirc` names a different directory. `init` and `sync-prompts` refuse to write through a symlink that leaves the repo, copy an edited prompt file to `<name>.bak` before replacing it, and delete only files the engine owns.

## Slash Commands

The primary interface is slash commands in your AI tool, all sharing canonical definitions in `.wiki-instructions/`: `/wiki-ingest`, `/wiki-query`, `/wiki-refresh`, `/wiki-onboard`, `/wiki-lint`, `/wiki-upgrade`, `/wiki-watch`. The multi-tool integration model is documented in [wiki/prologue/repo-map.md](wiki/prologue/repo-map.md).

## Commands

Run `wiki-engine help` for the full list, `wiki-engine schema` for the embedded v2 schema and page templates, or `wiki-engine --json <command>` for structured output. Command-by-command details: [wiki/prologue/repo-map.md](wiki/prologue/repo-map.md).

Core commands: `init`, `sync-prompts`, `schema`, `list`, `headings`, `search`, `relevant`, `context`, `graph`, `summary`, `log-tail`, `changed`, `candidates`, `check`, `impact`, `lint` (`--fix`), `watch --once`, `diff`, `refresh`, `stats`, `upgrade`.

## Configuration (`.wikirc`)

Config lives in a `.wikirc` file at the repo root (sensible defaults apply when absent). Full reference: [wiki/prologue/config.md](wiki/prologue/config.md).

## Contracts & Compatibility Policy

Releases use clean `vX.Y.Z` semantic versions. The compatibility surface covered by semver is:

- **CLI commands, flags, and exit codes:**

  | Code | Meaning |
  |------|---------|
  | `0` | Success (including `wiki-engine check` with review recommendations only) |
  | `1` | Runtime or finding failure: lint issues, `graph --strict` findings, `check` breakage, git, config, or I/O errors |
  | `2` | Usage error: unknown command or flag, missing or extra argument, invalid flag value. `check` also uses `2` for git and setup errors (e.g., outside a Git repo or invalid base ref) |

  `-h` and `--help` exit `0`.
- **Global JSON envelope (version `1`):** `{"ok": bool, "data": ..., "error": "..."}`. With `--json`, `ok` is `false` whenever the exit code is non-zero, and `data` is kept when the command has findings to report. Empty lists encode as `[]`, never `null`.
- **Graph JSON schema (version `2`):** `{"schema_version": "2", "nodes": [], "edges": [], "unlinked": [], "stats": {}, "issues": []}`
- **Frontmatter schema (version `2`):** Real YAML (`gopkg.in/yaml.v3`) with `id`, `title`, `type`, `lifecycle`, workflow `status`/`severity`, `related`, `depends_on`, structured `sources`, and structured `references`.
  - **Migration policy:** v1 frontmatter (`status: planned|current|legacy|deprecated` and inline `references: [source:...]`) remains readable for one major release; `wiki-engine lint --fix` migrates pages deterministically. Missing lifecycle metadata never becomes active by default.
  - **Major-version policy:** Additive JSON fields, flags, and config keys ship in minor releases; breaking removals (such as retiring v1 frontmatter fallbacks or the `relevant` alias) require a major release.
- **`.wikirc` configuration keys:** [wiki/prologue/config.md](wiki/prologue/config.md).

## Development

```bash
make help             # Show all targets
make build            # Build to bin/wiki-engine
make test             # Run all unit tests
make lint             # Run go vet + wiki-engine lint
make audit            # Repo-wide wiki integrity, action-pin, and schema parity audit
make golangci-lint    # Static analysis with the repo config (golangci-lint v2)
make govulncheck      # Vulnerability scan (pinned govulncheck)
make sync-scaffold    # Sync scaffold/ → internal/scaffold/files/ for embedding
make integration      # End-to-end integration test suite
make install          # go install globally
```

## Self-Upgrade

```bash
wiki-engine upgrade                 # Fail-closed checksum + provenance verified binary upgrade
wiki-engine upgrade --from-source   # Explicit pinned `go install` from release tag
wiki-engine init                    # Reconcile prompts, symlinks, and shims in each repo
```

## Project Links

- [Changelog](CHANGELOG.md)
- [Contributing](CONTRIBUTING.md)
- [Security Policy](SECURITY.md)
- [Release runbook](wiki/operations/release.md)

## License

MIT
