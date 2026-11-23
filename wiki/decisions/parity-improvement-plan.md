---
id: parity-improvement-plan
title: Parity Improvement Plan
lifecycle: current
description: "Phased plan to raise go-wiki-engine engineering quality while preserving its public, multi-tool product model."
sources:
  - file: internal/config/config.go
  - file: internal/engine/engine.go
  - file: internal/engine/engine_lint.go
  - file: internal/engine/frontmatter.go
  - file: internal/engine/graph.go
  - file: internal/scaffold/scaffold.go
  - file: internal/upgrade/upgrade.go
---
# Parity Improvement Plan

## Goal

Raise this repository's engineering quality to a production-grade level without
changing its public, multi-tool product boundary.

The project must keep these strengths:

- Public CLI distribution
- GitHub Copilot, Claude Code, and pi.dev support
- A zero-API-key deterministic CLI
- Composable inspection commands
- A configurable `.wikirc`
- A broad and useful scaffold
- Human-readable graph output
- JSON output for automation
- Integration and race tests

The project must gain these qualities:

- Strict path containment
- Correct YAML parsing
- A typed page model
- A separate knowledge-lifecycle field
- Source citation and dependency metadata
- Branch-aware documentation checks
- A single-read wiki snapshot
- Fail-closed upgrades
- Reproducible and attested releases
- Idempotent bootstrap behavior

The plan compares behavior, not repository naming. The current implementation
must not lose its public, multi-tool scope.

## Parity Definition

Parity means all of these conditions are true:

1. The CLI cannot read or traverse outside the selected repository and wiki.
2. The documented frontmatter format is parsed as YAML.
3. Invalid, missing, or ambiguous metadata does not become active knowledge.
4. Every lint run uses one consistent wiki snapshot.
5. Every page has a stable machine identity.
6. Page authority is separate from page workflow status.
7. Pages can declare exact source files, related pages, and dependencies.
8. `changed` and impact analysis use a merge base and include working-tree
   changes.
9. Release verification cannot silently downgrade to an unverified install path.
10. CI proves tests, race safety, integration behavior, schema parity, scaffold
    parity, and release integrity.

## Architecture Direction

Keep the broad CLI. Replace the current shared `engine.go` model with explicit
models and focused packages.

```text
cmd/wiki-engine/
  main.go
  commands.go
  output.go

internal/config/
  config.go
  validate.go

internal/model/
  page.go
  wiki.go
  frontmatter.go
  graph.go
  source.go

internal/git/
  changes.go
  refs.go
  history.go

internal/engine/
  context.go
  search.go
  summary.go
  impact.go
  graph.go
  watch.go

internal/lint/
  lint.go
  snapshot.go
  checkers/

internal/scaffold/
  scaffold.go
  reconcile.go

internal/upgrade/
  upgrade.go
  verify.go
```

Apply these invariants:

1. Parse one page once.
2. Use one path resolver.
3. Use one source-reference resolver.
4. Use one Git change pipeline.
5. Build graph views from one graph model.
6. Treat operational I/O errors as command errors.
7. Keep semantic wiki writing in agent workflows.
8. Keep the global JSON envelope stable.
9. Keep release verification fail-closed.

## Priority Matrix

| Priority | Workstream | Outcome |
|---|---|---|
| P0 | Path and graph containment | No operation escapes the repo or wiki |
| P0 | YAML and frontmatter correctness | Metadata cannot silently default to active |
| P0 | Fail-closed upgrade | Verification failure cannot switch trust models |
| P0 | CI supply-chain hardening | Immutable Actions and attested releases |
| P1 | Typed page and single-read lint model | Consistent, correct validation |
| P1 | Authority lifecycle | Agents rank knowledge by trust |
| P1 | Exact source and dependency graph | Change impact is path-exact |
| P1 | Branch-aware change check | Docs and citations follow current branches |
| P2 | Idempotent bootstrap and reconciliation | Re-running setup is safe |
| P2 | Schema and command parity checks | Docs and code cannot drift silently |
| P2 | Performance and bounded context | Large wikis remain usable |
| P3 | Package and developer experience | The public product remains easy to adopt |

---

## Phase 0: Test Baseline and Safety Net

### Objective

Freeze the existing public behavior before internal changes.

### Work

1. Add golden tests for every CLI command in plain-text and JSON modes.
2. Add exit-code tests for success, usage errors, content errors, and setup
   errors.
