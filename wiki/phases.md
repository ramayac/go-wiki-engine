---
status: current
description: "Rollout phases and status board for wiki maturity."
superseded_by: ""
---
# Wiki Phases

## Status Board

| Phase | Name | Status | Exit Signal |
|---|---|---|---|
| 0 | Bootstrap the wiki | completed | Required files exist |
| 1 | Populate repo map | completed | Architecture and exclusions are recorded |
| 2 | First ingest cycle | completed | At least one ingest entry exists in log.md |
| 3 | Slash command coherence | completed | Prompts use active context and no overlap |
| 4 | CI & self-linting | completed | Linter gates PRs in GitHub Actions pipeline |
| 5 | Cleanup & polish | completed | Dead code removed and todo backlog cleaned |

## Post-Completion

All rollout phases are complete; the board is now a historical record. Open work
lives in [todo.md](todo.md). Reopen or append phases when a new feature wave
(e.g. a hardening cycle) needs staged tracking again.

## Related

- Open work: [todo.md](todo.md)
- Activity history: [log.md](log.md)
- Conventions: [README.md](README.md), [schema.md](schema.md)
