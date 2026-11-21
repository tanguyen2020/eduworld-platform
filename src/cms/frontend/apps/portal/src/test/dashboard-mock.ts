// MOCK (test only, AGENTS R17): in-memory stand-in of the learning-module dashboard and report operations used by the
// F056 screens — getSchoolDashboard, getStudentLearningProfile, getRegulationReferenceReport, getExamSessionReport, listSchoolAcademicYears (module identity), listSchoolAcademicYears (module identity), createLmsReport/getLmsReport, getTenantLearningSummary,
// createTenantReport/getTenantReport, getPlatformLearningSummary, createPlatformReport/getPlatformReport and
// listSubjects — answering per specs/openapi/eduworld-api.v1.yaml (golden data of TH Demo A). Tenant isolation is
// simulated: another tenant's X-Tenant-Id gets 404 (F056-AC-21); a forbidden subject gets 403 PERMISSION_DENIED.
// The school fixture still carries the class fields the contract keeps until WP-L2 (`classes`, `by_class`, `class_ids`,
// attention `class_id`) so the tests prove the portal never shows them (ADR-0095). Exports need an Idempotency-Key and go queued → running → succeeded with a signed link. Per-test behaviours go
// through `state.next` (queued Problem answers per operation). Never imported by application code; NOT a backend.
import { problem, TENANTS, type MockPortalBackend, type RecordedRequest } from './mock-backend';
import { LMS_IDS } from './lms-mock';
import { IDS } from './school-lms-mock';

const json = (status: number, body: unknown, headers: Record<string, string> = {}) =>
  new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json', ...headers } });

const T_A = TENANTS.thDemoA.tenant_id;

export const DASH_IDS = {
  loChiaDu: '0193a4d2-ab00-7000-8000-000000000001',
  loNhan7: '0193a4d2-ab00-7000-8000-000000000002',
  assignmentGraded: '0193a4d2-ac00-7000-8000-000000000001',
  assignmentPractice: '0193a4d2-ac00-7000-8000-000000000002',
  topicNhanChia: '0193a4d2-ad00-7000-8000-000000000001',
  question1: '0193a4d2-ae00-7000-8000-000000000001',
  question2: '0193a4d2-ae00-7000-8000-000000000002',
} as const;

export const AS_OF = '2026-11-21T03:05:00Z';

type Row = Record<string, unknown>;

export function schoolDashboardFixture(): Row {
  return {
    school_id: IDS.schoolA,
    data_as_of: AS_OF,
    grain: 'day',
    is_partial: true,
    widgets: {
      assignments_open: 2,
      assignments_due_soon: 1,
      assignment_completion_rate: 0.6,
      assignments: [{ assignment_id: DASH_IDS.assignmentGraded, title: 'Bảng nhân 6–9', due_at: '2026-11-22T10:00:00Z', status: 'open', submitted: 2, total: 3, missed: 0, not_started: 1, by_class: [{ class_id: IDS.class3A, submitted: 2, total: 3 }] }],
      classes: [{ class_id: IDS.class3A, name: '3A', grade_level: 3, assignment_completion_rate: 0.67, accuracy: 0.7, active_students: 3 }],
      weak_objectives: [{ objective_id: DASH_IDS.loChiaDu, code: 'TOAN.3.CHIA_DU', yccd_ref: 'YCCĐ 3.12', below_proficient_ratio: 0.55, class_ids: [IDS.class3A] }],
      students_needing_attention: [
        { student_id: IDS.studentAn, display_name: 'Nguyễn An', class_id: IDS.class3A, reasons: ['rapid_guess'] },
        { student_id: LMS_IDS.studentBinh, display_name: 'Lê Bình', class_id: IDS.class3A, reasons: ['missed_assignments', 'inactive_game'] },
      ],
      students_needing_attention_total: 2,
      self_study: { avg_active_minutes_per_student: 42, recommendation_started_rate: 0.68, objectives_improved_after_recommendation: 37 },
      active_minutes_series: [
        { period_start: '2026-11-20', value: 40, is_final: true },
        { period_start: '2026-11-21', value: null, is_final: false },
      ],
      safety_notifications_unacknowledged: 1,
    },
  };
}

