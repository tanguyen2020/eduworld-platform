// Tests of the synthetic probe logic (F131 T035: F131-AC-19, AC-20, TC-AUTH-044). The HTTP client is a fake server
// (MOCK, AGENTS R17) recording every call. Run: node --test tests/synthetic/k6/
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { classify, retryAfterSeconds, parseState, promText, runStudentLogin, runClientConfig, resultCode, resultOf } from './lib/probe.js';

const CFG = { baseUrl: 'https://api.qa.invalid', username: 'hs.qa.probe', password: 'not-a-real-password', deviceId: '00000000-0000-4000-8000-000000000001', clientVersion: '1.0.0', platform: 'windows', idempotencyKey: () => '00000000-0000-4000-8000-0000000000aa' };

// MOCK HTTP server: route → { status, body, headers }.
function fakeHttp(routes) {
  const calls = [];
  const respond = (method, url, body, params) => {
    calls.push({ method, url, body, headers: params?.headers ?? {} });
    const path = url.replace(CFG.baseUrl, '').replace(/\?.*$/, '');
    const key = Object.keys(routes).find((k) => new RegExp(`^${k}$`).test(`${method} ${path}`));
    const r = key ? routes[key] : { status: 404 };
    return { status: r.status, headers: r.headers ?? {}, body: JSON.stringify(r.body ?? {}), json: () => r.body ?? {} };
  };
  return { calls, get: (u, p) => respond('GET', u, null, p), post: (u, b, p) => respond('POST', u, b, p) };
}
const LOGIN_OK = { status: 200, body: { tokens: { access_token: 'a.b.c', refresh_token: 'r' }, bootstrap: { tenant_id: '00000000-0000-4000-8000-0000000000qa' } } };
const OK_ROUTES = {
  'POST /api/v1/auth/game/login': LOGIN_OK,
  'POST /api/v1/game/sessions': { status: 201, body: { game_session_id: 's1' } },
  'POST /api/v1/game/sessions/s1:end': { status: 204 },
  'POST /api/v1/auth/game/logout': { status: 204 },
};

test('classify and Retry-After parsing', () => {
  assert.equal(classify(200), 'ok'); assert.equal(classify(204), 'ok');
  assert.equal(classify(429), 'rate_limited'); assert.equal(classify(503), 'error'); assert.equal(classify(401), 'error');
  assert.equal(retryAfterSeconds({ 'retry-after': '30' }), 30);
  assert.equal(retryAfterSeconds({ 'Retry-After': new Date(Date.now() + 120000).toUTCString() }), 120);
  assert.equal(retryAfterSeconds({}), 60);
  assert.equal(retryAfterSeconds({ 'Retry-After': '99999' }), 3600);
  assert.equal(resultOf(resultCode('rate_limited')), 'rate_limited');
});

test('student_login: full chain login → session → end → logout, QA tenant header, no attempt endpoint (AC-20)', () => {
  const http = fakeHttp(OK_ROUTES);
  const r = runStudentLogin(http, CFG);
  assert.equal(r.result, 'ok');
  assert.deepEqual(http.calls.map((c) => `${c.method} ${c.url.replace(CFG.baseUrl, '')}`), [
    'POST /api/v1/auth/game/login', 'POST /api/v1/game/sessions', 'POST /api/v1/game/sessions/s1:end', 'POST /api/v1/auth/game/logout']);
  assert.ok(!http.calls.some((c) => /attempt|exam|answer|quest|learning/.test(c.url)), 'never touches learning endpoints');
  assert.equal(http.calls[1].headers['X-Tenant-Id'], LOGIN_OK.body.bootstrap.tenant_id);
  assert.equal(http.calls[1].headers.Authorization, 'Bearer a.b.c');
});

test('429 LOGIN_QUEUE_BUSY: result rate_limited, Retry-After kept, no retry and no further call (TC-AUTH-044)', () => {
  const http = fakeHttp({ 'POST /api/v1/auth/game/login': { status: 429, headers: { 'Retry-After': '45' }, body: { code: 'LOGIN_QUEUE_BUSY' } } });
  const r = runStudentLogin(http, CFG);
  assert.deepEqual([r.result, r.retryAfter, r.step], ['rate_limited', 45, 'login']);
  assert.equal(http.calls.length, 1, 'exactly one call: no burst of retries');
});

test('5xx on session: error, but the probe still logs out (cleanup)', () => {
  const http = fakeHttp({ ...OK_ROUTES, 'POST /api/v1/game/sessions': { status: 503 } });
  const r = runStudentLogin(http, CFG);
  assert.deepEqual([r.result, r.step], ['error', 'session']);
  assert.equal(http.calls.at(-1).url, `${CFG.baseUrl}/api/v1/auth/game/logout`);
});

test('invalid credentials: error at login, nothing else called', () => {
  const http = fakeHttp({ 'POST /api/v1/auth/game/login': { status: 401, body: { code: 'INVALID_CREDENTIALS' } } });
  assert.equal(runStudentLogin(http, CFG).result, 'error');
  assert.equal(http.calls.length, 1);
});

test('client_config: ok / error / rate_limited', () => {
  assert.equal(runClientConfig(fakeHttp({ 'GET /api/v1/game/client-config': { status: 200 } }), CFG).result, 'ok');
  assert.equal(runClientConfig(fakeHttp({ 'GET /api/v1/game/client-config': { status: 502 } }), CFG).result, 'error');
  const rl = runClientConfig(fakeHttp({ 'GET /api/v1/game/client-config': { status: 429, headers: { 'Retry-After': '10' } } }), CFG);
  assert.deepEqual([rl.result, rl.retryAfter], ['rate_limited', 10]);
});

test('textfile: metric names/labels of MONITORING §3, next_allowed round-trips as state', () => {
  const t = promText({ probe: 'student_login', result: 'rate_limited', durationSeconds: 0.5, nowSeconds: 1000, nextAllowed: 1045 });
  assert.match(t, /^eduworld_synthetic_probe_success\{probe="student_login",result="rate_limited"\} 0$/m);
  assert.match(t, /^eduworld_synthetic_probe_duration_seconds\{probe="student_login"\} 0\.500$/m);
  assert.match(t, /^eduworld_synthetic_probe_last_run_timestamp_seconds\{probe="student_login"\} 1000$/m);
  assert.equal(parseState(t), 1045);
  assert.equal(parseState(''), 0);
  assert.ok(!/hs\.qa|password|token/.test(t), 'no credential or user in the metrics (R14)');
});
