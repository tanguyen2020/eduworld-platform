// F004 T023/T044 (configuration part) — F004-AC-15, AC-28: check-no-body-logging.mjs.
//   node --test deploy/apisix/scripts/check-no-body-logging.test.mjs
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { check } from './check-no-body-logging.mjs';
import { render } from './render.mjs';
import { yaml, ROOT, loadEnv, loadRoutes } from './lib.mjs';

const src = () => ({ routes: [
  { id: 'game', uris: ['/api/v1/auth/game/login'], sensitive: true },
  { id: 'portal', uris: ['/api/v1/auth/portal/*'], sensitive: true },
  { id: 'player', uris: ['/api/v1/game/*'] },
] });
const rendered = () => ({ public: {
  routes: [
    { id: 'public-game', uris: ['/api/v1/auth/game/login'], plugins: { 'request-id': {} } },
    { id: 'public-portal', uris: ['/api/v1/auth/portal/*'], plugins: {} },
    { id: 'public-player', uris: ['/api/v1/game/*'], plugins: { 'http-logger': { uri: 'x' } } },
  ],
  plugin_metadata: [{ id: 'opentelemetry', additional_attributes: ['ew_tenant_id'] }],
} });
const config = () => ({ nginx_config: { http: { access_log_format: '$remote_addr "$request_method $uri" $status $request_time' } } });

test('a clean configuration has no problem', () => {
  assert.deepEqual(check(config(), rendered(), src()), []);
});

test('include_req_body on any route fails', () => {
  const r = rendered();
  r.public.routes[2].plugins['http-logger'].include_req_body = true;
  assert.match(check(config(), r, src()).join('\n'), /http-logger.include_req_body is set/);
});

test('any logger on /auth/game/* or /auth/portal/* fails', () => {
  for (const i of [0, 1]) {
    const r = rendered();
    r.public.routes[i].plugins['kafka-logger'] = {};
    assert.match(check(config(), r, src()).join('\n'), /logger kafka-logger on a sensitive route/);
  }
});

test('login routes must be marked sensitive', () => {
  const s = src();
  delete s.routes[1].sensitive;
  assert.match(check(config(), rendered(), s).join('\n'), /portal: serves a login prefix and must be sensitive/);
});

test('OpenTelemetry must not export bodies, headers or credentials', () => {
  for (const attr of ['request_body', 'http_authorization', 'http_cookie', 'http_x_ew_csrf', 'args']) {
    const r = rendered();
    r.public.plugin_metadata[0].additional_attributes.push(attr);
    assert.match(check(config(), r, src()).join('\n'), new RegExp(`opentelemetry attribute ${attr}`));
  }
  const r = rendered();
  r.public.plugin_metadata[0].additional_header_prefix_attributes = ['x-'];
  assert.match(check(config(), r, src()).join('\n'), /exports request headers/);
});

test('access log format with the request line, args, body or credentials fails', () => {
  for (const v of ['$request', '$request_uri', '$args', '$request_body', '$http_authorization', '$http_cookie', '$arg_ticket']) {
    const c = config();
    c.nginx_config.http.access_log_format += ` ${v}`;
    assert.equal(check(c, rendered(), src()).length, 1, v);
  }
});

test('logger log_format reading the body fails', () => {
  const r = rendered();
  r.public.routes[2].plugins['http-logger'].log_format = { body: '$request_body' };
  assert.match(check(config(), r, src()).join('\n'), /log_format reads \$request_body/);
});

test('the repository configuration passes', () => {
  const env = loadEnv('local.env');
  const s = loadRoutes(env);
  const r = Object.fromEntries(['public', 'internal'].map((gw) => [gw, yaml.load(render(s, env, gw).replace(/#END\s*$/, ''))]));
  const c = yaml.load(readFileSync(new URL('config.yaml', ROOT), 'utf8'));
  assert.deepEqual(check(c, r, s), []);
});
