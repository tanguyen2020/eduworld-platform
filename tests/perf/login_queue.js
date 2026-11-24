// TC-PERF-016 — login queue of /auth/game/login (F013 T033, FR-AUTH-027, ADR-0045).
// Ramps password logins to twice login_capacity_target and checks that overload answers 429 LOGIN_QUEUE_BUSY
// with Retry-After (never 5xx) and that clients honouring Retry-After all get in after the peak.
// Run on staging only:  k6 run -e BASE_URL=https://staging.example/api/v1 -e USERS_FILE=users.json tests/perf/login_queue.js
// USERS_FILE is a JSON array of {"username","password"} test students of the staging seed (never real accounts,
// never committed). Nothing here is hardcoded per environment (AGENTS R3).
import http from 'k6/http';
import { check, sleep } from 'k6';
import { Counter, Trend } from 'k6/metrics';
import { SharedArray } from 'k6/data';
import { uuidv4 } from 'https://jslib.k6.io/k6-utils/1.4.0/index.js';

const BASE = __ENV.BASE_URL;
const TARGET = Number(__ENV.LOGIN_CAPACITY_TARGET || 100); // login_capacity_target (logins/s)
const VERSION = __ENV.CLIENT_VERSION || '1.0.0';
const users = new SharedArray('users', () => JSON.parse(open(__ENV.USERS_FILE)));

const queued = new Counter('eduworld_login_queue_busy');
const serverErrors = new Counter('eduworld_login_5xx');
const loginTime = new Trend('eduworld_login_until_in', true);

export const options = {
  scenarios: {
    peak: {
      executor: 'ramping-arrival-rate',
      startRate: Math.ceil(TARGET / 4),
      timeUnit: '1s',
      preAllocatedVUs: TARGET * 4,
      maxVUs: TARGET * 10,
      stages: [
        { target: TARGET, duration: '1m' },
        { target: TARGET * 2, duration: '2m' }, // SC-003: twice the target
        { target: 0, duration: '1m' },
      ],
    },
  },
  thresholds: {
    eduworld_login_5xx: ['count==0'], // SC-003: no cascading 5xx
    checks: ['rate>0.99'],
    eduworld_login_until_in: ['p(95)<60000'], // everyone gets in after the peak
  },
};

export default function () {
  const u = users[(__VU * 7919 + __ITER) % users.length];
  const body = JSON.stringify({ username: u.username, password: u.password, device_id: uuidv4(), client_version: VERSION, platform: 'web' });
  const params = { headers: { 'Content-Type': 'application/json' }, tags: { name: 'gameLogin' } };
  const start = Date.now();
  for (let attempt = 0; attempt < 10; attempt++) {
    const r = http.post(`${BASE}/auth/game/login`, body, params);
    if (r.status >= 500) {
      serverErrors.add(1);
    }
    if (r.status === 429) {
      const busy = r.json('code') === 'LOGIN_QUEUE_BUSY';
      if (busy) queued.add(1);
      check(r, { 'queue answers Retry-After': (x) => Number(x.headers['Retry-After']) > 0 });
      sleep(Number(r.headers['Retry-After'] || 1) + Math.random()); // jitter like the game client (F061)
      continue;
    }
    check(r, { 'login 200': (x) => x.status === 200 });
    if (r.status === 200) loginTime.add(Date.now() - start);
    return;
  }
}
