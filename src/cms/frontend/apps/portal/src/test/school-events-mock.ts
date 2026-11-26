// MOCK (test only, AGENTS R17): in-memory stand-in of the school event operations of specs/openapi/eduworld-api.v1.yaml
// used by the F081 screens — listEventTemplates, listSchoolEvents (session tenant or System Admin tenant_ids),
// createSchoolEvent, getSchoolEvent (ETag), updateSchoolEvent (If-Match), cancelSchoolEvent, getSchoolEventProgress —
// plugged into MockPortalBackend as an override. It records requests, keeps a little state and simulates the answers the
// screens must handle (404 of another tenant, 409, 412, 422 with errors[].field, 503); it enforces NO authorization and is
// NOT a backend (the real rules live in module game, F077/F081). Sensitive-word fixtures are placeholders only (R15).
import { IDS as LMS_IDS } from './school-lms-mock';
import { cursorPage, problem, TENANTS, type MockPortalBackend, type RecordedRequest } from './mock-backend';

const json = (status: number, body: unknown, headers: Record<string, string> = {}) =>
  new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json', ...headers } });

export const T_A = TENANTS.thDemoA.tenant_id;
export const T_B = TENANTS.thcsDemoB.tenant_id;

/** Placeholder the MOCK flags as a sensitive word (never a real word — R15). */
export const FLAGGED = 'zqflag';

export const EV = {
  template: '0193a4d2-ab00-7000-8000-000000000001',
  templateB: '0193a4d2-ab00-7000-8000-000000000002',
  quest1: '0193a4d2-ab10-7000-8000-000000000001',
  quest2: '0193a4d2-ab10-7000-8000-000000000002',
  eventScheduled: '0193a4d2-ac00-7000-8000-000000000001',
  eventLive: '0193a4d2-ac00-7000-8000-000000000002',
  eventSuspended: '0193a4d2-ac00-7000-8000-000000000003',
  eventB: '0193a4d2-ac00-7000-8000-0000000000b1',
  goal: '0193a4d2-ad00-7000-8000-000000000001',
  schoolA: LMS_IDS.schoolA,
} as const;

const iso = (days: number) => new Date(Date.now() + days * 86_400_000).toISOString();

export function mathWeekTemplate() {
  return {
    template_id: EV.template,
    code: 'template.math_week',
    name: { vi: 'Tuần lễ Toán học', en: 'Maths Week' },
    description: { vi: 'Cả trường cùng giải toán', en: 'The whole school solves maths' },
    event_kind: 'school',
    allowed_params: ['description', 'audience.grade_levels', 'community_goal_target', 'quest_ids', 'reward_option_id'],
    max_duration_days: 14,
    quest_options: [EV.quest1, EV.quest2],
    challenge_options: [],
    quest_option_items: [
      { id: EV.quest1, title: { vi: 'Bài toán phiên chợ', en: 'Market problems' }, grade_levels: [3] },
      { id: EV.quest2, title: { vi: 'Đếm sao', en: 'Count the stars' }, grade_levels: [4, 5] },
    ],
    challenge_option_items: [],
    reward_options: [{ reward_option_id: 'school_badge', label: { vi: 'Huy hiệu trường', en: 'School badge' } }],
  };
}

const event = (id: string, tenant: string, status: string, vi: string, extra: Record<string, unknown> = {}) => ({
  event_id: id,
  tenant_id: tenant,
  template_id: EV.template,
  name: { vi },
  starts_at: status === 'scheduled' ? iso(2) : iso(-1),
  ends_at: status === 'scheduled' ? iso(9) : iso(6),
  audience: { school_ids: [EV.schoolA], grade_levels: [3, 4] },
  community_goal_target: 300,
  status,
  version: 1,
  created_at: iso(-3),
  ...extra,
});

export function schoolEventsState() {
  return {
    templates: { [T_A]: [mathWeekTemplate()], [T_B]: [] as unknown[] } as Record<string, unknown[]>,
    events: [
      event(EV.eventScheduled, T_A, 'scheduled', 'Tuần lễ Toán học trường A', { created_at: iso(-1) }),
      event(EV.eventLive, T_A, 'live', 'Ngày hội đọc sách', { created_at: iso(-2) }),
      event(EV.eventSuspended, T_A, 'suspended', 'Sự kiện bị tạm dừng', { created_at: iso(-3) }),
      event(EV.eventB, T_B, 'live', 'Sự kiện THCS Demo B', { created_at: iso(-4) }),
    ] as Record<string, unknown>[],
    /** Page size of listSchoolEvents (the screen pages with "Xem thêm"). */
    eventPageSize: 50,
    /** createSchoolEvent answers 503 while true (module chat down). */
    screeningDown: false,
    /** createSchoolEvent answers 409 (limit of concurrent events) while true. */
    tooMany: false,
    progress: {
      event_id: EV.eventLive,
      event_run_id: '0193a4d2-ae00-7000-8000-000000000001',
      status: 'live',
      school_id: EV.schoolA,
      participants_count: 42,
      community_goal: {
        goal_id: EV.goal,
        metric: 'correct_answers',
        target: 300,
        total: 180,
        milestones: [
          { pct: 25, reached: true },
          { pct: 50, reached: true },
          { pct: 75, reached: false },
          { pct: 100, reached: false },
        ],
      },
      // already sorted and suppressed by the server (grades 4–9, ≥ 5 contributors)
      by_grade: [
        { grade_level: 4, contribution_total: 60, contributors_count: 12 },
        { grade_level: 5, contribution_total: 120, contributors_count: 20 },
      ],
      data_as_of: '2026-11-25T03:00:00Z',
    } as Record<string, unknown>,
    created: 0,
  };
}

