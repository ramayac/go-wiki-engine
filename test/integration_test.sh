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

# Test: impact
echo "--- impact ---"
echo "package main // changed" > main.go
git add main.go && git commit -q -m "source change"
# impact needs changed files as args, test with a known file
"$BIN" impact main.go | grep -q "main.go" || { echo "FAIL: impact failed"; exit 1; }
echo "  ok"

# Test: duplicate detection (low threshold)
echo "--- duplicate detection ---"
echo -e "---\nstatus: current\ndescription: dup1\n---\n# Duplicate page" > wiki/dup1.md
echo -e "---\nstatus: current\ndescription: dup2\n---\n# Duplicate page" > wiki/dup2.md
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
"$BIN" context --active | grep -q "== active wiki graph ==" || { echo "FAIL: active graph header missing"; exit 1; }
"$BIN" context --active | grep -q "index.md \[current\]" || { echo "FAIL: active node index.md missing in graph"; exit 1; }
"$BIN" context --active | grep -q "  -> prologue/schema.md" || { echo "FAIL: active edge in graph missing"; exit 1; }
# Sort topo check
"$BIN" context --active --sort=topo | grep -q "== active wiki graph ==" || { echo "FAIL: topo sort failed"; exit 1; }
# JSON graph format check
"$BIN" --json context --active | grep -q '"nodes"' || { echo "FAIL: json graph output missing nodes"; exit 1; }
"$BIN" --json context --active | grep -q '"edges"' || { echo "FAIL: json graph output missing edges"; exit 1; }
# Test: lint --check and --skip flags
echo "--- lint flags ---"
# Add a duplicate-content issue (by overriding dup1 and dup2 with same content)
echo -e "---\nstatus: current\ndescription: same\n---\nSame content" > wiki/dup1.md
echo -e "---\nstatus: current\ndescription: same\n---\nSame content" > wiki/dup2.md
# We set threshold low so it flags duplicate
echo 'duplicate_threshold = 0.1' >> .wikirc

# 1. lint --check=front-matter should pass because front matter is ok
"$BIN" lint --check=front-matter || { echo "FAIL: lint --check=front-matter failed"; exit 1; }

# 2. lint --skip=duplicate-content,orphans,heading-hierarchy should pass because we skip duplicate, orphans, and heading-hierarchy checks
"$BIN" lint --skip=duplicate-content,orphans,heading-hierarchy || { echo "FAIL: lint --skip=... failed"; exit 1; }

# 3. normal lint should fail because of duplicate-content
"$BIN" lint && { echo "FAIL: expected duplicate lint failure"; exit 1; } || true
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

# Test: continuous watch is disabled when watch_interval = 0
echo "--- watch disabled ---"
if "$BIN" watch >/dev/null 2>&1; then
  echo "FAIL: watch with watch_interval=0 should exit non-zero"
  exit 1
fi
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

echo ""
echo "=== all integration tests passed ==="
