// k6 synthetic probe `client_config` (F131 T038, F131-AC-19): GET /api/v1/game/client-config without authentication,
// every 5 minutes (service synthetic-probe). Result as a node_exporter textfile (eduworld_synthetic_probe_*).
//   EW_PROBE_BASE_URL, EW_PROBE_CLIENT_VERSION, EW_PROBE_PLATFORM (default windows), EW_PROBE_TEXTFILE_DIR (default /textfile)
import http from 'k6/http';
import { Gauge } from 'k6/metrics';
import { runClientConfig, promText, parseState, resultCode, resultOf } from './lib/probe.js';

const PROBE = 'client_config';
const dir = __ENV.EW_PROBE_TEXTFILE_DIR || '/textfile';
let previous = '';
try { previous = open(`${dir}/eduworld_synthetic_probe_${PROBE}.prom`); } catch (e) { previous = ''; }
const cfg = { baseUrl: __ENV.EW_PROBE_BASE_URL, clientVersion: __ENV.EW_PROBE_CLIENT_VERSION, platform: __ENV.EW_PROBE_PLATFORM || 'windows' };

export const options = { vus: 1, iterations: 1, summaryTrendStats: ['max'], noConnectionReuse: true };
const gResult = new Gauge('probe_result');
const gDuration = new Gauge('probe_duration_seconds');
const gNext = new Gauge('probe_next_allowed');

export default function () {
  const now = Date.now();
  const nextAllowed = parseState(previous);
  if (now / 1000 < nextAllowed) { gResult.add(resultCode('rate_limited')); gDuration.add(0); gNext.add(nextAllowed); return; }
  const r = runClientConfig(http, cfg, now);
  gResult.add(resultCode(r.result));
  gDuration.add((Date.now() - now) / 1000);
  gNext.add(r.result === 'rate_limited' ? Math.floor(now / 1000) + r.retryAfter : 0);
}

export function handleSummary(data) {
  const v = (m) => data.metrics[m]?.values?.value ?? 0;
  const text = promText({ probe: PROBE, result: resultOf(v('probe_result')), durationSeconds: v('probe_duration_seconds'), nowSeconds: Date.now() / 1000, nextAllowed: v('probe_next_allowed') });
  return { [`${dir}/.${PROBE}.prom.tmp`]: text, stdout: `${PROBE} ${resultOf(v('probe_result'))}\n` };
}
