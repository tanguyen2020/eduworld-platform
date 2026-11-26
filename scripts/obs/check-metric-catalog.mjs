// Mandatory metrics of MONITORING §3 present with the right type, unit suffix and labels (F131 T018, F131-AC-02).
// Sources (one or more):
//   --metrics=<binary>=<file|url>   raw /metrics of a binary (core-api, realtime, worker-events|chat|jobs) or exporter
//                                   (apisix, postgres, nats); for backend binaries the labels Prometheus attaches at
//                                   scrape (service, profile — MONITORING §2) are added, as on Swarm;
//   --prom=<url>                    a Prometheus (staging): reads the series of the last 30 minutes (job
//                                   obs-metric-catalog of release.yml, after the staging deploy);
//   --static=<dir>                  Go sources: lists mandatory metrics never named in the code (report only, exit 0
//                                   unless --strict) — early signal for the module owners.
// Checks: every main-table metric is present (wildcards like pg_stat_database_* need one match); a Counter ends with
// _total, a Histogram has _bucket series and a unit suffix (_seconds/_bytes), a Gauge does not end with _total; when
// the "Label chính" column lists `service`/`module`, every series of that metric carries them.
// Usage: node scripts/obs/check-metric-catalog.mjs (--metrics=… | --prom=… | --static=…) [--origin=backend|apisix|postgres|nats]…
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { MONITORING, readMonitoring, parseMetrics, metricOrigin, report } from './lib-monitoring-md.mjs';
import { parseExposition } from './check-label-policy.mjs';

const BACKEND = { 'core-api': { service: 'core-api' }, realtime: { service: 'realtime' } };
for (const p of ['events', 'chat', 'jobs']) BACKEND[`worker-${p}`] = { service: 'worker', profile: p };

/** Static naming rules of one catalogue entry (independent of any scrape). */
export function namingViolations(m) {
  const out = [];
  if (m.name.endsWith('*') || !m.name.startsWith('eduworld_')) return out; // exporter names are given by the exporter
  if (m.type === 'counter' && !m.name.endsWith('_total')) out.push(`counter ${m.name} must end with _total`);
  if (m.type === 'gauge' && m.name.endsWith('_total')) out.push(`gauge ${m.name} must not end with _total`);
  if (m.type === 'histogram' && !/_(seconds|bytes)$/.test(m.name)) out.push(`histogram ${m.name} needs a unit suffix (_seconds/_bytes)`);
  return out;
}

