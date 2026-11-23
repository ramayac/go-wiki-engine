#!/usr/bin/env python3
"""Headless Claude Code wiki benchmark.

Runs each question as a real `claude -p` session in a throwaway copy of the
target repository, grades the answers blind, and reports per arm. See
README.md in this directory for the arms, the isolation flags and the caveats.

Usage:
  bench.py run    [--repo PATH] [--questions FILE] [--model M] [--arms A,B,C]
                  [--reps N] [--jobs N] [--only Q1,Q2] [--max-budget-usd N]
                  [--out DIR] [--disable-plugin NAME@SOURCE ...]
                  [--hook-settings FILE] [--exclude PATH ...] [--claude-bin BIN]
  bench.py grade  [--out DIR | --repo PATH] [--grader-model M]
  bench.py report [--out DIR | --repo PATH]
  bench.py clean  [--out DIR | --repo PATH]

Python 3.9+, standard library only.
"""
import argparse
import datetime
import json
import os
import random
import re
import shutil
import statistics as st
import string
import subprocess
import sys
import tempfile
from concurrent.futures import ThreadPoolExecutor

HERE = os.path.dirname(os.path.abspath(__file__))
DEFAULT_QUESTIONS = os.path.normpath(os.path.join(HERE, '..', 'questions.md'))
MARKER = '.wiki-bench-workspace'
# The skill's own directory. Every workspace drops it, so no arm can read the answer keys.
SKILL_DIR = '.claude/skills/wiki-benchmark'
# A questions.md that still holds this line is the unedited template.
PLACEHOLDER = '<!-- TEMPLATE: replace these example questions -->'
ENGINE_SUBCOMMANDS = ('search', 'relevant', 'context', 'graph', 'summary', 'check', 'impact', 'lint',
                      'health', 'stats', 'list', 'headings', 'log-tail', 'schema')
ENTRY_FILES = ('CLAUDE.md', 'AGENTS.md', '.github/copilot-instructions.md')
WIKI_COMMANDS = ('wiki-seed', 'wiki-query', 'wiki-plan', 'wiki-save', 'wiki-ingest', 'wiki-upgrade')
# The fenced block that `wiki-engine init` writes into CLAUDE.md and AGENTS.md.
SHIM_BLOCK = re.compile(r'\n*<!-- wiki-engine:shim:v\d+ -->.*?<!-- /wiki-engine:shim -->\n*', re.S)
# One path segment: the same rule as the engine's wiki_dir check.
WIKI_DIR_OK = re.compile(r'^[A-Za-z0-9][A-Za-z0-9._-]*$')
# Wiki tooling files that the engine installs. Arm A has none of them.
WIKI_TOOLING = ('.wiki-instructions', '.wikirc', '.pi/skills/wiki',
                '.github/instructions/wiki-maintainer.instructions.md')
WIKI_TOOLING_GLOBS = (('.claude/commands', 'wiki-'), ('.github/prompts', 'wiki-'))
ANSWER_FORMAT = ('\n\nDo not edit any file. Reply in exactly this format:\n'
                 'ANSWER: <2 to 8 sentences>\n'
                 'FILES: <files you relied on, repo-relative, comma-separated>\n'
                 'CONFIDENCE: <high | medium | low>')


def _read(path, **kw):
    with open(path, encoding='utf-8', errors=kw.pop('errors', 'strict')) as f:
        return f.read()


def _write(path, text):
    with open(path, 'w', encoding='utf-8') as f:
        f.write(text)


def _load_json(path):
    with open(path, encoding='utf-8') as f:
        return json.load(f)


def _dump_json(obj, path):
    with open(path, 'w', encoding='utf-8') as f:
        json.dump(obj, f, indent=1)


# ---------------------------------------------------------------- questions

def load_questions(path):
    """Load questions from JSON (list of objects) or from the skill's questions.md."""
    text = _read(path)
    if PLACEHOLDER in text:
        raise SystemExit(f'{path} still holds the template marker. Replace the example questions '
                         'with real ones and delete the marker line first.')
    qs = json.loads(text) if path.endswith('.json') else parse_questions_md(text)
    for q in qs:
        for field in ('id', 'cat', 'q', 'key'):
            if not q.get(field):
                raise SystemExit(f'{path}: question {q.get("id", "?")} has no {field!r}')
        if '_' in q['id']:
            raise SystemExit(f'{path}: question id {q["id"]!r} must not contain "_"')
        q.setdefault('wrong', [])
        q.setdefault('sources', '')
    ids = [q['id'] for q in qs]
    if len(ids) != len(set(ids)):
        raise SystemExit(f'{path}: duplicate question ids')
    return qs


