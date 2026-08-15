---
status: current
description: "Specification and usage guide for the .wikirc configuration file."
superseded_by: ""
---
# Configuration Reference

All settings live in `.wikirc` at the repo root. If the file is absent, sensible defaults are used. Values are `key = "value"` pairs (quotes optional for simple values). Arrays use bracket syntax. Architecture context lives in [repo-map.md](repo-map.md).

## Paths

### `wiki_dir`
Directory name for the wiki, relative to repo root.
```
wiki_dir = "wiki"
```
| Default | `"wiki"` |
|--------:|----------|

### `default_diff`
Git diff range used by `changed`, `candidates`, `refresh`, and `watch`.
```
default_diff = "main...HEAD"
```
| Default | `"main...HEAD"` |
|--------:|-----------------|

### `log_lines`
Number of log entries shown by `log-tail`.
```
log_lines = 10
```
| Default | `10` |
|--------:|------|

## Detection Thresholds

### `duplicate_threshold`
Jaccard word-set similarity above which two pages are flagged as potential duplicates by the `duplicate-content` lint checker (see [operations/lint.md](operations/lint.md)). Pages that share >70% word overlap typically indicate copy-paste drift.

```
duplicate_threshold = 0.7
```
| Default | `0.7` |
|--------:|-------|
| Range | 0.0–1.0 |
| Disable | Set to `0` |

### `stale_days`
Days since a page's last git commit before it is flagged as stale by the `stale-content` lint checker (see [operations/lint.md](operations/lint.md)). Falls back to filesystem mtime when git history is unavailable. When the repo has active source changes (`wiki-engine changed` returns files), severity upgrades from `info` to `warn`.

```
stale_days = 30
```
| Default | `30` |
|--------:|------|
| Disable | Set to `0` |

## Context Loading

### `context_summarize`
When `true`, `wiki-engine context` defaults to `--summarize` mode, including per-page previews (first heading, first paragraph, line count) in catalog entries. Useful for wikis with large pages where reading everything would waste tokens. The behavior can be toggled per invocation with the explicit flags.

```
context_summarize = false
```
| Default | `false` |
|--------:|---------|

When enabled, agents can follow a progressive disclosure pattern:

| Step | Command | Typical tokens |
|------|---------|----------------|
| 1. Catalog + previews | `wiki-engine context --summarize` | ~2K |
| 2. Borderline pages | `wiki-engine summary <page>` | ~200 |
| 3. Confirmed relevant | Read full page | per-page |

## Watch Mode

### `watch_interval`
Polling interval in seconds for `wiki-engine watch`. When > 0, the watch loop runs `changed` + `candidates` + `lint` on every tick. When 0, continuous `wiki-engine watch` exits with guidance; use `wiki-engine watch --once` for a single check regardless.

```
watch_interval = 0
```
| Default | `0` (continuous watch disabled) |
|--------:|-----------------|
| Typical | `60`–`300` |

## Ignored Paths

### `ignore`
Paths excluded from candidate filtering by `wiki-engine candidates` — the exclusion contract is described in [schema.md](schema.md). Three match modes:

| Pattern | Matches |
|---------|---------|
| `"wiki/"` | Directory prefix — any path starting with `wiki/` |
| `"*.log"` | Glob — matches file basename via `filepath.Match` |
| `"bin/tool"` | Exact path match |

```
ignore = [
  "wiki/",
  "bin/",
  "vendor/",
  "*.log",
  "*.tmp",
]
```

| Default | `["wiki/", "bin/", "*.log", "*.tmp"]` |
|--------:|------------------------------------------|

## Example

See [`.wikirc.example`](../.wikirc.example) at the repo root for a fully commented template.
