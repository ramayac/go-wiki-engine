---
description: "Upgrade the wiki-engine CLI binary, reconcile all prompt layers, migrate the wiki to the current schema, and verify wiki integrity."
name: "Wiki Upgrade"
argument-hint: "none"
agent: "agent"
---

Upgrade the wiki-engine CLI. Reconcile the prompts. Migrate the wiki to the current schema. This prompt is idempotent. Follow [wiki-maintainer.md](wiki-maintainer.md).

## Steps

1. Run `wiki-engine upgrade`. It verifies the checksum and the provenance, and it refuses a downgrade. Add `--from-source` only to build from the release tag with Go.
2. Run `wiki-engine init` (or `wiki-engine sync-prompts`) in the repo root. It reconciles the prompt files, the symlinks, and the root shims. It does not overwrite user content.
3. Run `wiki-engine schema` and `wiki-engine context --active`. Read `wiki/vocabulary.yaml` and `wiki/index.md`.
4. Backfill `lifecycle` on each page that lacks it. Default to `current`. Choose another value only when the page clearly shows it (unbuilt work, or replaced behaviour). List each page that you propose to mark non-`current`, with the reason. Ask the user to confirm before you apply it. For `superseded`, set `superseded_by` to an existing page id.
5. Run `wiki-engine lint --fix`. It migrates legacy `status`, adds banners and index badges, rewrites `[[id]]` links, and orders the index sections. Do not do these by hand.
6. Run `wiki-engine lint`, and fix by hand what remains: a `[[id]]` link with no target page, a tag that is not in `wiki/vocabulary.yaml`, and any other error.
7. Run `wiki-engine check`, `wiki-engine graph --strict`, and `wiki-engine health`.
8. Insert one `update` entry above the newest `## [` heading in `wiki/prologue/log.md`. The log is newest first. Use the heading `## [YYYY-MM-DD] update | <title>`. Summarize the migration. Write in STE.
9. Summarize: the new version, the reconciled prompts, the pages that you backfilled, the pages marked non-`current` (with the reason), the fix counts that `lint --fix` printed, and the final lint, check, and graph status.
