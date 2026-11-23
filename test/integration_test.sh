#!/usr/bin/env bash
# Integration tests for wiki-engine.
# Run from repo root: bash test/integration_test.sh
set -euo pipefail

BIN="$(pwd)/bin/wiki-engine"
TMPDIR="$(mktemp -d)"
trap 'rm -rf "$TMPDIR"' EXIT

echo "=== integration tests ==="
echo "tmp dir: $TMPDIR"

# Setup: init a git repo
cd "$TMPDIR"
git init -q -b main
git config user.email "test@test"
git config user.name "Test"

# Create a source file to have something to diff against.
echo "package main" > main.go
git add main.go
git commit -q -m "initial commit"

# Test: init
echo "--- init ---"
"$BIN" init
test -d wiki || { echo "FAIL: wiki/ not created"; exit 1; }
test -f .wikirc || { echo "FAIL: .wikirc not created"; exit 1; }
test -f .github/prompts/wiki-ingest.prompt.md || { echo "FAIL: prompts missing"; exit 1; }
test -d .pi/skills/wiki || { echo "FAIL: pi skill missing"; exit 1; }
echo "  ok"

# Test: tool layers are symlinks (skip on platforms without symlink support)
echo "--- prompt symlinks ---"
case "$(uname -s 2>/dev/null || echo Windows)" in
  Windows|MINGW*|MSYS*|CYGWIN*) echo "  skipped (no symlinks)" ;;
  *)
    test -L .github/prompts/wiki-ingest.prompt.md || { echo "FAIL: prompts should be symlinks"; exit 1; }
    test -L .github/prompts/wiki-watch.prompt.md || { echo "FAIL: watch prompt symlink missing"; exit 1; }
    test -L .claude/commands/wiki-watch.md || { echo "FAIL: claude watch symlink missing"; exit 1; }
    echo "  ok"
    ;;
esac

# Test: --json init creates wiki/ (not a directory named after the flag)
echo "--- json init ---"
mkdir -p "$TMPDIR/jsontest"
cd "$TMPDIR/jsontest"
git init -q -b main
git config user.email "test@test"
git config user.name "Test"
"$BIN" --json init
test -d wiki || { echo "FAIL: --json init did not create wiki/"; exit 1; }
cd "$TMPDIR"
echo "  ok"

# Test: help goes to stdout when explicitly requested
echo "--- help ---"
"$BIN" help 2>/dev/null | grep -q "wiki-engine — repo-local wiki management tool" || { echo "FAIL: help should print to stdout"; exit 1; }
"$BIN" list -h 2>/dev/null | grep -q "Usage:" || { echo "FAIL: list -h should print usage to stdout"; exit 1; }
echo "  ok"

# Test: list
echo "--- list ---"
count=$("$BIN" list | wc -l)
if [ "$count" -lt 5 ]; then
  echo "FAIL: expected >=5 wiki files, got $count"
  exit 1
fi
echo "  ok ($count files)"

# Test: headings
echo "--- headings ---"
"$BIN" headings | grep -q "Wiki Index" || { echo "FAIL: expected 'Wiki Index' heading"; exit 1; }
echo "  ok"

# Test: search
echo "--- search ---"
"$BIN" search "schema" | grep -q "schema.md" || { echo "FAIL: search failed"; exit 1; }
echo "  ok"

# Test: lint
echo "--- lint ---"
"$BIN" lint
echo "  ok"

# Test: stats
echo "--- stats ---"
"$BIN" stats | grep -q "files:" || { echo "FAIL: stats failed"; exit 1; }
echo "  ok"

# Test: context
echo "--- context ---"
"$BIN" context --minimal | grep -q "catalog" || { echo "FAIL: context failed"; exit 1; }
echo "  ok"

# Test: summary
echo "--- summary ---"
"$BIN" summary README.md | grep -q "# Wiki" || { echo "FAIL: summary failed"; exit 1; }
echo "  ok"

# Test: relevant
echo "--- relevant ---"
"$BIN" relevant "wiki" 3 | grep -q "." || { echo "FAIL: relevant failed"; exit 1; }
echo "  ok"

