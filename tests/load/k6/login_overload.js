// TC-PERF-016 — login overload (F132-AC-03, FR-AUTH-027): 2× the design peak (2 × login_capacity_peak_rps password
// logins/s) for 5 min. The login queue must answer 429 LOGIN_QUEUE_BUSY with Retry-After, never a cascading 5xx;
// clients (lib/auth.js, like F061) wait Retry-After + jitter and 100% get in after the peak (eduworld_login_gave_up
// == 0). loadbot -scenario login runs alongside to prove every bot then reaches the WebSocket. tests/perf/login_queue.js
// (F013) stays the module-level check of the same queue.
import { SharedArray } from 'k6/data';
import exec from 'k6/execution';
import { login } from './lib/auth.js';
import { baseOptions, checkEnv, dur, param, scaled } from './lib/thresholds.js';

const gen = JSON.parse(open('./scenarios.gen.json'));
checkEnv(__ENV);
const BASE = __ENV.BASE_URL;
const bots = new SharedArray('bots', () => JSON.parse(open(__ENV.BOTS_FILE)));
const PEAK = scaled(param(gen, 'login_capacity_peak_rps'), __ENV);

export const options = Object.assign(baseOptions(gen, 'login_overload', __ENV), {
  scenarios: {
    overload: { executor: 'ramping-arrival-rate', startRate: 0, timeUnit: '1s', preAllocatedVUs: PEAK * 4, maxVUs: PEAK * 20,
      stages: [{ target: PEAK, duration: dur('60s', __ENV) }, { target: PEAK * 2, duration: dur('5m', __ENV) }, { target: 0, duration: dur('60s', __ENV) }] },
  },
});

export default function () {
  // A failed login after the client's retries counts in eduworld_login_gave_up (threshold count==0).
  login(BASE, bots[exec.scenario.iterationInTest % bots.length], __ENV.EW_LOADBOT_PASSWORD, { maxAttempts: 60 });
}
