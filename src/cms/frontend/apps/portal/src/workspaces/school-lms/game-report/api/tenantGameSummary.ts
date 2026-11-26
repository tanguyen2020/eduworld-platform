// Read hooks of "Hoạt động game" (F080 T055, T061, F080-AC-27, AC-29, AC-30, GAME_ANALYTICS §7.2): getTenantGameSummary
// (aggregates of the tenant's own students, cells < 5 students suppressed by the server — a `null` number) through the
// page DashboardQueue, and the schools of the user's scope for the school filter (listTenantSchools — a School Admin
// only receives the schools of their scope; every cursor page, a dropdown is never one page). Query keys carry the tenant
// of the data. `429 DASHBOARD_BUSY` is retried by the apiClient (2–5 s, at most twice); nothing is aggregated here.
import { useQuery } from '@tanstack/react-query';
import { useAllPages } from '../../../../shared/api/fetchAllPages';
import { useApi, type Schemas } from '../../common/api';
import { dashKey, useDashboardContext } from '../../dashboard/api/dashboardApi';
import { DASHBOARD_QUERY_TIMING } from '../../dashboard/model/limits';
import { useSchoolLmsScope } from '../../common/scope';

export type TenantGameSummary = Schemas['TenantGameSummary'];
export type GradeBand = Schemas['GradeBand'];
type School = Schemas['School'];

export const GRADE_BANDS: GradeBand[] = ['g1_3', 'g4_5', 'g6_7', 'g8_9'];

export interface GameReportFilters {
  from: string;
  to: string;
  grain?: 'day' | 'week' | 'month';
  schoolIds: string[];
  gradeBands: GradeBand[];
}

/** Query of getTenantGameSummary (school_ids and grade_bands may be combined here). */
export function gameSummaryQuery(f: GameReportFilters) {
  return {
    from: f.from,
    to: f.to,
    ...(f.grain ? { grain: f.grain } : {}),
    ...(f.schoolIds.length ? { school_ids: f.schoolIds } : {}),
    ...(f.gradeBands.length ? { grade_bands: f.gradeBands } : {}),
  };
}

export function useTenantGameSummary(f: GameReportFilters) {
  const { api, scope, queue } = useDashboardContext();
  const query = gameSummaryQuery(f);
  return useQuery({
    queryKey: dashKey('game-summary', scope, query),
    queryFn: () => queue.run(() => api.call<TenantGameSummary>(api.client.GET('/tenant/analytics/game/summary', { params: { query }, headers: scope.headers } as never))),
    enabled: scope.ready,
    ...DASHBOARD_QUERY_TIMING,
  });
}

/** Every school of the user's scope (all cursor pages), for the school filter of a `unit` tenant. */
export function useScopeSchools(enabled: boolean) {
  const api = useApi();
  const scope = useSchoolLmsScope();
  return useAllPages<School>(
    ['school-lms', 'game-report-schools', scope.key],
    (query) => api.call<{ items: School[]; page: Schemas['PageInfo'] }>(api.client.GET('/tenant/schools', { params: { query: { ...query, status: 'active' } }, headers: scope.headers } as never)),
    { enabled: enabled && scope.ready, staleTime: 5 * 60_000 },
  );
}
