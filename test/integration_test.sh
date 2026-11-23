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
out=$("$BIN" help 2>/dev/null)
grep -q "wiki-engine — repo-local wiki management tool" <<<"$out" || { echo "FAIL: help should print to stdout"; exit 1; }
out=$("$BIN" list -h 2>/dev/null)
grep -q "Usage:" <<<"$out" || { echo "FAIL: list -h should print usage to stdout"; exit 1; }
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
out=$("$BIN" headings)
grep -q "Wiki Index" <<<"$out" || { echo "FAIL: expected 'Wiki Index' heading"; exit 1; }
echo "  ok"

# Test: search
echo "--- search ---"
out=$("$BIN" search "schema")
grep -q "schema.md" <<<"$out" || { echo "FAIL: search failed"; exit 1; }
echo "  ok"

# Test: lint
echo "--- lint ---"
"$BIN" lint
echo "  ok"

# Test: stats
echo "--- stats ---"
out=$("$BIN" stats)
grep -q "files:" <<<"$out" || { echo "FAIL: stats failed"; exit 1; }
echo "  ok"

# Test: context
echo "--- context ---"
out=$("$BIN" context --minimal)
grep -q "catalog" <<<"$out" || { echo "FAIL: context failed"; exit 1; }
echo "  ok"

# Test: summary
echo "--- summary ---"
out=$("$BIN" summary README.md)
grep -q "# Wiki" <<<"$out" || { echo "FAIL: summary failed"; exit 1; }
echo "  ok"

# Test: relevant
echo "--- relevant ---"
out=$("$BIN" relevant "wiki" 3)
grep -q "." <<<"$out" || { echo "FAIL: relevant failed"; exit 1; }
echo "  ok"

# Test: --json
echo "--- json ---"
out=$("$BIN" --json stats)
grep -q '"ok"' <<<"$out" || { echo "FAIL: json output missing"; exit 1; }
out=$("$BIN" --json lint)
grep -q '\[\]' <<<"$out" || { echo "FAIL: json lint failed"; exit 1; }
echo "  ok"

# Test: --json on admin commands
echo "--- json admin commands ---"
out=$("$BIN" --json version)
grep -q '"ok": true' <<<"$out" || { echo "FAIL: --json version should emit an envelope"; exit 1; }
out=$("$BIN" --json version)
grep -qE '"data": "[^"]+"' <<<"$out" || { echo "FAIL: --json version should carry the version in data"; exit 1; }
out=$("$BIN" --json sync-prompts)
grep -q '"updated"' <<<"$out" || { echo "FAIL: --json sync-prompts should carry updated files"; exit 1; }
out=$("$BIN" --json sync-prompts)
grep -q '"removed"' <<<"$out" || { echo "FAIL: --json sync-prompts should carry a removed list"; exit 1; }
if "$BIN" search -- >/dev/null 2>&1; then
  echo "FAIL: search -- without terms should exit non-zero"
  exit 1
fi
echo "  ok"

# Test: --json fatal errors carry the envelope
echo "--- json error envelope ---"
out=$("$BIN" --json summary does-not-exist.md 2>/dev/null || true)
grep -q '"ok": false' <<<"$out" || { echo "FAIL: fatal error should emit ok:false envelope"; exit 1; }
grep -q '"error"' <<<"$out" || { echo "FAIL: fatal error should carry an error field"; exit 1; }
out=$("$BIN" --json definitely-not-a-command 2>/dev/null || true)
grep -q '"ok": false' <<<"$out" || { echo "FAIL: unknown command should emit ok:false envelope"; exit 1; }
echo "  ok"

# Test: --json usage errors carry the envelope too
echo "--- json usage errors ---"
out=$("$BIN" --json search 2>/dev/null || true)
grep -q '"ok": false' <<<"$out" || { echo "FAIL: --json search without query should emit ok:false"; exit 1; }
grep -q "usage" <<<"$out" || { echo "FAIL: usage error envelope should carry the usage line"; exit 1; }
out=$("$BIN" --json summary 2>/dev/null || true)
grep -q '"ok": false' <<<"$out" || { echo "FAIL: --json summary without page should emit ok:false"; exit 1; }
if "$BIN" --json search >/dev/null 2>&1; then
  echo "FAIL: --json search without query should exit non-zero"
  exit 1