3. Add integration coverage for:
   - `init`
   - Custom `wiki_dir`
   - `sync-prompts`
   - `lint`
   - `context`
   - `graph`
   - `impact`
   - `diff`
   - `watch --once`
   - `upgrade` with a mocked release endpoint
4. Add a cross-platform path fixture for relative links, escaped paths, CRLF,
   and Unicode names.
5. Add a test that records all public command names and flags.
6. Keep the existing race, integration, audit, and lint gates.

### Files

- `cmd/wiki-engine/main_test.go`
- `test/integration_test.sh`
- `internal/audit/audit_test.go`
- New command-contract test files

### Exit Criteria

- Existing commands retain their documented behavior.
- Every public command has JSON and non-JSON tests.
- The baseline suite is stable on Linux, macOS, and Windows CI.

---

## Phase 1: Correctness and Containment

### 1.1 Path Containment

#### Problem

`Init` validates a wiki directory, but runtime `.wikirc` loading accepts values
that can escape the repository. `ExtractLinks` can also resolve a Markdown
target outside the configured wiki.

#### Work

1. Create `internal/paths` with one containment API.
2. Require the wiki directory to be one safe path segment.
3. Reject:
   - Empty values
   - `.`
   - `..`
   - Absolute paths
   - Path separators
   - Repository metadata names
4. Apply the validator in:
   - `.wikirc` load
   - `init`
   - `sync-prompts`
   - Graph traversal
   - Page summary
   - Source-reference validation
5. Resolve a file and verify that it remains inside the intended boundary
   before opening it.

#### Tests

Add cases for:

- `../wiki`
- `/tmp/wiki`
- `wiki/child`
- `wiki\child`
- Symlink escapes
- Markdown links with `../`
- Root-level repository links
- Percent-encoded path components

#### Exit Criteria

- No CLI path can select a wiki outside the repo.
- No page-to-page graph traversal can leave the wiki.
- Repo-file citations can still reference valid files outside the wiki.

### 1.2 Replace the Frontmatter Parser

#### Problem

The current hand-written parser supports only selected flat lines. It treats
the file as YAML but does not provide YAML semantics. It also starts from
`status: current`, so a missing field can become active.

#### Work

1. Add `gopkg.in/yaml.v3`.
2. Parse a frontmatter block into `map[string]any`.
3. Parse typed fields from the map.
4. Keep unknown fields available for future checks.
5. Do not apply semantic defaults during parsing.
6. Return distinct results for:
   - Missing frontmatter
   - Unterminated block
   - Invalid YAML
   - Valid YAML with a missing required field
7. Preserve line numbers when practical.
8. Add a compatibility decoder for old inline `references`.

Example target model:

```go
type PageMetadata struct {
    ID           string
    Title        string
    Type         string
    Lifecycle    string
    Status       string
    Description  string
    Created      string
    Updated      string
    Tags         []string
    Related      []string
    DependsOn    []string
    SupersededBy string
    Sources      []SourceReference
    References   []Reference
}
```

#### Migration

Keep the old schema readable:

- `status: current` maps to `lifecycle: current`.
- A missing `lifecycle` is reported by lint.
- A page with no `status` does not become active.
- Old inline references continue to parse.
- A migration command or lint fixer emits structured fields.

#### Tests

Add:

- Missing status
- Multiline YAML values
- Multiline lists
- Quoted commas
- Escaped quotes
- Unknown fields
- Invalid YAML
- CRLF
- Unterminated frontmatter
- YAML scalar type errors

#### Exit Criteria

- The parser matches the documented YAML contract.
- No missing field receives an implicit active value.
- Old repositories remain readable during migration.

### 1.3 Propagate Operational Errors

#### Problem

Search, graph, stats, context, and lint can hide read or traversal failures.

#### Work

1. Return errors from wiki walkers.
2. Ignore only explicit, documented file categories.
3. Distinguish:
   - Missing file
   - Permission error
   - Invalid path
   - Invalid metadata
4. Return a structured partial-output warning only when a command contract
   explicitly supports partial results.
5. Remove success output on fatal read failure.

#### Exit Criteria

- No unreadable included page is silently skipped.
- `--json` error output identifies the failed operation and path.
- Partial behavior is always explicit.

---

## Phase 2: Typed Wiki Model

### 2.1 Stable Page Identity

#### Problem

File paths are currently the page identity. Repeated basenames, moved pages, and
frontmatter relationships are difficult to validate.

#### Work

Add a stable `id` field:

```yaml
id: architecture
title: Architecture
type: concept
```

