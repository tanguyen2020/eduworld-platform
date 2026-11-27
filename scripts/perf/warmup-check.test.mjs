// Tests of warmup-check.mjs with a stubbed Prometheus (TEST STAND-IN). Run: node --test scripts/perf/
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { check, peaks } from './warmup-check.mjs';

const prom = (running) => async () => ({ json: async () => ({ status: 'success', data: { result: Object.entries(running).map(([s, v]) => ({ metric: { swarm_service: s }, value: [0, String(v)] })) } }) });

test('staging peaks are read from the F130 scale schedule', () => {
  const p = peaks('staging');
  assert.ok(p['core-api'] >= 1 && p.realtime >= 1);
});

test('every service at peak passes; one below peak or missing fails (TC-PERF-017)', async () => {
  const p = peaks('staging');
  const ok = await check('staging', 'http://prom', prom(p));
  assert.equal(ok.peak_ok, true);
  const low = await check('staging', 'http://prom', prom({ ...p, 'core-api': p['core-api'] - 1 }));
  assert.equal(low.peak_ok, false);
  assert.equal(low.services.find((s) => s.name === 'core-api').ok, false);
  const { realtime, ...rest } = p;
  assert.ok(realtime);
  assert.equal((await check('staging', 'http://prom', prom(rest))).peak_ok, false);
});
