---
description: "Build a compact implementation handoff from repository and wiki evidence. Use when: planning a change, or before you clear or compact a long conversation."
name: "Wiki Plan"
argument-hint: "Implementation goal..."
agent: "agent"
---

Write a compact implementation plan that stays useful after the conversation is compacted. Follow [wiki-maintainer.md](wiki-maintainer.md).

## Steps

1. Treat the user's argument as the implementation goal. If it is empty, ask for the goal.
2. Run `wiki-engine search "<goal terms>"`. If no result is useful, retry once with other terms. Use `--include-history` only for historical or planned context.
3. For each good result, run `wiki-engine graph <id>`. If the goal names repo files, run `wiki-engine impact <file>` for each file. Record a file that no page cites as an open question. Read only the relevant `current` and `wip` pages, and the code and tests that control the behaviour.
4. Resolve important ambiguity before you write the plan. Do not implement the change in this command.
5. Write `.wiki-engine/handoff.md` with these headings: Goal, Non-goals, Assumptions, Decisions, Relevant wiki evidence, Code anchors, Implementation steps, Validation, and Open questions. Keep it short. Link the exact wiki pages and repo files. Include the commands that validate the change. Write in STE.
6. Obey the data-protection rule. Never write secrets, tokens, personal data, or whole payloads in the handoff.
7. Tell the user: after the conversation is cleared or compacted, the implementation agent reads `.wiki-engine/handoff.md` first. The file is temporary. Delete it when the work is complete.

This command writes only `.wiki-engine/handoff.md`. The `.wikirc` `ignore` list and `.gitignore` exclude that folder. Do not change the wiki or the application code.
