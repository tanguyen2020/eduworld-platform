// Login and request helpers of the F132 k6 scenarios: behave like the game client (F061) — gameLogin honouring the
// login queue (429 LOGIN_QUEUE_BUSY → wait Retry-After + jitter, never a tight retry), createGameSession with
// X-Device-Id (one device per bot, AGENTS R30) and Idempotency-Key (UUIDv7) — and tag every request with kind/name
// so scenarios.yaml thresholds select them. Bots come from bots.json of datagen; the password is EW_LOADBOT_PASSWORD
// (a secret of the run, never committed, never logged). No remote module import: scripts run offline.
import http from 'k6/http';
import { sleep } from 'k6';
import { Counter, Rate, Trend } from 'k6/metrics';
import { retryWait, uuidv7 } from './util.js';

export { retryWait, uuidv4, uuidv7 } from './util.js';

export const serverErrors = new Counter('eduworld_http_5xx');
export const serverErrorRate = new Rate('eduworld_http_5xx_rate');
export const throttled = new Counter('eduworld_http_429');
export const queueBusy = new Counter('eduworld_login_queue_busy');
export const rateLimited = new Counter('eduworld_login_rate_limited');
export const retryAfterMissing = new Counter('eduworld_login_retry_after_missing');
export const gaveUp = new Counter('eduworld_login_gave_up');
export const untilIn = new Trend('eduworld_login_until_in', true);

/** Counts 5xx (eduworld_http_5xx, eduworld_http_5xx_rate) and 429 (eduworld_http_429) answers. */
export function track(res) {
  const five = res.status >= 500 || res.status === 0;
  if (five) serverErrors.add(1);
  serverErrorRate.add(five);
  if (res.status === 429) throttled.add(1);
  return res;
}

function params(tags, headers) {
  return { headers: Object.assign({ 'Content-Type': 'application/json', 'Accept-Language': 'vi' }, headers || {}), tags };
}

/**
 * gameLogin → createGameSession like the game. Returns { token, tenantId, gameSessionId, deviceId } or null after
 * maxAttempts. bot = { username, device_id }.
 */
export function login(base, bot, password, opts = {}) {
  const version = opts.clientVersion || '1.0.0';
  const platform = opts.platform || 'web';
  const maxAttempts = opts.maxAttempts || 30;
  const start = Date.now();
  const body = JSON.stringify({ username: bot.username, password, device_id: bot.device_id, client_version: version, platform });
  for (let i = 0; i < maxAttempts; i++) {
    const r = track(http.post(`${base}/auth/game/login`, body, params({ kind: 'login', name: 'gameLogin' })));
    if (r.status === 200) {
      const s = { token: r.json('tokens.access_token'), refresh: r.json('tokens.refresh_token'), tenantId: r.json('bootstrap.tenant_id'), deviceId: bot.device_id };
      if (opts.session !== false) {
        const g = track(http.post(`${base}/game/sessions`, JSON.stringify({ client_platform: platform, client_version: version }),
          params({ kind: 'session', name: 'createGameSession' }, sessionHeaders(s, { idem: true, device: true }))));
        if (g.status !== 201) return null;
        s.gameSessionId = g.json('game_session_id');
      }
      untilIn.add(Date.now() - start);
      return s;
    }
    if (r.status === 429) {
      if (r.json('code') === 'RATE_LIMITED') rateLimited.add(1);
      else queueBusy.add(1);
      if (!r.headers['Retry-After']) retryAfterMissing.add(1);
      sleep(retryWait(r.headers['Retry-After']));
      continue;
    }
    if (r.status >= 500 || r.status === 0) {
      sleep(Math.min(2 ** i, 60) + Math.random());
      continue;
    }
    return null; // 401/403/422/426: terminal
  }
  gaveUp.add(1);
  return null;
}

/** Headers of an authenticated call; idem adds a fresh Idempotency-Key (or the given string). */
export function sessionHeaders(s, o = {}) {
  const h = { Authorization: `Bearer ${s.token}`, 'X-Tenant-Id': s.tenantId };
  if (o.idem) h['Idempotency-Key'] = typeof o.idem === 'string' ? o.idem : uuidv7();
  if (o.device) h['X-Device-Id'] = s.deviceId;
  return h;
}

/** Authenticated GET/POST/PUT tagged with kind (read/write/…) and name (operationId). */
export function call(method, url, s, kind, name, body, o = {}) {
  const p = params({ kind, name }, sessionHeaders(s, { idem: o.idem !== undefined ? o.idem : method !== 'GET' }));
  return track(http.request(method, url, body === undefined ? null : JSON.stringify(body), p));
}

/** The bot of this VU from bots.json (round robin), offset lets two scripts use disjoint bots. */
export function botFor(bots, vu, offset = 0) {
  return bots[(vu - 1 + offset) % bots.length];
}

let vuState = null; // module state is per VU in k6

/** The logged-in session of this VU (logs in once, on the first iteration). */
export function vuSession(base, bots, password, offset = 0) {
  if (!vuState) vuState = login(base, botFor(bots, __VU, offset), password);
  return vuState;
}
