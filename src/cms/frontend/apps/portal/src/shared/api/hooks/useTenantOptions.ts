// useTenantOptions (F051 T045, F051-AC-22/27): `listTenants` (internal gateway, `platform.cross_tenant.read`) as the
// source of the System Admin tenant filter and target-tenant field. Search by code/name, debounced; the search text
// is cut at the `maxLength` of the `q` parameter in OpenAPI (100).
import { useQuery } from '@tanstack/react-query';
import { useEffect, useState } from 'react';
import { usePortalRuntime } from '../../../app/runtime';
import type { components } from '../schema.gen';

export type TenantSummary = components['schemas']['TenantSummary'];

/** `maxLength` of `listTenants` `q` (specs/openapi/eduworld-api.v1.yaml). */
export const TENANT_SEARCH_MAX_LENGTH = 100;
export const TENANT_SEARCH_DEBOUNCE_MS = 300;

export function clampTenantSearch(q: string): string {
  return q.slice(0, TENANT_SEARCH_MAX_LENGTH);
}

function useDebounced<T>(value: T, ms: number): T {
  const [v, setV] = useState(value);
  useEffect(() => {
    const h = setTimeout(() => setV(value), ms);
    return () => clearTimeout(h);
  }, [value, ms]);
  return v;
}

export function useTenantOptions(search: string, enabled: boolean) {
  const { api } = usePortalRuntime();
  const q = useDebounced(clampTenantSearch(search.trim()), TENANT_SEARCH_DEBOUNCE_MS);
  return useQuery({
    queryKey: ['admin', 'tenants', q],
    queryFn: async () => {
      const res = await api.call<{ data: TenantSummary[] }>(api.client.GET('/admin/tenants', { params: { query: q ? { q } : {} } }));
      return res.data;
    },
    enabled,
    staleTime: 30_000,
  });
}
