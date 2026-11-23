---
description: "Answer a repository question from the wiki first. Use when: querying architecture, repo workflows, exclusions, or durable decisions before scanning source files."
name: "Wiki Query"
argument-hint: "Ask a repository question..."
agent: "agent"
---

Answer the user's repository question from the wiki first. Follow [wiki-maintainer.md](wiki-maintainer.md).

## Steps

1. Treat the user's argument as the question.
2. Locate first. Run `wiki-engine search "<terms>"` before you read any page. The default search returns only `current`, `implemented`, and `wip` pages. For a historical or future-state question, add `--include-history`. Refine with `--keyword`, `--tag`, `--type`, or `--limit`. If no result is useful, retry once with other domain terms before you say that the wiki has no coverage. Use `wiki-engine relevant <query>` for a ranked list.
3. If the question names a repo file, run `wiki-engine impact <file>`. It lists the pages that cite the file. If no page cites it, say so. That is a gap in the wiki.
4. Read `wiki/index.md` only if the question is broad or search finds nothing.
5. When relationships matter, run `wiki-engine graph <id>` for each good result. Run the full `wiki-engine graph` only if no result gives an anchor.
6. Read the best pages. Answer with Markdown links to the pages. If the answer contains documentation text, write it in STE. Check each page's `lifecycle`. Trust `current` pages as today's truth. Treat `implemented` pages as a record of shipped work, and check the code for current behaviour. Treat `planned` and `wip` as not yet real. Treat `legacy` and `superseded` as history. For a `superseded` page, follow `superseded_by` to the live page.
7. If the answer uses a page that is not `current`, say so.
8. Use source files, starting with the `sources:` of the pages, only if the wiki lacks the evidence.
9. If the wiki does not cover the question, say so. Suggest the Wiki Save prompt for the facts that you learned. If you change a page, run `wiki-engine lint` and `wiki-engine check`.

In the answer: answer directly, state whether the wiki alone was enough or you used source files, and list any wiki update.
