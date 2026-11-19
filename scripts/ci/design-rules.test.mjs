// F005 T028, F005-AC-15 (DEPLOYMENT §10.2, ADR-0088): a PR touching design/** starts design.yml, which runs
// make design-tokens-check, uploads the contrast report and calls godot.yml and web.yml (workflow_call).
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { loadWorkflows, workflowsTriggered } from './path-filters.mjs';

const wf = Object.fromEntries(loadWorkflows().map(({ id, doc }) => [id, doc]));
const on = (doc) => doc.on ?? doc[true];

test('design/** triggers design.yml (and only token-related lanes)', () => {
  assert.deepEqual(workflowsTriggered(['design/README.md']), ['design', 'security']);
  assert.ok(workflowsTriggered(['design/shared-tokens/brand.json']).includes('design'));
  assert.ok(workflowsTriggered(['design/checks/contrast.mjs']).includes('design'));
});

test('design.yml runs the pipeline check and publishes the contrast report', () => {
  const steps = wf.design.jobs.tokens.steps;
  assert.ok(steps.some((s) => /make design-tokens-check/.test(s.run ?? '')));
  assert.ok(steps.some((s) => /actions\/upload-artifact@/.test(s.uses ?? '') && /design\/build\/reports/.test(s.with?.path ?? '')));
});

test('design.yml calls godot.yml and web.yml after the token job', () => {
  assert.equal(wf.design.jobs.godot.uses, './.github/workflows/godot.yml');
  assert.equal(wf.design.jobs.web.uses, './.github/workflows/web.yml');
  assert.equal(wf.design.jobs.godot.needs, 'tokens');
  assert.equal(wf.design.jobs.web.needs, 'tokens');
});

test('godot.yml and web.yml are callable (workflow_call) and keep their own path filters', () => {
  for (const id of ['godot', 'web']) {
    assert.ok('workflow_call' in on(wf[id]), id);
  }
  assert.ok(on(wf.godot).pull_request.paths.includes('design/game-tokens/**'));
  assert.ok(on(wf.godot).pull_request.paths.includes('design/shared-tokens/**'));
  assert.ok(on(wf.web).pull_request.paths.includes('design/admin-tokens/**'));
  assert.ok(on(wf.web).pull_request.paths.includes('design/shared-tokens/**'));
});
