// F004 T026 — F004-AC-19, AC-20, AC-21: render.mjs output per environment.
//   node --test deploy/apisix/scripts/render.test.mjs
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { render } from './render.mjs';
import { yaml, ROOT, loadEnv, loadRoutes } from './lib.mjs';

const doc = (envFile, gw) => {
  const env = loadEnv(envFile);
  const out = render(loadRoutes(env), env, gw);
  assert.ok(out.endsWith('#END\n'), 'standalone file must end with #END');
  return yaml.load(out.replace(/#END\s*$/, ''));
};
const byId = (d, id) => d.routes.find((r) => r.id === id);

test('F004-AC-19: staging forces HTTPS with HSTS; config allows TLS 1.2+ only', () => {
  for (const r of doc('staging.env.example', 'public').routes) {
    assert.deepEqual(r.plugins.redirect, { http_to_https: true }, r.id);
    assert.match(r.plugins['response-rewrite'].headers.set['Strict-Transport-Security'], /max-age=\d+/, r.id);
  }
  for (const r of doc('local.env', 'public').routes) assert.equal(r.plugins.redirect, undefined, `${r.id}: local is plain HTTP`);
  const cfg = yaml.load(readFileSync(new URL('config.yaml', ROOT), 'utf8'));
  assert.deepEqual(cfg.apisix.ssl.ssl_protocols.split(/\s+/).sort(), ['TLSv1.2', 'TLSv1.3']);
});

test('F004-AC-20: /realtime/v1 is a websocket route with limit-conn; access log has no query string', () => {
  const rt = byId(doc('local.env', 'public'), 'public-realtime');
  assert.equal(rt.enable_websocket, true);
  assert.equal(rt.upstream_id, 'realtime');
  assert.equal(rt.plugins['limit-conn'].conn, 200);
  assert.equal(rt.plugins['openid-connect'], undefined, 'realtime authenticates with a ticket, not a bearer token');
  const fmt = yaml.load(readFileSync(new URL('config.yaml', ROOT), 'utf8')).nginx_config.http.access_log_format;
  assert.doesNotMatch(fmt, /\$(request|args|request_uri|query_string)(?![a-z_])/);
});

test('F004-AC-21: common plugins on every public route, ip-restriction on every internal route', () => {
  for (const r of doc('local.env', 'public').routes) {
    for (const p of ['request-id', 'prometheus', 'opentelemetry', 'cors']) assert.ok(r.plugins[p], `${r.id} lacks ${p}`);
    assert.equal(r.plugins['ip-restriction'], undefined);
  }
  const internal = doc('local.env', 'internal');
  assert.ok(internal.routes.length > 0);
  for (const r of internal.routes) {
    assert.ok(r.plugins['ip-restriction'].whitelist.length > 0, r.id);
    assert.equal(r.plugins.cors, undefined, `${r.id}: internal gateway has no CORS`);
  }
});

test('OIDC routes validate issuer and audience; login routes carry the login limiter and no OIDC', () => {
  const d = doc('staging.env.example', 'public');
  const oidc = byId(d, 'public-player').plugins['openid-connect'];
  assert.equal(oidc.bearer_only, true);
  assert.deepEqual(oidc.claim_validator.issuer.valid_issuers, ['https://auth.staging.eduworld.example/realms/eduworld']);
  assert.equal(oidc.claim_validator.audience.match_with_client_id, true);
  const login = byId(d, 'public-auth-game-login');
  assert.equal(login.plugins['openid-connect'], undefined);
  assert.equal(login.plugins['ew-login-key'].device_count, 10);
  assert.ok(login.plugins['client-control'].max_body_size > 0);
  const portal = byId(d, 'public-auth-portal');
  assert.equal(portal.host, 'portal.staging.eduworld.example');
  assert.equal(portal.plugins['ew-tenant-header'], undefined);
});

test('api_rate_default: every OIDC route counts per user in Valkey; login and public routes do not', () => {
  for (const gw of ['public', 'internal']) {
    for (const r of doc('local.env', gw).routes) {
      const rate = r.plugins['ew-api-rate'];
      if (r.plugins['openid-connect']) {
        assert.ok(rate, `${r.id} lacks ew-api-rate`);
        assert.equal(rate.redis_host, 'valkey');
        assert.equal(rate.allow_degradation, true);
        assert.equal(rate.redis_password, undefined, 'no secret in the rendered file');
      } else {
        assert.equal(rate, undefined, `${r.id}: ew-api-rate without OIDC`);
      }
    }
  }
  const d = doc('local.env', 'public');
  assert.deepEqual([byId(d, 'public-player').plugins['ew-api-rate'].read_count, byId(d, 'public-player').plugins['ew-api-rate'].write_count], [300, 60]);
  assert.equal(byId(d, 'public-auth-session').plugins['ew-api-rate'].read_count, 10);
});

test('internal gateway never serves player or login routes', () => {
  const ids = doc('local.env', 'internal').routes.map((r) => r.id);
  for (const id of ids) assert.doesNotMatch(id, /player|auth-|realtime/);
});

test('the committed standalone files are up to date', () => {
  const env = loadEnv('local.env');
  const src = loadRoutes(env);
  for (const gw of ['public', 'internal']) {
    const cur = readFileSync(new URL(`standalone/${gw}/apisix.yaml`, ROOT), 'utf8').replace(/\r\n/g, '\n');
    assert.equal(cur, render(src, env, gw), `standalone/${gw}/apisix.yaml is stale (run make apisix-render)`);
  }
});

test('F133 (Web build on play.<domain>): game routes allow the play origin; portal cookie routes do not', () => {
  for (const [envFile, play, portalOrigin] of [
    ['local.env', 'http://localhost:8060', 'http://localhost:5173'],
    ['staging.env.example', 'https://play.staging.eduworld.example', 'https://portal.staging.eduworld.example'],
  ]) {
    const d = doc(envFile, 'public');
    const origins = (id) => byId(d, id).plugins.cors.allow_origins.split(',');
    for (const id of ['auth-game-login', 'auth-game-token', 'auth-game-logout', 'game-client-config', 'player', 'realtime']) {
      assert.deepEqual(origins(`public-${id}`), [portalOrigin, play], `${envFile} ${id}`);
    }
    for (const id of ['auth-portal', 'auth-session', 'tenant-optional', 'portal-tenant']) {
      assert.deepEqual(origins(`public-${id}`), [portalOrigin], `${envFile} ${id} must not accept the play origin`);
    }
    for (const r of d.routes) {
      assert.ok(!r.plugins.cors.allow_origins.includes('*'), `${r.id}: no wildcard origin`);
      assert.equal(r.plugins.cors.allow_credential, true, `${r.id}: credential rule unchanged`);
      // The game client sends these on every call (F061-AC-16): the preflight must allow them.
      for (const h of ['X-Client-Version', 'X-Client-Platform', 'X-Device-Id', 'X-Tenant-Id', 'Idempotency-Key', 'traceparent']) {
        assert.ok(r.plugins.cors.allow_headers.split(',').includes(h), `${r.id} lacks ${h}`);
      }
    }
  }
});

test('F133: an origin list with a wildcard, a path or an unknown cors value is refused', () => {
  const env = loadEnv('local.env');
  const src = loadRoutes(env);
  assert.throws(() => render(src, { ...env, EW_CORS_GAME_ORIGINS: '*' }, 'public'), /not an origin/);
  assert.throws(() => render(src, { ...env, EW_CORS_GAME_ORIGINS: 'https://play.example/v1' }, 'public'), /not an origin/);
  assert.throws(() => render(src, { ...env, EW_CORS_GAME_ORIGINS: '' }, 'public'), /EW_CORS_GAME_ORIGINS is empty/);
  const bad = { ...src, routes: [{ ...src.routes[0], cors: 'any' }] };
  assert.throws(() => render(bad, env, 'public'), /cors must be "game"/);
});
