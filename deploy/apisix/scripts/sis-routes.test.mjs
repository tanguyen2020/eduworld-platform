// F017 T044/T056 — F017-AC-20, AC-30: SIS administration (/api/v1/sis/*) and the SIS webhook
// (/internal/sis/webhooks/*) exist only on the internal gateway; the webhook has no OIDC (HMAC in module
// identity), a partner IP allowlist, a body limit and no body logging.
//   node --test deploy/apisix/scripts/sis-routes.test.mjs
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { render } from './render.mjs';
import { yaml, loadEnv, loadRoutes } from './lib.mjs';

const doc = (envFile, gw) => {
  const env = loadEnv(envFile);
  return { env, d: yaml.load(render(loadRoutes(env), env, gw).replace(/#END\s*$/, '')) };
};

test('F017-AC-30: no SIS route on the public gateway', () => {
  for (const envFile of ['local.env', 'staging.env.example']) {
    for (const r of doc(envFile, 'public').d.routes) {
      for (const u of r.uris) assert.ok(!u.startsWith('/api/v1/sis') && !u.startsWith('/internal/'), `${envFile} ${r.id}: ${u}`);
    }
  }
});

test('F017-AC-20: the webhook route is internal, partner allowlist, no OIDC, bounded body', () => {
  for (const envFile of ['local.env', 'staging.env.example']) {
    const { env, d } = doc(envFile, 'internal');
    const r = d.routes.find((x) => x.id === 'internal-sis-webhook');
    assert.ok(r, `${envFile}: internal-sis-webhook`);
    assert.deepEqual(r.uris, ['/internal/sis/webhooks/*']);
    assert.deepEqual(r.methods, ['POST']);
    assert.equal(r.plugins['openid-connect'], undefined);
    assert.deepEqual(r.plugins['ip-restriction'].whitelist, env.EW_SIS_WEBHOOK_CIDRS.split(',').map((s) => s.trim()));
    assert.equal(r.plugins['client-control'].max_body_size, 8388608);
    const sis = d.routes.find((x) => x.uris.includes('/api/v1/sis/*'));
    assert.ok(sis.plugins['openid-connect'], 'SIS administration keeps OIDC');
  }
});
