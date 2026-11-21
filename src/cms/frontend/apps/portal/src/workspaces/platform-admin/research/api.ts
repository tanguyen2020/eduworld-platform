// Research API hooks of workspace D (F035 US8, F035-AC-35): research agreements, efficacy studies (cohorts = school × grade of the study — ADR-0095, waves,
// withdrawals, reports, pseudonymised dataset) and A/B learning experiments. Hooks only call the API and shape the
// requests — every rule (pairing, statistics, guardrails, small-cell suppression) is computed by the backend (AGENTS
// R1, R7). Tenant writes carry the X-Tenant-Id of their target tenant; lists of tenant data carry `tenant_ids[]`
// (CrossTenantScope, audit `cross_tenant.accessed`).
import { useInfiniteQuery, useQuery, useQueryClient, type QueryKey } from '@tanstack/react-query';
import { useIdempotentMutation } from '../../../shared/api/useIdempotentMutation';
import { useCrossTenantScope } from '../../../shared/tenancy/useCrossTenantScope';
import { pageQuery, usePlatformApi, versionIfMatch, type PageInfo, type Schemas } from '../api/common';

export type ResearchAgreement = Schemas['ResearchAgreement'];
export type ResearchAgreementCreate = Schemas['ResearchAgreementCreate'];
export type EfficacyStudy = Schemas['EfficacyStudy'];
export type EfficacyStudyCreate = Schemas['EfficacyStudyCreate'];
export type EfficacyStudyStatus = Schemas['EfficacyStudyStatus'];
export type EfficacyStudyCohort = Schemas['EfficacyStudyCohort'];
export type EfficacyStudyCohortCreate = Schemas['EfficacyStudyCohortCreate'];
export type EfficacyWave = Schemas['EfficacyWave'];
export type EfficacyWaveCreate = Schemas['EfficacyWaveCreate'];
export type EfficacyWithdrawalCreate = Schemas['EfficacyWithdrawalCreate'];
export type EfficacyWithdrawal = Schemas['EfficacyWithdrawal'];
export type EfficacyReport = Schemas['EfficacyReport'];
export type LearningExperiment = Schemas['LearningExperiment'];
export type LearningExperimentCreate = Schemas['LearningExperimentCreate'];
export type LearningExperimentStatus = Schemas['LearningExperimentStatus'];
export type LearningExperimentKind = Schemas['LearningExperimentKind'];
export type ExperimentArm = Schemas['ExperimentArm'];
export type ExperimentResults = Schemas['ExperimentResults'];
export type ReportJob = Schemas['ReportJob'];

export const STUDY_STATUSES: readonly EfficacyStudyStatus[] = ['draft', 'active', 'completed', 'cancelled'];
export const EXPERIMENT_STATUSES: readonly LearningExperimentStatus[] = ['draft', 'shadow', 'live', 'stopped', 'completed'];
export const EXPERIMENT_KINDS: readonly LearningExperimentKind[] = ['mastery_model', 'recommendation_rule'];
export const WAVE_KINDS: readonly Schemas['EfficacyWaveKind'][] = ['validation', 'pre', 'post'];
export const ARMS: readonly Schemas['EfficacyArm'][] = ['intervention', 'comparison', 'validation'];
export const WITHDRAW_SCOPES: readonly EfficacyWithdrawalCreate['scope'][] = ['tenant', 'school', 'cohort', 'students'];
/** Parameters of the rule engine an arm may change (backend catalog, spec Clarifications 2026-11-21 (b)). */
export const RULE_PARAMS = ['review_threshold', 'max_recommendations', 'prerequisite_depth', 'spaced_review_days', 'practice_difficulty', 'diagnostic_threshold'] as const;

/** `items` + `page` lists of the research API. */
interface ItemsPage<T> {
  items: T[];
  page: PageInfo;
}

const KEY = ['platform', 'research'] as const;
const studyKey = (id: string): QueryKey => [...KEY, 'study', id];
const experimentKey = (id: string): QueryKey => [...KEY, 'experiment', id];

function useItemsList<T>(queryKey: QueryKey, fetchPage: (cursor: string | undefined) => Promise<ItemsPage<T>>, enabled = true) {
  const q = useInfiniteQuery({
    queryKey,
    queryFn: ({ pageParam }) => fetchPage(pageParam),
    initialPageParam: undefined as string | undefined,
    getNextPageParam: (last: ItemsPage<T>) => (last.page.has_more && last.page.next_cursor ? last.page.next_cursor : undefined),
    enabled,
  });
  return { ...q, rows: q.data?.pages.flatMap((p) => p.items) ?? [] };
}

