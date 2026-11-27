// REST share of one game session (TEST_STRATEGY §8.1) used by baseline_api, soak, stress and ccu_10k: bootstrap
// reads (profile, manifests, channels, leaderboard) and, when ACTIVITY_ID is set, a practice attempt answered and
// submitted with an Idempotency-Key. The server scores; the script never sends a score (ADR-0016).
import { check, sleep } from 'k6';
import { call } from './auth.js';

/** Bootstrap reads of a session (kind=read). */
export function bootstrap(base, s, env) {
  call('GET', `${base}/game/profile`, s, 'read', 'getGameProfile');
  call('GET', `${base}/game/content-manifest`, s, 'read', 'getContentManifest');
  call('GET', `${base}/game/world/manifest`, s, 'read', 'getWorldManifest');
  call('GET', `${base}/chat/channels`, s, 'read', 'listMyChannels');
  if (env.LEADERBOARD_ID) call('GET', `${base}/leaderboards/${env.LEADERBOARD_ID}?limit=20`, s, 'read', 'getLeaderboard');
}

/** One practice attempt: start, answer every choice item, submit (kind=write). Returns true when submit is 2xx. */
export function practiceAttempt(base, s, env, thinkS = 1) {
  if (!env.ACTIVITY_ID) return false;
  const st = call('POST', `${base}/learning/activities/${env.ACTIVITY_ID}/attempts`, s, 'write', 'startActivityAttempt', { mode: 'practice' });
  if (!check(st, { 'attempt started': (r) => r.status === 201 || r.status === 200 })) return false;
  const id = st.json('attempt_id');
  for (const it of st.json('items') || []) {
    const opts = (it.render_payload && it.render_payload.options) || [];
    if (!opts.length) continue;
    sleep(thinkS);
    call('POST', `${base}/learning/activity-attempts/${id}/responses`, s, 'write', 'submitActivityResponse',
      { kind: 'question', position: it.position, question_version_id: it.question_version_id, response: { option_id: opts[0].option_id || opts[0].id } });
  }
  const sub = call('POST', `${base}/learning/activity-attempts/${id}:submit`, s, 'write', 'submitActivityAttempt');
  return check(sub, { 'attempt submitted': (r) => r.status === 200 });
}

/** A whole REST session iteration with think times scaled by THINK_SCALE (1 = model of §8.1). */
export function restIteration(base, s, env) {
  const k = Number(env.THINK_SCALE || 1);
  bootstrap(base, s, env);
  sleep(5 * k);
  if (Math.random() < 0.5) practiceAttempt(base, s, env, 2 * k);
  sleep(10 * k);
}
