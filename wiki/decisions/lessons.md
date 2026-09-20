---
status: current
description: "Lessons learned during design, testing, and implementation."
superseded_by: ""
---
# Lessons

Accumulated design insights from real usage sessions. Each entry records a gap that was discovered, the consequence, and what was built to close it.

Related: [log.md](../prologue/log.md) — chronological record of every change; [todo.md](todo.md) — gaps still open; [repo-map.md](../prologue/repo-map.md) — the architecture these lessons shaped.

---

## 2026-09-20 — Front matter declarations must be exempt from prose checkers

### What happened

Dogfooding the new `references` front matter field (`external:https://...` on release.md) made `wiki-engine lint` fail immediately: the `bare-urls` checker scans every line as prose and flagged the declared URL as a bare URL. The declaration was valid; the checker just didn't know about it.

### Why it matters

Every new structured declaration added to front matter can collide with a checker that treats the whole file as prose. If the first adopter hits a false positive, the convention dies on contact.

### The fix

`bareUrlChecker` now skips the front matter block entirely; front matter declarations are validated by purpose-built checkers (`references`), not prose checkers. Regression test proves a valid `external:` reference produces no bare-urls issue.

### Key design principle confirmed

**Structured declarations get structured validators.** When a field carries machine-readable meaning, the prose-level checkers must step aside and a dedicated checker must own it.

---

## 2026-09-20 — Declared references are authoritative; prose is a fallback only

### What happened

The naive `impact` design (basename text scan everywhere) made declared references pointless: a page that declared `source:internal/engine/graph.go` but mentioned `main.go` in prose would be flagged as impacted by any `main.go` change — a false positive the declaration was meant to prevent.

### The fix

Two-tier `impact`: pages with declared `source:` references match only on those exact paths; pages without references keep the basename text scan. Declarations are authoritative, fallback is for undeclared pages.

### Key design principle confirmed

**Explicit metadata must override inference, not compete with it.** When both signals exist, the structured one wins; the heuristic applies only where structure is absent.

---

## 2026-09-20 — Every mode × flag combination needs a test

### What happened

A post-implementation audit found two logic gaps in the `graph` command that all green tests had missed: `graph --json <page>` silently ignored the page argument (returned the whole graph), and `graph <page> --strict` exited 0 on an unhealthy wiki (the strict gate lived only in the tree branch). Both were gaps between modes, invisible to tests that exercised each mode in isolation.

### The fix

Integration coverage for the combinations: JSON + neighborhood, strict + neighborhood. The neighborhood is now computed once before mode dispatch, so every branch shares the same strict gate.

### Key design principle confirmed

**Test the cross-product of modes and flags, not just the modes.** Command surface grows combinatorially; the seams between branches are where arguments get silently dropped.

---

## 2026-09-20 — Every checker must respect page lifecycle

### What happened

A follow-up audit found the new `references` checker was the only checker that validated `legacy`/`deprecated` pages. Deprecated pages legitimately point at retired files — that is often the reason they were retired — so a single broken `source:` reference on an archived page would fail lint forever, forcing edits to pages the lifecycle contract says to leave untouched. `orphans` and `leaf-pages` already skipped them; the new checker forgot the convention.

### The fix

`referencesChecker` skips `legacy`/`deprecated` pages, matching `orphans`/`leaf-pages`. Regression test proves a deprecated page with broken references produces no issues.

### Key design principle confirmed

**Checkers share one lifecycle contract.** When a new validator joins the suite, its first question is "does this apply to archived pages?" — and the answer must match the existing checkers, or lint punishes pages that are deliberately frozen.

---

## 2026-05-01 — The prompt duplication trap

### What happened

The `.github/prompts/` files contained Copilot-specific YAML frontmatter. To add Claude Code support, the naive approach would be to duplicate each workflow file into `.claude/commands/`. But five workflows × two tools = drift. Every edit to a prompt would need to be manually mirrored.

### Why it matters

Duplicated workflow files **diverge**. One tool's version gets improved, the other stays stale. Or worse, a well-meaning contributor edits the copy in `.claude/` and now the two tools give different instructions to the agent.

### The fix

Created `.wiki-instructions/` as the **canonical home** for all workflow definitions. Each tool directory contains symlinks back to the canonical files:

```
.wiki-instructions/ingest.md     ← edit here
.github/prompts/wiki-ingest.prompt.md  → symlink
.claude/commands/wiki-ingest.md         → symlink
```

Tool compatibility is handled via frontmatter fields:
- Both Copilot and Claude Code use `description`
- Copilot-specific fields (`name`, `argument-hint`, `agent`) are silently ignored by Claude Code
The filename difference gives each tool its preferred naming convention.

