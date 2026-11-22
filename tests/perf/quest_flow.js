// TC-PERF-013 — quest flow game → learning (F072 T047, plan "Performance Goals": listQuests p95 ≤ 300 ms, the
// createAttempt facade call behind startQuestActivity p95 < 50 ms, measured server side by
// eduworld_internal_client_duration_seconds{operation="createAttempt"}). Each VU is one student: listQuests, getQuest,
// acceptQuest (Idempotency-Key), then startQuestActivity of the learning step of the quest, and abandonQuest so the next
// iteration starts again. The quest must be a published learning quest assigned to the tenant whose first step is an
// `activity` step (depends_on: []), e.g. published through the Game Operations Console. Run on staging only:
//   k6 run -e BASE_URL=https://staging.example/api/v1 -e PLAYERS_FILE=players.json -e QUEST_ID=<uuid> -e STEP_ID=<step> tests/perf/quest_flow.js
// PLAYERS_FILE is a JSON array of {"token","tenant_id"} game tokens of staging test students (never real accounts, never
// committed). Nothing here is hardcoded per environment (AGENTS R3).
import http from 'k6/http';
import { check, sleep } from 'k6';
import { Counter, Trend } from 'k6/metrics';
import { SharedArray } from 'k6/data';
import { uuidv4 } from 'https://jslib.k6.io/k6-utils/1.4.0/index.js';

const BASE = __ENV.BASE_URL;
const QUEST = __ENV.QUEST_ID;
const STEP = __ENV.STEP_ID;
const players = new SharedArray('players', () => JSON.parse(open(__ENV.PLAYERS_FILE)));

const listTime = new Trend('eduworld_quest_list_ms', true);
const startTime = new Trend('eduworld_quest_start_activity_ms', true);
const serverErrors = new Counter('eduworld_quest_5xx');

export const options = {
  scenarios: {
    quests: { executor: 'ramping-vus', startVUs: 0, stages: [{ duration: '2m', target: Number(__ENV.VUS || 200) }, { duration: '5m', target: Number(__ENV.VUS || 200) }] },
  },
  thresholds: {
    eduworld_quest_list_ms: ['p(95)<300'],
    eduworld_quest_5xx: ['count==0'],
    checks: ['rate>0.99'],
  },
};

function headers(p, idem) {
  const h = { Authorization: `Bearer ${p.token}`, 'X-Tenant-Id': p.tenant_id, 'Content-Type': 'application/json' };
  if (idem) h['Idempotency-Key'] = idem;
  return { headers: h };
}

// uuidv7 builds a time-ordered key (Idempotency-Key must be a recent UUIDv7).
function uuidv7() {
  const hex = Date.now().toString(16).padStart(12, '0');
  const r = uuidv4().replace(/-/g, '');
  return `${hex.slice(0, 8)}-${hex.slice(8, 12)}-7${r.slice(13, 16)}-${r.slice(16, 20)}-${r.slice(20, 32)}`;
}

function errors(res) {
  if (res.status >= 500) serverErrors.add(1);
}

export default function () {
  const p = players[(__VU - 1) % players.length];
  const list = http.get(`${BASE}/game/quests?limit=20`, headers(p));
  errors(list);
  listTime.add(list.timings.duration);
  check(list, { 'list 200': (r) => r.status === 200 });
  const get = http.get(`${BASE}/game/quests/${QUEST}`, headers(p));
  errors(get);
  check(get, { 'get 200': (r) => r.status === 200 });
  const acc = http.post(`${BASE}/game/quests/${QUEST}:accept`, '{}', headers(p, uuidv7()));
  errors(acc);
  if (!check(acc, { 'accepted': (r) => r.status === 200 })) {
    sleep(1);
    return;
  }
  const st = http.post(`${BASE}/game/quests/${QUEST}/steps/${STEP}/activities:start`, '{}', headers(p, uuidv7()));
  errors(st);
  startTime.add(st.timings.duration);
  check(st, { 'attempt created (or daily limit)': (r) => r.status === 201 || r.status === 409 });
  const ab = http.post(`${BASE}/game/quests/${QUEST}:abandon`, null, headers(p, uuidv7()));
  errors(ab);
  sleep(1);
}
