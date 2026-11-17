// F004 T031 — fails when the gateways could log or trace a request body, credentials or query string
// (F004-AC-15, AC-16, AC-28; MONITORING §8): logger plugins with include_*_body, log formats or OTel
// attributes reading bodies, Authorization, cookies, X-EW-CSRF or args; any logger on /auth/game/* and
// /auth/portal/*; those routes not marked sensitive in routes.yaml.
//   node deploy/apisix/scripts/check-no-body-logging.mjs
import { readFileSync } from 'node:fs';
import { yaml, ROOT, loadEnv, loadRoutes, isMain } from './lib.mjs';

export const SENSITIVE_PREFIXES = ['/api/v1/auth/game/', '/api/v1/auth/portal/'];
// nginx variables that carry a body, credentials or the query string ($request = full request line).
const FORBIDDEN_VAR = /\$(request_body|request_uri|args|arg_\w+|query_string|http_authorization|http_cookie|cookie_\w+|http_x_ew_csrf|request)(?![a-z_])/;
const FORBIDDEN_ATTR = /body|authorization|cookie|csrf|password|otp|args|query|token/i;
const BODY_FLAGS = ['include_req_body', 'include_resp_body', 'include_req_body_expr', 'include_resp_body_expr'];

const isLogger = (name) => /logger|logging$|^syslog$/.test(name);

function strings(v, out = []) {
  if (typeof v === 'string') out.push(v);
  else if (Array.isArray(v)) v.forEach((x) => strings(x, out));
  else if (v && typeof v === 'object') Object.values(v).forEach((x) => strings(x, out));
  return out;
}

function checkPlugins(where, plugins, sensitive, p) {
  for (const [name, conf] of Object.entries(plugins ?? {})) {
    if (isLogger(name)) {
      if (sensitive) p.push(`${where}: logger ${name} on a sensitive route`);
      for (const f of BODY_FLAGS) if (conf?.[f]) p.push(`${where}: ${name}.${f} is set`);
      for (const s of strings(conf?.log_format ?? {})) if (FORBIDDEN_VAR.test(s)) p.push(`${where}: ${name}.log_format reads ${s}`);
    }
    if (name === 'opentelemetry') checkOtel(where, conf, p);
  }
}

function checkOtel(where, conf, p) {
  if (conf?.additional_header_prefix_attributes?.length) p.push(`${where}: opentelemetry exports request headers`);
  for (const a of conf?.additional_attributes ?? []) {
    if (FORBIDDEN_ATTR.test(a) || FORBIDDEN_VAR.test(`$${a}`)) p.push(`${where}: opentelemetry attribute ${a}`);
  }
}

// check(config, rendered, src): config = parsed config.yaml, rendered = {gateway: parsed standalone}, src = routes.yaml.
export function check(config, rendered, src) {
  const p = [];
  const http = config?.nginx_config?.http ?? {};
  for (const k of ['access_log_format', 'http_configuration_snippet', 'http_server_configuration_snippet']) {
    if (http[k] && FORBIDDEN_VAR.test(http[k])) p.push(`config.yaml nginx_config.http.${k} logs a body, credential or query string`);
  }
  for (const [name, conf] of Object.entries(config?.plugin_attr ?? {})) {
    if (isLogger(name)) for (const f of BODY_FLAGS) if (conf?.[f]) p.push(`config.yaml plugin_attr.${name}.${f} is set`);
  }
  for (const r of src.routes) {
    if (r.uris.some((u) => SENSITIVE_PREFIXES.some((pre) => u.startsWith(pre))) && r.sensitive !== true) {
      p.push(`routes.yaml ${r.id}: serves a login prefix and must be sensitive: true`);
    }
  }
  for (const [gw, doc] of Object.entries(rendered)) {
    for (const r of doc.routes ?? []) {
      const sensitive = r.uris.some((u) => SENSITIVE_PREFIXES.some((pre) => u.startsWith(pre)));
      checkPlugins(`${gw}/${r.id}`, r.plugins, sensitive, p);
    }
    for (const g of doc.global_rules ?? []) checkPlugins(`${gw}/global_rules/${g.id}`, g.plugins, true, p);
    for (const m of doc.plugin_metadata ?? []) {
      if (m.id === 'opentelemetry') checkOtel(`${gw}/plugin_metadata/opentelemetry`, m, p);
      if (isLogger(m.id)) for (const s of strings(m.log_format ?? {})) if (FORBIDDEN_VAR.test(s)) p.push(`${gw}/plugin_metadata/${m.id}: log_format reads ${s}`);
    }
  }
  return p;
}


if (isMain(import.meta.url)) {
  const config = yaml.load(readFileSync(new URL('config.yaml', ROOT), 'utf8'));
  const rendered = Object.fromEntries(['public', 'internal'].map((gw) =>
    [gw, yaml.load(readFileSync(new URL(`standalone/${gw}/apisix.yaml`, ROOT), 'utf8').replace(/#END\s*$/, ''))]));
  const problems = check(config, rendered, loadRoutes(loadEnv('local.env')));
  for (const x of problems) console.error(x);
  console.log(`check-no-body-logging: ${problems.length} problem(s)`);
  process.exit(problems.length ? 1 : 0);
}
