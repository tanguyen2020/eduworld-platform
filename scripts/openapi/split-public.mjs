#!/usr/bin/env node
// Tách spec public gộp `specs/openapi/eduworld-api.v1.yaml` (NGUỒN SỰ THẬT) thành
// `specs/openapi/modules/<module>.v1.yaml` theo `x-service` (tên module sở hữu) của từng operation (ADR-0074).
// File sinh ra KHÔNG được sửa tay; sửa spec gộp rồi chạy lại script này.
//
// - `x-service: <module>`            → operation thuộc 1 module.
// - `x-service: [a, b]` + `x-service-routing: { param, map }`
//                                  → operation được nhân cho từng module, enum của tham số `param`
//                                    thu hẹp theo `map` (vd. /game-admin/{config_type}).
// - components: chỉ giữ phần được tham chiếu (bao đóng bắc cầu của $ref).
//
// Chạy:  npm ci --prefix scripts && node scripts/openapi/split-public.mjs [--check]
//   --check: không ghi file, trả exit 1 nếu file sinh ra khác bản đang có (dùng trong CI).
import { createRequire } from 'node:module';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const require = createRequire(import.meta.url);
const yaml = require('js-yaml');

const here = path.dirname(fileURLToPath(import.meta.url));
const repo = path.resolve(here, '..', '..');
const SRC = path.join(repo, 'specs/openapi/eduworld-api.v1.yaml');
const OUT_DIR = path.join(repo, 'specs/openapi/modules');
const SERVICES = ['identity', 'learning', 'game', 'competition', 'chat'];
const METHODS = ['get', 'put', 'post', 'delete', 'options', 'head', 'patch', 'trace'];
const checkOnly = process.argv.includes('--check');

const spec = yaml.load(fs.readFileSync(SRC, 'utf8'));
const clone = (o) => JSON.parse(JSON.stringify(o));

function resolveLocal(ref) {
  if (!ref.startsWith('#/')) throw new Error(`Chỉ hỗ trợ $ref nội bộ: ${ref}`);
  return ref.slice(2).split('/').map((s) => s.replace(/~1/g, '/').replace(/~0/g, '~'))
    .reduce((o, k) => (o == null ? undefined : o[k]), spec);
}

function collectRefs(node, acc) {
  if (Array.isArray(node)) { node.forEach((n) => collectRefs(n, acc)); return; }
  if (node && typeof node === 'object') {
    for (const [k, v] of Object.entries(node)) {
      if (k === '$ref' && typeof v === 'string') {
        if (!acc.has(v)) { acc.add(v); collectRefs(resolveLocal(v), acc); }
      } else collectRefs(v, acc);
    }
  }
}

function narrowParam(op, param, allowed) {
  // Thay tham số `param` (inline hoặc $ref) bằng bản inline có enum thu hẹp.
  op.parameters = (op.parameters || []).map((p) => {
    const real = p.$ref ? resolveLocal(p.$ref) : p;
    if (real && real.name === param) {
      const copy = clone(real);
      const sch = copy.schema && copy.schema.$ref ? clone(resolveLocal(copy.schema.$ref)) : copy.schema;
      copy.schema = { ...sch, enum: allowed };
      return copy;
    }
    return p;
  });
}

const perService = Object.fromEntries(SERVICES.map((s) => [s, {}]));
let total = 0;
for (const [p, item] of Object.entries(spec.paths)) {
  for (const m of METHODS) {
    const op = item[m];
    if (!op) continue;
    total++;
    const xs = op['x-service'];
    const svcs = Array.isArray(xs) ? xs : [xs];
    if (!xs || svcs.some((s) => !SERVICES.includes(s))) throw new Error(`Thiếu/sai x-service: ${m.toUpperCase()} ${p}`);
    for (const s of svcs) {
      const copy = clone(op);
      if (Array.isArray(xs)) {
        const routing = op['x-service-routing'];
        if (!routing) throw new Error(`x-service nhiều giá trị cần x-service-routing: ${p}`);
        const allowed = Object.entries(routing.map).filter(([, v]) => v === s).map(([k]) => k);
        narrowParam(copy, routing.param, allowed);
        copy['x-service'] = s;
        delete copy['x-service-routing'];
        copy.operationId = `${copy.operationId}_${s}`;
      }
      const target = (perService[s][p] = perService[s][p] || {});
      for (const [k, v] of Object.entries(item)) if (!METHODS.includes(k) && !(k in target)) target[k] = clone(v);
      target[m] = copy;
    }
  }
}

let changed = 0;
for (const s of SERVICES) {
  const paths = perService[s];
  const refs = new Set();
  collectRefs(paths, refs);
  collectRefs(spec.security || [], refs);
  const components = {};
  for (const r of refs) {
    const [, , type, name] = r.split('/');
    components[type] = components[type] || {};
    components[type][name] = clone(resolveLocal(r));
  }
  components.securitySchemes = clone(spec.components.securitySchemes);
  const usedTags = new Set(Object.values(paths).flatMap((it) => METHODS.flatMap((m) => (it[m] && it[m].tags) || [])));
  const doc = {
    openapi: spec.openapi,
    info: {
      ...clone(spec.info),
      title: `EduWorld module ${s} — Public API`,
      description: `FILE SINH TỰ ĐỘNG từ specs/openapi/eduworld-api.v1.yaml bởi scripts/openapi/split-public.mjs — KHÔNG sửa tay.\n\nCác operation public do module \`${s}\` xử lý (ADR-0074). APISIX định tuyến mọi route REST tới binary \`core-api\`.`,
    },
    servers: clone(spec.servers),
    security: clone(spec.security),
    tags: (spec.tags || []).filter((t) => usedTags.has(t.name)),
    paths,
    components,
  };
  const text = '# GENERATED — do not edit. Source: specs/openapi/eduworld-api.v1.yaml\n' +
    yaml.dump(doc, { lineWidth: -1, noRefs: true, sortKeys: false });
  const out = path.join(OUT_DIR, `${s}.v1.yaml`);
  const prev = fs.existsSync(out) ? fs.readFileSync(out, 'utf8') : '';
  if (prev !== text) {
    changed++;
    if (!checkOnly) fs.writeFileSync(out, text);
  }
  const ops = Object.values(paths).reduce((n, it) => n + METHODS.filter((m) => it[m]).length, 0);
  console.log(`${s}.v1.yaml: ${Object.keys(paths).length} path, ${ops} operation, ${Object.values(components).reduce((n, c) => n + Object.keys(c).length, 0)} component`);
}
console.log(`tổng operation trong spec gộp: ${total}`);
if (checkOnly && changed) { console.error(`${changed} file khác bản sinh — chạy lại split-public.mjs`); process.exit(1); }
