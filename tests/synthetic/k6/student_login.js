// k6 synthetic probe `student_login` (F131 T038, F131-AC-19, AC-20): QA student of the QA tenant logs in, creates and
// ends a game session, logs out. Run every 5 minutes by supercronic in the service synthetic-probe (obs stack); the
// result is a node_exporter textfile (eduworld_synthetic_probe_*), never a learning write. Credentials come from Docker
// secret files; a 429 is recorded as result="rate_limited" and the next run waits for Retry-After.
//   EW_PROBE_BASE_URL, EW_PROBE_USERNAME, EW_PROBE_PASSWORD_FILE, EW_PROBE_DEVICE_ID, EW_PROBE_CLIENT_VERSION,
//   EW_PROBE_PLATFORM (default windows), EW_PROBE_TEXTFILE_DIR (default /textfile)
import http from 'k6/http';
import crypto from 'k6/crypto';
import { Gauge } from 'k6/metrics';
import { runStudentLogin, promText, parseState, resultCode, resultOf } from './lib/probe.js';

const PROBE = 'student_login';
const dir = __ENV.EW_PROBE_TEXTFILE_DIR || '/textfile';
const final = `${dir}/eduworld_synthetic_probe_${PROBE}.prom`;
let previous = '';
try { previous = open(final); } catch (e) { previous = ''; }
const cfg = {
  baseUrl: __ENV.EW_PROBE_BASE_URL,
  username: __ENV.EW_PROBE_USERNAME,
  password: open(__ENV.EW_PROBE_PASSWORD_FILE).trim(),
  deviceId: __ENV.EW_PROBE_DEVICE_ID,
  clientVersion: __ENV.EW_PROBE_CLIENT_VERSION,
  platform: __ENV.EW_PROBE_PLATFORM || 'windows',
  idempotencyKey: () => uuid(),
};
function uuid() {
  const b = new Uint8Array(crypto.randomBytes(16));
  b[6] = (b[6] & 0x0f) | 0x40; b[8] = (b[8] & 0x3f) | 0x80;
  const h = Array.from(b, (x) => x.toString(16).padStart(2, '0')).join('');
  return `${h.slice(0, 8)}-${h.slice(8, 12)}-${h.slice(12, 16)}-${h.slice(16, 20)}-${h.slice(20)}`;
}

export const options = { vus: 1, iterations: 1, summaryTrendStats: ['max'], noConnectionReuse: true };
const gResult = new Gauge('probe_result');
const gDuration = new Gauge('probe_duration_seconds');
const gNext = new Gauge('probe_next_allowed');

export default function () {
  const now = Date.now();
  const nextAllowed = parseState(previous);
  if (now / 1000 < nextAllowed) { // still inside Retry-After of the previous run: do not call at all
    gResult.add(resultCode('rate_limited')); gDuration.add(0); gNext.add(nextAllowed);
    return;
  }
  const t0 = Date.now();
  const r = runStudentLogin(http, cfg, now);
  gResult.add(resultCode(r.result));
  gDuration.add((Date.now() - t0) / 1000);
  gNext.add(r.result === 'rate_limited' ? Math.floor(now / 1000) + r.retryAfter : 0);
}

export function handleSummary(data) {
  const v = (m) => data.metrics[m]?.values?.value ?? 0;
  const text = promText({ probe: PROBE, result: resultOf(v('probe_result')), durationSeconds: v('probe_duration_seconds'), nowSeconds: Date.now() / 1000, nextAllowed: v('probe_next_allowed') });
  return { [`${dir}/.${PROBE}.prom.tmp`]: text, stdout: `${PROBE} ${resultOf(v('probe_result'))}\n` };
}