function studentProfile(): Row {
  return {
    student_id: IDS.studentAn,
    school_id: IDS.schoolA,
    data_as_of: AS_OF,
    grain: 'week',
    is_partial: false,
    subjects: [
      { subject_id: IDS.subjectMath, accuracy: 0.8, active_minutes: 120, activities_completed: 9, lessons_completed: 3, mastery_by_topic: [{ topic_id: DASH_IDS.topicNhanChia, topic_name: 'Nhân chia trong bảng', level_dist: [0, 1, 2, 3, 1] }] },
    ],
    trend_weeks: [
      { subject_id: IDS.subjectMath, week_start: '2026-11-09', mastered_count: 1, mean_mastery: 0.5 },
      { subject_id: IDS.subjectMath, week_start: '2026-11-16', mastered_count: 2, mean_mastery: 0.6 },
    ],
    recent_attempts: [{ assignment_id: DASH_IDS.assignmentGraded, title: 'Bảng nhân 6–9', status: 'submitted', score: 8, max_score: 10, subject_id: IDS.subjectMath, submitted_at: '2026-11-20T03:00:00Z' }],
    active_recommendations: [{ recommendation_id: '0193a4d2-af00-7000-8000-000000000001', rule_code: 'R2', reason_code: 'weak_objective', subject_id: IDS.subjectMath }],
  };
}

function examReport(): Row {
  return {
    exam_session_id: LMS_IDS.examSession,
    status: 'closed',
    is_official: true,
    computed: true,
    computed_at: AS_OF,
    student_count: 40,
    mean: 6.4,
    median: 6.5,
    pass_rate: 0.75,
    score_distribution: [0, 0, 1, 2, 3, 5, 8, 9, 6, 4, 2, 0, 0, 0, 0, 0, 0, 0, 0, 0],
    reliability_warning: true,
    variant_gap: false,
    variants: [{ variant_no: 1, n: 40, mean: 6.4, sd: 1.2, enough_data: true, method: 'kr20', reliability: 0.55, reliability_warning: true, sem: 0.8 }],
    items: [
      { question_version_id: DASH_IDS.question1, n: 40, p_value: 0.42, rapid_guess_rate: 0.1, discrimination: 0.3, avg_time_ms: 32000, problem: true, top_misconceptions: [{ tag: 'nhan_nham_bang_7', count: 9 }] },
      { question_version_id: DASH_IDS.question2, n: 40, p_value: 0.88, rapid_guess_rate: 0.02, avg_time_ms: 15000, problem: false, top_misconceptions: [] },
    ],
    problem_items: [],
  };
}

function summary(platform: boolean): Row {
  const cell = (school: string | undefined, tenant: string | undefined, grade: number, suppressed: boolean) => ({
    school_id: school,
    tenant_id: tenant,
    grade_level: grade,
    subject_id: IDS.subjectMath,
    suppressed,
    students: suppressed ? null : 32,
    active_students: suppressed ? null : 21,
    accuracy: suppressed ? null : 0.71,
    active_minutes_avg: suppressed ? null : 38,
    attempts_completed: suppressed ? null : 410,
  });
  return {
    data_as_of: AS_OF,
    grain: 'week',
    is_partial: false,
    active_students: 21,
    accuracy: 0.71,
    active_minutes_avg: 38,
    assignment_completion_rate: 0.8,
    attempts_completed: 410,
    cells: platform ? [cell(undefined, T_A, 3, false), cell(undefined, TENANTS.thcsDemoB.tenant_id, 6, true)] : [cell(IDS.schoolA, undefined, 3, false), cell(IDS.schoolA, undefined, 4, true)],
    ...(platform ? { by_tenant: [{ tenant_id: T_A, active_students: 21, accuracy: 0.71 }, { tenant_id: TENANTS.thcsDemoB.tenant_id, active_students: null, accuracy: null }] } : {}),
    suppressed_cells: 1,
  };
}

