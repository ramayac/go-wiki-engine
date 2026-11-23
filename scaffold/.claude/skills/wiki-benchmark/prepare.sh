#!/usr/bin/env bash
# Builds the two read-only workspaces for the wiki benchmark, outside the repo:
#   full/    - HEAD with the wiki (arms B and C)
#   nowiki/  - HEAD without the wiki, its tooling and the fenced wiki-engine
#              block in CLAUDE.md and AGENTS.md (arm A)
# Both drop .claude/, so no arm can read this skill's answer keys. Neither has
# git history, so the arms are equal on that point. Uncommitted changes are not
# in HEAD, so commit the wiki before you run this script.
set -euo pipefail

repo="$(git rev-parse --show-toplevel)"
commit="$(git -C "$repo" rev-parse --short HEAD)"

# The wiki directory comes from wiki_dir in .wikirc at HEAD. The default is "wiki".
wiki="$(git -C "$repo" show HEAD:.wikirc 2>/dev/null \
  | sed -n 's/^[[:space:]]*wiki_dir[[:space:]]*=[[:space:]]*"\([^"]*\)".*/\1/p' | head -n 1 || true)"
wiki="${wiki:-wiki}"
case "$wiki" in
  .* | */* | *[!A-Za-z0-9._-]*) echo "wiki_dir '$wiki' is not one safe path segment" >&2; exit 1 ;;
esac
if ! git -C "$repo" cat-file -e "HEAD:$wiki" 2>/dev/null; then
  echo "no $wiki/ at HEAD. Run 'wiki-engine init' and commit the wiki first." >&2
  exit 1
fi

tmp="${TMPDIR:-/tmp}"
out="$(mktemp -d "${tmp%/}/wiki-benchmark.XXXXXX")"

for arm in full nowiki; do
  mkdir -p "$out/$arm"
  git -C "$repo" archive HEAD | tar -x -C "$out/$arm"
  rm -rf "$out/$arm/.claude"
done

nw="$out/nowiki"
rm -rf "${nw:?}/$wiki" "$nw/.wiki-instructions" "$nw/.wikirc" "$nw/.pi/skills/wiki" \
  "$nw/.github/instructions/wiki-maintainer.instructions.md"
rm -f "$nw"/.github/prompts/wiki-*.prompt.md
for f in CLAUDE.md AGENTS.md; do
  [ -f "$nw/$f" ] || continue
  perl -0pi -e 's/\n*<!-- wiki-engine:shim:v\d+ -->.*?<!-- \/wiki-engine:shim -->\n*/\n/s' "$nw/$f"
  # A file that held only the wiki block is now empty. Remove it.
  grep -q '[^[:space:]]' "$nw/$f" || rm -f "$nw/$f"
done

test -d "$out/full/$wiki" && test ! -e "$nw/$wiki"
if grep -rqs "wiki-engine:shim" "$nw/CLAUDE.md" "$nw/AGENTS.md"; then
  echo "nowiki still has the wiki-engine block" >&2
  exit 1
fi
! find "$out" -path '*/.claude/*' | grep -q .

echo "OUT=$out"
echo "COMMIT=$commit"
echo "WIKI_DIR=$wiki"
echo "FULL=$out/full"
echo "NOWIKI=$nw"
