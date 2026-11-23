# Headless wiki benchmark

`bench.py` runs each benchmark question as a real `claude -p` session in a
throwaway copy of a repository. It then grades the answers and writes a
report. Arm names are hidden from the grader. It adds to the [`wiki-benchmark`](../SKILL.md)
skill. It does not replace it.

Why it exists: the skill's `Explore` answer agents load no `CLAUDE.md`, so they
cannot show whether a session follows the wiki routing in `CLAUDE.md`. A
headless session loads the project `CLAUDE.md` as a real session does.

## Requirements

- Python 3.9 or later. The script uses only the standard library.
- `claude` on `PATH`, logged in. Runs are billed to that account.
- `git` and `tar`. The target repository must be a git repository.
- A repository that ran `wiki-engine init`, with the wiki committed.

## Quick start

From the repository you want to measure. Replace the template questions in
[questions.md](../questions.md) first. The run stops while the template marker
is in the file.

```bash
B=.claude/skills/wiki-benchmark/headless/bench.py
python3 "$B" run --reps 2        # answer runs
python3 "$B" grade               # grading, after ALL arms finish
python3 "$B" report              # writes report.md and rows.json
python3 "$B" clean               # removes the temporary workspaces
```

`grade`, `report` and `clean` use the newest directory under
`<repo>/.wiki-engine/benchmark/headless/`. If `run` used `--out`, pass the
same `--out` to them.

`--questions` is optional. The default is the skill's [questions.md](../questions.md).
To benchmark a repository other than the one that holds the script, pass
`--repo` and `--questions`. Questions are about one repository, so the run
stops when `--questions` is missing in that case.

A cheap pilot: `run --only Q1,Q2 --arms A,B --reps 1 --model haiku --max-budget-usd 0.5`.

## Arms

| Arm | Workspace | Prompt prefix |
|---|---|---|
| A - no wiki | `NOWIKI` | none |
| B - as shipped | `FULL` | none |
| C - wiki first | `FULL` | `Start at <wiki>/index.md; read code to confirm.` |
| D - B plus a hook (optional) | `FULL_HOOK` | none |

`<wiki>` is the wiki directory. The default arms are `A,B,C`. Arm D runs only
with `--arms A,B,C,D --hook-settings FILE`. The script copies `FILE` to
`.claude/settings.local.json` in the `FULL_HOOK` workspace. The script ships no hook.

## Workspaces

`run` builds each workspace in a new temporary directory outside the repository:

1. Stop if a parent directory of the workspace root has a `CLAUDE.md`.
2. Extract `git archive HEAD`.
3. All workspaces: remove the skill directory `.claude/skills/wiki-benchmark`, so no arm reads the answer keys. Also remove the questions file when it is inside the repository, and each `--exclude PATH`. A path must be relative to the repository root and stay inside it. The run stops on an absolute path or a path with `..`.
4. `NOWIKI`: remove the wiki directory (`wiki_dir` from `.wikirc`, default `wiki`), the engine tooling (`.wiki-instructions/`, `.wikirc`, `.pi/skills/wiki/`, `.claude/commands/wiki-*`, `.github/prompts/wiki-*`, `.github/instructions/wiki-maintainer.instructions.md`), and the fenced block from `<!-- wiki-engine:shim:v2 -->` to `<!-- /wiki-engine:shim -->` in `CLAUDE.md` and `AGENTS.md`. A file that held only that block is removed.
5. Leak check: stop if any workspace file contains the first 80 characters of a question. The check does not look for the answer keys.
6. `git init` and one commit. The workspace has no history, so arm A cannot read the wiki through `git show`.

If a step fails, the run removes the temporary directory.

Step 1 matters because Claude Code loads `CLAUDE.md` from parent directories.
A workspace inside the target repository would load the full wiki routing in arm A.

The script prints a warning when a `NOWIKI` entry file (`CLAUDE.md`,
`AGENTS.md`, `.github/copilot-instructions.md`) still mentions `<wiki>/`
outside the fenced block. Arm A can then chase dangling references. The
script does not remove those lines, because they are part of the repository's
own text.

## Isolation

Each session runs with:

```
claude -p --output-format stream-json --verbose --setting-sources project,local
  --strict-mcp-config --no-session-persistence
  --disallowedTools Edit Write NotebookEdit --max-budget-usd <N> --model <M>
```

