// TC-PERF-014 — 10,000 CCU whole system (F132-AC-01). The players are loadbot (-scenario ccu: login, WebSocket, hub
// 70% / match 20% / learning 10%, their REST calls); this k6 script runs beside it and carries the traffic loadbot
// does not: every client boot calls getClientConfig (unauthenticated) at the arrival rate of target_ccu clients over
// the ramp, then the reconnect churn of the hold, and READERS logged-in sessions (a bot slice disjoint from loadbot,
// BOT_OFFSET) reading profile/manifests/leaderboard. Both report under the same RUN_ID (testid) so F131 panels and the
// perf report join them. WS per replica, circuit breakers and restarts are Prometheus checks of scenarios.yaml.
import http from 'k6/http';
import { check } from 'k6';
import { SharedArray } from 'k6/data';
import { track, vuSession } from './lib/auth.js';
import { bootstrap } from './lib/session.js';
import { baseOptions, checkEnv, dur, param, scaled } from './lib/thresholds.js';

const gen = JSON.parse(open('./scenarios.gen.json'));
checkEnv(__ENV);
const BASE = __ENV.BASE_URL;
const bots = new SharedArray('bots', () => JSON.parse(open(__ENV.BOTS_FILE)));
const CCU = scaled(param(gen, 'target_ccu'), __ENV);
const RAMP_S = parseInt(dur('10m', __ENV), 10);
const READERS = scaled(Number(__ENV.READERS || 0), __ENV);
const OFFSET = Number(__ENV.BOT_OFFSET || CCU);

const scenarios = {
  boot: { executor: 'constant-arrival-rate', rate: Math.max(1, Math.ceil(CCU / RAMP_S)), timeUnit: '1s', duration: dur('10m', __ENV),
    preAllocatedVUs: 50, maxVUs: 500, exec: 'boot' },
  churn: { executor: 'constant-arrival-rate', rate: Math.max(1, Math.ceil(CCU / 1800)), timeUnit: '1s', duration: dur('60m', __ENV),
    startTime: dur('10m', __ENV), preAllocatedVUs: 20, maxVUs: 200, exec: 'boot' }, // a 30-min session renews ~1/1800 s
};
if (Number(__ENV.READERS || 0) > 0) {
  scenarios.readers = { executor: 'constant-vus', vus: READERS, duration: dur('70m', __ENV), exec: 'reader' };
}

export const options = Object.assign(baseOptions(gen, 'ccu_10k', __ENV), { scenarios });

export function boot() {
  const r = track(http.get(`${BASE}/game/client-config?platform=web&version=${__ENV.CLIENT_VERSION || '1.0.0'}`,
    { tags: { kind: 'read', name: 'getClientConfig' } }));
  check(r, { 'client config': (x) => x.status === 200 || x.status === 429 });
}

export function reader() {
  const s = vuSession(BASE, bots, __ENV.EW_LOADBOT_PASSWORD, OFFSET);
  if (s) bootstrap(BASE, s, __ENV);
}
