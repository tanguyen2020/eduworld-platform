// Tenant scope of API calls (F051 T045, F051-AC-23..26, ADMIN_PORTAL §2.5) — pure helpers used by list/form screens:
// - System Admin cross-tenant list: no X-Tenant-Id (platform scope), `tenant_ids[]` = the tenant filter (empty = all);
// - tenant user: never `tenant_ids` (the apiClient strips it too) — the session X-Tenant-Id applies;
// - screen accepting only X-Tenant-Id: System Admin must pick exactly one tenant first (requireSingleTenant);
// - write by System Admin: always one target tenant (or Platform when the screen allows it), never "all tenants".
import { TENANT_HEADER } from './middleware/tenant';

export interface TenantScopeContext {
  /** `platform.cross_tenant.read` (System Admin). */
  crossTenant: boolean;
  /** Tenant filter of the topbar (empty = all tenants). */
  filter: readonly string[];
}

export function withTenantScope<Q extends Record<string, unknown>>(query: Q, ctx: TenantScopeContext): Omit<Q, 'tenant_ids'> & { tenant_ids?: string[] } {
  const { tenant_ids: _ignored, ...rest } = query;
  void _ignored;
  if (ctx.crossTenant && ctx.filter.length > 0) return { ...rest, tenant_ids: [...ctx.filter] };
  return rest;
}

export type SingleTenantResult = { ready: true; headers: Record<string, string> } | { ready: false };

/** F051-AC-24: endpoints that only take X-Tenant-Id. Tenant users are always ready (session header). */
export function requireSingleTenant(ctx: TenantScopeContext): SingleTenantResult {
  if (!ctx.crossTenant) return { ready: true, headers: {} };
  return ctx.filter.length === 1 ? { ready: true, headers: { [TENANT_HEADER]: ctx.filter[0]! } } : { ready: false };
}

export const PLATFORM_TARGET = 'platform';

/** F051-AC-25: headers of a System Admin write for the target chosen in TenantTargetField. */
export function targetTenantHeaders(target: string): Record<string, string> {
  if (!target) throw new Error('target tenant required');
  return target === PLATFORM_TARGET ? {} : { [TENANT_HEADER]: target };
}