def _squash(s):
    return re.sub(r'\s+', ' ', s).strip()


def parse_questions_md(text):
    """Parse the `## Q1 — cat` / **Question:** / **Key points:** format of questions.md."""
    out = []
    blocks = re.split(r'^## ', text, flags=re.M)[1:]
    for b in blocks:
        head, _, body = b.partition('\n')
        m = re.match(r'(\S+)\s+[—-]+\s+(\w+)', head)
        if not m:
            continue

        def field(name, nxt):
            fm = re.search(r'\*\*' + name + r':\*\*(.*?)(?=\*\*(?:' + nxt + r'):\*\*|^---\s*$|\Z)',
                           body, flags=re.S | re.M)
            return fm.group(1).strip() if fm else ''

        q = field('Question', 'Key points')
        keys_raw = field('Key points', 'Typical wrong claims')
        wrong = field('Typical wrong claims', 'Sources')
        sources = field('Sources', 'NONE')
        keys = [_squash(k) for k in re.split(r'^\s*\d+\.\s+', keys_raw, flags=re.M) if k.strip()]
        out.append({'id': m.group(1), 'cat': m.group(2), 'q': _squash(q), 'key': keys,
                    'wrong': [_squash(w) for w in wrong.split(';') if w.strip()],
                    'sources': _squash(sources)})
    return out


# ---------------------------------------------------------------- detector

def wiki_patterns(wiki_dir):
    w = re.escape(wiki_dir.strip('/'))
    return {
        # Corrected detector: a path segment or bare word equal to the wiki dir.
        'consulted': re.compile(r'(^|[\s/"\'=])' + w + r'(/|\s|$|")'),
        'index': re.compile(r'(^|[\s/"\'=])' + w + r'/index\.md'),
        'engine': re.compile(r'(^|[\s;&|(`/])wiki-engine\s+(' + '|'.join(ENGINE_SUBCOMMANDS) + r')\b'),
    }


def classify_tool_use(name, tool_input, pats):
    """Return the set of wiki signals one tool_use carries."""
    # A Grep pattern is search text, not a path, so it does not count.
    s = json.dumps({k: v for k, v in tool_input.items() if not (name == 'Grep' and k == 'pattern')})
    hits = set()
    if pats['consulted'].search(s):
        hits.add('consulted')
    if pats['index'].search(s):
        hits.add('index')
    if name == 'Bash' and pats['engine'].search(tool_input.get('command', '')):
        hits.add('engine')
    if name in ('Skill', 'SlashCommand') and re.search(r'(^|["/\s])(' + '|'.join(WIKI_COMMANDS) + r')\b', s):
        hits.add('engine')
    return hits


def parse_run(path, pats):
    r = {'tools': 0, 'consulted': 0, 'index': 0, 'engine': 0, 'agents': 0,
         'result': '', 'cost': None, 'turns': None, 'ms': None, 'is_error': None,
         'init': None, 'tokens': 0}
    if not os.path.exists(path):
        return r
    for line in _read(path, errors='replace').splitlines():
        try:
            o = json.loads(line)
        except ValueError:
            continue
        if o.get('type') == 'system' and o.get('subtype') == 'init' and r['init'] is None:
            r['init'] = {k: o.get(k) for k in ('claude_code_version', 'model', 'mcp_servers', 'plugins', 'skills', 'cwd')}
        if o.get('type') == 'result':
            u = o.get('usage') or {}
            r.update(result=o.get('result') or '', cost=o.get('total_cost_usd'), turns=o.get('num_turns'),
                     ms=o.get('duration_ms'), is_error=o.get('is_error'),
                     tokens=sum(u.get(k, 0) or 0 for k in ('input_tokens', 'cache_read_input_tokens',
                                                           'cache_creation_input_tokens', 'output_tokens')))
        if o.get('type') != 'assistant':
            continue
        for x in (o.get('message') or {}).get('content') or []:
            if not isinstance(x, dict) or x.get('type') != 'tool_use':
                continue
            r['tools'] += 1
            if x.get('name') in ('Agent', 'Task'):
                r['agents'] += 1
            for h in classify_tool_use(x.get('name'), x.get('input') or {}, pats):
                r[h] += 1
    return r


