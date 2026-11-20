// pnpm gen:schemas (F053 T010, research R2, AGENTS R33): copies the content JSON Schemas the editors validate with
// (ajv 2020-12) from specs/schemas — the single source — into src/shared/api/schemas/ (generated, never edited by
// hand). `--check` fails (exit 1) when a committed copy differs (part of `pnpm gen:check`).
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const HERE = path.dirname(fileURLToPath(import.meta.url));
export const PORTAL_DIR = path.resolve(HERE, '..');
export const REPO_ROOT = path.resolve(PORTAL_DIR, '..', '..', '..', '..', '..');
export const OUT_DIR = path.join(PORTAL_DIR, 'src/shared/api/schemas');

/** specs/schemas source → file name in OUT_DIR. */
export const SCHEMAS = {
  'specs/schemas/question.schema.json': 'question.schema.json',
  'specs/schemas/common/enums.schema.json': 'enums.schema.json',
  // Activity Builder (F058, F066): config of each activity type, checked with ajv before saving.
  'specs/schemas/activities/activity-base.schema.json': 'activities/activity-base.schema.json',
  'specs/schemas/activities/quiz.schema.json': 'activities/quiz.schema.json',
  'specs/schemas/activities/interactive-exercise.schema.json': 'activities/interactive-exercise.schema.json',
  'specs/schemas/activities/video-lesson.schema.json': 'activities/video-lesson.schema.json',
  'specs/schemas/activities/audio-lesson.schema.json': 'activities/audio-lesson.schema.json',
  'specs/schemas/activities/simulation.schema.json': 'activities/simulation.schema.json',
};

const norm = (s) => s.replace(/\r\n/g, '\n');

export function expected() {
  return Object.entries(SCHEMAS).map(([src, name]) => ({ name, content: norm(fs.readFileSync(path.join(REPO_ROOT, src), 'utf8')) }));
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  const check = process.argv.includes('--check');
  let drift = 0;
  fs.mkdirSync(path.join(OUT_DIR, 'activities'), { recursive: true });
  for (const { name, content } of expected()) {
    const out = path.join(OUT_DIR, name);
    const cur = fs.existsSync(out) ? norm(fs.readFileSync(out, 'utf8')) : null;
    if (check) {
      if (cur !== content) {
        console.error(`gen:schemas: ${path.relative(REPO_ROOT, out)} differs from its source — run pnpm gen:schemas`);
        drift++;
      }
    } else if (cur !== content) {
      fs.writeFileSync(out, content);
      console.log(`gen:schemas: wrote ${path.relative(REPO_ROOT, out)}`);
    }
  }
  if (drift) process.exit(1);
  if (check) console.log(`gen:schemas: ${Object.keys(SCHEMAS).length} schema(s) up to date`);
}