Rules:

1. The ID is globally unique.
2. The ID matches the filename without `.md`.
3. The root `index.md` uses the reserved identity `index`.
4. `related` and `depends_on` use IDs.
5. Graph and search accept an ID or a repo-relative page path.
6. Moving a page updates the ID only when the filename changes.

#### Exit Criteria

- Duplicate IDs fail lint.
- A renamed or moved page is detected by graph and index checks.
- Path and ID lookups have deterministic precedence.

### 2.2 Page Types

Add the same semantic page classes:

| Type | Purpose |
|---|---|
| `concept` | Stable system knowledge |
| `entity` | Component, service, store, or integration |
| `incident` | Failure, response, and lesson |
| `decision` | Choice, alternatives, and consequences |
| `runbook` | Procedure with verification |

Type-specific checks:

- Decisions require `status`.
- Incidents require `severity` and `status`.
- Entity dependencies must point to entity pages.
- Concepts and runbooks should cite source files or source directories.

Keep page types additive. A repository can ignore optional templates, but it
must not receive silent parser failures.

### 2.3 Authority Lifecycle

Split page authority from workflow status.

Lifecycle values:

| Lifecycle | Meaning |
|---|---|
| `current` | Trusted current knowledge |
| `implemented` | A shipped plan or record |
| `wip` | Partially true and in progress |
| `planned` | Intended future state |
| `legacy` | Historical context |
| `superseded` | Replaced by another page |

Keep `status` for decision and incident workflow:

- Decision: `proposed`, `accepted`, `superseded`
- Incident: `active`, `resolved`

Compatibility rule:

1. Read old `status: planned|current|legacy|deprecated`.
2. Map to a lifecycle.
3. Warn on the old form.
4. Add a fixer or migration command.
5. Remove the fallback only in the next major release.

Lifecycle-aware behavior:

- Search defaults to `current`, `implemented`, and `wip`.
- Historical views require `--include-history`.
- Implemented, legacy, and superseded pages do not receive active-drift checks.
- Superseded pages require `superseded_by`.
- Non-current pages receive a visible banner and index badge.

### 2.4 Structured References

Replace the overloaded inline syntax with a structured model:

```yaml
sources:
  - file: internal/engine/graph.go
    repo: go-wiki-engine
references:
  - type: external
    value: https://example.com/design
  - type: issue
    value: PROJ-123
related: [architecture]
depends_on: [storage]
```

Compatibility:

- Continue reading `source:...`, `external:...`, and `issue:...` inline
  references.
- Lint the values through one reference model.
- Emit structured form with the fixer.

Reference rules:

1. Same-repo source paths must exist for checked live pages.
2. Cross-repo source entries require a repo value.
3. External references must use `http` or `https`.
4. Issue keys must use a configurable or documented pattern.
5. Related IDs must resolve.
6. Dependency IDs must resolve to entity pages.

---

## Phase 3: Single-Read Lint Architecture

### Problem

The checker interface is useful, but each checker reads and walks the wiki
independently. This duplicates I/O and can make one lint run observe different
states.

### Work

1. Create `WikiSnapshot`:

```go
type WikiSnapshot struct {
    Root      string
    WikiDir   string
    Pages     []*Page
    Index     *Page
    Log       *Page
    Tags      map[string]struct{}
    ByID      map[string]*Page
    ByPath    map[string]*Page
    LoadIssue []Issue
}
```

2. Read every included file once.
3. Parse frontmatter once.
4. Extract links once.
5. Extract references once.
6. Let checkers inspect the snapshot.
7. Keep file-system and Git checks in separate checker categories.
8. Add a checker-level timeout only if a real performance problem is measured.

### Checker Registry

Keep selectable checks:

```text
wiki-engine lint --check=frontmatter,links,sources
wiki-engine lint --skip=duplicate-content
```

Add grouped names:

- `schema`
- `links`
- `graph`
- `freshness`
- `security`

Do not make `--skip=all` valid.

### Mechanical Fixes

Add `lint --fix` in this order:

1. Convert old frontmatter fields.
2. Convert resolvable wiki links.
3. Add lifecycle banners.
4. Add index badges.
5. Reorder index entries by lifecycle.
6. Convert supported inline references.

Each fixer must:

- Be idempotent.
- Preserve unrelated content.
- Report file and count.
- Avoid changes when a target is ambiguous.

### Exit Criteria

- Each file is read once per lint run.
- Existing checker selection remains compatible.
- A failing read becomes one clear operational issue.
- Mechanical fixes are deterministic.

