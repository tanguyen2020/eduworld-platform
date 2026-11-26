// F004 T006 — renders routes/routes.yaml for one environment into APISIX standalone configs
// standalone/public/apisix.yaml and standalone/internal/apisix.yaml (ending with #END).
//   node deploy/apisix/scripts/render.mjs [--env=local] [--check]
import { readFileSync, writeFileSync, mkdirSync } from 'node:fs';
import { yaml, ROOT, loadEnv, loadEnvFile, loadRoutes, isMain } from './lib.mjs';

// X-Client-Version and X-Client-Platform are sent by the game client on every call (F061-AC-16); a Web build on
// another origin (play.<domain>, F133) needs them in the preflight answer.
const CORS_HEADERS = 'Authorization,Content-Type,X-Tenant-Id,Idempotency-Key,If-Match,X-Request-Id,X-Device-Id,X-EW-CSRF,traceparent,X-Client-Version,X-Client-Platform';

// An allowlisted origin is scheme://host[:port] only: no wildcard, no path, no credentials (CORS stays an
// allowlist, SECURITY_ARCHITECTURE §5).
const ORIGIN = /^https?:\/\/[a-z0-9]([a-z0-9-]*[a-z0-9])?(\.[a-z0-9]([a-z0-9-]*[a-z0-9])?)*(:\d{1,5})?$/;

function originList(env, name) {
  const list = String(env[name] ?? '').split(',').map((s) => s.trim()).filter(Boolean);
  for (const o of list) {
    if (!ORIGIN.test(o)) throw new Error(`${name}: "${o}" is not an origin (scheme://host[:port], no wildcard)`);
  }
  return list;
}

// corsOrigins: the portal origin(s) on every public route; routes marked `cors: game` (the game client's
// routes, F133) also accept the Web play origin(s) of EW_CORS_GAME_ORIGINS. Cookie routes (/auth/portal/*) never
// get the play origin.
export function corsOrigins(env, r) {
  if (r.cors !== undefined && r.cors !== 'game') throw new Error(`route ${r.id}: cors must be "game" or absent`);
  const list = originList(env, 'EW_CORS_ALLOWED_ORIGINS');
  if (list.length === 0) throw new Error('EW_CORS_ALLOWED_ORIGINS is empty');
  if (r.cors === 'game') {
    const game = originList(env, 'EW_CORS_GAME_ORIGINS');
    if (game.length === 0) throw new Error(`route ${r.id}: EW_CORS_GAME_ORIGINS is empty`);
    list.push(...game);
  }
  return [...new Set(list)].join(',');
}

function upstream(id, hostport) {
  const [host, port] = hostport.split(':');
  return { id, type: 'roundrobin', scheme: 'http', nodes: [{ host, port: Number(port), weight: 1 }],
    timeout: { connect: 3, send: 30, read: 30 } };
}

