// Scenario helper of the chaos runner (F132 T021, data-model §1): parses tests/chaos/scenarios/*.yaml for
// run-chaos.sh (bash has no YAML parser; node is already required by the repo tooling) and validates them.
//   node scenario.mjs env <file> [target]        shell assignments (SC_*) for eval, ${TARGET} substituted
//   node scenario.mjs expect <file> <during|after> TSV: name, op, value, promql
//   node scenario.mjs alerts <file>              TSV: alert name, runbook
//   node scenario.mjs value                      first sample value of a Prometheus /api/v1/query JSON on stdin (NaN if none)
//   node scenario.mjs compare <v> <op> <t>       exit 0 when "v op t" holds (NaN never holds)
//   node scenario.mjs validate [dir]             schema + alert names exist in the F131 rules + runbooks exist
import fs from 'node:fs';
import path from 'node:path';
import { createRequire } from 'node:module';
import { fileURLToPath } from 'node:url';

const require = createRequire(import.meta.url);
const ROOT = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..', '..', '..');
const yaml = require(path.join(ROOT, 'scripts', 'node_modules', 'js-yaml'));

export const FAULTS = ['swarm_service_down', 'replica_kill', 'module_db_down', 'nats_down', 'valkey_down', 'db_failover',
  'node_drain', 'external_fault', 'latency'];
export const OPS = ['<', '<=', '>', '>=', '=='];
export const RECONCILE = ['attempts', 'chat', 'none'];
const MODULE_DBS = ['eduworld_identity', 'eduworld_learning', 'eduworld_game', 'eduworld_competition', 'eduworld_chat'];

export function load(file, target = '') {
  const doc = yaml.load(fs.readFileSync(file, 'utf8'));
  if (doc?.fault && typeof doc.fault.target === 'string') doc.fault.target = doc.fault.target.replace('${TARGET}', target);
  return doc;
}

/** Returns a list of problems of one scenario document (empty = valid). alertNames: Set of F131 alert names. */
export function validate(doc, { alertNames, root = ROOT, file = '' } = {}) {
  const errs = [];
  const add = (m) => errs.push(`${path.basename(file)}: ${m}`);
  if (!doc || typeof doc !== 'object') return [`${file}: not a mapping`];
  if (typeof doc.id !== 'string' || !doc.id || doc.id.length > 32) add('id (TC code, <= 32 chars) required');
  if (typeof doc.name !== 'string' || !/^[a-z0-9-]+$/.test(doc.name)) add('name (kebab-case) required');
  if (file && doc.name && path.basename(file, '.yaml') !== doc.name) add(`name ${doc.name} differs from the file name`);
  if (!Array.isArray(doc.acceptance) || !doc.acceptance.every((a) => /^F132-AC-\d{2}$/.test(a))) add('acceptance: list of F132-AC-NN');
  if (doc.tool !== 'chaos') add('tool must be chaos');
  const l = doc.load ?? {};
  if (typeof l.scenario !== 'string' || typeof l.factor !== 'number' || l.of !== 'target_ccu') add('load {scenario, factor, of: target_ccu} (parameter by name, never its value)');
  const f = doc.fault ?? {};
  if (!FAULTS.includes(f.type)) add(`fault.type must be one of ${FAULTS.join(', ')}`);
  if (typeof f.target !== 'string') add('fault.target required');
  if (f.type === 'module_db_down' && !MODULE_DBS.includes(f.target)) add(`module_db_down target must be one of ${MODULE_DBS.join(', ')}`);
  if (!Number.isInteger(f.duration) || f.duration < 30 || f.duration > 3600) add('fault.duration: seconds in [30, 3600]');
  if (!Number.isInteger(doc.recovery_timeout) || doc.recovery_timeout < 60) add('recovery_timeout: seconds >= 60');
  if (!RECONCILE.includes(doc.reconcile)) add(`reconcile must be one of ${RECONCILE.join(', ')}`);
  const phases = doc.expect ?? {};
  let n = 0;
  for (const phase of ['during', 'after']) {
    for (const e of phases[phase] ?? []) {
      n++;
      if (!e.name || typeof e.promql !== 'string' || !OPS.includes(e.op) || typeof e.value !== 'number') add(`expect.${phase}: {name, promql, op, value} (${e.name ?? '?'})`);
      if (typeof e.promql === 'string' && /[\t\n]/.test(e.promql.trim())) add(`expect.${phase}: promql on one line (${e.name})`);
    }
  }
  if (n === 0) add('at least one expectation');
  if (!Array.isArray(doc.alerts) || doc.alerts.length === 0) add('alerts: at least one expected F131 alert');
  for (const a of doc.alerts ?? []) {
    if (alertNames && !alertNames.has(a.name)) add(`alert ${a.name} is not an alert of deploy/observability/prometheus/rules`);
    if (a.runbook !== `docs/11-operations/runbooks/${a.name}.md`) add(`alert ${a.name}: runbook must be docs/11-operations/runbooks/${a.name}.md`);
    else if (!fs.existsSync(path.join(root, a.runbook))) add(`runbook ${a.runbook} missing`);
  }
  return errs;
}

