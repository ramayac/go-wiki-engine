"""Unit tests for bench.py. Hermetic: no network, no `claude` binary, no model call.
Run from the repository root: python3 -m unittest discover .claude/skills/wiki-benchmark/headless"""
import json
import os
import shutil
import subprocess
import sys
import tempfile
import unittest

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import bench  # noqa: E402


class Detector(unittest.TestCase):
    pats = bench.wiki_patterns('wiki')

    def hits(self, name, inp):
        return bench.classify_tool_use(name, inp, self.pats)

    def test_bare_wiki_word_in_bash(self):
        # A bare word, not a path: `wiki` as a grep path.
        self.assertIn('consulted', self.hits('Bash', {'command': 'grep -rli datadog wiki docs'}))

    def test_wiki_at_end_of_command(self):
        self.assertIn('consulted', self.hits('Bash', {'command': 'ls wiki'}))

    def test_read_absolute_wiki_path(self):
        h = self.hits('Read', {'file_path': '/tmp/ws/FULL/wiki/concepts/gotchas.md'})
        self.assertEqual(h, {'consulted'})

    def test_grep_path_field(self):
        self.assertIn('consulted', self.hits('Grep', {'pattern': 'x', 'path': 'wiki'}))

    def test_index_read(self):
        self.assertEqual(self.hits('Read', {'file_path': '/ws/wiki/index.md'}), {'consulted', 'index'})

    def test_negatives(self):
        for cmd in ('cat docs/wikipedia.md', 'ls mywiki/', 'grep -r wikiLink src', 'echo wiki-engine:shim:v2'):
            self.assertEqual(self.hits('Bash', {'command': cmd}), set(), cmd)

    def test_engine_calls(self):
        for cmd in ('wiki-engine search "x"', './bin/wiki-engine lint',
                    'cd /ws && wiki-engine graph --include-sources', 'wiki-engine impact a.go'):
            self.assertIn('engine', self.hits('Bash', {'command': cmd}), cmd)

    def test_known_false_positives_removed(self):
        self.assertEqual(self.hits('Grep', {'pattern': 'wiki', 'path': 'src'}), set())
        self.assertEqual(self.hits('Skill', {'skill': 'wiki-benchmark'}), set())
        self.assertEqual(self.hits('Read', {'file_path': '/r/mywiki/index.md'}), set())

    def test_engine_name_alone_is_not_a_call(self):
        self.assertNotIn('engine', self.hits('Bash', {'command': 'grep -rn wiki-engine README.md'}))

    def test_slash_command(self):
        self.assertIn('engine', self.hits('Skill', {'skill': 'wiki-query', 'args': 'x'}))

    def test_custom_wiki_dir(self):
        p = bench.wiki_patterns('docs/kb')
        self.assertIn('consulted', bench.classify_tool_use('Bash', {'command': 'grep -r x docs/kb'}, p))
        self.assertEqual(bench.classify_tool_use('Bash', {'command': 'grep -r x wiki'}, p), set())


class Questions(unittest.TestCase):
    def template(self):
        return bench._read(bench.DEFAULT_QUESTIONS)

    def test_template_parses_to_three_examples(self):
        qs = bench.parse_questions_md(self.template())
        self.assertEqual([q['id'] for q in qs], ['Q1', 'Q2', 'Q3'])
        self.assertEqual({q['cat'] for q in qs}, {'code', 'why', 'both'})
        for q in qs:
            self.assertTrue(q['q'].endswith('?'), q['id'])
            self.assertTrue(q['key'], q['id'])
            self.assertTrue(q['wrong'], q['id'])
            self.assertTrue(q['sources'], q['id'])

    def test_template_marker_blocks_a_run(self):
        self.assertIn(bench.PLACEHOLDER, self.template())
        with self.assertRaises(SystemExit):
            bench.load_questions(bench.DEFAULT_QUESTIONS)

    def test_edited_file_loads(self):
        text = self.template().replace(bench.PLACEHOLDER, '')
        with tempfile.TemporaryDirectory() as d:
            p = os.path.join(d, 'questions.md')
            bench._write(p, text)
            self.assertEqual(len(bench.load_questions(p)), 3)

    def test_multiline_key_point_is_joined(self):
        text = ('## Q1 - code\n**Question:** Why?\n\n**Key points:**\n1. First\n   continued here.\n2. Second\n\n'
                '**Typical wrong claims:** a; b\n\n**Sources:** `x.go`\n')
        q = bench.parse_questions_md(text)[0]
        self.assertEqual(q['key'], ['First continued here.', 'Second'])
        self.assertEqual(q['wrong'], ['a', 'b'])

    def test_json_rejects_underscore_ids(self):
        with tempfile.NamedTemporaryFile('w', suffix='.json', delete=False) as f:
            json.dump([{'id': 'A_1', 'cat': 'code', 'q': 'x?', 'key': ['y']}], f)
        with self.assertRaises(SystemExit):
            bench.load_questions(f.name)
        os.remove(f.name)


