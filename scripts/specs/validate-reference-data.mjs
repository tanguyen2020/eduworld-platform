#!/usr/bin/env node
// Validate dữ liệu danh mục trong specs/reference-data theo schema specs/schemas/reference/* (ADR-0060)
// và kiểm tra nghiệp vụ: đúng 34 tỉnh/thành VN đang hiệu lực, mọi đơn vị cũ kế thừa tới đơn vị đang hiệu lực,
// không trùng mã, quốc gia tham chiếu tồn tại. Exit 1 nếu có lỗi.
// Chạy:  npm ci --prefix scripts && node scripts/specs/validate-reference-data.mjs
import { createRequire } from 'node:module';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const require = createRequire(import.meta.url);
const Ajv2020 = require('ajv/dist/2020').default;
const addFormats = require('ajv-formats').default;

const here = path.dirname(fileURLToPath(import.meta.url));
const specs = path.resolve(here, '..', '..', 'specs');
const read = (p) => JSON.parse(fs.readFileSync(path.join(specs, p), 'utf8'));

const ajv = new Ajv2020({ strict: true, allErrors: true });
addFormats(ajv);
const errors = [];
const check = (schemaFile, dataFile) => {
  const validate = ajv.compile(read(schemaFile));
  const data = read(dataFile);
  if (!validate(data)) errors.push(...validate.errors.map((e) => `${dataFile}${e.instancePath} ${e.message}`));
  return data;
};

const countries = check('schemas/reference/country.schema.json', 'reference-data/countries.json');
const provinces = check('schemas/reference/province.schema.json', 'reference-data/provinces.vn.json');
const successions = check('schemas/reference/province-succession.schema.json', 'reference-data/province-successions.vn.json');

const EXPECTED_ACTIVE_VN = 34;
const codes = new Set();
for (const p of provinces.items) {
  if (codes.has(p.code)) errors.push(`trùng mã tỉnh: ${p.code}`);
  codes.add(p.code);
  if (!countries.items.some((c) => c.country_code === p.country_code)) errors.push(`quốc gia không tồn tại: ${p.country_code} (${p.code})`);
  if (p.status === 'active' && p.effective_to) errors.push(`đơn vị active có effective_to: ${p.code}`);
  if (p.status === 'inactive' && !p.effective_to) errors.push(`đơn vị inactive thiếu effective_to: ${p.code}`);
}
const activeVN = provinces.items.filter((p) => p.country_code === 'VN' && p.status === 'active');
if (activeVN.length !== EXPECTED_ACTIVE_VN) errors.push(`số tỉnh/thành VN active = ${activeVN.length}, cần ${EXPECTED_ACTIVE_VN}`);

const byCode = new Map(provinces.items.map((p) => [p.code, p]));
const succOf = new Map();
for (const s of successions.items) {
  if (succOf.has(s.old_code)) errors.push(`đơn vị cũ có nhiều kế thừa: ${s.old_code}`);
  succOf.set(s.old_code, s.new_code);
  if (!byCode.has(s.old_code)) errors.push(`kế thừa từ mã không tồn tại: ${s.old_code}`);
  if (!byCode.has(s.new_code)) errors.push(`kế thừa tới mã không tồn tại: ${s.new_code}`);
}
for (const p of provinces.items.filter((x) => x.status === 'inactive')) {
  let cur = p.code;
  const seen = new Set();
  while (byCode.get(cur)?.status === 'inactive') {
    if (seen.has(cur) || !succOf.has(cur)) { errors.push(`đơn vị cũ không kế thừa tới đơn vị active: ${p.code}`); break; }
    seen.add(cur);
    cur = succOf.get(cur);
  }
}
for (const a of activeVN) {
  if (!successions.items.some((s) => s.new_code === a.code)) errors.push(`đơn vị active không có đơn vị cũ nào kế thừa tới: ${a.code}`);
}

if (errors.length) {
  console.error(`validate-reference-data: ${errors.length} lỗi`);
  for (const e of errors) console.error('  - ' + e);
  process.exit(1);
}
console.log(`validate-reference-data: ${countries.items.length} quốc gia, ${provinces.items.length} tỉnh/thành (${activeVN.length} VN active), ${successions.items.length} kế thừa, 0 lỗi`);
