// F060 T036, F060-AC-19 (GODOT_ARCHITECTURE §11, DEPLOYMENT §10.2, ADR-0088): a PR touching src/game/**, specs/**,
// design/game-tokens/** or design/shared-tokens/** starts godot.yml, whose jobs run in the order
// design-tokens-check → lint (gdlint + typed + theme/layer/signal lints) → GdUnit4 → export Windows + Android
// (with artifact scan) → smoke boot.
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { loadWorkflows, workflowsTriggered } from './path-filters.mjs';

const wf = Object.fromEntries(loadWorkflows().map(({ id, doc }) => [id, doc]));
const godot = wf.godot;
const jobs = godot.jobs;
const needs = (id) => [].concat(jobs[id].needs ?? []);
const runs = (id) => jobs[id].steps.map((s) => s.run ?? '').join('\n');

/** Every job reachable backwards from `id` through `needs`. */
function ancestors(id, seen = new Set()) {
  for (const n of needs(id)) {
    if (!seen.has(n)) {
      seen.add(n);
      ancestors(n, seen);
    }
  }
  return seen;
}

test('game, specs and game/shared token changes start godot.yml', () => {
  for (const file of [
    'src/game/scripts/core/settings.gd',
    'specs/openapi/eduworld-api.v1.yaml',
    'design/game-tokens/semantic.json',
    'design/shared-tokens/domain.json',
  ]) {
    assert.ok(workflowsTriggered([file]).includes('godot'), file);
  }
  assert.ok(!workflowsTriggered(['design/admin-tokens/seed.json']).includes('godot'));
  assert.ok('workflow_call' in (godot.on ?? godot[true]));
});

test('jobs run in the order tokens → lint → test → export → smoke', () => {
  assert.deepEqual(needs('lint'), ['tokens']);
  assert.deepEqual(needs('test'), ['lint']);
  assert.ok(ancestors('export').has('test'));
  assert.ok(ancestors('export').has('lint'));
  assert.deepEqual(needs('smoke'), ['export']);
  assert.ok(ancestors('smoke').has('tokens'));
});

test('each stage runs the expected command', () => {
  assert.match(runs('tokens'), /make design-tokens-check/);
  assert.match(runs('lint'), /gdtoolkit==/);
  assert.match(runs('lint'), /make godot-lint/);
  assert.match(runs('test'), /make godot-test/);
  assert.match(runs('export'), /make godot-export-windows/);
  assert.match(runs('export'), /make godot-export-android/);
  assert.match(runs('smoke'), /make godot-smoke .*GAME_ENV=mock/);
});

test('native SecureStorage plugins are built before export (ADR-0086)', () => {
  assert.ok(needs('export').includes('platform-windows'));
  assert.ok(needs('export').includes('platform-android'));
  assert.match(String(jobs['platform-windows']['runs-on']), /windows/);
  assert.match(runs('platform-windows'), /scons platform=windows/);
  assert.match(runs('platform-android'), /gradle .*assemble/);
});

test('Godot comes from the pinned, checksum-verified setup script', () => {
  for (const id of ['lint', 'test', 'export', 'smoke']) {
    assert.match(runs(id), /scripts\/ci\/setup-godot\.sh/, id);
  }
});