export type SchoolEventsState = ReturnType<typeof schoolEventsState>;

/** Installs the MOCK school event handlers on a MockPortalBackend (first override). */
export function installSchoolEventsMock(backend: MockPortalBackend, state: SchoolEventsState = schoolEventsState()): SchoolEventsState {
  const tenantOf = (r: RecordedRequest) => r.headers['x-tenant-id'];
  const find = (r: RecordedRequest, id: string) => state.events.find((e) => e.event_id === id && e.tenant_id === tenantOf(r));
  const keys = new Map<string, Record<string, unknown>>();
  backend.overrides.unshift((r) => {
    const q = new URLSearchParams(r.search);
    let x: RegExpExecArray | null;
    if (r.method === 'GET' && r.path === '/lms/event-templates') {
      return json(200, cursorPage(state.templates[tenantOf(r) ?? ''] ?? [], q, 'data', 50));
    }
    if (r.method === 'GET' && r.path === '/lms/school-events') {
      const status = q.get('status');
      const header = tenantOf(r);
      const filter = q.getAll('tenant_ids');
      const rows = state.events.filter((e) => (header ? e.tenant_id === header : !filter.length || filter.includes(String(e.tenant_id))) && (!status || e.status === status));
      return json(200, cursorPage(rows, q, 'data', state.eventPageSize));
    }
    if (r.method === 'POST' && r.path === '/lms/school-events') {
      const key = r.headers['idempotency-key'] ?? '';
      if (keys.has(key)) return json(201, keys.get(key), { 'Idempotent-Replayed': 'true' });
      if (state.screeningDown) return problem(503, 'UNAVAILABLE');
      if (state.tooMany) return problem(409, 'CONFLICT');
      const b = r.body as Record<string, unknown>;
      const name = b.name as { vi?: string };
      if (name?.vi?.includes(FLAGGED)) return problem(422, 'VALIDATION_FAILED', { errors: [{ field: 'name.vi', code: 'invalid', message: 'text not allowed' }] });
      if (Array.isArray(b.challenge_ids)) return problem(422, 'EVENT_TEMPLATE_PARAM_NOT_ALLOWED', { errors: [{ field: 'challenge_ids', code: 'invalid', message: 'not allowed by the template' }] });
      state.created++;
      const ev = { ...b, event_id: `0193a4d2-ac00-7000-8000-${String(100 + state.created).padStart(12, '0')}`, tenant_id: tenantOf(r) ?? T_A, status: 'scheduled', version: 1, created_at: new Date().toISOString() };
      state.events.unshift(ev);
      keys.set(key, ev);
      return json(201, ev, { ETag: '"1"' });
    }
    if ((x = /^\/lms\/school-events\/([^/:]+)\/progress$/.exec(r.path)) && r.method === 'GET') {
      const ev = find(r, x[1]!);
      if (!ev) return problem(404, 'NOT_FOUND');
      return json(200, { ...state.progress, event_id: ev.event_id, status: ev.status });
    }
    if ((x = /^\/lms\/school-events\/([^/:]+):cancel$/.exec(r.path)) && r.method === 'POST') {
      const ev = find(r, x[1]!);
      if (!ev) return problem(404, 'NOT_FOUND');
      ev.status = 'cancelled';
      return json(200, ev);
    }
    if ((x = /^\/lms\/school-events\/([^/:]+)$/.exec(r.path))) {
      const ev = find(r, x[1]!);
      if (!ev) return problem(404, 'NOT_FOUND');
      if (r.method === 'GET') return json(200, ev, { ETag: `"${String(ev.version)}"` });
      if (r.method === 'PATCH') {
        if (ev.status !== 'scheduled') return problem(409, 'CONFLICT');
        if (r.headers['if-match'] !== `"${String(ev.version)}"`) return problem(412, 'VERSION_MISMATCH');
        Object.assign(ev, r.body as Record<string, unknown>, { version: Number(ev.version) + 1 });
        return json(200, ev, { ETag: `"${String(ev.version)}"` });
      }
    }
    return undefined;
  });
  return state;
}

export interface RenderSchoolEventsOptions {
  as?: string;
  state?: SchoolEventsState;
  disabledMenus?: string[];
  disabledActions?: string[];
  width?: number;
}

/** MOCK (test only): the whole portal at `url` signed in as `as` with the F052 and F081 MOCK handlers. */
export async function renderSchoolEvents(url: string, opts: RenderSchoolEventsOptions = {}) {
  const { renderSchoolLms } = await import('./school-lms-harness');
  const { ACCOUNTS } = await import('./mock-backend');
  const { SAFETY_ACCOUNTS } = await import('./chat-safety-mock');
  const name = opts.as ?? 'ad.truong.a';
  const account = ACCOUNTS[name] ? undefined : SAFETY_ACCOUNTS[name];
  const state = opts.state ?? schoolEventsState();
  const r = await renderSchoolLms(url, {
    as: account ? 'ad.truong.a' : name,
    ...(account ? { account } : {}),
    disabledMenus: opts.disabledMenus,
    disabledActions: opts.disabledActions,
    width: opts.width,
    beforeRender: (backend) => installSchoolEventsMock(backend, state),
  });
  return { ...r, events: state };
}
