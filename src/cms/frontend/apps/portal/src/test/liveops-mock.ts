// MOCK (test only, AGENTS R17): in-memory stand-in of the F080 operations of specs/openapi/eduworld-api.v1.yaml used by
// workspace E (getLiveOpsOverview, listEventRuns, getEventRunStats, suspend/resume/cancelEventRun, the Game Analytics
// reads and exports, listGameConfigVersions for the deep link, listCatalogSchools) and by "Hoạt động game" of workspace C
// (getTenantGameSummary, listTenantSchools of a `unit` tenant), plugged into MockPortalBackend as overrides. It records
// requests, keeps a little state, can delay answers to observe concurrency and simulates the answers the screens must
// handle (503, 409, 429, 422, 504, 403); it enforces NO authorization and is NOT a backend.
import { configure } from '@testing-library/react';
import { vi } from 'vitest';
import { catalogWorkspaces, SAFETY_ACCOUNTS } from './chat-safety-mock';
import { ACCOUNTS, permissionsOf, problem, TENANTS, type MockAccount, type MockPortalBackend, type RecordedRequest } from './mock-backend';
import { accountWith, installSchoolLmsMock } from './school-lms-mock';

configure({ asyncUtilTimeout: 60_000 });
vi.setConfig({ testTimeout: 600_000 }); // whole-portal screens on a shared, throttled CPU (other agents run tests too)

const json = (status: number, body: unknown) => new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } });

export const LO = {
  event1: '0193a4d2-e100-7000-8000-000000000001',
  run1: '0193a4d2-e200-7000-8000-000000000001',
  run2: '0193a4d2-e200-7000-8000-000000000002',
  quest1: '0193a4d2-e300-7000-8000-000000000001',
  boss1: '0193a4d2-e300-7000-8000-000000000002',
  exportJob: '0193a4d2-e400-7000-8000-000000000001',
  schoolU1: '0193a4d2-2000-7000-8000-0000000000d1',
  schoolU2: '0193a4d2-2000-7000-8000-0000000000d2',
} as const;

export const AS_OF = '2026-11-26T09:41:07+07:00';

/** Permissions of the platform role `liveops` (src/backend/pkg/authz/roles.go). */
export const LIVEOPS_PERMS = [
  'game.event.write', 'game.event.operate', 'game.season_track.write', 'game.content.publish', 'content.release.rollback', 'game.liveops.read', 'game.analytics.read',
  'competition.config.season.write',
];

export const LIVEOPS_ACCOUNTS: Record<string, MockAccount> = {
  'lo.vanhanh': {
    username: 'lo.vanhanh', email: 'lo.vanhanh@eduworld.test', password: ACCOUNTS['sa.admin']!.password, platform: true, sub: '0193a4d2-1111-7000-8000-0000000000f1',
    displayName: 'Live Ops', tenants: [], perms: { platform: { workspaces: catalogWorkspaces(LIVEOPS_PERMS), permissions: LIVEOPS_PERMS } },
  },
  /** Live Ops reader without game.event.operate (no Tạm dừng / Tiếp tục / Hủy). */
  'lo.xem': {
    username: 'lo.xem', email: 'lo.xem@eduworld.test', password: ACCOUNTS['sa.admin']!.password, platform: true, sub: '0193a4d2-1111-7000-8000-0000000000f2',
    displayName: 'Live Ops viewer', tenants: [], perms: { platform: { workspaces: catalogWorkspaces(['game.liveops.read', 'game.analytics.read']), permissions: ['game.liveops.read', 'game.analytics.read'] } },
  },
  'sm.antoan': SAFETY_ACCOUNTS['sm.antoan']!,
  /** Tenant Admin of the `unit` tenant pgd-demo (analytics.tenant.read). */
  'ad.donvi': accountWith('ad.donvi', TENANTS.pgdDemo, permissionsOf(['school_lms']), '0193a4d2-1111-7000-8000-0000000000f3'),
  /** School Admin of TH Demo A: analytics.school.read only. */
  'ad.truong.a': accountWith('ad.truong.a', TENANTS.thDemoA, ['analytics.school.read', 'lms.report.read', 'report.export', 'tenant.school.read'], '0193a4d2-1111-7000-8000-000000000001'),
};

export const kpi = (code: string, value: number | null, status: 'normal' | 'warning' | 'incident' | 'unknown' = 'normal', extra: Record<string, unknown> = {}) => ({ code, value, unit: 'count', status, ...extra });

