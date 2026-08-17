---
status: current
description: "Runbook for cutting and verifying a go-wiki-engine release."
superseded_by: ""
---
# Release Runbook

How to cut, publish, and verify a release of go-wiki-engine.

## Versioning Policy

- Tags use clean `vX.Y.Z` semantic versions. Legacy tags (`0.1.0`, `v.0.5.0`) stay in place because `wiki-engine upgrade` asset URLs embed them.
- Semver covers: CLI flags, the `--json` envelope, `.wikirc` keys (see [config.md](../prologue/config.md)), and the wiki contract (see [schema.md](../prologue/schema.md)).
- The runtime stays zero-dependency (Go standard library only); dev-only tooling is not part of the module.

## Pre-release Checklist

1. All gates green: `make test`, `make lint`, `make audit`, `make integration`, `make golangci-lint`, and `go test -race ./...`.
2. `make sync-scaffold` leaves `internal/scaffold/files` unchanged.
3. `wiki-engine lint` and `wiki-engine context --active` are clean on this repo.
4. `CHANGELOG.md` has its Unreleased entries moved to the new version and dated.

## Cut & Publish

1. Tag and push: `git tag vX.Y.Z && git push origin vX.Y.Z`.
2. Publish the GitHub release. `.github/workflows/release.yml` builds the platform assets and `checksums.txt` and uploads them.
3. Asset names follow `wiki-engine_<tag>_<goos>_<goarch>.{tar.gz,zip}` — the exact contract `wiki-engine upgrade` matches against.

## Post-release Verification

1. `go install github.com/ramayac/go-wiki-engine/cmd/wiki-engine@vX.Y.Z` — `wiki-engine version` must report vX.Y.Z.
2. From an installed older binary, run `wiki-engine upgrade` and confirm the checksum-verified replacement succeeds.
3. Run `wiki-engine sync-prompts` in each initialized repo to pull updated prompts and instructions.

## Post-release Bookkeeping

- Append a dated entry to [log.md](../prologue/log.md).
- Update the backlog in [todo.md](../decisions/todo.md).
- Link new features from [repo-map.md](../prologue/repo-map.md) if the architecture changed.

## Related

- [repo-map.md](../prologue/repo-map.md) — architecture and build path this runbook operates on.
- [config.md](../prologue/config.md) — the `.wikirc` keys covered by semver.
- [schema.md](../prologue/schema.md) — the wiki contract covered by semver.
