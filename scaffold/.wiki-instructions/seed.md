---
description: "Fill an empty or thin wiki from the repository's code and docs in one pass, grouped by domain. Use when: the wiki is empty or has only template placeholders, there is no ingest history, or part of the repo has no page. NOT for incremental updates - use Wiki Ingest for that."
name: "Wiki Seed"
argument-hint: "Optional subdirectory or focus word; empty for the whole repo..."
agent: "agent"
---

Fill the wiki from the repository in one pass. Run this once after `wiki-engine init`. Run it again for a part of the repo that has no page.

## Scope

The user's argument narrows the pass:

- A subdirectory: inventory only that tree.
- A focus word: inventory the parts of the repo that implement it.
- No argument: inventory the whole repository.

## Rules

- Follow [wiki-maintainer.md](wiki-maintainer.md). Write in STE. Obey the data-protection rule.
- Create pages. Do not edit an existing page. Report an overlap instead.
- Three files are the exception. Fill `wiki/prologue/repo-map.md` and `wiki/prologue/phases.md` when they still hold template placeholders. Edit `wiki/index.md` to add entries.
- Do not change repository code. Record findings. Do not fix them.
- Tags: follow the vocabulary rule in the maintainer guidelines.
- If every domain in the inventory already has a page, say so, list the pages, and write nothing.

## Steps

1. Run `wiki-engine schema`, `wiki-engine context --active`, and `wiki-engine log-tail`. Read `wiki/index.md` and `wiki/vocabulary.yaml`.
2. Inventory the scope. Read these items in order:
   - the root README, and `docs/`, `ARCHITECTURE.md`, `CONTRIBUTING.md`, `AGENTS.md`, `CLAUDE.md` when they exist;
   - manifests (`go.mod`, `package.json`, `pyproject.toml`, `pom.xml`, `Makefile`);
   - entrypoints, route or handler registrations, CI workflows, infrastructure files, and the test layout;
   - ADR and postmortem folders.

   Read the code at the boundaries: entrypoints, integrations, data access. Never infer behaviour from a name alone. Run `wiki-engine candidates` for a quick list of source files.
3. Group the inventory into domains. Use these signals in order. Stop at the first clean split: top-level folders; deploy units; shared name prefixes; shared data stores, events, or APIs. Aim for one page per cohesive area. For most repos that is five to twelve pages. State the grouping and the reason.
4. Search before you plan. For each domain, run `wiki-engine search "<terms>" --include-history --limit 10`. For each key file, run `wiki-engine impact <file>`. It lists the pages that cite the file. Drop a domain that a page already covers. List it as an overlap.
5. Show a table: domain, page `id` and `type`, source files, reason. Add the findings you expect. Ask the user to confirm the grouping. Do not write until the user answers.
6. Write the pages. Run `wiki-engine schema --type <type>` for each template.
   - Write one `concept` per domain. Add a Mermaid diagram when a picture is clearer than text.
   - Write one `entity` per component, service, data store, or integration. Name the entities it needs in `depends_on`.
   - Write a `runbook` for each documented procedure (setup, test, deploy, release). Give each step a `**Verify:**` command.
   - Write a `decision` for each ADR and an `incident` for each postmortem. Keep the dates and status.
   - Put each page in a folder by type or domain (for example `wiki/architectures/`, `wiki/entities/`, `wiki/operations/`).
   - Fill `sources:` with the exact repo paths. Link the same files in the text.
   - Take the reason for a design from the README, `docs/`, ADRs, and code comments. Write it in `## Why`. If no source states the reason, leave out `## Why`. Do not write the rationale marker. Do not guess. A seed has no session, commit, or PR, so a marker on every page gives no information.
   - Set `lifecycle: current`, unless the source says the work is planned, in progress, or replaced.
   - Quote the dates (`created: "YYYY-MM-DD"`). Make `id` match the file name without `.md`.
7. Record each finding as one line in `wiki/architectures/gotchas.md`: `- **[OPEN]** <one sentence>. Evidence: [path](../../path).` Findings are code that no page explains, a producer with no consumer, a missing retry path, a resource outside the infrastructure code, or a ticket id in a comment. Create the page from the `concept` template if it does not exist.
8. Fill `wiki/prologue/repo-map.md`: Purpose, High-Signal Areas, Generated Artifacts, Build and Run Path, and Ignored Paths. Set the phases in `wiki/prologue/phases.md` to match the work done. Link the new pages from `wiki/index.md`, one entry for each page.
9. Insert one `seed` entry above the newest `## [` heading in `wiki/prologue/log.md`. The log is newest first. Use the heading `## [YYYY-MM-DD] seed | <title>`.
10. Run `wiki-engine lint --fix`, then `wiki-engine lint`. Fix by hand what remains.
11. Run `wiki-engine check` and `wiki-engine health`. Then run `wiki-engine graph --strict`. For each inventoried key file, run `wiki-engine impact <file>`. List the key files that no page cites.
12. Summarize: the grouping, the pages written, the overlaps skipped, the findings recorded, the key files that no page cites, and the final lint, check, and health results.
