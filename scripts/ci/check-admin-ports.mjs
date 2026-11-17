// F001-AC-06 — the admin port (container :9090) is never routed through APISIX and is published to the
// host only as 9100 (core-api), 9106 (realtime) or 9200 (worker) in local compose files.
// Usage: node scripts/ci/check-admin-ports.mjs [compose files…]  (defaults: deploy/compose/*.yml, deploy/apisix/apisix.yaml)
import fs from 'node:fs';
import path from 'node:path';
import { createRequire } from 'node:module';

const require = createRequire(import.meta.url);
const yaml = require('../node_modules/js-yaml');

const ADMIN_CONTAINER_PORT = 9090;
const ALLOWED_HOST = new Map([['core-api', 9100], ['realtime', 9106], ['worker', 9200]]);
const ADMIN_PORTS = new Set([9090, 9100, 9106, 9200]);

export function checkCompose(file, doc) {
  const out = [];
  for (const [name, svc] of Object.entries(doc?.services ?? {})) {
    for (const p of svc.ports ?? []) {
      const spec = typeof p === 'object' ? `${p.published ?? ''}:${p.target}` : String(p);
      const parts = spec.split(':');
      const target = Number(parts.at(-1).split('/')[0]);
      const published = parts.length > 1 ? Number(parts.at(-2)) : target;
      if (target === ADMIN_CONTAINER_PORT && ALLOWED_HOST.get(name) !== published) {
        out.push(`${file}: service ${name} publishes admin port 9090 as ${published} (allowed only ${ALLOWED_HOST.get(name) ?? 'none'})`);
      }
      if (target !== ADMIN_CONTAINER_PORT && [...ALLOWED_HOST.values()].includes(published) && ALLOWED_HOST.get(name) !== published) {
        out.push(`${file}: service ${name} takes host port ${published} reserved for admin ports`);
      }
    }
  }
  return out;
}

export function checkApisix(file, doc) {
  const out = [];
  const visit = (node, where) => {
    if (Array.isArray(node)) node.forEach((n, i) => visit(n, `${where}[${i}]`));
    else if (node && typeof node === 'object') {
      for (const [k, v] of Object.entries(node)) {
        if (k === 'nodes') {
          const targets = Array.isArray(v) ? v.map((n) => `${n.host}:${n.port}`) : Object.keys(v);
          for (const t of targets) {
            const port = Number(String(t).split(':').at(-1));
            if (ADMIN_PORTS.has(port)) out.push(`${file}: ${where}.nodes targets admin port ${t}`);
          }
        } else visit(v, `${where}.${k}`);
      }
    } else if (typeof node === 'string' && /\$\{\{?[A-Z_]*ADMIN|:(9090|9100|9106|9200)\b/.test(node)) {
      out.push(`${file}: ${where} references an admin port (${node})`);
    }
  };
  visit(doc, 'apisix');
  return out;
}

const args = process.argv.slice(2);
const composeFiles = args.length
  ? args.filter((f) => !f.endsWith('apisix.yaml'))
  : fs.readdirSync('deploy/compose').filter((f) => /\.ya?ml$/.test(f)).map((f) => path.join('deploy/compose', f));
const apisixFiles = args.length ? args.filter((f) => f.endsWith('apisix.yaml')) : ['deploy/apisix/apisix.yaml'];

const violations = [
  ...composeFiles.flatMap((f) => checkCompose(f, yaml.load(fs.readFileSync(f, 'utf8')))),
  ...apisixFiles.filter((f) => fs.existsSync(f)).flatMap((f) => checkApisix(f, yaml.load(fs.readFileSync(f, 'utf8').replace(/^#END\s*$/m, '')))),
];
for (const v of violations) console.error(v);
console.log(`check-admin-ports: ${composeFiles.length} compose, ${apisixFiles.length} apisix, ${violations.length} lỗi`);
process.exit(violations.length ? 1 : 0);