# Test: --json
echo "--- json ---"
"$BIN" --json stats | grep -q '"ok"' || { echo "FAIL: json output missing"; exit 1; }
"$BIN" --json lint | grep -q '\[\]' || { echo "FAIL: json lint failed"; exit 1; }
echo "  ok"

# Test: --json on admin commands
echo "--- json admin commands ---"
"$BIN" --json version | grep -q '"ok": true' || { echo "FAIL: --json version should emit an envelope"; exit 1; }
"$BIN" --json version | grep -qE '"data": "[^"]+"' || { echo "FAIL: --json version should carry the version in data"; exit 1; }
"$BIN" --json sync-prompts | grep -q '"updated"' || { echo "FAIL: --json sync-prompts should carry updated files"; exit 1; }
"$BIN" --json sync-prompts | grep -q '"removed"' || { echo "FAIL: --json sync-prompts should carry a removed list"; exit 1; }
if "$BIN" search -- >/dev/null 2>&1; then
  echo "FAIL: search -- without terms should exit non-zero"
  exit 1
fi
echo "  ok"

# Test: --json fatal errors carry the envelope
echo "--- json error envelope ---"
out=$("$BIN" --json summary does-not-exist.md 2>/dev/null || true)
echo "$out" | grep -q '"ok": false' || { echo "FAIL: fatal error should emit ok:false envelope"; exit 1; }
echo "$out" | grep -q '"error"' || { echo "FAIL: fatal error should carry an error field"; exit 1; }
out=$("$BIN" --json definitely-not-a-command 2>/dev/null || true)
echo "$out" | grep -q '"ok": false' || { echo "FAIL: unknown command should emit ok:false envelope"; exit 1; }
echo "  ok"

# Test: --json usage errors carry the envelope too
echo "--- json usage errors ---"
out=$("$BIN" --json search 2>/dev/null || true)
echo "$out" | grep -q '"ok": false' || { echo "FAIL: --json search without query should emit ok:false"; exit 1; }
echo "$out" | grep -q "usage" || { echo "FAIL: usage error envelope should carry the usage line"; exit 1; }
out=$("$BIN" --json summary 2>/dev/null || true)
echo "$out" | grep -q '"ok": false' || { echo "FAIL: --json summary without page should emit ok:false"; exit 1; }
if "$BIN" --json search >/dev/null 2>&1; then
  echo "FAIL: --json search without query should exit non-zero"
  exit 1
fi
echo "  ok"

# Test: diff
echo "--- diff ---"
echo "# test change" >> wiki/README.md
git add wiki/ && git commit -q -m "wiki change"
"$BIN" diff HEAD~1 HEAD | grep -q "changed" || { echo "FAIL: diff failed"; exit 1; }
echo "  ok"

# Test: watch --once
echo "--- watch --once ---"
"$BIN" watch --once 2>&1 || true  # may fail if no diff range available
echo "  ok"

# Test: impact (exact default + --fuzzy opt-in)
echo "--- impact ---"
echo "package main // changed" > main.go
git add main.go && git commit -q -m "source change"
"$BIN" impact --fuzzy main.go | grep -q "main.go" || { echo "FAIL: impact --fuzzy failed"; exit 1; }
"$BIN" --json impact main.go | grep -q '"changed_file"' || { echo "FAIL: json impact failed"; exit 1; }
echo "  ok"

# Test: duplicate detection (low threshold)
echo "--- duplicate detection ---"
printf '%s\n' '---' 'id: dup1' 'lifecycle: current' 'description: dup1' '---' '# Duplicate page' '' 'This is a long shared paragraph with many distinct vocabulary words alpha beta gamma delta epsilon zeta eta theta iota kappa lambda.' > wiki/dup1.md
printf '%s\n' '---' 'id: dup2' 'lifecycle: current' 'description: dup2' '---' '# Duplicate page' '' 'This is a long shared paragraph with many distinct vocabulary words alpha beta gamma delta epsilon zeta eta theta iota kappa lambda.' > wiki/dup2.md
# Set a very low threshold to trigger it
echo 'duplicate_threshold = 0.1' >> .wikirc
out=$("$BIN" lint 2>&1 || true)
echo "$out" | grep -q "duplicate-content" || { echo "FAIL: duplicate detection missing"; exit 1; }
echo "  duplicate detected"

