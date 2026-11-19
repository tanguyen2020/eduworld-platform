// tenant: X-Tenant-Id by operation metadata (ADR-0065, F050-AC-11, F051-AC-05/06/23–26):
//   `none` → never sent;
//   session in a tenant (tenant staff) → always the working tenant, whatever the caller set (one tenant per session);
//   platform scope (no working tenant) → only the target tenant the caller set explicitly (System Admin write, or a
//   screen that only accepts the header — F051-AC-24/25); otherwise none (cross-tenant read with `tenant_ids[]`).
// `tenant_ids` is for System Admin only: it is stripped from the query whenever the session has a working tenant
// (F051-AC-26 — tenant users never send it).
import type { TenantHeaderMode } from '../types';

export const TENANT_HEADER = 'X-Tenant-Id';
export const TENANT_IDS_PARAMS = ['tenant_ids', 'tenant_ids[]'] as const;

export function applyTenant(headers: Headers, mode: TenantHeaderMode, tenantId: string | null): void {
  if (mode === 'none') {
    headers.delete(TENANT_HEADER);
    return;
  }
  if (tenantId) {
    headers.set(TENANT_HEADER, tenantId);
    return;
  }
  // platform scope: keep the explicit target tenant of the caller (System Admin), nothing otherwise
  if (!headers.get(TENANT_HEADER)) headers.delete(TENANT_HEADER);
}

/** Query string without `tenant_ids` when the session works in one tenant. */
export function scopeSearch(search: string, tenantId: string | null): string {
  if (!tenantId || !search) return search;
  const params = new URLSearchParams(search);
  for (const p of TENANT_IDS_PARAMS) params.delete(p);
  const s = params.toString();
  return s ? `?${s}` : '';
}
