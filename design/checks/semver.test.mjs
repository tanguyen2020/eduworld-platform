// F005-AC-13, F005-AC-14: rename/removal needs MAJOR; removed token must have been $deprecated; CHANGELOG entry.
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { execFileSync } from 'node:child_process';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { checkChangelog, checkSemver, compareSnapshots, snapshotAtRef } from './semver.mjs';
import { find } from './test-helpers.mjs';

const snap = (version, entries) => ({ version, tokens: new Map(entries.map(([p, deprecated = false]) => [p, { deprecated }])) });

test('adding tokens needs no MAJOR bump', () => {
  assert.deepEqual(compareSnapshots(snap('0.1.0', [['a']]), snap('0.2.0', [['a'], ['b']])), []);
});

test('renaming a token without a MAJOR bump fails', () => {
  const errors = compareSnapshots(snap('0.1.0', [['color.text.primary', true]]), snap('0.2.0', [['color.text.main']]));
  assert.match(find(errors, 'semver-major').message, /color\.text\.primary/);
});

test('removing a token that was not deprecated fails even with a MAJOR bump', () => {
  const errors = compareSnapshots(snap('1.4.0', [['a'], ['b']]), snap('2.0.0', [['a']]));
  assert.equal(find(errors, 'semver-major'), undefined);
  assert.ok(find(errors, 'deprecated-before-removal', 'b'));
});

test('removing a deprecated token with a MAJOR bump passes', () => {
  assert.deepEqual(compareSnapshots(snap('1.4.0', [['a'], ['b', true]]), snap('2.0.0', [['a']])), []);
});

test('version going down fails', () => {
  assert.ok(find(compareSnapshots(snap('1.0.0', [['a']]), snap('0.9.0', [['a']])), 'semver'));
});

test('the CHANGELOG has an entry for the current version', () => {
  assert.deepEqual(checkChangelog(JSON.parse(fs.readFileSync(new URL('../package.json', import.meta.url))).version), []);
  assert.ok(find(checkChangelog('9.9.9'), 'semver'));
});

test('snapshotAtRef reads design/ from git history (temporary repository)', () => {
  const repo = fs.mkdtempSync(path.join(os.tmpdir(), 'ew-semver-'));
  const git = (...a) => execFileSync('git', a, { cwd: repo, stdio: 'pipe' });
  try {
    git('init', '-q');
    git('config', 'user.email', 'ci@example.invalid');
    git('config', 'user.name', 'ci');
    fs.mkdirSync(path.join(repo, 'design/game-tokens'), { recursive: true });
    fs.writeFileSync(path.join(repo, 'design/package.json'), JSON.stringify({ version: '0.1.0' }));
    fs.writeFileSync(path.join(repo, 'design/game-tokens/semantic.json'), JSON.stringify({ old: { $type: 'number', $value: 1, $description: 'old' } }));
    git('add', '.');
    git('commit', '-qm', 'base');
    const base = snapshotAtRef('HEAD', repo);
    assert.equal(base.version, '0.1.0');
    assert.deepEqual([...base.tokens.keys()], ['old']);
    assert.equal(snapshotAtRef('no-such-ref', repo), null);
    // working tree of the real design/ against this base: "old" was removed without deprecation
    const { errors, skipped } = checkSemver('HEAD', undefined, repo);
    assert.equal(skipped, false);
    assert.ok(find(errors, 'deprecated-before-removal', 'old'));
  } finally {
    fs.rmSync(repo, { recursive: true, force: true });
  }
});
