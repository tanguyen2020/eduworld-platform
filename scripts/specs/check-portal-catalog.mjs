#!/usr/bin/env node
// Kiểm tra danh mục menu/action của Admin Portal (ADR-0090):
//   1. specs/portal/menu-catalog.yaml hợp lệ theo specs/schemas/portal-menu-catalog.schema.json (Ajv 2020 strict).
//   2. Khóa duy nhất; khóa menu = `menu.<workspace>.<menu>`, khóa action = `action.<workspace>.<menu>.<action>` nằm đúng dưới
//      workspace/menu cha; `label_key` = `portal.<key>`; `path` của menu nằm dưới `path` của workspace, không trùng.
//   3. Quy tắc cha–con và danh sách mục được bảo vệ: menu `can_disable: false` đúng bằng danh sách của ADR-0090 mục 2;
//      action có `can_disable` bằng menu cha.
//   4. Permission trong `required_permissions` có trong catalog permission (docs/04-admin-portal/ADMIN_PORTAL.md §2.2).
//   5. Hai chiều catalog ↔ OpenAPI gộp (specs/openapi/eduworld-api.v1.yaml):
//      - mọi `operation_ids` tồn tại và có `x-portal-action` trỏ đúng action (dạng `op[param=value]` ↔ `x-portal-action: {param, map}`);
//      - mọi operation có `x-portal-action` thuộc một action trong catalog và được action đó liệt kê; dạng `{param, map}` phủ đủ enum
//        của tham số; mỗi operation thuộc đúng một action;
//      - mọi operation dưới /cms, /lms, /tenant, /schools, /admin, /game-admin, /game-media, /sis có `x-portal-action`;
//        `x-portal-shell: true` (vỏ portal, không bị featuregate chặn) không đi cùng `x-portal-action` và không nằm trong catalog.
// Chạy:  npm ci --prefix scripts && node scripts/specs/check-portal-catalog.mjs   (exit 1 nếu có lỗi)
import { createRequire } from 'node:module';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const require = createRequire(import.meta.url);
const yaml = require('js-yaml');
const Ajv2020 = require('ajv/dist/2020').default;

const here = path.dirname(fileURLToPath(import.meta.url));
const repo = path.resolve(here, '..', '..');
const CATALOG = path.join(repo, 'specs/portal/menu-catalog.yaml');
const SCHEMA = path.join(repo, 'specs/schemas/portal-menu-catalog.schema.json');
const OPENAPI = path.join(repo, 'specs/openapi/eduworld-api.v1.yaml');
const ADMIN_PORTAL = path.join(repo, 'docs/04-admin-portal/ADMIN_PORTAL.md');
const METHODS = ['get', 'put', 'post', 'delete', 'patch'];
const PORTAL_PREFIX = /^\/(cms|lms|tenant|schools|admin|game-admin|game-media|sis)(\/|:|$)/;
// ADR-0090 mục 2: menu không tắt được (mọi action của chúng cũng không tắt được).
const PROTECTED_MENUS = [
  'menu.school_lms.access_control', 'menu.school_lms.safety_notifications', 'menu.school_lms.trash',
  'menu.platform_admin.tenants', 'menu.platform_admin.access_control', 'menu.platform_admin.feature_toggles',
  'menu.platform_admin.audit_logs', 'menu.platform_admin.trash', 'menu.live_ops.moderation_cases', 'menu.live_ops.sanctions',
];

const errors = [];
const err = (m) => errors.push(m);

// ---------- 1. schema
const catalog = yaml.load(fs.readFileSync(CATALOG, 'utf8'));
const ajv = new Ajv2020({ strict: true, allErrors: true });
const validate = ajv.compile(JSON.parse(fs.readFileSync(SCHEMA, 'utf8')));
if (!validate(catalog)) {
  for (const e of validate.errors) err(`schema: ${e.instancePath || '/'} ${e.message}`);
  report();
}