export interface DashboardMockState {
  /** Queued answers per operation (first = next call), e.g. `next.school = [problem(504,'DASHBOARD_QUERY_TIMEOUT')]`. */
  next: Record<string, Response[]>;
  school: () => Row;
  /** Subjects outside the caller's scope → 403 PERMISSION_DENIED (academic manager by subject). */
  forbiddenSubjects: Set<string>;
  jobs: Map<string, Row & { polls: number; kind: string }>;
  /** Export limit reached for the next create calls (429 EXPORT_LIMIT_REACHED). */
  exportLimit: boolean;
  /** Seconds before a succeeded link expires (relative to now). */
  linkTtlSeconds: number;
  idem: Map<string, Row>;
  /** listSchoolAcademicYears of school A (module identity, F010 T047). */
  academicYears: () => Row[];
}

/** Current school year around today and the previous one; `terms` empty as the real backend today (OQ F010-4). */
export function academicYearsFixture(): Row[] {
  const d = (offset: number) => new Date(Date.now() + offset * 86_400_000).toISOString().slice(0, 10);
  return [
    { academic_year_id: '0193a4d2-af00-7000-8000-000000000001', name: 'Năm hiện tại', start_date: d(-60), end_date: d(200), is_current: true, terms: [] },
    { academic_year_id: '0193a4d2-af00-7000-8000-000000000002', name: 'Năm trước', start_date: d(-430), end_date: d(-70), is_current: false, terms: [] },
  ];
}

let seq = 0;

