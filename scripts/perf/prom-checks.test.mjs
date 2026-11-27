// Tests of prom-checks.mjs with a stubbed Prometheus (TEST STAND-IN, AGENTS R17). Run: node --test scripts/perf/
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { evaluate, expand } from './prom-checks.mjs';
import { load } from './render-scenarios.mjs';

function stub(values) {
  const calls = [];
  const fn = async (url) => {
    calls.push(decodeURIComponent(url));
    const q = decodeURIComponent(new URL(url).searchParams.get('query'));
    const hit = Object.entries(values).find(([k]) => q.includes(k));
    const result = hit && hit[1] !== null ? [{ metric: {}, value: [0, String(hit[1])] }] : [];
    return { ok: true, json: async () => ({ status: 'success', data: { resultType: 'vector', result } }) };
  };
  fn.calls = calls;
  return fn;
}

test('placeholders are replaced by the run window and timestamps', () => {
  assert.equal(expand('rate(x[$window]) @ $end @ $scale_at', { start: 1000, end: 4600, scaleAt: 2000 }), 'rate(x[3600s]) @ 4600 @ 2000');
});

test('TC-PERF-014: WS per replica above ws_per_gateway_node and a missing metric both fail', async () => {
  const f = stub({ eduworld_ws_connections: 4200, sli_realtime_delivery: 0.1, eduworld_internal_client_duration_seconds: 0.02,
    eduworld_outbox_lag_seconds: 0.3, swarm_task_restarts_total: 0, eduworld_loadbot_logins_total: 0, eduworld_external_client_circuit_state: null });
  const res = await evaluate(load(), 'ccu_10k', { prom: 'http://prom:9090/', start: 0, end: 4200 }, f);
  const by = Object.fromEntries(res.map((r) => [r.name, r]));
  assert.equal(by.ws_per_replica.ok, false);
  assert.equal(by.ws_per_replica.observed, 4200);
  assert.equal(by.circuit_open.observed, null);
  assert.equal(by.circuit_open.ok, false);
  assert.equal(by.delivery_p95.ok, true);
  assert.ok(f.calls[0].startsWith('http://prom:9090/api/v1/query?query='));
});

test('an unknown run is an error', async () => {
  await assert.rejects(evaluate(load(), 'nope', { prom: 'http://p', start: 0, end: 1 }, stub({})));
});
