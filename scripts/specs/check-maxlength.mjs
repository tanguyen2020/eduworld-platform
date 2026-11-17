#!/usr/bin/env node
// Kiểm tra ADR-0041: mọi schema kiểu string trong specs/ phải có `maxLength`.
// Ngoại lệ: có `enum` hoặc `const`; `format` ∈ {uuid, date, date-time, time, duration, binary, byte}.
// Bỏ qua dữ liệu ví dụ (`example`, `examples`, `default`, `const`, `enum`, khóa `x-*`), thư mục specs/examples,
// specs/_deferred (ADR-0082, không thuộc phạm vi lint) và thư mục feature spec specs/NNN-* (Markdown, ADR-0082).
// Phạm vi: OpenAPI (public, services, internal), AsyncAPI, SIS contract, JSON Schema (schemas/, events/).
//
// Chạy:  npm ci --prefix scripts && node scripts/specs/check-maxlength.mjs
// Exit 1 nếu có vi phạm (dùng trong CI cùng Spectral rule `string-must-have-maxlength`).
import { createRequire } from 'node:module';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const require = createRequire(import.meta.url);
const yaml = require('js-yaml');

const here = path.dirname(fileURLToPath(import.meta.url));
const specsDir = path.resolve(here, '..', '..', 'specs');
const EXEMPT_FORMATS = new Set(['uuid', 'date', 'date-time', 'time', 'duration', 'binary', 'byte']);
const SKIP_KEYS = new Set(['example', 'examples', 'default', 'const', 'enum']);

function listFiles(dir) {
  const out = [];
  for (const e of fs.readdirSync(dir, { withFileTypes: true })) {
    const p = path.join(dir, e.name);
    if (e.isDirectory()) { if (!["examples", "node_modules", "_deferred"].includes(e.name) && !/^d{3}-/.test(e.name)) out.push(...listFiles(p)); }
    else if (/\.(ya?ml|json)$/.test(e.name) && !e.name.startsWith('.')) out.push(p);
  }
  return out;
}

const isString = (t) => t === 'string' || (Array.isArray(t) && t.includes('string'));
const violations = [];
let checked = 0;

function walk(node, ptr, file) {
  if (Array.isArray(node)) { node.forEach((n, i) => walk(n, `${ptr}/${i}`, file)); return; }
  if (!node || typeof node !== 'object') return;
  if (isString(node.type)) {
    checked++;
    const exempt = 'enum' in node || 'const' in node || (node.format && EXEMPT_FORMATS.has(node.format));
    if (!exempt && typeof node.maxLength !== 'number') violations.push(`${path.relative(specsDir, file)}#${ptr}`);
  }
  for (const [k, v] of Object.entries(node)) {
    if (SKIP_KEYS.has(k) || k.startsWith('x-')) continue;
    walk(v, `${ptr}/${k.replace(/~/g, '~0').replace(/\//g, '~1')}`, file);
  }
}

const files = listFiles(specsDir);
for (const f of files) walk(yaml.load(fs.readFileSync(f, 'utf8')), '', f);
console.log(`check-maxlength: ${files.length} file, ${checked} schema string, ${violations.length} vi phạm`);
if (violations.length) { for (const v of violations) console.log('  thiếu maxLength:', v); process.exit(1); }
