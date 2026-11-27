// TC-PERF-006 — soak 4 h at 0.7× CCU (F132-AC-04): same session model as baseline_api; memory trend and latency
// drift (p95 of the last 30 min vs the first 30 min after the ramp, perf_soak_latency_drift_max) are checked by
// scripts/perf/run.sh from Prometheus and the k6 time series.
import { SharedArray } from 'k6/data';
import { vuSession } from './lib/auth.js';
import { restIteration } from './lib/session.js';
import { baseOptions, checkEnv, dur, param, scaled } from './lib/thresholds.js';

const gen = JSON.parse(open('./scenarios.gen.json'));
checkEnv(__ENV);
const BASE = __ENV.BASE_URL;
const bots = new SharedArray('bots', () => JSON.parse(open(__ENV.BOTS_FILE)));
const VUS = scaled(Math.round(0.7 * param(gen, 'target_ccu')), __ENV);

export const options = Object.assign(baseOptions(gen, 'soak', __ENV), {
  scenarios: {
    soak: { executor: 'ramping-vus', startVUs: 0, gracefulRampDown: '30s',
      stages: [{ target: VUS, duration: dur('10m', __ENV) }, { target: VUS, duration: dur('4h', __ENV) }] },
  },
});

export default function () {
  const s = vuSession(BASE, bots, __ENV.EW_LOADBOT_PASSWORD);
  if (s) restIteration(BASE, s, __ENV);
}