# ---------------------------------------------------------------- workspaces

def git(repo, *args, **kw):
    return subprocess.run(['git', '-C', repo] + list(args), check=True, capture_output=True, **kw).stdout


def read_wiki_dir(repo, commit):
    """Read wiki_dir from .wikirc at the commit (default `wiki`). Stop on an unsafe value."""
    try:
        rc = git(repo, 'show', f'{commit}:.wikirc', text=True)
    except subprocess.CalledProcessError:
        return 'wiki'
    m = re.search(r'^\s*wiki_dir\s*=\s*"([^"]*)"', rc, flags=re.M)
    name = m.group(1) if m else 'wiki'
    if not WIKI_DIR_OK.match(name):
        raise SystemExit(f'.wikirc: wiki_dir {name!r} is not one safe path segment')
    return name


def strip_shim_block(text):
    """Remove the fenced wiki-engine block. Same pattern as ../prepare.sh."""
    return SHIM_BLOCK.sub('\n', text)


def ancestor_instruction_files(path):
    found, p = [], os.path.dirname(os.path.abspath(path))
    while True:
        for name in ('CLAUDE.md', 'CLAUDE.local.md', os.path.join('.claude', 'CLAUDE.md')):
            if os.path.exists(os.path.join(p, name)):
                found.append(os.path.join(p, name))
        parent = os.path.dirname(p)
        if parent == p:
            return found
        p = parent


def leak_check(ws, questions):
    needles = [_squash(q['q'])[:80] for q in questions]
    for root, dirs, files in os.walk(ws):
        dirs[:] = [d for d in dirs if d != '.git']
        for f in files:
            fp = os.path.join(root, f)
            try:
                text = _squash(_read(fp, errors='ignore'))
            except OSError:
                continue
            for n in needles:
                if n and n in text:
                    raise SystemExit(f'leak: {fp} contains question text "{n[:50]}...". '
                                     'Remove it with --exclude, or move the questions file.')


def safe_relpath(path):
    """Return a normalized path inside the workspace, or stop. Never lets a path escape it."""
    norm = os.path.normpath(path)
    if os.path.isabs(norm) or norm == '.' or norm == '..' or norm.startswith('..' + os.sep):
        raise SystemExit(f'--exclude {path!r}: use a path relative to the repository root, inside it')
    return norm


def question_excludes(qpath, repo):
    """Paths to remove so no arm can read the answer keys: the file, or its whole skill directory."""
    if not qpath.startswith(repo + os.sep):
        return []
    parts = os.path.relpath(qpath, repo).split(os.sep)
    if parts[:2] == ['.claude', 'skills'] and len(parts) > 3:
        return [os.path.join(*parts[:3])]
    return [os.path.join(*parts)]


def build_workspaces(repo, commit, arms, wiki_dir, excludes, hook_settings, questions):
    root = tempfile.mkdtemp(prefix='wiki-bench-')
    try:
        return _build_workspaces(root, repo, commit, arms, wiki_dir, excludes, hook_settings, questions)
    except BaseException:
        shutil.rmtree(root, ignore_errors=True)
        raise