- `--setting-sources project,local` leaves out the user-level `~/.claude/CLAUDE.md`.
- `--strict-mcp-config` with no `--mcp-config` loads no MCP server.
- `--no-session-persistence` does not save the session to disk.

Check these flags against your Claude Code version before you trust a result.
They can change.

Inline mods can still load with these flags. Use `--disable-plugin
<name>@<source>` (repeatable) to disable one by name. To find the names, run one
pilot and read the isolation record in `report.md`. The report lists each
non-builtin plugin and each MCP server per arm, and prints a warning when an arm
has one.

The `init` event does not list which `CLAUDE.md` files loaded. The exclusion of
the user `CLAUDE.md` rests on the flag, not on a per-run record.

`--bare` is not used: it skips `CLAUDE.md` discovery and hooks, so the session is
not "as shipped". `--disable-slash-commands` is not used, because it also
disables the shipped `/wiki-*` commands.

## Grading

`grade` sends all answers to one question to one grader session in the
`FULL` workspace (default model `sonnet`). Each answer gets a random
three-letter ID, and the order is shuffled across arms. The grader sees no
arm names. The answer text can still show the arm. For example, an arm A
answer can say that the repository has no wiki. Run `grade` once, after every
arm has finished. `grade` skips a question that already has `grades/<id>.json`.
To grade again, delete `grades/`.

Grader output is a model judgment. A grader can count "the wiki does not exist"
as a wrong claim. Check each wrong claim by hand before you quote a count.

## Report

`report.md` has, per arm: accuracy, wrong claims, total and median cost,
median tokens, median turns, median duration, and three routing signals.
It also has a category split, a per-question table, the isolation record,
and every graded wrong claim.

Routing signals, per run:

- **Consulted wiki:** at least one tool input matches `(^|[\s/"'=])<wiki>(/|\s|$|")`. The `pattern` of a `Grep` call does not count.
- **Read index.md:** at least one tool input matches `(^|[\s/"'=])<wiki>/index\.md`.
- **wiki-engine / slash:** a Bash command calls `wiki-engine <subcommand>`, or a `Skill` or `SlashCommand` call names one of the six `/wiki-*` commands.

Known errors. A false negative: a repo-wide search such as `grep -r x .` also
reads the wiki, but it does not count. A false positive: a Bash command that
searches for the word, such as `grep -rn wiki src`, counts as consulted.
Read the transcripts in `runs/` before you rely on a small difference.

Cost is the result event's `total_cost_usd`. Tokens are the sum of the
result event's `usage` fields (input, cache read, cache write, output). The
report gives the numbers for the skill's decision rule, but it does not apply
the rule.

## Output and committing results

The default output directory is
`<repo>/.wiki-engine/benchmark/headless/<date>-<commit>/`. `wiki-engine init`
adds `.wiki-engine/` to an existing `.gitignore`.

To commit results, pass `--out` to a tracked path. The script writes a
`.gitignore` there that excludes `runs/`. Raw transcripts can contain any
file content that a session read, so do not commit them. The committable
files are `meta.json`, `grades/`, `rows.json` and `report.md`. These contain
the questions, the answer keys and the answers. `meta.json` and `rows.json`
also contain absolute local paths. Review them before you commit. If the output
directory is inside the repository, `run` excludes it from the workspaces.
Results that you committed earlier are in `HEAD`, so later runs fail the leak
check unless you pass `--exclude <that path>`.

## Question file (JSON)

The default is `questions.md`. A JSON file is also accepted:

```json
[{"id": "W1", "cat": "why", "q": "Why ...?",
  "key": ["key point 1", "key point 2"],
  "wrong": ["typical wrong claim"],
  "sources": "path/to/file (function)"}]
```

`id`, `cat`, `q` and `key` are required. An `id` must not contain `_`. Check
each key against the code and git, not against the wiki. If the file is inside
the repository, the run removes it from the workspaces (step 3 above).

## Re-running

`run` skips each run that already has a result in `runs/`. To add
repetitions or questions, run again with the same `--out` and the same
settings. The run stops if the commit, the question file, the model, the
disabled plugins, the budget, or the arms differ from the first `run`. Use a
new `--out` for a different setup.

## Tests

```bash
make bench-test        # in the wiki-engine repository
python3 -m unittest discover .claude/skills/wiki-benchmark/headless   # in any repository
```

The tests use no network, no `claude` binary and no model call. Run them after
you change `bench.py`.
