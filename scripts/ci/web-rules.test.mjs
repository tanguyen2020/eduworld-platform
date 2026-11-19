// F050 T035, F050-AC-21 (DEPLOYMENT §10.2, ADR-0088): web.yml runs on src/cms/frontend/**, specs/openapi/**,
// design/admin-tokens/**, design/shared-tokens/**, specs/portal/** (F051 menu catalog) and contains lint, gen:check, test, build, Storybook, axe, visual.
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { loadWorkflows, workflowsTriggered } from './path-filters.mjs';

const web = loadWorkflows().find((w) => w.id === 'web').doc;
const steps = Object.values(web.jobs).flatMap((j) => j.steps ?? []);
const runs = steps.map((s) => s.run ?? '').join('\n');

test('path filter of web.yml', () => {
  for (const f of ['src/cms/frontend/apps/portal/src/main.tsx', 'specs/openapi/eduworld-api.v1.yaml', 'specs/portal/menu-catalog.yaml', 'design/admin-tokens/seed.json', 'design/shared-tokens/domain.json']) {
    assert.ok(workflowsTriggered([f]).includes('web'), f);
  }
  for (const f of ['design/game-tokens/semantic.json', 'src/game/project.godot', 'docs/README.md']) {
    assert.ok(!workflowsTriggered([f]).includes('web'), f);
  }
});

test('web.yml runs lint, gen:check, tests, build, Storybook, axe and visual', () => {
  for (const cmd of ['pnpm lint', 'pnpm typecheck', 'pnpm gen:check', 'pnpm test', 'pnpm build', 'pnpm storybook:build', 'storybook-a11y.mjs', 'pnpm test:visual']) {
    assert.ok(runs.includes(cmd), cmd);
  }
});

test('web.yml is callable from design.yml', () => {
  assert.ok('workflow_call' in (web.on ?? web[true]));
});