Build integration: `go:embed` follows symlinks at compile time, embedding regular file content. `make sync-scaffold` uses `cp -rL` to dereference symlinks so the embedded FS is self-contained. No changes needed to `Init()` or the build pipeline.

### Key design principle confirmed

**Symlinks are the DRY mechanism for tool conventions.** Don't duplicate workflow files. Don't generate them. Just symlink from each tool's directory to a single canonical source. Every tool's convention is honored without maintaining parallel copies.

---

## 2026-04-16 — The prompt-upgrade gap

### What happened

A new prompt (`wiki-onboard.prompt.md`) and improvements to the `wiki-maintainer.instructions.md` were added to the go-wiki-engine scaffold. Existing repos that had already run `wiki-engine init` had no way to receive these updates. `wiki-engine upgrade` only reinstalls the binary via `go install` — it never touches the `.github/` files already in place.

### Why it matters

Prompts and instructions are the **intelligence layer** of the system. If the binary improves its scaffold but existing repos are silently stuck on the old version, users get none of the benefit. Worse, there is no visible signal that anything is missing — the old prompts just continue to work, masking the drift.

### The fix

Added `wiki-engine sync-prompts`: a new subcommand that overwrites all files under `.github/prompts/` and `.github/instructions/` with the current embedded versions. It explicitly does **not** touch `wiki/` content or `.wikirc`, both of which are user-authored.

After a `wiki-engine upgrade`, users run:

```bash
wiki-engine sync-prompts
```

The `upgrade` subcommand now prints a reminder to do this.

### Key design principle confirmed

**Separate user-authored files from tool-authored files.** The rule is:

| Path | Owner | On upgrade |
|---|---|---|
| `wiki/` | User (agent writes, human reviews) | Never overwrite |
| `.wikirc` | User | Never overwrite |
| `.github/prompts/` | Tool (wiki-engine scaffold) | Safe to overwrite |
| `.github/instructions/` | Tool (wiki-engine scaffold) | Safe to overwrite |

Any new scaffolded path must be categorized this way before shipping.

---

## 2026-04-16 — The cold-start / incremental-ingest confusion

### What happened

A brand-new project (Mana-world-shift) was onboarded using `/wiki-ingest` — the only prompt available at the time. `wiki-engine changed` returned nothing (correct: no git diff on an empty wiki, no prior commits to compare against), and the prompt gave no fallback guidance. The agent had to improvise: manually surveying the repo, filling in `repo-map.md`, creating topic pages, and migrating external docs — all steps that should be in a prompt.

### Why it matters

An incremental ingest (comparing a diff range) and a cold-start survey (reading the whole repo from scratch) are **fundamentally different operations**. Conflating them leaves the cold-start case either silently ignored or handled inconsistently across sessions.

### The fix

Added `wiki-onboard.prompt.md` as an explicit cold-start prompt. It:
- Falls back to manual repo survey when `wiki-engine changed` is empty
- Checks for external knowledge files (`docs/`, `AGENTS.md`, etc.) to migrate before creating new pages
- Requires a fully filled `repo-map.md` (no placeholder comments) before moving on
- Advances `phases.md` phases 1+2 as part of its own steps

Updated `wiki-ingest.prompt.md` with a detection hint: if `wiki-engine changed` is empty and `log.md` has no entries, switch to Wiki Onboard.

### Key design principle confirmed

**Prompts are workflows, not just descriptions.** A prompt that says "do the ingest" but gives no fallback for the case where there's nothing to diff is incomplete. Every prompt should handle its failure modes explicitly.

---

## 2026-04-16 — External docs outside wiki/ are invisible to the ingest loop

### What happened

Mana-world-shift had three durable knowledge files outside `wiki/`: `docs/bigPlan.md`, `docs/lessons001.md`, and `AGENTS.md`. None of the existing ingest/refresh operations mentioned checking for external docs. They would have been silently skipped on every future ingest cycle, left to drift out of sync with the wiki.

### The fix

Added an **External Docs Migration Rule** to `wiki/operations/ingest.md`: before ingesting code changes, check for files in `docs/`, `AGENTS.md`, `CONTRIBUTING.md`, `ARCHITECTURE.md` that contain durable knowledge. If found and not yet in the wiki, migrate them — copy to `wiki/<name>.md`, replace the original with a stub redirect, and log the migration.

Added the same step to the `wiki-onboard` prompt (step 2) so cold-starts catch it automatically.

### Key design principle confirmed

