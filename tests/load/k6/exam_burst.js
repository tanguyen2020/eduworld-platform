// TC-PERF-005 — simultaneous exam submit (F132-AC-04): one grade of 500 students (EXAM_STUDENTS) starts the exam
// session EXAM_SESSION_ID, answers, then everybody submits inside a 60 s window. No paper lost (every submit gets 200
// or a replay of it), no duplicate (a retry with the same Idempotency-Key replays the same attempt), p95 < 1 s.
import { check, sleep } from 'k6';
import { SharedArray } from 'k6/data';
import { Counter } from 'k6/metrics';
import { call, uuidv7, vuSession } from './lib/auth.js';
import { baseOptions, checkEnv, dur, scaled } from './lib/thresholds.js';

const gen = JSON.parse(open('./scenarios.gen.json'));
checkEnv(__ENV);
const BASE = __ENV.BASE_URL;
const bots = new SharedArray('bots', () => JSON.parse(open(__ENV.BOTS_FILE)));
const STUDENTS = scaled(Number(__ENV.EXAM_STUDENTS || 500), __ENV);
const lost = new Counter('eduworld_exam_submit_lost');
const duplicate = new Counter('eduworld_exam_submit_duplicate');
const WINDOW_S = parseInt(dur('60s', __ENV), 10);

export const options = Object.assign(baseOptions(gen, 'exam_burst', __ENV), {
  scenarios: { burst: { executor: 'per-vu-iterations', vus: STUDENTS, iterations: 1, maxDuration: dur('10m', __ENV) } },
});

export default function () {
  const s = vuSession(BASE, bots, __ENV.EW_LOADBOT_PASSWORD);
  if (!s || !__ENV.EXAM_SESSION_ID) return;
  const st = call('POST', `${BASE}/exams/sessions/${__ENV.EXAM_SESSION_ID}/attempts`, s, 'write', 'startExamAttempt', {});
  if (!check(st, { 'exam started': (r) => r.status === 201 })) return;
  const id = st.json('attempt_id');
  for (const it of st.json('items') || []) {
    const opts = (it.render_payload && it.render_payload.options) || [];
    if (!opts.length) continue;
    call('PUT', `${BASE}/exams/attempts/${id}/answers/${it.position}`, s, 'write', 'saveExamAnswer',
      { position: it.position, question_version_id: it.question_version_id, response: { option_id: opts[0].option_id || opts[0].id } }, { idem: false });
  }
  sleep(Math.random() * WINDOW_S); // everybody submits inside the window
  const key = uuidv7();
  const first = call('POST', `${BASE}/exams/attempts/${id}:submit`, s, 'write', 'submitExamAttempt', undefined, { idem: key });
  if (first.status !== 200) lost.add(1);
  // A client retry after a lost answer: same key must replay the same result, never a second submission.
  const replay = call('POST', `${BASE}/exams/attempts/${id}:submit`, s, 'write', 'submitExamAttemptReplay', undefined, { idem: key });
  if (replay.status !== 200 || replay.json('attempt_id') !== first.json('attempt_id') || replay.json('submitted_at') !== first.json('submitted_at')) duplicate.add(1);
}
