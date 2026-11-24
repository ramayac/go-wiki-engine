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

### Changed

- **Exit codes separate usage errors from runtime failures:** Usage errors (unknown command or flag, missing or extra argument, invalid flag value, conflicting flags) exit `2`; they exited `1` before. Runtime and finding failures stay at `1`, and `check` keeps `0`/`1`/`2`. See the exit-code table in [README.md](README.md).
- **`--json` `ok` matches the exit code:** `graph --strict` and `watch --once` emit `"ok": false` with an `error` summary (and the same `data`) when they exit `1`.
- **`refresh --json` emits one JSON object** (`diff_range`, `needed`, `wiki_files`, `recent_log`, `changed`, `candidates`, `lint`). `refresh` exits `1` when its lint section fails, using the same `fail_severity` gate as `lint`.
- **Empty results encode as `[]`, not `null`:** This applies to `changed`, `candidates`, `search`, `relevant`, `log-tail`, `headings`, `diff`, `impact`, `watch --once`, `context`, graph neighborhood, and `sync-prompts`.
- **`impact` is exact by default:** It matches declared `sources`, directory coverage, and exact Markdown repo links, with explicit match reasons in JSON. Basename prose matching is opt-in via `--fuzzy`.
- **Default diff detects the base branch:** `main...HEAD` no longer fails on `master` (or other) repos; the engine detects the base branch like `check`. Git errors now show git's reason and tell "not a git repository" from "unknown revision".
- **`ignore` patterns:** A pattern with `/` now matches the full path (`docs/*.md`, `**`). A bare name matches any path segment.
- **Git 2.24 or newer is required.**
- **`sync-prompts` removes only engine-owned files:** It removes symlinks that point into `.wiki-instructions/` and files named in `retiredManagedFiles`. User files in the sync directories, `.pi/skills/wiki/`, and `.wiki-instructions/` stay; before, any `wiki-*` file or file the engine did not ship was deleted.
- **Edited managed files are backed up:** A hand-edited regular file that `sync-prompts` or `init` replaces with a tool-directory symlink is first copied to `<name>.bak` (then `.bak.1`, ...), with one line on stderr. Identical files are replaced silently.
- **`wiki_dir` and `init` argument rules:** A `wiki_dir` that starts with `-` is rejected in `init` and in `.wikirc`. `init <name>` in a repo whose `.wikirc` names a different wiki directory now fails with a message instead of leaving `.wikirc` stale.
- **`upgrade` verifies provenance only through `gh attestation verify`:** It is pinned to the `release.yml` signer workflow as well as the repository, and no longer trusts an `.intoto.jsonl` file served by the release. Without `gh` or attestations it still proceeds on the checksum, unless `--require-attestation` is set; any other `gh` failure stops the upgrade. `SECURITY.md` and the release page state what is proven.
- **`upgrade` is fail-closed:** It never falls back to `go install` on download, checksum, or provenance failure. It streams archives to disk with compressed/expanded size caps and path-traversal checks, and `go install` needs explicit `--from-source` pinned to the release tag.

### Fixed

- **`init 'a$b'` wrote `wiki_dir = "a"`:** The `$` was treated as a template reference; `init` now keeps the name.
- **`--` terminator:** `summary`, `diff`, and `log-tail` now treat `--` as the end of flags, like the other commands.
- **Unknown command with a broken `.wikirc`:** The command name is checked before `.wikirc` loads, so the error says "unknown command".
- **Continuous `watch` repeated output:** It prints a tick only when the changed files, candidates, or lint issues differ from the last printed tick.
- **Subdirectory projects:** `changed` and `candidates` return paths relative to `.wikirc`, unquoted for non-ASCII names. `candidates` no longer lists deleted files or the old path of a rename.
- **Non-ASCII wiki paths and shallow clones:** `stale-content` now sees non-ASCII page names and skips (with one `info` issue) in a shallow clone. `check` reports a cited commit missing from a shallow clone as a warning (`unverifiable_commit`), not `invalid_commit`.
- **`phase-consistency` on boards numbered from 1:** A phase now compares with the nearest lower phase on the board, not with `num-1`, so a board that starts at 1 or skips numbers no longer reports a missing phase. Only `in-progress` and `completed` phases are checked; a `blocked` phase may wait on an open one.
- **`markdown-format` false positives:** Task-list items such as `- [ ] (optional)` no longer report a spaced link, and prose such as `m[i][j]` no longer reports a reference-style link. Real `[text] (url)` and `[text][ref]` still report.
- **`bare-urls` and `<https://...>`:** A URL in angle brackets is a valid autolink and is no longer flagged. A naked URL still is.
- **`duplicate-content` flood:** The check prints the 50 most similar pairs and one `... and M more duplicate pair(s) not shown` line, not one line per pair.
- **Broken index link reported twice:** `cross-page-links` no longer repeats a link that `index-links` already reported at the same file, line, and target. `--check=cross-page-links` alone still reports it.
- **`relevant` and `search` read fenced `# comment` lines as headings:** Ranking, section names, and `summary` now skip ``` and ~~~ fenced code. `headings` also skips `~~~` fences.
- **`lint --fix` rewrote CRLF files to LF:** A fix now keeps each page's line endings and any UTF-8 BOM.
- **UTF-8 BOM:** A page that starts with a BOM no longer reports `missing front matter block`.

### Security

- **Git option injection:** `changed`, `candidates`, `refresh`, `watch --once`, and the `stale-content` check passed the diff range to `git diff` unchecked, so `--output=<path>` could overwrite a file. Ranges must now be revisions or `a..b` / `a...b` of existing revisions, `default_diff` may not start with `-`, and git calls pass `--end-of-options` and `--` where a user or config value reaches them.
- **Symlink write-through in `init` and `sync-prompts`:** The commands wrote through a symlinked managed directory (`.wiki-instructions`, `.claude/commands`, `.github/prompts`, `.github/instructions`, `.pi/skills/wiki`) or a symlinked `AGENTS.md`, `CLAUDE.md`, or `.wikirc` and changed files outside the repo. Every write, mkdir, remove, and symlink now checks that the real destination stays inside the repo and fails with an error naming the path. A symlinked file inside a managed directory is still replaced, not followed.
- **Unsigned provenance in `upgrade`:** A release-served `<asset>.intoto.jsonl` could report "verified" and satisfy `--require-attestation`. The code path is removed, so only a successful `gh attestation verify` verifies.
- **Release tag validation in `upgrade`:** The tag from the redirect must match `vMAJOR.MINOR.PATCH[-pre]` (64 characters at most) before any download or `go install`.
- **Downgrade guard in `upgrade`:** The upgrade refuses a latest release lower than the running version and names both versions. Dev and dirty builds may still upgrade.

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
