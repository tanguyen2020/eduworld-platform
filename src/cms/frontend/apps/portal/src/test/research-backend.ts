// MOCK (test only, AGENTS R17): in-memory stand-in of the F035 research routes of module `learning`
// (`/admin/learning/research-agreements*`, `/admin/learning/efficacy-studies*`, `/admin/learning/experiments*`,
// `createPlatformReport efficacy_dataset`) answering per specs/openapi/eduworld-api.v1.yaml so the research screens can
// be tested. It records the requests, enforces no authorization and computes no statistic; NOT a backend.
import { MockPortalBackend, problem, TENANTS, type RecordedRequest } from './mock-backend';
import { withPlatformMock } from './platform-backend';

type Obj = Record<string, unknown>;

const json = (status: number, body: unknown, headers: Record<string, string> = {}) =>
  new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json', ...headers } });
const page = (items: unknown[]) => ({ items, page: { has_more: false, next_cursor: null, limit: 20 } });

export const RIDS = {
  study: '0193a4d2-5000-7000-8000-000000000001',
  subject: '0193a4d2-5000-7000-8000-000000000002',
  lo: '0193a4d2-5000-7000-8000-000000000003',
  formA: '0193a4d2-5000-7000-8000-000000000004',
  formB: '0193a4d2-5000-7000-8000-000000000005',
  school: '0193a4d2-5000-7000-8000-000000000007',
  agreement: '0193a4d2-5000-7000-8000-000000000008',
  experiment: '0193a4d2-5000-7000-8000-000000000009',
  report: '0193a4d2-5000-7000-8000-00000000000a',
  dataset: '0193a4d2-5000-7000-8000-00000000000b',
  wave: '0193a4d2-5000-7000-8000-00000000000c',
} as const;

export interface ResearchState {
  studies: Obj[];
  cohorts: Obj[];
  agreements: Obj[];
  experiments: Obj[];
  waves: Obj[];
}

export function researchState(): ResearchState {
  return {
    studies: [
      {
        study_id: RIDS.study, code: 'math-g3-2026', title: 'Toán 3 – học kỳ I', subject_id: RIDS.subject, grade_level: 3, objective_ids: [RIDS.lo],
        form_a_blueprint_version_id: RIDS.formA, form_b_blueprint_version_id: RIDS.formB, min_intervention_weeks: 8, status: 'draft', cohort_count: 0,
        pair_count: 0, tenant_ids: [], created_at: '2026-11-17T08:00:00Z', version: 1,
      },
    ],
    cohorts: [],
    agreements: [
      { agreement_id: RIDS.agreement, tenant_id: TENANTS.thDemoA.tenant_id, agreement_ref: 'HD-2026-01', covers: ['efficacy_study', 'experiment'], agreed_on: '2026-09-01', status: 'active', created_at: '2026-11-17T08:00:00Z' },
    ],
    experiments: [
      {
        experiment_id: RIDS.experiment, code: 'mastery-we-1-1', kind: 'mastery_model', unit: 'school_grade', status: 'draft', revision: 1, tenant_ids: [TENANTS.thDemoA.tenant_id],
        arms: [{ code: 'control', allocation_pct: 50, is_control: true, model_version: 'we-1.0' }, { code: 'we11', allocation_pct: 50, is_control: false, model_version: 'we-1.1' }],
        start_at: '2026-11-20T00:00:00Z', end_at: '2026-12-20T00:00:00Z', created_at: '2026-11-18T00:00:00Z',
      },
    ],
    waves: [],
  };
}