# Test: active flag on list and context
echo "--- active flag ---"
# Make one of the operations legacy
echo -e "---\nstatus: legacy\ndescription: Legacy lint procedure\n---\n# Legacy Lint" > wiki/operations/lint.md
# Should not show up in list --active
"$BIN" list --active | grep -q "wiki/operations/lint.md" && { echo "FAIL: legacy page in list --active"; exit 1; }
# Should show up in context without --active
"$BIN" context | grep -q "operations/lint.md \[legacy\]" || { echo "FAIL: status missing in context catalog"; exit 1; }
# Should not show up in context --active
"$BIN" context --active | grep -q "operations/lint.md" && { echo "FAIL: legacy page in context --active"; exit 1; }
# Should show the active wiki graph format
active_out=$("$BIN" context --active)
echo "$active_out" | grep -q "== active wiki graph ==" || { echo "FAIL: active graph header missing"; exit 1; }
echo "$active_out" | grep -q "index.md \[current\]" || { echo "FAIL: active node index.md missing in graph"; exit 1; }
echo "$active_out" | grep -q "  -> prologue/schema.md" || { echo "FAIL: active edge in graph missing"; exit 1; }
# Sort topo check
topo_out=$("$BIN" context --active --sort=topo)
echo "$topo_out" | grep -q "== active wiki graph ==" || { echo "FAIL: topo sort failed"; exit 1; }
# Meaningless flag combos must be rejected, not silently ignored
if "$BIN" context --sort=topo >/dev/null 2>&1; then
  echo "FAIL: --sort without --active should be rejected"
  exit 1
fi
if "$BIN" context --minimal --active >/dev/null 2>&1; then
  echo "FAIL: --minimal --active should be rejected"
  exit 1
fi
# JSON graph format check
active_json=$("$BIN" --json context --active)
echo "$active_json" | grep -q '"nodes"' || { echo "FAIL: json graph output missing nodes"; exit 1; }
echo "$active_json" | grep -q '"edges"' || { echo "FAIL: json graph output missing edges"; exit 1; }
# graph command: navigation tree, neighborhood, stats, strict gate
# Run in a fresh-init project: earlier sections leave orphan pages (dup1/dup2) around.
echo "--- graph ---"
mkdir -p "$TMPDIR/graphproj"
cd "$TMPDIR/graphproj"
git init -q -b main
git config user.email "test@test"
git config user.name "Test"
"$BIN" init
graph_out=$("$BIN" graph)
echo "$graph_out" | grep -q "== wiki graph ==" || { echo "FAIL: graph header missing"; exit 1; }
echo "$graph_out" | grep -q "index.md \[current\]" || { echo "FAIL: graph tree missing root node"; exit 1; }
echo "$graph_out" | grep -q "stats: " || { echo "FAIL: graph stats missing"; exit 1; }
"$BIN" graph prologue/schema.md | grep -q "== backlinks ==" || { echo "FAIL: graph neighborhood missing backlinks"; exit 1; }
dot_out=$("$BIN" graph --dot)
echo "$dot_out" | grep -q "digraph wiki {" || { echo "FAIL: graph dot output missing digraph header"; exit 1; }
graph_json=$("$BIN" --json graph)
echo "$graph_json" | grep -q '"stats"' || { echo "FAIL: json graph output missing stats"; exit 1; }
"$BIN" graph --strict || { echo "FAIL: graph --strict should pass on a healthy wiki"; exit 1; }
if "$BIN" graph nope.md >/dev/null 2>&1; then
  echo "FAIL: graph with unknown page should be rejected"
  exit 1
fi
if "$BIN" graph --dot --json >/dev/null 2>&1; then
  echo "FAIL: graph --dot --json should be rejected"
  exit 1
