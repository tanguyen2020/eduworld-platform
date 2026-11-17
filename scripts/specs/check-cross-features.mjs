#!/usr/bin/env node
// Kiểm tra nhất quán event và bảng giữa các feature spec (R33 — một nguồn sở hữu; ADR-0082).
// Chạy: node scripts/specs/check-cross-features.mjs [--verbose] [--list]
//
// Nguồn đọc:
//   - Catalog domain event: docs/07-backend/EVENT_DESIGN.md §4.1–§4.9 (bảng "| Event | Producer | …"),
//     cột Producer → module (identity/learning/game/competition/chat; "mọi module" = event chung);
//     bản đồ consumer §3.2 (chỉ kiểm tên event).
//   - Schema payload: specs/events/payloads/<module>/<event_type>.v<N>.schema.json.
//   - Feature: specs/NNN-*/data-model.md mục "## 4. Event / outbox" và contracts/README.md mục
//     "## 4. Event (NATS)" (chỉ dòng bảng; cột 1 là tên event trong backtick).
//
// Vai trò của một dòng event trong feature:
//   - consume: data-model — cột 1 có "tiêu thụ" mà không có "phát"; contracts — cột 2 bắt đầu "Tiêu thụ",
//     hoặc phía producer (trước "→", "/", ";", " phát") ghi feature khác (Fxxx) hoặc module không thuộc feature.
//   - ref (phát nhưng schema của feature khác): dòng phát có "owner Fxxx" với Fxxx ≠ feature hiện tại.
//   - owner: dòng phát còn lại — feature này tạo schema payload.
//   - Bỏ qua: dòng "learning event" (ghi `learning.learning_events`, không qua outbox), "không phát",
//     subject JetStream/NATS core, mẫu `<entity>.deleted`/`.restored`.
//   "owner Fxxx" trên dòng tiêu thụ là khai báo producer mong đợi và được đối chiếu.
//
// Lỗi (exit 1): event có 0 hoặc > 1 feature sở hữu (theo module phát); feature sở hữu khác module producer
// của EVENT_DESIGN; event dùng trong feature nhưng không có trong EVENT_DESIGN; event được tiêu thụ/phát lại
// mà không feature nào sở hữu; "owner Fxxx" khai báo sai; file schema hoặc đường dẫn schema mang tên event
// không có trong EVENT_DESIGN hoặc sai thư mục module; schema của feature `Done` chưa có file; bảng đánh dấu
// "Mới" trong data-model của hơn một feature; EVENT_DESIGN có event trùng dòng hoặc §3.2 nhắc event không có ở §4.
// Cảnh báo: owner chưa tạo file schema (feature chưa Done); event chung (mọi module) phát không ghi owner.
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..', '..');
const verbose = process.argv.includes('--verbose');
const list = process.argv.includes('--list');
const rel = (p) => path.relative(root, p).replaceAll('\\', '/');
const readLines = (p) => fs.readFileSync(p, 'utf8').split(/\r?\n/);

let errors = 0; let warnings = 0;
const err = (where, msg) => { errors++; console.error(`ERROR ${where}: ${msg}`); };
const warn = (where, msg) => { warnings++; if (verbose) console.warn(`WARN  ${where}: ${msg}`); };

const MODULES = ['identity', 'learning', 'game', 'competition', 'chat'];
const EVENT_RE = /^[a-z][a-z0-9_]*\.[a-z][a-z0-9_]*$/;
const WILDCARD_RE = /^[a-z][a-z0-9_]*\.(?:[a-z0-9_]*\*)$/;
const GENERIC_RE = /\.(deleted|restored)$/; // mẫu <entity>.deleted / <entity>.restored (EVENT_DESIGN §4.1)
// Tên schema hợp lệ không phải event_type: payload chung cho mẫu <entity>.deleted/.restored (F020).
const SCHEMA_ALIASES = new Set(['resource.deleted', 'resource.restored']);