export const metricSeries = (code: string, values: (number | null)[], extra: Record<string, unknown> = {}) => ({
  metric_code: code,
  unit: 'count',
  granularity: 'day',
  points: values.map((v, i) => ({ period_start: `2026-11-${String(20 + i).padStart(2, '0')}`, value: v, suppressed: v === null })),
  ...extra,
});

const meta = { grain: 'day', data_as_of: AS_OF, is_partial: false };

export function liveOpsState() {
  return {
    overview: { as_of: AS_OF, kpis: [kpi('ccu', 1234), kpi('reports_pending', 12, 'warning'), kpi('http_5xx_rate', 0.012, 'incident', { unit: 'ratio' }), kpi('new_kpi_code', null, 'unknown')] } as Record<string, unknown>,
    overviewError: null as Response | null,
    runs: [
      { event_id: LO.event1, event_run_id: LO.run1, event_kind: 'world_festival', code: 'fest-1', name: { vi: 'Lễ hội Mùa Thu', en: 'Autumn Festival' }, status: 'live', starts_at: '2026-11-20T00:00:00+07:00', ends_at: '2026-11-30T00:00:00+07:00', suspended_reason: null },
      { event_id: LO.event1, event_run_id: LO.run2, event_kind: 'community_goal', code: 'goal-1', name: { vi: 'Mục tiêu chung' }, status: 'suspended', starts_at: '2026-11-21T00:00:00+07:00', ends_at: '2026-11-28T00:00:00+07:00', suspended_reason: 'Sự cố phần thưởng' },
    ] as Record<string, unknown>[],
    /** Answer of the next run operation (e.g. 409); null = apply the transition. */
    runOpAnswer: null as Response | null,
    /** Milliseconds every analytics read waits (to observe the ≤ 4 in flight). */
    analyticsDelayMs: 0,
    inFlight: 0,
    maxInFlight: 0,
    /** Answers by path of the analytics reads (first match): errors per panel. */
    analyticsAnswers: [] as ((r: RecordedRequest) => Response | undefined)[],
    metrics: (codes: string[], q: URLSearchParams) => {
      const dimParam = ['school_ids', 'grade_bands', 'platforms', 'locales', 'device_tiers'].find((p) => q.getAll(p).length);
      if (dimParam) return codes.flatMap((c) => q.getAll(dimParam).map((v) => metricSeries(c, [10, 12], { dims: { [dimParam]: v } })));
      return codes.map((c) => metricSeries(c, c === 'session_duration_p50' ? [600, 660] : c === 'stickiness' ? [0.3, 0.32] : [100, 120], c === 'stickiness' ? { unit: 'ratio' } : c === 'session_duration_p50' ? { unit: 'seconds' } : {}));
    },
    exportPolls: 0,
    /** While true, getGameAnalyticsExport answers `running` (to observe "Đang chuẩn bị báo cáo"). */
    exportHold: false,
    exportStatus: 'running' as 'running' | 'succeeded',
    summary: null as Record<string, unknown> | null,
    summaryAnswers: [] as Response[],
  };
}
export type LiveOpsState = ReturnType<typeof liveOpsState>;

export const tenantGameSummary = (extra: Record<string, unknown> = {}) => ({
  from: '2026-11-20',
  to: '2026-11-26',
  grain: 'day',
  data_as_of: AS_OF,
  is_partial: false,
  available_from: null,
  suppressed_cells: 2,
  active_students: 240,
  dau_avg: 80,
  retention_d7: 0.42,
  quest_completion_rate: 0.61,
  event_participation_rate: 0.35,
  sessions_over_reminder_rate: 0.12,
  sessions_over_second_reminder_rate: 0.03,
  late_night_play_rate: null,
  session_duration_p50_seconds: 1500,
  active_days_3plus_rate: 0.38,
  chapter0_completion_rate: 0.7,
  social_play_rate: 0.27,
  active_students_series: [
    { t: '2026-11-25', value: 70 },
    { t: '2026-11-26', value: null },
  ],
  platform_distribution: [
    { key: 'web', share: 0.6 },
    { key: 'android', share: null },
  ],
  device_tier_distribution: [{ key: 'mid', share: 0.8 }],
  by_school: [
    { school_id: LO.schoolU1, active_students: 200, quest_completion_rate: 0.6 },
    { school_id: LO.schoolU2, active_students: null, quest_completion_rate: null },
  ],
  by_grade_band: [{ grade_band: 'g4_5', active_students: 120, quest_completion_rate: 0.5 }],
  ...extra,
});