class WikiDir(unittest.TestCase):
    def repo_with(self, wikirc):
        d = tempfile.mkdtemp()
        self.addCleanup(shutil.rmtree, d)
        if wikirc is not None:
            bench._write(os.path.join(d, '.wikirc'), wikirc)
        for c in (['init', '-q'], ['add', '-A'],
                  ['-c', 'user.name=t', '-c', 'user.email=t@t', '-c', 'commit.gpgsign=false',
                   'commit', '-qm', 'x', '--allow-empty']):
            subprocess.run(['git'] + c, cwd=d, check=True)
        return d

    def test_default_is_wiki(self):
        self.assertEqual(bench.read_wiki_dir(self.repo_with(None), 'HEAD'), 'wiki')
        self.assertEqual(bench.read_wiki_dir(self.repo_with('log_lines = 5\n'), 'HEAD'), 'wiki')

    def test_reads_wiki_dir_from_wikirc(self):
        self.assertEqual(bench.read_wiki_dir(self.repo_with('wiki_dir = "docs"\nignore = []\n'), 'HEAD'), 'docs')

    def test_unsafe_wiki_dir_stops(self):
        for bad in ('../x', 'a/b', '/abs', '..', ''):
            with self.assertRaises(SystemExit, msg=bad):
                bench.read_wiki_dir(self.repo_with(f'wiki_dir = "{bad}"\n'), 'HEAD')


class Workspace(unittest.TestCase):
    def test_strip_shim_block(self):
        text = ('intro\n\n<!-- wiki-engine:shim:v2 -->\n# Claude Instructions\nStart here: wiki/index.md\n'
                '<!-- /wiki-engine:shim -->\n\noutro\n')
        out = bench.strip_shim_block(text)
        self.assertNotIn('wiki-engine:shim', out)
        self.assertNotIn('index.md', out)
        self.assertIn('intro', out)
        self.assertIn('outro', out)

    def test_strip_shim_block_only_block_leaves_blank(self):
        text = '<!-- wiki-engine:shim:v2 -->\nx\n<!-- /wiki-engine:shim -->\n'
        self.assertEqual(bench.strip_shim_block(text).strip(), '')

    def test_leak_check(self):
        with tempfile.TemporaryDirectory() as d:
            bench._write(os.path.join(d, 'a.md'), 'Which command-line\nflags does it accept?')
            with self.assertRaises(SystemExit):
                bench.leak_check(d, [{'q': 'Which command-line flags does it accept?'}])
            bench.leak_check(d, [{'q': 'Something else entirely?'}])


class Excludes(unittest.TestCase):
    def test_safe_relpath_rejects_escapes(self):
        for bad in ('/abs/path', '..', '../x', 'a/../../x', '.', ''):
            with self.assertRaises(SystemExit, msg=bad):
                bench.safe_relpath(bad)
        self.assertEqual(bench.safe_relpath('results/run1/'), os.path.join('results', 'run1'))

    def test_question_excludes(self):
        r = '/repo'
        self.assertEqual(bench.question_excludes('/elsewhere/q.json', r), [])
        self.assertEqual(bench.question_excludes('/repo/q.json', r), ['q.json'])
        self.assertEqual(bench.question_excludes('/repo/docs/q.json', r), [os.path.join('docs', 'q.json')])
        self.assertEqual(bench.question_excludes('/repo/.claude/skills/wiki-benchmark/questions.md', r),
                         [os.path.join('.claude', 'skills', 'wiki-benchmark')])


SHIM = '<!-- wiki-engine:shim:v2 -->\n# Claude Instructions\nStart here: {w}/index.md\n<!-- /wiki-engine:shim -->\n'