---

## Phase 4: Exact Change and Citation Analysis

### 4.1 Unify Git Change Discovery

#### Problem

`changed`, `candidates`, `impact`, `refresh`, and `watch` use overlapping
change logic. The current `changed` default range is simpler than a
branch-aware merge-base comparison.

#### Work

Create `internal/git/changes.go` with:

1. Base-ref resolution.
2. Merge-base calculation.
3. Rename tracking.
4. Tracked working-tree changes.
5. Untracked files.
6. Deleted and moved paths.
7. Working-tree modes that can be disabled for reproducible CI.

Output:

```go
type ChangeSet struct {
    Base          string
    MergeBase     string
    ChangedFiles  []string
    Renames       map[string]string
    Untracked     []string
}
```

Use this in:

- `changed`
- `candidates`
- `impact`
- `watch`
- `refresh`
- The new citation check

### 4.2 Add `wiki-engine check`

Add a branch-aware command:

```text
wiki-engine check [--base <ref>] [--json]
```

Behavior:

1. Load the change set.
2. Find changed live pages.
3. Validate their source and commit citations.
4. Find unchanged pages that cite changed source files.
5. Report `broken`, `warnings`, and `pages_to_review`.
6. Exit 1 only for breakage introduced by the branch.
7. Exit 2 for Git or setup errors.
8. Exit 0 for review recommendations alone.

### 4.3 Replace Ambiguous Impact Fallbacks

Current impact analysis can match every page that contains a basename.

New behavior:

1. Prefer exact `sources` matches.
2. Use exact Markdown links to repo files.
3. Use directory coverage for declared source directories.
4. Keep basename matching only behind:

```text
wiki-engine impact --fuzzy
```

5. Include the match reason in JSON output.

### Exit Criteria

- A renamed or deleted cited file reaches the correct page.
- Cross-repo sources do not match local files.
- Pages with exact sources ignore unrelated prose mentions.
- Untracked files are visible in local check mode.

---

## Phase 5: Graph and Search Model

### 5.1 Keep the Existing Visual Graph UX

Preserve:

- ASCII tree
- Neighborhood view
- Backlinks
- References
- DOT output
- Graph statistics
- Strict mode

### 5.2 Upgrade Graph Semantics

Build the graph from:

- Page IDs
- Validated Markdown links
- `related`
- `depends_on`
- Optional source nodes

Rules:

1. Page edges are deduplicated.
2. Self-edges are rejected.
3. The index is a synthesized root.
4. Historical pages do not connect active traversal paths by default.
5. The index cannot connect every page in neighborhood traversal.
6. Source nodes use repo-relative path IDs.
7. Source nodes remain opt-in for full graph output.

JSON:

```json
{
  "nodes": [],
  "edges": [],
  "unlinked": [],
  "stats": {},
  "issues": []
}
```

Add a schema version to the graph payload.

### 5.3 Upgrade Search

Keep simple `search` behavior for exact-line use. Upgrade `relevant` or replace it
with one field-weighted ranker.

Ranking inputs:

- Page title
- Page ID
- Tags
- Section heading
- Body
- Exact phrase hits
- Lifecycle trust rank
- Unique query-term coverage

Filters:

- `--tag`
- `--type`
- `--lifecycle`
- `--include-history`
- `--limit`
- `--keyword`

Return:

- Page path and ID
- Lifecycle
- Score
- Matched terms
- Coverage
- Section heading
- Start line
- Bounded context

Keep `relevant` as a compatibility alias for one major release.

### Exit Criteria

- Search and graph use the same typed page IDs.
- Search excludes historical content by default.
- Visual graph features do not weaken path containment.
- Graph and search metadata are stable in JSON.

---

## Phase 6: Bootstrap and Reconciliation

### Problem

`init` is a one-time operation. Prompt updates are separated into
`sync-prompts`. This creates a safe flow, but it does not reconcile drift in a
single command.

### Work

1. Preserve `init` for new repositories.
2. Make repeated `init` act as reconciliation instead of always failing.
3. Detect the existing managed wiki before creating files.
4. Preserve:
   - Existing index
   - Existing log
   - Existing pages
   - Existing `.wikirc` values
   - User-authored AGENTS and CLAUDE content
5. Reconcile:
   - Prompt files
   - Tool symlinks
   - Ignore entries
   - Root shims
   - Schema templates
