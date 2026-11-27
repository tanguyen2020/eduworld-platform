// Warm-up precondition of TC-PERF-002 and TC-PERF-014 (TC-PERF-017, F132-AC-02, warm_up_windows, F130): every
// service of deploy/swarm/<env>/scale-schedule.yaml must run at least its `peak` replicas (swarm-exporter metric
// swarm_service_replicas_running, F131) before the run starts. Writes {peak_ok, checked_at, services[]}.
// Usage: node scripts/perf/warmup-check.mjs --env=staging --prom=<url> [--out=warmup.json]   (exit 1 when not at peak)
import fs from 'node:fs';
import path from 'node:path';
import { createRequire } from 'node:module';
import { fileURLToPath } from 'node:url';

const require = createRequire(import.meta.url);
const yaml = require('../node_modules/js-yaml');
const ROOT = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../..');

/** Peak replicas per service of the scale schedule of env. */
export function peaks(env) {
  const doc = yaml.load(fs.readFileSync(path.join(ROOT, `deploy/swarm/${env}/scale-schedule.yaml`), 'utf8'));
  return Object.fromEntries((doc.services || []).map((s) => [s.name, s.peak]));
}

/** Compares running replicas (Prometheus) with the peak of every service. */
export async function check(env, prom, fetchFn = fetch, now = new Date()) {
  const want = peaks(env);
  const q = 'max by (swarm_service) (swarm_service_replicas_running)';
  const res = await fetchFn(`${prom.replace(/\/$/, '')}/api/v1/query?query=${encodeURIComponent(q)}`);
  const body = await res.json();
  const running = Object.fromEntries(((body.data && body.data.result) || []).map((r) => [r.metric.swarm_service, Number(r.value[1])]));
  const services = Object.entries(want).map(([name, peak]) => ({ name, peak, running: running[name] ?? null, ok: (running[name] ?? -1) >= peak }));
  return { peak_ok: services.every((s) => s.ok), checked_at: now.toISOString(), services };
}

function arg(name) {
  const a = process.argv.find((x) => x.startsWith(`--${name}=`));
  return a ? a.slice(name.length + 3) : undefined;
}

if (process.argv[1] && fileURLToPath(import.meta.url) === path.resolve(process.argv[1])) {
  const env = arg('env');
  const prom = arg('prom');
  if (!env || !prom) {
    console.error('usage: warmup-check.mjs --env=<env> --prom=<url> [--out=<file>]');
    process.exit(2);
  }
  const r = await check(env, prom);
  const text = `${JSON.stringify(r, null, 2)}\n`;
  if (arg('out')) fs.writeFileSync(arg('out'), text);
  for (const s of r.services.filter((x) => !x.ok)) console.error(`warmup-check: ${s.name} runs ${s.running ?? 'no data'} < peak ${s.peak}`);
  console.log(`warmup-check: ${r.peak_ok ? 'every service at peak' : 'NOT at peak'}`);
  process.exit(r.peak_ok ? 0 : 1);
}