fi
# json mode honors the page argument (neighborhood JSON, not the whole graph)
"$BIN" --json graph prologue/schema.md | grep -q '"backlinks"' || { echo "FAIL: json graph <page> missing neighborhood backlinks"; exit 1; }
# strict applies to neighborhood mode too
printf '%s\n' '---' 'status: current' 'description: Orphan2' '---' '# Orphan2' > wiki/orphan2.md
if "$BIN" graph prologue/schema.md --strict >/dev/null 2>&1; then
  echo "FAIL: graph <page> --strict should fail with an orphan page"
  exit 1
fi
rm wiki/orphan2.md
# graph --strict must fail when an orphan exists and when a link is broken
printf '%s\n' '---' 'status: current' 'description: Orphan' '---' '# Orphan' > wiki/orphan.md
if "$BIN" graph --strict >/dev/null 2>&1; then
  echo "FAIL: graph --strict should fail with an orphan page"
  exit 1
fi
"$BIN" graph | grep -q "orphan.md" || { echo "FAIL: graph should report the orphan page"; exit 1; }
printf '%s\n' '---' 'status: current' 'description: Broken' '---' '# Broken' '- [nope.md](nope.md)' > wiki/broken.md
cp wiki/index.md "$TMPDIR/index.md.bak"
printf '%s\n' '- [broken.md](broken.md)' >> wiki/index.md
if "$BIN" graph --strict >/dev/null 2>&1; then
  echo "FAIL: graph --strict should fail with a broken link"
  exit 1
fi
"$BIN" graph | grep -q "broken edge" || { echo "FAIL: graph should report the broken link"; exit 1; }
# references in front matter surface in the neighborhood view and are linted
echo "package main" > main.go
printf '%s\n' '---' 'status: current' 'description: Refs' 'references: [source:main.go, issue:JIRA-42]' '---' '# Refs' > wiki/refs.md
printf '%s\n' '- [refs.md](refs.md)' >> wiki/index.md
"$BIN" graph refs.md | grep -q "== references ==" || { echo "FAIL: graph neighborhood missing references section"; exit 1; }
"$BIN" graph refs.md | grep -q "issue: JIRA-42" || { echo "FAIL: graph neighborhood missing issue reference"; exit 1; }
"$BIN" lint --check=references || { echo "FAIL: lint --check=references should pass on valid references"; exit 1; }
# a broken source reference fails the references checker
printf '%s\n' '---' 'status: current' 'description: Bad' 'references: [source:nope.go, issue:not-a-key]' '---' '# Bad' > wiki/badref.md
if "$BIN" lint --check=references >/dev/null 2>&1; then
  echo "FAIL: lint --check=references should fail on broken source and malformed issue"
  exit 1
fi
rm wiki/refs.md wiki/badref.md
cp "$TMPDIR/index.md.bak" wiki/index.md
rm wiki/orphan.md wiki/broken.md
cd "$TMPDIR"
# Test: lint --check and --skip flags
echo "--- lint flags ---"
# Add a duplicate-content issue (by overriding dup1 and dup2 with same content >10 words)
printf '%s\n' '---' 'id: dup1' 'lifecycle: current' 'description: same' '---' '# Same' '' 'Shared body text alpha beta gamma delta epsilon zeta eta theta iota kappa lambda mu.' > wiki/dup1.md
printf '%s\n' '---' 'id: dup2' 'lifecycle: current' 'description: same' '---' '# Same' '' 'Shared body text alpha beta gamma delta epsilon zeta eta theta iota kappa lambda mu.' > wiki/dup2.md
# We set threshold low so it flags duplicate
echo 'duplicate_threshold = 0.1' >> .wikirc

# 1. lint --check=front-matter should pass because front matter is ok
"$BIN" lint --check=front-matter || { echo "FAIL: lint --check=front-matter failed"; exit 1; }

# 2. lint --skip=duplicate-content,orphans,heading-hierarchy should pass because we skip duplicate, orphans, and heading-hierarchy checks
"$BIN" lint --skip=duplicate-content,orphans,heading-hierarchy || { echo "FAIL: lint --skip=... failed"; exit 1; }

# 3. normal lint should fail because of duplicate-content
"$BIN" lint && { echo "FAIL: expected duplicate lint failure"; exit 1; } || true
echo "  ok"

# Test: unknown checker names fail loudly

