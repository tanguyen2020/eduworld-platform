// TC-PERF-011 — chat burst (F132-AC-04): 200 hot channels × 20 messages/s for 5 min. CHANNELS_FILE lists the hot
// channels as [{channel_id, members:[username…]}] (party/match channels of perf-* bots, prepared by the run);
// SENDERS_PER_CHANNEL members each send RATE/SENDERS messages per second so no single player crosses the chat rate
// limit. Every VU watches chat.message.created of its channel: a seq seen twice is a duplicate, an acked message never
// seen created is lost (both must be 0). NATS/Valkey CPU < 70% is a Prometheus check.
import ws from 'k6/ws';
import { check } from 'k6';
import { SharedArray } from 'k6/data';
import { Counter } from 'k6/metrics';
import { login, uuidv4 } from './lib/auth.js';
import { envelope, ticket, wsParams } from './lib/rt.js';
import { baseOptions, checkEnv, dur, scaled } from './lib/thresholds.js';

const gen = JSON.parse(open('./scenarios.gen.json'));
checkEnv(__ENV);
const BASE = __ENV.BASE_URL;
const channels = new SharedArray('channels', () => JSON.parse(open(__ENV.CHANNELS_FILE)));
const bots = new SharedArray('bots', () => JSON.parse(open(__ENV.BOTS_FILE)));
const CHANNELS = Math.min(channels.length, scaled(200, __ENV));
const SENDERS = Number(__ENV.SENDERS_PER_CHANNEL || 20);
const RATE = Number(__ENV.RATE_PER_CHANNEL || 20); // messages/s per channel (TC-PERF-011)
const HOLD_MS = parseInt(dur('5m', __ENV), 10) * 1000;
const dupSeq = new Counter('eduworld_chat_seq_duplicate');
const lost = new Counter('eduworld_chat_lost');

export const options = Object.assign(baseOptions(gen, 'chat_burst', __ENV), {
  scenarios: { burst: { executor: 'per-vu-iterations', vus: CHANNELS * SENDERS, iterations: 1, maxDuration: dur('15m', __ENV) } },
});

export default function () {
  const ch = channels[Math.floor((__VU - 1) / SENDERS) % CHANNELS];
  const username = ch.members[((__VU - 1) % SENDERS) % ch.members.length];
  const bot = bots.find((b) => b.username === username) || { username, device_id: uuidv4() };
  const s = login(BASE, bot, __ENV.EW_LOADBOT_PASSWORD);
  const url = s && ticket(BASE, s);
  if (!url) return;
  const seen = {};
  const acked = {};
  const res = ws.connect(url, wsParams, (socket) => {
    socket.on('message', (raw) => {
      const m = JSON.parse(raw);
      const p = m.payload || {};
      if (m.type === 'chat.message.created' && p.channel_id === ch.channel_id) {
        if (seen[p.seq]) dupSeq.add(1);
        seen[p.seq] = true;
        delete acked[p.message_id];
      } else if (m.type === 'system.command.acked' && p.result && p.result.message_id) {
        acked[p.result.message_id] = true;
      }
    });
    socket.setInterval(() => {
      socket.send(envelope('chat.message.send', { channel_id: ch.channel_id, client_message_id: uuidv4(), kind: 'text', body: 'Cố lên nào' })[1]);
    }, Math.max(50, Math.round((1000 * SENDERS) / RATE)));
    socket.setInterval(() => socket.send(envelope('system.heartbeat.ping', { client_time: Date.now() })[1]), 15000);
    socket.setTimeout(() => {
      lost.add(Object.keys(acked).length); // acked by JetStream but never delivered back on the channel
      socket.close();
    }, HOLD_MS + 5000);
  });
  check(res, { 'websocket upgraded': (r) => r && r.status === 101 });
}
