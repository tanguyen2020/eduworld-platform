// getLiveOpsOverview (F080 T040, F080-AC-20, ADMIN_PORTAL §3.14): real-time KPIs that module game reads from Prometheus
// (the portal never calls Prometheus). Refetched every LIVE_REFRESH_MS (inside the 15–30 s of the spec) while the
// screen is mounted; a 503 (Prometheus down) is an error state — the screen never turns it into zeros. A System Admin
// may narrow the KPIs to the tenants of the topbar filter (`tenant_ids[]`, ADR-0027 §6).
import { useQuery } from '@tanstack/react-query';
import { usePermissions } from '../../../permissions/usePermissions';
import { withTenantScope } from '../../../shared/api/withTenantScope';
import { useTenantFilter } from '../../../shell/tenantFilterStore';
import { liveOpsKey, useLiveOpsApi, type Schemas } from './common';

export type LiveOpsOverview = Schemas['LiveOpsOverview'];
export type LiveOpsKpi = Schemas['LiveOpsKpi'];
export type KpiStatus = LiveOpsKpi['status'];

/** Auto refresh of the Live dashboard: 20 s (spec F080-AC-20: 15–30 s; a client cadence, not a business parameter). */
export const LIVE_REFRESH_MS = 20_000;

export function useLiveOpsOverview() {
  const api = useLiveOpsApi();
  const { crossTenant } = usePermissions();
  const filter = useTenantFilter();
  const query = withTenantScope({}, { crossTenant, filter });
  return useQuery({
    queryKey: liveOpsKey('overview', query),
    queryFn: () => api.call<LiveOpsOverview>(api.client.GET('/admin/liveops/overview', { params: { query } })),
    refetchInterval: LIVE_REFRESH_MS,
    refetchIntervalInBackground: false,
    staleTime: LIVE_REFRESH_MS / 2,
  });
}
