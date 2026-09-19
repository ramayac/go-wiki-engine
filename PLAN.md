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

- [x] A1. `sync-prompts` must never delete user-owned files: `cleanOrphanedFiles`
      removes only wiki-managed files (`wiki-*` basename, `.pi/skills/wiki/`,
      explicit retired list: migrate-shims.md, summarize.md)
- [x] A2. `diff` must fail loudly on invalid git refs: `filesAtRef` verifies refs
      via `git rev-parse --verify`, `Diff` propagates every error
- [x] Tests: `TestSyncPromptsPreservesUserFiles`, `TestDiffInvalidRef`,
      `TestDiffRefsBeforeWikiExisted`
- [x] Docs: repo-map `sync-prompts` row; CHANGELOG `[Unreleased] → Fixed`

## Phase B — JSON contract everywhere

- [x] B1. `fatal()` emits the `{ok:false, error}` envelope in `--json` mode
      (unknown command, missing args, watch error path included)
- [x] B2. `--json` implemented for `version`, `init`, `sync-prompts`, `upgrade`
- [x] B3. `sync-prompts`: tip moved to stderr (never corrupts stdout), wiki-dir
      aware (`cfg.WikiDir`), dead "no instruction files" branch removed
- [x] B4. `writeJSONResult` refactored to `writeJSONResultTo(io.Writer, ...)` for testability
- [x] Tests: `TestWriteJSONResultTo`, integration: `--json version`/`--json sync-prompts`/error envelopes
- [x] Docs: repo-map JSON Output Contract corrected (omitempty reality, fatal-error envelope), usage text

## Phase C — Lint gate integrity

- [x] C1. `lint --check=/-skip=` reject unknown checker names; `--skip=all` rejected
      (`validateLintSelectors` + `engine.KnownCheckerNames()`)
- [x] C2. Lint on a missing wiki dir → one clear diagnostic instead of 18 noisy lines
      (early check in `LintWithOptions`)
- [x] C3. Invalid `fail_severity` value warns on load and falls back to `warn`
- [x] Tests: `TestValidateLintSelectors`, `TestLintMissingWikiDir`, `TestLoadInvalidFailSeverity`,
      integration (unknown checker, `--skip=all`)
- [x] Docs: config.md `fail_severity` note; CHANGELOG

## Phase D — Context summarize coherence

- [x] D1. Plain-text `context --summarize` prints per-page previews + line counts
      (currently computed but only emitted in JSON)
- [x] D2. `context --active --summarize` (explicit flag) rejected with a clear
      error instead of being silently ignored (config-defaulted summarize stays ignored)
- [x] D3. Fix `wiki-maintainer.md` prompt (live + scaffold): recommend
      `context --summarize`, not the no-op `--active --summarize`; `make sync-scaffold`
- [x] Tests: integration additions (plain summaries, active+summarize rejection)
- [x] Docs: config.md clarification; CHANGELOG

## Phase E — Error propagation & resource hygiene

- [x] E1. `markersChecker`: close files per iteration (FD leak)
- [x] E2. `extractZip`: close entry readers per iteration (FD accumulation)
- [x] E3. `Headings`/`Search`: propagate `os.Open` errors (CHANGELOG claim made true)
- [x] E4. `Refresh`: propagate `List`/`LogTail`/`Changed` errors
- [x] E5. `upgrade`: verify the replacement by running `version` on the new binary
- [x] Tests: `TestHeadingsPropagatesOpenError` (skipped as root)
- [x] Docs: CHANGELOG

## Phase F — Unix ergonomics

- [x] F1. `--` flag terminator for free-form commands (`search`, `impact`);
      post-`--` args are positional, terminator stripped from queries;
      `--` also protects literal `--json` arguments
- [x] F2. `-h` / `--help` after any command shows usage and exits 0
      (terminator-aware: `search -- -h` searches for `-h`)
- [x] F3. Usage text: `--json` accepted anywhere + `--` documented
- [x] Tests: `TestValidateCommandArgs` additions, `TestPositionalArgs`,
      `TestArgsAfterFilters` terminator cases
- [x] Docs: usage, CHANGELOG

## Phase G — Prompt & doc polish

- [x] G1. `onboard.md` shim template lists all 7 slash commands (live + scaffold + sync)
- [x] G2. `upgrade.md` wording: "confirming the new version" (matches new behavior)
- [x] G3. repo-map `changed` row: drop the "non-ignored" claim (code never filtered ignores there)
- [x] G4. Append dated entry to `wiki/prologue/log.md` for this fix round
- [x] Tests: `make audit` (instruction-layer identity), full gate suite
- [x] Docs: CHANGELOG final pass

## Status

| Phase | Status | Commit |
|---|---|---|
| A — Critical correctness | ✅ | `fix: sync-prompts must not delete user files; diff fails loudly on invalid refs` |
| B — JSON contract | ✅ | `fix: honor --json on every command and on fatal error paths` |
| C — Lint gate integrity | ✅ | `fix: validate lint selectors and fail_severity; single diagnostic for missing wiki dir` |
| D — Context summarize | ✅ | `fix: plain context --summarize shows previews; reject --active --summarize` |
| E — Error propagation & resources | ✅ | `fix: propagate scanner open errors; per-file FD hygiene; verify upgraded binary` |
| F — Unix ergonomics | ✅ | `fix: -- flag terminator and -h after commands` |
| G — Prompt & doc polish | ✅ | `fix: prompt polish — onboard shim lists all commands, upgrade wording, changed row` |
