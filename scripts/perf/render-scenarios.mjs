// Renders tests/load/scenarios.yaml (source) into tests/load/k6/scenarios.gen.json, the file k6 scripts open() for
// their thresholds (k6 has no YAML parser). F132 T010/T013. Also validates the scenario file: every entry has an id,
// a run name and a tool; every k6 threshold has metric/expr/ref; every Prometheus check has a query, an operator and
// a numeric value; every TC-PERF of TEST_STRATEGY §8.2 is covered or listed out of scope.
// Usage: node scripts/perf/render-scenarios.mjs [--check]   (exit 1 when stale or invalid)
import fs from 'node:fs';
import path from 'node:path';
import { createRequire } from 'node:module';
import { fileURLToPath } from 'node:url';

const require = createRequire(import.meta.url);
const yaml = require('../node_modules/js-yaml');

const ROOT = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../..');
export const SOURCE = path.join(ROOT, 'tests/load/scenarios.yaml');
export const TARGET = path.join(ROOT, 'tests/load/k6/scenarios.gen.json');

// TC-PERF ids of TEST_STRATEGY §8.2 (001…025).
export const REQUIRED = Array.from({ length: 25 }, (_, i) => `TC-PERF-${String(i + 1).padStart(3, '0')}`);
const TOOLS = new Set(['k6', 'loadbot', 'chaos', 'keycloak-benchmark', 'staging']);
const OPS = new Set(['<', '<=', '==', '>=', '>']);

/** Returns a list of problems of a parsed scenario document. */
export function validate(doc) {
  const out = [];
  const seen = new Set();
  for (const s of doc.scenarios ?? []) {
    const at = s.id ?? '?';
    if (!s.id || !s.run || !s.tool) out.push(`${at}: id, run and tool are required`);
    if (seen.has(s.run)) out.push(`${at}: duplicate run name ${s.run}`);
    seen.add(s.run);
    if (s.tool && !TOOLS.has(s.tool)) out.push(`${at}: unknown tool ${s.tool}`);
    if (s.script && !fs.existsSync(path.join(ROOT, s.script))) out.push(`${at}: script ${s.script} does not exist`);
    for (const t of s.k6 ?? []) {
      if (!t.metric || !t.expr || !t.ref) out.push(`${at}: k6 threshold needs metric, expr, ref (${JSON.stringify(t)})`);
    }
    for (const p of s.prom ?? []) {
      if (!p.name || !p.query || !OPS.has(p.op) || typeof p.value !== 'number' || !p.ref) out.push(`${at}: prom check needs name, query, op, numeric value, ref (${p.name})`);
    }
  }
  const covered = new Set([...(doc.scenarios ?? []).map((s) => s.id), ...(doc.out_of_scope ?? []).flatMap((o) => o.ids ?? [])]);
  for (const id of REQUIRED) if (!covered.has(id)) out.push(`${id}: neither a scenario nor out of scope`);
  return out;
}

/** The JSON consumed by k6 and run.sh: parameters and, per run name, id, tool, script, loadbot args, k6 thresholds. */
export function render(doc) {
  const runs = {};
  for (const s of doc.scenarios ?? []) {
    runs[s.run] = { id: s.id, tool: s.tool, script: s.script || null, args: s.args || null, precondition: s.precondition || null, k6: (s.k6 ?? []).map(({ metric, expr, ref }) => ({ metric, expr, ref })) };
  }
  const parameters = Object.fromEntries(Object.entries(doc.parameters ?? {}).map(([k, v]) => [k, v.value]));
  return `${JSON.stringify({ generated_from: 'tests/load/scenarios.yaml', parameters, runs }, null, 2)}\n`;
}

export function load(file = SOURCE) {
  return yaml.load(fs.readFileSync(file, 'utf8'));
}

if (process.argv[1] && fileURLToPath(import.meta.url) === path.resolve(process.argv[1])) {
  const doc = load();
  const problems = validate(doc);
  if (problems.length) {
    for (const p of problems) console.error(`render-scenarios: ${p}`);
    process.exit(1);
  }
  const text = render(doc);
  if (process.argv.includes('--check')) {
    const cur = fs.existsSync(TARGET) ? fs.readFileSync(TARGET, 'utf8').replace(/\r\n/g, '\n') : '';
    if (cur !== text) {
      console.error('render-scenarios: tests/load/k6/scenarios.gen.json is stale; run node scripts/perf/render-scenarios.mjs');
      process.exit(1);
    }
    console.log(`render-scenarios: ${Object.keys(JSON.parse(text).runs).length} runs, fresh`);
  } else {
    fs.writeFileSync(TARGET, text);
    console.log(`render-scenarios: wrote ${path.relative(ROOT, TARGET)}`);
  }
}