fi
echo "  ok"

# Test: diff
echo "--- diff ---"
echo "# test change" >> wiki/README.md
git add wiki/ && git commit -q -m "wiki change"
out=$("$BIN" diff HEAD~1 HEAD)
grep -q "changed" <<<"$out" || { echo "FAIL: diff failed"; exit 1; }
echo "  ok"

# Test: watch --once
echo "--- watch --once ---"
"$BIN" watch --once 2>&1 || true  # may fail if no diff range available
echo "  ok"

# Test: impact (exact default + --fuzzy opt-in)
echo "--- impact ---"
echo "package main // changed" > main.go
git add main.go && git commit -q -m "source change"
out=$("$BIN" impact --fuzzy main.go)
grep -q "main.go" <<<"$out" || { echo "FAIL: impact --fuzzy failed"; exit 1; }
out=$("$BIN" --json impact main.go)
grep -q '"changed_file"' <<<"$out" || { echo "FAIL: json impact failed"; exit 1; }
echo "  ok"

# Test: duplicate detection (low threshold)
echo "--- duplicate detection ---"
printf '%s\n' '---' 'id: dup1' 'lifecycle: current' 'description: dup1' '---' '# Duplicate page' '' 'This is a long shared paragraph with many distinct vocabulary words alpha beta gamma delta epsilon zeta eta theta iota kappa lambda.' > wiki/dup1.md
printf '%s\n' '---' 'id: dup2' 'lifecycle: current' 'description: dup2' '---' '# Duplicate page' '' 'This is a long shared paragraph with many distinct vocabulary words alpha beta gamma delta epsilon zeta eta theta iota kappa lambda.' > wiki/dup2.md
# Set a very low threshold to trigger it
echo 'duplicate_threshold = 0.1' >> .wikirc
out=$("$BIN" lint 2>&1 || true)
grep -q "duplicate-content" <<<"$out" || { echo "FAIL: duplicate detection missing"; exit 1; }
echo "  duplicate detected"

# Test: active flag on list and context
echo "--- active flag ---"
# Make one of the operations legacy
echo -e "---\nstatus: legacy\ndescription: Legacy lint procedure\n---\n# Legacy Lint" > wiki/operations/lint.md
# Should not show up in list --active
out=$("$BIN" list --active)
if grep -q "wiki/operations/lint.md" <<<"$out"; then echo "FAIL: legacy page in list --active"; exit 1; fi
# Should show up in context without --active
out=$("$BIN" context)
grep -q "operations/lint.md \[legacy\]" <<<"$out" || { echo "FAIL: status missing in context catalog"; exit 1; }
# Should not show up in context --active
out=$("$BIN" context --active)
if grep -q "operations/lint.md" <<<"$out"; then echo "FAIL: legacy page in context --active"; exit 1; fi
# Should show the active wiki graph format
active_out=$("$BIN" context --active)
grep -q "== active wiki graph ==" <<<"$active_out" || { echo "FAIL: active graph header missing"; exit 1; }
grep -q "index.md \[current\]" <<<"$active_out" || { echo "FAIL: active node index.md missing in graph"; exit 1; }
grep -q "  -> prologue/schema.md" <<<"$active_out" || { echo "FAIL: active edge in graph missing"; exit 1; }
# Sort topo check
topo_out=$("$BIN" context --active --sort=topo)
grep -q "== active wiki graph ==" <<<"$topo_out" || { echo "FAIL: topo sort failed"; exit 1; }
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
grep -q '"nodes"' <<<"$active_json" || { echo "FAIL: json graph output missing nodes"; exit 1; }
grep -q '"edges"' <<<"$active_json" || { echo "FAIL: json graph output missing edges"; exit 1; }
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
grep -q "== wiki graph ==" <<<"$graph_out" || { echo "FAIL: graph header missing"; exit 1; }
grep -q "index.md \[current\]" <<<"$graph_out" || { echo "FAIL: graph tree missing root node"; exit 1; }
grep -q "stats: " <<<"$graph_out" || { echo "FAIL: graph stats missing"; exit 1; }
out=$("$BIN" graph prologue/schema.md)
grep -q "== backlinks ==" <<<"$out" || { echo "FAIL: graph neighborhood missing backlinks"; exit 1; }
dot_out=$("$BIN" graph --dot)
grep -q "digraph wiki {" <<<"$dot_out" || { echo "FAIL: graph dot output missing digraph header"; exit 1; }
graph_json=$("$BIN" --json graph)
grep -q '"stats"' <<<"$graph_json" || { echo "FAIL: json graph output missing stats"; exit 1; }
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
out=$("$BIN" --json graph prologue/schema.md)
grep -q '"backlinks"' <<<"$out" || { echo "FAIL: json graph <page> missing neighborhood backlinks"; exit 1; }
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
out=$("$BIN" graph)
grep -q "orphan.md" <<<"$out" || { echo "FAIL: graph should report the orphan page"; exit 1; }
printf '%s\n' '---' 'status: current' 'description: Broken' '---' '# Broken' '- [nope.md](nope.md)' > wiki/broken.md
cp wiki/index.md "$TMPDIR/index.md.bak"
printf '%s\n' '- [broken.md](broken.md)' >> wiki/index.md
if "$BIN" graph --strict >/dev/null 2>&1; then
  echo "FAIL: graph --strict should fail with a broken link"
  exit 1