echo "--- lint unknown checker ---"
if "$BIN" lint --check=definitely-not-a-checker >/dev/null 2>&1; then
  echo "FAIL: unknown checker should fail"
  exit 1
fi
out=$("$BIN" lint --check=definitely-not-a-checker 2>&1 || true)
echo "$out" | grep -q "unknown checker" || { echo "FAIL: expected unknown checker message"; exit 1; }
if "$BIN" lint --skip=all >/dev/null 2>&1; then
  echo "FAIL: --skip=all should be rejected"
  exit 1
fi
echo "  ok"

# Test: failing lint reports ok:false in JSON envelope
echo "--- lint json envelope ---"
if "$BIN" --json lint >/dev/null 2>&1; then
  echo "FAIL: failing lint should exit non-zero"
  exit 1
fi
out=$("$BIN" --json lint 2>/dev/null || true)
echo "$out" | grep -q '"ok": false' || { echo "FAIL: failing lint should report ok:false"; exit 1; }
echo "  ok"

# Test: lint on a missing wiki dir carries the diagnostic in the error field
echo "--- json lint missing wiki dir ---"
mkdir -p "$TMPDIR/nowikit"
cd "$TMPDIR/nowikit"
git init -q -b main
git config user.email "test@test"
git config user.name "Test"
out=$("$BIN" --json lint 2>/dev/null || true)
echo "$out" | grep -q '"error": "wiki directory not found' || { echo "FAIL: missing wiki dir should carry the diagnostic in the error field"; exit 1; }
cd "$TMPDIR"
echo "  ok"

# Test: duplicate_threshold = 0 disables duplicate detection
echo "--- duplicate_threshold 0 ---"
echo 'duplicate_threshold = 0' >> .wikirc
"$BIN" lint --check=duplicate-content || { echo "FAIL: duplicate-content should be disabled with threshold 0"; exit 1; }
echo "  ok"

# Test: context_summarize = true defaults context to --summarize mode
echo "--- context_summarize default ---"
echo 'context_summarize = true' >> .wikirc
"$BIN" --json context | grep -q '"summarized": true' || { echo "FAIL: context_summarize should default context to summarize mode"; exit 1; }
echo "  ok"

# Test: plain-text context --summarize includes per-page previews
echo "--- context summarize plain ---"
"$BIN" context --summarize | grep -q "(lines:" || { echo "FAIL: plain context --summarize should include previews and line counts"; exit 1; }
if "$BIN" context --active --summarize >/dev/null 2>&1; then
  echo "FAIL: --active --summarize should be rejected"
  exit 1
fi
out=$("$BIN" context --active --summarize 2>&1 || true)
echo "$out" | grep -q "cannot be combined" || { echo "FAIL: rejection should explain the combination"; exit 1; }
echo "  ok"

# Test: continuous watch is disabled when watch_interval = 0
echo "--- watch disabled ---"
if "$BIN" watch >/dev/null 2>&1; then
  echo "FAIL: watch with watch_interval=0 should exit non-zero"
  exit 1
fi
out=$("$BIN" --json watch 2>/dev/null || true)
echo "$out" | grep -q '"ok": false' || { echo "FAIL: --json watch guidance should emit ok:false"; exit 1; }
echo "  ok"

# Test: legacy flat wiki layout still lints (backward compatibility)
echo "--- legacy flat layout ---"
mkdir -p "$TMPDIR/legacytest"
cd "$TMPDIR/legacytest"
git init -q -b main
git config user.email "test@test"
git config user.name "Test"
mkdir -p wiki/operations
cat > wiki/index.md <<'EOF'
---
status: current
description: "Index"
---
# Index

- [README.md](README.md) | README
- [log.md](log.md) | Log
- [schema.md](schema.md) | Schema
- [phases.md](phases.md) | Phases
- [repo-map.md](repo-map.md) | Repo Map
- [operations/ingest.md](operations/ingest.md) | Ingest
- [operations/query.md](operations/query.md) | Query
- [operations/lint.md](operations/lint.md) | Lint
EOF
for f in README log schema phases repo-map; do
  cat > "wiki/$f.md" <<EOF
