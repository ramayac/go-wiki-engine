---
id: audit-v2-fixes
title: Audit v2 Fixes
type: decision
status: accepted
lifecycle: current
description: "Intentional contract changes from the second audit fix PR, and the limits left in place."
sources:
  - file: cmd/wiki-engine/main.go
  - file: internal/config/config.go
  - file: internal/git/refs.go
  - file: internal/scaffold/scaffold.go
  - file: internal/upgrade/upgrade.go
related: [parity-improvement-plan]
---
# Audit v2 Fixes

## Decision

The audit-fix PR closes the findings of the second comparison audit. It changes behavior in the ways below. Each change is intentional and is listed in [CHANGELOG.md](../../CHANGELOG.md).

## Contract changes

- **Usage errors exit 2.** Unknown command or flag, missing or extra argument, and conflicting flags exit `2`. Runtime failures stay at `1`. Scripts can now tell a bad call from a failed run.
- **A `wiki_dir` that starts with `-` is rejected** in `init` and in `.wikirc`. A leading dash could reach git or the shell as an option.
- **Edited managed files are backed up.** `sync-prompts` and `init` copy a hand-edited file to `<name>.bak` before they replace it with a symlink.
- **`sync-prompts` deletes only engine-owned files.** These are symlinks into `.wiki-instructions/` and files named in `retiredManagedFiles`. User files stay.
- **Unsigned provenance is no longer "verified".** Only `gh attestation verify` verifies. A release-served `.intoto.jsonl` file proves nothing.
- **`upgrade` refuses a downgrade.** It also validates the release tag before any download.
- **The default diff detects the base branch.** `main...HEAD` works on a repo that has `master` and no `main`.
- **`ignore` patterns match by path.** A pattern with `/` matches the full path. A bare name matches any path segment.
- **Git 2.24 or newer is required.** Git calls use `--end-of-options` where a user value reaches them.

## Reason

Several findings let a config value or an argument reach git as an option, write outside the repo through a symlink, or delete a user file. The fixes close these paths. The exit-code and ownership changes make the safe behavior visible to scripts.

## Consequences

Scripts that test for exit code `1` on a usage error must test for `2`. A repo that relied on `sync-prompts` to delete its own `wiki-*` files must remove them by hand.

### Known limits

The PR leaves these behaviors unchanged on purpose.

- A non-numeric `--limit`, `--max-pages`, or `log-tail` value still falls back to the default.
- `init --wiki-dir=X` against a `.wikirc` that names another directory, with no existing wiki directory, leaves `.wikirc` stale.
- The downgrade guard compares pre-release versions of the same `x.y.z` as equal.
- The tag pattern rejects the legacy tag `v.0.5.0`.
- A checksum-only upgrade proceeds when `gh` or attestations are unavailable, unless `--require-attestation` is set.
- `rev-parse`, `merge-base`, `ls-tree`, and `cat-file` use a leading-dash guard and `--`, not `--end-of-options`.

## Related

- [parity-improvement-plan.md](parity-improvement-plan.md) - the plan that introduced the surfaces these fixes harden.
- [log.md](../prologue/log.md) - timeline entry for the fix PR.
