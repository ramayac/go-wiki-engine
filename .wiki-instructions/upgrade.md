---
description: "Upgrade the wiki-engine CLI binary, reconcile all prompt layers, and verify wiki integrity."
name: "Wiki Upgrade"
argument-hint: "none"
agent: "agent"
---

Upgrade the wiki-engine CLI and reconcile all instructions, then verify wiki integrity, lint health, and citations.

## Required context

- Start from active context: run `wiki-engine context --active`.
- Respect each page's `lifecycle` (ignore `planned` as current fact; follow `superseded_by` links).

## Execution steps

1. Run `wiki-engine upgrade` (fail-closed checksum and provenance verification; pass `--from-source` only if explicitly building from the release tag via Go).
2. Run `wiki-engine init` (or `wiki-engine sync-prompts`) in the repository root to idempotently reconcile prompt files, symlinks, and root shims without overwriting user content.
3. Verify structure and citations:
   ```bash
   wiki-engine lint --json
   wiki-engine check --json
   ```
4. If `lint` reports legacy frontmatter or mechanical issues, run `wiki-engine lint --fix` (or `/wiki-lint`) and ensure changed pages include exact `sources:`.
5. Confirm the active wiki graph is healthy: `wiki-engine graph --strict` and `wiki-engine context --active`.

Finish by summarizing the new version, verification status, reconciled prompts, and confirming lint, check, and graph are healthy.
