---
name: wiki-benchmark
description: Measure whether this repo's wiki makes an agent answer questions better. Runs 7 questions through 3 Haiku arms (no wiki, wiki available, wiki first), grades the answers blind with Sonnet, and reports accuracy, wrong claims, tokens, tool calls and time per arm.
---

# Wiki benchmark

This skill tests one claim: an agent that can read the wiki answers questions
about this repo more correctly, or at a lower cost, than an agent that reads
only the code. The wiki directory is `wiki_dir` in `.wikirc` (default `wiki`).
Below, `<wiki_dir>` means that directory.

The questions and answer keys are in [questions.md](questions.md). That file
belongs to the repo owner. The keys must be checked against the code and git,
not against the wiki.

`Explore` agents load no `CLAUDE.md`, so this skill cannot test whether a
session follows the wiki routing in `CLAUDE.md`. To test the routing, use the
headless runner in [headless/](headless/README.md).

This skill spawns agents, so it works only in Claude Code.

## Arms

| Arm | Workspace | Instruction to the agent |
|---|---|---|
| A - no wiki | `NOWIKI` | Answer from the repository files. |
| B - wiki available | `FULL` | The repository has a `<wiki_dir>/` directory. Use it if it helps. |
| C - wiki first | `FULL` | Start at `<wiki_dir>/index.md`. You may read code to confirm. |

`NOWIKI` has no `<wiki_dir>/`, no wiki tooling files, and no fenced
`wiki-engine` block in `CLAUDE.md` or `AGENTS.md`. Both workspaces have no
`.claude/` (so no arm can read the answer keys) and no git history. The rest of
`CLAUDE.md` and `README.md` stays, because a repo without a wiki still has those
files. If that text answers a question, arm A can score on it without the wiki.

## Decision rule (fixed before the run)

- **Accuracy** of an arm = key points hit / key points possible x 100.
- The wiki **helps** when arm B or C is at least 15 points above arm A, or
  within 5 points of arm A with at least 25% fewer median tokens.
- The wiki **hurts** when arm B or C makes more wrong claims than arm A.
- Seven questions are a pilot. Report the result as a direction, not as proof.

Models are fixed: Haiku for the answer agents, Sonnet for the graders.
Do not change them for a run.

## Steps

### 0. Check the question file
Read [questions.md](questions.md).

- If it contains the line `<!-- TEMPLATE: replace these example questions -->`,
  **stop**. Tell the user to replace the 3 example questions with 7 real ones and
  to delete that line. Do not run anything else.
- If it does not hold 7 questions, tell the user. Continue only if the user agrees.

### 1. Check the answer keys
For each question, open the files on its `Sources` line and confirm the key
points are still true at `HEAD`. If a key is out of date, stop and tell the
user. Do not change a key during a run.

### 2. Build the workspaces
```bash
bash .claude/skills/wiki-benchmark/prepare.sh
```
The workspaces come from `HEAD`. Commit the wiki and the questions first.
Record `OUT`, `COMMIT`, `WIKI_DIR`, `FULL` and `NOWIKI` from the output.

**Verify:** the script exits 0. It checks that `NOWIKI` has no wiki directory
and no wiki block, and that no workspace has `.claude/`.

### 3. Tell the user what you will spawn
Say: "Spawning <N x 3> Haiku answer agents (<N> questions x 3 arms) and <N> Sonnet
grader agents. All are read-only." Then continue.

### 4. Run the answer agents
Spawn one agent for each question and arm, with `subagent_type: "Explore"` and
`model: "haiku"`. Each agent gets one question and a fresh context. Send one
message for each question, with its three arms in parallel, so the arms of a
question run under the same conditions.

Use this prompt. Replace `<DIR>`, `<ARM INSTRUCTION>` and `<QUESTION>`:

```
You answer one question about the repository in <DIR>.
Read only files under <DIR>. Do not read any other path. Do not edit any file.
<ARM INSTRUCTION>

Question: <QUESTION>

Reply in exactly this format:
ANSWER: <2 to 6 sentences>
FILES: <the files you used, as paths relative to <DIR>, comma-separated>
CONFIDENCE: <high | medium | low>
```

For each run, record the answer and, from the completion notification, the
subagent tokens, tool uses and duration.

### 5. Grade blind
Give each answer a random three-letter ID. Do not record the arm next to the
ID in anything the grader sees. For each question, spawn one grader with
`subagent_type: "Explore"` and `model: "sonnet"`. Give it the question, the
key points, the typical wrong claims, the `Sources` line, and the three
answers in a shuffled order:

```
You grade three anonymous answers to one question about the repository in <FULL>.
Use the key points as the reference. Read the source files to check any claim the
key does not cover. A wrong claim is a stated fact that the key or the code
contradicts. Do not edit any file.

Question: <QUESTION>
Key points: <KEY POINTS, numbered>
Typical wrong claims: <LIST>
Sources: <SOURCES>

Answers:
<ID1>: <ANSWER + FILES>
<ID2>: <ANSWER + FILES>
<ID3>: <ANSWER + FILES>

Reply with one JSON object per line, one line per answer:
{"id": "<ID>", "points_hit": [<numbers>], "wrong_claims": ["<short text>"], "files_exist": <true|false>}
```

### 6. Report
Map the IDs back to arms. Write the report to
`.wiki-engine/benchmark/<YYYY-MM-DD>-<COMMIT>/results.md`. `wiki-engine init`
adds `.wiki-engine/` to an existing `.gitignore`. Check that the directory is
ignored before you leave the report there. Include:

- One summary table, one row per arm: accuracy, wrong claims, median tokens,
  median tool uses, median duration.
- The same accuracy split by category (`code`, `why`, `both`).
- One table with the score for each question and arm.
- The decision from the rule above, in one sentence.
- All raw answers and grader lines, labeled by arm.

Show the user the summary table, the category split and the decision.

### 7. Clean up
Remove the `OUT` directory that `prepare.sh` printed. Use the literal path,
for example `rm -rf /tmp/wiki-benchmark.AbC123`. Do not build the path with a
command substitution or a variable.

## Rules
- Do not edit files in the repository during a run. The only file you write is
  the report.
- Do not show the arm of an answer to a grader.
- Do not rerun a single arm to improve its score. To repeat the benchmark,
  run all arms again and report both runs.
- Do not edit `questions.md` or any other skill file during a run.
  `wiki-engine sync-prompts` replaces every skill file except `questions.md`.
