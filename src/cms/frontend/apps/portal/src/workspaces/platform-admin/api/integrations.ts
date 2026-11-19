// Integrations (read only) hooks (F059 T046, F059-AC-22/23, FR-SIS-007, ADMIN_PORTAL §10): SIS connections of every
// tenant (listSisConnections: `tenant_ids[]`, `status`, cursor), their schools (listSisConnectionSchools) and sync runs
// (listSisSyncRuns). Responses carry no secret (`has_secret` only, F011-AC-27/F017-AC-04) and the screens render only
// schema fields. `getIntegrationOverview` (Keycloak/APISIX reference, delta T009) is not in the contract yet
// (Câu hỏi mở F059-Q2). No write operation here.
import { useQuery } from '@tanstack/react-query';
import { useCrossTenantScope } from '../../../shared/tenancy/useCrossTenantScope';
import { PAGE_SIZE } from '../limits';
import { useCursorList, usePlatformApi, type CursorPage, type Schemas } from './common';

export type SisConnection = Schemas['SisConnection'];
export type SisConnectionStatus = Schemas['SisConnectionStatus'];
export type SyncRun = Schemas['SyncRun'];
export type SisConnectionSchool = Schemas['SisConnectionSchool'];

/** getIntegrationOverview is not in specs/openapi/eduworld-api.v1.yaml yet (F059 T009). */
export const INTEGRATION_OVERVIEW_AVAILABLE = false;

export function useSisConnections(status: SisConnectionStatus | undefined) {
  const api = usePlatformApi();
  const scope = useCrossTenantScope();
  const query = scope.listQuery(status ? { status } : {});
  return useCursorList<SisConnection>(['platform', 'sis-connections', query], (cursor) =>
    api.call<CursorPage<SisConnection>>(api.client.GET('/sis/connections', { params: { query: { ...query, limit: PAGE_SIZE, ...(cursor ? { cursor } : {}) } } })),
  );
}

export function useSisConnectionSchools(connectionId: string) {
  const api = usePlatformApi();
  return useQuery({
    queryKey: ['platform', 'sis-connection-schools', connectionId],
    queryFn: async () => (await api.call(api.client.GET('/admin/sis/connections/{connection_id}/schools', { params: { path: { connection_id: connectionId } } }))).data,
  });
}

export function useSisSyncRuns(connectionId: string) {
  const api = usePlatformApi();
  return useQuery({
    queryKey: ['platform', 'sis-sync-runs', connectionId],
    queryFn: async () =>
      (await api.call(api.client.GET('/sis/connections/{connection_id}/sync-runs', { params: { path: { connection_id: connectionId }, query: { limit: PAGE_SIZE } } }))).data,
  });
}