def strip_wiki_from(ws, wiki_dir):
    """Turn a copy of the repository into its no-wiki twin (arm A)."""
    shutil.rmtree(os.path.join(ws, wiki_dir), ignore_errors=True)
    for rel in WIKI_TOOLING:
        p = os.path.join(ws, rel)
        if os.path.isdir(p) and not os.path.islink(p):
            shutil.rmtree(p)
        elif os.path.lexists(p):
            os.remove(p)
    for d, prefix in WIKI_TOOLING_GLOBS:
        dd = os.path.join(ws, d)
        if os.path.isdir(dd):
            for f in os.listdir(dd):
                if f.startswith(prefix):
                    os.remove(os.path.join(dd, f))
    for name in ('CLAUDE.md', 'AGENTS.md'):
        p = os.path.join(ws, name)
        if not os.path.isfile(p):
            continue
        text = strip_shim_block(_read(p, errors='ignore'))
        if text.strip():
            _write(p, text)
        else:
            os.remove(p)  # The file held only the wiki block.
    for ef in ENTRY_FILES:
        p = os.path.join(ws, ef)
        if os.path.exists(p) and re.search(re.escape(wiki_dir) + '/', _read(p, errors='ignore')):
            print(f'warning: NOWIKI {ef} still mentions {wiki_dir}/ (arm A may chase dangling references)',
                  file=sys.stderr)


def _build_workspaces(root, repo, commit, arms, wiki_dir, excludes, hook_settings, questions):
    _write(os.path.join(root, MARKER), '')
    found = ancestor_instruction_files(os.path.join(root, 'x'))
    if found:
        raise SystemExit(f'workspace root {root} has instruction files in its ancestors: {found}. Set TMPDIR elsewhere.')
    archive = git(repo, 'archive', commit)
    names = {'A': 'NOWIKI', 'B': 'FULL', 'C': 'FULL', 'D': 'FULL_HOOK'}
    for name in sorted({names[a] for a in arms} | {'FULL'}):
        ws = os.path.join(root, name)
        os.makedirs(ws)
        subprocess.run(['tar', '-x', '-C', ws], input=archive, check=True)
        for ex in excludes:
            p = os.path.join(ws, safe_relpath(ex))
            if os.path.isdir(p) and not os.path.islink(p):
                shutil.rmtree(p)
            elif os.path.lexists(p):
                os.remove(p)
        if name == 'NOWIKI':
            strip_wiki_from(ws, wiki_dir)
        if name == 'FULL_HOOK':
            os.makedirs(os.path.join(ws, '.claude'), exist_ok=True)
            shutil.copy(hook_settings, os.path.join(ws, '.claude', 'settings.local.json'))
        leak_check(ws, questions)
        subprocess.run(['git', 'init', '-q'], cwd=ws, check=True)
        subprocess.run(['git', 'add', '-A'], cwd=ws, check=True)
        subprocess.run(['git', '-c', 'user.name=wiki-bench', '-c', 'user.email=wiki-bench@localhost',
                        '-c', 'commit.gpgsign=false', 'commit', '-qm', f'snapshot {commit}'], cwd=ws, check=True)
    if 'A' in arms and os.path.exists(os.path.join(root, 'NOWIKI', wiki_dir)):
        raise SystemExit('NOWIKI still has the wiki directory')
    return root, {a: os.path.join(root, names[a]) for a in arms}


# ---------------------------------------------------------------- claude

def base_cmd(meta):
    cmd = [meta['claude_bin'], '-p', '--output-format', 'stream-json', '--verbose',
           '--setting-sources', 'project,local', '--strict-mcp-config', '--no-session-persistence']
    if meta['disable_plugins']:
        cmd += ['--settings', json.dumps({'enabledPlugins': {p: False for p in meta['disable_plugins']}})]
    return cmd + ['--disallowedTools', 'Edit', 'Write', 'NotebookEdit',
                  '--max-budget-usd', str(meta['max_budget_usd'])]


def claude(meta, model, prompt, cwd, out, timeout=1200):
    with open(out + '.tmp', 'w') as f:
        p = subprocess.run(base_cmd(meta) + ['--model', model, prompt], cwd=cwd, stdin=subprocess.DEVNULL,
                           stdout=f, stderr=subprocess.STDOUT, timeout=timeout)
    os.replace(out + '.tmp', out)
    return p.returncode


def finished(path):
    if not os.path.exists(path):
        return False
    return re.search(r'"type"\s*:\s*"result"', _read(path, errors='replace')) is not None


# ---------------------------------------------------------------- commands

def default_out(repo, commit):
    return os.path.join(repo, '.wiki-engine', 'benchmark', 'headless',
                        f'{datetime.date.today().isoformat()}-{commit}')


