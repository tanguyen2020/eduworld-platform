// Thresholds and load sizing of the F132 k6 scenarios (T010). Pure functions: k6 scripts pass the parsed
// tests/load/k6/scenarios.gen.json (rendered from tests/load/scenarios.yaml) and __ENV; node tests pass fixtures.
// No number is defined here: thresholds come from scenarios.yaml, where each one names its parameter/NFR.

/** Environments a load script may target (src/backend/tools/envguard, F132-AC-07). */
export const ALLOWED_ENVS = ['local', 'dev', 'staging'];

/** Throws unless EW_ENV names a test environment; returns the normalised value. */
export function checkEnv(env) {
  const e = String((env && env.EW_ENV) || '').trim().toLowerCase();
  if (!ALLOWED_ENVS.includes(e)) {
    throw new Error(`EW_ENV=${JSON.stringify((env && env.EW_ENV) || '')} refused: load scripts run only on ${ALLOWED_ENVS.join(', ')} (F132-AC-07)`);
  }
  return e;
}

/** k6 thresholds object of one run name: { metric: [expr, …] }, merged when a metric has several criteria. */
export function thresholdsFor(gen, run) {
  const r = gen && gen.runs && gen.runs[run];
  if (!r) throw new Error(`run ${run} is not in scenarios.gen.json`);
  const out = {};
  for (const t of r.k6) {
    (out[t.metric] = out[t.metric] || []).push(t.expr);
  }
  return out;
}

/** Parameter value of scenarios.yaml (e.g. target_ccu). */
export function param(gen, name) {
  const v = gen && gen.parameters && gen.parameters[name];
  if (v === undefined) throw new Error(`parameter ${name} is not in scenarios.gen.json`);
  return v;
}

/** Scales a load figure by SCALE (smoke runs use e.g. SCALE=0.001); never below 1. */
export function scaled(value, env) {
  const f = Number((env && env.SCALE) || 1);
  return Math.max(1, Math.round(value * (f > 0 ? f : 1)));
}

/** Scales a k6 duration ("10m", "4h", "30s") by DURATION_SCALE; never below 1 s. */
export function dur(text, env) {
  const m = /^(\d+)(s|m|h)$/.exec(text);
  if (!m) throw new Error(`bad duration ${text}`);
  const secs = Number(m[1]) * { s: 1, m: 60, h: 3600 }[m[2]];
  const f = Number((env && env.DURATION_SCALE) || 1);
  return `${Math.max(1, Math.round(secs * (f > 0 ? f : 1)))}s`;
}

/** Common k6 options: thresholds of the run, testid tag (Prometheus remote write label of F131 panels), summary stats. */
export function baseOptions(gen, run, env) {
  return {
    thresholds: thresholdsFor(gen, run),
    tags: { testid: (env && env.RUN_ID) || `adhoc-${run}`, run, tc: gen.runs[run].id },
    summaryTrendStats: ['avg', 'min', 'med', 'p(90)', 'p(95)', 'p(99)', 'max', 'count'],
    discardResponseBodies: false,
  };
}
