// Integrations (read only) hooks (F059 T046, F059-AC-22/23, FR-SIS-007, ADMIN_PORTAL §10): SIS connections of every
// tenant (listSisConnections: `tenant_ids[]`, `status`, cursor), their schools (listSisConnectionSchools) and sync runs
// (listSisSyncRuns). Responses carry no secret (`has_secret` only, F011-AC-27/F017-AC-04) and the screens render only
// schema fields. getIntegrationOverview (OQ F059-2 resolved 2026-11-20): Keycloak issuer/realm/clients, platform MFA,
// APISIX gateway URLs (null = not configured; route groups not provided — OQ F059-12), SIS counts by status, all from
// the server configuration without any secret. No write operation here.
import { useQuery } from '@tanstack/react-query';
import { useCrossTenantScope } from '../../../shared/tenancy/useCrossTenantScope';
import { PAGE_SIZE } from '../limits';
import { useCursorList, usePlatformApi, type CursorPage, type Schemas } from './common';

export type SisConnection = Schemas['SisConnection'];
export type SisConnectionStatus = Schemas['SisConnectionStatus'];
export type SyncRun = Schemas['SyncRun'];
export type SisConnectionSchool = Schemas['SisConnectionSchool'];
export type IntegrationOverview = Schemas['IntegrationOverview'];

export function useIntegrationOverview(enabled: boolean) {
  const api = usePlatformApi();
  return useQuery({ queryKey: ['platform', 'integration-overview'], queryFn: () => api.call(api.client.GET('/admin/integrations', {})), enabled });
}

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
