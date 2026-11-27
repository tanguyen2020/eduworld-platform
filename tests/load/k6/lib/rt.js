// Realtime helpers of the k6 chat scenarios (specs/asyncapi/eduworld-realtime.v1.yaml): a single-use ticket from
// createRealtimeTicket, the WebSocket on ?ticket= with subprotocol eduworld.v1, client envelopes {v, type, id, ts,
// payload}. Message bodies are never logged.
import { call, uuidv7 } from './auth.js';

/** Ticket URL for this session (createRealtimeTicket); null on failure. */
export function ticket(base, s) {
  const r = call('POST', `${base}/game/sessions/${s.gameSessionId}/realtime-tickets`, s, 'session', 'createRealtimeTicket', undefined, { idem: false });
  if (r.status !== 201) return null;
  const sep = r.json('url').includes('?') ? '&' : '?';
  return `${r.json('url')}${sep}ticket=${encodeURIComponent(r.json('ticket'))}`;
}

/** WebSocket params of the gateway. */
export const wsParams = { headers: { 'Sec-WebSocket-Protocol': 'eduworld.v1' }, tags: { kind: 'ws', name: 'realtime' } };

/** A client envelope; returns [id, text]. */
export function envelope(type, payload) {
  const id = uuidv7();
  return [id, JSON.stringify({ v: 1, type, id, ts: Date.now(), payload })];
}

/** First channel of listMyChannels allowing text, or the CHANNEL_ID override. */
export function textChannel(base, s, env) {
  if (env.CHANNEL_ID) return env.CHANNEL_ID;
  const r = call('GET', `${base}/chat/channels`, s, 'read', 'listMyChannels');
  if (r.status !== 200) return null;
  const ch = (r.json('data') || []).find((c) => (c.sendable_kinds || []).includes('text'));
  return ch ? ch.channel_id : null;
}
