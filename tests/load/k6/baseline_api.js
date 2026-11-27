// TC-PERF-001 — baseline API (F132-AC-04): ramp to 1× CCU (target_ccu) in 10 min, hold 30 min; every VU is one game
// session doing the REST share of TEST_STRATEGY §8.1. Thresholds: tests/load/scenarios.yaml (NFR-PERF-001/002,
// error < 0.5%). Run through scripts/perf/run.sh (EW_ENV, BASE_URL, BOTS_FILE, EW_LOADBOT_PASSWORD, RUN_ID; SCALE and
// DURATION_SCALE shrink a smoke run).
import { SharedArray } from 'k6/data';
import { vuSession } from './lib/auth.js';
import { restIteration } from './lib/session.js';
import { baseOptions, checkEnv, dur, param, scaled } from './lib/thresholds.js';

const gen = JSON.parse(open('./scenarios.gen.json'));
checkEnv(__ENV);
const BASE = __ENV.BASE_URL;
const bots = new SharedArray('bots', () => JSON.parse(open(__ENV.BOTS_FILE)));
const VUS = scaled(param(gen, 'target_ccu'), __ENV);

export const options = Object.assign(baseOptions(gen, 'baseline_api', __ENV), {
  scenarios: {
    baseline: { executor: 'ramping-vus', startVUs: 0, gracefulRampDown: '30s',
      stages: [{ target: VUS, duration: dur('10m', __ENV) }, { target: VUS, duration: dur('30m', __ENV) }] },
  },
});

export default function () {
  const s = vuSession(BASE, bots, __ENV.EW_LOADBOT_PASSWORD);
  if (s) restIteration(BASE, s, __ENV);
}