class BuildWorkspaces(unittest.TestCase):
    wiki = 'wiki'
    wikirc = None

    def setUp(self):
        self.tmp = tempfile.mkdtemp()
        self.repo = os.path.join(self.tmp, 'repo')
        w = self.wiki
        files = {
            'CLAUDE.md': 'intro\n\n' + SHIM.format(w=w),
            'AGENTS.md': SHIM.format(w=w),
            w + '/index.md': '# index\n', 'src/a.go': 'package a\n',
            '.wiki-instructions/ingest.md': 'i', '.wikirc': 'x = 1\n',
            '.pi/skills/wiki/SKILL.md': 's',
            '.github/prompts/wiki-query.prompt.md': 'q', '.github/prompts/other.prompt.md': 'o',
            '.github/instructions/wiki-maintainer.instructions.md': 'm',
            '.claude/commands/wiki-query.md': 'q', '.claude/commands/other.md': 'o',
            '.claude/skills/wiki-benchmark/questions.md': 'Which flags does lint accept?',
            'keep/k.txt': 'k',
        }
        if self.wikirc:
            files['.wikirc'] = self.wikirc
        for rel, text in files.items():
            os.makedirs(os.path.dirname(os.path.join(self.repo, rel)), exist_ok=True)
            bench._write(os.path.join(self.repo, rel), text)
        for c in (['init', '-q'], ['add', '-A'],
                  ['-c', 'user.name=t', '-c', 'user.email=t@t', '-c', 'commit.gpgsign=false', 'commit', '-qm', 'x']):
            subprocess.run(['git'] + c, cwd=self.repo, check=True)

    def tearDown(self):
        shutil.rmtree(self.tmp)

    def exists(self, ws, rel):
        return os.path.exists(os.path.join(ws, rel))

    def test_nowiki_and_full(self):
        qs = [{'q': 'Which flags does lint accept?'}]
        ex = bench.question_excludes(os.path.join(self.repo, '.claude/skills/wiki-benchmark/questions.md'), self.repo)
        root, ws = bench.build_workspaces(self.repo, 'HEAD', ['A', 'B'], self.wiki, ex, None, qs)
        try:
            a, b = ws['A'], ws['B']
            for rel in (self.wiki, '.wiki-instructions', '.wikirc', '.pi/skills/wiki', 'AGENTS.md',
                        '.claude/commands/wiki-query.md', '.github/prompts/wiki-query.prompt.md',
                        '.github/instructions/wiki-maintainer.instructions.md'):
                self.assertFalse(self.exists(a, rel), rel)
            self.assertTrue(self.exists(a, '.claude/commands/other.md'))
            self.assertTrue(self.exists(a, '.github/prompts/other.prompt.md'))
            self.assertTrue(self.exists(a, 'src/a.go'))
            cm = bench._read(os.path.join(a, 'CLAUDE.md'))
            self.assertNotIn('wiki-engine:shim', cm)
            self.assertIn('intro', cm)
            for rel in (self.wiki + '/index.md', 'AGENTS.md', '.wiki-instructions/ingest.md', '.wikirc',
                        '.claude/commands/wiki-query.md'):
                self.assertTrue(self.exists(b, rel), rel)
            self.assertIn('wiki-engine:shim', bench._read(os.path.join(b, 'CLAUDE.md')))
            for w in (a, b):
                self.assertFalse(self.exists(w, '.claude/skills/wiki-benchmark'))
                log = subprocess.run(['git', 'log', '--oneline'], cwd=w, capture_output=True, text=True).stdout
                self.assertEqual(len(log.splitlines()), 1)
        finally:
            shutil.rmtree(root)

    def test_skill_dir_is_dropped_even_without_question_excludes(self):
        root, ws = bench.build_workspaces(self.repo, 'HEAD', ['B'], self.wiki, [bench.SKILL_DIR], None, [])
        try:
            self.assertFalse(self.exists(ws['B'], '.claude/skills/wiki-benchmark'))
        finally:
            shutil.rmtree(root)

    def test_leak_stops_build(self):
        with self.assertRaises(SystemExit):
            bench.build_workspaces(self.repo, 'HEAD', ['B'], self.wiki, [], None, [{'q': 'Which flags does lint accept?'}])

    def test_absolute_exclude_never_touches_the_real_repo(self):
        with self.assertRaises(SystemExit):
            bench.safe_relpath(os.path.join(self.repo, 'keep'))
        self.assertTrue(os.path.exists(os.path.join(self.repo, 'keep/k.txt')))


class BuildWorkspacesCustomDir(BuildWorkspaces):
    wiki = 'docs-kb'
    wikirc = 'wiki_dir = "docs-kb"\n'

    def test_wiki_dir_comes_from_wikirc(self):
        self.assertEqual(bench.read_wiki_dir(self.repo, 'HEAD'), 'docs-kb')


class ParseRun(unittest.TestCase):
    def test_counts_and_result(self):
        lines = [
            {'type': 'system', 'subtype': 'init', 'claude_code_version': '9.9', 'model': 'm',
             'mcp_servers': [], 'plugins': [], 'skills': [], 'cwd': '/ws'},
            {'type': 'assistant', 'message': {'content': [
                {'type': 'tool_use', 'name': 'Bash', 'input': {'command': 'grep -rl x wiki docs'}},
                {'type': 'tool_use', 'name': 'Read', 'input': {'file_path': '/ws/src/a.go'}}]}},
            {'type': 'result', 'result': 'ANSWER: y', 'total_cost_usd': 0.1, 'num_turns': 3,
             'duration_ms': 2000, 'is_error': False, 'usage': {'input_tokens': 5, 'output_tokens': 7}},
        ]
        with tempfile.NamedTemporaryFile('w', suffix='.jsonl', delete=False) as f:
            f.write('\n'.join(json.dumps(x) for x in lines) + '\nnot json\n')
        r = bench.parse_run(f.name, bench.wiki_patterns('wiki'))
        os.remove(f.name)
        self.assertEqual((r['tools'], r['consulted'], r['index'], r['engine']), (2, 1, 0, 0))
        self.assertEqual((r['result'], r['cost'], r['turns'], r['tokens']), ('ANSWER: y', 0.1, 3, 12))
        self.assertEqual(r['init']['claude_code_version'], '9.9')


if __name__ == '__main__':
    unittest.main()
