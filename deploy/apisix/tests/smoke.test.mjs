// F004 T016/T017/T018/T024/T025 — gateway smoke tests against the local compose stack (APISIX public
// :9080, internal :9180, Keycloak :8180). APISIX_SMOKE=1 node --test deploy/apisix/tests/smoke.test.mjs
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { execFileSync } from 'node:child_process';
import { randomUUID } from 'node:crypto';
import { passwordGrant } from '../../keycloak/scripts/kc.mjs';

const PUB = process.env.APISIX_PUBLIC ?? 'http://localhost:9080';
const INT = process.env.APISIX_INTERNAL ?? 'http://localhost:9180';
const run = process.env.APISIX_SMOKE === '1' ? test : test.skip;
const S = 'change-me-local', P = 'Demo@12345';

async function call(base, path, { method = 'GET', token, headers = {}, body } = {}) {
  const h = { ...headers };
  if (token) h.authorization = `Bearer ${token}`;
  if (body !== undefined) h['content-type'] = 'application/json';
  const r = await fetch(base + path, { method, headers: h, body: body === undefined ? undefined : JSON.stringify(body) });
  const text = await r.text();
  let json = null;
  try { json = JSON.parse(text); } catch { /* not JSON */ }
  return { status: r.status, headers: r.headers, text, json };
}
const routeNotFound = (r) => r.status === 404 && /Route Not Found/.test(r.text);
const gameToken = async () => (await passwordGrant('eduworld-game-backend', S, 'hs.an.3a', P)).json.access_token;

run('F004-AC-09: internal prefixes do not exist on the public gateway', async () => {
  for (const p of ['/api/v1/admin/tenants', '/api/v1/sis/connections', '/api/v1/game-admin/quests', '/api/v1/game-media:presign-upload',
    '/internal/sis/webhooks/x', '/internal/v1/principals:resolve', '/livez', '/metrics']) {
    assert.ok(routeNotFound(await call(PUB, p)), `${p} routed on the public gateway`);
  }
  // Public routes exist (reach core-api, whatever it answers).
  assert.ok(!routeNotFound(await call(PUB, '/api/v1/game/client-config')));
});

run('F004-AC-10: platform prefixes exist on the internal gateway, behind OIDC', async () => {
  const r = await call(INT, '/api/v1/admin/tenants');
  assert.equal(r.status, 401);
  assert.ok(routeNotFound(await call(INT, '/api/v1/game/profile')), 'player routes must not be on the internal gateway');
});

run('F004-AC-13: missing, tampered or foreign tokens are rejected with 401 before the backend', async () => {
  assert.equal((await call(PUB, '/api/v1/game/profile')).status, 401);
  assert.equal((await call(PUB, '/api/v1/auth/game/logout', { method: 'POST', headers: { 'X-Tenant-Id': randomUUID() } })).status, 401,
    'game logout uses the bearer token');
  const t = await gameToken();
  const tampered = `${t.slice(0, -4)}AAAA`;
  assert.equal((await call(PUB, '/api/v1/game/profile', { token: tampered, headers: { 'X-Tenant-Id': randomUUID() } })).status, 401);
  const master = await fetch('http://localhost:8180/realms/master/protocol/openid-connect/token', { method: 'POST',
    body: new URLSearchParams({ grant_type: 'password', client_id: 'admin-cli', username: 'admin', password: process.env.KC_ADMIN_PASSWORD ?? '' }) });
  if (master.ok) {
    const foreign = (await master.json()).access_token;
    assert.equal((await call(PUB, '/api/v1/game/profile', { token: foreign, headers: { 'X-Tenant-Id': randomUUID() } })).status, 401);
  }
});

run('F004-AC-11/12: X-Tenant-Id required, UUID format, optional and exempt routes', async () => {
  const t = await gameToken();
  const missing = await call(PUB, '/api/v1/game/profile', { token: t });
  assert.equal(missing.status, 400);
  assert.equal(missing.json.code, 'TENANT_HEADER_REQUIRED');
  assert.match(missing.headers.get('content-type'), /application\/problem\+json/);
  const bad = await call(PUB, '/api/v1/game/profile', { token: t, headers: { 'X-Tenant-Id': 'abc' } });
  assert.equal(bad.status, 400);
  assert.equal(bad.json.code, 'BAD_REQUEST');
  const ok = await call(PUB, '/api/v1/game/profile', { token: t, headers: { 'X-Tenant-Id': randomUUID() } });
  assert.notEqual(ok.status, 400, 'valid header reaches the backend');
  assert.notEqual(ok.status, 401);
  const exempt = await call(PUB, '/api/v1/me/tenants', { token: t });
  assert.notEqual(exempt.status, 400, '/me/tenants is exempt (tenant_header_required)');
  const optional = await call(PUB, '/api/v1/me/permissions', { token: t });
  assert.notEqual(optional.status, 400, '/me/permissions accepts no header');
});