6. Add explicit wiki rename through `init --wiki-dir <name>`.
7. Refuse rename when the destination exists.
8. Remove retired engine-managed files only when the ownership rule is clear.
9. Keep `sync-prompts` as a compatibility command.

### Root Shim Rules

Generated root files must:

- Append a fenced wiki pointer to existing user content.
- Carry a versioned marker.
- Refresh pristine generated shims.
- Preserve edited shims.
- Never delete an existing shim without a request.

### Exit Criteria

- Running setup twice does not overwrite user content.
- Prompt and config drift can be repaired by one command.
- A wiki can be renamed without breaking generated links.

---

## Phase 7: Release and Supply-Chain Parity

### 7.1 Pin GitHub Actions

Pin every external Action to a full commit SHA.

Keep a version comment:

```yaml
uses: actions/checkout@<40-character-sha> # v4
```

Add a test or workflow policy that rejects mutable tags.

### 7.2 Fail-Closed Upgrade

Change the default upgrade flow:

1. Download checksums.
2. Download the release asset.
3. Verify checksum.
4. Verify artifact provenance when available.
5. Stage the replacement.
6. Replace atomically.
7. Execute `version` on the replacement.
8. Fail the command if verification fails.

Rules:

- Do not fall back from a checksum or attestation failure.
- Keep `go install` as an explicit `--from-source` path.
- Pin source install to the selected release tag.
- Cap both compressed and expanded download size.
- Stream archives to disk.
- Reject archive entries with unsafe paths.

### 7.3 Build Provenance

Add:

- Minimal workflow permissions
- Build-provenance attestation
- Attestation verification in the upgrader
- Per-platform checksums
- SBOM only if the project begins publishing dependencies that justify it

### 7.4 Vulnerability and Static Analysis

Keep:

- `go vet`
- `golangci-lint`
- Race tests
- Integration tests

Add:

- `govulncheck` after the YAML dependency is introduced
- A dependency review step
- A pinned tool-version policy

### Exit Criteria

- A failed verified download never installs another source.
- Every release Action is immutable.
- The release has checksums and provenance.
- Upgrade can explain every verification result.

---

## Phase 8: Schema, Prompt, and Documentation Parity

### 8.1 Schema in Binary

Keep the scaffold schema for backward compatibility, but make the binary able
to print the active schema:

```text
wiki-engine schema
wiki-engine schema --type entity
wiki-engine schema --section lifecycle
```

The embedded schema becomes the source of truth for:

- Required fields
- Valid page types
- Lifecycle values
- Source syntax
- Page templates

The repo-local schema remains a human guide.

### 8.2 Lifecycle Prompt Updates

Update every agent workflow to:

1. Start from active context.
2. Read lifecycle before relying on a page.
3. Ignore planned content as current fact.
4. Follow supersession links.
5. Add exact sources to changed pages.
6. Run lint after writes.
7. Run citation check after ingest or save.

Keep all tool layers generated from the same canonical workflow files.

### 8.3 Schema Parity Tests

Add tests for:

- Go vocabulary equals schema vocabulary.
- Page templates equal typed model requirements.
- CLI usage equals command registration.
- Wiki command docs equal executable flags.
- Live prompts equal scaffold prompts.
- Scaffold equals embedded files.

### 8.4 Public Documentation

Keep and improve:

- README
- CHANGELOG
- CONTRIBUTING
- SECURITY
- Release runbook
- Compatibility policy

Add:

- Exit-code contract
- JSON schema version
- Frontmatter schema version
- Migration and major-version policy

---

## Phase 9: Performance and Scale

### Work

1. Bound `context --summarize` output.
2. Add page and byte limits.
3. Report truncation.
4. Build graph and search indexes from the same snapshot.
5. Replace repeated graph rebuilds with one in-memory view per command.
6. Benchmark search on 100, 1,000, and 10,000 pages.
7. Replace O(n²) duplicate content checks when benchmarks show a cost problem.
8. Exclude frontmatter, templates, and code blocks from duplicate analysis.
9. Cache Git page history in one invocation per operation.

### Exit Criteria

- Large output reports truncation.
- One command has one file snapshot.
- Search and graph remain deterministic.
- CI records benchmark regressions when practical.

---

## Compatibility Plan

### CLI

Keep existing commands valid during migration.

| Existing command | Future behavior |
|---|---|
| `search` | Remains; gains filters and context |
| `relevant` | Compatibility alias for ranked search |
| `list --active` | Uses lifecycle filter |
| `context` | Uses typed lifecycle metadata |
| `graph` | Keeps all visual modes |
| `impact` | Defaults to exact references; fuzzy mode is explicit |
| `sync-prompts` | Remains; reconciliation becomes available through init |
| `upgrade` | Keeps command; becomes fail-closed |

