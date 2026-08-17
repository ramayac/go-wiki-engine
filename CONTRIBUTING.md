# Contributing to go-wiki-engine

The project is a zero-dependency Go CLI for repo-local wikis. Architecture
facts live in the [wiki](wiki/index.md) — read it before broad changes.

## Prerequisites

- **Go 1.24+** (see `go.mod`)
- **git** — required by the diff-driven commands (`changed`, `candidates`,
  `refresh`, `watch`, `diff`, `impact`) and by the test suite
- **make** — optional; `make help` lists all targets

## Development Loop

```bash
make build            # compile to bin/wiki-engine
make test             # unit tests
make lint             # go vet + wiki-engine lint on this repo's own wiki
make audit            # repo-wide wiki reference integrity audit
make integration      # end-to-end CLI suite (test/integration_test.sh)
make golangci-lint    # golangci-lint v2 with .golangci-lint.yml
go test -race ./...   # race detector
```

All of these run in CI (`.github/workflows/test.yml`).

## Scaffold Changes

`scaffold/` is the human-readable source of truth for the templates embedded
into the binary. After any edit there, run `make sync-scaffold` before
building — CI fails when `scaffold/` and `internal/scaffold/files/` drift.
`go:embed` follows symlinks, so the sync copy dereferences them (`cp -rL`).

## Wiki Conventions

This repo dogfoods its own tool. Before editing wiki pages, read
[wiki/prologue/schema.md](wiki/prologue/schema.md): every page needs YAML
front matter (`status`, `description`), cross-links, and an
[index.md](wiki/index.md) catalog entry. `wiki/prologue/log.md` is append-only.
Run `make audit` after structural changes (page moves, new category
directories, scaffold or prompt edits).

## Releases

Cutting and verifying releases is described in the
[release runbook](wiki/operations/release.md).