fi
out=$("$BIN" graph)
grep -q "broken edge" <<<"$out" || { echo "FAIL: graph should report the broken link"; exit 1; }
# references in front matter surface in the neighborhood view and are linted
echo "package main" > main.go
printf '%s\n' '---' 'status: current' 'description: Refs' 'references: [source:main.go, issue:JIRA-42]' '---' '# Refs' > wiki/refs.md
printf '%s\n' '- [refs.md](refs.md)' >> wiki/index.md
out=$("$BIN" graph refs.md)
grep -q "== references ==" <<<"$out" || { echo "FAIL: graph neighborhood missing references section"; exit 1; }
out=$("$BIN" graph refs.md)
grep -q "issue: JIRA-42" <<<"$out" || { echo "FAIL: graph neighborhood missing issue reference"; exit 1; }
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
grep -q "unknown checker" <<<"$out" || { echo "FAIL: expected unknown checker message"; exit 1; }
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
grep -q '"ok": false' <<<"$out" || { echo "FAIL: failing lint should report ok:false"; exit 1; }
echo "  ok"

# Test: lint on a missing wiki dir carries the diagnostic in the error field
echo "--- json lint missing wiki dir ---"
mkdir -p "$TMPDIR/nowikit"
cd "$TMPDIR/nowikit"
git init -q -b main
git config user.email "test@test"
git config user.name "Test"
out=$("$BIN" --json lint 2>/dev/null || true)
grep -q '"error": "wiki directory not found' <<<"$out" || { echo "FAIL: missing wiki dir should carry the diagnostic in the error field"; exit 1; }
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
out=$("$BIN" --json context)
grep -q '"summarized": true' <<<"$out" || { echo "FAIL: context_summarize should default context to summarize mode"; exit 1; }
echo "  ok"

# Test: plain-text context --summarize includes per-page previews
echo "--- context summarize plain ---"
out=$("$BIN" context --summarize)
grep -q "(lines:" <<<"$out" || { echo "FAIL: plain context --summarize should include previews and line counts"; exit 1; }
if "$BIN" context --active --summarize >/dev/null 2>&1; then
  echo "FAIL: --active --summarize should be rejected"
  exit 1
fi
out=$("$BIN" context --active --summarize 2>&1 || true)
grep -q "cannot be combined" <<<"$out" || { echo "FAIL: rejection should explain the combination"; exit 1; }
echo "  ok"

# Test: continuous watch is disabled when watch_interval = 0
echo "--- watch disabled ---"
if "$BIN" watch >/dev/null 2>&1; then
  echo "FAIL: watch with watch_interval=0 should exit non-zero"
  exit 1
fi
out=$("$BIN" --json watch 2>/dev/null || true)
grep -q '"ok": false' <<<"$out" || { echo "FAIL: --json watch guidance should emit ok:false"; exit 1; }
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
out=$("$BIN" schema --section lifecycle)
grep -q "superseded" <<<"$out" || { echo "FAIL: schema --section lifecycle failed"; exit 1; }
out=$("$BIN" --json schema --type entity)
grep -q '"name": "entity"' <<<"$out" || { echo "FAIL: json schema --type entity failed"; exit 1; }

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
grep -q '"total_fixes": 0' <<<"$fix2" || { echo "FAIL: second lint --fix was not idempotent: $fix2"; exit 1; }