// Domain con / thành phần trong cột Producer của EVENT_DESIGN → module (EVENT_DESIGN §4 đoạn đầu).
const PRODUCER_KEYWORDS = [
  [/mọi module/, 'common'],
  [/^(identity|tenancy|organization|sis|audit)\b|student-inactive-cleanup/, 'identity'],
  [/^(learning|content|curriculum|activity engine|activity|exam|learning_record|learning_analytics|lms)\b/, 'learning'],
  [/^(game|game_session|quest|minigame|gamification|reward|world|event|season_track)\b/, 'game'],
  [/^(competition|party|multiplayer|ranking|leaderboard|tournament|season)\b/, 'competition'],
  [/^(chat|chat-persister|friend|club|moderation|notification)\b|gameplay-chat-purge/, 'chat'],
];

// ---------- Tách ô bảng và tên event ----------
const cellsOf = (line) => {
  const t = line.trim();
  if (!t.startsWith('|')) return null;
  return t.replace(/^\|/, '').replace(/\|\s*$/, '').split(/(?<!\\)\|/).map((c) => c.trim());
};
const isSeparator = (cells) => cells.every((c) => /^:?-{2,}:?$/.test(c) || c === '');

// Tách tên event trong backtick; hỗ trợ viết tắt `a.b_c` / `_d`, `a.b` / `c`, `a.b` / `.c`, `a.b/c`.
// known: tập tên hợp lệ để chọn ứng viên khi viết tắt mơ hồ.
function eventsIn(text, known) {
  const out = [];
  const re = /`([^`]+)`/g; let m; let base = null; let lastEnd = 0;
  const expand = (b, suffix) => {
    const [entity, action] = b.split('.');
    const s = suffix.replace(/^[._]/, '');
    const cands = suffix.startsWith('_')
      ? [`${entity}.${action.replace(/_[a-z0-9]+$/, '')}_${s}`]
      : [`${entity}.${s}`, `${entity}.${action.replace(/_[a-z0-9]+$/, '')}_${s}`];
    return cands.find((c) => known?.has(c)) || cands[0];
  };
  while ((m = re.exec(text))) {
    const between = text.slice(lastEnd, m.index);
    lastEnd = re.lastIndex;
    const parts = m[1].trim().split('/').map((p) => p.trim());
    const first = parts[0];
    if (EVENT_RE.test(first) || WILDCARD_RE.test(first)) {
      out.push(first); base = EVENT_RE.test(first) ? first : null;
      for (const p of parts.slice(1)) if (base && /^[._]?[a-z][a-z0-9_]*$/.test(p)) out.push(expand(base, p));
    } else if (base && /^\s*\/\s*$/.test(between) && /^[._]?[a-z][a-z0-9_]*$/.test(first) && parts.length === 1) {
      out.push(expand(base, first));
    } else base = /^\s*\/\s*$/.test(between) ? base : null;
  }
  return out;
}

// Đường dẫn schema payload nhắc trong một ô: trả [{module|null, name}].
function schemaRefsIn(text) {
  const out = [];
  const re = /(?:payloads\/([a-z]+)\/|…\/(?:([a-z]+)\/)?|\.\.\.\/|`)([a-z][a-z0-9_]*\.[a-z][a-z0-9_]*)\.v\d+(?:\.schema\.json)?\b/g;
  let m;
  while ((m = re.exec(text))) out.push({ module: m[1] || m[2] || null, name: m[3] });
  for (const mm of text.matchAll(/(?:^|[\s(,+])([a-z][a-z0-9_]*\.[a-z][a-z0-9_]*)\.v\d+\.schema\.json/g)) out.push({ module: null, name: mm[1] });
  return out;
}
const modulesOfPathsIn = (text) => [...new Set([...text.matchAll(/payloads\/([a-z]+)\//g)].map((m) => m[1]))];

// ---------- 1. Catalog EVENT_DESIGN ----------
const edPath = path.join(root, 'docs/07-backend/EVENT_DESIGN.md');
const edLines = readLines(edPath);
const catalog = new Map(); // name -> {modules:Set, common, line, hint:Set}
const catalogNames = new Set();
{
  // Lượt 1: gom tên để giải viết tắt.
  let inCat = false; let header = null;
  const rows = [];
  edLines.forEach((line, i) => {
    if (/^## 4\. Catalog domain events/.test(line)) inCat = true;
    else if (/^### 4\.10\b|^## 5\./.test(line)) inCat = false;
    if (!inCat) return;
    const c = cellsOf(line);
    if (!c) { header = null; return; }
    if (!header) { header = c; return; }
    if (isSeparator(c)) return;
    if (header[0] !== 'Event' || header[1] !== 'Producer') return;
    rows.push({ c, i });
  });
  for (const { c } of rows) for (const e of eventsIn(c[0])) if (EVENT_RE.test(e)) catalogNames.add(e);
  let prevModules = null;
  for (const { c, i } of rows) {
    const where = `EVENT_DESIGN.md:${i + 1}`;
    const names = eventsIn(c[0], catalogNames).filter((e) => EVENT_RE.test(e));
    if (!names.length) continue; // dòng mẫu `<entity>.deleted` không có tên cụ thể
    const prod = c[1].toLowerCase();
    let modules = new Set();
    if (/^như trên/.test(prod)) modules = new Set(prevModules || []);
    else if (/^media worker/.test(prod)) modules = new Set(['learning', 'game', 'chat']); // worker media của module sở hữu (ADR-0044)
    else {
      // "learning / game (module sở hữu item)" → nhiều module; mỗi phần ánh xạ theo từ khóa.
      for (const part of prod.split('(')[0].split(' / ')) {
        for (const [re, mod] of PRODUCER_KEYWORDS) if (re.test(part.trim().replace(/^worker\s+/, ''))) { modules.add(mod); break; }
      }
    }
    if (!modules.size) err(where, `không xác định được module producer từ "${c[1]}"`);
    prevModules = modules;
    const hint = new Set([...c[1].matchAll(/\bF\d{3}\b/g)].map((m) => m[0]));
    for (const n of names) {
      if (catalog.has(n)) { err(where, `event ${n} xuất hiện ở hai dòng catalog (dòng ${catalog.get(n).line})`); continue; }
      catalog.set(n, { modules, common: modules.has('common'), line: i + 1, hint });
    }
  }
  // §3.2 bản đồ consumer: tên event phải có trong §4.
  let in32 = false; let hdr = null;
  edLines.forEach((line, i) => {
    if (/^### 3\.2\b/.test(line)) in32 = true; else if (/^##+ /.test(line)) in32 = false;
    if (!in32) return;
    const c = cellsOf(line);
    if (!c) { hdr = null; return; }
    if (!hdr) { hdr = c; return; }
    if (isSeparator(c)) return;
    for (const e of eventsIn(c[0], catalogNames)) {
      if (WILDCARD_RE.test(e)) { const pre = e.replace(/\*$/, ''); if (![...catalog.keys()].some((k) => k.startsWith(pre))) err(`EVENT_DESIGN.md:${i + 1}`, `§3.2 nhắc ${e} nhưng §4 không có event khớp`); }
      else if (!GENERIC_RE.test(e) && !catalog.has(e)) err(`EVENT_DESIGN.md:${i + 1}`, `§3.2 nhắc ${e} nhưng §4 không có`);
    }
  });
}

// ---------- 2. File schema payload ----------
const payloadDir = path.join(root, 'specs/events/payloads');
const schemaFiles = new Map(); // module:name -> file
for (const mod of fs.existsSync(payloadDir) ? fs.readdirSync(payloadDir) : []) {
  const dir = path.join(payloadDir, mod);
  if (!fs.statSync(dir).isDirectory()) continue;
  for (const f of fs.readdirSync(dir)) {
    const m = f.match(/^([a-z][a-z0-9_]*\.[a-z][a-z0-9_]*)\.v\d+\.schema\.json$/);
    const where = rel(path.join(dir, f));
    if (!m) { err(where, 'tên file phải là <event_type>.v<N>.schema.json'); continue; }
    const ev = catalog.get(m[1]);
    if (!ev) err(where, `event ${m[1]} không có trong EVENT_DESIGN §4`);
    else if (mod === 'common' ? !ev.common : !ev.modules.has(mod)) err(where, `thư mục ${mod}/ khác module producer (${[...ev.modules].join(', ')}) của EVENT_DESIGN`);
    schemaFiles.set(`${mod}:${m[1]}`, where);
  }
}

// ---------- 3. Feature ----------
const specsDir = path.join(root, 'specs');
const features = fs.readdirSync(specsDir, { withFileTypes: true })
  .filter((d) => d.isDirectory() && /^\d{3}-[a-z0-9-]+$/.test(d.name)).map((d) => d.name).sort();
const feat = new Map(); // fid -> {dir, modules:Set, status}
for (const dir of features) {
  const fid = `F${dir.slice(0, 3)}`;
  const head = readLines(path.join(specsDir, dir, 'spec.md')).slice(0, 4).join('\n');
  const modField = (head.match(/\*\*Module\*\*:\s*([^\n]*?)(?:\s+·\s+\*\*|$)/m) || [])[1] || '';
  const modList = [...modField.matchAll(/module `([a-z]+)`/g)].map((m) => m[1]).filter((m) => MODULES.includes(m));
  const modules = new Set(modList);
  const primary = modList[0] || null; // module đầu tiên của trường Module — dùng khi event phát ở nhiều module
  const status = (head.match(/\*\*Status\*\*:\s*([A-Za-z ]+?)\s*(?:·|$)/m) || [])[1] || '';
  feat.set(fid, { dir, modules, primary, status });
}

const entries = []; // {fid, where, name, role, module, declared:Set}
const schemaRefs = []; // {where, module, name}
const newTables = new Map(); // table -> [{fid, where}]

const ownerDecl = (text) => new Set([...text.matchAll(/\bowner\s+(F\d{3}(?:\s*\/\s*F\d{3})*)/gi)].flatMap((m) => m[1].split('/').map((s) => s.trim())));
const fidsIn = (text) => new Set([...text.matchAll(/\bF\d{3}\b/g)].map((m) => m[0]));

function moduleFor(name, rowText, role, self, producerModules) {
  const ev = catalog.get(name);
  const refs = schemaRefsIn(rowText).filter((r) => r.name === name && r.module);
  if (refs.length) return refs[0].module;
  const paths = modulesOfPathsIn(rowText);
  if (paths.length === 1 && !(ev?.common)) return paths[0];
  if (ev && !ev.common && ev.modules.size === 1) return [...ev.modules][0];
  if (ev?.common) return 'common';
  const cand = role === 'consume' ? producerModules : [...self.modules];
  const inter = cand.filter((m) => !ev || ev.modules.has(m));
  if (inter.length === 1) return inter[0];
  if (role !== 'consume' && self.primary && inter.includes(self.primary)) return self.primary;
  return null;
}

for (const dir of features) {
  const fid = `F${dir.slice(0, 3)}`;
  const self = feat.get(fid);
  for (const file of ['data-model.md', 'contracts/README.md']) {
    const p = path.join(specsDir, dir, file);
    if (!fs.existsSync(p)) continue;
    const lines = readLines(p);
    let inSec = false; let header = null;
    lines.forEach((line, i) => {
      const where = `${dir}/${file}:${i + 1}`;
      // Bảng mới (chỉ data-model)
      if (file === 'data-model.md') {
        if (/^#{2,4} /.test(line)) {
          for (const seg of line.split(' · ')) {
            const k = seg.indexOf('(');
            if (k < 0) continue;
            const paren = seg.slice(k + 1);
            if (!/^\**Mới\b/.test(paren) || /^\**Mới\**\s+ở\s+F\d{3}/.test(paren)) continue;
            for (const t of [...seg.slice(0, k).matchAll(/`([a-z_]+\.[a-z_]+)`/g)].map((m) => m[1])) {
              if (!newTables.has(t)) newTables.set(t, []);
              newTables.get(t).push({ fid, where });
            }
          }
        } else {
          const c = cellsOf(line);
          if (c && c.length >= 2 && /^\**Mới\b/.test(c[1])) {
            for (const t of [...c[0].matchAll(/`([a-z_]+\.[a-z_]+)`/g)].map((m) => m[1])) {
              if (!newTables.has(t)) newTables.set(t, []);
              newTables.get(t).push({ fid, where });
            }
          }
        }
      }
      if (/^## /.test(line)) { inSec = /^##\s+\d+\.\s+Event\b/.test(line); header = null; return; }
      if (!inSec) return;
      const c = cellsOf(line);
      if (!c) { header = null; return; }
      if (!header) { header = c; return; }
      if (isSeparator(c) || c.length < 2) return;
      const rowText = c.join(' | ');
      for (const r of schemaRefsIn(rowText)) schemaRefs.push({ where, ...r });
      const first = c[0];
      if (/^—/.test(first) || /learning event/i.test(first)) return; // "—": không phát/tiêu thụ
      if (/không phát/i.test(first) || /^\**không phát\**/i.test(c[1])) return;
      let names = /`audit\.recorded`/.test(first) ? ['audit.recorded'] : eventsIn(first, catalogNames);
      names = names.filter((n) => !(GENERIC_RE.test(n) && !catalog.has(n)));
      if (!names.length) return;
      let role; let producerFids = new Set(); let producerModules = [];
      if (file === 'data-model.md') {
        role = /\bphát\b/i.test(first) ? 'publish' : /tiêu thụ/i.test(first) ? 'consume' : 'publish';
      } else {
        const d = c[1].replace(/\*\*/g, '');
        if (/^tiêu thụ/i.test(d)) role = 'consume';
        else if (/^phát/i.test(d)) role = 'publish';
        else {
          const side = d.split(/→|↔|;| \/ | phát\b/)[0];
          producerFids = new Set([...fidsIn(side)].filter((f) => f !== fid));
          producerModules = MODULES.filter((m) => new RegExp(`\\b${m}\\b`, 'i').test(side));
          if (/tiêu thụ/i.test(side)) role = 'consume';
          else if (producerFids.size) role = 'consume';
          else if (producerModules.length && !producerModules.some((m) => self.modules.has(m))) role = 'consume';
          else role = 'publish';
        }
      }
      const declared = ownerDecl(rowText);
      for (const name of names) {
        if (WILDCARD_RE.test(name)) {
          const pre = name.replace(/\*$/, '');
          if (![...catalog.keys()].some((k) => k.startsWith(pre))) err(where, `${name} không khớp event nào trong EVENT_DESIGN §4`);
          continue;
        }
        let r = role;
        if (r === 'publish') {
          const ev = catalog.get(name);
          if (declared.size && !declared.has(fid)) r = 'ref';
          else if (ev?.common && !declared.has(fid)) r = 'ref';
          else r = 'owner';
        }
        const exp = r === 'consume' ? new Set([...declared, ...producerFids]) : declared;
        entries.push({ fid, where, name, role: r, module: moduleFor(name, rowText, r, self, producerModules), declared: exp });
      }
    });
  }
}

// ---------- 4. Đối chiếu ----------
const owners = new Map(); // module:name -> [{fid, where}]
const byName = new Map(); // name -> Set(module) có owner
for (const e of entries) {
  const ev = catalog.get(e.name);
  if (!ev) { err(e.where, `event ${e.name} không có trong EVENT_DESIGN §4 (bổ sung catalog hoặc đổi tên)`); continue; }
  if (e.role !== 'owner') continue;
  if (!e.module) { err(e.where, `không xác định được module phát của ${e.name} (ghi đường dẫn specs/events/payloads/<module>/)`); continue; }
  const mods = ev.common ? new Set(['common']) : ev.modules;
  if (!mods.has(e.module)) err(e.where, `${e.fid} sở hữu ${e.name} ở module ${e.module} nhưng EVENT_DESIGN ghi producer ${[...ev.modules].join('/')}`);
  else if (e.module !== 'common' && !feat.get(e.fid).modules.has(e.module)) err(e.where, `${e.fid} không thuộc module ${e.module} nên không thể sở hữu ${e.name}`);
  const key = `${e.module}:${e.name}`;
  if (!owners.has(key)) owners.set(key, []);
  if (!owners.get(key).some((o) => o.fid === e.fid)) owners.get(key).push({ fid: e.fid, where: e.where });
  if (!byName.has(e.name)) byName.set(e.name, new Set());
  byName.get(e.name).add(e.module);
}
for (const [key, os] of owners) if (os.length > 1) err(os[1].where, `${key} có ${os.length} feature sở hữu: ${os.map((o) => o.fid).join(', ')} — chỉ một feature tạo schema, feature khác ghi "owner Fxxx"`);

const ownerOf = (name, module) => {
  const keys = module ? [`${module}:${name}`] : [...(byName.get(name) || [])].map((m) => `${m}:${name}`);
  return keys.flatMap((k) => owners.get(k) || []).map((o) => o.fid);
};
for (const e of entries) {
  if (e.role === 'owner' || !catalog.has(e.name)) continue;
  const os = ownerOf(e.name, e.module === 'common' ? 'common' : e.module);
  const verb = e.role === 'consume' ? 'tiêu thụ' : 'phát lại';
  if (!os.length) { err(e.where, `${e.fid} ${verb} ${e.name}${e.module ? ` (${e.module})` : ''} nhưng không feature nào sở hữu`); continue; }
  if (e.declared.size && !os.some((o) => e.declared.has(o))) err(e.where, `khai báo producer/owner ${[...e.declared].join('/')} cho ${e.name} nhưng feature sở hữu là ${os.join(', ')}`);
}

for (const [name, ev] of catalog) {
  const mods = ev.common ? ['common'] : [...ev.modules];
  const owned = mods.filter((m) => owners.has(`${m}:${name}`));
  const multi = mods.length > 1;
  if (!owned.length) err(`EVENT_DESIGN.md:${ev.line}`, `${name} chưa có feature sở hữu schema (thêm dòng phát vào data-model §4 của feature thực hiện use case)`);
  for (const m of owned) {
    const o = owners.get(`${m}:${name}`)[0];
    if (ev.hint.size && !ev.hint.has(o.fid)) err(`EVENT_DESIGN.md:${ev.line}`, `cột Producer ghi ${[...ev.hint].join('/')} nhưng feature sở hữu ${name} là ${o.fid}`);
    if (!schemaFiles.has(`${m}:${name}`)) {
      if (feat.get(o.fid).status === 'Done') err(o.where, `${o.fid} Done nhưng chưa có specs/events/payloads/${m}/${name}.v1.schema.json`);
      else warn(o.where, `chưa có specs/events/payloads/${m}/${name}.v<N>.schema.json (owner ${o.fid}, ${feat.get(o.fid).status})`);
    }
  }
  if (multi && owned.length && verbose) for (const m of mods.filter((x) => !owned.includes(x))) warn(`EVENT_DESIGN.md:${ev.line}`, `${name} ở module ${m} chưa có feature phát`);
}

for (const r of schemaRefs) {
  if (SCHEMA_ALIASES.has(r.name) || GENERIC_RE.test(r.name) && !catalog.has(r.name)) continue;
  const ev = catalog.get(r.name);
  if (!ev) { err(r.where, `đường dẫn schema ${r.name}.v<N> không mang tên event nào trong EVENT_DESIGN §4`); continue; }
  if (r.module && (r.module === 'common' ? !ev.common : !ev.modules.has(r.module))) err(r.where, `schema ${r.module}/${r.name} sai thư mục module (producer ${[...ev.modules].join('/')})`);
}

for (const [t, uses] of newTables) {
  const fids = [...new Set(uses.map((u) => u.fid))];
  if (fids.length > 1) err(uses[1].where, `bảng ${t} đánh dấu "Mới" ở ${fids.join(', ')} — chỉ feature tạo bảng ghi "Mới", feature khác ghi "Có sẵn (Fxxx)"`);
}

if (list) {
  for (const [key, os] of [...owners].sort()) console.log(`${key.padEnd(56)} ${os.map((o) => o.fid).join(', ')}`);
}
const consumed = entries.filter((e) => e.role === 'consume').length;
console.log(`check-cross-features: ${features.length} feature, ${catalog.size} event trong EVENT_DESIGN, ${owners.size} cặp module:event có owner, ${consumed} dòng tiêu thụ, ${schemaFiles.size} file schema, ${newTables.size} bảng "Mới", ${errors} lỗi, ${warnings} cảnh báo${warnings && !verbose ? ' (--verbose để xem)' : ''}`);
process.exit(errors ? 1 : 0);