// ---------- 4 (chuẩn bị). permission catalog từ ADMIN_PORTAL §2.2
function permissionCatalog() {
  const md = fs.readFileSync(ADMIN_PORTAL, 'utf8').split('\n');
  const start = md.findIndex((l) => /^### 2\.2\./.test(l));
  const end = md.findIndex((l, i) => i > start && /^### /.test(l));
  const perms = new Set();
  for (const line of md.slice(start, end)) {
    if (!line.startsWith('|')) continue;
    // `nhóm.*` (`a`, `b.c` …) → nhóm.a, nhóm.b.c ; `x.y/z` → x.y, x.z ; mã đầy đủ giữ nguyên
    for (const m of line.matchAll(/`([a-z_]+(?:\.[a-z_]+)*)\.\*`\s*\(([^)]*)\)/g)) {
      for (const s of m[2].matchAll(/`([a-z_]+(?:\.[a-z_]+)*)`/g)) perms.add(`${m[1]}.${s[1]}`);
    }
    for (const m of line.matchAll(/`([a-z_]+(?:\.[a-z_]+)+(?:\/[a-z_]+)*)`/g)) {
      const [first, ...alts] = m[1].split('/');
      perms.add(first);
      const base = first.slice(0, first.lastIndexOf('.'));
      for (const a of alts) perms.add(`${base}.${a}`);
    }
  }
  return perms;
}
const PERMS = permissionCatalog();

// ---------- 2, 3, 4. cấu trúc catalog
const keys = new Set();
const actions = new Map(); // action key -> { ops: Set<string> }
const paths = new Set();
let menuCount = 0, actionCount = 0, protMenus = 0, protActions = 0;
const protectedSeen = [];
for (const ws of catalog.workspaces) {
  if (keys.has(ws.key)) err(`workspace trùng: ${ws.key}`);
  keys.add(ws.key);
  if (ws.label_key !== `portal.workspace.${ws.key}`) err(`${ws.key}: label_key phải là portal.workspace.${ws.key}`);
  const orders = new Set();
  for (const m of ws.menus) {
    menuCount++;
    if (keys.has(m.key)) err(`khóa trùng: ${m.key}`);
    keys.add(m.key);
    const [, mws, mname] = m.key.split('.');
    if (mws !== ws.key) err(`${m.key}: nằm dưới workspace ${ws.key}`);
    if (m.label_key !== `portal.${m.key}`) err(`${m.key}: label_key phải là portal.${m.key}`);
    if (!m.path.startsWith(`${ws.path}/`)) err(`${m.key}: path ${m.path} không nằm dưới ${ws.path}`);
    if (paths.has(m.path)) err(`${m.key}: path trùng ${m.path}`);
    paths.add(m.path);
    if (orders.has(m.order)) err(`${m.key}: order ${m.order} trùng trong ${ws.key}`);
    orders.add(m.order);
    if (!m.can_disable) { protMenus++; protectedSeen.push(m.key); }
    for (const p of m.required_permissions) if (!PERMS.has(p)) err(`${m.key}: permission không có trong ADMIN_PORTAL §2.2: ${p}`);
    const akeys = new Set();
    for (const a of m.actions) {
      actionCount++;
      if (keys.has(a.key)) err(`khóa trùng: ${a.key}`);
      keys.add(a.key);
      const parts = a.key.split('.');
      if (parts[1] !== mws || parts[2] !== mname) err(`${a.key}: phải nằm dưới menu ${m.key}`);
      if (akeys.has(parts[3])) err(`${a.key}: trùng trong menu`);
      akeys.add(parts[3]);
      if (a.label_key !== `portal.${a.key}`) err(`${a.key}: label_key phải là portal.${a.key}`);
      if (a.can_disable !== m.can_disable) err(`${a.key}: can_disable phải bằng menu cha (${m.can_disable})`);
      if (!a.can_disable) protActions++;
      for (const p of a.required_permissions) if (!PERMS.has(p)) err(`${a.key}: permission không có trong ADMIN_PORTAL §2.2: ${p}`);
      actions.set(a.key, { ops: new Set(a.operation_ids) });
    }
  }
}
for (const k of PROTECTED_MENUS) if (!protectedSeen.includes(k)) err(`ADR-0090 mục 2: ${k} phải có can_disable: false`);
for (const k of protectedSeen) if (!PROTECTED_MENUS.includes(k)) err(`${k}: can_disable: false nhưng không thuộc danh sách bảo vệ của ADR-0090 mục 2`);

// ---------- 5. hai chiều với OpenAPI
const oas = yaml.load(fs.readFileSync(OPENAPI, 'utf8'));
const resolve = (ref) => ref.slice(2).split('/').reduce((o, k) => o?.[k.replace(/~1/g, '/').replace(/~0/g, '~')], oas);
const ops = new Map(); // operationId -> { path, op, params }
for (const [p, item] of Object.entries(oas.paths)) {
  for (const m of METHODS) {
    const op = item[m];
    if (!op) continue;
    const params = [...(item.parameters || []), ...(op.parameters || [])].map((x) => (x.$ref ? resolve(x.$ref) : x));
    ops.set(op.operationId, { path: p, method: m, op, params });
  }
}
function paramEnum(entry, name) {
  const prm = entry.params.find((x) => x && x.name === name);
  if (!prm) return null;
  const sch = prm.schema?.$ref ? resolve(prm.schema.$ref) : prm.schema;
  return sch?.enum ?? null;
}
// catalog → OpenAPI
const claimed = new Map(); // "op" hoặc "op[param=value]" -> action key
for (const [akey, a] of actions) {
  for (const ref of a.ops) {
    const m = ref.match(/^([A-Za-z0-9]+)(?:\[([a-z_]+)=([a-z0-9_-]+)\])?$/);
    const [, opId, prm, val] = m;
    if (claimed.has(ref)) err(`${ref} thuộc hai action: ${claimed.get(ref)}, ${akey}`);
    claimed.set(ref, akey);
    const entry = ops.get(opId);
    if (!entry) { err(`${akey}: operationId không tồn tại trong OpenAPI: ${opId}`); continue; }
    const xa = entry.op['x-portal-action'];
    if (!prm) {
      if (xa !== akey) err(`${opId}: x-portal-action phải là ${akey} (đang: ${JSON.stringify(xa) ?? 'thiếu'})`);
    } else {
      if (!xa || typeof xa !== 'object' || xa.param !== prm) { err(`${opId}: cần x-portal-action {param: ${prm}, map} cho ${ref}`); continue; }
      if (xa.map?.[val] !== akey) err(`${opId}: x-portal-action.map.${val} phải là ${akey} (đang: ${xa.map?.[val] ?? 'thiếu'})`);
      const en = paramEnum(entry, prm);
      if (en && !en.includes(val)) err(`${ref}: giá trị ${val} không thuộc enum của tham số ${prm}`);
    }
  }
}
// OpenAPI → catalog
let annotated = 0, shell = 0;
for (const [opId, entry] of ops) {
  const xa = entry.op['x-portal-action'];
  const xs = entry.op['x-portal-shell'];
  if (xs !== undefined && xs !== true) err(`${opId}: x-portal-shell chỉ nhận true`);
  if (xs && xa) err(`${opId}: không khai báo đồng thời x-portal-shell và x-portal-action`);
  if (xs) {
    shell++;
    for (const ref of claimed.keys()) if (ref === opId || ref.startsWith(`${opId}[`)) err(`${opId}: x-portal-shell nhưng có trong catalog (${claimed.get(ref)})`);
  }
  if (!xa) {
    if (!xs && PORTAL_PREFIX.test(entry.path)) err(`${entry.method.toUpperCase()} ${entry.path} (${opId}): route portal thiếu x-portal-action`);
    continue;
  }
  annotated++;
  if (typeof xa === 'string') {
    if (!actions.has(xa)) err(`${opId}: x-portal-action ${xa} không có trong catalog`);
    else if (!actions.get(xa).ops.has(opId)) err(`${opId}: action ${xa} không liệt kê operation này trong operation_ids`);
  } else if (xa && typeof xa === 'object' && typeof xa.param === 'string' && xa.map && typeof xa.map === 'object') {
    const en = paramEnum(entry, xa.param);
    if (!en) err(`${opId}: tham số ${xa.param} của x-portal-action không có enum`);
    for (const v of en ?? []) if (!(v in xa.map)) err(`${opId}: x-portal-action.map thiếu giá trị ${v} của ${xa.param}`);
    for (const [v, akey] of Object.entries(xa.map)) {
      if (en && !en.includes(v)) err(`${opId}: x-portal-action.map.${v} không thuộc enum của ${xa.param}`);
      if (!actions.has(akey)) err(`${opId}: x-portal-action.map.${v} = ${akey} không có trong catalog`);
      else if (!actions.get(akey).ops.has(`${opId}[${xa.param}=${v}]`)) err(`${opId}: action ${akey} không liệt kê ${opId}[${xa.param}=${v}]`);
    }
  } else err(`${opId}: x-portal-action phải là khóa action hoặc {param, map}`);
}

report();
function report() {
  for (const e of errors) console.log(`  ${e}`);
  console.log(`check-portal-catalog: ${catalog.workspaces?.length ?? 0} workspace, ${menuCount} menu, ${actionCount} action `
    + `(không tắt được: ${protMenus} menu, ${protActions} action), ${annotated} operation có x-portal-action, ${shell} x-portal-shell, `
    + `${PERMS.size} permission trong catalog, ${errors.length} lỗi`);
  process.exit(errors.length ? 1 : 0);
}
