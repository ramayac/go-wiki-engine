# Lessons

Accumulated design insights from real usage sessions. Each entry records a gap that was discovered, the consequence, and what was built to close it.

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
