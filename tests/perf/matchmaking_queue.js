// TC-PERF-004 — matchmaking queue at 20% CCU (F104 T040, F104-AC-27, NFR-PERF-007: wait p90 ≤ 45 s when the queue has
// enough players of the same grade). Each VU is one student: it creates a ticket (Idempotency-Key), polls
// getMatchmakingTicket (REST fallback of the WebSocket push) until proposed, accepts the ready check, and records the
// wait until matched. Run on staging only:
//   k6 run -e BASE_URL=https://staging.example/api/v1 -e PLAYERS_FILE=players.json -e SUBJECT_ID=<uuid> tests/perf/matchmaking_queue.js
// PLAYERS_FILE is a JSON array of {"token","tenant_id"} game tokens of staging test students of one grade and school
// (never real accounts, never committed). Nothing here is hardcoded per environment (AGENTS R3).
import http from 'k6/http';
import { check, sleep } from 'k6';
import { Counter, Trend } from 'k6/metrics';
import { SharedArray } from 'k6/data';
import { uuidv4 } from 'https://jslib.k6.io/k6-utils/1.4.0/index.js';

const BASE = __ENV.BASE_URL;
const SUBJECT = __ENV.SUBJECT_ID;
const players = new SharedArray('players', () => JSON.parse(open(__ENV.PLAYERS_FILE)));

const wait = new Trend('eduworld_mm_wait_until_matched', true);
const serverErrors = new Counter('eduworld_mm_5xx');
const unmatched = new Counter('eduworld_mm_unmatched');

export const options = {
  scenarios: {
    queue: { executor: 'per-vu-iterations', vus: Math.min(players.length, Number(__ENV.VUS || players.length)), iterations: 3, maxDuration: '20m' },
  },
  thresholds: {
    eduworld_mm_wait_until_matched: ['p(90)<45000'], // NFR-PERF-007
    eduworld_mm_5xx: ['count==0'],
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

export default function () {
  const p = players[(__VU - 1) % players.length];
  const start = Date.now();
  const res = http.post(`${BASE}/competition/matchmaking/tickets`, JSON.stringify({ mode: 'battle_1v1', subject_id: SUBJECT, matchmaking_type: 'skill' }),
    headers(p, uuidv7()));
  if (res.status >= 500) serverErrors.add(1);
  if (!check(res, { 'ticket created': (r) => r.status === 201 })) {
    sleep(5);
    return;
  }
  const ticket = res.json('ticket_id');
  let accepted = '';
  while (Date.now() - start < 6 * 60 * 1000) {
    const g = http.get(`${BASE}/competition/matchmaking/tickets/${ticket}`, headers(p));
    if (g.status >= 500) serverErrors.add(1);
    const status = g.status === 200 ? g.json('status') : 'lost';
    if (status === 'matched') {
      wait.add(Date.now() - start);
      return;
    }
    if (status === 'proposed' && g.json('proposal_id') !== accepted) {
      accepted = g.json('proposal_id');
      const a = http.post(`${BASE}/competition/matches/${accepted}/proposal:accept`, null, headers(p));
      if (a.status >= 500) serverErrors.add(1);
    }
    if (status === 'cancelled' || status === 'expired' || status === 'lost') break;
    sleep(1);
  }
  unmatched.add(1);
}
