// Tests for check-workflows (F001 T035, F001-AC-25, F001-AC-26). Run: node --test scripts/ci/check-workflows.test.mjs
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { checkWorkflowText } from './check-workflows.mjs';

const SHA = '11bd71901bbe5b1630ceea73d27597364c9af683';

const good = `name: ok
on: { pull_request: { paths: ["docs/**"] } }
permissions:
  contents: read
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@${SHA} # v4.2.2
      - uses: ./.github/actions/local
      - run: echo ok
`;

const msgs = (text) => checkWorkflowText('fixture.yml', text).map((v) => `${v.line}:${v.rule}`);

test('a compliant workflow has no violation', () => {
  assert.deepEqual(msgs(good), []);
});

test('third-party action pinned by tag fails on the exact line', () => {
  const text = good.replace(`actions/checkout@${SHA} # v4.2.2`, 'actions/checkout@v4');
  assert.deepEqual(msgs(text), ['9:pin-sha']);
});

test('short SHA is not a full pin', () => {
  const text = good.replace(SHA, SHA.slice(0, 7));
  assert.deepEqual(msgs(text), ['9:pin-sha']);
});

test('missing top-level permissions fails', () => {
  const text = good.replace('permissions:\n  contents: read\n', '');
  assert.deepEqual(msgs(text), ['1:permissions']);
});

test('write-all default permissions fail', () => {
  const text = good.replace('permissions:\n  contents: read', 'permissions: write-all');
  assert.deepEqual(msgs(text), ['3:permissions']);
});

test('iOS job on a Linux runner fails', () => {
  const text = good.replace('  build:\n    runs-on: ubuntu-latest', '  export-ios:\n    runs-on: ubuntu-latest');
  assert.deepEqual(msgs(text), ['6:ios-runner']);
});

test('iOS job on macOS passes', () => {
  const text = good.replace('  build:\n    runs-on: ubuntu-latest', '  export-ios:\n    runs-on: macos-14');
  assert.deepEqual(msgs(text), []);
});

const deployJob = (runsOn, cond) => `name: release
on: { push: { branches: [main] }, pull_request: {} }
permissions:
  contents: read
jobs:
  deploy-staging:
    runs-on: ${runsOn}
    environment: staging
${cond ? `    if: ${cond}\n` : ''}    steps:
      - run: echo deploy
`;

test('deploy job on GitHub-hosted runner fails', () => {
  assert.deepEqual(msgs(deployJob('ubuntu-latest', "github.event.pull_request.head.repo.fork != true")), ['6:deploy-runner']);
});

test('deploy job without fork guard fails', () => {
  assert.deepEqual(msgs(deployJob('[self-hosted, deploy]', '')), ['6:deploy-fork-guard']);
});

test('deploy job on self-hosted deploy runner with fork guard passes', () => {
  assert.deepEqual(msgs(deployJob('[self-hosted, deploy]', "github.event_name == 'push' && github.event.pull_request.head.repo.fork != true")), []);
});

// F130 T052: the APISIX admin key is a secret of the deploy jobs only (apisix:sync, canary), in the step that uses it.
const keyJob = (environment, where) => `name: k
on: workflow_dispatch
permissions:
  contents: read
jobs:
  sync:
    runs-on: [self-hosted, deploy]
${environment ? `    environment: ${environment}\n` : ''}    if: github.event.pull_request.head.repo.fork != true
${where === 'job' ? '    env:\n      APISIX_ADMIN_KEY: ${{ secrets.APISIX_ADMIN_KEY }}\n' : ''}    steps:
      - run: echo sync
${where === 'step' ? '        env:\n          APISIX_ADMIN_KEY: ${{ secrets.APISIX_ADMIN_KEY }}\n' : ''}`;

test('APISIX admin key in the step env of a staging/production job passes', () => {
  assert.deepEqual(msgs(keyJob('production', 'step')), []);
  assert.deepEqual(msgs(keyJob('${{ inputs.environment }}', 'step')), []);
});

test('APISIX admin key without a deploy Environment fails', () => {
  assert.deepEqual(msgs(keyJob('', 'step')), ['6:apisix-admin-key']);
  assert.deepEqual(msgs(keyJob('staging-readonly', 'step')), ['6:apisix-admin-key']);
});

test('APISIX admin key in the job env fails', () => {
  assert.deepEqual(msgs(keyJob('staging', 'job')), ['6:apisix-admin-key']);
});