---
status: current
description: "$f"
---
# $f

See [index.md](index.md).
EOF
done
cat > wiki/operations/ingest.md <<'EOF'
---
status: current
description: "ingest"
---
# ingest

See [../index.md](../index.md).
EOF
cat > wiki/operations/query.md <<'EOF'
---
status: current
description: "query"
---
# query

See [ingest.md](ingest.md).
EOF
cat > wiki/operations/lint.md <<'EOF'
---
status: current
description: "lint"
---
# lint

See [query.md](query.md).
EOF
cat > .wikirc <<'EOF'
wiki_dir = "wiki"
EOF
git add .
git commit -q -m "legacy flat wiki"
"$BIN" lint || { echo "FAIL: legacy flat layout should still lint"; exit 1; }
cd "$TMPDIR"
echo "  ok"

# --- Parity scenarios (Test Plan 1-12) ---
echo "--- parity end-to-end scenarios ---"
mkdir -p "$TMPDIR/parityproj"
cd "$TMPDIR/parityproj"
git init -q -b main
git config user.email "test@test"
git config user.name "Test"
mkdir -p pkg/store
echo "package store" > pkg/store/db.go
echo "package main" > main.go
git add . && git commit -q -m "initial"

# 1. Initialize repository & check schema command
"$BIN" init
"$BIN" schema --section lifecycle | grep -q "superseded" || { echo "FAIL: schema --section lifecycle failed"; exit 1; }
"$BIN" --json schema --type entity | grep -q '"name": "entity"' || { echo "FAIL: json schema --type entity failed"; exit 1; }

# 2. Re-run initialization after prompt edit — repairs prompt, preserves user content
echo "# Custom user note" > AGENTS.md
echo "drifted" > .wiki-instructions/ingest.md
echo "custom-keep" > wiki/custom-keep.md
"$BIN" init
grep -q "Custom user note" AGENTS.md || { echo "FAIL: re-init lost user AGENTS.md content"; exit 1; }
grep -q "wiki/index.md" AGENTS.md || { echo "FAIL: re-init did not append fenced pointer"; exit 1; }
! grep -qx "drifted" .wiki-instructions/ingest.md || { echo "FAIL: re-init did not repair drifted prompt"; exit 1; }
grep -q "custom-keep" wiki/custom-keep.md || { echo "FAIL: re-init lost custom wiki page"; exit 1; }
rm wiki/custom-keep.md

# 3. Reject wiki path outside repo / multi-segment / reserved
if "$BIN" init ../outside >/dev/null 2>&1; then
  echo "FAIL: init should reject ../outside"
  exit 1
fi
if "$BIN" init wiki/child >/dev/null 2>&1; then
  echo "FAIL: init should reject multi-segment wiki/child"
  exit 1
fi
if "$BIN" init .git >/dev/null 2>&1; then
  echo "FAIL: init should reject .git"
  exit 1
fi

