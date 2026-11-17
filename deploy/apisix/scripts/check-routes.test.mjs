// F004 T019 — F004-AC-14 (plus AC-09/AC-13 at the configuration level): check-routes.mjs.
//   node --test deploy/apisix/scripts/check-routes.test.mjs
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { check, operations, route } from './check-routes.mjs';
import { yaml, ROOT, loadEnv, loadRoutes } from './lib.mjs';

const base = () => ({
  rate_profiles: { api: { read_count: 300, write_count: 60 } },
  routes: [
    { id: 'player', gateway: 'public', uris: ['/api/v1/game/*'], upstream: 'core-api', modules: ['game'], auth: 'oidc', rate: 'api', tenant_header: 'required', priority: 100 },
    { id: 'cfg', gateway: 'public', uris: ['/api/v1/game/client-config'], methods: ['GET'], upstream: 'core-api', modules: ['game'], auth: 'none', tenant_header: 'none', priority: 250 },
    { id: 'admin', gateway: 'internal', uris: ['/api/v1/admin/*'], upstream: 'core-api', modules: ['identity'], auth: 'oidc', rate: 'api', tenant_header: 'optional', priority: 100 },
    { id: 'rt', gateway: 'public', uris: ['/realtime/v1'], upstream: 'realtime', modules: ['realtime'], auth: 'ticket', tenant_header: 'none', websocket: true },
  ],
});
const T = [{ $ref: '#/components/parameters/TenantIdHeader' }];
const spec = () => ({
  paths: {
    '/game/profile': { get: { operationId: 'getProfile', 'x-service': 'game', parameters: T } },
    '/game/client-config': { get: { operationId: 'getClientConfig', 'x-service': 'game', security: [] } },
    '/admin/tenants': { get: { operationId: 'listTenants', 'x-service': 'identity', 'x-internal': true } },
  },
});

test('valid configuration has no problem', () => {
  assert.deepEqual(check(base(), spec()), []);
});

test('the highest priority route wins', () => {
  assert.equal(route(base(), 'public', 'GET', '/api/v1/game/client-config').id, 'cfg');
  assert.equal(route(base(), 'public', 'POST', '/api/v1/game/client-config').id, 'player');
  assert.equal(route(base(), 'public', 'GET', '/api/v1/admin/tenants'), undefined);
});

test('/api/v1/* on an upstream other than core-api fails', () => {
  const s = base();
  s.routes[0].upstream = 'realtime';
  assert.match(check(s, spec()).join('\n'), /player: \/api\/v1\/game\/\* must go to upstream core-api/);
});

test('/realtime/v1 on an upstream other than realtime fails', () => {
  const s = base();
  s.routes[3].upstream = 'core-api';
  assert.match(check(s, spec()).join('\n'), /rt: \/realtime\/v1 must go to upstream realtime/);
});

test('an operation without route fails', () => {
  const sp = spec();
  sp.paths['/lms/classes'] = { get: { operationId: 'listClasses', 'x-service': 'learning', parameters: T } };
  assert.match(check(base(), sp).join('\n'), /listClasses\): no route on the public gateway/);
});

test('an internal prefix on the public gateway fails (route and operation)', () => {
  const s = base();
  s.routes[2].gateway = 'both';
  const p = check(s, spec()).join('\n');
  assert.match(p, /internal prefix \/api\/v1\/admin\/\* on the public gateway/);
  assert.match(p, /listTenants\): internal operation reachable on the public gateway/);
});

test('an authenticated route without rate profile fails (api_rate_default)', () => {
  const s = base();
  delete s.routes[0].rate;
  assert.match(check(s, spec()).join('\n'), /player: authenticated route without rate profile/);
  const s2 = base();
  s2.routes[2].rate = 'missing';
  assert.match(check(s2, spec()).join('\n'), /admin: authenticated route without rate profile/);
});

test('a catch-all route fails', () => {
  const s = base();
  s.routes[2].uris = ['/api/v1/*'];
  assert.match(check(s, spec()).join('\n'), /catch-all/);
});

test('/admin/* without x-internal fails', () => {
  const sp = spec();
  delete sp.paths['/admin/tenants'].get['x-internal'];
  assert.match(check(base(), sp).join('\n'), /\/admin\/\* must be x-internal/);
});

test('authentication, tenant header and module must match the operation', () => {
  const s = base();
  s.routes[0].auth = 'none';
  s.routes[0].tenant_header = 'none';
  s.routes[0].modules = ['learning'];
  const p = check(s, spec()).join('\n');
  assert.match(p, /authenticated operation without openid-connect/);
  assert.match(p, /tenant-scoped operation on route player without ew-tenant-header/);
  assert.match(p, /module game not declared on route player/);
  const s2 = base();
  s2.routes[1].auth = 'oidc';
  assert.match(check(s2, spec()).join('\n'), /public operation behind oidc/);
  const sp = spec();
  sp.paths['/game/profile'].get.parameters = [];
  assert.match(check(base(), sp).join('\n'), /route player requires X-Tenant-Id, operation is none/);
});

test('operations() reads path and operation parameters', () => {
  const sp = spec();
  sp.paths['/me/permissions'] = { parameters: [{ $ref: '#/components/parameters/TenantIdHeaderOptional' }], get: { operationId: 'p' } };
  const op = operations(sp).find((o) => o.id === 'p');
  assert.equal(op.tenant, 'optional');
  assert.equal(op.uri, '/api/v1/me/permissions');
});

test('the repository routes match the merged spec', () => {
  const src = loadRoutes(loadEnv('local.env'));
  const sp = yaml.load(readFileSync(new URL('../../specs/openapi/eduworld-api.v1.yaml', ROOT), 'utf8'));
  assert.deepEqual(check(src, sp), []);
});
