#!/usr/bin/env node
// Kiểm tra enum học tập dùng chung là MỘT danh sách (EDU_FIT_REVIEW SPC-02, SPC-03; ADR-0072):
//   Nguồn: specs/schemas/common/enums.schema.json ($defs). JSON Schema khác phải $ref tới đây (không kiểm ở script này).
//   OpenAPI/AsyncAPI khai báo inline: components.schemas.<Tên> (nếu có) phải có enum y hệt (cùng giá trị, cùng thứ tự).
// Tên không có trong một file chỉ đếm (in ra với --verbose), không lỗi.
//
// Chạy:  npm ci --prefix scripts && node scripts/specs/check-enums.mjs   (exit 1 nếu lệch)
import { createRequire } from 'node:module';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const require = createRequire(import.meta.url);
const yaml = require('js-yaml');

const here = path.dirname(fileURLToPath(import.meta.url));
const repo = path.resolve(here, '..', '..');
const NAMES = ['QuestionType', 'ActivityType', 'FeedbackTiming', 'ResultRelease', 'AttemptMode', 'AttemptStatus', 'LearningContentType'];
const SOURCE = 'specs/schemas/common/enums.schema.json';
const TARGETS = [
  'specs/openapi/eduworld-api.v1.yaml',
  'specs/openapi/modules/learning.internal.v1.yaml',
  'specs/openapi/modules/game.internal.v1.yaml',
  'specs/openapi/modules/competition.internal.v1.yaml',
  'specs/asyncapi/eduworld-realtime.v1.yaml',
];

// Giá trị deferred (ADR-0082): không được xuất hiện trong enum nào của specs đang dùng (specs/_deferred không quét).
const DEFERRED_VALUES = ['written_response', 'speaking', 'project', 'rubric', 'teacher_review', 'ai_suggestion'];

const errors = [];
const infos = [];
const src = JSON.parse(fs.readFileSync(path.join(repo, SOURCE), 'utf8'));
const canonical = {};
for (const n of NAMES) {
  const e = src.$defs?.[n]?.enum;
  if (!Array.isArray(e)) errors.push(`${SOURCE}: thiếu $defs.${n}.enum`);
  else canonical[n] = e;
}

let compared = 0;
for (const rel of TARGETS) {
  const file = path.join(repo, rel);
  if (!fs.existsSync(file)) { infos.push(`${rel}: không có file`); continue; }
  const doc = yaml.load(fs.readFileSync(file, 'utf8'));
  const schemas = doc?.components?.schemas || {};
  for (const n of NAMES) {
    if (!canonical[n]) continue;
    const s = schemas[n];
    if (!s) { infos.push(`${rel}: không khai báo ${n}`); continue; }
    const e = s.enum;
    compared++;
    if (!Array.isArray(e)) { errors.push(`${rel}: components.schemas.${n} không có enum`); continue; }
    if (JSON.stringify(e) !== JSON.stringify(canonical[n])) {
      const missing = canonical[n].filter((x) => !e.includes(x));
      const extra = e.filter((x) => !canonical[n].includes(x));
      const detail = missing.length || extra.length ? `thiếu [${missing.join(', ')}], thừa [${extra.join(', ')}]` : 'khác thứ tự';
      errors.push(`${rel}: ${n} lệch ${SOURCE} — ${detail}`);
    }
  }
}

// Quét mọi enum trong spec gộp, internal, AsyncAPI, schemas, events: không chứa giá trị deferred.
const walkEnums = (node, at, out) => {
  if (Array.isArray(node)) node.forEach((x, i) => walkEnums(x, `${at}[${i}]`, out));
  else if (node && typeof node === 'object') {
    for (const [k, v] of Object.entries(node)) {
      if (k === 'enum' && Array.isArray(v)) for (const x of v) if (DEFERRED_VALUES.includes(x)) out.push(`${at}.enum chứa '${x}'`);
      walkEnums(v, `${at}.${k}`, out);
    }
  }
};
const listFiles = (d, ext) => fs.existsSync(path.join(repo, d)) ? fs.readdirSync(path.join(repo, d), { withFileTypes: true }).flatMap((e) =>
  e.isDirectory() ? listFiles(`${d}/${e.name}`, ext) : ext.some((x) => e.name.endsWith(x)) ? [`${d}/${e.name}`] : []) : [];
const enumScan = ['specs/openapi', 'specs/asyncapi', 'specs/schemas', 'specs/events', 'specs/sis'].flatMap((d) => listFiles(d, ['.yaml', '.json']));
for (const rel of enumScan) {
  const text = fs.readFileSync(path.join(repo, rel), 'utf8');
  const doc = rel.endsWith('.json') ? JSON.parse(text) : yaml.load(text);
  const found = [];
  walkEnums(doc, '$', found);
  for (const f of found) errors.push(`${rel}: ${f} (deferred, ADR-0082)`);
}

if (process.argv.includes('--verbose')) for (const i of infos) console.log('  info:', i);
for (const e of errors) console.log('  LỖI:', e);
console.log(`check-enums: ${NAMES.length} enum nguồn, ${compared} khai báo đã so sánh, ${enumScan.length} file quét giá trị deferred, ${infos.length} khai báo vắng mặt (--verbose để xem), ${errors.length} lỗi`);
if (errors.length) process.exit(1);
