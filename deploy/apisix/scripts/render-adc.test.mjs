// F130 T052 — the ADC output (render.mjs --format=adc) is equivalent to the standalone render of the same source:
// same route set per gateway, uris, methods, priority, hosts, websocket flag, plugins (deep-equal) and upstream nodes;
// same plugin metadata; ADC shape (strict keys of the ADC 0.31 schema, inline upstream without id, labels for the
// canary). Real `adc lint/diff/sync` against throwaway gateways: deploy/apisix/scripts/validate-adc.sh.
//   node --test deploy/apisix/scripts/render-adc.test.mjs
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { build, adcObject, renderAdc, ADC_LABEL_UPSTREAM, ADC_LABEL_GATEWAY } from './render.mjs';
import { yaml, loadEnv, loadRoutes } from './lib.mjs';

const ENVS = ['local.env', 'staging.env.example'];
const GATEWAYS = ['public', 'internal'];
const load = (envFile) => { const env = loadEnv(envFile); return { env, src: loadRoutes(env) }; };

// Keys accepted by the ADC 0.31 declarative schema (additionalProperties: false) for the objects we emit.
const ADC_KEYS = {
  top: ['services', 'consumers', 'consumer_groups', 'global_rules', 'plugin_metadata', 'ssls'],
  service: ['id', 'name', 'description', 'labels', 'upstream', 'upstreams', 'plugins', 'path_prefix', 'strip_path_prefix', 'hosts', 'routes', 'stream_routes'],
  route: ['id', 'name', 'description', 'labels', 'hosts', 'uris', 'priority', 'timeout', 'vars', 'methods', 'enable_websocket', 'remote_addrs', 'plugins', 'filter_func'],
  upstream: ['name', 'description', 'labels', 'type', 'hash_on', 'key', 'checks', 'nodes', 'scheme', 'retries', 'retry_timeout', 'timeout', 'tls',
    'keepalive_pool', 'pass_host', 'upstream_host', 'service_name', 'discovery_type', 'discovery_args'],
};
const subset = (obj, allowed, what) => {
  for (const k of Object.keys(obj)) assert.ok(allowed.includes(k), `${what}: key "${k}" is not in the ADC schema`);
};

for (const envFile of ENVS) {
  for (const gw of GATEWAYS) {
    test(`F130 T052: ADC ${gw} (${envFile}) has the same routes, plugins and upstream nodes as the standalone render`, () => {
      const { env, src } = load(envFile);
      const std = build(src, env, gw);
      const adc = adcObject(src, env, gw);
      const adcRoutes = adc.services.flatMap((s) => s.routes.map((r) => ({ r, s })));
      assert.deepEqual(adcRoutes.map(({ r }) => r.name).sort(), std.routes.map((r) => r.id).sort(), 'same route set');
      for (const sr of std.routes) {
        const { r, s } = adcRoutes.find(({ r: x }) => x.name === sr.id);
        assert.deepEqual(r.uris, sr.uris, `${sr.id}: uris`);
        assert.deepEqual(r.methods, sr.methods, `${sr.id}: methods`);
        assert.equal(r.priority, sr.priority, `${sr.id}: priority`);
        assert.deepEqual(r.hosts, sr.host ? [sr.host] : undefined, `${sr.id}: host → hosts`);
        assert.equal(r.enable_websocket, sr.enable_websocket, `${sr.id}: websocket`);
        assert.deepEqual(r.plugins, sr.plugins, `${sr.id}: plugins`);
        const up = std.upstreams.find((u) => u.id === sr.upstream_id);
        assert.deepEqual(s.upstream.nodes, up.nodes, `${sr.id}: upstream nodes`);
        assert.deepEqual(s.upstream.timeout, up.timeout, `${sr.id}: upstream timeout`);
        assert.equal(s.upstream.scheme, up.scheme);
        assert.equal(s.upstream.type, up.type);
        assert.equal(r.labels[ADC_LABEL_UPSTREAM], sr.upstream_id, `${sr.id}: canary label`);
        assert.equal(r.labels[ADC_LABEL_GATEWAY], gw);
      }
      assert.deepEqual(Object.entries(adc.plugin_metadata).map(([id, m]) => ({ id, ...m })), std.plugin_metadata, 'plugin metadata');
    });

    test(`F130 T052: ADC ${gw} (${envFile}) uses only ADC schema keys, inline upstream without id`, () => {
      const { env, src } = load(envFile);
      const text = renderAdc(src, env, gw);
      const doc = yaml.load(text);
      subset(doc, ADC_KEYS.top, 'top level');
      const names = new Set();
      for (const s of doc.services) {
        subset(s, ADC_KEYS.service, `service ${s.name}`);
        subset(s.upstream, ADC_KEYS.upstream, `upstream of ${s.name}`);
        assert.equal(s.upstream.id, undefined, 'an inline service upstream has no id (ADC derives ids from names)');
        for (const n of s.upstream.nodes) assert.deepEqual(Object.keys(n).sort(), ['host', 'port', 'weight']);
        assert.ok(s.name.startsWith(`${gw}-`), 'service names are unique across gateways');
        for (const r of s.routes) {
          subset(r, ADC_KEYS.route, `route ${r.name}`);
          assert.ok(r.uris.length > 0 && r.name.length > 0);
          assert.ok(!names.has(r.name), `duplicate route ${r.name}`);
          names.add(r.name);
        }
      }
    });
  }
}

test('F130 T052: an internal prefix never reaches the public ADC file (ADR-0009)', () => {
  const { env, src } = load('staging.env.example');
  const uris = adcObject(src, env, 'public').services.flatMap((s) => s.routes.flatMap((r) => r.uris));
  for (const u of uris) assert.doesNotMatch(u, /^\/api\/v1\/(admin|sis|game-admin)(\/|$)|^\/internal\//, u);
});

test('F130 T052: the ADC file carries no secret (keys stay in the gateway environment)', () => {
  const { env, src } = load('staging.env.example');
  for (const gw of GATEWAYS) {
    const text = renderAdc(src, env, gw);
    assert.doesNotMatch(text, /admin_key|X-API-KEY|password:/i);
    for (const s of yaml.load(text).services) {
      for (const r of s.routes) if (r.plugins['ew-api-rate']) {
        assert.equal(r.plugins['ew-api-rate'].redis_username, 'apisix', 'Valkey ACL user of the gateway (deploy/valkey/users.acl.template)');
        assert.equal(r.plugins['ew-api-rate'].redis_password, undefined);
      }
    }
  }
});
