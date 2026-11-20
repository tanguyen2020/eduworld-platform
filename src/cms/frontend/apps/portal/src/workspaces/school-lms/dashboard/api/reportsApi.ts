// Export and summary hooks (F056 T036, T040; F056-AC-14, AC-15, AC-17): createLmsReport / createTenantReport (one
// Idempotency-Key per user intent, AGENTS R10) then getLmsReport / getTenantReport polled while the job is queued or
// running — the file is only reached through the signed link of a succeeded job (never from the create request). The
// tracked job belongs to the tenant it was created in: after a tenant switch it is no longer polled nor shown.
// getTenantLearningSummary reads the tenant rollups with cells < 5 students suppressed by the server.
import { useQuery } from '@tanstack/react-query';
import { useState } from 'react';
import { useIdempotentMutation } from '../../../../shared/api/useIdempotentMutation';
import type { Schemas } from '../../common/api';
import { REPORT_POLL_MS, DASHBOARD_QUERY_TIMING } from '../model/limits';
import { dashKey, useDashboardContext } from './dashboardApi';

export type ReportJob = Schemas['ReportJob'];
export type LmsReportRequest = Schemas['LmsReportRequest'];
export type ReportRequest = Schemas['ReportRequest'];
export type LearningSummary = Schemas['LearningSummary'];

const running = (j: ReportJob | undefined) => j?.status === 'queued' || j?.status === 'running';

/** Polling cadence of a job query: every REPORT_POLL_MS while queued/running, then stop. */
export const pollWhileRunning = (q: { state: { data?: ReportJob } }) => (running(q.state.data) ? REPORT_POLL_MS : false);

type Kind = 'lms' | 'tenant';

/** Create + follow one export of workspace C (LMS reports or tenant reports). */
export function useReportExport<Req extends LmsReportRequest | ReportRequest>(kind: Kind) {
  const { api, scope } = useDashboardContext();
  const [tracked, setTracked] = useState<{ tenant: string; job: ReportJob } | null>(null);
  const create = useIdempotentMutation<ReportJob, Req>(
    (body, intent) =>
      api.call<ReportJob>(
        kind === 'lms'
          ? api.client.POST('/lms/reports', { body: body as LmsReportRequest, headers: { ...scope.headers, ...intent } } as never)
          : api.client.POST('/tenant/reports', { body: body as ReportRequest, headers: { ...scope.headers, ...intent } } as never),
      ),
    { onSuccess: (job) => setTracked({ tenant: scope.key, job }) },
  );
  const current = tracked && tracked.tenant === scope.key ? tracked.job : null;
  const job = useQuery({
    queryKey: dashKey(`report-${kind}`, scope, current?.report_id),
    queryFn: () =>
      api.call<ReportJob>(
        kind === 'lms'
          ? api.client.GET('/lms/reports/{report_id}', { params: { path: { report_id: current!.report_id } }, headers: scope.headers } as never)
          : api.client.GET('/tenant/reports/{report_id}', { params: { path: { report_id: current!.report_id } }, headers: scope.headers } as never),
      ),
    enabled: scope.ready && !!current && running(current),
    refetchInterval: pollWhileRunning,
  });
  return {
    job: current ? (job.data ?? current) : null,
    create: (body: Req) => create.mutate(body),
    pending: create.isPending,
    error: create.error ?? job.error,
  };
}

export interface SummaryFilters {
  schoolIds: string[];
  grade?: number;
  from: string;
  to: string;
  grain?: 'day' | 'week' | 'month';
}

/** getTenantLearningSummary (Tenant Admin, analytics.tenant.read). */
export function useTenantLearningSummary(f: SummaryFilters) {
  const { api, scope, queue } = useDashboardContext();
  const query = { from: f.from, to: f.to, ...(f.schoolIds.length ? { school_ids: f.schoolIds } : {}), ...(f.grade ? { grade_level: f.grade } : {}), ...(f.grain ? { grain: f.grain } : {}) };
  return useQuery({
    queryKey: dashKey('tenant-summary', scope, query),
    queryFn: () => queue.run(() => api.call<LearningSummary>(api.client.GET('/tenant/analytics/learning/summary', { params: { query }, headers: scope.headers } as never))),
    enabled: scope.ready,
    ...DASHBOARD_QUERY_TIMING,
  });
}
