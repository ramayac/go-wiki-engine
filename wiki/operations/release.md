---
id: release
title: Release Runbook
lifecycle: current
description: "Runbook for cutting and verifying a go-wiki-engine release."
sources:
  - file: internal/upgrade/upgrade.go
  - file: internal/scaffold/scaffold.go
references:
  - type: external
    value: "https://github.com/ramayac/go-wiki-engine/releases"
---
# Release Runbook

How to cut, publish, and verify a release of go-wiki-engine.

## Versioning Policy

- Tags use clean `vX.Y.Z` semantic versions. Legacy tags (`0.1.0`, `v.0.5.0`) stay in place because `wiki-engine upgrade` asset URLs embed them.
- Semver covers: CLI flags, the `--json` envelope, `.wikirc` keys (see [config.md](../prologue/config.md)), and the wiki contract (see [schema.md](../prologue/schema.md)).
- The runtime keeps minimal audited dependencies (Go standard library + `gopkg.in/yaml.v3`); CI runs `govulncheck` and Dependency Review.
- All GitHub Actions in `.github/workflows/` are pinned by 40-character immutable commit SHA and release assets publish both per-platform checksums and SLSA build-provenance attestations.

## Pre-release Checklist

1. All gates green: `make test`, `make lint`, `make audit`, `make integration`, `make golangci-lint`, `make govulncheck`, and `go test -race ./...`.
2. `make sync-scaffold` leaves `internal/scaffold/files` unchanged.
3. `wiki-engine lint`, `wiki-engine check`, `wiki-engine graph --strict`, and `wiki-engine context --active` are clean on this repo.
4. `CHANGELOG.md` has its Unreleased entries moved to the new version and dated.

## Cut & Publish

1. Tag and push: `git tag vX.Y.Z && git push origin vX.Y.Z`.
2. Publish the GitHub release. `.github/workflows/release.yml` builds the platform assets, generates per-platform `.sha256` files and `checksums.txt`, attests build provenance via `actions/attest-build-provenance`, and uploads assets.
3. Asset names follow `wiki-engine_<tag>_<goos>_<goarch>.{tar.gz,zip}` — the exact contract `wiki-engine upgrade` matches against.

## Post-release Verification

1. `go install github.com/ramayac/go-wiki-engine/cmd/wiki-engine@vX.Y.Z` — `wiki-engine version` must report vX.Y.Z.
2. From an installed older binary, run `wiki-engine upgrade` (fail-closed checksum + provenance check) and confirm the replacement succeeds.
3. Run `wiki-engine init` (or `wiki-engine sync-prompts`) in each initialized repo to reconcile prompts, symlinks, and root shims.

## Post-release Bookkeeping

- Append a dated entry to [log.md](../prologue/log.md).
- Update the backlog in [todo.md](../decisions/todo.md).
- Link new features from [repo-map.md](../prologue/repo-map.md) if the architecture changed.

## Related

- [repo-map.md](../prologue/repo-map.md) — architecture and build path this runbook operates on.
- [config.md](../prologue/config.md) — the `.wikirc` keys covered by semver.
- [schema.md](../prologue/schema.md) — the wiki contract covered by semver.