/** samples: [{ name, labels }] from all sources. Returns violations for the catalogue entries of `origins`. */
export function checkCatalog(catalog, samples, { origins = null, file = MONITORING } = {}) {
  const v = [];
  const byName = new Map();
  for (const s of samples) {
    if (!byName.has(s.name)) byName.set(s.name, []);
    byName.get(s.name).push(s);
  }
  const names = [...byName.keys()];
  for (const m of catalog) {
    if (origins && !origins.includes(metricOrigin(m))) continue;
    const add = (rule, msg) => v.push({ file, line: m.line, rule, message: msg });
    for (const msg of namingViolations(m)) add('naming', msg);
    let series;
    if (m.name.endsWith('*')) {
      const pre = m.name.slice(0, -1);
      series = names.filter((n) => n.startsWith(pre)).flatMap((n) => byName.get(n));
    } else if (m.type === 'histogram' && m.name.endsWith('_bucket')) {
      series = byName.get(m.name) ?? [];
    } else if (m.type === 'histogram') {
      series = byName.get(`${m.name}_bucket`) ?? [];
      if (!series.length && byName.has(m.name)) add('type', `${m.name} is exposed but not as a histogram (no _bucket series)`);
    } else {
      series = byName.get(m.name) ?? [];
      if (m.type === 'counter' && !series.length && byName.has(m.name.replace(/_total$/, ''))) series = byName.get(m.name.replace(/_total$/, ''));
    }
    if (!series.length) { add('missing', `mandatory metric ${m.name} (${metricOrigin(m)}: ${m.source.replace(/`/g, '').slice(0, 60)}) is not exposed`); continue; }
    for (const l of ['service', 'module'].filter((x) => m.labels.includes(x))) {
      if (series.some((s) => !s.labels[l])) add('label', `${m.name}: series without label ${l}`);
    }
  }
  return v;
}

async function fetchText(src) {
  if (/^https?:\/\//.test(src)) {
    const r = await fetch(src, { signal: AbortSignal.timeout(15000) });
    if (!r.ok) throw new Error(`${src}: HTTP ${r.status}`);
    return r.text();
  }
  return fs.readFileSync(src, 'utf8');
}

/** Series of Prometheus for the catalogue (last 30 minutes). */
async function promSamples(base, catalog) {
  const out = [];
  const end = Math.floor(Date.now() / 1000);
  for (const m of catalog) {
    const sel = m.name.endsWith('*') ? `{__name__=~"${m.name.slice(0, -1)}.*"}` : (m.type === 'histogram' && !m.name.endsWith('_bucket') ? `${m.name}_bucket` : m.name);
    const u = `${base.replace(/\/$/, '')}/api/v1/series?match[]=${encodeURIComponent(sel)}&start=${end - 1800}&end=${end}`;
    const r = await fetch(u, { signal: AbortSignal.timeout(15000) });
    if (!r.ok) throw new Error(`Prometheus ${r.status} for ${sel}`);
    const body = await r.json();
    for (const s of body.data ?? []) { const { __name__: name, ...labels } = s; out.push({ name, labels }); }
  }
  return out;
}

/** Mandatory metrics never named in the Go sources (string literal or Name: field). */
export function staticMissing(catalog, dir) {
  const text = [];
  const walk = (d) => {
    for (const e of fs.readdirSync(d, { withFileTypes: true })) {
      const p = path.join(d, e.name);
      if (e.isDirectory()) { if (!['node_modules', '.git', 'testdata'].includes(e.name)) walk(p); } else if (e.name.endsWith('.go') && !e.name.endsWith('_test.go')) text.push(fs.readFileSync(p, 'utf8'));
    }
  };
  walk(dir);
  const all = text.join('\n');
  return catalog.filter((m) => metricOrigin(m) === 'backend' && !m.name.endsWith('*') && !all.includes(`"${m.name}"`));
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  const args = process.argv.slice(2);
  const get = (k) => args.filter((a) => a.startsWith(`--${k}=`)).map((a) => a.slice(k.length + 3));
  const { main } = parseMetrics(readMonitoring(get('monitoring')[0] ?? MONITORING));
  if (get('static').length) {
    const miss = staticMissing(main, get('static')[0]);
    for (const m of miss) console.log(`not in code: ${m.name} (${m.source.replace(/`/g, '').slice(0, 70)})`);
    console.log(`check-metric-catalog --static: ${main.length - miss.length - main.filter((m) => metricOrigin(m) !== 'backend' || m.name.endsWith('*')).length} found, ${miss.length} mandatory backend metric(s) not named in the code`);
    process.exit(args.includes('--strict') && miss.length ? 1 : 0);
  }
  const samples = [];
  for (const spec of get('metrics')) {
    const i = spec.indexOf('=');
    const [binary, src] = i > 0 && !/^[a-z]:[\\/]/i.test(spec) ? [spec.slice(0, i), spec.slice(i + 1)] : ['', spec];
    const extra = BACKEND[binary] ?? {};
    for (const s of parseExposition(await fetchText(src))) samples.push({ name: s.name, labels: { ...extra, ...s.labels } });
  }
  for (const base of get('prom')) samples.push(...await promSamples(base, main));
  if (!get('metrics').length && !get('prom').length) { console.error('usage: --metrics=<binary>=<file|url> | --prom=<url> | --static=<dir>'); process.exit(2); }
  const origins = get('origin').length ? get('origin') : null;
  const v = checkCatalog(main, samples, { origins });
  process.exit(report('check-metric-catalog', v));
}
