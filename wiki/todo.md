---
status: current
description: "Improvement backlog categorized by tier and priority."
superseded_by: ""
---
# TODO — Improvement Backlog

Ranked by implementation difficulty within each tier. Effort assumes familiarity with the codebase.

---

## Tier 2 — Medium (3–8 hours each)

| # | Task | Effort | Status |
|---|------|--------|--------|
| 9 | **agy integration: `.agy/commands/`** | 3h | ⬜ Deferred — needs research on agy's command format |

---

## Tier 4 — Harder (design + implementation, 2–4 days each)

| # | Task | Effort | Status |
|---|------|--------|--------|
| 25 | **Recursive `.wikirc` lookup** | 2d | ⬜ Deferred |

---

## Tier 5 — Long-term / speculative

| # | Task | Effort | Status |
|---|------|--------|--------|
| 31 | **Wiki template system** | 3d | ⬜ |
| 32 | **Multi-repo wiki aggregation** | 4d | ⬜ |

---

## Tier 6 — Hardening & Page Lifecycle (New Backlog)

Refer to the [Improvement Plan](improvement-plan.md) for the detailed roadmap and phase breakdown of the hardening tasks.

| # | Task | Effort | Status |
|---|------|--------|--------|
| 33 | **Fix lint severity gating (exit code)** | 0.5h | ✅ Done |
| 34 | **Front matter schema & YAML parser** | 4h | ✅ Done |
| 35 | **Front matter lint checker** | 2h | ✅ Done |
| 36 | **Lifecycle filtering in list/context (`--active`)** | 3h | ✅ Done |
| 37 | **Index format checker** | 1.5h | ✅ Done |
| 38 | **Slash command lifecycle awareness** | 2h | ✅ Done — updated prompts to use context --active |
| 39 | **Bare URL and HTML link checker** | 1h | ✅ Done |
| 40 | **`--check` / `--skip` lint flags** | 2h | ✅ Done |
| 41 | **CI linting pipeline setup** | 1h | ✅ Done — configured .github/workflows/test.yml to run make lint on push/PR |
| 42 | **Dead code cleanup** | 0.5h | ⬜ Planned |
| 43 | **Prompt restructuring (reduce overlap)** | 3h | ✅ Done — extracted shared steps, merged shims, moved summarize to maintaining guidelines |
| 44 | **Wiki front matter migration** | 1h | ✅ Done |
| 45 | **Shared Link-Parsing Helper** | 2h | ✅ Done |
| 46 | **Directed Graph Builder (`BuildWikiGraph()`)** | 5h | ✅ Done |
| 47 | **Topological/Chronological Sorting of Graph** | 2h | ✅ Done |
| 48 | **Compact Context Graph Export (`context --active`)** | 4h | ✅ Done |

---

## Tier 7 — Loose Ends & Bookkeeping

Address **after** the main improvement plan (Phases 1–5). These are housekeeping items surfaced during the 2026-06-10 wiki audit.

| # | Task | Effort | Status |
|---|------|--------|--------|
| 49 | **Fix `.wikirc` branch name inconsistency** | 5m | ✅ Done — standardized `.wikirc.example` to `master...HEAD` |
| 50 | **Sync `summarize.md` into live `.wiki-instructions/`** | 5m | ✅ Done — ran `wiki-engine sync-prompts` |
| 51 | **Add front matter to all wiki pages** | 1h | ✅ Done — all wiki pages migrated to include YAML front matter |
| 52 | **Update `phases.md` with forward-looking entries** | 15m | ⬜ Deferred — add Phase 3 (linter hardening), Phase 4 (lifecycle), Phase 5 (CI) status rows |
| 53 | **Clean up `todo.md` stale status markers** | 15m | ⬜ Deferred — prune completed Tier 1–5 items, renumber remaining; mark `impact` (#19 in original) as done |
