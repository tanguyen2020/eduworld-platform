// TC-PERF-002 — school-start login storm (F132-AC-02): login_capacity_target — 5,000 password logins at a peak of
// 100/s (worst case: every entry types a password, ADR-0064 §6) — then MODE=mixed: half of the entries come back with
// refresh mode=resume (ADR-0086). ops-scheduler must have scaled to `peak` first (TC-PERF-017; checked by
// scripts/perf/run.sh). Keycloak CPU < 70% and Keycloak p95 are read from Prometheus / keycloak-benchmark.
import http from 'k6/http';
import { SharedArray } from 'k6/data';
import exec from 'k6/execution';
import { Trend } from 'k6/metrics';
import { login, track } from './lib/auth.js';
import { baseOptions, checkEnv, dur, param, scaled } from './lib/thresholds.js';

const gen = JSON.parse(open('./scenarios.gen.json'));
checkEnv(__ENV);
const BASE = __ENV.BASE_URL;
const MIXED = __ENV.MODE === 'mixed';
const bots = new SharedArray('bots', () => JSON.parse(open(__ENV.BOTS_FILE)));
const PEAK = scaled(param(gen, 'login_capacity_peak_rps'), __ENV);
const refreshTime = new Trend('eduworld_refresh_ms', true);
const refreshTokens = {}; // per VU: username → refresh token of an earlier login of this VU

// 0 → peak in 60 s (≈ 3,000 logins) then peak → 0 in 40 s (≈ 2,000): login_capacity_storm in one burst.
export const options = Object.assign(baseOptions(gen, 'login_storm', __ENV), {
  scenarios: {
    storm: { executor: 'ramping-arrival-rate', startRate: 0, timeUnit: '1s', preAllocatedVUs: PEAK * 3, maxVUs: PEAK * 10,
      stages: [{ target: PEAK, duration: dur('60s', __ENV) }, { target: 0, duration: dur('40s', __ENV) }] },
  },
});

export default function () {
  const bot = bots[exec.scenario.iterationInTest % bots.length];
  const rt = refreshTokens[bot.username];
  if (MIXED && rt && Math.random() < 0.5) {
    const t0 = Date.now();
    const r = track(http.post(`${BASE}/auth/game/token:refresh`, JSON.stringify({ refresh_token: rt, device_id: bot.device_id, mode: 'resume', client_version: '1.0.0' }),
      { headers: { 'Content-Type': 'application/json' }, tags: { kind: 'refresh', name: 'refreshGameToken' } }));
    refreshTime.add(Date.now() - t0);
    if (r.status === 200) {
      refreshTokens[bot.username] = r.json('tokens.refresh_token');
      return;
    }
  }
  const s = login(BASE, bot, __ENV.EW_LOADBOT_PASSWORD);
  if (s && MIXED) refreshTokens[bot.username] = s.refresh;
}