# 5. Search active vs historical pages
out=$("$BIN" search "Unique legacy body")
if grep -q "legacy-doc.md" <<<"$out"; then
  echo "FAIL: default search must exclude legacy pages"
  exit 1
fi
out=$("$BIN" search --include-history "Unique legacy body")
grep -q "legacy-doc.md" <<<"$out" || { echo "FAIL: search --include-history should find legacy page"; exit 1; }

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
out=$("$BIN" --json graph --include-sources)
grep -q '"schema_version": "2"' <<<"$out" || { echo "FAIL: graph JSON missing schema_version 2"; exit 1; }
out=$("$BIN" --json graph --include-sources)
grep -q '"kind": "depends_on"' <<<"$out" || { echo "FAIL: graph missing depends_on edge"; exit 1; }
out=$("$BIN" --json graph --include-sources)
grep -q '"kind": "source"' <<<"$out" || { echo "FAIL: graph missing opt-in source node"; exit 1; }
out=$("$BIN" graph store)
grep -q "entities/store.md" <<<"$out" || { echo "FAIL: graph ID lookup failed"; exit 1; }

git add . && git commit -q -m "commit typed wiki"
git checkout -q -b feature/parity-check

# 7 & 8 & 10. Change a cited file (working tree) and run check -> exit 0 with pages_to_review
echo "package store // changed" > pkg/store/db.go
chk_out=$("$BIN" --json check --base main)
grep -q '"ok": true' <<<"$chk_out" || { echo "FAIL: check on modified cited file should be ok:true (review only)"; exit 1; }
grep -q 'wiki/entities/store.md' <<<"$chk_out" || { echo "FAIL: check should recommend reviewing store.md"; exit 1; }
grep -q '"directory"' <<<"$chk_out" || { echo "FAIL: check should report directory match reason"; exit 1; }
# Exact impact also reports directory match reason
imp_out=$("$BIN" --json impact pkg/store/db.go)
grep -q '"reason": "directory"' <<<"$imp_out" || { echo "FAIL: impact missing directory reason"; exit 1; }

# 9. Delete a cited file and verify check fails with exit 1 and reports breakage
git rm -q main.go
git commit -q -m "delete main.go"
if "$BIN" check --base main >/dev/null 2>&1; then
  echo "FAIL: check should exit 1 when cited source is deleted"
  exit 1
fi
brk_out=$("$BIN" --json check --base main || true)
grep -q '"deleted_source"' <<<"$brk_out" || { echo "FAIL: check JSON missing deleted_source finding"; exit 1; }

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

# --- Security regressions ---
echo "--- security: option injection, symlink write-through, ownership ---"
mkdir -p "$TMPDIR/secproj"
cd "$TMPDIR/secproj"
git init -q -b main
git config user.email "test@test"
git config user.name "Test"
echo "package main" > main.go
git add main.go && git commit -q -m "initial"
"$BIN" init >/dev/null
git add -A && git commit -q -m "wiki"
echo "victim-original" > "$TMPDIR/victim"

# (a) default_diff in .wikirc must not reach git as an option
cp .wikirc "$TMPDIR/wikirc.good"
echo 'default_diff = "--output=../victim"' >> .wikirc
if "$BIN" changed >/dev/null 2>&1; then
  echo "FAIL: changed should reject an option-like default_diff"
  exit 1
fi
test "$(cat "$TMPDIR/victim")" = "victim-original" || { echo "FAIL: default_diff option injection modified the victim file"; exit 1; }
cp "$TMPDIR/wikirc.good" .wikirc

# (b) an option-like diff range on the command line is rejected
if "$BIN" changed -- '--output=../victim' >/dev/null 2>&1; then
  echo "FAIL: changed should reject an option-like diff range"
  exit 1
fi
test "$(cat "$TMPDIR/victim")" = "victim-original" || { echo "FAIL: diff range option injection modified the victim file"; exit 1; }

