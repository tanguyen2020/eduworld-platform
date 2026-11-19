// Permission catalog helpers (F059 T011, F059-AC-01/03, ADMIN_PORTAL §2.2): pure display functions over the
// `listAllPermissions` catalog — group by `group_code`, filter by `scope_kind` and code text, and the roles that hold
// each permission. They only shape what is shown: the backend decides which permission a role may hold (422
// PERMISSION_NOT_ALLOWED_FOR_SCOPE, ADR-0066). Shared by workspace D (F059) and workspace C (F052).
import type { components } from '../api/schema.gen';

export type Permission = components['schemas']['Permission'];
export type PermissionScopeKind = components['schemas']['PermissionScopeKind'];

export interface PermissionGroup {
  groupCode: string;
  items: Permission[];
}

export interface PermissionFilter {
  scopeKind?: PermissionScopeKind;
  /** Part of the code (case-insensitive). */
  text?: string;
}

export function filterPermissions(items: readonly Permission[], f: PermissionFilter = {}): Permission[] {
  const text = f.text?.trim().toLowerCase() ?? '';
  return items.filter((p) => (!f.scopeKind || p.scope_kind === f.scopeKind) && (!text || p.code.toLowerCase().includes(text)));
}

/** Groups by `group_code` (groups and codes sorted), after the filter. */
export function groupPermissions(items: readonly Permission[], f: PermissionFilter = {}): PermissionGroup[] {
  const groups = new Map<string, Permission[]>();
  for (const p of filterPermissions(items, f)) {
    const list = groups.get(p.group_code) ?? [];
    list.push(p);
    groups.set(p.group_code, list);
  }
  return [...groups.entries()]
    .sort(([a], [b]) => a.localeCompare(b))
    .map(([groupCode, list]) => ({ groupCode, items: [...list].sort((a, b) => a.code.localeCompare(b.code)) }));
}

/** Permission code → codes of the roles (of the loaded page) that contain it. */
export function rolesByPermission(roles: readonly { code: string; permission_codes: readonly string[] }[]): Map<string, string[]> {
  const out = new Map<string, string[]>();
  for (const r of roles) {
    for (const c of r.permission_codes) {
      const list = out.get(c) ?? [];
      if (!list.includes(r.code)) list.push(r.code);
      out.set(c, list);
    }
  }
  for (const list of out.values()) list.sort();
  return out;
}

/** Picker offer of a role: only permissions whose `scope_kind` equals the role's `role_scope_kind` (F059-AC-03). */
export const pickablePermissions = (items: readonly Permission[], roleScopeKind: PermissionScopeKind): Permission[] =>
  filterPermissions(items, { scopeKind: roleScopeKind });
