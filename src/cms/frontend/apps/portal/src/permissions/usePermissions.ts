// usePermissions (F051 T033): the effective permission view of the session for menus, ActionGate and the System
// Admin tenant filter. Display only — the backend checks every request (AGENTS R20, ADR-0027).
import { useMyPermissions } from '../shared/api/hooks/useMyPermissions';
import type { PermissionView } from './menuRegistry';

/** Only `system_admin` holds it (ADMIN_PORTAL §2.1 mục 4): tenant filter, `tenant_ids[]`, Tenant column. */
export const CROSS_TENANT_PERMISSION = 'platform.cross_tenant.read';

export function canReadAcrossTenants(p: PermissionView | undefined): boolean {
  return !!p?.permissions.includes(CROSS_TENANT_PERMISSION);
}

export function usePermissions() {
  const q = useMyPermissions();
  const view: PermissionView | undefined = q.data;
  return { view, isLoading: q.isPending, error: q.error, refetch: q.refetch, crossTenant: canReadAcrossTenants(view) };
}
