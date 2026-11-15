#!/usr/bin/env node
// Kiểm tra catalog mã lỗi là MỘT danh sách duy nhất (API_DESIGN §4.1):
//   1. Mã trong các bảng của docs/07-backend/API_DESIGN.md §4.1 == enum `ProblemCode` của
//      specs/openapi/eduworld-api.v1.yaml (không thừa, không thiếu ở cả hai phía).
//   2. Enum `ErrorCode` (WebSocket) của specs/asyncapi/eduworld-realtime.v1.yaml: mỗi mã phải có trong `ProblemCode`
//      (trùng tên) hoặc nằm trong danh sách "Các mã chỉ có trên WS" của API_DESIGN §4.1; mã chỉ-WS không được có trong
//      `ProblemCode` và phải có trong `ErrorCode`.
//   3. Không mã nào lặp lại trong bảng hoặc trong enum.
// Sub-code trong `errors[].code` (vd. PERMISSION_NOT_ALLOWED_FOR_SCOPE) không thuộc catalog `code` nên không so sánh.
//
// Chạy:  npm ci --prefix scripts && node scripts/specs/check-problem-codes.mjs   (exit 1 nếu lệch)
import { createRequire } from 'node:module';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const require = createRequire(import.meta.url);
const yaml = require('js-yaml');

const here = path.dirname(fileURLToPath(import.meta.url));
const repo = path.resolve(here, '..', '..');
const API_DESIGN = path.join(repo, 'docs/07-backend/API_DESIGN.md');
const OPENAPI = path.join(repo, 'specs/openapi/eduworld-api.v1.yaml');
const ASYNCAPI = path.join(repo, 'specs/asyncapi/eduworld-realtime.v1.yaml');

const errors = [];
const md = fs.readFileSync(API_DESIGN, 'utf8').split('\n');
const start = md.findIndex((l) => /^### 4\.1\./.test(l));
const end = md.findIndex((l, i) => i > start && /^## /.test(l));
if (start < 0 || end < 0) { console.error('Không tìm thấy mục §4.1 trong API_DESIGN.md'); process.exit(1); }

const tableCodes = [];
let wsOnly = [];
for (let i = start; i < end; i++) {
  const line = md[i];
  const cells = line.split('|').map((c) => c.trim());
  // Hàng bảng: | HTTP | `CODE` [/ `CODE2`] | Khi nào |
  if (line.startsWith('|') && /^\d{3}$/.test(cells[1] || '')) {
    for (const m of (cells[2] || '').matchAll(/`([A-Z][A-Z0-9_]+)`/g)) tableCodes.push({ code: m[1], line: i + 1 });
  }
  if (/Các mã chỉ có trên WS/.test(line)) {
    wsOnly = [...line.slice(line.indexOf('Các mã chỉ có trên WS')).matchAll(/`([A-Z][A-Z0-9_]+)`/g)].map((m) => m[1]);
  }
}

const oas = yaml.load(fs.readFileSync(OPENAPI, 'utf8'));
const problemCodes = oas.components?.schemas?.ProblemCode?.enum;
if (!Array.isArray(problemCodes)) { console.error('Không tìm thấy components.schemas.ProblemCode.enum'); process.exit(1); }
const asy = yaml.load(fs.readFileSync(ASYNCAPI, 'utf8'));
const wsCodes = asy.components?.schemas?.ErrorCode?.enum;
if (!Array.isArray(wsCodes)) { console.error('Không tìm thấy components.schemas.ErrorCode.enum trong AsyncAPI'); process.exit(1); }

const dupes = (arr) => arr.filter((x, i) => arr.indexOf(x) !== i);
for (const d of new Set(dupes(tableCodes.map((t) => t.code)))) errors.push(`API_DESIGN §4.1: mã ${d} lặp lại (dòng ${tableCodes.filter((t) => t.code === d).map((t) => t.line).join(', ')})`);
for (const d of new Set(dupes(problemCodes))) errors.push(`OpenAPI ProblemCode: mã ${d} lặp lại`);
for (const d of new Set(dupes(wsCodes))) errors.push(`AsyncAPI ErrorCode: mã ${d} lặp lại`);

const tableSet = new Set(tableCodes.map((t) => t.code));
const pcSet = new Set(problemCodes);
for (const c of tableSet) if (!pcSet.has(c)) errors.push(`Có trong API_DESIGN §4.1, thiếu trong ProblemCode: ${c}`);
for (const c of pcSet) if (!tableSet.has(c)) errors.push(`Có trong ProblemCode, thiếu trong API_DESIGN §4.1: ${c}`);

const wsOnlySet = new Set(wsOnly);
const wsSet = new Set(wsCodes);
if (!wsOnly.length) errors.push('API_DESIGN §4.1: không tìm thấy câu "Các mã chỉ có trên WS: …"');
for (const c of wsCodes) if (!pcSet.has(c) && !wsOnlySet.has(c)) errors.push(`AsyncAPI ErrorCode ${c}: không có trong ProblemCode và không được khai báo là mã chỉ-WS`);
for (const c of wsOnly) {
  if (pcSet.has(c)) errors.push(`Mã chỉ-WS ${c} lại có trong ProblemCode`);
  if (!wsSet.has(c)) errors.push(`Mã chỉ-WS ${c} không có trong AsyncAPI ErrorCode`);
}

console.log(`check-problem-codes: API_DESIGN §4.1 ${tableSet.size} mã, ProblemCode ${pcSet.size}, AsyncAPI ErrorCode ${wsSet.size} (chỉ-WS ${wsOnlySet.size}), ${errors.length} lỗi`);
if (errors.length) { for (const e of errors) console.log('  ' + e); process.exit(1); }