def latest_out(repo):
    base = os.path.join(os.path.abspath(repo), '.wiki-engine', 'benchmark', 'headless')
    dirs = [os.path.join(base, d) for d in os.listdir(base)] if os.path.isdir(base) else []
    dirs = [d for d in dirs if os.path.exists(os.path.join(d, 'meta.json'))]
    if not dirs:
        raise SystemExit(f'no benchmark output under {base}; pass --out or --repo')
    return max(dirs, key=os.path.getmtime)


def load_meta(out):
    return _load_json(os.path.join(out, 'meta.json'))


def cmd_run(a):
    repo = os.path.abspath(a.repo)
    commit = git(repo, 'rev-parse', '--short', 'HEAD', text=True).strip()
    if not a.questions:
        if os.path.commonpath([HERE, repo]) != repo:
            raise SystemExit('--questions is required: the default questions.md belongs to the repository '
                             'that holds this script, not to --repo')
        a.questions = DEFAULT_QUESTIONS
    questions = load_questions(os.path.abspath(a.questions))
    arms = a.arms.split(',')
    if 'D' in arms and not a.hook_settings:
        raise SystemExit('arm D needs --hook-settings FILE')
    if set(arms) - set('ABCD'):
        raise SystemExit('arms must be from A,B,C,D')
    out = os.path.abspath(a.out or default_out(repo, commit))
    os.makedirs(os.path.join(out, 'runs'), exist_ok=True)
    _write(os.path.join(out, '.gitignore'),
           '# Raw transcripts can contain file contents read from the repository.\nruns/\n')
    qpath = os.path.abspath(a.questions)
    excludes = [safe_relpath(x) for x in a.exclude] + [os.path.normpath(SKILL_DIR)] + question_excludes(qpath, repo)
    if out.startswith(repo + os.sep):
        excludes.append(os.path.relpath(out, repo))
    wiki_dir = read_wiki_dir(repo, commit)
    meta_path = os.path.join(out, 'meta.json')
    meta = load_meta(out) if os.path.exists(meta_path) else {}
    if meta and not os.path.isdir(meta.get('workspace_root', '')):
        raise SystemExit(f'{out} exists but its workspaces are gone; use a new --out')
    if meta:
        want = {'commit': commit, 'questions_file': qpath, 'model': a.model,
                'disable_plugins': a.disable_plugin, 'max_budget_usd': a.max_budget_usd}
        diff = [k for k, v in want.items() if meta.get(k) != v]
        if set(arms) - set(meta['arms']):
            diff.append('arms')
        if diff:
            raise SystemExit(f'{out} was started with different {", ".join(diff)}; use a new --out')
    if not meta:
        root, ws = build_workspaces(repo, commit, arms, wiki_dir, excludes, a.hook_settings, questions)
        meta = {'repo': repo, 'commit': commit, 'questions_file': qpath, 'questions': questions,
                'wiki_dir': wiki_dir, 'model': a.model, 'arms': arms, 'workspace_root': root,
                'workspaces': ws, 'full': os.path.join(root, 'FULL'), 'excludes': excludes,
                'disable_plugins': a.disable_plugin, 'max_budget_usd': a.max_budget_usd,
                'claude_bin': a.claude_bin, 'started': datetime.datetime.now().isoformat(timespec='seconds')}
        _dump_json(meta, meta_path)
    print(f'OUT={out}\nWORKSPACES={meta["workspace_root"]}', flush=True)
    qs = [q for q in questions if not a.only or q['id'] in a.only.split(',')]
    prefix = {'C': f'Start at {wiki_dir}/index.md; read code to confirm.\n\n'}
    jobs = [(q, arm, r) for r in range(1, a.reps + 1) for q in qs for arm in arms]

    def one(job):
        q, arm, rep = job
        path = os.path.join(out, 'runs', f'{q["id"]}_{arm}_{rep}.jsonl')
        if finished(path):
            return path, 'skip'
        try:
            rc = claude(meta, a.model, prefix.get(arm, '') + q['q'] + ANSWER_FORMAT,
                        meta['workspaces'][arm], path)
        except subprocess.TimeoutExpired:
            rc = 'timeout'
        return path, rc

    with ThreadPoolExecutor(a.jobs) as ex:
        for path, rc in ex.map(one, jobs):
            print(os.path.basename(path), rc, flush=True)