// ---------- agreements ----------

/** listResearchAgreements: `tenant_ids` is mandatory (Clarification 2026-11-21), so the list waits for a selection. */
export function useAgreements(tenantIds: readonly string[], status?: ResearchAgreement['status']) {
  const api = usePlatformApi();
  const query = { tenant_ids: [...tenantIds], ...(status ? { status } : {}) };
  return useItemsList<ResearchAgreement>([...KEY, 'agreements', query], (cursor) =>
    api.call<ItemsPage<ResearchAgreement>>(api.client.GET('/admin/learning/research-agreements', { params: { query: { ...query, ...pageQuery(cursor) } } })),
    tenantIds.length > 0,
  );
}

export function useCreateAgreement() {
  const api = usePlatformApi();
  const scope = useCrossTenantScope();
  const qc = useQueryClient();
  return useIdempotentMutation(
    ({ tenantId, body }: { tenantId: string; body: ResearchAgreementCreate }, headers) =>
      api.call<ResearchAgreement>(api.client.POST('/admin/learning/research-agreements', { body, headers: { ...headers, ...scope.writeHeaders(tenantId) } })),
    { onSuccess: () => qc.invalidateQueries({ queryKey: [...KEY, 'agreements'] }) },
  );
}

export function useRevokeAgreement() {
  const api = usePlatformApi();
  const scope = useCrossTenantScope();
  const qc = useQueryClient();
  return useIdempotentMutation(
    ({ agreement, reason }: { agreement: ResearchAgreement; reason: string }, headers) =>
      api.call<ResearchAgreement>(
        api.client.POST('/admin/learning/research-agreements/{agreement_id}:revoke', {
          params: { path: { agreement_id: agreement.agreement_id } },
          body: { reason },
          headers: { ...headers, ...scope.writeHeaders(agreement.tenant_id) },
        }),
      ),
    { onSuccess: () => qc.invalidateQueries({ queryKey: KEY }) },
  );
}

// ---------- studies ----------

export function useStudies(status?: EfficacyStudyStatus) {
  const api = usePlatformApi();
  const scope = useCrossTenantScope();
  const query = scope.listQuery(status ? { status } : {});
  return useItemsList<EfficacyStudy>([...KEY, 'studies', query], (cursor) =>
    api.call<ItemsPage<EfficacyStudy>>(api.client.GET('/admin/learning/efficacy-studies', { params: { query: { ...query, ...pageQuery(cursor) } } })),
  );
}

export function useStudy(id: string) {
  const api = usePlatformApi();
  return useQuery({
    queryKey: studyKey(id),
    queryFn: () => api.call<EfficacyStudy>(api.client.GET('/admin/learning/efficacy-studies/{study_id}', { params: { path: { study_id: id } } })),
  });
}

export function useCreateStudy() {
  const api = usePlatformApi();
  const qc = useQueryClient();
  return useIdempotentMutation(
    (body: EfficacyStudyCreate, headers) => api.call<EfficacyStudy>(api.client.POST('/admin/learning/efficacy-studies', { body, headers })),
    { onSuccess: () => qc.invalidateQueries({ queryKey: [...KEY, 'studies'] }) },
  );
}

/** activateEfficacyStudy / completeEfficacyStudy. */
export function useStudyTransition(studyId: string, transition: 'activate' | 'complete') {
  const api = usePlatformApi();
  const qc = useQueryClient();
  return useIdempotentMutation(
    (_: void, headers) =>
      api.call<EfficacyStudy>(
        transition === 'activate'
          ? api.client.POST('/admin/learning/efficacy-studies/{study_id}:activate', { params: { path: { study_id: studyId } }, headers })
          : api.client.POST('/admin/learning/efficacy-studies/{study_id}:complete', { params: { path: { study_id: studyId } }, headers }),
      ),
    { onSuccess: () => qc.invalidateQueries({ queryKey: KEY }) },
  );
}

