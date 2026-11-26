// Read hooks of Game Analytics (F080 T044, F080-AC-21, F080-AC-22, F080-AC-23): the only code of the analytics screen
// that calls the network (AGENTS R1). Every read goes through the page's DashboardQueue (≤ dashboard_max_concurrent_per_user
// in flight, ADMIN_PORTAL §3.14.1); `429 DASHBOARD_BUSY` is retried by the apiClient (2–5 s, at most twice — one retry
// layer only) and `504 DASHBOARD_QUERY_TIMEOUT` is never repeated. Exports are asynchronous jobs: create with an
// Idempotency-Key per intent, then poll getGameAnalyticsExport until succeeded/failed (never download from the request).
import { useDashboardQueue } from '@eduworld/ops-ui/dashboard';
import { useQuery } from '@tanstack/react-query';
import { useState } from 'react';
import { useIdempotentMutation } from '../../../shared/api/useIdempotentMutation';
import { liveOpsKey, useLiveOpsApi, type PageInfo, type Schemas } from '../api/common';
import type { AnalyticsQuery } from './model';

export type GameMetricsResponse = Schemas['GameMetricsResponse'];
export type GameMetricSeries = Schemas['GameMetricSeries'];
export type RetentionMatrix = Schemas['RetentionMatrix'];
export type FunnelReport = Schemas['FunnelReport'];
export type ContentStatsItem = Schemas['ContentStatsItem'];
export type PerformanceReportRow = Schemas['PerformanceReportRow'];
export type AnalyticsExportJob = Schemas['AnalyticsExportJob'];
export type AnalyticsExportRequest = Schemas['AnalyticsExportRequest'];
export type ContentKind = ContentStatsItem['kind'];

interface Meta {
  grain: Schemas['DashboardGrain'];
  data_as_of: string;
  is_partial: boolean;
  available_from?: string | null;
}
export type ContentStatsPage = Meta & { data: ContentStatsItem[]; page?: PageInfo };
export type PerformanceReport = Meta & { data: PerformanceReportRow[] };

/** Rows of the content table (cursor pages, ≤ Limit.maximum). */
export const CONTENT_PAGE_SIZE = 10;
/** Polling of a running export job (client cadence, not a business parameter). */
export const EXPORT_POLL_MS = 2000;

function useRead<T>(name: string, query: Record<string, unknown>, fetch: () => Promise<T>, enabled = true) {
  const queue = useDashboardQueue();
  return useQuery({ queryKey: liveOpsKey('analytics', name, query), queryFn: () => queue.run(fetch), enabled, staleTime: 60_000 });
}

export function useGameMetrics(name: string, metricCodes: string[], query: AnalyticsQuery, enabled = true) {
  const api = useLiveOpsApi();
  const q = { ...query, metric_codes: metricCodes };
  return useRead(`metrics-${name}`, q, () => api.call<GameMetricsResponse>(api.client.GET('/admin/analytics/game/metrics', { params: { query: q } } as never)), enabled);
}

export function useGameRetention(query: AnalyticsQuery) {
  const api = useLiveOpsApi();
  const { from, to, ...rest } = query;
  const q = { ...rest, cohort_from: from as string, cohort_to: to as string };
  return useRead('retention', q, () => api.call<RetentionMatrix>(api.client.GET('/admin/analytics/game/retention', { params: { query: q } } as never)));
}

export function useGameFunnel(funnelCode: 'onboarding', query: AnalyticsQuery) {
  const api = useLiveOpsApi();
  return useRead(`funnel-${funnelCode}`, query, () =>
    api.call<FunnelReport>(api.client.GET('/admin/analytics/game/funnels/{funnel_code}', { params: { path: { funnel_code: funnelCode }, query } } as never)),
  );
}

export function useGameContentStats(kind: ContentKind, query: AnalyticsQuery, cursor: string | undefined) {
  const api = useLiveOpsApi();
  const q = { ...query, kind, sort: 'abandon_rate', limit: CONTENT_PAGE_SIZE, ...(cursor ? { cursor } : {}) };
  return useRead(`content-${kind}`, q, () => api.call<ContentStatsPage>(api.client.GET('/admin/analytics/game/content-stats', { params: { query: q } } as never)));
}

export function useGamePerformance(query: AnalyticsQuery) {
  const api = useLiveOpsApi();
  return useRead('performance', query, () => api.call<PerformanceReport>(api.client.GET('/admin/analytics/game/performance', { params: { query } } as never)));
}

const running = (j: AnalyticsExportJob | undefined) => j?.status === 'queued' || j?.status === 'running';

/** Create + follow one analytics export (CSV). */
export function useAnalyticsExport() {
  const api = useLiveOpsApi();
  const [tracked, setTracked] = useState<AnalyticsExportJob | null>(null);
  const create = useIdempotentMutation<AnalyticsExportJob, AnalyticsExportRequest>(
    (body, intent) => api.call<AnalyticsExportJob>(api.client.POST('/admin/analytics/game/exports', { body, headers: { ...intent } } as never)),
    { onSuccess: (job) => setTracked(job) },
  );
  const poll = useQuery({
    queryKey: liveOpsKey('analytics-export', tracked?.job_id),
    queryFn: () =>
      api.call<AnalyticsExportJob>(api.client.GET('/admin/analytics/game/exports/{export_id}', { params: { path: { export_id: tracked!.job_id } } } as never)),
    enabled: !!tracked && running(tracked),
    refetchInterval: (q) => (running(q.state.data) || !q.state.data ? EXPORT_POLL_MS : false),
  });
  return {
    job: tracked ? (poll.data ?? tracked) : null,
    create: (body: AnalyticsExportRequest) => create.mutate(body),
    pending: create.isPending,
    error: create.error ?? poll.error,
  };
}