def cmd_grade(a):
    out = os.path.abspath(a.out or latest_out(a.repo))
    meta = load_meta(out)
    pats = wiki_patterns(meta['wiki_dir'])
    gdir, rawdir = os.path.join(out, 'grades'), os.path.join(out, 'runs', 'grader')
    os.makedirs(gdir, exist_ok=True)
    os.makedirs(rawdir, exist_ok=True)
    if not os.path.isdir(meta['full']):
        raise SystemExit(f'grader workspace {meta["full"]} is gone; re-run `run` with a fresh --out')
    rng = random.Random(1729)
    for q in meta['questions']:
        gp = os.path.join(gdir, q['id'] + '.json')
        if os.path.exists(gp):
            continue
        items = []
        for f in sorted(os.listdir(os.path.join(out, 'runs'))):
            if f.startswith(q['id'] + '_') and f.endswith('.jsonl'):
                items.append((f[:-6], parse_run(os.path.join(out, 'runs', f), pats)['result']))
        if not items:
            continue
        rng.shuffle(items)
        ids, used = {}, set()
        for k, _ in items:
            while True:
                i = ''.join(rng.choice(string.ascii_uppercase) for _ in range(3))
                if i not in used:
                    ids[k] = i
                    used.add(i)
                    break
        keys = '\n'.join(f'{n + 1}. {k}' for n, k in enumerate(q['key']))
        answers = '\n\n'.join(f'{ids[k]}: {ans}' for k, ans in items)
        prompt = (f'You grade {len(items)} anonymous answers to one question about the repository in the current directory.\n'
                  'Use the key points as the reference. Read the source files to check any claim the key does not cover. '
                  'A key point is hit only if the answer clearly states it (paraphrase is fine). A wrong claim is a stated fact '
                  'that the key or the code contradicts. Do not edit any file.\n\n'
                  f'Question: {q["q"]}\nKey points:\n{keys}\nTypical wrong claims: {"; ".join(q["wrong"])}\n'
                  f'Sources: {q["sources"]}\n\nAnswers:\n{answers}\n\n'
                  'Reply with ONLY one JSON object per line, one line per answer:\n'
                  '{"id": "<ID>", "points_hit": [<numbers>], "wrong_claims": ["<short text>"]}')
        raw = os.path.join(rawdir, q['id'] + '.jsonl')
        claude(meta, a.grader_model, prompt, meta['full'], raw)
        res = parse_run(raw, pats)['result']
        inv = {v: k for k, v in ids.items()}
        grades = {}
        for line in res.splitlines():
            m = re.search(r'\{.*\}', line)
            if not m:
                continue
            try:
                j = json.loads(m.group(0))
            except ValueError:
                continue
            if j.get('id') in inv:
                grades[inv[j['id']]] = j
        _dump_json({'grader_model': a.grader_model, 'ids': ids, 'grades': grades, 'raw': res}, gp)
        print(q['id'], 'graded', len(grades), '/', len(items), flush=True)


def _med(xs):
    xs = [x for x in xs if x is not None]
    return round(st.median(xs), 3) if xs else None


def build_rows(out, meta):
    pats = wiki_patterns(meta['wiki_dir'])
    qs = {q['id']: q for q in meta['questions']}
    rows = []
    for f in sorted(os.listdir(os.path.join(out, 'runs'))):
        if not f.endswith('.jsonl'):
            continue
        qid, arm, rep = f[:-6].split('_')
        if qid not in qs:
            continue
        p = parse_run(os.path.join(out, 'runs', f), pats)
        gp = os.path.join(out, 'grades', qid + '.json')
        g = _load_json(gp)['grades'].get(f[:-6]) if os.path.exists(gp) else None
        nkey = len(qs[qid]['key'])
        hit = {n for n in (g or {}).get('points_hit', []) if isinstance(n, int) and 1 <= n <= nkey}
        rows.append(dict(q=qid, cat=qs[qid]['cat'], arm=arm, rep=int(rep),
                         acc=(100 * len(hit) / nkey) if g else None,
                         wrong=len(g.get('wrong_claims', [])) if g else None,
                         wrong_claims=(g or {}).get('wrong_claims', []), **p))
    return rows