export function useStudyCohorts(studyId: string) {
  const api = usePlatformApi();
  const scope = useCrossTenantScope();
  const query = scope.listQuery({});
  return useItemsList<EfficacyStudyCohort>([...studyKey(studyId), 'cohorts', query], (cursor) =>
    api.call<ItemsPage<EfficacyStudyCohort>>(
      api.client.GET('/admin/learning/efficacy-studies/{study_id}/cohorts', { params: { path: { study_id: studyId }, query: { ...query, ...pageQuery(cursor) } } }),
    ),
  );
}

export function useAddStudyCohort(studyId: string) {
  const api = usePlatformApi();
  const scope = useCrossTenantScope();
  const qc = useQueryClient();
  return useIdempotentMutation(
    ({ tenantId, body }: { tenantId: string; body: EfficacyStudyCohortCreate }, headers) =>
      api.call<EfficacyStudyCohort>(
        api.client.POST('/admin/learning/efficacy-studies/{study_id}/cohorts', {
          params: { path: { study_id: studyId } },
          body,
          headers: { ...headers, ...scope.writeHeaders(tenantId) },
        }),
      ),
    { onSuccess: () => qc.invalidateQueries({ queryKey: studyKey(studyId) }) },
  );
}

export function useRemoveStudyCohort(studyId: string) {
  const api = usePlatformApi();
  const scope = useCrossTenantScope();
  const qc = useQueryClient();
  return useIdempotentMutation(
    (c: EfficacyStudyCohort, headers) =>
      api.call<unknown>(
        api.client.DELETE('/admin/learning/efficacy-studies/{study_id}/cohorts/{school_id}', {
          params: { path: { study_id: studyId, school_id: c.school_id } },
          headers: { ...headers, ...scope.writeHeaders(c.tenant_id) },
        }),
      ),
    { onSuccess: () => qc.invalidateQueries({ queryKey: studyKey(studyId) }) },
  );
}

export function useWaves(studyId: string) {
  const api = usePlatformApi();
  return useQuery({
    queryKey: [...studyKey(studyId), 'waves'],
    queryFn: () =>
      api.call<Schemas['EfficacyWaveList']>(api.client.GET('/admin/learning/efficacy-studies/{study_id}/waves', { params: { path: { study_id: studyId } } })),
  });
}

export function useScheduleWave(studyId: string) {
  const api = usePlatformApi();
  const qc = useQueryClient();
  return useIdempotentMutation(
    (body: EfficacyWaveCreate, headers) =>
      api.call<EfficacyWave>(api.client.POST('/admin/learning/efficacy-studies/{study_id}/waves', { params: { path: { study_id: studyId } }, body, headers })),
    { onSuccess: () => qc.invalidateQueries({ queryKey: studyKey(studyId) }) },
  );
}

export function useWithdraw(studyId: string) {
  const api = usePlatformApi();
  const scope = useCrossTenantScope();
  const qc = useQueryClient();
  return useIdempotentMutation(
    ({ tenantId, body }: { tenantId: string; body: EfficacyWithdrawalCreate }, headers) =>
      api.call<EfficacyWithdrawal>(
        api.client.POST('/admin/learning/efficacy-studies/{study_id}:withdraw', {
          params: { path: { study_id: studyId } },
          body,
          headers: { ...headers, ...scope.writeHeaders(tenantId) },
        }),
      ),
    { onSuccess: () => qc.invalidateQueries({ queryKey: studyKey(studyId) }) },
  );
}

export function useCreateReport(studyId: string) {
  const api = usePlatformApi();
  return useIdempotentMutation((excluded: string[], headers) =>
    api.call<EfficacyReport>(
      api.client.POST('/admin/learning/efficacy-studies/{study_id}/reports', {
        params: { path: { study_id: studyId } },
        body: excluded.length ? { excluded_question_version_ids: excluded } : {},
        headers,
      }),
    ),
  );
}

const RUNNING = ['queued', 'running'];

/** getEfficacyStudyReport, polled while the job runs. */
export function useReport(studyId: string, reportId: string | undefined) {
  const api = usePlatformApi();
  return useQuery({
    queryKey: [...studyKey(studyId), 'report', reportId],
    queryFn: () =>
      api.call<EfficacyReport>(
        api.client.GET('/admin/learning/efficacy-studies/{study_id}/reports/{report_id}', { params: { path: { study_id: studyId, report_id: reportId! } } }),
      ),
    enabled: !!reportId,
    refetchInterval: (q) => (RUNNING.includes(q.state.data?.status ?? 'queued') ? 3_000 : false),
  });
}

