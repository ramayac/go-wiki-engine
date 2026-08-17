# Plan: go-wiki-engine 1.0 release audit + remediation

## Audit verdict

Engineering is 1.0-grade (zero-dep Go CLI, 17 composable lint checkers, checksum-verified self-upgrade, CI with unit + audit + integration + drift guard). NOT press-the-button ready: release tag hygiene, ~17 scanner.Err()/concat nits, untested upgrade download path, missing end-user docs (CHANGELOG/CONTRIBUTING/SECURITY/release runbook), no golangci-lint/race in CI.

## Decisions (from user)

- Backlog todos (#9, #25, #31, #32) are nice-to-have, NOT gating for 1.0.
- Add golangci-lint to CI (yes).
- Ship full user-facing doc set (CHANGELOG.md, CONTRIBUTING.md, SECURITY.md, release runbook).
- Versioning: clean vX.Y.Z tags; semver covers CLI flags, --json envelope, .wikirc keys, wiki contract. Keep zero-runtime-deps (stdlib only).

## Phases

### Phase A — Release hygiene & versioning

- A1: Versioning & compatibility policy section in README + repo-map (vX.Y.Z, what semver covers).
- A2: Release runbook wiki/operations/release.md (tag naming, publish -> release.yml assets + checksums.txt, verify upgrade end-to-end, post-release wiki ingest). Add to index.md + link from repo-map.

### Phase B — Code polish (parallel A/C/D/E)

- B1: Add scanner.Err() checks after Scan loops: engine.go (Headings, Search, Stats, Summary, Relevant), engine_lint.go (~5 sites), main.go impact stdin loop.
- B2: Replace WriteString(f + "\n") concats in engine.go ~L840-870 with two WriteString calls.
- B3: Reject unknown command flags instead of silently ignoring (runEngine handlers; at minimum for list/lint/context/watch).
- B4: .wikirc parser warns on unknown keys (config.Load) instead of silent drop.
- B5: engine.Changed: wrap git exec error with hint "requires git and a git repository".

### Phase C — Tooling (parallel)

- C1: Add .golangci-lint.yml (errcheck, govet, staticcheck, ineffassign, unused, misspell) + Makefile target; wire into test.yml.
- C2: Add go test -race ./... step in test.yml.
- C3: Document min Go version (go.mod = 1.24.4; state Go 1.24+ in README/CONTRIBUTING).

### Phase D — Tests (parallel)

- D1: Refactor internal/upgrade/upgrade.go: extract injectable run(baseURL, executablePath) so tests use httptest.Server; Run() stays thin wrapper.
- D2: upgrade_test.go: success (download+checksum+replace), checksum mismatch error, no-asset -> go-install fallback, HTTP 404 -> fallback, extract failure. Closes todo #54.

### Phase E — User-facing docs (parallel)

- E1: CHANGELOG.md (Keep a Changelog; retroactive highlights from wiki/prologue/log.md 0.x -> 1.0.0).
- E2: CONTRIBUTING.md (Go 1.24+, git; make targets; make sync-scaffold rule; wiki conventions; pointer to runbook).
- E3: SECURITY.md (reporting channel; checksum-verified upgrade; no remote content execution).
- E4: README Prerequisites (git required for changed/candidates/refresh/watch/diff/impact), supported platforms (linux amd64/arm64, darwin amd64/arm64, windows amd64), links to CHANGELOG/CONTRIBUTING/SECURITY.
- E5: repo-map Build and Release Path -> point at runbook.

### Phase F — Release execution (depends on A-E)

- F1: Close todo #57; tag v0.7.0 from master after all green. (Release tag hygiene: v0.7.0 -> v1.0.0; v1.0.0 is the first semver-compliant release).
- F2: Verify release.yml assets + checksums.txt; test wiki-engine upgrade from an installed 0.x binary.
- F3: Post-release wiki log.md entry; keep backlog items open for 1.x.

## Relevant files

- cmd/wiki-engine/main.go — arg dispatch; impact stdin scanner L486; usage text.
- internal/engine/engine.go — scanner loops L85/129/288/446/485/601; WriteString L840-870.
- internal/engine/engine_lint.go — scanner loops L448/528/565/613/664.
- internal/config/config.go — Load(): unknown-key warning.
- internal/upgrade/upgrade.go + upgrade_test.go — injectable run + httptest tests.
- .github/workflows/test.yml + release.yml — CI wiring.
- Makefile — golangci-lint target.
- README.md, CHANGELOG.md, CONTRIBUTING.md, SECURITY.md (new).
- wiki/operations/release.md (new), wiki/index.md, wiki/prologue/repo-map.md, wiki/decisions/todo.md (#54, #57).

## Verification

1. make lint && make test && make audit && make integration green locally and in CI.
2. golangci-lint clean with new config; go test -race ./... passes.
3. New upgrade tests cover success/mismatch/fallback/404/extract-failure.
4. make sync-scaffold && git diff --quiet -- internal/scaffold/files (drift guard).
5. Manual: go install @v1.0.0 -> wiki-engine version reports v1.0.0; wiki-engine upgrade from old binary succeeds on linux.
6. wiki-engine lint + context --active on this repo clean after doc edits.

## Further considerations

- Add windows-latest CI test job to cover symlink-fallback path? Recommended yes, cheap.
- Reproducible builds: version embeds commit+date; optional SOURCE_DATE_EPOCH support (low priority).
- upgrade go-install fallback uses @latest (could skip majors at 2.0); revisit then.