// build returns the gateway configuration as APISIX objects (routes with upstream_id, upstreams with id, plugin
// metadata with id) — the single model behind both outputs: render() (standalone, local) and renderAdc() (ADC,
// traditional/etcd gateways of dev/staging/production, DEPLOYMENT §5).
export function build(src, env, gateway) {
  const routes = [];
  for (const r of src.routes) {
    if (r.gateway !== gateway && r.gateway !== 'both') continue;
    const plugins = {
      'request-id': { header_name: 'X-Request-Id', include_in_response: true },
      prometheus: { prefer_name: true },
      opentelemetry: { sampler: { name: 'always_on' } },
    };
    if (gateway === 'public') {
      plugins.cors = { allow_origins: corsOrigins(env, r), allow_methods: 'GET,POST,PUT,PATCH,DELETE,OPTIONS',
        allow_headers: CORS_HEADERS, expose_headers: 'X-Request-Id,Idempotent-Replayed,Retry-After,RateLimit-Limit,RateLimit-Remaining,RateLimit-Reset,ETag',
        allow_credential: true, max_age: 600 };
    }
    if (gateway === 'internal') {
      plugins['ip-restriction'] = { whitelist: env[r.ip_allow ?? 'EW_ADMIN_CIDRS'].split(',').map((s) => s.trim()).filter(Boolean),
        message: 'Forbidden' };
    }
    if (r.body_max) plugins['client-control'] = { max_body_size: r.body_max };
    if (r.auth === 'oidc') {
      plugins['openid-connect'] = {
        client_id: env.EW_OIDC_AUDIENCE, client_secret: 'bearer-only-not-used', discovery: env.EW_OIDC_DISCOVERY,
        bearer_only: true, use_jwks: true, token_signing_alg_values_expected: 'RS256',
        set_access_token_header: false, set_id_token_header: false, set_userinfo_header: false,
        claim_validator: {
          issuer: { valid_issuers: env.EW_OIDC_ISSUERS.split(',').map((s) => s.trim()) },
          audience: { claim: 'aud', required: true, match_with_client_id: true },
        },
      };
    }
    if (r.tenant_header && r.tenant_header !== 'none') plugins['ew-tenant-header'] = { mode: r.tenant_header };
    if (r.login) plugins['ew-login-key'] = { ...src.login_profiles[r.login] };
    if (r.rate) {
      // redis_username: the Valkey ACL user of the gateway (deploy/valkey/users.acl.template, user apisix); the
      // password stays in the gateway environment (EW_APISIX_VALKEY_PASSWORD).
      plugins['ew-api-rate'] = { name: r.rate, ...src.rate_profiles[r.rate], redis_host: src.rate_store.host,
        redis_port: Number(src.rate_store.port), redis_username: String(src.rate_store.username), allow_degradation: true };
    }
    if (r.websocket) {
      plugins['limit-conn'] = { conn: Number(env.EW_REALTIME_CONN_PER_IP), burst: 0, default_conn_delay: 0.1,
        key_type: 'var', key: 'remote_addr', rejected_code: 503 };
    }
    if (gateway === 'public' && env.EW_TLS === 'true') {
      plugins.redirect = { http_to_https: true };
      plugins['response-rewrite'] = { headers: { set: { 'Strict-Transport-Security': 'max-age=31536000; includeSubDomains' } } };
    }
    const route = { id: `${gateway}-${r.id}`, uris: r.uris, priority: r.priority ?? 0, upstream_id: r.upstream, plugins };
    if (r.methods) route.methods = r.methods;
    if (r.host) route.host = r.host;
    if (r.websocket) route.enable_websocket = true;
    routes.push(route);
  }
  const used = new Set(routes.map((r) => r.upstream_id));
  const upstreams = Object.entries(src.upstreams).filter(([id]) => used.has(id)).map(([id, hp]) => upstream(id, hp));
  // OpenTelemetry collector (APISIX 3.13 reads it from plugin metadata). Spans carry the tenant
  // (ctx.var.ew_tenant_id from ew-tenant-header), never bodies or credentials.
  const plugin_metadata = [{ id: 'opentelemetry', trace_id_source: 'x-request-id',
    resource: { 'service.name': `apisix-${gateway}` },
    collector: { address: env.EW_OTEL_COLLECTOR, request_timeout: 3 },
    set_ngx_var: false, additional_attributes: ['ew_tenant_id'] }];
  return { routes, upstreams, plugin_metadata };
}

export function render(src, env, gateway) {
  const header = `# Code generated by deploy/apisix/scripts/render.mjs from routes/routes.yaml (${gateway} gateway). DO NOT EDIT.\n`;
  return `${header}${yaml.dump(build(src, env, gateway), { lineWidth: 140, noRefs: true })}#END\n`;
}

// Labels ADC writes on every service and route: the canary (scripts/deploy/canary.sh) selects the routes of upstream
// core-api by ew-upstream, because ADC routes reference their service (service_id), not an upstream id.
export const ADC_LABEL_GATEWAY = 'ew-gateway';
export const ADC_LABEL_UPSTREAM = 'ew-upstream';

