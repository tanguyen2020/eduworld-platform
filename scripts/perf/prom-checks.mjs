// Evaluates the Prometheus checks of one run of tests/load/scenarios.yaml against the staging Prometheus (F131) over
// the run window and writes prom.json for render-report.mjs (F132-AC-01/02/05/06). `$window` becomes the run length,
// `$end`/`$scale_at` unix seconds (PromQL @ modifier). A query returning no sample is "no data" (observed null), which
// fails the run: a missing metric is never a pass.
// Usage: node scripts/perf/prom-checks.mjs --run=<run> --prom=<url> --start=<unix s> --end=<unix s> [--scale-at=<unix s>] --out=prom.json
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { load } from './render-scenarios.mjs';

const OPS = { '<': (a, b) => a < b, '<=': (a, b) => a <= b, '==': (a, b) => a === b, '>=': (a, b) => a >= b, '>': (a, b) => a > b };

/** Substitutes the run placeholders of a query. */
export function expand(query, { start, end, scaleAt }) {
  const win = `${Math.max(60, Math.round(end - start))}s`;
  return query.replaceAll('$window', win).replaceAll('$end', String(Math.round(end))).replaceAll('$scale_at', String(Math.round(scaleAt ?? (start + end) / 2)));
}

/** Runs every check of `run`; fetchFn(url) → Response-like {ok, json()}. */
export async function evaluate(doc, run, { prom, start, end, scaleAt }, fetchFn = fetch) {
  const sc = (doc.scenarios || []).find((s) => s.run === run);
  if (!sc) throw new Error(`run ${run} is not in scenarios.yaml`);
  const out = [];
  for (const c of sc.prom || []) {
    const q = expand(c.query, { start, end, scaleAt });
    let observed = null;
    try {
      const res = await fetchFn(`${prom.replace(/\/$/, '')}/api/v1/query?query=${encodeURIComponent(q)}&time=${Math.round(end)}`);
      const body = await res.json();
      const r = body && body.data && body.data.result;
      if (res.ok && body.status === 'success' && Array.isArray(r) && r.length) {
        observed = Math.max(...r.map((x) => Number(x.value[1])).filter((n) => !Number.isNaN(n)));
        if (!Number.isFinite(observed)) observed = null;
      } else if (res.ok && body.status === 'success' && body.data && body.data.resultType === 'scalar') {
        observed = Number(body.data.result[1]);
      }
    } catch {
      observed = null;
    }
    out.push({ name: c.name, query: q, op: c.op, value: c.value, ref: c.ref, observed, ok: observed !== null && OPS[c.op](observed, c.value) });
  }
  return out;
}

function arg(name) {
  const a = process.argv.find((x) => x.startsWith(`--${name}=`));
  return a ? a.slice(name.length + 3) : undefined;
}

if (process.argv[1] && fileURLToPath(import.meta.url) === path.resolve(process.argv[1])) {
  const [run, prom, out] = [arg('run'), arg('prom'), arg('out')];
  const start = Number(arg('start'));
  const end = Number(arg('end'));
  if (!run || !prom || !out || !start || !end) {
    console.error('usage: prom-checks.mjs --run=<run> --prom=<url> --start=<s> --end=<s> [--scale-at=<s>] --out=<file>');
    process.exit(2);
  }
  const scaleAt = arg('scale-at') ? Number(arg('scale-at')) : undefined;
  const res = await evaluate(load(), run, { prom, start, end, scaleAt });
  fs.writeFileSync(out, `${JSON.stringify(res, null, 2)}\n`);
  const bad = res.filter((r) => !r.ok).length;
  console.log(`prom-checks: ${res.length - bad}/${res.length} ok → ${out}`);
}
