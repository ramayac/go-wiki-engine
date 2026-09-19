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

## Status

| Phase | Status | Commit |
|---|---|---|
| H — P0 silent misbehavior | ✅ | `fix: parse single-line ignore arrays; reject empty wiki_dir; JSON usage envelopes` |
| I — P1 real gaps | ✅ | `fix: reject escaping wiki dirs; active-phase preferences; YAML # rules; strict numeric parsing` |
| J — P2a robustness & hygiene | ✅ | `fix: cap upgrade downloads; search .md only; separate sync removed list` |
| K — P2b consistency & ergonomics | ✅ | `fix: reject meaningless context flag combos; orphans lifecycle rule; help on stdout` |

**Round 2 complete.** Branch `feat/audit-fixes` carries both rounds.
Final gates: `make test` (6 packages), `make lint`, `make audit`,
`make integration`, plus `go test -race ./...` and `make golangci-lint`.
