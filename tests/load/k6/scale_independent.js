// F132-AC-05 (TC-PERF-013 of NFR-SCAL-003) — independent scaling: a constant arrival rate (RATE req/s, chosen above
// what the baseline replicas of core-api serve) of read routes for 20 min; at SCALE_AT scripts/perf/run.sh scales
// ONE Swarm service (`--scale core-api=<peak>@10m`, then worker-chat in a second run). Each request is tagged
// phase=before|after. Pass criteria are Prometheus checks of scenarios.yaml: core-api throughput rises ≥ 20% and the
// other services (realtime) were not redeployed (replicas/updates unchanged).
import { check } from 'k6';
import { SharedArray } from 'k6/data';
import exec from 'k6/execution';
import { call, vuSession } from './lib/auth.js';
import { baseOptions, checkEnv, dur, scaled } from './lib/thresholds.js';

const gen = JSON.parse(open('./scenarios.gen.json'));
checkEnv(__ENV);
const BASE = __ENV.BASE_URL;
const bots = new SharedArray('bots', () => JSON.parse(open(__ENV.BOTS_FILE)));
const SCALE_AT_MS = parseInt(dur(__ENV.SCALE_AT || '10m', __ENV), 10) * 1000;

export const options = Object.assign(baseOptions(gen, 'scale_independent', __ENV), {
  scenarios: {
    steady: { executor: 'constant-arrival-rate', rate: scaled(Number(__ENV.RATE || 2000), __ENV), timeUnit: '1s', duration: dur('20m', __ENV),
      preAllocatedVUs: scaled(500, __ENV), maxVUs: scaled(5000, __ENV) },
  },
});

export default function () {
  const s = vuSession(BASE, bots, __ENV.EW_LOADBOT_PASSWORD);
  if (!s) return;
  exec.vu.metrics.tags.phase = exec.instance.currentTestRunDuration < SCALE_AT_MS ? 'before' : 'after';
  const r = call('GET', `${BASE}/game/profile`, s, 'read', 'getGameProfile');
  check(r, { 'profile ok': (x) => x.status === 200 });
}
