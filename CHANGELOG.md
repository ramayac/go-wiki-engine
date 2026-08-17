# Changelog

All notable changes to go-wiki-engine are documented here. Format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and releases follow
[Semantic Versioning](https://semver.org/spec/v2.0.0.html); the compatibility
surface is defined in [README.md](README.md).

## [Unreleased]

### Added

- `.golangci-lint.yml` (v2 config) and a `make golangci-lint` target; CI runs golangci-lint and `go test -race ./...`.
- Mocked-HTTP tests for the `wiki-engine upgrade` download path: success, checksum mismatch, no-asset fallback, latest-tag failure fallback, and extraction failure.
- User-facing docs: `CONTRIBUTING.md`, `SECURITY.md`, and the release runbook ([wiki/operations/release.md](wiki/operations/release.md)).
- `README.md` prerequisites and supported-platform sections, plus a versioning & compatibility policy.

### Changed

- CLI commands reject unknown flags instead of silently ignoring them.
- `.wikirc` loading warns about unknown keys.
- `changed` errors explain that git and a git repository are required.
- File scanners now propagate scan errors instead of swallowing them.
- `wiki-engine upgrade` internals refactored so the download flow is injectable and testable.

## [0.x]

Legacy pre-1.0 releases (tags `0.1.0`, `v.0.5.0`). Consolidated from the wiki
maintenance log ([wiki/prologue/log.md](wiki/prologue/log.md)).

### Added

- 17 composable lint checkers with a `fail_severity` exit gate.
- Page lifecycle front matter: `planned` / `current` / `legacy` / `deprecated`.
- Active wiki graph navigation (`context --active`, `--sort=topo|chrono`).
- Progressive disclosure (`context --summarize`, `summary`, `relevant`).
- Change detection (`changed`, `candidates`, `impact`, `diff`, `refresh`, `watch --once`).
- Multi-tool instruction layer: canonical `.wiki-instructions/` with symlinks for GitHub Copilot, Claude Code, and pi.dev.
- Checksum-verified self-upgrade (`upgrade`) and prompt syncing (`sync-prompts`).
- Organized wiki layout (`prologue/`, `decisions/`, `operations/`) with legacy flat-layout fallback.

### Fixed

- `log-tail` returning the oldest entries instead of the most recent.
- Path traversal via `summary`.
- Cross-page links now strictly page-relative (wiki-root fallback removed).
- Stale-content detection now uses git commit dates (mtime fallback).