/** Installs the MOCK dashboard handlers (first override) and returns their state. */
export function installDashboardMock(backend: MockPortalBackend): DashboardMockState {
  const state: DashboardMockState = { next: {}, school: schoolDashboardFixture, forbiddenSubjects: new Set(), jobs: new Map(), exportLimit: false, linkTtlSeconds: 900, idem: new Map(), academicYears: academicYearsFixture };
  const tenantOf = (r: RecordedRequest) => r.headers['x-tenant-id'];
  const q = (r: RecordedRequest) => new URLSearchParams(r.search);
  const queued = (op: string) => state.next[op]?.shift();
  const jobView = (j: Row & { polls: number; kind: string }) => Object.fromEntries(Object.entries(j).filter(([k]) => k !== 'polls' && k !== 'kind'));
  const create = (r: RecordedRequest, kind: string) => {
    const key = r.headers['idempotency-key'];
    if (!key) return problem(400, 'IDEMPOTENCY_KEY_REQUIRED');
    if (state.exportLimit) return new Response(JSON.stringify({ type: 'about:blank', title: 'x', status: 429, code: 'EXPORT_LIMIT_REACHED', trace_id: 'trace-export' }), { status: 429, headers: { 'Content-Type': 'application/problem+json', 'Retry-After': '0' } });
    const prev = state.idem.get(`${kind}|${key}`);
    if (prev) return json(202, prev, { 'Idempotent-Replayed': 'true' });
    const body = (r.body ?? {}) as Row;
    const job = { report_id: `0193a4d2-b100-7000-8000-${String(++seq).padStart(12, '0')}`, status: 'queued', report_type: String(body.report_type), format: String(body.format), created_at: AS_OF, polls: 0, kind };
    state.jobs.set(job.report_id, job);
    const view = jobView(job);
    state.idem.set(`${kind}|${key}`, view);
    return json(202, view);
  };
  const poll = (id: string, kind: string) => {
    const j = state.jobs.get(id);
    if (!j || j.kind !== kind) return problem(404, 'NOT_FOUND');
    j.polls++;
    if (j.polls === 1) j.status = 'running';
    else if (j.status === 'running') {
      j.status = 'succeeded';
      j.download_url = `https://exports.test/${id}.${j.format}?sig=mock`;
      j.download_expires_at = new Date(Date.now() + state.linkTtlSeconds * 1000).toISOString();
      j.row_count = 2;
      j.finished_at = AS_OF;
    }
    return json(200, jobView(j));
  };

  backend.overrides.unshift((r) => {
    const p = r.path;
    const m = r.method;
    let x: RegExpExecArray | null;
    const tenantRoute = /^\/(lms|exams|tenant\/analytics|tenant\/reports|learning\/subjects)/.test(p);
    if (tenantRoute && tenantOf(r) !== T_A) return problem(404, 'NOT_FOUND');
    if (m === 'GET' && p === '/learning/subjects') {
      return json(200, {
        items: [
          { subject_id: IDS.subjectMath, code: 'math', name: { vi: 'Toán', en: 'Math' } },
          { subject_id: LMS_IDS.subjectViet, code: 'vietnamese', name: { vi: 'Tiếng Việt', en: 'Vietnamese' } },
        ],
      });
    }
    if ((x = /^\/schools\/([^/]+)\/academic-years$/.exec(p)) && m === 'GET') {
      if (tenantOf(r) !== T_A || x[1] !== IDS.schoolA) return problem(404, 'NOT_FOUND');
      return json(200, { data: state.academicYears() });
    }
    if (m === 'GET' && p === '/lms/dashboards/school') {
      const n = queued('school');
      if (n) return n;
      if (q(r).get('school_id') !== IDS.schoolA) return problem(404, 'NOT_FOUND');
      if (state.forbiddenSubjects.has(q(r).get('subject_id') ?? '')) return problem(403, 'PERMISSION_DENIED');
      return json(200, { ...state.school(), from: q(r).get('from'), to: q(r).get('to') });
    }
    if ((x = /^\/lms\/students\/([^/]+)\/analytics$/.exec(p)) && m === 'GET') {
      const n = queued('student');
      if (n) return n;
      return x[1] === IDS.studentAn ? json(200, studentProfile()) : problem(404, 'NOT_FOUND');
    }
    if ((x = /^\/lms\/schools\/([^/]+)\/reference-levels$/.exec(p)) && m === 'GET') {
      if (x[1] !== IDS.schoolA) return problem(404, 'NOT_FOUND');
      return json(200, {
        school_id: IDS.schoolA,
        subject_id: q(r).get('subject_id'),
        period: q(r).get('period'),
        academic_year: 2026,
        disclaimer: 'Mức tham khảo suy ra từ kết quả tự chấm theo hồ sơ đánh giá.',
        items: [
          { student_id: IDS.studentAn, class_id: IDS.class3A, level_code: 'completed', coverage_ratio: 0.8, rapid_guess_ratio: 0.1, profile_version: 1 },
          { student_id: IDS.studentSapBiDon, class_id: IDS.class3A, level_code: 'insufficient_data', coverage_ratio: 0.1, rapid_guess_ratio: 0, profile_version: 1, low_reliability_exam_ids: [LMS_IDS.examSession] },
        ],
        page: { has_more: false, limit: 100, next_cursor: null },
      });
    }
    if ((x = /^\/exams\/sessions\/([^/]+)\/report$/.exec(p)) && m === 'GET') {
      return x[1] === LMS_IDS.examSession ? json(200, examReport()) : problem(404, 'NOT_FOUND');
    }
    if (m === 'POST' && p === '/lms/reports') return create(r, 'lms');
    if ((x = /^\/lms\/reports\/([^/]+)$/.exec(p)) && m === 'GET') return poll(x[1]!, 'lms');
    if (m === 'GET' && p === '/tenant/analytics/learning/summary') return queued('tenantSummary') ?? json(200, summary(false));
    if (m === 'POST' && p === '/tenant/reports') return create(r, 'tenant');
    if ((x = /^\/tenant\/reports\/([^/]+)$/.exec(p)) && m === 'GET') return poll(x[1]!, 'tenant');
    if (m === 'GET' && p === '/admin/analytics/learning/summary') return json(200, summary(true));
    if (m === 'POST' && p === '/admin/reports') return create(r, 'platform');
    if ((x = /^\/admin\/reports\/([^/]+)$/.exec(p)) && m === 'GET') return poll(x[1]!, 'platform');
    if (m === 'GET' && p === '/cms/learning/subjects') return json(200, { data: [{ subject_id: IDS.subjectMath, code: 'math', name: { vi: 'Toán' } }], page: { has_more: false, limit: 100, next_cursor: null } });
    return undefined;
  });
  return state;
}
