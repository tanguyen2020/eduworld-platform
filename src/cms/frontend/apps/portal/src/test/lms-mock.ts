// MOCK (test only, AGENTS R17): in-memory stand-in of the learning-module LMS operations used by the F055 screens —
// assignments (create/publish/update/cancel/release/archive/remind, extend/excuse, submissions; targets by grade of a
// school ± students or a student list — ADR-0095), the content picker lists, the student search of the wizard (interim
// listTenantGradeStudents of the F052 MOCK), exam session / papers / accommodations and regrade — answering per
// specs/openapi/eduworld-api.v1.yaml. Writes check Idempotency-Key (same key → same answer, no duplicate)
// and If-Match like the backend (W/"n"). Data belongs to TH Demo A; another tenant's X-Tenant-Id gets 404.
// Never imported by application code; NOT a real backend and no live integration.
import { problem, TENANTS, type MockPortalBackend, type RecordedRequest } from './mock-backend';
import { IDS } from './school-lms-mock';

const json = (status: number, body: unknown, headers: Record<string, string> = {}) =>
  new Response(status === 204 ? null : JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json', ...headers } });
const etag = (v: number) => ({ ETag: `W/"${v}"` });
const page = (data: unknown[]) => ({ data, page: { has_more: false, limit: 50, next_cursor: null } });

export const LMS_IDS = {
  subjectViet: '0193a4d2-9000-7000-8000-000000000002',
  activityQuiz: '0193a4d2-b000-7000-8000-000000000001',
  activityQuizV: '0193a4d2-b000-7000-8000-0000000000f1',
  activityDraft: '0193a4d2-b000-7000-8000-000000000002',
  activityDisabledType: '0193a4d2-b000-7000-8000-000000000003',
  questionSet: '0193a4d2-b000-7000-8000-000000000004',
  questionSetV: '0193a4d2-b000-7000-8000-0000000000f4',
  quest: '0193a4d2-b000-7000-8000-000000000005',
  questV: '0193a4d2-b000-7000-8000-0000000000f5',
  studentBinh: '0193a4d2-4000-7000-8000-0000000000b2',
  studentCuong4: '0193a4d2-4000-7000-8000-0000000000c4',
  examSession: '0193a4d2-d000-7000-8000-000000000001',
  questionV1: '0193a4d2-d000-7000-8000-0000000000a1',
} as const;

const T_A = TENANTS.thDemoA.tenant_id;
/** A student of another school: returned by the MOCK search so a test can pick them; createAssignment answers 422. */
export const OTHER_SCHOOL_STUDENT = '0193a4d2-4000-7000-8000-0000000000f9';

type Row = Record<string, unknown>;

export interface LmsMockState {
  assignments: Map<string, Row & { version: number }>;
  submissions: Row[];
  session: Row & { version: number };
  /** Idempotency-Key → recorded response (status + body). */
  idem: Map<string, { status: number; body: unknown; headers?: Record<string, string> }>;
  remindCount: number;
  regradeStatus: string;
}

let seq = 0;
const newId = () => `0193a4d2-9999-7000-8000-${String(++seq).padStart(12, '0')}`;

export function lmsAssignment(over: Row = {}): Row & { version: number } {
  return {
    assignment_id: newId(),
    tenant_id: T_A,
    school_id: IDS.schoolA,
    subject_id: IDS.subjectMath,
    title: 'Ôn tập chương 3',
    content_ref: { content_type: 'activity', content_id: LMS_IDS.activityQuiz, pinned_version_id: LMS_IDS.activityQuizV },
    mode: 'practice',
    targets: [{ target_type: 'grade', grade_level: 3, student_count: 3 }],
    open_at: '2026-11-20T01:00:00Z',
    due_at: '2026-11-27T10:00:00Z',
    status: 'open',
    created_by: IDS.userQlcm,
    result_release: 'after_close',
    completion: { assigned: 3, submitted: 1, missed: 0, excused: 0 },
    version: 1,
    ...over,
  };
}

/** Installs the MOCK LMS handlers (first override) and returns their state for assertions. */
export function installLmsMock(backend: MockPortalBackend): LmsMockState {
  const state: LmsMockState = {
    assignments: new Map(),
    // module learning does not fill AssignmentSubmission.display_name (no PII in its projections): the portal joins the
    // names of listTenantGradeStudents (F055-AC-08, Clarifications 2026-11-28)
    submissions: [
      { assignment_id: '', student_id: IDS.studentAn, status: 'submitted', attempts_used: 1, score: 8, max_score: 10, rapid_guess_count: 2, submitted_at: '2026-11-21T03:00:00Z' },
      { assignment_id: '', student_id: LMS_IDS.studentBinh, status: 'not_started', attempts_used: 0, score: null, max_score: null, rapid_guess_count: 0 },
      { assignment_id: '', student_id: IDS.studentSapBiDon, status: 'missed', attempts_used: 0, score: null, rapid_guess_count: 0 },
    ],
    session: {
      exam_session_id: LMS_IDS.examSession,
      type: 'unit_test',
      is_official: true,
      duration_seconds: 2400,
      status: 'closed',
      accommodations: [],
      version: 1,
    },
    idem: new Map(),
    remindCount: 0,
    regradeStatus: 'queued',
  };
  const tenantOk = (r: RecordedRequest) => r.headers['x-tenant-id'] === T_A || r.headers['x-tenant-id'] === undefined;
  const idemKey = (r: RecordedRequest) => r.headers['idempotency-key'];
  /** Same Idempotency-Key → the first answer again (no second write). */
  const once = (r: RecordedRequest, run: () => { status: number; body: unknown; headers?: Record<string, string> } | Response): Response => {
    const key = idemKey(r);
    if (!key) return problem(400, 'IDEMPOTENCY_KEY_REQUIRED');
    const prev = state.idem.get(`${r.path}|${key}`);
    if (prev) return json(prev.status, prev.body, prev.headers);
    const res = run();
    if (res instanceof Response) return res;
    state.idem.set(`${r.path}|${key}`, res);
    return json(res.status, res.body, res.headers);
  };
  const checkIfMatch = (r: RecordedRequest, version: number): Response | null => {
    const h = r.headers['if-match'];
    if (!h) return problem(428, 'PRECONDITION_REQUIRED');
    return h.replace(/^W\//, '').replace(/"/g, '') === String(version) ? null : problem(412, 'VERSION_MISMATCH');
  };
  /** Assignment / ExamSession views carry no version field (the ETag header does). */
  const view = (a: Row & { version: number }) => Object.fromEntries(Object.entries(a).filter(([k]) => k !== 'version'));

  backend.overrides.unshift((r) => {
    const p = r.path;
    const m = r.method;
    if (!/^\/(lms|exams|cms\/learning\/(activities|activity-types|question-sets|assessment-blueprints|lessons|regrade-requests))/.test(p)) return undefined;
    if (!tenantOk(r)) return problem(404, 'NOT_FOUND');
    const b = (r.body ?? {}) as Row;
    let x: RegExpExecArray | null;
    // ---------- content picker
    if (m === 'GET' && p === '/cms/learning/activity-types') {
      return json(200, {
        data: [
          { activity_type: 'quiz', schema_url: 'x', evaluation_strategies: [], min_client_version: '1.0.0', status: 'enabled' },
          { activity_type: 'simulation', schema_url: 'x', evaluation_strategies: [], min_client_version: '1.0.0', status: 'disabled' },
        ],
      });
    }
    if (m === 'GET' && p === '/cms/learning/activities') {
      return json(
        200,
        page([
          { activity_id: LMS_IDS.activityQuiz, activity_version_id: LMS_IDS.activityQuizV, version: 2, activity_type: 'quiz', title: { vi: 'Quiz phép nhân' }, status: 'published' },
          { activity_id: LMS_IDS.activityDraft, version: 1, activity_type: 'quiz', title: { vi: 'Quiz nháp' }, status: 'draft' },
          { activity_id: LMS_IDS.activityDisabledType, activity_version_id: newId(), version: 1, activity_type: 'simulation', title: { vi: 'Mô phỏng ngoài Release' }, status: 'published' },
        ]),
      );
    }
    if (m === 'GET' && /^\/cms\/learning\/(question-sets|assessment-blueprints|lessons)$/.test(p)) {
      const kind = p.split('/').pop()!;
      const ct = kind === 'question-sets' ? 'question_set' : kind === 'lessons' ? 'lesson' : 'assessment_blueprint';
      return json(200, page([{ content_type: ct, content_id: LMS_IDS.questionSet, content_version_id: LMS_IDS.questionSetV, version: 1, status: 'published', title: { vi: `Bộ ${kind} chương 3` } }]));
    }
    if (m === 'GET' && p === '/lms/catalog/game-activities') {
      return json(200, page([{ content_type: 'quest', content_id: LMS_IDS.quest, content_version_id: LMS_IDS.questV, title: 'Quest Rừng Số' }]));
    }
    // ---------- assignments
    if (p === '/lms/assignments') {
      if (m === 'GET') {
        const q = new URLSearchParams(r.search);
        const grade = Number(q.get('grade_level'));
        const keep = (a: Row) =>
          (!q.get('status') || a.status === q.get('status')) &&
          (!q.get('school_id') || a.school_id === q.get('school_id')) &&
          (!q.get('subject_id') || a.subject_id === q.get('subject_id')) &&
          (!grade || (a.targets as Row[]).some((x) => x.grade_level === grade));
        return json(200, page([...state.assignments.values()].filter(keep).map(view)));
      }
      if (m === 'POST')
        return once(r, () => {
          // MOCK of the ADR-0095 check: a student of another school in a list → 422 at that list
          const bad = ((b.targets ?? []) as Row[]).findIndex((x) => [...((x.include_student_ids ?? []) as string[]), ...((x.student_ids ?? []) as string[])].includes(OTHER_SCHOOL_STUDENT));
          if (bad >= 0) return problem(422, 'VALIDATION_FAILED', { errors: [{ field: `/targets/${bad}/${(b.targets as Row[])[bad]!.target_type === 'grade' ? 'include_student_ids' : 'student_ids'}`, code: 'STUDENT_OUT_OF_SCHOOL' }] });
          const a = lmsAssignment({ ...b, status: 'draft', completion: { assigned: 0, submitted: 0, missed: 0, excused: 0 } });
          state.assignments.set(String(a.assignment_id), a);
          return { status: 201, body: view(a), headers: etag(a.version) };
        });
    }
    if ((x = /^\/lms\/assignments\/([^/:]+)(?::([a-z-]+))?$/.exec(p))) {
      const a = state.assignments.get(x[1]!);
      if (!a) return problem(404, 'NOT_FOUND');
      const op = x[2];
      if (!op && m === 'GET') return json(200, view(a), etag(a.version));
      if (!op && m === 'PATCH') {
        const bad = checkIfMatch(r, a.version);
        if (bad) return bad;
        if (a.status === 'open' && Object.keys(b).some((k) => !['due_at', 'instructions', 'late_policy'].includes(k))) return problem(422, 'ASSIGNMENT_NOT_EDITABLE');
        Object.assign(a, b, { version: a.version + 1 });
        return json(200, view(a), etag(a.version));
      }
      if (op === 'publish')
        return once(r, () => {
          const bad = checkIfMatch(r, a.version);
          if (bad) return bad;
          a.status = 'open';
          a.version++;
          return { status: 200, body: view(a), headers: etag(a.version) };
        });
      if (op === 'cancel')
        return once(r, () => {
          if (a.status === 'open' && ((a.completion as Row).submitted as number) > 0) return problem(409, 'CONFLICT');
          a.status = 'cancelled';
          a.version++;
          return { status: 200, body: view(a) };
        });
      if (op === 'release-results')
        return once(r, () => {
          a.status = 'results_released';
          a.version++;
          return { status: 200, body: view(a) };
        });
      if (op === 'archive')
        return once(r, () => {
          a.status = 'archived';
          return { status: 200, body: view(a) };
        });
      if (op === 'remind') {
        if (!idemKey(r)) return problem(400, 'IDEMPOTENCY_KEY_REQUIRED');
        state.remindCount++;
        if (state.remindCount > 1) return new Response(JSON.stringify({ type: 'about:blank', title: 'RATE_LIMITED', status: 429, code: 'RATE_LIMITED', trace_id: 'trace-remind' }), { status: 429, headers: { 'Content-Type': 'application/problem+json', 'Retry-After': '3600' } });
        return json(202, { recipients: 1 });
      }
    }
    if ((x = /^\/lms\/assignments\/([^/]+)\/submissions$/.exec(p)) && m === 'GET') {
      const status = new URLSearchParams(r.search).get('status');
      return json(200, page(state.submissions.filter((s) => !status || s.status === status).map((s) => ({ ...s, assignment_id: x![1] }))));
    }
    if ((x = /^\/lms\/assignments\/([^/]+)\/students\/([^/]+):(extend|excuse)$/.exec(p)) && m === 'POST') {
      return once(r, () => {
        const s = state.submissions.find((k) => k.student_id === x![2]);
        if (!s) return problem(404, 'NOT_FOUND');
        if (x![3] === 'extend') s.due_at_override = b.due_at;
        else s.status = 'excused';
        return { status: 200, body: { ...s, assignment_id: x![1] } };
      });
    }
    // ---------- exams & regrade
    if ((x = /^\/exams\/sessions\/([^/]+)$/.exec(p)) && m === 'GET') {
      if (x[1] !== LMS_IDS.examSession) return problem(404, 'NOT_FOUND');
      return json(200, view(state.session), etag(state.session.version));
    }
    if ((x = /^\/exams\/sessions\/([^/]+)\/papers$/.exec(p)) && m === 'GET') {
      return json(200, { data: [{ exam_paper_id: newId(), variant_no: 1, sections: [{ section_id: 's1', items: [{ position: 1, question_version_id: LMS_IDS.questionV1, points: 1 }] }] }] });
    }
    if ((x = /^\/exams\/sessions\/([^/]+)\/accommodations$/.exec(p)) && m === 'PUT') {
      return once(r, () => {
        const bad = checkIfMatch(r, state.session.version);
        if (bad) return bad;
        state.session.accommodations = b.accommodations;
        state.session.version++;
        return { status: 200, body: view(state.session), headers: etag(state.session.version) };
      });
    }
    if (m === 'POST' && p === '/cms/learning/regrade-requests:preview') return json(200, { affected_attempts: 12 });
    if (m === 'POST' && p === '/cms/learning/regrade-requests')
      return once(r, () => ({ status: 202, body: { regrade_request_id: LMS_IDS.questionV1, scope: 'exam_session', action: b.action, status: 'queued', requested_by: IDS.userQlcm, created_at: '2026-11-21T03:00:00Z', affected_attempts: 12 } }));
    if (m === 'GET' && /^\/cms\/learning\/regrade-requests\/[^/]+$/.test(p)) {
      return json(200, { regrade_request_id: LMS_IDS.questionV1, scope: 'exam_session', action: 'accept_all', status: state.regradeStatus, requested_by: IDS.userQlcm, created_at: '2026-11-21T03:00:00Z', affected_attempts: 12, re_evaluated_attempts: 12 });
    }
    return undefined;
  });
  return state;
}
