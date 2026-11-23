---
description: "Update the wiki from current repository changes. Use when: ingesting a feature branch, filing architecture updates, or converting diffs into durable wiki knowledge."
name: "Wiki Ingest"
argument-hint: "Optional diff range or focus area..."
agent: "agent"
---

Ingest the current repository changes into the wiki.

> **Cold-start check:** If `wiki-engine changed` returns no output AND `wiki/prologue/log.md` has no prior ingest entries, this is a first-time onboarding — switch to the **Wiki Onboard** prompt instead of continuing here.

## Required context

- Start from active context: run `wiki-engine context --active` to get the current active wiki graph.
- Check each page's `lifecycle` before relying on it; ignore `planned` pages as current fact and follow `superseded_by` links to active replacements.
- Follow the core wiki editing guidelines defined in [wiki-maintainer.md](wiki-maintainer.md).
- If wiki-engine is not installed, read [wiki/index.md](../../wiki/index.md) and [wiki/repo-map.md](../../wiki/prologue/repo-map.md) instead.

## Execution steps

1. Run `wiki-engine changed` and `wiki-engine candidates` (branch merge-base plus working-tree and untracked files).
2. Run `wiki-engine check` and `wiki-engine impact <files...>` to identify broken citations and pages affected by changed sources.
3. Ignore repo-specific excluded paths (configured in `.wikirc`).
4. Read only the changed source files that are relevant to the ingest.
5. Decide whether each durable fact belongs in an existing active wiki page or needs a new page.
6. When creating or modifying pages:
   - Ensure every page has valid YAML front matter (`id`, `title`, `type`, `lifecycle`, `description`) — run `wiki-engine schema --type <type>` for templates.
   - Add exact `sources:` entries for all repo files or directories documented by the page.
   - If replacing or retiring an existing page, set `lifecycle: superseded` and `superseded_by: "<replacement-id-or-path>"`.
   - Use standard relative Markdown links (e.g., `[Text](file.md)`) and cross-link related pages (`related`, `depends_on` for entities).
7. Update [wiki/index.md](../../wiki/index.md) if coverage or lifecycle changed.
8. Append an ingest entry to [wiki/log.md](../../wiki/prologue/log.md) using the required heading format.
9. Run `wiki-engine lint` and `wiki-engine check` to verify wiki structure, links, and citations, plus `wiki-engine context --active` to confirm graph connectivity.

Finish by summarizing what was added, what changed, what was superseded (if any), and what still needs human review.
