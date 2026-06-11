# Progress

## Status
Completed Phase 2: Linter Hardening

## Tasks
- [x] 1A. Document front matter schema in scaffold/wiki/schema.md and add front matter to all scaffold/wiki/*.md
- [x] 1B. Add ParseFrontMatter() to internal/engine/ (minimal hand-written YAML parser)
- [x] 1C. Add frontMatterChecker (required fields, valid status values, superseded_by linkage)
- [x] 1D. Add --active flag to list and context for lifecycle filtering
- [x] 1.5A. Extract shared link-parsing helper from crossPageLinksChecker and orphansChecker
- [x] 1.5B. Add BuildWikiGraph() — BFS from index.md, skips deprecated/legacy nodes
- [x] 1.5C. Implement topological (by depth) and chronological (by created/updated/mtime) graph sorting
- [x] 1.5D. Modify wiki-engine context --active to output compact graph reference instead of full page summaries
- [x] 2A. Fix Lint() severity gating — SevInfo issues should not cause exit code 1 (made fail_severity configurable in .wikirc, defaulting to warn)
- [x] 2B. Add indexFormatChecker — validate index entries use title, relative path, and pipe-separated description
- [x] 2C. Add bareUrlChecker — detect bare URLs and HTML <a> tags outside code blocks
- [x] 2D. Add frontMatterChecker (completed in Phase 1)
- [x] 2E. Add --check / --skip flags to wiki-engine lint

## Files Changed
- `scaffold/wiki/*.md` and `scaffold/wiki/operations/*.md` (added front matter)
- `internal/config/config.go` (added and parsed FailSeverity setting, defaulting to warn)
- `internal/config/config_test.go` (updated config loading tests)
- `internal/engine/frontmatter.go` (created front matter parser and PageFrontMatter method)
- `internal/engine/frontmatter_test.go` (created front matter unit tests)
- `internal/engine/graph.go` (created BFS BuildWikiGraph and SortNodes, extracted ExtractLinks helper)
- `internal/engine/graph_test.go` (created unit tests for graph builder and sorting)
- `internal/engine/engine.go` (updated Summary and Context signature/implementation)
- `internal/engine/engine_lint.go` (added and registered frontMatterChecker, indexFormatChecker, bareUrlChecker; implemented LintWithOptions; refactored orphansChecker to use ExtractLinks)
- `internal/engine/engine_test.go` (updated test setup and added tests for all checkers and options)
- `cmd/wiki-engine/main.go` (updated context, list and lint commands to support all new flags)
- `test/integration_test.sh` (updated to support front matter and tested active graph, sorting, json, and check/skip lint flags)

## Notes
- Phases 1, 1.5, and 2 completed and fully verified via go unit tests and integration_test.sh.
- All integration and unit tests pass successfully.



