// TC-PERF-012 — chat search over 10 million messages (F132-AC-04): players search their channels (tenant +
// membership filters are enforced by the server). The 10M messages of perf-* tenants are produced beforehand by bot
// runs (chat_burst, soak), never real chat. SEARCH_TERMS are neutral words of the bot phrases.
import { check, sleep } from 'k6';
import { SharedArray } from 'k6/data';
import { call, vuSession } from './lib/auth.js';
import { baseOptions, checkEnv, dur, scaled } from './lib/thresholds.js';

const gen = JSON.parse(open('./scenarios.gen.json'));
checkEnv(__ENV);
const BASE = __ENV.BASE_URL;
const bots = new SharedArray('bots', () => JSON.parse(open(__ENV.BOTS_FILE)));
const TERMS = (__ENV.SEARCH_TERMS || 'chào,sẵn sàng,cố lên,cảm ơn').split(',');

export const options = Object.assign(baseOptions(gen, 'chat_search', __ENV), {
  scenarios: { search: { executor: 'constant-vus', vus: scaled(200, __ENV), duration: dur('10m', __ENV) } },
});

export default function () {
  const s = vuSession(BASE, bots, __ENV.EW_LOADBOT_PASSWORD);
  if (!s) return;
  const q = encodeURIComponent(TERMS[Math.floor(Math.random() * TERMS.length)]);
  check(call('GET', `${BASE}/chat/search?q=${q}&limit=20`, s, 'search', 'searchMessages'), { 'search ok': (r) => r.status === 200 || r.status === 429 });
  sleep(Number(__ENV.THINK_SCALE || 1) * 3);
}