**The wiki is the single source of truth.** Any durable knowledge living outside `wiki/` will drift. The ingest loop must actively look for and absorb external knowledge files, not just react to git diffs of source code.

---

## 2026-04-17 — The AI entrypoint gap

### What happened

A repo initialized with `wiki-engine init` had a well-populated wiki. But when a developer opened the repo in a new AI tool (e.g., Claude Code or a different agent runtime), the agent had no idea the wiki existed. It would scan the file tree, find source code, and start reasoning from scratch — ignoring `wiki/index.md` entirely.

The convention files that AI tools consult on startup (`AGENTS.md`, `CLAUDE.md`) were either missing or — as in the Mana-world-shift session — contained full hand-written documentation that duplicated or diverged from the wiki.

### Why it matters

The wiki has no value if agents never read it. The prompts and instructions in `.github/` only help tools that already know to look there (VS Code Copilot with the extension installed). Tools that use `AGENTS.md` or `CLAUDE.md` as their context entrypoint would completely bypass the wiki system.

This compounds the external-docs problem: instead of one place for truth, you get three (wiki, AGENTS.md, CLAUDE.md) that drift independently.

### The fix

Added `AGENTS.md` and `CLAUDE.md` as **shim files** in the scaffold. Both files redirect to `wiki/index.md` with a single sentence. They are:

- Created by `wiki-engine init` if they do not exist
- Created by `wiki-engine sync-prompts` if they do not exist (so existing repos can get them without re-initing)
- **Never overwritten** if they already exist — user-customised content is preserved

The same create-only semantics that `Init` already used for `.wikirc` are now applied to these files via the internal `syncShims()` helper.

### Key design principle confirmed

**Match the conventions of every tool in the ecosystem.** Different AI tools look for context in different places. The scaffold should install shims for all known entrypoint conventions, each pointing back to the single source of truth.

---

## 2026-06-10 — Hardened Upgrade Checksum Verification and Loop Defer Resource Management

### What happened

During linter hardening and prompt lifecycle updates:
1. We wanted to secure the self-upgrade command (`wiki-engine upgrade`) from downloading unverified binaries directly from GitHub.
2. In the linter loop implementations of `internal/engine/engine.go`, we observed code that processed multiple files under a loop using manual `f.Close()` calls. Attempting to use a standard `defer f.Close()` within a loop causes file handles to accumulate until the enclosing method returns, risking file descriptor exhaustion.
3. Synchronizing prompts using `wiki-engine sync-prompts` previously left deprecated or removed prompt files (such as `migrate-shims.md` and `summarize.md`) in the target directories because the file sync did not account for orphaned target files.

### Why it matters

- **Security Integrity:** Downloading and running binaries without hash verification makes the tool vulnerable to compromised delivery vectors (e.g., tampered release assets).
- **Resource Leaks:** CLI tools operating on large codebases can hit operating system limits on open file descriptors if loops do not release file resources immediately.
- **Scaffold Hygiene:** Orphaned command templates and stale instruction configurations in user repos cause AI agents to try to invoke commands that no longer exist or are deprecated, resulting in prompt drift and overlap.

### The fix

- **Release Asset Hashing:** Upgraded `upgrade.Run()` to fetch `checksums.txt` from the GitHub release tag, resolve the matching asset for the running OS/Arch, compute its SHA-256 hash, and verify it matches the record in `checksums.txt` before replacing the binary. Added a fallback to `go install` for development/offline envs.
- **Anonymous Loop Defers:** Wrapped loop iteration file tasks inside an anonymous function block:
  ```go
  for _, rel := range files {
      func() {
          f, err := os.Open(abs)
          if err != nil { return }
          defer f.Close()
          // ... file scanning ...
      }()
  }
  ```
  This guarantees that `defer` evaluates and closes file handles immediately on each iteration.
- **Orphan Cleanup:** Added checks to remove obsolete files from the target directories that are no longer part of the template scaffold during synchronization.

### Key design principle confirmed

**Continuous hygiene is critical for both plumbing and instruction layers.** Low-level CLI operations must manage OS resources defensively, while high-level AI prompt instructions must be synchronized and kept free of stale or redundant files to prevent agent context pollution.

---

## 2026-08-14 — Improvement plan retired; its design decisions archived here

### What happened

The `wiki/improvement-plan.md` roadmap (phases 1–5) was fully implemented and verified. Rather than letting the document drift as a half-historical, half-current page, it was marked `deprecated` with `superseded_by: repo-map.md`, and the durable decisions from its "Part 7" were folded into this page so future sessions still see why the code works the way it does.

