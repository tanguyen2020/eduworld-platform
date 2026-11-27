#!/usr/bin/env node
// Size gate of the client artifacts (F133 T012, F133-AC-04, NFR-PERF-006). Limits (CONFIG_DEFAULTS `build_size`)
// are read from their owner, PERFORMANCE_BUDGET §11 (R33) — never typed here:
//   - Android installer (each APK/AAB), iOS installer (IPA or zipped .xcarchive): file size;
//   - Web first download: Brotli size of every file of the Web export folder (what the CDN serves pre-compressed);
//   - each zone bundle (.pck): Brotli size ("≤ 40 MB nén").
// 1 MB = 1 000 000 bytes (the stricter reading). Brotli quality 9 is a little larger than the quality the CDN upload
// uses, so the gate errs on the safe side.
//
//   node scripts/godot/check-build-size.mjs [--android=<apk|aab>]… [--ios=<ipa>] [--web=<dir>] [--bundles=<dir>] \
//        [--report=<file.md>] [--json=<file.json>] [--budget=<markdown file, default docs/…/PERFORMANCE_BUDGET.md>]
// Exit 0 within budget, 1 over budget, 2 usage error (nothing to measure, missing file). The Markdown report is
// attached to each release (F133-AC-04).
import fs from 'node:fs';
import path from 'node:path';
import zlib from 'node:zlib';
import { fileURLToPath } from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));
const DEFAULT_BUDGET = path.join(here, '..', '..', 'docs', '08-game-client', 'PERFORMANCE_BUDGET.md');
const ROWS = {
  android: /^Bản cài Android/,
  ios: /^Bản cài iOS/,
  web: /^Web tải đầu/,
  bundle: /^Gói nội dung mỗi vùng/,
  cache: /^Bộ nhớ cache nội dung/,
};
const UNIT = { KB: 1e3, MB: 1e6, GB: 1e9 };

/** Byte limits of §11 of PERFORMANCE_BUDGET (Markdown text). */
export function parseBudget(markdown) {
  const start = markdown.search(/^## 11\./m);
  if (start < 0) throw new Error('check-build-size: §11 not found in the budget document');
  const rest = markdown.slice(start + 1);
  const end = rest.search(/^## /m);
  const section = end >= 0 ? rest.slice(0, end) : rest;
  const out = {};
  for (const line of section.split(/\r?\n/)) {
    const cells = line.split('|').map((c) => c.trim());
    if (cells.length < 4) continue;
    for (const [key, re] of Object.entries(ROWS)) {
      if (!re.test(cells[1])) continue;
      const m = cells[2].match(/≤\s*([0-9]+(?:[.,][0-9]+)?)\s*(KB|MB|GB)/);
      if (!m) throw new Error(`check-build-size: no "≤ N MB" limit in the row "${cells[1]}"`);
      out[key] = Math.round(Number(m[1].replace(',', '.')) * UNIT[m[2]]);
    }
  }
  for (const [key, label] of [['android', 'Android'], ['ios', 'iOS'], ['web', 'Web'], ['bundle', 'bundle']]) {
    if (!(key in out)) throw new Error(`check-build-size: §11 has no ${label} limit row`);
  }
  return out;
}

const brotli = (buf) => zlib.brotliCompressSync(buf, { params: { [zlib.constants.BROTLI_PARAM_QUALITY]: 9,
  [zlib.constants.BROTLI_PARAM_SIZE_HINT]: buf.length } }).length;

function filesOf(dir) {
  return fs.readdirSync(dir, { withFileTypes: true }).flatMap((e) =>
    e.isDirectory() ? filesOf(path.join(dir, e.name)) : [path.join(dir, e.name)]);
}

/** Rows {name, kind, bytes, limit, ok, how} for the given artifacts. */
export function measure({ android = [], ios = [], web = '', bundles = '' }, budget) {
  const rows = [];
  const row = (name, kind, bytes, how) => rows.push({ name, kind, bytes, limit: budget[kind], ok: bytes <= budget[kind], how });
  for (const f of [].concat(android)) row(`android ${path.basename(f)}`, 'android', fs.statSync(f).size, 'file size');
  for (const f of [].concat(ios)) row(`ios ${path.basename(f)}`, 'ios', fs.statSync(f).size, 'file size');
  if (web) {
    const total = filesOf(web).reduce((sum, f) => sum + brotli(fs.readFileSync(f)), 0);
    row('web first download', 'web', total, 'Brotli, all files');
  }
  if (bundles) {
    for (const f of filesOf(bundles).filter((f) => f.endsWith('.pck')).sort()) {
      row(`bundle ${path.basename(f)}`, 'bundle', brotli(fs.readFileSync(f)), 'Brotli');
    }
  }
  return rows;
}

const mb = (n) => `${(n / 1e6).toFixed(2)} MB`;

export function renderReport(rows, budget) {
  const lines = [
    '# Báo cáo dung lượng client (F133-AC-04)',
    '',
    'Ngưỡng `build_size` đọc từ `docs/08-game-client/PERFORMANCE_BUDGET.md` §11 (1 MB = 1 000 000 byte).',
    '',
    '| Artifact | Cách đo | Dung lượng | Ngưỡng | Kết quả |',
    '|---|---|---|---|---|',
    ...rows.map((r) => `| ${r.name} | ${r.how} | ${mb(r.bytes)} | ${mb(r.limit)} | ${r.ok ? '✅' : '❌ vượt'} |`),
    '',
    `Cache thiết bị tối đa: ${budget.cache ? mb(budget.cache) : '—'} (client tự xóa LRU, không đo ở CI).`,
  ];
  return lines.join('\n') + '\n';
}

function main(argv) {
  const opts = { android: [], ios: [], web: '', bundles: '', report: '', json: '', budget: DEFAULT_BUDGET };
  for (const a of argv) {
    const m = a.match(/^--([a-z]+)=(.*)$/);
    if (!m || !(m[1] in opts)) { console.error(`check-build-size: unknown argument ${a}`); return 2; }
    if (Array.isArray(opts[m[1]])) opts[m[1]].push(m[2]); else opts[m[1]] = m[2];
  }
  const inputs = [...opts.android, ...opts.ios, opts.web, opts.bundles].filter(Boolean);
  if (inputs.length === 0) { console.error('usage: check-build-size.mjs --android=… --ios=… --web=<dir> --bundles=<dir>'); return 2; }
  for (const f of inputs) if (!fs.existsSync(f)) { console.error(`check-build-size: ${f} not found`); return 2; }
  const budget = parseBudget(fs.readFileSync(opts.budget, 'utf8'));
  const rows = measure(opts, budget);
  const text = renderReport(rows, budget);
  process.stdout.write(text);
  if (opts.report) fs.writeFileSync(opts.report, text);
  if (opts.json) fs.writeFileSync(opts.json, JSON.stringify({ budget, rows }, null, 2) + '\n');
  const over = rows.filter((r) => !r.ok);
  for (const r of over) console.error(`check-build-size: ${r.name} ${mb(r.bytes)} > ${mb(r.limit)} (PERFORMANCE_BUDGET §11)`);
  return over.length ? 1 : 0;
}

if (process.argv[1] && fileURLToPath(import.meta.url) === path.resolve(process.argv[1])) {
  try { process.exit(main(process.argv.slice(2))); } catch (e) { console.error(e.message); process.exit(2); }
}
