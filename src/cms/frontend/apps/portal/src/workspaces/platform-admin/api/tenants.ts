// Tenant hooks (F057 T039, F057-AC-17): listTenants (cursor, `q`, `tenant_type`), createTenant (Idempotency-Key),
// updateTenant (merge patch, If-Match, Idempotency-Key). The list carries no per-tenant version, so If-Match is known
// only from the ETag of an earlier create/update of the same tenant in this session (Câu hỏi mở F057-Q3); without it
// the server answers 428 PRECONDITION_REQUIRED, shown by its code.
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { useIdempotentMutation } from '../../../shared/api/useIdempotentMutation';
import { pageQuery, useCursorList, usePlatformApi, type CursorPage, type Schemas } from './common';

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

/** Tenant id → summary of the first page of listTenants (names of the Tenant column); unknown ids show the raw id. */
export function useTenantDirectory(enabled = true) {
  const api = usePlatformApi();
  const q = useQuery({
    queryKey: ['platform', 'tenant-directory'],
    queryFn: () => api.call<CursorPage<TenantSummary>>(api.client.GET('/admin/tenants', { params: { query: { limit: 100 } } })),
    enabled,
    staleTime: 60_000,
  });
  return new Map((q.data?.data ?? []).map((t) => [t.tenant_id, t] as const));
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
    { onSuccess: () => qc.invalidateQueries({ queryKey: ['platform', 'tenants'] }) },
  );
}

export function useUpdateTenant() {
  const api = usePlatformApi();
  const qc = useQueryClient();
  return useIdempotentMutation(
    async ({ tenantId, patch }: { tenantId: string; patch: TenantUpdate }, headers) => {
      const pending = api.client.PATCH('/admin/tenants/{tenant_id}', {
        params: { path: { tenant_id: tenantId } },
        body: patch,
        headers: { ...headers, 'Content-Type': 'application/merge-patch+json' },
      });
      const data = await api.call(pending);
      api.etags.remember(tenantPath(tenantId), (await pending).response.headers.get('ETag'));
      return data;
    },
    { onSuccess: () => qc.invalidateQueries({ queryKey: ['platform', 'tenants'] }) },
  );
}
