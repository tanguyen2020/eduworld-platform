#!/usr/bin/env node
// Compile mọi JSON Schema trong specs/schemas và specs/events ở Ajv full strict (allowMatchingProperties)
// và validate các ví dụ trong specs/examples theo bảng ánh xạ bên dưới. Exit 1 nếu có lỗi.
// Không quét specs/_deferred (ADR-0082): nội dung deferred không thuộc phạm vi lint/codegen.
// Chạy:  npm ci --prefix scripts && node scripts/specs/validate-schemas.mjs
import { createRequire } from 'node:module';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const require = createRequire(import.meta.url);
const Ajv2020 = require('ajv/dist/2020').default;
const addFormats = require('ajv-formats').default;

const here = path.dirname(fileURLToPath(import.meta.url));
const specs = path.resolve(here, '..', '..', 'specs');
const EXAMPLES = {
  'quest.math-g3-multiplication.json': 'schemas/quest.schema.json',
  'minigame.math-g3-matching.json': 'schemas/minigame-config.schema.json',
  'chat-policy.platform-default.json': 'schemas/chat-policy.schema.json',
  'sensitive-term-catalog.placeholder.json': 'schemas/sensitive-term-catalog.schema.json',
  'progression-curve.default.json': 'schemas/progression-curve.schema.json',
  'challenge.math-g3-time-attack.json': 'schemas/challenge.schema.json',
  'boss.math-fraction-dragon.json': 'schemas/boss.schema.json',
  'achievement.explorer-math-zone.json': 'schemas/achievement.schema.json',
  'shop-item.explorer-hat.json': 'schemas/shop-item.schema.json',
  'event.mid-autumn-festival.json': 'schemas/event.schema.json',
  'season-track.season-1.json': 'schemas/season-track.schema.json',
  'activity.video-lesson.science-g4-water-cycle.json': 'schemas/activities/video-lesson.schema.json',
  'activity.audio-lesson.english-g3-my-name.json': 'schemas/activities/audio-lesson.schema.json',
  'activity.simulation.physics-lever.json': 'schemas/activities/simulation.schema.json',
  'curriculum.gdpt2018-sample-math.json': 'schemas/curriculum-import.schema.json',
};

const warnings = [];
const ajv = new Ajv2020({ strict: true, allowMatchingProperties: true, allErrors: true, logger: { log() {}, warn: (m) => warnings.push(m), error: console.error } });
addFormats(ajv);

// Quét đệ quy (vd. schemas/activities/).
const listJson = (d) => fs.readdirSync(path.join(specs, d), { withFileTypes: true }).flatMap((e) =>
  e.isDirectory() ? listJson(`${d}/${e.name}`) : e.name.endsWith('.json') ? [`${d}/${e.name}`] : []);
const files = ['schemas', 'events'].flatMap(listJson);
const ids = {};
for (const f of files) { const s = JSON.parse(fs.readFileSync(path.join(specs, f), 'utf8')); ids[f] = s.$id; ajv.addSchema(s); }

let failed = 0;
for (const f of files) {
  try { ajv.getSchema(ids[f]); } catch (e) { failed++; console.log(`COMPILE FAIL ${f}: ${e.message}`); }
}
for (const [ex, schema] of Object.entries(EXAMPLES)) {
  const validate = ajv.getSchema(ids[schema]);
  const ok = validate(JSON.parse(fs.readFileSync(path.join(specs, 'examples', ex), 'utf8')));
  if (!ok) { failed++; console.log(`INVALID examples/${ex}: ${JSON.stringify(validate.errors).slice(0, 500)}`); }
}
console.log(`validate-schemas: ${files.length} schema, ${Object.keys(EXAMPLES).length} example, ${warnings.length} cảnh báo strict, ${failed} lỗi`);
if (failed || warnings.length) process.exit(1);
