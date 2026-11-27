// TC-PERF-007 — stress until saturation (F132-AC-04): +0.25× target_ccu every 5 min up to 2×. The system must degrade
// with 429 (rate limits, login queue, dashboard busy), never with cascading 5xx; the saturation point (first stage
// whose read p95 breaks NFR-PERF-001 or whose 429 rate passes 1%) is computed by scripts/perf/render-report.mjs from
// the per-stage tag.
import { SharedArray } from 'k6/data';
import exec from 'k6/execution';
import { vuSession } from './lib/auth.js';
import { restIteration } from './lib/session.js';
import { baseOptions, checkEnv, dur, param, scaled } from './lib/thresholds.js';

const gen = JSON.parse(open('./scenarios.gen.json'));
checkEnv(__ENV);
const BASE = __ENV.BASE_URL;
const bots = new SharedArray('bots', () => JSON.parse(open(__ENV.BOTS_FILE)));
const STAGE_S = parseInt(dur('5m', __ENV), 10);
const STEP = scaled(Math.round(0.25 * param(gen, 'target_ccu')), __ENV);

export const options = Object.assign(baseOptions(gen, 'stress', __ENV), {
  scenarios: {
    stress: { executor: 'ramping-vus', startVUs: 0, gracefulRampDown: '30s',
      stages: Array.from({ length: 8 }, (_, i) => ({ target: STEP * (i + 1), duration: dur('5m', __ENV) })) },
  },
});

export default function () {
  const s = vuSession(BASE, bots, __ENV.EW_LOADBOT_PASSWORD);
  if (!s) return;
  exec.vu.metrics.tags.stage = String(Math.floor(exec.instance.currentTestRunDuration / 1000 / STAGE_S));
  restIteration(BASE, s, __ENV);
}
