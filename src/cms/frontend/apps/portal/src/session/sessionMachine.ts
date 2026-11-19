// sessionMachine (F051 T018, data-model §2): portal session states and the post-login routing rule of
// ADMIN_PORTAL §1.3b as pure functions (no API call, no storage). The backend stays the authority on access.
import type { components } from '../shared/api/schema.gen';

export type MyTenant = components['schemas']['MyTenant'];
export type MyTenantsResponse = components['schemas']['MyTenantsResponse'];
export type TenantType = components['schemas']['TenantType'];

export type Scope = { kind: 'platform' } | { kind: 'tenant'; tenantId: string; code: string; name: string; tenantType: TenantType };

export type SessionStatus =
  | 'restoring'
  | 'unauthenticated'
  | 'resolving_scope'
  | 'scope_error'
  | 'selecting_tenant'
  | 'ready'
  | 'no_access'
  | 'expired';

export type NoAccessReason = 'no_role' | 'student';

export interface SessionState {
  status: SessionStatus;
  scope: Scope | null;
  tenants: MyTenant[];
  hasPlatformRole: boolean;
  /** Tenant to pre-select on the picker (remembered or session tenant still valid). */
  preselectedTenantId: string | null;
  noAccessReason: NoAccessReason | null;
}

export const initialSession: SessionState = {
  status: 'restoring',
  scope: null,
  tenants: [],
  hasPlatformRole: false,
  preselectedTenantId: null,
  noAccessReason: null,
};

export type ScopeDecision =
  | { kind: 'platform' }
  | { kind: 'tenant'; tenant: MyTenant }
  | { kind: 'select'; preselectedTenantId: string | null }
  | { kind: 'no_access' };

export const tenantScope = (t: MyTenant): Scope => ({ kind: 'tenant', tenantId: t.tenant_id, code: t.code, name: t.name, tenantType: t.tenant_type });

/**
 * F051-AC-03: `has_platform_role` → platform scope; exactly one tenant → that tenant; several → picker (pre-select the
 * tab's session tenant, else the remembered one, only if still listed); none and no platform role → no access.
 * `sessionTenantId` (F5 restore, AC-28) re-enters its tenant directly when it is still listed.
 */
export function routeAfterLogin(res: MyTenantsResponse, remembered: string | null, sessionTenantId: string | null = null): ScopeDecision {
  if (res.has_platform_role) return { kind: 'platform' };
  const items = res.items.filter((t) => t.status !== 'disabled');
  if (items.length === 0) return { kind: 'no_access' };
  if (items.length === 1) return { kind: 'tenant', tenant: items[0]! };
  const fromSession = sessionTenantId ? items.find((t) => t.tenant_id === sessionTenantId) : undefined;
  if (fromSession) return { kind: 'tenant', tenant: fromSession };
  const pre = remembered && items.some((t) => t.tenant_id === remembered) ? remembered : null;
  return { kind: 'select', preselectedTenantId: pre };
}

export type SessionAction =
  | { type: 'restore_failed' }
  | { type: 'authenticated' }
  | { type: 'tenants_loaded'; response: MyTenantsResponse; decision: ScopeDecision }
  | { type: 'tenants_failed' }
  | { type: 'tenant_selected'; tenant: MyTenant }
  | { type: 'switch_tenant' }
  | { type: 'tenant_access_changed' }
  | { type: 'staff_only' }
  | { type: 'no_workspace' }
  | { type: 'expired' }
  | { type: 'signed_out' };

export function sessionReducer(state: SessionState, action: SessionAction): SessionState {
  switch (action.type) {
    case 'restore_failed':
    case 'signed_out':
      return { ...initialSession, status: 'unauthenticated' };
    case 'authenticated':
      return { ...initialSession, status: 'resolving_scope' };
    case 'tenants_loaded': {
      const base = { ...state, tenants: action.response.items, hasPlatformRole: action.response.has_platform_role, noAccessReason: null };
      const d = action.decision;
      if (d.kind === 'platform') return { ...base, status: 'ready', scope: { kind: 'platform' }, preselectedTenantId: null };
      if (d.kind === 'tenant') return { ...base, status: 'ready', scope: tenantScope(d.tenant), preselectedTenantId: null };
      if (d.kind === 'select') return { ...base, status: 'selecting_tenant', scope: null, preselectedTenantId: d.preselectedTenantId };
      return { ...base, status: 'no_access', scope: null, noAccessReason: 'no_role' };
    }
    case 'tenants_failed':
      return { ...state, status: 'scope_error', scope: null };
    case 'tenant_selected':
      return { ...state, status: 'ready', scope: tenantScope(action.tenant), preselectedTenantId: null };
    case 'switch_tenant':
      return { ...state, status: 'selecting_tenant', preselectedTenantId: state.scope?.kind === 'tenant' ? state.scope.tenantId : null, scope: null };
    case 'tenant_access_changed':
      return { ...state, status: 'resolving_scope', scope: null };
    case 'staff_only':
      return { ...initialSession, status: 'no_access', noAccessReason: 'student' };
    case 'no_workspace':
      return { ...state, status: 'no_access', noAccessReason: 'no_role' };
    case 'expired':
      return state.status === 'unauthenticated' ? state : { ...initialSession, status: 'expired' };
    default:
      return state;
  }
}

/** Query-key segment of the session scope (data-model: every query key contains it). */
export function scopeKey(scope: Scope | null): string {
  if (!scope) return 'none';
  return scope.kind === 'platform' ? 'platform' : scope.tenantId;
}
