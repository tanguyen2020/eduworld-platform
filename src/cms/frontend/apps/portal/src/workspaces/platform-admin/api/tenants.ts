// Tenant hooks (F057 T039, F057-AC-17): listTenants (cursor, `q`, `tenant_type`), createTenant (Idempotency-Key),
// updateTenant (merge patch, If-Match, Idempotency-Key). If-Match = `version` of the list row (OQ F057-3 resolved
// 2026-11-20); a stale version gets 412 VERSION_MISMATCH, shown by its code.
import { useQueryClient } from '@tanstack/react-query';
import { useTenantOptions } from '../../../shared/api/hooks/useTenantOptions';
import { useIdempotentMutation } from '../../../shared/api/useIdempotentMutation';
import { pageQuery, reloadOnStaleVersion, useCursorList, usePlatformApi, versionIfMatch, type CursorPage, type Schemas } from './common';

export type TenantSummary = Schemas['TenantSummary'];
export type TenantType = Schemas['TenantType'];
export type TenantCreate = Schemas['TenantCreate'];
export type TenantUpdate = Schemas['TenantUpdate'];

export const TENANT_TYPES: readonly TenantType[] = ['school', 'unit', 'common'];

const tenantPath = (id: string) => `/admin/tenants/${id}`;

export function useTenantList(q: string, tenantType: TenantType | undefined) {
  const api = usePlatformApi();
  return useCursorList<TenantSummary>(['platform', 'tenants', q, tenantType ?? null], (cursor) =>
    api.call<CursorPage<TenantSummary>>(
      api.client.GET('/admin/tenants', { params: { query: { ...pageQuery(cursor), ...(q ? { q } : {}), ...(tenantType ? { tenant_type: tenantType } : {}) } } }),
    ),
  );
}

/** Tenant id → summary of EVERY tenant (fetchAllPages over listTenants, shared cache with useTenantOptions) for the
 * Tenant column; unknown ids show the raw id. */
export function useTenantDirectory(enabled = true) {
  return new Map(useTenantOptions('', enabled).items.map((t) => [t.tenant_id, t] as const));
}

export function useCreateTenant() {
  const api = usePlatformApi();
  const qc = useQueryClient();
  return useIdempotentMutation(
    async (body: TenantCreate, headers) => {
      const pending = api.client.POST('/admin/tenants', { body, headers });
      const data = await api.call(pending);
      api.etags.remember(tenantPath(data.tenant_id), (await pending).response.headers.get('ETag'));
      return data;
    },
    { onSuccess: () => Promise.all([qc.invalidateQueries({ queryKey: ['platform', 'tenants'] }), qc.invalidateQueries({ queryKey: ['admin', 'tenants'] })]) },
  );
}

export function useUpdateTenant() {
  const api = usePlatformApi();
  const qc = useQueryClient();
  return useIdempotentMutation(
    async ({ tenantId, version, patch }: { tenantId: string; version?: number; patch: TenantUpdate }, headers) => {
      const pending = api.client.PATCH('/admin/tenants/{tenant_id}', {
        params: { path: { tenant_id: tenantId } },
        body: patch,
        headers: { ...headers, ...versionIfMatch(version), 'Content-Type': 'application/merge-patch+json' },
      });
      const data = await api.call(pending);
      api.etags.remember(tenantPath(tenantId), (await pending).response.headers.get('ETag'));
      return data;
    },
    { onSuccess: () => qc.invalidateQueries({ queryKey: ['platform', 'tenants'] }), onError: reloadOnStaleVersion(qc, ['platform', 'tenants']) },
  );
}