# 4. Migrate old frontmatter with lint --fix (and verify idempotency)
cat > wiki/legacy-doc.md <<'EOF'
---
status: legacy
description: "Old doc"
references: [source:main.go, external:https://example.com]
---
# Old Doc

Unique legacy body phrase only inside historical page.

See [Index](index.md).
EOF
printf '%s\n' '- [legacy-doc.md](legacy-doc.md) | Old doc' >> wiki/index.md
"$BIN" lint --fix
grep -q "id: legacy-doc" wiki/legacy-doc.md || { echo "FAIL: lint --fix did not add id"; exit 1; }
grep -q "lifecycle: legacy" wiki/legacy-doc.md || { echo "FAIL: lint --fix did not migrate status to lifecycle"; exit 1; }
grep -q "sources:" wiki/legacy-doc.md || { echo "FAIL: lint --fix did not convert inline source reference"; exit 1; }
grep -q '\[legacy\]' wiki/index.md || { echo "FAIL: lint --fix did not add index badge"; exit 1; }
fix2=$("$BIN" --json lint --fix)
echo "$fix2" | grep -q '"total_fixes": 0' || { echo "FAIL: second lint --fix was not idempotent: $fix2"; exit 1; }

# 5. Search active vs historical pages
if "$BIN" search "Unique legacy body" | grep -q "legacy-doc.md"; then
  echo "FAIL: default search must exclude legacy pages"
  exit 1
fi
"$BIN" search --include-history "Unique legacy body" | grep -q "legacy-doc.md" || { echo "FAIL: search --include-history should find legacy page"; exit 1; }

# 6. Build graph with dependencies and source nodes
mkdir -p wiki/entities
cat > wiki/entities/store.md <<'EOF'
---
id: store
title: Store
type: entity
lifecycle: current
description: "Storage engine"
sources:
  - file: pkg/store/
  - file: main.go
---
# Store

See [Index](../index.md).
EOF
cat > wiki/entities/api.md <<'EOF'
---
id: api
title: API
type: entity
lifecycle: current
description: "API layer"
depends_on: [store]
---
# API

See [Index](../index.md).
EOF
printf '%s\n' '- [entities/store.md](entities/store.md) | Storage engine' '- [entities/api.md](entities/api.md) | API layer' >> wiki/index.md
"$BIN" --json graph --include-sources | grep -q '"schema_version": "2"' || { echo "FAIL: graph JSON missing schema_version 2"; exit 1; }
"$BIN" --json graph --include-sources | grep -q '"kind": "depends_on"' || { echo "FAIL: graph missing depends_on edge"; exit 1; }
"$BIN" --json graph --include-sources | grep -q '"kind": "source"' || { echo "FAIL: graph missing opt-in source node"; exit 1; }
"$BIN" graph store | grep -q "entities/store.md" || { echo "FAIL: graph ID lookup failed"; exit 1; }

git add . && git commit -q -m "commit typed wiki"
git checkout -q -b feature/parity-check

# 7 & 8 & 10. Change a cited file (working tree) and run check -> exit 0 with pages_to_review
echo "package store // changed" > pkg/store/db.go
chk_out=$("$BIN" --json check --base main)
echo "$chk_out" | grep -q '"ok": true' || { echo "FAIL: check on modified cited file should be ok:true (review only)"; exit 1; }
echo "$chk_out" | grep -q 'wiki/entities/store.md' || { echo "FAIL: check should recommend reviewing store.md"; exit 1; }
echo "$chk_out" | grep -q '"directory"' || { echo "FAIL: check should report directory match reason"; exit 1; }
# Exact impact also reports directory match reason
imp_out=$("$BIN" --json impact pkg/store/db.go)
echo "$imp_out" | grep -q '"reason": "directory"' || { echo "FAIL: impact missing directory reason"; exit 1; }

# 9. Delete a cited file and verify check fails with exit 1 and reports breakage
git rm -q main.go
git commit -q -m "delete main.go"
if "$BIN" check --base main >/dev/null 2>&1; then
  echo "FAIL: check should exit 1 when cited source is deleted"
  exit 1
fi
brk_out=$("$BIN" --json check --base main || true)
echo "$brk_out" | grep -q '"deleted_source"' || { echo "FAIL: check JSON missing deleted_source finding"; exit 1; }

# Restore main.go so repo is healthy again
git checkout main -- main.go
git commit -q -m "restore main.go"

# 11 & 12. Rename wiki directory and re-run setup verifying idempotency
"$BIN" init --wiki-dir docs
test -d docs || { echo "FAIL: rename did not create docs/"; exit 1; }
test ! -d wiki || { echo "FAIL: rename left old wiki/ behind"; exit 1; }
grep -q 'wiki_dir = "docs"' .wikirc || { echo "FAIL: rename did not update .wikirc"; exit 1; }
grep -q 'docs/index.md' AGENTS.md || { echo "FAIL: rename did not update AGENTS.md shim"; exit 1; }
# Re-running init preserves unrelated files and docs/ content
echo "unrelated" > notes.txt
"$BIN" init
grep -qx "unrelated" notes.txt || { echo "FAIL: re-init modified unrelated file"; exit 1; }
"$BIN" lint || { echo "FAIL: renamed docs/ wiki failed lint"; exit 1; }

cd "$TMPDIR"
echo "  ok"

echo ""
echo "=== all integration tests passed ==="
