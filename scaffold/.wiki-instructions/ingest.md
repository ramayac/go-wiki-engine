---
description: "Update the wiki from a code diff. Use when: ingesting a feature branch, filing architecture updates, or converting diffs into durable wiki knowledge."
name: "Wiki Ingest"
argument-hint: "Optional diff file, git ref or range, or focus area..."
agent: "agent"
---

Ingest a code change into the wiki.

> **Cold start:** If `wiki/prologue/log.md` has no `ingest` entries AND the wiki has no real pages (only the scaffold pages and placeholders), do not continue. Run the **Wiki Seed** prompt instead.

## Inputs

The user's argument resolves to one diff:

- A path to a diff file, or
- A git ref or range (for example `HEAD`, `main..HEAD`, a commit hash), or
- No argument: `git diff --cached`. If nothing is staged, use `git diff`.

If you cannot resolve the diff, ask the user.

The diff shows what changed. The reason is in the commit messages and the PR description. Read them:

- For a range, run `git log --format=%B <range>`. For one commit, run `git log -1 --format=%B <ref>`. Staged and working-tree changes have no messages yet.
- If the diff is the work of the current branch, `gh` is available, and the branch has a PR, run `gh pr view --json body -q .body`. For any other diff, do not run it: it reads the PR of the current branch.
- If `gh` is missing, there is no PR, or the command fails, use the commit messages only. Do not stop to ask.

## Rules

- Follow [wiki-maintainer.md](wiki-maintainer.md). Write in STE. Obey the data-protection rule.
- Edit only pages whose subject the diff changes. Do not change `created:` on an existing page. Change only `updated:`.
- No-op: a typo, comment-only, or formatting diff changes no wiki concept. Say so. Write nothing. Add no log entry.
- Flag a contradiction with the contradiction flag. Do not overwrite it silently.
- Lifecycle: if the change makes a page obsolete, do not delete it. Set `lifecycle: legacy`, or `superseded` with `superseded_by`. If the change ships `planned` or `wip` work, promote the `lifecycle` (`current`, or `implemented` for a plan or decision page). Then fix the banner and the index badge by hand. `lint --fix` adds them but never removes them.

## Steps

1. Resolve the diff (see Inputs). Run `wiki-engine schema` and `wiki-engine context --active`. Read `wiki/vocabulary.yaml`. Read `wiki/index.md` only if you add a page or change a lifecycle.
2. Find the first candidate pages.
   - Diff is the current branch (staged, working tree, or a range that ends at `HEAD`): run `wiki-engine check`. Its "pages to review" list names the pages that cite a changed file.
   - Any other diff: run `wiki-engine impact <files>` with the changed paths. If no page cites a file, note the path for the summary.
   - Run `wiki-engine changed` or `wiki-engine candidates` (add a range or `--base=<ref>`) to list the ingest-worthy files.
3. Search for pages that citations cannot show. Run `wiki-engine search "<key terms or symbols>" --include-history --limit 20`. If no result is useful, retry once with a shorter exact phrase and `--keyword`. For each good result, run `wiki-engine graph <id>` to see its neighbours. Run the full `wiki-engine graph` only if no result gives an anchor.
4. Read the diff and pick the affected pages. If none, stop and report a no-op.
5. Read each candidate page in full before you edit it.
6. Edit the pages in place. Set `updated: "YYYY-MM-DD"` (quoted). Cite the PR number, or a commit that is on the base branch. A squash merge removes feature-branch commits.
   - Record why, not what. Take the reason from the commit messages and the PR description. If no source states the reason, write the rationale marker (see `wiki-engine schema --section conventions`). Do not guess.
   - If a source now states the reason for a page that has the marker, replace the marker line with the reason.
   - If the change makes a `## Stops applying when` condition true, update or retire that page.
7. Create a page for each new concept, entity, decision, incident, or runbook. Follow [save.md](save.md) for the template, `lifecycle`, `sources:`, and cross-links.
8. Update `wiki/index.md` for each new or relabelled page. Add the entry only. `lint --fix` orders the sections and adds the badges.
9. Insert ONE `ingest` entry above the newest `## [` heading in `wiki/prologue/log.md`. The log is newest first. Use the heading `## [YYYY-MM-DD] ingest | <title>`.
10. Run `wiki-engine lint --fix`, then `wiki-engine lint`. Fix by hand what remains.
11. Run `wiki-engine check`. Fix each broken citation, then run it again. For each page in "pages to review", update it or say in the summary why it needs no change. For an unmerged-commit warning, cite the PR number.
12. Run `wiki-engine health`. Report each signal: duplicate title, aging work, `updated` behind git, index-only page, open marker, and log rotation. If the log needs rotation, follow the log-rotation rule in `wiki-engine schema --section conventions`.
13. Summarize: the pages touched, the changed files that no page cites, the review pages that needed no change (with the reason), and the final lint, check, and health results. List each rationale marker that you wrote, with its page. Tell the user to reply with the reason to fill it in. Do not wait for a reply.