def render_report(rows, meta):
    arms = sorted({r['arm'] for r in rows})
    L = [f'# Headless wiki benchmark — {os.path.basename(meta["repo"])} @ {meta["commit"]}', '',
         f'- Answer model (requested): `{meta["model"]}`. Questions in file: {len(meta["questions"])} '
         f'({len({r["q"] for r in rows})} run) from `{os.path.basename(meta["questions_file"])}`. Wiki dir: `{meta["wiki_dir"]}`.',
         f'- Disabled plugins: {meta["disable_plugins"] or "none"}. Budget cap per run: ${meta["max_budget_usd"]}.', '',
         '| arm | runs | graded | acc % | wrong claims | total $ | median $ | median tokens | median turns | median s | consulted wiki | read index.md | wiki-engine / slash |',
         '|---|---|---|---|---|---|---|---|---|---|---|---|---|']
    summary = {}
    for a in arms:
        rs = [r for r in rows if r['arm'] == a]
        g = [r for r in rs if r['acc'] is not None]
        acc = round(st.mean(r['acc'] for r in g), 1) if g else None
        summary[a] = {'acc': acc, 'median_cost': _med([r['cost'] for r in rs]),
                      'median_tokens': _med([r['tokens'] or None for r in rs]),
                      'wrong': sum(r['wrong'] or 0 for r in g)}
        L.append(f'| {a} | {len(rs)} | {len(g)} | {acc if acc is not None else "-"} | {summary[a]["wrong"]} | '
                 f'{round(sum(r["cost"] or 0 for r in rs), 2)} | {summary[a]["median_cost"]} | {summary[a]["median_tokens"]} | '
                 f'{_med([r["turns"] for r in rs])} | {_med([r["ms"] / 1000 if r["ms"] else None for r in rs])} | '
                 f'{sum(1 for r in rs if r["consulted"])}/{len(rs)} | {sum(1 for r in rs if r["index"])}/{len(rs)} | '
                 f'{sum(1 for r in rs if r["engine"])}/{len(rs)} |')
    L += ['', 'Arms: A = no wiki, B = as shipped, C = wiki first (prompt prefix), D = B plus `--hook-settings`.',
          'Consulted wiki: at least one tool input matches `(^|[\\s/"\'=])<wiki>(/|\\s|$|")`. '
          'Arm A can match too, when it chases a dangling reference. '
          'Known errors: a repo-wide `grep -r x .` reads the wiki but does not count; '
          'a Bash search for the word, such as `grep -rn wiki src`, counts. See README.md.', '']
    if 'A' in summary and summary['A']['acc'] is not None:
        L.append('## Against arm A')
        L.append('')
        L.append('Decision rule from ../SKILL.md: the wiki helps when an arm is at least 15 points above arm A, '
                 'or within 5 points of arm A with at least 25% fewer median tokens. It hurts when an arm makes more '
                 'wrong claims than arm A. This report gives the numbers; a person applies the rule.')
        L.append('')
        base = summary['A']
        for a in arms:
            if a == 'A' or summary[a]['acc'] is None:
                continue
            def rel(k):
                return (f'{round(100 * (summary[a][k] / base[k] - 1)):+d}%'
                        if base[k] and summary[a][k] else '-')
            L.append(f'- {a}: accuracy {summary[a]["acc"] - base["acc"]:+.1f} points, median tokens {rel("median_tokens")}, '
                     f'median cost {rel("median_cost")}, '
                     f'wrong claims {summary[a]["wrong"]} vs {base["wrong"]}.')
        L.append('')
    L += ['## Accuracy by category', '', '| category | ' + ' | '.join(arms) + ' |', '|---|' + '---|' * len(arms)]
    for c in sorted({r['cat'] for r in rows}):
        cells = []
        for a in arms:
            xs = [r['acc'] for r in rows if r['cat'] == c and r['arm'] == a and r['acc'] is not None]
            cells.append(str(round(st.mean(xs), 1)) if xs else '-')
        L.append(f'| {c} | ' + ' | '.join(cells) + ' |')
    L += ['', '## Per question (accuracy per rep)', '', '| question | ' + ' | '.join(arms) + ' |',
          '|---|' + '---|' * len(arms)]
    for q in meta['questions']:
        cells = []
        for a in arms:
            xs = sorted((r for r in rows if r['q'] == q['id'] and r['arm'] == a), key=lambda r: r['rep'])
            cells.append(' '.join('-' if r['acc'] is None else str(round(r['acc'])) for r in xs) or '-')
        if any(c != '-' for c in cells):
            L.append(f'| {q["id"]} ({q["cat"]}) | ' + ' | '.join(cells) + ' |')
    L += ['', '## Isolation record (from each run\'s init event)', '']
    for a in arms:
        rs = [r for r in rows if r['arm'] == a and r['init']]
        vers = sorted({r['init']['claude_code_version'] or '?' for r in rs})
        models = sorted({r['init']['model'] or '?' for r in rs})
        mcp = sorted({json.dumps(s) for r in rs for s in (r['init']['mcp_servers'] or [])})
        plugins = sorted({p.get('source', '?') for r in rs for p in (r['init']['plugins'] or [])
                          if not str(p.get('source', '')).endswith('@builtin')})
        L.append(f'- {a}: Claude Code {", ".join(vers)}; model {", ".join(models)}; '
                 f'MCP servers: {", ".join(mcp) or "none"}; non-builtin plugins: {", ".join(plugins) or "none"}.')
        if mcp or plugins:
            L.append(f'  - **Warning:** arm {a} was not isolated. Disable these with `--disable-plugin` or `--strict-mcp-config`.')
    errs = [r for r in rows if r['is_error'] or r['cost'] is None]
    if errs:
        L += ['', f'**{len(errs)} run(s) ended with an error or no result record:** ' +
              ', '.join(f'{r["q"]}_{r["arm"]}_{r["rep"]}' for r in errs)]
    L += ['', '## Wrong claims (as graded)', '']
    wl = [f'- {r["q"]} {r["arm"]} rep {r["rep"]}: {w}' for r in rows for w in r['wrong_claims']]
    L += wl or ['None.']
    L += ['', 'Caveats: grader output is a model judgment, so check each wrong claim by hand before you quote a count. '
          'Single-turn questions measure retrieval, not long tasks.']
    return '\n'.join(L) + '\n'


