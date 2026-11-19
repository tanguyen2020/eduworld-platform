// tenant: X-Tenant-Id by operation metadata (ADR-0065, F050-AC-11): `required` → always the working tenant,
// `optional` → only when a working tenant exists (System Admin cross-tenant reads send none), `none` → never.
import type { TenantHeaderMode } from '../types';

export const TENANT_HEADER = 'X-Tenant-Id';

export function applyTenant(headers: Headers, mode: TenantHeaderMode, tenantId: string | null): void {
  if (mode === 'none' || !tenantId) {
    headers.delete(TENANT_HEADER);
    return;
  }
  headers.set(TENANT_HEADER, tenantId);
}
