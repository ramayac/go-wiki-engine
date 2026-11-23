---
description: "Run the wiki linter and automatically fix structural errors, broken links, formatting issues, or missing metadata."
name: "Wiki Lint"
argument-hint: "none"
agent: "agent"
---

Interpret `wiki-engine lint` and `wiki-engine check` output and fix all issues automatically.

## Required context

- Run `wiki-engine context --active` to get the current active wiki snapshot.
- Run `wiki-engine lint --json` and `wiki-engine check --json` to get all structural and citation issues.

## Execution steps

1. Run `wiki-engine lint --fix` first to apply deterministic mechanical fixes (legacy `status` → `lifecycle`, inferred `id`/`title`, resolvable `[[wiki links]]`, lifecycle banners, index badges, section ordering, and inline `references` → structured `sources`/`references`).
2. Run `wiki-engine lint --json` and `wiki-engine check --json`.
3. If both outputs show no issues, report that the wiki is clean and exit.
4. For each remaining issue, fix the root cause:
   - **`front-matter` / `page-ids` / `page-types` / `relationships`**: Ensure valid YAML frontmatter, unique `id` matching filename stem, explicit `lifecycle`, required workflow `status`/`severity` on `decision`/`incident` pages, valid `superseded_by` links, and valid `related`/`depends_on` IDs.
   - **`sources` / `references` / `check`**: Ensure `sources:` paths exist (or carry `repo:` for cross-repo citations) and update stale citations after renames or deletions.
   - **`index-format` / `cross-page-links` / `orphans` / `leaf-pages`**: Keep standard relative Markdown links strictly contained inside `wiki/` and cross-link active pages (`log.md` is the only intentional leaf).
   - **`path-containment`**: Remove any link or source escaping the repo/wiki boundary.
   - **`heading-hierarchy` / `log-headings` / `log-chronology` / `markers` / `duplicate-content` / `stale-content`**: Resolve formatting and freshness warnings.
5. Re-run `wiki-engine lint`, `wiki-engine check`, and `wiki-engine graph --strict` to confirm everything passes cleanly.
6. If pages moved or renamed, run `make audit` to verify repo-wide reference parity.
7. Summarize which issues were identified, what changed, and confirm all gates pass.