def cmd_report(a):
    out = os.path.abspath(a.out or latest_out(a.repo))
    meta = load_meta(out)
    rows = build_rows(out, meta)
    _dump_json(rows, os.path.join(out, 'rows.json'))
    md = render_report(rows, meta)
    _write(os.path.join(out, 'report.md'), md)
    print(md)


def cmd_clean(a):
    out = os.path.abspath(a.out or latest_out(a.repo))
    root = load_meta(out)['workspace_root']
    if not os.path.exists(os.path.join(root, MARKER)):
        raise SystemExit(f'{root} has no {MARKER}; not removing it')
    shutil.rmtree(root)
    print('removed', root)


def main(argv=None):
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    sub = ap.add_subparsers(dest='cmd', required=True)
    r = sub.add_parser('run')
    r.add_argument('--repo', default='.')
    r.add_argument('--questions', help='default: ../questions.md, for the repository that holds this script')
    r.add_argument('--model', default='opus')
    r.add_argument('--arms', default='A,B,C')
    r.add_argument('--reps', type=int, default=2)
    r.add_argument('--jobs', type=int, default=4)
    r.add_argument('--only', default='')
    r.add_argument('--max-budget-usd', type=float, default=3.0)
    r.add_argument('--out')
    r.add_argument('--disable-plugin', action='append', default=[], metavar='NAME@SOURCE')
    r.add_argument('--hook-settings')
    r.add_argument('--exclude', action='append', default=[], metavar='PATH')
    r.add_argument('--claude-bin', default='claude')
    g = sub.add_parser('grade')
    g.add_argument('--grader-model', default='sonnet')
    parsers = [g, sub.add_parser('report'), sub.add_parser('clean')]
    for x in parsers:
        x.add_argument('--out', help='output dir (default: newest under <repo>/.wiki-engine/benchmark/headless)')
        x.add_argument('--repo', default='.')
    a = ap.parse_args(argv)
    {'run': cmd_run, 'grade': cmd_grade, 'report': cmd_report, 'clean': cmd_clean}[a.cmd](a)


if __name__ == '__main__':
    main()
