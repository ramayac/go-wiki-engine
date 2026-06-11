# TODO — Improvement Backlog

Refer to the [Improvement Plan](improvement-plan.md) for the detailed roadmap and phase breakdown.

Ranked by implementation difficulty within each tier. Effort assumes familiarity with the codebase.
✅ = completed.

---

## Tier 1 — Easy (1–2 hours each) ✅ ALL DONE

| # | Task | Effort | Status |
|---|------|--------|--------|
| 1 | **Dogfood: populate repo-map.md** | 1h | ✅ Updated with new commands, pi.dev integration, expanded subcommand table |
| 2 | **Dogfood: add ingest entries to log.md** | 30m | ✅ Added 2026-05-28 ingest entry covering all changes |
| 3 | **Add CI test workflow** | 1h | ✅ `.github/workflows/test.yml`: vet + test + build on push/PR |
| 4 | **`wiki-engine stats` command** | 1.5h | ✅ `engine.Stats()`, CLI wired with `--json` support |
| 5 | **`--json` flag scaffold** | 1.5h | ✅ `--json` on all commands, consistent envelope shape |
| 6 | **Add `lessons.md` to index.md** | 15m | ✅ Added `lessons.md` and `todo.md` to index |
| 7 | **Add stub tests for untested packages** | 1h | ✅ `cmd/wiki-engine/main_test.go`, `upgrade_test.go`; 5→0 untested pkgs |

---

## Tier 2 — Medium (3–8 hours each) ~90% DONE

| # | Task | Effort | Status |
|---|------|--------|--------|
| 8 | **pi.dev skill: `.pi/skills/wiki/SKILL.md`** | 3h | ✅ Skill file created, scaffold updated, `SyncPrompts` expanded, tests added |
| 9 | **agy integration: `.agy/commands/`** | 3h | ⬜ Deferred — needs research on agy's command format |
| 10 | **`wiki-engine context` command** | 4h | ✅ `engine.Context()`, catalog from index, recent log, phase status. `--minimal` mode |
| 11 | **Update `.wiki-instructions/` to use `context`** | 2h | ✅ All 5 workflow files updated to reference `wiki-engine context` |
| 12 | **Cross-page link validation** | 4h | ✅ `crossPageLinksChecker` — all .md files, strips inline code, tries relative + wiki-root paths |
| 13 | **Orphan page detection** | 2h | ✅ `orphansChecker` — detects .md files not in index.md (with skip list for meta pages) |
| 14 | **Heading hierarchy validation** | 3h | ✅ `headingHierarchyChecker` — skipped levels + multiple h1s |
| 15 | **Phase consistency check** | 2h | ✅ `phaseConsistencyChecker` — validates status values against known set |
| 16 | **`wiki-engine summary <page>` command** | 2h | ✅ `engine.Summary()` — first heading + first paragraph preview |
| 17 | **Log chronology validation** | 2h | ✅ `logChronologyChecker` — descending date order check |

---

## Tier 3 — Hard (1–3 days each) ✅ ALL DONE

| # | Task | Effort | Status |
|---|------|--------|--------|
| 18 | **`wiki-engine relevant <query>` command** | 1.5d | ✅ `engine.Relevant()` — heading ×3 + body scoring, sorted, top-N |
| 19 | **`wiki-engine impact <changed-files>` command** | 1d | ✅ `engine.Impact()` — maps changed files to wiki pages that mention them |
| 20 | **Refactor lint to composable checker pattern** | 1d | ✅ `Checker` interface, 9 implementations, `allCheckers()`, severity levels |
| 21 | **`--json` flag on all commands** | 1d | ✅ Consistent JSON envelope on `list`, `headings`, `search`, `log-tail`, `changed`, `candidates`, `lint`, `stats`, `context`, `summary`, `relevant` |
| 22 | **Integration tests** | 1.5d | ✅ `test/integration_test.sh` — 14 end-to-end tests covering init, list, search, lint, stats, context, summary, relevant, json, diff, watch, impact, cache, duplicate detection |
| 23 | **Dead link detection to external files** | 1d | ✅ `externalLinksChecker` — validates wiki→source-file links, part of lint |

---

## Tier 4 — Harder (design + implementation, 2–4 days each) ~75% DONE

| # | Task | Effort | Status |
|---|------|--------|--------|
| 24 | **Wiki cache: `.wiki/.cache.json`** | 2d | ✅ `engine_cache.go` — mtime-validated cache, `--rebuild-cache` flag, `cache_enabled` config |
| 25 | **Recursive `.wikirc` lookup** | 2d | ⬜ Deferred |
| 26 | **`wiki-engine diff <from> <to>` command** | 2d | ✅ `engine.Diff()` — added/removed/changed wiki files between two git refs |
| 27 | **Severity levels in lint** | 1.5d | ✅ Already done — `SevInfo`, `SevWarn`, `SevError` |

---

## Tier 5 — Long-term / speculative ~60% DONE

| # | Task | Effort | Status |
|---|------|--------|--------|
| 28 | **Duplicate content detection** | 3d | ✅ `duplicateContentChecker` — Jaccard similarity, `duplicate_threshold` config |
| 29 | **Stale content signal** | 2d | ✅ `staleContentChecker` — mtime threshold, `stale_days` config, source-change-aware |
| 30 | **`wiki-engine watch` mode** | 3d | ✅ `engine.WatchOnce()` + `watch` command with `--once` and continuous modes, `watch_interval` config |
| 31 | **Wiki template system** | 3d | ⬜ |
| 32 | **Multi-repo wiki aggregation** | 4d | ⬜ |

---

## Tier 6 — Hardening & Page Lifecycle (New Backlog)

Refer to the [Improvement Plan](improvement-plan.md) for full phase breakdown.

| # | Task | Effort | Status |
|---|------|--------|--------|
| 33 | **Fix lint severity gating (exit code)** | 0.5h | ⬜ Planned |
| 34 | **Front matter schema & YAML parser** | 4h | ⬜ Planned |
| 35 | **Front matter lint checker** | 2h | ⬜ Planned |
| 36 | **Lifecycle filtering in list/context (`--active`)** | 3h | ⬜ Planned |
| 37 | **Index format checker** | 1.5h | ⬜ Planned |
| 38 | **Slash command lifecycle awareness** | 2h | ⬜ Planned |
| 39 | **Bare URL and HTML link checker** | 1h | ⬜ Planned |
| 40 | **`--check` / `--skip` lint flags** | 2h | ⬜ Planned |
| 41 | **CI linting pipeline setup** | 1h | ⬜ Planned |
| 42 | **Dead code cleanup** | 0.5h | ⬜ Planned |
| 43 | **Prompt restructuring (reduce overlap)** | 3h | ⬜ Planned |
| 44 | **Wiki front matter migration** | 1h | ⬜ Planned |