# (c) a symlinked managed directory must not be written through
rm -rf .wiki-instructions
mkdir "$TMPDIR/outside"
echo "outside-original" > "$TMPDIR/outside/ingest.md"
case "$(uname -s 2>/dev/null || echo Windows)" in
  Windows|MINGW*|MSYS*|CYGWIN*) echo "  symlink checks skipped (no symlinks)" ;;
  *)
    ln -s "$TMPDIR/outside" .wiki-instructions
    if "$BIN" sync-prompts >/dev/null 2>&1; then
      echo "FAIL: sync-prompts should fail when .wiki-instructions is a symlink outside the repo"
      exit 1
    fi
    test "$(cat "$TMPDIR/outside/ingest.md")" = "outside-original" || { echo "FAIL: sync-prompts wrote through a symlink"; exit 1; }
    test "$(ls "$TMPDIR/outside")" = "ingest.md" || { echo "FAIL: sync-prompts created files outside the repo"; exit 1; }
    rm .wiki-instructions
    "$BIN" sync-prompts >/dev/null
    ;;
esac

# (d) user-owned files in managed directories survive sync-prompts
mkdir -p .claude/commands
echo "my command" > .claude/commands/wiki-custom.md
"$BIN" sync-prompts >/dev/null
test "$(cat .claude/commands/wiki-custom.md)" = "my command" || { echo "FAIL: sync-prompts removed or changed a user file"; exit 1; }

# (e) an unknown command is a usage error (exit 2)
rc=0
"$BIN" bogus >/dev/null 2>&1 || rc=$?
test "$rc" -eq 2 || { echo "FAIL: unknown command should exit 2, got $rc"; exit 1; }

cd "$TMPDIR"
echo "  ok"

# --- vocabulary ---
echo "--- vocabulary ---"
mkdir -p "$TMPDIR/vocabproj"
cd "$TMPDIR/vocabproj"
git init -q -b main
git config user.email "test@test"
git config user.name "Test"
echo "package main" > main.go
git add . && git commit -q -m "initial"
"$BIN" init
test -f wiki/vocabulary.yaml || { echo "FAIL: init did not create wiki/vocabulary.yaml"; exit 1; }

# Free-form by default: any tag passes.
cat > wiki/tagged.md <<'EOF'
---
id: tagged
type: concept
lifecycle: current
description: "Tag test page"
tags: [Typo-Tag]
---
# Tagged

See [Index](index.md).
EOF
printf '%s\n' '- [tagged.md](tagged.md) | Tag test page' >> wiki/index.md
"$BIN" lint --check=vocabulary >/dev/null || { echo "FAIL: free-form vocabulary rejected a tag"; exit 1; }

# Listing one tag makes the vocabulary controlled.
printf 'concerns:\n  - typo-tag\n' > wiki/vocabulary.yaml
rc=0
out=$("$BIN" lint --check=vocabulary 2>&1) || rc=$?
test "$rc" -eq 1 || { echo "FAIL: unlisted tag should fail lint (exit 1), got $rc"; exit 1; }
grep -q 'did you mean "typo-tag"' <<<"$out" || { echo "FAIL: no did-you-mean hint: $out"; exit 1; }
"$BIN" lint --skip=vocabulary >/dev/null || { echo "FAIL: --skip=vocabulary did not silence the check"; exit 1; }

# Fixing the tag passes; re-running init keeps the user file.
sed -i.bak 's/Typo-Tag/typo-tag/' wiki/tagged.md && rm wiki/tagged.md.bak
"$BIN" lint --check=vocabulary >/dev/null || { echo "FAIL: listed tag rejected"; exit 1; }
"$BIN" init >/dev/null
grep -q "typo-tag" wiki/vocabulary.yaml || { echo "FAIL: re-init overwrote wiki/vocabulary.yaml"; exit 1; }

# A malformed file is an error, not free-form.
printf 'concerns: [a\n' > wiki/vocabulary.yaml
rc=0
"$BIN" lint --check=vocabulary >/dev/null 2>&1 || rc=$?
test "$rc" -eq 1 || { echo "FAIL: malformed vocabulary should fail lint, got $rc"; exit 1; }

out=$("$BIN" schema --section vocabulary)
grep -q "vocabulary.yaml" <<<"$out" || { echo "FAIL: schema --section vocabulary failed"; exit 1; }
cd "$TMPDIR"
echo "  ok"

echo ""
echo "=== all integration tests passed ==="