run('F004-AC-17/18: login limits per device and username (not per IP) with Problem Details and RateLimit headers', async () => {
  const device = `dev-${randomUUID()}`;
  let last;
  for (let i = 0; i < 11; i++) {
    last = await call(PUB, '/api/v1/auth/game/login', { method: 'POST', headers: { 'X-Device-Id': device },
      body: { username: `user-${randomUUID()}`, password: 'x' } });
  }
  assert.equal(last.status, 429);
  assert.equal(last.json.code, 'RATE_LIMITED');
  for (const h of ['retry-after', 'ratelimit-limit', 'ratelimit-remaining', 'ratelimit-reset']) assert.ok(last.headers.get(h), `header ${h}`);
  const username = `same-${randomUUID()}`;
  for (let i = 0; i < 11; i++) {
    last = await call(PUB, '/api/v1/auth/game/login', { method: 'POST', headers: { 'X-Device-Id': `dev-${randomUUID()}` },
      body: { username: username.toUpperCase().padEnd(username.length + 2), password: 'x' } });
  }
  assert.equal(last.status, 429, 'same normalized username from different devices');
  // Same IP, other devices and users: not limited (school NAT, R30).
  const other = await call(PUB, '/api/v1/auth/game/login', { method: 'POST', headers: { 'X-Device-Id': `dev-${randomUUID()}` },
    body: { username: `fresh-${randomUUID()}`, password: 'x' } });
  assert.notEqual(other.status, 429);
});

run('api_rate_default: writes per user are limited in Valkey (Problem 429), reads counted apart', async () => {
  const t = await gameToken();
  const headers = { 'X-Tenant-Id': randomUUID() };
  let last;
  for (let i = 0; i < 61; i++) {
    last = await call(PUB, '/api/v1/game/telemetry', { method: 'POST', token: t, headers, body: {} });
    if (last.status === 429) break;
  }
  assert.equal(last.status, 429);
  assert.equal(last.json.code, 'RATE_LIMITED');
  assert.match(last.headers.get('content-type'), /application\/problem\+json/);
  for (const h of ['retry-after', 'ratelimit-limit', 'ratelimit-reset']) assert.ok(last.headers.get(h), `header ${h}`);
  assert.equal(last.headers.get('ratelimit-limit'), '60');
  const read = await call(PUB, '/api/v1/game/profile', { token: t, headers });
  assert.notEqual(read.status, 429, 'reads have their own counter');
  assert.equal(read.headers.get('ratelimit-limit'), '300');
});

run('F004-AC-21: CORS preflight allows only the allowlisted origin', async () => {
  const pre = (origin) => fetch(`${PUB}/api/v1/game/profile`, { method: 'OPTIONS', headers: { Origin: origin,
    'Access-Control-Request-Method': 'GET', 'Access-Control-Request-Headers': 'authorization,x-tenant-id' } });
  const ok = await pre('http://localhost:5173');
  assert.equal(ok.headers.get('access-control-allow-origin'), 'http://localhost:5173');
  assert.match(ok.headers.get('access-control-allow-headers') ?? '', /X-Tenant-Id/i);
  const foreign = await pre('http://evil.example');
  assert.notEqual(foreign.headers.get('access-control-allow-origin'), 'http://evil.example');
  assert.notEqual(foreign.headers.get('access-control-allow-origin'), '*');
});

run('F004-AC-15/16: the password of a login never reaches the gateway logs', async () => {
  const secret = `Pw-${randomUUID()}`;
  await call(PUB, '/api/v1/auth/game/login', { method: 'POST', headers: { 'X-Device-Id': `dev-${randomUUID()}` },
    body: { username: `log-${randomUUID()}`, password: secret } });
  await call(PUB, '/api/v1/auth/portal/login', { method: 'POST', headers: { 'X-Device-Id': `dev-${randomUUID()}` },
    body: { username: `log-${randomUUID()}`, password: secret } });
  const logs = execFileSync('docker', ['logs', '--since', '2m', 'eduworld-apisix-1'], { encoding: 'utf8', stdio: ['ignore', 'pipe', 'pipe'] });
  assert.ok(!logs.includes(secret), 'password found in APISIX logs');
  assert.ok(!/\/realtime\/v1\?/.test(logs), 'realtime query string in access log');
});