const REPORT = {
  report_id: RIDS.report, study_id: RIDS.study, status: 'succeeded', descriptive_only: true, created_at: '2026-11-21T08:00:00Z', finished_at: '2026-11-21T08:01:00Z',
  excluded_question_version_ids: [],
  results: {
    arms: {
      intervention: { cohorts: 1, participants: 25, analysed: 24, withdrawn: null, attrition_rate: 0.04, pre_mean: 0.41, post_mean: 0.66, gain_mean: 0.25 },
      comparison: { cohorts: 1, participants: 26, analysed: 25, withdrawn: null, attrition_rate: 0.038, pre_mean: 0.4, post_mean: 0.5, gain_mean: 0.1 },
    },
    effect: { hedges_g: 0.512, ci_low: 0.021, ci_high: 1.003 },
    objectives: [{ objective_id: RIDS.lo, intervention_gain: 0.3, comparison_gain: null }],
    attrition_warning: false,
  },
};

/** Installs the MOCK research routes on `backend` (after the workspace D mock); returns the mutable state. */
export function withResearchMock(backend: MockPortalBackend, state: ResearchState = researchState()): ResearchState {
  const s = state;
  backend.overrides.push((rec: RecordedRequest) => {
    const body = (rec.body ?? {}) as Obj;
    const route = `${rec.method} ${rec.path}`;
    const q = new URLSearchParams(rec.search);
    const P = '/admin/learning';
    if (route === `GET ${P}/research-agreements`) {
      const ids = q.getAll('tenant_ids');
      if (!ids.length) return problem(422, 'VALIDATION_FAILED');
      return json(200, page(s.agreements.filter((a) => ids.includes(String(a.tenant_id)))));
    }
    if (route === `POST ${P}/research-agreements`) {
      const tenant = rec.headers['x-tenant-id'];
      if (!tenant) return problem(400, 'TENANT_HEADER_REQUIRED');
      const a = { agreement_id: `0193a4d2-5100-7000-8000-${String(s.agreements.length).padStart(12, '0')}`, tenant_id: tenant, ...body, status: 'active', created_at: '2026-11-21T08:00:00Z' };
      s.agreements.push(a);
      return json(201, a);
    }
    const revoke = /^\/admin\/learning\/research-agreements\/([^/]+):revoke$/.exec(rec.path);
    if (revoke && rec.method === 'POST') {
      const a = s.agreements.find((x) => x.agreement_id === revoke[1]);
      if (!a) return problem(404, 'NOT_FOUND');
      Object.assign(a, { status: 'revoked', revoked_at: '2026-11-21T09:00:00Z', revoke_reason: body.reason });
      return json(200, a);
    }
    if (route === `GET ${P}/efficacy-studies`) return json(200, page(s.studies.filter((x) => !q.get('status') || x.status === q.get('status'))));
    if (route === `POST ${P}/efficacy-studies`) {
      const st = { study_id: `0193a4d2-5200-7000-8000-${String(s.studies.length).padStart(12, '0')}`, ...body, status: 'draft', created_at: '2026-11-21T08:00:00Z', version: 1 };
      s.studies.push(st);
      return json(201, st);
    }
    const study = /^\/admin\/learning\/efficacy-studies\/([^/:]+)(.*)$/.exec(rec.path);
    if (study) {
      const st = s.studies.find((x) => x.study_id === study[1]);
      if (!st) return problem(404, 'NOT_FOUND');
      const rest = study[2];
      if (rest === '' && rec.method === 'GET') return json(200, st, { ETag: `W/"${String(st.version)}"` });
      if (rest === ':activate') {
        if (st.status !== 'draft') return problem(422, 'INVALID_STATE_TRANSITION');
        st.status = 'active';
        st.activated_at = '2026-11-21T08:30:00Z';
        return json(200, st);
      }
      if (rest === ':complete') return json(200, Object.assign(st, { status: 'completed' }));
      if (rest === '/cohorts' && rec.method === 'GET') return json(200, page(s.cohorts));
      if (rest === '/cohorts' && rec.method === 'POST') {
        const tenant = rec.headers['x-tenant-id'];
        if (!tenant) return problem(400, 'TENANT_HEADER_REQUIRED');
        const c = { study_id: st.study_id, tenant_id: tenant, grade_level: st.grade_level, conflict: false, status: 'enrolled', participant_count: 0, ...body };
        s.cohorts.push(c);
        return json(201, c);
      }
      if (rest === '/waves' && rec.method === 'GET') return json(200, { items: s.waves });
      if (rest === '/waves' && rec.method === 'POST') {
        const w = { wave_id: RIDS.wave, study_id: st.study_id, status: 'scheduled', duration_seconds: 2400, cohorts: [], ...body };
        s.waves.push(w);
        return json(201, w);
      }
      if (rest === ':withdraw') {
        if (!rec.headers['x-tenant-id']) return problem(400, 'TENANT_HEADER_REQUIRED');
        return json(200, { withdrawal_id: RIDS.wave, tenant_id: rec.headers['x-tenant-id'], scope: body.scope, affected_count: (body.student_ids as unknown[] | undefined)?.length ?? 1, cancelled_exam_session_count: 0, created_at: '2026-11-21T09:00:00Z' });
      }
      if (rest === '/reports' && rec.method === 'POST') return json(202, { ...REPORT, status: 'queued', results: null });
      if (rest === `/reports/${RIDS.report}`) return json(200, REPORT);
    }
    if (route === 'POST /admin/reports') {
      return json(202, { report_id: RIDS.dataset, report_type: 'efficacy_dataset', format: 'csv', status: 'queued', created_at: '2026-11-21T09:00:00Z' });
    }
    if (route === `GET /admin/reports/${RIDS.dataset}`) {
      return json(200, { report_id: RIDS.dataset, report_type: 'efficacy_dataset', format: 'csv', status: 'succeeded', row_count: 49, download_url: 'https://files.example.test/efficacy.csv', download_expires_at: '2026-11-21T10:00:00Z', created_at: '2026-11-21T09:00:00Z' });
    }
    if (route === `GET ${P}/experiments`) return json(200, page(s.experiments));
    if (route === `POST ${P}/experiments`) {
      const e = { experiment_id: `0193a4d2-5300-7000-8000-${String(s.experiments.length).padStart(12, '0')}`, ...body, status: 'draft', revision: 1, created_at: '2026-11-21T08:00:00Z' };
      s.experiments.push(e);
      return json(201, e);
    }
    const exp = /^\/admin\/learning\/experiments\/([^/:]+)(.*)$/.exec(rec.path);
    if (exp) {
      const e = s.experiments.find((x) => x.experiment_id === exp[1]);
      if (!e) return problem(404, 'NOT_FOUND');
      const rest = exp[2];
      if (rest === '' && rec.method === 'GET') return json(200, e, { ETag: `W/"${String(e.revision)}"` });
      if (rest === ':start') return json(200, Object.assign(e, { status: body.exposure }));
      if (rest === ':stop') return json(200, Object.assign(e, { status: 'stopped', stop_reason: 'manual' }));
      if (rest === '/results') {
        return json(200, {
          experiment_id: e.experiment_id, computed_at: '2026-11-21T02:00:00Z', guardrail_breached: false,
          arms: [
            { arm_code: 'control', is_control: true, suppressed: false, unit_count: 3, student_count: 70, primary_metric: 0.71 },
            { arm_code: 'we11', is_control: false, suppressed: true, unit_count: null, student_count: null, primary_metric: null },
          ],
        });
      }
    }
    return undefined;
  });
  return s;
}

/** Renders the portal at `url` for `account` (default `sa.admin`) with the MOCK workspace D + research routes. */
export async function renderResearch(url: string, opts: { account?: string; state?: ResearchState; disabledActions?: string[]; disabledMenus?: string[] } = {}) {
  const { renderPortal } = await import('./portal-harness');
  const backend = new MockPortalBackend().withSession(opts.account ?? 'sa.admin');
  backend.disabledActions = opts.disabledActions ?? [];
  backend.disabledMenus = opts.disabledMenus ?? [];
  withPlatformMock(backend);
  const state = withResearchMock(backend, opts.state);
  const rendered = await renderPortal({ url, backend });
  return { ...rendered, state };
}