export function alertNames(root = ROOT) {
  const dir = path.join(root, 'deploy', 'observability', 'prometheus', 'rules');
  const names = new Set();
  for (const f of fs.readdirSync(dir).filter((x) => x.endsWith('.yaml'))) {
    for (const g of yaml.load(fs.readFileSync(path.join(dir, f), 'utf8'))?.groups ?? []) {
      for (const r of g.rules ?? []) if (r.alert) names.add(r.alert);
    }
  }
  return names;
}

export function compare(v, op, t) {
  const a = Number(v), b = Number(t);
  if (Number.isNaN(a) || Number.isNaN(b)) return false;
  switch (op) {
    case '<': return a < b;
    case '<=': return a <= b;
    case '>': return a > b;
    case '>=': return a >= b;
    case '==': return a === b;
    default: return false;
  }
}

const q = (s) => `'${String(s).replace(/'/g, `'\\''`)}'`;

function main(argv) {
  const [cmd, ...a] = argv;
  switch (cmd) {
    case 'env': {
      const d = load(a[0], a[1] ?? '');
      const p = d.fault?.params ?? {};
      const out = {
        SC_ID: d.id, SC_NAME: d.name, SC_TYPE: d.fault.type, SC_TARGET: d.fault.target, SC_DURATION: d.fault.duration,
        SC_RECOVERY_TIMEOUT: d.recovery_timeout, SC_RECONCILE: d.reconcile, SC_ACCEPTANCE: (d.acceptance ?? []).join(' '),
        SC_LOAD: `${d.load.factor}x ${d.load.of} (${d.load.scenario})`,
      };
      for (const [k, v] of Object.entries(p)) out[`SC_P_${k.toUpperCase()}`] = v;
      for (const [k, v] of Object.entries(out)) console.log(`${k}=${q(v)}`);
      return 0;
    }
    case 'expect':
      for (const e of load(a[0]).expect?.[a[1]] ?? []) console.log([e.name, e.op, e.value, e.promql.trim()].join('\t'));
      return 0;
    case 'alerts':
      for (const x of load(a[0]).alerts ?? []) console.log(`${x.name}\t${x.runbook}`);
      return 0;
    case 'value': {
      let v = 'NaN';
      try {
        const j = JSON.parse(fs.readFileSync(0, 'utf8'));
        const r = j?.data?.result;
        if (Array.isArray(r) && r.length) v = r[0].value?.[1] ?? 'NaN';
        else if (j?.data?.resultType === 'scalar') v = j.data.result[1];
      } catch { /* NaN */ }
      console.log(v);
      return 0;
    }
    case 'compare':
      return compare(a[0], a[1], a[2]) ? 0 : 1;
    case 'validate': {
      const dir = a[0] ?? path.join(ROOT, 'tests', 'chaos', 'scenarios');
      const names = alertNames();
      const errs = [];
      const files = fs.readdirSync(dir).filter((x) => x.endsWith('.yaml'));
      for (const f of files) errs.push(...validate(load(path.join(dir, f), 'example'), { alertNames: names, file: path.join(dir, f) }));
      for (const e of errs) console.error(e);
      console.log(`chaos scenarios: ${files.length} checked, ${errs.length} problem(s)`);
      return errs.length ? 1 : 0;
    }
    default:
      console.error('usage: scenario.mjs env|expect|alerts|value|compare|validate …');
      return 2;
  }
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) process.exit(main(process.argv.slice(2)));
