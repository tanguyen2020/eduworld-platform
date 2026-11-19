// Display helpers of UserRole assignment (F052 T035, F052-AC-14, F052-AC-16, ADR-0072) — pure, unit-tested. They only
// shape the UI (which roles/scopes are offered, which endpoint the dialog calls); the server decides and answers
// 403 PERMISSION_DENIED / 422 ROLE_NOT_FOUND when called directly (edge case "School Admin vượt quyền").
import type { Schemas } from '../common/api';

export type Role = Schemas['Role'];
export type Permission = Schemas['Permission'];
export type EffectiveRole = Schemas['EffectiveRole'];

/** Scopes a tenant UserRole can have in the portal: Đơn vị / Trường / Môn — never Lớp (ADR-0072). */
export const GRANT_SCOPES = ['tenant', 'school', 'subject'] as const;
export type GrantScope = (typeof GRANT_SCOPES)[number];

/** Roles the granter may hand out: tenant roles whose permissions the granter holds themselves. */
export function grantableRoles(roles: readonly Role[], granterPermissions: readonly string[]): Role[] {
  const held = new Set(granterPermissions);
  return roles.filter((r) => r.role_scope_kind === 'tenant' && r.permission_codes.every((p) => held.has(p)));
}

export type GrantMode = { kind: 'tenant' } | { kind: 'school'; schoolIds: string[] };

/**
 * Endpoint of a grant: a granter holding a tenant-wide role grants with grantTenantUserRole; a School Admin (roles scoped
 * to schools) with grantSchoolUserRole in their school(s) and may not offer the "Đơn vị" scope.
 */
export function grantMode(roles: readonly EffectiveRole[] | undefined): GrantMode {
  const list = roles ?? [];
  if (list.some((r) => r.scope_type === 'tenant' || r.scope_type === 'platform')) return { kind: 'tenant' };
  const schoolIds = [...new Set(list.filter((r) => r.scope_type === 'school' && r.scope_id).map((r) => r.scope_id!))];
  // no scoped role known (e.g. roles not returned): the tenant route; the server still checks the scope
  return schoolIds.length ? { kind: 'school', schoolIds } : { kind: 'tenant' };
}

export function scopesFor(mode: GrantMode): GrantScope[] {
  return mode.kind === 'tenant' ? [...GRANT_SCOPES] : ['school', 'subject'];
}

/** Permission picker groups: tenant permissions only, grouped by `group_code`; empty groups are not shown. */
export function groupPermissions(perms: readonly Permission[]): { group: string; items: Permission[] }[] {
  const groups = new Map<string, Permission[]>();
  for (const p of perms) {
    if (p.scope_kind !== 'tenant') continue;
    const g = p.group_code || p.code.split('.')[0]!;
    groups.set(g, [...(groups.get(g) ?? []), p]);
  }
  return [...groups.entries()].filter(([, items]) => items.length > 0).map(([group, items]) => ({ group, items }));
}
