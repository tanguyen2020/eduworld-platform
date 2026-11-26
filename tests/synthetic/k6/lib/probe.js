// Synthetic probe logic (F131 T038, F131-AC-19, AC-20; TEST_STRATEGY §4). Pure functions shared by the k6 scripts and
// the node tests (student_login.test.js): no k6 import here, the HTTP client is injected (k6/http in production, a fake
// in tests). The probe only reads and logs a QA student in/out: it never starts an attempt or writes learning data.

export const RESULTS = ['ok', 'error', 'rate_limited'];
const RESULT_CODE = { ok: 0, error: 1, rate_limited: 2 };
export const resultCode = (r) => RESULT_CODE[r];
export const resultOf = (code) => RESULTS[code] ?? 'error';

/** ok for 2xx, rate_limited for 429 (LOGIN_QUEUE_BUSY / RATE_LIMITED), error otherwise. */
export function classify(status) {
  if (status === 429) return 'rate_limited';
  return status >= 200 && status < 300 ? 'ok' : 'error';
}

function header(headers, name) {
  if (!headers) return undefined;
  const k = Object.keys(headers).find((h) => h.toLowerCase() === name.toLowerCase());
  return k === undefined ? undefined : headers[k];
}

/** Seconds to wait from a Retry-After header (delta-seconds or HTTP date); default 60, bounded to [1, 3600]. */
export function retryAfterSeconds(headers, nowMs = Date.now()) {
  const v = header(headers, 'Retry-After');
  let s = 60;
  if (v !== undefined && /^\s*\d+\s*$/.test(String(v))) s = Number(v);
  else if (v !== undefined && !Number.isNaN(Date.parse(v))) s = Math.ceil((Date.parse(v) - nowMs) / 1000);
  return Math.min(3600, Math.max(1, s));
}

/** next_allowed timestamp kept from the previous run (its textfile), 0 when absent. */
export function parseState(text) {
  const m = /eduworld_synthetic_probe_next_allowed_timestamp_seconds\{[^}]*\}\s+([0-9.]+)/.exec(text ?? '');
  return m ? Number(m[1]) : 0;
}

const esc = (s) => String(s).replace(/\\/g, '\\\\').replace(/"/g, '\\"').replace(/\n/g, ' ');

/** Prometheus textfile (node_exporter textfile collector) of one probe run. Labels: probe, result only (R14). */
export function promText({ probe, result, durationSeconds, nowSeconds, nextAllowed = 0 }) {
  const p = `probe="${esc(probe)}"`;
  return [
    '# HELP eduworld_synthetic_probe_success 1 when the last run of the synthetic probe succeeded (F131-AC-19).',
    '# TYPE eduworld_synthetic_probe_success gauge',
    `eduworld_synthetic_probe_success{${p},result="${esc(result)}"} ${result === 'ok' ? 1 : 0}`,
    '# HELP eduworld_synthetic_probe_duration_seconds Duration of the last run of the synthetic probe.',
    '# TYPE eduworld_synthetic_probe_duration_seconds gauge',
    `eduworld_synthetic_probe_duration_seconds{${p}} ${Number(durationSeconds).toFixed(3)}`,
    '# HELP eduworld_synthetic_probe_last_run_timestamp_seconds Unix time of the last run (stale probe detection).',
    '# TYPE eduworld_synthetic_probe_last_run_timestamp_seconds gauge',
    `eduworld_synthetic_probe_last_run_timestamp_seconds{${p}} ${Math.floor(nowSeconds)}`,
    '# HELP eduworld_synthetic_probe_next_allowed_timestamp_seconds Earliest next call after a 429 Retry-After (0 = now).',
    '# TYPE eduworld_synthetic_probe_next_allowed_timestamp_seconds gauge',
    `eduworld_synthetic_probe_next_allowed_timestamp_seconds{${p}} ${Math.floor(nextAllowed)}`,
    '',
  ].join('\n');
}

function json(res) {
  try { return typeof res.json === 'function' ? res.json() : JSON.parse(res.body); } catch (e) { return null; }
}

/** Outcome of one HTTP step: null to continue, or a terminal { result, retryAfter }. */
function stop(res, nowMs) {
  const r = classify(res.status);
  if (r === 'ok') return null;
  return { result: r, retryAfter: r === 'rate_limited' ? retryAfterSeconds(res.headers, nowMs) : 0, status: res.status };
}

/** Probe client_config: GET /api/v1/game/client-config (no authentication). */
export function runClientConfig(http, cfg, nowMs = Date.now()) {
  const res = http.get(`${cfg.baseUrl}/api/v1/game/client-config?platform=${cfg.platform}&version=${cfg.clientVersion}`, { tags: { probe: 'client_config' }, timeout: '10s' });
  return stop(res, nowMs) ?? { result: 'ok', retryAfter: 0, status: res.status };
}

/**
 * Probe student_login: gameLogin → createGameSession → endGameSession → gameLogout with the QA student (tenant QA).
 * A 429 (LOGIN_QUEUE_BUSY / RATE_LIMITED) stops the chain at once: no retry inside the run, and the next run waits
 * for Retry-After (state in the textfile). Cleanup (end session, logout) still runs after a successful login.
 */
export function runStudentLogin(http, cfg, nowMs = Date.now()) {
  const common = { 'Content-Type': 'application/json', 'Accept-Language': 'vi', 'X-Device-Id': cfg.deviceId };
  const login = http.post(`${cfg.baseUrl}/api/v1/auth/game/login`, JSON.stringify({
    username: cfg.username, password: cfg.password, device_id: cfg.deviceId, client_version: cfg.clientVersion, platform: cfg.platform,
  }), { headers: common, tags: { probe: 'student_login', step: 'login' }, timeout: '15s' });
  const failed = stop(login, nowMs);
  if (failed) return { ...failed, step: 'login' };
  const body = json(login) ?? {};
  const access = body.tokens?.access_token;
  const refresh = body.tokens?.refresh_token;
  const tenant = body.bootstrap?.tenant_id;
  if (!access || !tenant) return { result: 'error', retryAfter: 0, step: 'login', status: login.status };
  const auth = { ...common, Authorization: `Bearer ${access}`, 'X-Tenant-Id': tenant };
  const session = http.post(`${cfg.baseUrl}/api/v1/game/sessions`, JSON.stringify({ client_platform: cfg.platform, client_version: cfg.clientVersion }),
    { headers: { ...auth, 'Idempotency-Key': cfg.idempotencyKey() }, tags: { probe: 'student_login', step: 'session' }, timeout: '10s' });
  let outcome = stop(session, nowMs);
  if (outcome) outcome = { ...outcome, step: 'session' };
  const sid = json(session)?.game_session_id;
  if (!outcome && sid) {
    const end = http.post(`${cfg.baseUrl}/api/v1/game/sessions/${sid}:end`, null, {
      headers: auth, tags: { probe: 'student_login', step: 'end' }, timeout: '10s',
    });
    const e = stop(end, nowMs);
    if (e) outcome = { ...e, step: 'end' };
  }
  const logout = http.post(`${cfg.baseUrl}/api/v1/auth/game/logout`, JSON.stringify({ refresh_token: refresh }),
    { headers: auth, tags: { probe: 'student_login', step: 'logout' }, timeout: '10s' });
  const l = stop(logout, nowMs);
  if (!outcome && l) outcome = { ...l, step: 'logout' };
  return outcome ?? { result: 'ok', retryAfter: 0, step: 'logout', status: logout.status };
}
