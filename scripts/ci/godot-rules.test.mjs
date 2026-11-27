// F060 T036, F060-AC-19 (GODOT_ARCHITECTURE §11, DEPLOYMENT §10.2, ADR-0088): a PR touching src/game/**, specs/**,
// design/game-tokens/** or design/shared-tokens/** starts godot.yml, whose jobs run in the order
// design-tokens-check → lint (gdlint + typed + theme/layer/signal lints) → GdUnit4 → export Android (with artifact
// scan) → smoke boot. No Windows build (ADR-0101).
import { test } from 'node:test';
import fs from 'node:fs';
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
  assert.match(runs('export'), /make godot-export-android/);
  assert.match(runs('smoke'), /make godot-smoke .*GAME_ENV=mock/);
});

test('native SecureStorage plugins are built before export (ADR-0086)', () => {
  assert.ok(needs('export').includes('platform-android'));
  assert.match(runs('platform-android'), /gradle .*assemble/);
});

test('there is no Windows build, plugin or export (ADR-0101)', () => {
  assert.ok(!('platform-windows' in jobs));
  for (const [id, job] of Object.entries(jobs)) {
    assert.doesNotMatch(String(job['runs-on'] ?? ''), /windows/i, id);
    assert.doesNotMatch(runs(id), /godot-export-windows|scons platform=windows|Windows Desktop|\.exe\b/, id);
  }
  assert.ok(!('SCONS_VERSION' in (godot.env ?? {})));
});

test('Godot comes from the pinned, checksum-verified setup script', () => {
  for (const id of ['lint', 'test', 'export', 'smoke']) {
    assert.match(runs(id), /scripts\/ci\/setup-godot\.sh/, id);
  }
});

