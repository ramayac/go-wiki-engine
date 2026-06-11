---
description: "Use when maintaining the repo wiki, ingesting repository changes into wiki/, answering architecture questions from the wiki first, or linting wiki coverage and cross-references."
name: "Wiki Maintainer"
---

# Wiki Maintainer

## Core rules

- Treat `wiki/` as the persistent knowledge layer for this repository.
- Start broad repo-analysis tasks by running `wiki-engine context --active`, then reading only the relevant active pages from the catalog.
- Update the wiki incrementally instead of rewriting it from scratch.
- Keep wiki files plain Markdown with stable filenames and grep-friendly headings.
- Use standard relative Markdown links (e.g., `[Link Text](path/to/file.md)`) for all cross-page references. Never use HTML links, absolute paths, or wiki-style links (like `[[Page]]`).
- Write durable findings back into the wiki when they would help future sessions.

## Prompt selection

| Situation | Use prompt |
|---|---|
| Wiki is empty or only has template placeholders | `wiki-onboard` |
| Absorbing a feature branch or batch of commits | `wiki-ingest` |
| Answering a question about the repo | `wiki-query` |
| Periodic health check, fixing drift | `wiki-refresh` |
| Structural issues, broken links, formatting errors, or missing metadata | `wiki-lint` |

## Page Lifecycle Statuses

Every wiki page must contain a YAML front matter block specifying its status:
- **`planned`**: Page created as a placeholder/todo. Agents may update it to fill in content.
- **`current`**: Page is active, accurate, and represents the current state. Read by default.
- **`legacy`**: Page is outdated but kept for historical context. Excluded from active context.
- **`deprecated`**: Page is fully replaced. Must specify `superseded_by: "new-page.md"` in front matter. Excluded from active context.

## Shared Editing & Writing Checklist

When writing or modifying any wiki page, always follow this checklist:
1. **Durable over ephemeral:** Write facts that survive the next 10 commits, not descriptions of specific line numbers or temporary variables.
2. **One concern per file:** Split files when a page starts covering two or more unrelated subsystems.
3. **Grep-friendly headings:** Use terms that appear in the source code so `wiki-engine search` returns useful results.
4. **Link, don't duplicate:** If a fact already lives in `repo-map.md`, reference it rather than repeating it.
5. **Format correctness:** Never use HTML links (`<a>`), bare URLs, or wiki-links (`[[links]]`). Use only standard relative markdown links.
6. **Required Metadata:** Ensure every file starts with a valid YAML front matter:
   ```yaml
   ---
   status: current
   description: "One-line summary of this page."
   ---
   ```

## Progressive Disclosure & Summaries

When the wiki is large, use **progressive disclosure** to manage token usage and avoid reading every file:
- **Active snapshot with summaries:** Run `wiki-engine context --active --summarize` (if `context_summarize = true` is set in `.wikirc`). This outputs each page's first heading, first paragraph, and line count.
- **Progressive reading:**
  - If a page has `line_count` ≤ 50, read it directly if the summary suggests relevance.
  - If a page has `line_count` > 50, first preview it with `wiki-engine summary <page>` before committing to a full read.
- Use `wiki-engine relevant <query>` to search for semantically or topologically relevant pages before reading.

## Cold-start checklist

When `wiki/log.md` has no prior entries:

1. Run `wiki-engine candidates` before assuming there's nothing to do.
2. Check for external knowledge files outside `wiki/`: `docs/`, `AGENTS.md`, `CLAUDE.md`, `CONTRIBUTING.md`, `ARCHITECTURE.md`.
3. Fill in `wiki/repo-map.md` completely — no placeholder comments.
4. Create at least one topic page before closing the session.
5. Mark `phases.md` Phase 1 and Phase 2 as completed.

## External docs migration

If the repo has existing docs outside `wiki/` that contain durable knowledge:
- Move content to `wiki/<name>.md`.
- Replace the original file with a stub: `> This file has moved to [wiki/<name>.md](wiki/<name>.md)`.
- Log the migration in `wiki/log.md`.
- Update any references in `README.md` to point to the new wiki location.
