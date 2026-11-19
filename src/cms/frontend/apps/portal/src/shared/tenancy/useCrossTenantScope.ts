// useCrossTenantScope (F057 T015, F057-AC-02/03/04, FR-CMS-013, ADMIN_PORTAL §2.5): the request convention of every
// screen with per-tenant data, from the permission view (`platform.cross_tenant.read`) and the topbar tenant filter:
// - list: `listQuery(q)` adds `tenant_ids[]` only for the System Admin with a selection (empty = all tenants, no param);
//   the apiClient never sends X-Tenant-Id in platform scope and strips `tenant_ids` for tenant users;
// - header-only list (e.g. listCatalogSchools): `single` = exactly one selected tenant → its X-Tenant-Id, else not ready;
// - write: `writeHeaders(target)` = X-Tenant-Id of the one target tenant (none for "Platform"), never "all tenants".
// Display/request shaping only — the backend authorizes and scopes every request (AGENTS R7, R8, R20).
import { usePermissions } from '../../permissions/usePermissions';
import { useTenantFilter } from '../../shell/tenantFilterStore';
import { requireSingleTenant, targetTenantHeaders, withTenantScope, type SingleTenantResult } from '../api/withTenantScope';

export interface CrossTenantScope {
  /** System Admin (`platform.cross_tenant.read`): tenant filter, Tenant column, `tenant_ids[]`. */
  crossTenant: boolean;
  /** Selected tenant ids of the filter (empty = all tenants). */
  filter: readonly string[];
  listQuery<Q extends Record<string, unknown>>(query: Q): Omit<Q, 'tenant_ids'> & { tenant_ids?: string[] };
  single: SingleTenantResult;
  writeHeaders(target: string): Record<string, string>;
}

export function useCrossTenantScope(): CrossTenantScope {
  const { crossTenant } = usePermissions();
  const selected = useTenantFilter();
  const filter = crossTenant ? selected : [];
  const ctx = { crossTenant, filter };
  return {
    crossTenant,
    filter,
    listQuery: (query) => withTenantScope(query, ctx),
    single: requireSingleTenant(ctx),
    writeHeaders: targetTenantHeaders,
  };
}
