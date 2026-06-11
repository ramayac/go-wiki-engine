# Progress

## Status
Completed Phase 5: Cleanup & Polish. The improvement plan is 100% completed.

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
- [x] 3A. Update all .wiki-instructions/*.md to use context --active and respect lifecycle status
- [x] 3B. Add /wiki-lint prompt as new .wiki-instructions/lint.md
- [x] 3C. Reduce prompt overlap — extract shared steps into wiki-maintainer.md
- [x] 3D. Ensure prompt/command naming alignment (merge migrate-shims, document summarize as a flag)
- [x] 4A. Add make lint step to .github/workflows/test.yml CI pipeline
- [x] 4B. Add front matter to the project's own 14 wiki pages (migrated all 13 project wiki files)
- [x] 4C. Make make lint the PR gate for wiki health
- [x] 5A. Remove dead code: jsonOK(), jsonErr(), cachedList()
- [x] 5B. Fix todo.md staleness — mark impact (#19) as done, updated phases, and cleaned status markers
- [x] 5C. Fix scaffold sync drift — run wiki-engine sync-prompts to restore summarize.md
- [x] 5D. Fix .wikirc vs .wikirc.example branch inconsistency (master...HEAD → main...HEAD)
- [x] 5E. Replace manual f.Close() calls with defer f.Close() in engine.go

## Files Changed
- `scaffold/wiki/*.md` and `scaffold/wiki/operations/*.md` (added front matter)
- `wiki/*.md` and `wiki/operations/*.md` (added front matter to the project's own wiki pages)
- `internal/config/config.go` (added and parsed FailSeverity setting, defaulting to warn)
- `internal/config/config_test.go` (updated config loading tests)
- `internal/engine/frontmatter.go` (created front matter parser and PageFrontMatter method)
- `internal/engine/frontmatter_test.go` (created front matter unit tests)
- `internal/engine/graph.go` (created BFS BuildWikiGraph and SortNodes, extracted ExtractLinks helper)
- `internal/engine/graph_test.go` (created unit tests for graph builder and sorting)
- `internal/engine/engine.go` (updated Summary and Context signature/implementation; wrapped loop body file operations in anonymous functions with deferred closes; removed unused helper functions jsonOK and jsonErr)
- `internal/engine/engine_cache.go` (removed unused method cachedList)
- `internal/engine/engine_lint.go` (added and registered frontMatterChecker, indexFormatChecker, bareUrlChecker; implemented LintWithOptions; refactored orphansChecker to use ExtractLinks; refactored bareUrlChecker to check cleaned lines for HTML tags)
- `internal/engine/engine_test.go` (updated test setup and added tests for all checkers and options)
- `cmd/wiki-engine/main.go` (updated context, list and lint commands to support all new flags)
- `test/integration_test.sh` (updated to support front matter and tested active graph, sorting, json, and check/skip lint flags)
- `.github/workflows/test.yml` (consolidated individual check steps into make lint target)
- `scaffold/.wiki-instructions/` (updated all workflow instructions, merged migrate-shims and deleted summarize.md, created lint.md workflow)
- `.github/prompts/` & `.claude/commands/` (symlinks synchronized and cleaned up, added wiki-lint)
- `AGENTS.md` (replaced with the standard redirect shim pointing to the wiki)
- `wiki/improvement-plan.md` & `wiki/todo.md` (updated status tracking of all implementation phases and bookkeeping items)
- `README.md` (documented active lifecycles, linter selection check/skip flags, `/wiki-lint`, and newly supported .wikirc parameters)

## Notes
- All rollout and polish phases are fully completed and verified via go unit tests, integration_test.sh, and make lint.
- All integration and unit tests pass successfully.