### Design decisions that remain binding

1. **YAML parser scope** — the front matter parser is flat `key: value` pairs with single-line values only; no nested YAML, no external dependencies.
2. **Graph traversal cutoff** — BFS from `index.md` halts at `legacy`/`deprecated` nodes; an active page reachable only through an inactive path is excluded from `context --active`.
3. **Graph export formats** — `context --active` emits JSON `nodes` + `edges` with `--json`, and sorted text output otherwise.
4. **Severity gate** — `fail_severity` in `.wikirc` controls which severity fails `wiki-engine lint` (default `warn`).
5. **Missing front matter fallback** — pages without front matter are treated as `current` so unmigrated wikis keep working, but lint warns about the missing metadata.
6. **`superseded_by` validation** — the linter requires the target page to exist and be active (`current` or `planned`).
7. **Sorting options** — chronological by default with precedence `updated` → `created` → filesystem mtime; `--sort=topo` switches to depth order.
8. **Reference-style links** — `[text][ref]` links are flagged at warn severity by the markdown-format checker (added 2026-08-14, completing the optional Goal 4 item).

### Key design principle confirmed

**Retire plans into their artifacts.** A roadmap's value ends when it ships; its decisions don't. Archive the decisions in `lessons.md` and deprecate the plan page with `superseded_by`, instead of maintaining an indefinitely "current" plan document.

---

## 2026-08-15 — Wiki directory structure: canonical paths need fallback candidates

### What happened

Issue #5 asked for wiki pages grouped into subdirectories (`prologue/`, `decisions/`, `architectures/`) instead of one flat folder. Most of the engine already handled nesting (search, graph, link resolution are directory-relative), but six call sites hardcoded root-level paths: `LogTail`, `currentPhase`, and the `log-headings`, `log-chronology`, `phase-consistency`, `required-files` checkers, plus the `leaf-pages` and `stale-content` exemptions.

### Why it matters

Moving `log.md` and `phases.md` into `prologue/` would have silently broken `log-tail` and four lint checkers for every existing flat wiki. A hard break forces all current users to migrate on upgrade; a graceful one keeps old and new layouts valid during the transition.

### The fix

Added `canonicalPathCandidates` in `internal/engine/paths.go`: each logical file maps to an ordered list of accepted paths, organized layout first, legacy flat path second. `resolveWikiFile` picks the first existing candidate, and `required-files` reports a file missing only when every candidate is absent. Nested-layout fixtures and a legacy flat-layout block in the integration test lock the back-compat behavior in.

### Key design principle confirmed

**Prefer candidate lists over hardcoded paths for structural files.** When a directory convention changes, resolve logical files through a small indirection layer with legacy fallbacks — consumers stay readable, old layouts keep working, and the migration pressure lands only on new scaffolds.

---

## 2026-08-15 — Tolerance in validators hides the bugs they exist to find

### What happened

The follow-up audit of the organized-structure migration found real gaps that `wiki-engine lint` had reported as clean: `prologue/config.md` linked `operations/lint.md` (missing `../`), and the root `README.md`, scaffold templates, and prompt files still referenced `wiki/log.md`-style flat paths in prose. Lint passed because `crossPageLinksChecker` had a "try relative to wiki dir" fallback that made subdirectory pages resolve as if they lived at the root, and because lint only scans markdown links inside `wiki/` — prose references and non-wiki files are invisible to it.

### Why it matters

Every tolerance in a validator is a hole where drift accumulates silently. The fallback was added for convenience and converted a whole class of wrong links into accepted ones. Non-wiki files (README, prompts, scaffold) are where agents and users land first, so stale references there cost more than broken links deep in the wiki.

### The fix

- Removed the wiki-root fallback from `crossPageLinksChecker`: links are strictly page-relative, with a regression test proving a subdir page must use `../` for siblings.
- Added `internal/audit` — Go tests that run under `go test ./...` and `make audit`, checking strict wiki links, scaffold template links, front matter/superseded_by, graph reachability, duplicate basenames, active leaves, `wiki/<path>.md` references in every non-wiki `.md` file, instruction-layer identity between `.wiki-instructions/`/`.pi/` and `scaffold/`, and embedded-scaffold sync.
- Wired `make audit` into CI and documented it in the lint workflow, wiki-maintainer checklist, and operations/lint.md.

### Key design principle confirmed

**Automate the audit that found the bugs, and remove the tolerance that hid them.** A validator with a forgiving fallback and a checker with a blind spot (only wiki links) both failed — the fix is stricter semantics plus a repo-wide test that runs on every commit.