### Frontmatter

Support the old form for one major release:

```yaml
status: current
description: "..."
superseded_by: ""
references: [source:main.go]
```

Migrate to:

```yaml
id: main-service
title: Main Service
type: entity
lifecycle: current
created: "2026-11-23"
updated: "2026-11-23"
tags: [service]
sources:
  - file: main.go
    repo: go-wiki-engine
```

### `.wikirc`

Keep all supported keys. Add only:

- Base-ref fallback mode if needed
- Typed reference issue-key pattern if needed
- Schema version if repositories need an override

Do not add a broad mechanism that disables required trust checks.

---

## Test Plan

### Unit Tests

Required additions:

- Safe path resolution
- YAML parser and scalar types
- Missing lifecycle and status
- Global ID uniqueness
- Relationship and entity validation
- Structured source resolution
- Directory source coverage
- Cross-repo source filtering
- Branch merge-base calculation
- Working-tree and untracked changes
- Citation breakage attribution
- Source impact reason
- Graph containment
- Supersession traversal
- Upgrade verification failure
- Archive size and path checks
- Reconciliation idempotency

### Integration Tests

Add scenarios:

1. Initialize a repository.
2. Re-run initialization after prompt edits.
3. Reject a wiki path outside the repo.
4. Migrate old frontmatter.
5. Search active and historical pages.
6. Build a graph with dependencies and source nodes.
7. Change a cited file.
8. Run `check`.
9. Delete a cited file.
10. Verify review output.
11. Rename a wiki directory.
12. Run setup again and verify no unrelated file changed.

### CI Gates

Required jobs:

1. Unit tests
2. Race tests
3. Vet
4. golangci-lint
5. govulncheck
6. Integration tests
7. Wiki audit
8. Wiki lint
9. Scaffold sync
10. Action pin policy
11. Release verification test

---

## Milestones

### Milestone 1: Safety

Deliver:

- Safe paths
- Correct YAML
- Error propagation
- P0 tests

Release as a patch or minor release only when compatibility tests pass.

### Milestone 2: Typed Knowledge

Deliver:

- IDs
- Page types
- Lifecycle
- Structured sources
- Single-read snapshot

Release as a major release if required frontmatter fields become mandatory.

### Milestone 3: Exact Change Tracking

Deliver:

- Shared change model
- `check`
- Exact impact
- Citation validation

Release as a minor release.

### Milestone 4: Reconciliation and Supply Chain

Deliver:

- Idempotent setup
- Fail-closed upgrade
- Immutable Actions
- Attestation

Release as a minor release, or as a major release if upgrade behavior is
breaking.

### Milestone 5: Scale and Documentation

Deliver:

- Search ranking
- Bounded context
- Graph schema
- Compatibility policy
- Migration guide

---

## Completion Checklist

The parity work is complete only when all items below are true:

- [x] No command can read outside the intended repo or wiki boundary.
- [x] Missing lifecycle metadata cannot become active by default.
- [x] Frontmatter uses a real YAML parser.
- [x] Every lint run uses one wiki snapshot.
- [x] Pages have stable IDs.
- [x] Page types and relationships are validated.
- [x] Sources are exact and structured.
- [x] Branch checks include the working tree and untracked files.
- [x] Existing `impact` remains available and reports exact match reasons.
- [x] Upgrade verification has no silent fallback.
- [x] Actions are pinned by immutable SHA.
- [x] Release assets have checksums and provenance.
- [x] Re-running setup preserves user content.
- [x] Schema, prompts, flags, and docs have parity tests.
- [x] Race and integration tests run in CI.
- [x] The global JSON envelope remains stable and documented.

## Non-Goals

This plan does not:

- Remove Copilot or pi.dev support.
- Move semantic content generation into the Go binary.
- Replace the existing human-readable graph views with JSON-only output.
- Remove `.wikirc`.
- Replace the public CLI with the private product architecture.
- Delete historical wiki pages during migration.
- Force all users to migrate in one release.

## Related

- [lessons.md](lessons.md)
- [todo.md](todo.md)
- [repo-map.md](../prologue/repo-map.md)
- [schema.md](../prologue/schema.md)
- [operations/lint.md](../operations/lint.md)
- [operations/release.md](../operations/release.md)