const unitSchool = (id: string, vi: string) => ({ school_id: id, tenant_id: TENANTS.pgdDemo.tenant_id, code: vi, name: { vi }, school_level: 'primary', grade_min: 1, grade_max: 5, status: 'active', is_virtual: false, source: 'sis', version: 1 });

/** Installs the F080 MOCK on a backend (with the workspace C MOCK); returns its mutable state. */
export function installLiveOpsMock(backend: MockPortalBackend, state: LiveOpsState = liveOpsState()): LiveOpsState {
  installSchoolLmsMock(backend);
  const analytics = async (r: RecordedRequest, answer: () => Response): Promise<Response> => {
    state.inFlight++;
    state.maxInFlight = Math.max(state.maxInFlight, state.inFlight);
    try {
      if (state.analyticsDelayMs) await new Promise((res) => setTimeout(res, state.analyticsDelayMs));
      for (const a of state.analyticsAnswers) {
        const x = a(r);
        if (x) return x;
      }
      return answer();
    } finally {
      state.inFlight--;
    }
  };
  backend.overrides.unshift(async (r) => {
    const q = new URLSearchParams(r.search);
    const p = r.path;
    let x: RegExpExecArray | null;
    if (r.method === 'GET' && p === '/admin/liveops/overview') return state.overviewError ?? json(200, state.overview);
    if (r.method === 'GET' && p === '/game-admin/event-runs') {
      const statuses = q.getAll('status');
      const rows = state.runs.filter((x) => !statuses.length || statuses.includes(String(x.status)));
      const limit = Number(q.get('limit') ?? 20);
      const from = Number(q.get('cursor') ?? 0);
      const more = from + limit < rows.length;
      return json(200, { data: rows.slice(from, from + limit), page: { has_more: more, next_cursor: more ? String(from + limit) : null, limit } });
    }
    if ((x = /^\/game-admin\/events\/([^/]+)\/runs\/([^/:]+)\/stats$/.exec(p))) {
      const run = state.runs.find((y) => y.event_run_id === x![2]);
      if (!run) return problem(404, 'NOT_FOUND');
      return json(200, {
        event_run_id: run.event_run_id, status: run.status, participants: 321, participation_rate: 0.4, rewards_granted: 150, updated_at: AS_OF,
        community_goal: { goal_id: '0193a4d2-e500-7000-8000-000000000001', metric: 'quests_completed', scope: 'platform', progress: 500, target: 1000 },
        milestone_claims: [{ milestone_id: 'm1', claims: 40 }],
      });
    }
    if ((x = /^\/game-admin\/events\/([^/]+)\/runs\/([^/:]+):(suspend|resume|cancel)$/.exec(p)) && r.method === 'POST') {
      const run = state.runs.find((y) => y.event_run_id === x![2]);
      if (!run) return problem(404, 'NOT_FOUND');
      if (state.runOpAnswer) {
        const a = state.runOpAnswer;
        state.runOpAnswer = null;
        return a;
      }
      run.status = { suspend: 'suspended', resume: 'live', cancel: 'cancelled' }[x[3] as 'suspend']!;
      return json(200, run);
    }
    if ((x = /^\/game-admin\/(quests|challenges|bosses)\/([^/]+)\/versions$/.exec(p))) {
      return json(200, { item_id: x[2], current_version: 3, data: [{ version: 4, status: 'draft' }], page: { has_more: false, next_cursor: null, limit: 1 } });
    }
    if (r.method === 'GET' && p === '/catalog/schools') {
      return json(200, { items: [unitSchool(LO.schoolU1, 'TH Đơn vị 1')], page: { has_more: false, next_cursor: null, limit: 100 } });
    }
    if (r.method === 'GET' && p === '/admin/analytics/game/metrics') return analytics(r, () => json(200, { ...meta, data: state.metrics(q.getAll('metric_codes'), q) }));
    if (r.method === 'GET' && p === '/admin/analytics/game/retention')
      return analytics(r, () =>
        json(200, { ...meta, cohorts: [{ cohort_date: '2026-11-20', cohort_size: 50, retention: [{ day_offset: 1, rate: 0.5 }, { day_offset: 7, rate: null }] }, { cohort_date: '2026-11-21', cohort_size: null, retention: [{ day_offset: 1, rate: null }] }] }),
      );
    if (r.method === 'GET' && p === '/admin/analytics/game/funnels/onboarding')
      return analytics(r, () => json(200, { ...meta, funnel_code: 'onboarding', steps: [{ step_code: 'first_login', count: 100, conversion_rate: 1 }, { step_code: 'avatar_created', count: 80, conversion_rate: 0.8 }, { step_code: 'tutorial_completed', count: null, conversion_rate: null }] }));
    if (r.method === 'GET' && p === '/admin/analytics/game/content-stats') {
      const kind = q.get('kind') ?? 'quest';
      const id = kind === 'boss' ? LO.boss1 : LO.quest1;
      return analytics(r, () =>
        json(200, { ...meta, data: [{ kind, content_id: id, title: { vi: kind === 'boss' ? 'Boss Rồng Lửa' : 'Quest Rừng Xanh', en: 'Green Forest' }, starts: 200, completions: 120, completion_rate: 0.6, abandon_rate: 0.3, success_rate: 0.55, median_duration_ms: 900_000, flagged_for_review: true }], page: { has_more: false, next_cursor: null, limit: 10 } }),
      );
    }
    if (r.method === 'GET' && p === '/admin/analytics/game/performance')
      return analytics(r, () => json(200, { ...meta, data: [{ platform: 'web', device_tier: 'mid', preset: 'medium', renderer: 'compatibility', client_version: '1.2.0', sessions: 500, fps_p50: 45, fps_p10: 28, fps_budget_pass_rate: 0.9, load_time_p50_ms: 4200, crash_free_sessions: 0.995 }] }));
    if (r.method === 'POST' && p === '/admin/analytics/game/exports') return json(202, { job_id: LO.exportJob, status: 'queued', report: (r.body as { report: string }).report, row_count: 0, created_at: AS_OF });
    if (r.method === 'GET' && p === `/admin/analytics/game/exports/${LO.exportJob}`) {
      state.exportPolls++;
      const done = !state.exportHold && state.exportPolls >= 2;
      return json(200, { job_id: LO.exportJob, status: done ? 'succeeded' : 'running', report: 'metrics', row_count: done ? 42 : 0, created_at: AS_OF, ...(done ? { download_url: 'https://exports.example.test/analytics.csv', expires_at: '2099-01-01T00:00:00Z' } : {}) });
    }
    if (r.method === 'GET' && p === '/tenant/analytics/game/summary') {
      const a = state.summaryAnswers.shift();
      if (a) return a;
      return json(200, state.summary ?? tenantGameSummary());
    }
    if (r.method === 'GET' && p === '/tenant/schools' && r.headers['x-tenant-id'] === TENANTS.pgdDemo.tenant_id) {
      // two cursor pages: the school filter must walk every page
      const page2 = q.get('cursor') === 'p2';
      return json(200, page2 ? { items: [unitSchool(LO.schoolU2, 'TH Đơn vị 2')], page: { has_more: false, next_cursor: null, limit: 100 } } : { items: [unitSchool(LO.schoolU1, 'TH Đơn vị 1')], page: { has_more: true, next_cursor: 'p2', limit: 100 } });
    }
    return undefined;
  });
  return state;
}

export interface RenderLiveOpsOptions {
  account?: string;
  state?: LiveOpsState;
  disabledMenus?: string[];
  disabledActions?: string[];
  width?: number;
}

/** MOCK (test only): renders the whole portal at `url` signed in as `account` with the F080 MOCK installed. */
export async function renderLiveOps(url: string, opts: RenderLiveOpsOptions = {}) {
  const { MockPortalBackend, renderPortal, setViewportWidth } = await import('./portal-harness');
  const name = opts.account ?? 'sa.admin';
  const custom = LIVEOPS_ACCOUNTS[name];
  const backend = new MockPortalBackend().withSession(custom ? 'sa.admin' : name);
  if (custom) backend.user = custom;
  backend.disabledMenus = opts.disabledMenus ?? [];
  backend.disabledActions = opts.disabledActions ?? [];
  const state = installLiveOpsMock(backend, opts.state);
  if (opts.width) setViewportWidth(opts.width);
  const rendered = await renderPortal({ url, backend });
  return { ...rendered, state };
}
