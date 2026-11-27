// TC-PERF-013 — inter-module load at 1× CCU (F132-AC-04): the quest flow game → learning facade (listQuests,
// acceptQuest, startQuestActivity → createAttempt of learning, abandonQuest) as in tests/perf/quest_flow.js (F072),
// while loadbot -scenario battle drives competition → learning (battle scoring). Facade p95 < 50 ms (NFR-PERF-011),
// outbox lag < 1 s (NFR-PERF-012) and pool waits come from Prometheus (scenarios.yaml prom checks).
import { check, sleep } from 'k6';
import { SharedArray } from 'k6/data';
import { call, vuSession } from './lib/auth.js';
import { baseOptions, checkEnv, dur, param, scaled } from './lib/thresholds.js';

const gen = JSON.parse(open('./scenarios.gen.json'));
checkEnv(__ENV);
const BASE = __ENV.BASE_URL;
const QUEST = __ENV.QUEST_ID;
const STEP = __ENV.STEP_ID;
const bots = new SharedArray('bots', () => JSON.parse(open(__ENV.BOTS_FILE)));
const VUS = scaled(param(gen, 'target_ccu'), __ENV);

export const options = Object.assign(baseOptions(gen, 'inter_module', __ENV), {
  scenarios: {
    quests: { executor: 'ramping-vus', startVUs: 0, gracefulRampDown: '30s',
      stages: [{ target: VUS, duration: dur('10m', __ENV) }, { target: VUS, duration: dur('30m', __ENV) }] },
  },
});

export default function () {
  const s = vuSession(BASE, bots, __ENV.EW_LOADBOT_PASSWORD);
  if (!s || !QUEST || !STEP) return;
  const k = Number(__ENV.THINK_SCALE || 1);
  check(call('GET', `${BASE}/game/quests?limit=20`, s, 'read', 'listQuests'), { 'quests listed': (r) => r.status === 200 });
  const acc = call('POST', `${BASE}/game/quests/${QUEST}:accept`, s, 'write', 'acceptQuest', {});
  if (check(acc, { 'quest accepted': (r) => r.status === 200 })) {
    const st = call('POST', `${BASE}/game/quests/${QUEST}/steps/${STEP}/activities:start`, s, 'write', 'startQuestActivity', {});
    check(st, { 'attempt created (or daily limit)': (r) => r.status === 201 || r.status === 409 });
    call('POST', `${BASE}/game/quests/${QUEST}:abandon`, s, 'write', 'abandonQuest');
  }
  sleep(20 * k);
}
