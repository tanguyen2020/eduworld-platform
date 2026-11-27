// TC-PERF-010 — chat load (F132-AC-04): each player sends a chat message every CHAT_INTERVAL_S (6/min) on a channel
// it belongs to and times the ack (system.command.acked: moderation + JetStream ack, ADR-0035). At 1× CCU the Go bot
// (loadbot -scenario chat) is the driver; this k6 script checks the same path at smaller scale (VUS) and is the
// source of eduworld_chat_ack_ms. Persist lag / delivery come from Prometheus (scenarios.yaml).
import ws from 'k6/ws';
import { check } from 'k6';
import { SharedArray } from 'k6/data';
import { Counter, Trend } from 'k6/metrics';
import { uuidv4, vuSession } from './lib/auth.js';
import { envelope, textChannel, ticket, wsParams } from './lib/rt.js';
import { baseOptions, checkEnv, dur, scaled } from './lib/thresholds.js';

const gen = JSON.parse(open('./scenarios.gen.json'));
checkEnv(__ENV);
const BASE = __ENV.BASE_URL;
const bots = new SharedArray('bots', () => JSON.parse(open(__ENV.BOTS_FILE)));
const VUS = scaled(Number(__ENV.VUS || 1000), __ENV);
const HOLD_MS = parseInt(dur('30m', __ENV), 10) * 1000;
const INTERVAL_MS = Number(__ENV.CHAT_INTERVAL_S || 10) * 1000;
const ackTime = new Trend('eduworld_chat_ack_ms', true);
const failed = new Counter('eduworld_chat_send_failed');
const PHRASES = ['Xin chào cả nhóm', 'Mình sẵn sàng rồi', 'Cố lên nào', 'Cảm ơn bạn'];

export const options = Object.assign(baseOptions(gen, 'chat_load', __ENV), {
  scenarios: { chat: { executor: 'per-vu-iterations', vus: VUS, iterations: 1, maxDuration: dur('40m', __ENV) } },
});

export default function () {
  const s = vuSession(BASE, bots, __ENV.EW_LOADBOT_PASSWORD);
  const channel = s && textChannel(BASE, s, __ENV);
  const url = channel && ticket(BASE, s);
  if (!url) return;
  const sent = {};
  const res = ws.connect(url, wsParams, (socket) => {
    socket.on('message', (raw) => {
      const m = JSON.parse(raw);
      if (m.ref && sent[m.ref]) {
        if (m.type === 'system.command.acked') ackTime.add(Date.now() - sent[m.ref]);
        else if (m.type === 'system.command.failed') failed.add(1, { code: (m.payload && m.payload.code) || 'unknown' });
        delete sent[m.ref];
      }
    });
    socket.setInterval(() => {
      const [id, text] = envelope('chat.message.send', { channel_id: channel, client_message_id: uuidv4(), kind: 'text',
        body: PHRASES[Math.floor(Math.random() * PHRASES.length)] });
      sent[id] = Date.now();
      socket.send(text);
    }, INTERVAL_MS);
    socket.setInterval(() => socket.send(envelope('system.heartbeat.ping', { client_time: Date.now() })[1]), 15000);
    socket.setTimeout(() => socket.close(), HOLD_MS);
  });
  check(res, { 'websocket upgraded': (r) => r && r.status === 101 });
}
