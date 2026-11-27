// Tests of render-report.mjs (F132 T009, F132-AC-06). Run: node --test scripts/perf/
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { k6Rows, render, verdict } from './render-report.mjs';

const meta = {
  run_id: '2026-12-01-ccu_10k', run: 'ccu_10k', tc: 'TC-PERF-014', env: 'staging', started: 's', ended: 'e', commit: 'abc',
  backend_tag: 'backend/v1.0.0', images: { 'core-api': 'sha256:1' }, sizing: { app: '3 × 8 vCPU' }, warmup: { peak_ok: true, checked_at: 't' },
};
const k6ok = { metrics: { http_req_failed: { rate: 0.001, thresholds: { 'rate<0.005': false } }, 'http_req_duration{kind:read}': { 'p(95)': 120, thresholds: { 'p(95)<200': false } } } };
const k6bad = { metrics: { http_req_failed: { rate: 0.02, thresholds: { 'rate<0.005': true } }, 'http_req_duration{kind:read}': { 'p(95)': 320, thresholds: { 'p(95)<200': true } } } };
const promOk = [{ name: 'ws_per_replica', observed: 3500, op: '<=', value: 4000, ok: true, ref: 'ws_per_gateway_node' }];
const genOk = { max_cpu: 0.55 };
const base = { meta, loadbot: null, cpuMax: 0.8, targetCcu: 10000 };

test('a failing fixture is "không đạt" and never claims target_ccu (F132-AC-06)', () => {
  const { text, verdict: v } = render({ ...base, k6: k6bad, prom: promOk, generator: genOk });
  assert.equal(v.status, 'failed');
  assert.match(text, /Kết luận: \*\*không đạt\*\*/);
  assert.doesNotMatch(text, /đạt `target_ccu`/);
  assert.match(text, /chưa được chứng minh/);
  assert.match(text, /k6 threshold http_req_failed rate<0.005 failed/);
});

test('generator CPU above perf_generator_cpu_max makes the run invalid, even with green thresholds', () => {
  const { text, verdict: v } = render({ ...base, k6: k6ok, prom: promOk, generator: { max_cpu: 0.93 } });
  assert.equal(v.status, 'invalid');
  assert.match(text, /không hợp lệ/);
  assert.match(text, /93% > 80%/);
  assert.doesNotMatch(text, /đạt `target_ccu`/);
});

test('no generator measurement is invalid (saturation unknown)', () => {
  assert.equal(verdict({ meta, k6: k6ok, prom: promOk, generator: null, cpuMax: 0.8 }).status, 'invalid');
});

test('a Prometheus check without data fails the run', () => {
  const v = verdict({ meta, k6: k6ok, prom: [{ name: 'circuit_open', observed: null, op: '<', value: 2, ok: false, ref: 'r' }], generator: genOk, cpuMax: 0.8 });
  assert.equal(v.status, 'failed');
  assert.match(v.reasons.join(), /no data/);
});

test('TC-PERF-014 without warm-up at peak fails (TC-PERF-017)', () => {
  const v = verdict({ meta: { ...meta, warmup: { peak_ok: false } }, k6: k6ok, prom: promOk, generator: genOk, cpuMax: 0.8 });
  assert.equal(v.status, 'failed');
});

test('only a passed TC-PERF-014 on staging claims target_ccu; the same numbers on local do not', () => {
  const ok = render({ ...base, k6: k6ok, prom: promOk, generator: genOk });
  assert.equal(ok.verdict.status, 'passed');
  assert.match(ok.text, /đạt `target_ccu` \(10000 CCU\) trên staging/);
  assert.match(ok.text, /sha256:1/);
  assert.match(ok.text, /backend\/v1\.0\.0/);
  const local = render({ ...base, meta: { ...meta, env: 'local' }, k6: k6ok, prom: promOk, generator: genOk });
  assert.doesNotMatch(local.text, /đạt `target_ccu`/);
  assert.match(local.text, /Không phải bằng chứng năng lực/);
  const other = render({ ...base, meta: { ...meta, tc: 'TC-PERF-001', run: 'baseline_api', warmup: null }, k6: k6ok, prom: promOk, generator: genOk });
  assert.equal(other.verdict.status, 'passed');
  assert.doesNotMatch(other.text, /target_ccu/);
});

test('k6 thresholds in the handleSummary shape ({ok}) are read too', () => {
  assert.deepEqual(k6Rows({ metrics: { m: { thresholds: { 'count==0': { ok: false } } } } }), [{ metric: 'm', expr: 'count==0', ok: false }]);
});
