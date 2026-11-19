// What the grant dialog offers (F059 T011, F059-AC-07, ADR-0081, ADR-0072): pure display narrowing by the kind of
// account — platform staff (`primary_tenant_id = null`) → `platform` roles with scope `platform`, no X-Tenant-Id;
// staff with a home tenant → `tenant` roles (shared + custom roles of the target tenant) with scope `tenant`, `school`
// or `subject` and X-Tenant-Id = target tenant; students never get a portal role. The backend re-checks every grant
// (422 for a platform role at tenant scope, a portal role for a student, a `class` scope, ROLE_NOT_FOUND).
import type { Role } from '../api/roles';
import type { User, UserRoleScopeType } from '../api/userRoles';

export type AccountKind = 'platform' | 'tenant';

/** Role code that needs a training date (F012-AC-24, ADR-0072, CHILD_SAFETY). */
export const TRAINING_REQUIRED_ROLE = 'safety_moderator';

export const accountKindOf = (user: Pick<User, 'primary_tenant_id'>): AccountKind => (user.primary_tenant_id ? 'tenant' : 'platform');

export function canHavePortalRole(user: Pick<User, 'user_type'> | undefined): boolean {
  return user?.user_type !== 'student';
}

export function grantableRoles<R extends Pick<Role, 'role_scope_kind' | 'tenant_id'>>(roles: readonly R[], kind: AccountKind, targetTenantId?: string): R[] {
  if (kind === 'platform') return roles.filter((r) => r.role_scope_kind === 'platform' && !r.tenant_id);
  return roles.filter((r) => r.role_scope_kind === 'tenant' && (!r.tenant_id || r.tenant_id === targetTenantId));
}

export function grantableScopes(kind: AccountKind): readonly UserRoleScopeType[] {
  return kind === 'platform' ? ['platform'] : ['tenant', 'school', 'subject'];
}

/** `scope_id` is needed for `school` (school id) and `subject` (subject id); none for `platform`/`tenant`. */
export const scopeNeedsId = (scope: UserRoleScopeType): boolean => scope === 'school' || scope === 'subject';

/** `valid_to` must be after `valid_from` when both are set (form check; the server validates too). */
export function validRange(from?: string | null, to?: string | null): boolean {
  return !from || !to || new Date(to).getTime() > new Date(from).getTime();
}
