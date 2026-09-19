# Audit Fix Plan — feat/audit-fixes

Waterfall development plan for the findings of the pre-1.0 code & prompt audit.
Loop per phase: code fix → unit test → pass → update docs → update this plan → commit.

Base: `release/1.0-readiness`. Every phase commits once and only when `make test`,
`make lint`, `make audit`, and `make integration` pass.

## Definition of done (all phases)

- [ ] Fix implemented, matching existing code style
- [ ] Unit test proves the fix (negative scenarios included)
- [ ] Docs/prompts updated to match behavior
- [ ] `make test && make lint && make audit && make integration` green
- [ ] PLAN.md checkboxes updated, one commit per phase

## Phase A — Critical correctness

Fix the two release blockers: data loss and false success.

- [ ] A1. `sync-prompts` must never delete user-owned files: `cleanOrphanedFiles`
      removes only wiki-managed files (`wiki-*` basename, `.pi/skills/wiki/`,
      explicit retired list: migrate-shims.md, summarize.md)
- [ ] A2. `diff` must fail loudly on invalid git refs: `filesAtRef` verifies refs
      via `git rev-parse --verify`, `Diff` propagates every error
- [ ] Tests: `TestSyncPromptsPreservesUserFiles`, `TestDiffInvalidRef`
- [ ] Docs: repo-map `sync-prompts` row; CHANGELOG `[Unreleased] → Fixed`

## Phase B — JSON contract everywhere

- [ ] B1. `fatal()` emits the `{ok:false, error}` envelope in `--json` mode
      (unknown command, missing args, watch error path included)
- [ ] B2. `--json` implemented for `version`, `init`, `sync-prompts`, `upgrade`
- [ ] B3. `sync-prompts`: tip moved to stderr (never corrupts stdout), wiki-dir
      aware (`cfg.WikiDir`), dead "no instruction files" branch removed
- [ ] B4. `writeJSONResult` refactored to `writeJSONResultTo(io.Writer, ...)` for testability
- [ ] Tests: `TestWriteJSONResultTo`, integration: `--json version`
- [ ] Docs: repo-map JSON Output Contract corrected (omitempty reality, fatal-error envelope), usage text

## Phase C — Lint gate integrity

- [ ] C1. `lint --check=/-skip=` reject unknown checker names; `--skip=all` rejected
      (`validateLintSelectors` + `engine.KnownCheckerNames()`)
- [ ] C2. Lint on a missing wiki dir → one clear diagnostic instead of 18 noisy lines
      (early check in `LintWithOptions`)
- [ ] C3. Invalid `fail_severity` value warns on load and falls back to `warn`
- [ ] Tests: `TestValidateLintSelectors`, `TestLintMissingWikiDir`, `TestLoadInvalidFailSeverity`
- [ ] Docs: config.md `fail_severity` note; CHANGELOG

## Phase D — Context summarize coherence

- [ ] D1. Plain-text `context --summarize` prints per-page previews + line counts
      (currently computed but only emitted in JSON)
- [ ] D2. `context --active --summarize` (explicit flag) rejected with a clear
      error instead of being silently ignored (config-defaulted summarize stays ignored)
- [ ] D3. Fix `wiki-maintainer.md` prompt (live + scaffold): recommend
      `context --summarize`, not the no-op `--active --summarize`; `make sync-scaffold`
- [ ] Tests: integration additions (plain summaries, active+summarize rejection)
- [ ] Docs: config.md clarification; CHANGELOG

## Phase E — Error propagation & resource hygiene

- [ ] E1. `markersChecker`: close files per iteration (FD leak)
- [ ] E2. `extractZip`: close entry readers per iteration (FD accumulation)
- [ ] E3. `Headings`/`Search`: propagate `os.Open` errors (CHANGELOG claim made true)
- [ ] E4. `Refresh`: propagate `List`/`LogTail`/`Changed` errors
- [ ] E5. `upgrade`: verify the replacement by running `version` on the new binary
- [ ] Tests: `TestHeadingsPropagatesOpenError` (skipped as root)
- [ ] Docs: CHANGELOG

## Phase F — Unix ergonomics

- [ ] F1. `--` flag terminator for free-form commands (`search`, `impact`);
      post-`--` args are positional, terminator stripped from queries
- [ ] F2. `-h` / `--help` after any command shows usage and exits 0
- [ ] F3. Usage text: `--json` accepted anywhere + `--` documented
- [ ] Tests: `TestValidateCommandArgs` additions, `TestPositionalArgs`
- [ ] Docs: usage, CHANGELOG

## Phase G — Prompt & doc polish

- [ ] G1. `onboard.md` shim template lists all 7 slash commands (live + scaffold + sync)
- [ ] G2. `upgrade.md` wording: "confirming the new version" (matches new behavior)
- [ ] G3. repo-map `changed` row: drop the "non-ignored" claim (code never filtered ignores there)
- [ ] G4. Append dated entry to `wiki/prologue/log.md` for this fix round
- [ ] Tests: `make audit` (instruction-layer identity), full gate suite
- [ ] Docs: CHANGELOG final pass

## Status

| Phase | Status | Commit |
|---|---|---|
| A — Critical correctness | ⬜ | — |
| B — JSON contract | ⬜ | — |
| C — Lint gate integrity | ⬜ | — |
| D — Context summarize | ⬜ | — |
| E — Error propagation & resources | ⬜ | — |
| F — Unix ergonomics | ⬜ | — |
| G — Prompt & doc polish | ⬜ | — |