/** createPlatformReport `report_type = efficacy_dataset` (F035-AC-32…AC-34): pseudonymised CSV, link with a TTL. */
export function useExportDataset(study: EfficacyStudy) {
  const api = usePlatformApi();
  return useIdempotentMutation((_: void, headers) =>
    api.call<ReportJob>(
      api.client.POST('/admin/reports', {
        body: {
          report_type: 'efficacy_dataset',
          format: 'csv',
          from: study.created_at.slice(0, 10),
          to: new Date().toISOString().slice(0, 10),
          params: { study_id: study.study_id },
        },
        headers,
      }),
    ),
  );
}

export function usePlatformReport(reportId: string | undefined) {
  const api = usePlatformApi();
  return useQuery({
    queryKey: ['platform', 'reports', reportId],
    queryFn: () => api.call<ReportJob>(api.client.GET('/admin/reports/{report_id}', { params: { path: { report_id: reportId! } } })),
    enabled: !!reportId,
    refetchInterval: (q) => (RUNNING.includes(q.state.data?.status ?? 'queued') ? 3_000 : false),
  });
}

// ---------- experiments ----------

export function useExperiments(status?: LearningExperimentStatus, kind?: LearningExperimentKind) {
  const api = usePlatformApi();
  const query = { ...(status ? { status } : {}), ...(kind ? { kind } : {}) };
  return useItemsList<LearningExperiment>([...KEY, 'experiments', query], (cursor) =>
    api.call<ItemsPage<LearningExperiment>>(api.client.GET('/admin/learning/experiments', { params: { query: { ...query, ...pageQuery(cursor) } } })),
  );
}

export function useExperiment(id: string) {
  const api = usePlatformApi();
  return useQuery({
    queryKey: experimentKey(id),
    queryFn: () => api.call<LearningExperiment>(api.client.GET('/admin/learning/experiments/{experiment_id}', { params: { path: { experiment_id: id } } })),
  });
}

export function useExperimentResults(id: string) {
  const api = usePlatformApi();
  return useQuery({
    queryKey: [...experimentKey(id), 'results'],
    queryFn: () =>
      api.call<ExperimentResults>(api.client.GET('/admin/learning/experiments/{experiment_id}/results', { params: { path: { experiment_id: id } } })),
    retry: false,
  });
}

export function useCreateExperiment() {
  const api = usePlatformApi();
  const qc = useQueryClient();
  return useIdempotentMutation(
    (body: LearningExperimentCreate, headers) => api.call<LearningExperiment>(api.client.POST('/admin/learning/experiments', { body, headers })),
    { onSuccess: () => qc.invalidateQueries({ queryKey: [...KEY, 'experiments'] }) },
  );
}

/** updateLearningExperiment (draft only, If-Match = revision). */
export function useUpdateExperiment(e: LearningExperiment) {
  const api = usePlatformApi();
  const qc = useQueryClient();
  return useIdempotentMutation(
    (patch: Schemas['LearningExperimentUpdate'], headers) =>
      api.call<LearningExperiment>(
        api.client.PATCH('/admin/learning/experiments/{experiment_id}', {
          params: { path: { experiment_id: e.experiment_id } },
          body: patch,
          headers: { ...headers, ...versionIfMatch(e.revision), 'Content-Type': 'application/merge-patch+json' } as never,
        }),
      ),
    { onSuccess: () => qc.invalidateQueries({ queryKey: KEY }) },
  );
}

export function useStartExperiment(id: string) {
  const api = usePlatformApi();
  const qc = useQueryClient();
  return useIdempotentMutation(
    (exposure: 'shadow' | 'live', headers) =>
      api.call<LearningExperiment>(api.client.POST('/admin/learning/experiments/{experiment_id}:start', { params: { path: { experiment_id: id } }, body: { exposure }, headers })),
    { onSuccess: () => qc.invalidateQueries({ queryKey: KEY }) },
  );
}

export function useStopExperiment(id: string) {
  const api = usePlatformApi();
  const qc = useQueryClient();
  return useIdempotentMutation(
    (reason: string, headers) =>
      api.call<LearningExperiment>(api.client.POST('/admin/learning/experiments/{experiment_id}:stop', { params: { path: { experiment_id: id } }, body: { reason }, headers })),
    { onSuccess: () => qc.invalidateQueries({ queryKey: KEY }) },
  );
}