test('F133 T002/T028: the export job also exports the Web build, checks it and runs the shell tests', () => {
  assert.match(runs('export'), /make godot-export-web/);
  assert.match(runs('export'), /node --test src\/game\/web\/tests\//);
  const upload = jobs.export.steps.find((s) => String(s.uses ?? '').startsWith('actions/upload-artifact'));
  assert.match(String(upload.with.path), /src\/game\/build\/web\//);
  // iOS needs the macOS runner (T019): no iOS export on the Linux runners.
  assert.doesNotMatch(runs('export'), /godot-export-ios/);
});

// ---------- F133 T006 (F133-AC-01…09, F133-AC-17, DEPLOYMENT §10.2, ADR-0088, ADR-0101) ----------
const job = (id) => jobs[id];
const runsIn = (id) => runs(id);
const usesOf = (id) => jobs[id].steps.map((s) => String(s.uses ?? '')).filter(Boolean);

test('F133: a client/v* tag starts godot.yml; release tools, school kit and CDN rules are in the path filter', () => {
  const on = godot.on ?? godot[true];
  assert.deepEqual(on.push.tags, ['client/v*']);
  for (const p of ['scripts/godot/**', 'deploy/installer/school-kit/**', 'deploy/cdn/play/**']) {
    assert.ok(on.pull_request.paths.includes(p), p);
    assert.ok(on.push.paths.includes(p), p);
  }
  assert.ok(workflowsTriggered(['scripts/godot/export.sh']).includes('godot'));
  assert.ok(workflowsTriggered(['deploy/installer/school-kit/build-kit.sh']).includes('godot'));
});

test('F133: release script tests run on every godot run (node tests + F134 publish-index test)', () => {
  assert.match(runsIn('release-tools'), /node --test scripts\/godot\/ deploy\/installer\/school-kit\/tests\//);
  assert.match(runsIn('release-tools'), /publish_index_test\.sh/);
});

test('F133: export stamps version/build through export.sh; scan-artifact, size-check and bundles gate the artifacts', () => {
  assert.match(fs.readFileSync('Makefile', 'utf8'), /godot-export-android:[^\n]*\n(\t[^\n]*\n)*\tbash scripts\/godot\/export\.sh android --debug/);
  assert.match(fs.readFileSync('Makefile', 'utf8'), /godot-export-web:[^\n]*\n(\t[^\n]*\n)*\tbash scripts\/godot\/export\.sh web/);
  for (const id of ['scan-artifact', 'size-check', 'bundles']) assert.ok(needs(id).includes('export'), id);
  assert.match(runsIn('scan-artifact'), /scan-artifact\.sh --platform=android/);
  assert.match(runsIn('scan-artifact'), /scan-artifact\.sh --platform=web/);
  assert.match(runsIn('size-check'), /check-build-size\.mjs .*--report=/);
  assert.ok(usesOf('size-check').some((u) => u.startsWith('actions/upload-artifact')), 'size report is attached');
  assert.match(runsIn('bundles'), /build-bundles\.sh/);
});

test('F133: iOS export runs only on a macOS runner, on tags/main, behind a switch (T001/T019 pending)', () => {
  assert.match(String(job('export-ios')['runs-on']), /^macos-/);
  assert.match(String(job('export-ios').if), /vars\.IOS_BUILD_ENABLED == 'true'/);
  assert.match(String(job('export-ios').if), /refs\/tags\/client\/v/);
  assert.match(runsIn('export-ios'), /export\.sh ios/);
});

test('F133: staging/production jobs only on a client tag, with their Environment, on the deploy runner, never for forks', () => {
  const release = ['smoke-staging', 'publish-staging', 'publish-production'];
  for (const id of release) {
    const j = job(id);
    assert.ok(['staging', 'production'].includes(typeof j.environment === 'string' ? j.environment : j.environment.name), id);
    assert.match(String(j.if), /startsWith\(github\.ref, 'refs\/tags\/client\/v'\)/, id);
    assert.match(String(j.if), /vars\.CLIENT_RELEASE_ENABLED == 'true'/, id);
    assert.match(String(j.if), /head\.repo\.fork != true/, id);
    assert.deepEqual([].concat(j['runs-on']), ['self-hosted', 'deploy'], id);
  }
  assert.ok(needs('publish-staging').includes('smoke-staging'), 'publishing waits for the smoke (F133-AC-06)');
  for (const id of ['scan-artifact', 'size-check', 'bundles']) assert.ok(needs('publish-staging').includes(id), id);
  assert.ok(needs('publish-production').includes('publish-staging'), 'production is a promotion after staging (F133-AC-08)');
  assert.equal(job('publish-production').environment, 'production');
});

test('F133: staging smoke runs smoke.sh with the QA student secrets; publishing uses the CDN rules and F134 publish-index', () => {
  assert.match(runsIn('smoke-staging'), /scripts\/godot\/smoke\.sh --api=/);
  const env = JSON.stringify(job('smoke-staging').steps);
  assert.match(env, /secrets\.EW_SMOKE_PASSWORD/);
  assert.match(runsIn('publish-staging'), /publish-web\.mjs/);
  assert.match(runsIn('publish-staging'), /build-bundles\.sh[\s\S]*--upload=/);
  assert.match(runsIn('publish-staging'), /sign-manifest\.mjs manifest --platform=web/);
  assert.match(runsIn('publish-staging'), /school-kit\/build-kit\.sh/);
  assert.match(runsIn('publish-staging'), /school-kit\/publish-index\.sh --channel=staging/);
  assert.match(runsIn('publish-production'), /school-kit\/publish-index\.sh --channel=production/);
});

test('F133: no secret is written to the job env or echoed; private keys only reach files under RUNNER_TEMP', () => {
  for (const [id, j] of Object.entries(jobs)) {
    assert.doesNotMatch(JSON.stringify(j.env ?? {}), /secrets\./, `${id}: secrets only in step env`);
    assert.doesNotMatch(runsIn(id), /echo[^\n]*\$\{\{\s*secrets\./, `${id}: never echo a secret`);
  }
  assert.doesNotMatch(JSON.stringify(godot.env ?? {}), /secrets\./);
});
