---
description: "Save a conversation summary or lessons learned into the wiki. Use when: a session produced a durable fact, a decision, an incident lesson, or a procedure that no page records yet."
name: "Wiki Save"
argument-hint: "Summary and lessons to save..."
agent: "agent"
---

Save new knowledge from this session into the wiki.

## Rules

- Follow [wiki-maintainer.md](wiki-maintainer.md). Write in STE. Obey the data-protection rule.
- Treat the user's argument as the note to save. If it is empty, ask the user for the note.
- Prefer an update to an existing page over a new page. Do not create a near-duplicate.
- Pick the granularity of a domain, not a file: one page per cohesive area.

## Steps

1. Run `wiki-engine schema` and `wiki-engine context --active`. Read `wiki/vocabulary.yaml`. Read `wiki/index.md` only if you add a page or change a lifecycle.
2. Find existing pages. Run `wiki-engine search "<key terms from the note>" --include-history --limit 20`. If no result is useful, retry once with a shorter exact phrase and `--keyword`. For each good result, run `wiki-engine graph <id>` to see its neighbours. Run the full `wiki-engine graph` only if no result gives an anchor. If related pages are not linked, add links in both directions.
3. Classify the note: `concept`, `entity`, `incident`, `decision`, or `runbook`. Use `entity` for a thing (a component, service, data store, or integration). List the entities it needs in `depends_on`. Pick a kebab-case `id`.
4. Write the page in a folder by type or domain (for example `wiki/architectures/`, `wiki/entities/`, `wiki/decisions/`, `wiki/incidents/`, `wiki/operations/`). Run `wiki-engine schema --type <type>` for the front matter and the template.
   - Quote the dates (`created: "YYYY-MM-DD"`). Make `id` match the file name without `.md`.
   - Tags: follow the vocabulary rule in the maintainer guidelines.
   - Set `lifecycle`: `current` for present-state truth (the usual case); `implemented` for a plan or decision whose work has shipped; `wip` for work in progress; `planned` for work not yet built; `legacy` for kept history; `superseded` when another page replaces this one. For `superseded`, set `superseded_by`. Do not write the banner by hand. Step 7 adds it.
   - List the repo files that the page describes in `sources:` (exact repo paths). Link them under `## References`. A `current` concept or runbook with no repo file gets a lint warning.
   - To cite a change, use its PR number or a commit that is on the base branch. A squash merge removes feature-branch commits.
   - Record why, not what. Do not restate what the code shows. Link the code. Take the reason from this session: why the team made the choice, and which options it rejected. Write it in `## Why` (concept or entity), `## Decision` (decision), or `## Fix` (incident). A runbook needs no reason. If the session does not state the reason, write the rationale marker (see `wiki-engine schema --section conventions`). Do not guess. Do not stop to ask.
   - If the note gives the reason for a page that has the marker, replace the marker line with the reason. Do not keep both. This also applies when the user replies to the summary in step 10.
   - Fill `## Stops applying when` on every page type except runbook. Write a condition that someone can check: a version, a config value, a metric, or a process. If you do not know it, write "Not known."
   - If the note contradicts an existing page, do not overwrite it. Flag the conflict with the contradiction flag on that page.
5. Update `wiki/index.md` with the new entry. Add the entry only. `lint --fix` orders the sections and adds the badges.
6. Insert one `capture` entry above the newest `## [` heading in `wiki/prologue/log.md`. The log is newest first. Use the heading `## [YYYY-MM-DD] capture | <title>`.
7. Run `wiki-engine lint --fix`, then `wiki-engine lint`. Fix by hand what remains.
8. Run `wiki-engine check`. Fix each broken citation, then run it again. For each page in "pages to review", update it or say in the summary why it needs no change. For an unmerged-commit warning, cite the PR number.
9. Run `wiki-engine health`. Report each signal: duplicate title, aging work, `updated` behind git, index-only page, open marker, and log rotation. If the log needs rotation, follow the log-rotation rule in `wiki-engine schema --section conventions`.
10. Summarize: what you wrote, the pages you touched, and the final check result. List each rationale marker that you wrote, with its page. Tell the user to reply with the reason to fill it in. Do not wait for a reply.
