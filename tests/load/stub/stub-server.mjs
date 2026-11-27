// TEST STAND-IN of the public API for the k6 smoke runs of F132 (T010; AGENTS R17): it answers the routes the load
// scripts call with contract-shaped bodies, so a script, its thresholds and the report pipeline can be exercised on
// a laptop or in CI without any EduWorld service. It implements no rule of the real system and its numbers are never
// evidence of capacity. Modes (STUB_MODE):
//   ok      fast 2xx answers (thresholds pass)
//   slow    every read answers after 300 ms (> NFR-PERF-001 200 ms → read thresholds fail)
//   errors  5% of answers are 503 (error budget 0.5% → thresholds fail)
//   queue   the first login of every 3 answers 429 LOGIN_QUEUE_BUSY with Retry-After: 1 (login_overload passes)
// Usage: STUB_MODE=ok PORT=8080 node tests/load/stub/stub-server.mjs
import http from 'node:http';
import { randomUUID } from 'node:crypto';

const MODE = process.env.STUB_MODE || 'ok';
const PORT = Number(process.env.PORT || 8080);
let logins = 0;

function send(res, status, body, headers = {}) {
  res.writeHead(status, { 'Content-Type': status >= 400 ? 'application/problem+json' : 'application/json', ...headers });
  res.end(body === undefined ? '' : JSON.stringify(body));
}

const routes = [
  ['POST', /^\/api\/v1\/auth\/game\/login$/, () => {
    logins++;
    if (MODE === 'queue' && logins % 3 === 1) return [429, { status: 429, code: 'LOGIN_QUEUE_BUSY' }, { 'Retry-After': '1' }];
    return [200, { tokens: { access_token: 'stub-token', refresh_token: 'stub-refresh', token_type: 'Bearer', expires_in: 300, refresh_expires_in: 3600 },
      bootstrap: { tenant_id: '00000000-0000-4000-8000-000000000001', user: { user_id: randomUUID() }, student: { student_id: randomUUID(), grade_level: 3 } } }];
  }],
  ['POST', /^\/api\/v1\/auth\/game\/token:refresh$/, () => [200, { tokens: { access_token: 'stub-token', refresh_token: 'stub-refresh' } }]],
  ['POST', /^\/api\/v1\/game\/sessions$/, () => [201, { game_session_id: randomUUID(), started_at: new Date().toISOString(), login_at: new Date().toISOString() }]],
  ['POST', /^\/api\/v1\/game\/sessions\/[^/]+\/realtime-tickets$/, () => [201, { ticket: 'stub', expires_at: new Date().toISOString(), url: 'ws://stub/realtime/v1', protocol_version: 1 }]],
  ['GET', /^\/api\/v1\/game\/client-config/, () => [200, { min_client_version: '1.0.0' }]],
  ['GET', /^\/api\/v1\/game\/(profile|content-manifest|world\/manifest|quests)/, () => [200, { data: [] }]],
  ['GET', /^\/api\/v1\/chat\/channels$/, () => [200, { data: [] }]],
  ['GET', /^\/api\/v1\/(chat\/search|leaderboards\/|lms\/dashboards\/)/, () => [200, { data: [] }]],
  ['POST', /^\/api\/v1\/learning\/activities\/[^/]+\/attempts$/, () => [201, { attempt_id: randomUUID(), mode: 'practice', status: 'in_progress', started_at: new Date().toISOString(),
    items: [{ position: 1, question_version_id: randomUUID(), type: 'single_choice', render_payload: { options: [{ option_id: 'a' }, { option_id: 'b' }] } }] }]],
  ['POST', /^\/api\/v1\/learning\/activity-attempts\/[^/]+\/responses$/, () => [200, {}]],
  ['POST', /^\/api\/v1\/learning\/activity-attempts\/[^/]+:submit$/, () => [200, { status: 'submitted' }]],
];

http.createServer((req, res) => {
  req.resume();
  req.on('end', () => {
    const r = routes.find(([m, re]) => m === req.method && re.test(req.url));
    if (!r) return send(res, 404, { status: 404, code: 'NOT_FOUND' });
    if (MODE === 'errors' && Math.random() < 0.05) return send(res, 503, { status: 503, code: 'UNAVAILABLE' });
    const [status, body, headers] = r[2]();
    const delay = MODE === 'slow' && req.method === 'GET' ? 300 : 0;
    setTimeout(() => send(res, status, body, headers), delay);
  });
}).listen(PORT, () => console.log(`stub-server (TEST STAND-IN) mode=${MODE} on :${PORT}`));
