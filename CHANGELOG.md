# Changelog

All notable changes to go-wiki-engine are documented here. Format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and releases follow
[Semantic Versioning](https://semver.org/spec/v2.0.0.html); the compatibility
surface is defined in [README.md](README.md).

## [Unreleased]

### Added

- **Strict path containment (`internal/paths`):** Single-segment `wiki_dir` validation and symlink-aware boundary enforcement across `.wikirc`, `init`, `sync-prompts`, `summary`, graph traversal, and source resolution.
- **Real YAML frontmatter & typed wiki model (`internal/model`, `internal/schema`):** Replaced hand-written frontmatter parser with `gopkg.in/yaml.v3`, preserving line numbers and unknown fields with zero implicit active defaults; added stable page `id`, semantic `type` (`concept`, `entity`, `incident`, `decision`, `runbook`), separate authority `lifecycle` (`current`, `wip`, `implemented`, `planned`, `legacy`, `superseded`) vs decision/incident workflow `status`, `related`/`depends_on` relationships, and structured `sources`/`references`.
- **Single-read lint architecture & mechanical fixes (`internal/lint`):** Every lint run loads one consistent `WikiSnapshot` (reading each file once); added grouped selectors (`schema`, `links`, `graph`, `freshness`, `security`) and idempotent `wiki-engine lint --fix`.
- **Unified Git pipeline & `wiki-engine check` (`internal/git`):** Merge-base discovery with rename/delete tracking, working-tree and untracked changes (`--no-working-tree` for reproducible CI), and branch-aware citation/drift checks (exit 0 ok/review, 1 breakage, 2 setup error).
- **Embedded schema CLI (`wiki-engine schema`):** Prints authoritative schema, sections, and ready-to-use templates (`--type`, `--section`, `--json`).
- **Idempotent bootstrap & rename (`internal/scaffold`):** Re-running `wiki-engine init` reconciles prompts, symlinks, ignore entries, and versioned root shims while preserving user content; `wiki-engine init --wiki-dir <name>` safely renames a wiki directory.
- **Supply-chain hardening:** All GitHub Actions pinned by immutable 40-char commit SHA (enforced by `internal/audit`), minimal workflow permissions, SLSA build-provenance attestations in `release.yml`, per-platform `.sha256` files, `govulncheck`, and Dependency Review in CI.
- **Upgraded search, graph, and bounded context:** Field-weighted search ranker (`search --ranked` / `relevant`) with `--tag`, `--type`, `--lifecycle`, `--include-history`, `--limit`, `--keyword`; graph JSON `schema_version: "2"`, ID lookup, `related`/`depends_on` edges, and opt-in `--include-sources`; bounded `context` with `--max-pages`/`--max-bytes` truncation reporting.
- New `graph` command: node-based navigation map of the active wiki graph. `wiki-engine graph` prints an ASCII tree from `index.md` (diamonds and cycles render as `↰ (see above)` markers), `graph <page>` shows one page's backlinks, outgoing links, and declared references, `--json` emits structured nodes/edges/unlinked/stats/issues (with `<page>`: neighborhood JSON), `--dot` exports Graphviz DOT, and `--strict` exits non-zero when active pages are unreachable from `index.md` or the graph has issues (duplicate edges, self-loops, broken links) — in every mode.
- Graph health diagnostics: broken links are now surfaced as issues instead of being silently dropped by BFS traversal.
- Prompt layers document `graph` usage (wiki-maintainer, query, pi skill).
- New `references` front matter field: typed cross-references (`source:<path>`, `external:<url>`, `issue:<KEY>`) declared in a single-line bracket list. References are node annotations, not graph edges — they surface in `wiki-engine graph <page>` and graph JSON.
- New `references` lint checker: flags missing source files, scheme-less URLs, malformed issue keys, and unknown reference types.

### Fixed

- **Git option injection:** `changed`, `candidates`, `refresh`, `watch --once`, and the `stale-content` check passed the diff range to `git diff` unchecked, so `--output=<path>` could overwrite a file. Ranges now must be revisions or `a..b` / `a...b` of existing revisions, `default_diff` may not start with `-`, and git calls pass `--end-of-options` and `--` where a user or config value reaches them. A `wiki_dir` that starts with `-` is now rejected (see below). Git 2.24 or newer is required.
- **Symlink write-through in `init` and `sync-prompts`:** The commands wrote through a symlinked managed directory (`.wiki-instructions`, `.claude/commands`, `.github/prompts`, `.github/instructions`, `.pi/skills/wiki`) or a symlinked `AGENTS.md`, `CLAUDE.md`, or `.wikirc` and changed files outside the repo. Every write, mkdir, remove, and symlink now checks that the real destination stays inside the repo and fails with an error naming the path. A symlinked file inside a managed directory is still replaced, not followed.
- **`sync-prompts` deleted user files:** It removed any `wiki-*` file in the sync directories and anything under `.pi/skills/wiki/` and `.wiki-instructions/` that the engine did not ship. It now removes only symlinks that point into `.wiki-instructions/` and files named in `retiredManagedFiles`.
- **Silent overwrite of edited prompt files:** A hand-edited regular file that `sync-prompts` or `init` replaces with a tool-directory symlink is first copied to `<name>.bak` (then `.bak.1`, ...), with one line on stderr. Identical files are replaced silently.
- **`init` argument handling:** `init 'a$b'` wrote `wiki_dir = "a"` (the `$` was treated as a template reference); it now keeps the name. `wiki_dir` names that start with `-` are rejected in `init` and in `.wikirc`. `init <name>` in a repo whose `.wikirc` names a different wiki directory now fails with a message instead of leaving `.wikirc` stale.
- **Default diff on repos without `main`:** `main...HEAD` no longer fails on `master` (or other) repos; the engine detects the base branch like `check`. Git errors now show git's reason and tell "not a git repository" from "unknown revision".
- **Subdirectory projects:** `changed` and `candidates` return paths relative to `.wikirc`, unquoted for non-ASCII names. `candidates` no longer lists deleted files or the old path of a rename.
- **`ignore` patterns:** patterns with `/` now match the full path (`docs/*.md`, `**`), and bare names match any path segment.
- **Non-ASCII wiki paths and shallow clones:** `stale-content` now sees non-ASCII page names and skips (with one `info` issue) in a shallow clone. `check` reports a cited commit missing from a shallow clone as a warning (`unverifiable_commit`), not `invalid_commit`.

### Changed

- **`wiki-engine impact` is exact by default:** Matches declared `sources`, directory coverage, and exact Markdown repo links with explicit match reasons in JSON; basename prose matching is opt-in via `--fuzzy`.
- **`wiki-engine upgrade` is fail-closed:** Never silently falls back to `go install` on download, checksum, or provenance failure; streams archives to disk with compressed/expanded size caps and archive path-traversal checks; `go install` requires explicit `--from-source` pinned to the release tag.

## [1.0.1] - 2026-09-19

### Fixed

- `upgrade` now follows redirects when downloading release assets and `checksums.txt` — GitHub serves them via 302 redirects to signed storage URLs, so the checksum-verified download path was previously failing and silently falling back to `go install`.

## [1.0.0] - 2026-09-19

### Added

- `.golangci-lint.yml` (v2 config) and a `make golangci-lint` target; CI runs golangci-lint and `go test -race ./...`.
- Mocked-HTTP tests for the `wiki-engine upgrade` download path: success, checksum mismatch, no-asset fallback, latest-tag failure fallback, and extraction failure.
- User-facing docs: `CONTRIBUTING.md`, `SECURITY.md`, and the release runbook ([wiki/operations/release.md](wiki/operations/release.md)).
- `README.md` prerequisites and supported-platform sections, plus a versioning & compatibility policy.

### Changed

- CLI commands reject unknown flags instead of silently ignoring them.
- `.wikirc` loading warns about unknown keys.
- `changed` errors explain that git and a git repository are required.
- File scanners now propagate scan errors instead of swallowing them.
- `wiki-engine upgrade` internals refactored so the download flow is injectable and testable.

### Fixed

- `sync-prompts` no longer deletes user-owned files (custom slash commands, prompts, skills) in the sync directories — only wiki-managed files are cleaned up.
- `diff` now fails loudly on invalid git refs instead of reporting garbage output with exit code 0.
- `--json` is now honored by every command, including `version`, `init`, `sync-prompts`, and `upgrade`; fatal errors emit the `{ok: false, error}` envelope in JSON mode.
- `sync-prompts` diagnostics moved to stderr and the tips reference the configured `wiki_dir`.
- `lint --check=` / `lint --skip=` reject unknown checker names instead of silently running no checkers; `--skip=all` is rejected as a footgun.
- Lint on a missing wiki directory reports one clear diagnostic instead of a flood of per-checker errors.
- Invalid `fail_severity` values warn on load and fall back to `warn`.
- Plain-text `context --summarize` now prints per-page previews and line counts (previously only JSON carried them); `--active --summarize` is rejected with a clear error instead of being silently ignored, and the maintainer prompt now recommends `context --summarize`.
- `headings` and `search` now propagate file-open errors instead of silently skipping unreadable files; `refresh` propagates all sub-step errors.
- `markers` checker and `upgrade` zip extraction close files per iteration (file-descriptor hygiene).
- `upgrade` verifies the replacement binary by running `wiki-engine version` after the swap.
- Added a `--` flag terminator for free-form commands (`search`, `impact`) so arguments starting with dashes can be searched literally; `--` also protects a literal `--json` argument, and `-h`/`--help` works after any command.
- Prompt polish: the onboard shim template now lists all seven slash commands, and the upgrade workflow wording matches the new post-upgrade version verification.
- `.wikirc` now parses single-line `ignore` arrays (previously only the multiline form worked) and rejects empty/invalid `wiki_dir` values with a warning.
- Usage errors in `--json` mode (`search`, `summary`, `relevant`, `impact`, `diff` without arguments, disabled `watch`) now emit the standard error envelope.
- `init` rejects wiki directory names that escape the repository (`..`, absolute paths).
- `context` now reports the active phase as the last `in-progress` (or last `completed`) row of phases.md instead of blindly taking the last row.
- Front matter inline comments follow YAML rules: `#` only starts a comment outside quotes after whitespace, so `description: "C# guide"` survives.
- Numeric `.wikirc` values are parsed strictly — `log_lines = "1.5"` warns and falls back instead of silently becoming 15.
- `upgrade` downloads are capped at 100 MiB before checksum verification; `search` only scans markdown pages.
- `sync-prompts` now reports `updated` and `removed` separately (plain text and JSON) instead of mixing removal markers into the updated list.
- `context --json` omits `line_count` unless summaries are computed; `search --` with no terms shows usage; the `init` `.wikirc` rewrite is regex-based and tolerant of template whitespace.
- `context` rejects meaningless flag combinations (`--sort` without `--active`, `--minimal`/`--summarize` with `--active`) instead of silently ignoring them; `orphans` exempts `legacy`/`deprecated` pages like `leaf-pages` does; explicit help requests print to stdout.
- `upgrade` pins the `go install` fallback to the discovered release tag when it is known, fsyncs the staged binary before swapping it in, and `stale-content` fetches all page commit dates in a single `git log` invocation.
- `lint --json` carries the real diagnostic in the `error` field when the wiki directory is missing; `context --active` warns when unlinked-page detection fails.

## [0.x]

Legacy pre-1.0 releases (tags `0.1.0`, `v.0.5.0`). Consolidated from the wiki
maintenance log ([wiki/prologue/log.md](wiki/prologue/log.md)).

### Added

- 17 composable lint checkers with a `fail_severity` exit gate.
- Page lifecycle front matter: `planned` / `current` / `legacy` / `deprecated`.
- Active wiki graph navigation (`context --active`, `--sort=topo|chrono`).
- Progressive disclosure (`context --summarize`, `summary`, `relevant`).
- Change detection (`changed`, `candidates`, `impact`, `diff`, `refresh`, `watch --once`).
- Multi-tool instruction layer: canonical `.wiki-instructions/` with symlinks for GitHub Copilot, Claude Code, and pi.dev.
- Checksum-verified self-upgrade (`upgrade`) and prompt syncing (`sync-prompts`).
- Organized wiki layout (`prologue/`, `decisions/`, `operations/`) with legacy flat-layout fallback.

### Fixed

- `log-tail` returning the oldest entries instead of the most recent.
- Path traversal via `summary`.
- Cross-page links now strictly page-relative (wiki-root fallback removed).
- Stale-content detection now uses git commit dates (mtime fallback).
