// F130-AC-24 (ADR-0041, AGENTS R33): deploy/swarm/<env>/scale-schedule.yaml is valid against
// deploy/swarm/schema/scale-schedule.schema.json (bounded strings, baseline/peak present) and the cross-field rules:
// peak ≥ baseline, windows start < end and not overlapping (lead_minutes included), every service exists in the stack
// files of the environment, no duplicate. Errors carry the file, line and JSON path. Same rules as `ops-scheduler validate`.
// Usage: node scripts/deploy/check-scale-schedule.mjs [envDir…] → exit 1 on violations.
import fs from 'node:fs';
import path from 'node:path';
import { createRequire } from 'node:module';
import { fileURLToPath } from 'node:url';
import { REPO_ROOT, yaml, loadPolicy, loadStacks, lineOf } from './lib-policy.mjs';

const require = createRequire(import.meta.url);
const Ajv = require('../node_modules/ajv');
const schema = JSON.parse(fs.readFileSync(path.join(REPO_ROOT, 'deploy', 'swarm', 'schema', 'scale-schedule.schema.json'), 'utf8'));
const validate = new Ajv({ allErrors: true, strict: false }).compile(schema);
const minutes = (s) => { const [h, m] = s.split(':').map(Number); return h * 60 + m; };

/** Line of a JSON path (/services/2/peak) in the YAML text: the key of the leaf under the n-th list item. */
function lineOfPath(text, p) {
  const parts = p.split('/').filter(Boolean);
  let line = 1;
  for (let i = 0; i < parts.length; i++) {
    const part = parts[i];
    if (/^\d+$/.test(part)) {
      let seen = -1;
      const lines = text.split(/\r?\n/);
      for (let j = line; j < lines.length; j++) {
        if (/^\s*-\s/.test(lines[j])) { seen++; if (seen === Number(part)) { line = j + 1; break; } }
      }
    } else {
      const next = lineOf(text, new RegExp(`(^|[\s{,])${part}:`), line, 0);
      if (next) line = next;
    }
  }
  return line;
}

export function checkFile(file, knownServices) {
  const out = [];
  const text = fs.readFileSync(file, 'utf8');
  let doc;
  try { doc = yaml.load(text); } catch (e) { return [{ file, line: (e.mark?.line ?? 0) + 1, path: '/', message: e.reason ?? e.message }]; }
  const add = (p, message) => out.push({ file, line: lineOfPath(text, p), path: p, message });
  if (!validate(doc)) for (const e of validate.errors) add(e.instancePath || '/', `${e.message}${e.params?.missingProperty ? ` (${e.params.missingProperty})` : ''}`);
  const spans = [];
  (doc?.windows ?? []).forEach((w, i) => {
    if (!/^\d\d:\d\d$/.test(w?.start ?? '') || !/^\d\d:\d\d$/.test(w?.end ?? '')) return;
    if (minutes(w.start) >= minutes(w.end)) add(`/windows/${i}/end`, 'must be after start (windows do not cross midnight)');
    else spans.push({ a: minutes(w.start) - (doc.lead_minutes ?? 0), b: minutes(w.end), i });
  });
  spans.sort((x, y) => x.a - y.a);
  for (let k = 1; k < spans.length; k++) if (spans[k].a < spans[k - 1].b) add(`/windows/${spans[k].i}`, `overlaps window "${doc.windows[spans[k - 1].i].name}" (lead_minutes included)`);
  const seen = new Set();
  (doc?.services ?? []).forEach((s, i) => {
    if (Number.isInteger(s?.peak) && Number.isInteger(s?.baseline) && s.peak < s.baseline) add(`/services/${i}/peak`, `peak ${s.peak} is lower than baseline ${s.baseline}`);
    if (typeof s?.name === 'string') {
      if (seen.has(s.name)) add(`/services/${i}/name`, `duplicate service "${s.name}"`);
      seen.add(s.name);
      if (knownServices && !knownServices.has(s.name)) add(`/services/${i}/name`, `service "${s.name}" is not in the stack files of this environment`);
    }
  });
  return out;
}

export function checkEnv(envDir, policy = loadPolicy()) {
  const file = path.join(envDir, 'scale-schedule.yaml');
  if (!fs.existsSync(file)) return [{ file, line: 1, path: '/', message: 'scale-schedule.yaml is missing' }];
  const known = new Set(loadStacks(envDir, policy).filter((s) => !s.missing).flatMap((s) => Object.keys(s.doc.services ?? {})));
  return checkFile(file, known);
}

export function main(argv = process.argv.slice(2)) {
  const policy = loadPolicy();
  const dirs = argv.length ? argv : policy.envs.map((e) => path.join(REPO_ROOT, 'deploy', 'swarm', e));
  const out = dirs.flatMap((d) => checkEnv(d, policy));
  for (const v of out) console.error(`${path.relative(process.cwd(), v.file)}:${v.line}: ${v.path}: ${v.message}`);
  if (out.length) { console.error(`check-scale-schedule: ${out.length} violation(s)`); return 1; }
  console.log(`check-scale-schedule: ${dirs.length} schedule(s) ok`);
  return 0;
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) process.exit(main());
