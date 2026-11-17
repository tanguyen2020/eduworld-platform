// F004 T022 — compares routes/routes.yaml with the merged public spec (F004-AC-09, AC-11, AC-13, AC-14):
// every operation is served by a route on the right gateway (x-internal → internal only) with the right
// upstream, authentication, tenant header mode and module; internal prefixes never on the public gateway.
//   node deploy/apisix/scripts/check-routes.mjs
import { readFileSync } from 'node:fs';
import { yaml, ROOT, loadEnv, loadRoutes, isMain } from './lib.mjs';

const METHODS = ['get', 'put', 'post', 'delete', 'patch'];
const INTERNAL_PREFIXES = ['/api/v1/admin', '/api/v1/sis', '/api/v1/game-admin', '/api/v1/game-media', '/internal'];

export function operations(spec) {
  const out = [];
  for (const [path, item] of Object.entries(spec.paths)) {
    for (const m of METHODS) {
      const op = item[m];
      if (!op) continue;
      const params = [...(item.parameters ?? []), ...(op.parameters ?? [])].map((p) => (p.$ref ? p.$ref.split('/').pop() : p.name));
      out.push({
        id: op.operationId, method: m.toUpperCase(), path, uri: `/api/v1${path.replace(/\{[^}]+\}/g, 'x')}`,
        internal: op['x-internal'] === true, services: [].concat(op['x-service'] ?? []),
        tenant: params.includes('TenantIdHeader') ? 'required' : params.includes('TenantIdHeaderOptional') ? 'optional' : 'none',
        public: Array.isArray(op.security) && op.security.length === 0,
      });
    }
  }
  return out;
}

const matches = (pattern, uri) => (pattern.endsWith('*') ? uri.startsWith(pattern.slice(0, -1)) : uri === pattern);

// route returns the route APISIX would pick on gateway for (method, uri): highest priority wins.
export function route(src, gateway, method, uri) {
  return src.routes
    .filter((r) => (r.gateway === gateway || r.gateway === 'both') && (!r.methods || r.methods.includes(method))
      && r.uris.some((u) => matches(u, uri)))
    .sort((a, b) => (b.priority ?? 0) - (a.priority ?? 0))[0];
}

export function check(src, spec) {
  const p = [];
  for (const r of src.routes) {
    if (r.gateway !== 'internal') {
      for (const u of r.uris) {
        if (INTERNAL_PREFIXES.some((pre) => u.startsWith(pre))) p.push(`route ${r.id}: internal prefix ${u} on the public gateway`);
      }
    }
    for (const u of r.uris) {
      const want = u.startsWith('/realtime/') ? 'realtime' : 'core-api';
      if (r.upstream !== want) p.push(`route ${r.id}: ${u} must go to upstream ${want}`);
    }
    if (r.uris.some((u) => u === '/api/v1/*' || u === '/*')) p.push(`route ${r.id}: catch-all uri would expose internal prefixes`);
    if (r.auth === 'oidc' && !(r.rate && src.rate_profiles?.[r.rate])) p.push(`route ${r.id}: authenticated route without rate profile (api_rate_default)`);
  }
  for (const op of operations(spec)) {
    const where = `${op.method} ${op.path} (${op.id})`;
    if (op.path.startsWith('/admin/') && !op.internal) p.push(`${where}: /admin/* must be x-internal`);
    const pub = route(src, 'public', op.method, op.uri);
    const int = route(src, 'internal', op.method, op.uri);
    if (op.internal && pub) p.push(`${where}: internal operation reachable on the public gateway (route ${pub.id})`);
    const r = op.internal ? int : pub;
    if (!r) {
      p.push(`${where}: no route on the ${op.internal ? 'internal' : 'public'} gateway`);
      continue;
    }
    if (op.public && r.auth !== 'none') p.push(`${where}: public operation behind ${r.auth} (route ${r.id})`);
    if (!op.public && r.auth !== 'oidc') p.push(`${where}: authenticated operation without openid-connect (route ${r.id})`);
    if (r.tenant_header === 'required' && op.tenant !== 'required') p.push(`${where}: route ${r.id} requires X-Tenant-Id, operation is ${op.tenant}`);
    if (r.tenant_header === 'none' && op.tenant === 'required') p.push(`${where}: tenant-scoped operation on route ${r.id} without ew-tenant-header`);
    for (const s of op.services) if (!r.modules.includes(s)) p.push(`${where}: module ${s} not declared on route ${r.id}`);
  }
  return p;
}

if (isMain(import.meta.url)) {
  const src = loadRoutes(loadEnv('local.env'));
  const spec = yaml.load(readFileSync(new URL('../../specs/openapi/eduworld-api.v1.yaml', ROOT), 'utf8'));
  const problems = check(src, spec);
  for (const x of problems) console.error(x);
  console.log(`check-routes: ${operations(spec).length} operations, ${src.routes.length} routes, ${problems.length} problem(s)`);
  process.exit(problems.length ? 1 : 0);
}
