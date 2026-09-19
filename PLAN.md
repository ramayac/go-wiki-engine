# Audit Fix Plan — Round 2 (feat/audit-fixes)

Waterfall plan for the second-round audit findings. Loop per phase:
code fix → unit test → pass → update docs → update this plan → commit.

Base: `feat/audit-fixes` (round 1 phases A–G merged). Phases continue the sequence.

## Definition of done (all phases)

- [ ] Fix implemented, matching existing code style
- [ ] Unit test proves the fix (negative scenarios included)
- [ ] Docs/prompts updated to match behavior
- [ ] `make test && make lint && make audit && make integration` green
- [ ] PLAN.md checkboxes updated, one commit per phase

## Phase H — P0: silent misbehavior

- [x] H1. Single-line `ignore = ["a", "b"]` arrays are parsed (today they are
      silently dropped); `]` on the last entry line also handled
- [x] H2. Empty/dangerous `wiki_dir` values (`""`, `.`) warn and fall back to
      the default instead of making `list`/`lint` walk the repo root
- [x] H3. `--json` usage errors emit the error envelope: search, summary,
      relevant, impact (both paths), diff, and the watch interval-0 guidance
      (`usageError` helper honoring `useJSONMode`)
- [x] Tests: `TestLoadSingleLineIgnoreArray`, `TestLoadIgnoreBracketOnEntryLine`,
      `TestLoadEmptyWikiDir`, integration `--json` usage cases
- [x] Docs: config.md ignore section; CHANGELOG

## Phase I — P1: real gaps

- [x] I1. `init` rejects wiki dirs that escape the repo (`..`, absolute, `.`)
- [x] I2. `currentPhase` prefers the last `in-progress` row, then the last
      `completed`, then the last row (today: last row regardless of status)
- [x] I3. Front matter inline-`#` stripping only for unquoted values and only
      after whitespace (`description: "C# guide"` stays intact)
- [x] I4. Strict `parseInt`/`parseFloat` (`1.5`, `12x` no longer become 15/12);
      invalid values warn and fall back
- [x] Tests: `TestInitRejectsTraversalWikiDir`, `TestCurrentPhasePreferences`,
      `TestParseFrontMatterHashInValue`, `TestStrictNumericParsing`
- [x] Docs: config.md; CHANGELOG

## Phase J — P2a: robustness & hygiene

- [x] J1. `upgrade` downloads are capped (100 MiB) before checksum verification
- [x] J2. `search` scans only `.md` files (binary-safe, matches `headings`)
- [x] J3. `sync-prompts` returns separate `updated` / `removed` lists (no more
      "updated removed X" and mixed JSON arrays)
- [x] J4. `context` JSON `line_count` gets `omitempty` (no misleading 0s)
- [x] J5. `search --` alone shows usage instead of "query is empty"
- [x] J6. `init` `.wikirc` rewrite uses regex instead of exact string match
- [x] Tests: `TestDownloadSizeLimit`, `TestSearchSkipsNonMarkdown`, updated
      scaffold sync signatures, integration sync-prompts JSON + `search --`
- [x] Docs: repo-map sync-prompts JSON shape; CHANGELOG

## Phase K — P2b: consistency & ergonomics

- [x] K1. Reject explicit `context --sort=...` without `--active` and
      `--minimal --active` (no more silent no-op combos)
- [x] K2. `orphans` checker skips `legacy`/`deprecated` pages (consistent with
      `leaf-pages`)
- [x] K3. `wiki-engine help` writes to stdout (explicit help request; errors
      keep usage on stderr)
- [x] Tests: integration combo rejection + help stream, `TestLintOrphansSkipsNonActive`
- [x] Docs: repo-map context row, operations/lint.md note; CHANGELOG

## Phase L — Final small fixes (release polish)

- [x] L1. `upgrade` fallback pinned to the discovered release tag
      (`go install ...@vX.Y.Z`) when the tag is known; `@latest` only when the
      tag lookup itself failed (prevents version skew)
- [x] L2. `context --active` warns on stderr when unlinked-page detection fails
      instead of silently dropping the list
- [x] L3. `replaceExecutable` fsyncs the staged binary before the rename
      (no zero-length binary on power loss)
- [x] L4. `lint --json` on a missing wiki dir carries the real diagnostic in
      the envelope `error` field (`LintResult.Reason`)
- [x] L5. `stale-content` fetches all page commit dates in one `git log` call
      instead of one process spawn per page
- [x] Tests: fallback pinning assertions, integration missing-wiki-dir error field
- [x] Docs: CHANGELOG

## Status

| Phase | Status | Commit |
|---|---|---|
| H — P0 silent misbehavior | ✅ | `fix: parse single-line ignore arrays; reject empty wiki_dir; JSON usage envelopes` |
| I — P1 real gaps | ✅ | `fix: reject escaping wiki dirs; active-phase preferences; YAML # rules; strict numeric parsing` |
| J — P2a robustness & hygiene | ✅ | `fix: cap upgrade downloads; search .md only; separate sync removed list` |
| K — P2b consistency & ergonomics | ✅ | `fix: reject meaningless context flag combos; orphans lifecycle rule; help on stdout` |
| L — Final small fixes | ✅ | `fix: pin upgrade fallback to the release tag; fsync swap; batched stale dates; lint reason` |

**Rounds 1–3 complete.** Branch `feat/audit-fixes` carries all rounds.
Final gates: `make test` (6 packages), `make lint`, `make audit`,
`make integration`, plus `go test -race ./...` and `make golangci-lint`.
