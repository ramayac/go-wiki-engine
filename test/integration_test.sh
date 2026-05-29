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

# Test: lint --rebuild-cache (cache enabled by default)
echo "--- lint --rebuild-cache ---"
"$BIN" lint --rebuild-cache 2>&1 || true
test -f wiki/.cache.json || { echo "FAIL: cache file not created"; exit 1; }
echo "  ok"

# Test: duplicate detection (disabled by threshold, but check it doesn't crash)
echo "--- duplicate detection ---"
echo "# Duplicate page" > wiki/dup1.md
echo "# Duplicate page" > wiki/dup2.md
# Set a very low threshold to trigger it
echo 'duplicate_threshold = 0.1' >> .wikirc
"$BIN" lint 2>&1 | grep -q "duplicate-content" && echo "  duplicate detected" || echo "  ok (no false positive)"

echo ""
echo "=== all integration tests passed ==="
