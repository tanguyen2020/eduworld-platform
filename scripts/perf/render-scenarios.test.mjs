// Tests of render-scenarios.mjs (F132 T010/T013). Run: node --test scripts/perf/
import { test } from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import { REQUIRED, TARGET, load, render, validate } from './render-scenarios.mjs';

test('scenarios.yaml is valid and covers every TC-PERF of TEST_STRATEGY §8.2', () => {
  assert.deepEqual(validate(load()), []);
  assert.equal(REQUIRED.length, 25);
});

test('the committed scenarios.gen.json is fresh', () => {
  assert.equal(fs.readFileSync(TARGET, 'utf8').replace(/\r\n/g, '\n'), render(load()));
});

test('a missing TC, a bad prom check and a k6 threshold without ref are reported', () => {
  const doc = {
    scenarios: [
      { id: 'TC-PERF-001', run: 'a', tool: 'k6', k6: [{ metric: 'm', expr: 'p(95)<1' }] },
      { id: 'TC-PERF-002', run: 'b', tool: 'k6', prom: [{ name: 'x', query: 'up', op: '~', value: '1', ref: 'r' }] },
      { id: 'TC-PERF-003', run: 'b', tool: 'jmeter' },
    ],
    out_of_scope: [{ ids: REQUIRED.slice(3) }],
  };
  const p = validate(doc).join('\n');
  assert.match(p, /TC-PERF-001: k6 threshold needs metric, expr, ref/);
  assert.match(p, /TC-PERF-002: prom check needs/);
  assert.match(p, /duplicate run name b/);
  assert.match(p, /unknown tool jmeter/);
  assert.doesNotMatch(p, /TC-PERF-004/);
  const missing = validate({ scenarios: [], out_of_scope: [] });
  assert.equal(missing.length, 25);
});

test('every k6 script of scenarios.yaml names its run in baseOptions', () => {
  for (const s of load().scenarios.filter((x) => x.script && x.script.endsWith('.js'))) {
    const src = fs.readFileSync(new URL(`../../${s.script}`, import.meta.url), 'utf8');
    assert.match(src, /checkEnv\(__ENV\)/, `${s.script} must refuse production`);
    if (s.tool === 'k6') assert.match(src, new RegExp(`baseOptions\\(gen, '${s.run}'`), `${s.script} must use the thresholds of ${s.run}`);
  }
});
