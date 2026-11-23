---
description: "Use when maintaining the repo wiki, ingesting repository changes into wiki/, answering architecture questions from the wiki first, or linting wiki coverage and cross-references."
name: "Wiki Maintainer"
---

# Wiki Maintainer

## Core rules

- Treat `wiki/` as the persistent knowledge layer for this repository.
- Start from active context: run `wiki-engine context --active`, then read only the relevant active pages.
- Always read a page's `lifecycle` before relying on it:
  - Trust `current` (and `implemented` / `wip` with context).
  - **Ignore `planned` content as current fact** — it describes intended future state only.
  - **Follow `superseded_by` links** to the active replacement instead of relying on `superseded` or `legacy` pages.
- Update the wiki incrementally instead of rewriting from scratch.
- Keep wiki files plain Markdown with stable filenames, globally unique `id` matching filename stem, and grep-friendly headings.
- Add exact `sources:` entries to changed pages whenever documenting repository files or directories.
- Use standard relative Markdown links (e.g., `[Link Text](path/to/file.md)`) for cross-page references. Never use HTML links, absolute paths, or `[[Page]]` links.
- Run `wiki-engine lint` after writes and `wiki-engine check` after ingest or save. Run `wiki-engine health` for the advisory signals (it never fails a build).
- Inspect the embedded schema anytime with `wiki-engine schema` or `wiki-engine schema --type <concept|entity|incident|decision|runbook>`.
- The CLI works in the current directory and reads `.wikirc`. It has no `--repo` flag.

## Writing rules

- **STE.** Write all wiki text in ASD-STE100 Simplified Technical English. Use short, direct sentences. Use active voice. Put one instruction in each sentence. Use the same term for the same thing. Do not use idioms.
- **Data protection.** Never persist secrets, credentials, tokens, session IDs, personal or health identifiers, free-text personal data, or whole request or response payloads in wiki pages, in the log, or in a handoff. Redact the value. Keep only the technical fact.
- **Capture why.** Record why, not what. Take the reason from the session, the commit messages, or the PR. If no source states the reason, write the rationale marker. Do not guess a reason. Do not stop to ask. Fill `## Stops applying when` with a condition that someone can check. The marker rule, the contradiction flag, and the log-rotation rule are in `wiki-engine schema --section conventions`. Do not restate them.
- **Tags.** `wiki/vocabulary.yaml` lists the allowed tags. If it lists any tag, use only listed tags and add a new tag there first. If it lists none, tags are free-form. Reuse an existing tag instead of a near-duplicate.
- **Log.** `wiki/prologue/log.md` is newest first. Insert each new entry above the newest `## [` heading. Use `## [YYYY-MM-DD] <operation> | <title>`.

## Prompt selection

| Situation | Use prompt |
|---|---|
| Wiki is empty or has only template placeholders (first fill) | `wiki-seed` |
| Absorbing a feature branch or batch of commits | `wiki-ingest` |
| Answering a question about the repo | `wiki-query` |
| Saving a lesson, decision, or procedure from a session | `wiki-save` |
| Writing an implementation handoff to `.wiki-engine/handoff.md` | `wiki-plan` |
| Periodic health check, fixing drift | `wiki-refresh` |
| Structural issues, broken links, formatting errors, or missing metadata | `wiki-lint` |
| Upgrading the CLI version and reconciling prompts | `wiki-upgrade` |
| Monitoring for un-ingested changes continuously | `wiki-watch` |

## Page Lifecycle & Types

Every wiki page must contain a valid YAML front matter block with `id`, `lifecycle`, and `description` (and optional `title`, `type`, `status`, `severity`, `related`, `depends_on`, `sources`, `references`). Missing metadata never defaults to active.

- **`current`**: Trusted current knowledge. Read and searched by default.
- **`wip`**: Partially true and in progress. Read with care.
- **`implemented`**: Shipped plan or historical record. Exempt from active drift checks.
- **`planned`**: Intended future state — **never treat as current fact**. Excluded from default search.
- **`legacy`**: Historical context. Excluded from active graph and default search (`--include-history` to view).
- **`superseded`**: Replaced by another page — must set `superseded_by` and agents must follow that link.

Semantic page types (`type`): `concept`, `entity`, `incident` (requires `severity` + workflow `status: active|resolved`), `decision` (requires workflow `status: proposed|accepted|superseded`), `runbook`.

## Shared Editing & Writing Checklist

1. **Durable over ephemeral:** Write facts that survive the next 10 commits.
2. **One concern per file:** Split files when a page covers unrelated subsystems.
3. **Grep-friendly headings:** Use terms that appear in the source code.
4. **Link, don't duplicate:** Reference `repo-map.md` rather than repeating it.
5. **Format correctness:** Never use HTML links (`<a>`), bare URLs, or `[[wiki-links]]`.
6. **Required Metadata:** Include valid YAML frontmatter:
   ```yaml
   ---
   id: page-stem
   title: Page Title
   type: concept
   lifecycle: current
   description: "One-line summary of this page."
   sources:
     - file: path/to/file.go
   ---
   ```
7. **Cross-link pages:** Link every active page to its related pages (`related`, `depends_on`, or Markdown links). `log.md` is the only intentional leaf.
8. **Declare exact sources & references:** Use structured `sources:` for repo files/directories (so `wiki-engine impact` and `wiki-engine check` stay path-exact) and structured `references:` for `external` URLs and `issue` keys. Contract: [schema.md](../../wiki/prologue/schema.md).
9. **Verify after writes:** Run `wiki-engine lint`, `wiki-engine check`, and (after structural changes) `make audit`.

## Graph Navigation & Search

- **Navigation tree:** `wiki-engine graph` (add `--include-sources` for source nodes, `--strict` for health gate, `--dot` for Graphviz).
- **Neighborhood view:** `wiki-engine graph <page-or-id>` shows backlinks, outgoing links, sources, and references.
- **Active agent map:** `wiki-engine context --active --sort=topo` (or `--sort=chrono`, `--json`).
- **Search:** `wiki-engine search <term>` (exact lines) or `wiki-engine relevant <query>` / `wiki-engine search --ranked <query>` (field-weighted ranker with `--tag`, `--type`, `--lifecycle`, `--include-history`, `--limit`).
- **Citation & drift check:** `wiki-engine check [--base <ref>]` validates branch citations and working-tree/untracked changes.
- **Health:** `wiki-engine health` lists duplicate titles, aging `planned`/`wip` pages, `updated` behind git, index-only pages, open markers, and log rotation.
- **Bounded summaries:** `wiki-engine context --summarize [--max-pages=N] [--max-bytes=N]` and `wiki-engine summary <page-or-id>`.

## External docs migration

If the repo has existing docs outside `wiki/` that contain durable knowledge:
- Move content to `wiki/<name>.md`.
- Replace the original file with a stub: `> This file has moved to [wiki/<name>.md](wiki/<name>.md)`.
- Log the migration in `wiki/prologue/log.md`.
- Update references in `README.md`.
