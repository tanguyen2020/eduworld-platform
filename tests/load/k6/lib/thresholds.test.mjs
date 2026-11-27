// Unit tests of the pure k6 helpers (F132 T010). Run: node --test tests/load/k6/lib/
import { test } from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import { baseOptions, checkEnv, dur, param, scaled, thresholdsFor } from './thresholds.js';
import { retryWait, uuidv7 } from './util.js';

const gen = JSON.parse(fs.readFileSync(new URL('../scenarios.gen.json', import.meta.url), 'utf8'));

test('thresholds come from scenarios.yaml (TC-PERF-014 NFR-PERF-001/002, error budget)', () => {
  const t = thresholdsFor(gen, 'ccu_10k');
  assert.deepEqual(t['http_req_duration{kind:read}'], ['p(95)<200']);
  assert.deepEqual(t['http_req_duration{kind:write}'], ['p(95)<400']);
  assert.deepEqual(t.http_req_failed, ['rate<0.005']);
});

test('several criteria on one metric are merged (TC-PERF-025 p95 and max)', () => {
  assert.deepEqual(thresholdsFor(gen, 'dashboard_2y')['http_req_duration{kind:dashboard}'], ['p(95)<2000', 'max<5000']);
});

test('login overload requires the queue, Retry-After and no 5xx (F132-AC-03)', () => {
  const t = thresholdsFor(gen, 'login_overload');
  assert.deepEqual(t.eduworld_http_5xx, ['count==0']);
  assert.deepEqual(t.eduworld_login_queue_busy, ['count>0']);
  assert.deepEqual(t.eduworld_login_retry_after_missing, ['count==0']);
  assert.deepEqual(t.eduworld_login_gave_up, ['count==0']);
});

test('unknown run or parameter fails loudly', () => {
  assert.throws(() => thresholdsFor(gen, 'nope'));
  assert.throws(() => param(gen, 'nope'));
  assert.equal(param(gen, 'target_ccu'), 10000);
});

test('EW_ENV guard refuses production (F132-AC-07)', () => {
  assert.equal(checkEnv({ EW_ENV: 'Staging' }), 'staging');
  for (const e of ['production', 'prod', '', undefined]) assert.throws(() => checkEnv({ EW_ENV: e }));
});

test('SCALE and DURATION_SCALE shrink a smoke run, never below 1', () => {
  assert.equal(scaled(10000, { SCALE: '0.001' }), 10);
  assert.equal(scaled(100, { SCALE: '0.0001' }), 1);
  assert.equal(scaled(100, {}), 100);
  assert.equal(dur('10m', { DURATION_SCALE: '0.01' }), '6s');
  assert.equal(dur('4h', {}), '14400s');
  assert.equal(dur('30s', { DURATION_SCALE: '0.001' }), '1s');
  assert.throws(() => dur('10 min', {}));
});

test('base options tag the run id for the F131 panels', () => {
  const o = baseOptions(gen, 'baseline_api', { RUN_ID: '2026-12-01-ccu' });
  assert.equal(o.tags.testid, '2026-12-01-ccu');
  assert.equal(o.tags.tc, 'TC-PERF-001');
  assert.ok(o.summaryTrendStats.includes('p(95)'));
});

test('429 wait honours Retry-After plus jitter, never a tight retry', () => {
  assert.equal(retryWait('3', 120, 0.5), 3.5);
  assert.equal(retryWait(undefined, 120, 0), 1);
  assert.equal(retryWait('0', 120, 0), 1);
  assert.equal(retryWait('9999', 120, 0), 120);
});

test('Idempotency-Key is a time-ordered UUIDv7', () => {
  const a = uuidv7(1000);
  const b = uuidv7(2000);
  assert.match(a, /^[0-9a-f]{8}-[0-9a-f]{4}-7[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/);
  assert.ok(a.slice(0, 13) < b.slice(0, 13));
});