// adcObject maps the APISIX model onto the ADC declarative format (API7 ADC, backend apisix): one service per upstream
// (inline upstream, no id — ADC derives ids from names), its routes nested with `hosts` (array) and labels; plugin
// metadata becomes a map keyed by plugin name. Same route names/uris/methods/priority/plugins as the standalone
// output (render-adc.test.mjs proves the equivalence).
export function adcObject(src, env, gateway) {
  const m = build(src, env, gateway);
  const services = m.upstreams.map((u) => {
    const { id, ...up } = u;
    const routes = m.routes.filter((r) => r.upstream_id === id).map((r) => {
      const { id: name, upstream_id, host, ...rest } = r;
      const route = { name, ...rest };
      if (host) route.hosts = [host];
      route.labels = { [ADC_LABEL_GATEWAY]: gateway, [ADC_LABEL_UPSTREAM]: upstream_id };
      return route;
    });
    return { name: `${gateway}-${id}`, labels: { [ADC_LABEL_GATEWAY]: gateway, [ADC_LABEL_UPSTREAM]: id },
      upstream: { name: id, ...up }, routes };
  });
  const plugin_metadata = Object.fromEntries(m.plugin_metadata.map(({ id, ...meta }) => [id, meta]));
  return { services, plugin_metadata };
}

export function renderAdc(src, env, gateway) {
  const header = `# Code generated by deploy/apisix/scripts/render.mjs --format=adc from routes/routes.yaml (${gateway} gateway). DO NOT EDIT.\n` +
    '# Applied by scripts/deploy/apisix-sync.sh (CI job apisix:sync, DEPLOYMENT §5); never committed with real values.\n';
  return `${header}${yaml.dump(adcObject(src, env, gateway), { lineWidth: 140, noRefs: true })}`;
}

if (isMain(import.meta.url)) {
  // --env=staging reads routes/env/staging.env; a name with a dot is a file name (staging.env.example);
  // --env-file=<path> reads any env file (CI: the GitHub Environment variable APISIX_ROUTES_ENV written to a temp file).
  // --out=<dir> writes <dir>/<gateway>/apisix.yaml instead of standalone/ (CI diff of a PR).
  // --format=adc --out=<dir> writes <dir>/<gateway>.adc.yaml (ADC, rendered on the fly — never committed: real
  // environment values stay out of Git, AGENTS R3).
  const arg = (k) => process.argv.find((a) => a.startsWith(`--${k}=`))?.slice(k.length + 3);
  const envName = arg('env') ?? 'local';
  const envFile = arg('env-file');
  const outDir = arg('out');
  const format = arg('format') ?? 'standalone';
  const check = process.argv.includes('--check');
  const env = envFile ? loadEnvFile(envFile) : loadEnv(envName.includes('.') ? envName : `${envName}.env`);
  const src = loadRoutes(env);
  if (format === 'adc') {
    if (!outDir) { console.error('--format=adc needs --out=<dir>'); process.exit(2); }
    mkdirSync(outDir, { recursive: true });
    for (const gw of ['public', 'internal']) {
      const out = renderAdc(src, env, gw);
      writeFileSync(`${outDir}/${gw}.adc.yaml`, out);
      console.log(`rendered ${gw}.adc.yaml (${adcObject(src, env, gw).services.reduce((n, s) => n + s.routes.length, 0)} routes)`);
    }
    process.exit(0);
  }
  if (format !== 'standalone') { console.error(`unknown --format=${format}`); process.exit(2); }
  let stale = 0;
  for (const gw of ['public', 'internal']) {
    const out = render(src, env, gw);
    if (outDir) {
      mkdirSync(`${outDir}/${gw}`, { recursive: true });
      writeFileSync(`${outDir}/${gw}/apisix.yaml`, out);
      continue;
    }
    const file = new URL(`standalone/${gw}/apisix.yaml`, ROOT);
    if (check) {
      let cur = '';
      try { cur = readFileSync(file, 'utf8').replace(/\r\n/g, '\n'); } catch { /* missing */ }
      if (cur !== out) { console.error(`stale: standalone/${gw}/apisix.yaml (run make apisix-render)`); stale++; }
      continue;
    }
    mkdirSync(new URL(`standalone/${gw}/`, ROOT), { recursive: true });
    writeFileSync(file, out);
    console.log(`rendered standalone/${gw}/apisix.yaml (${(out.match(new RegExp(`^  - id: ${gw}-`, "gm")) ?? []).length} routes)`);
  }
  process.exit(stale ? 1 : 0);
}
