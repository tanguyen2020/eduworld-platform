// @vitest-environment node
// F059 T011 — pure display helpers: permission catalog grouping/filter (F059-AC-01), picker offer by scope_kind
// (F059-AC-03), what the grant dialog offers per account kind (F059-AC-07), validity range (F059-AC-08), end-reason
// labels (F059-AC-18), audit URL filter defaults (F059-AC-15/16), rejected permissions of a 422 (F059-AC-04).
import dayjs from 'dayjs';
import { describe, expect, it } from 'vitest';
import { ApiError } from '../api/middleware/problem';
import { filterFromParams, isPermissionAction, PERMISSION_AUDIT_ACTIONS } from '../../workspaces/platform-admin/api/audit';
import { endReasonKey } from '../../workspaces/platform-admin/api/loginEvents';
import { rejectedPermissions } from '../../workspaces/platform-admin/api/roles';
import { accountKindOf, canHavePortalRole, grantableRoles, grantableScopes, scopeNeedsId, validRange } from '../../workspaces/platform-admin/user-roles/grantableRoles';
import { groupByTenant } from '../../workspaces/platform-admin/api/userRoles';
import { filterPermissions, groupPermissions, pickablePermissions, rolesByPermission, type Permission } from './groupPermissions';

const p = (code: string, scope_kind: 'platform' | 'tenant', group_code: string): Permission => ({ code, scope_kind, group_code, name: { vi: code, en: code } });
const CATALOG = [p('platform.role.write', 'platform', 'access'), p('game.quest.write', 'platform', 'game'), p('platform.role.read', 'platform', 'access'), p('lms.class.read', 'tenant', 'lms')];

describe('permission catalog (F059-AC-01/03)', () => {
  it('groups by group_code, sorted, after filter', () => {
    expect(groupPermissions(CATALOG).map((g) => [g.groupCode, g.items.map((x) => x.code)])).toEqual([
      ['access', ['platform.role.read', 'platform.role.write']],
      ['game', ['game.quest.write']],
      ['lms', ['lms.class.read']],
    ]);
    expect(groupPermissions(CATALOG, { scopeKind: 'tenant' }).map((g) => g.groupCode)).toEqual(['lms']);
    expect(filterPermissions(CATALOG, { text: 'ROLE.R' }).map((x) => x.code)).toEqual(['platform.role.read']);
  });

  it('a tenant role is never offered platform.*, game.* permissions', () => {
    const offered = pickablePermissions(CATALOG, 'tenant').map((x) => x.code);
    expect(offered).toEqual(['lms.class.read']);
    expect(offered.some((c) => /^(platform|game|competition\.config)\./.test(c))).toBe(false);
  });

  it('roles holding each permission', () => {
    const m = rolesByPermission([
      { code: 'b_role', permission_codes: ['platform.role.read'] },
      { code: 'a_role', permission_codes: ['platform.role.read', 'game.quest.write'] },
    ]);
    expect(m.get('platform.role.read')).toEqual(['a_role', 'b_role']);
    expect(m.get('game.quest.write')).toEqual(['a_role']);
  });

  it('422 PERMISSION_NOT_ALLOWED_FOR_SCOPE → the rejected codes', () => {
    const e = new ApiError({ status: 422, code: 'VALIDATION_FAILED', i18nKey: 'error.VALIDATION_FAILED', fieldErrors: { 'permission_codes[1]': { code: 'PERMISSION_NOT_ALLOWED_FOR_SCOPE' } } as never });
    expect(rejectedPermissions(e, ['lms.class.read', 'game.quest.write'])).toEqual(['game.quest.write']);
    expect(rejectedPermissions(new Error('x'), ['a'])).toEqual([]);
  });
});

describe('grant dialog offer (F059-AC-07/08)', () => {
  const roles = [
    { role_scope_kind: 'platform' as const, tenant_id: null, code: 'liveops' },
    { role_scope_kind: 'tenant' as const, tenant_id: null, code: 'tenant_admin' },
    { role_scope_kind: 'tenant' as const, tenant_id: 'tA', code: 'custom_a' },
    { role_scope_kind: 'tenant' as const, tenant_id: 'tB', code: 'custom_b' },
  ];

  it('platform staff → platform roles at scope platform; staff with a home tenant → tenant roles of the target', () => {
    expect(accountKindOf({ primary_tenant_id: null })).toBe('platform');
    expect(accountKindOf({ primary_tenant_id: 'tA' })).toBe('tenant');
    expect(grantableRoles(roles, 'platform').map((r) => r.code)).toEqual(['liveops']);
    expect(grantableRoles(roles, 'tenant', 'tA').map((r) => r.code)).toEqual(['tenant_admin', 'custom_a']);
    expect(grantableScopes('platform')).toEqual(['platform']);
    expect(grantableScopes('tenant')).toEqual(['tenant', 'school', 'subject']);
    expect(grantableScopes('tenant')).not.toContain('class');
    expect(scopeNeedsId('school') && scopeNeedsId('subject') && !scopeNeedsId('tenant')).toBe(true);
  });

  it('students never get a portal role; valid_to after valid_from', () => {
    expect(canHavePortalRole({ user_type: 'student' })).toBe(false);
    expect(canHavePortalRole({ user_type: 'staff' })).toBe(true);
    expect(validRange('2026-11-01T00:00:00Z', '2026-12-01T00:00:00Z')).toBe(true);
    expect(validRange('2026-12-01T00:00:00Z', '2026-11-01T00:00:00Z')).toBe(false);
    expect(validRange(undefined, '2026-11-01T00:00:00Z')).toBe(true);
  });

  it('UserRoles grouped by tenant, platform first', () => {
    const rows = [{ tenant_id: 'tB' }, { tenant_id: null }, { tenant_id: 'tA' }] as never[];
    expect(groupByTenant(rows).map(([k]) => k)).toEqual([null, 'tA', 'tB']);
  });
});

describe('audit and login labels (F059-AC-15/16/18)', () => {
  it('audit filter defaults to the last 24 hours; the permission log only asks role.* / user_role.* actions', () => {
    const now = dayjs('2026-11-19T12:00:00Z');
    const f = filterFromParams(new URLSearchParams(''), 'system', now);
    expect(f.from).toBe('2026-11-18T12:00:00.000Z');
    expect(f.to).toBe('2026-11-19T12:00:00.000Z');
    expect(filterFromParams(new URLSearchParams('action=tenant.created'), 'permissions', now).action).toBe('role.created');
    expect(filterFromParams(new URLSearchParams('action=user_role.revoked'), 'permissions', now).action).toBe('user_role.revoked');
    expect(PERMISSION_AUDIT_ACTIONS.every(isPermissionAction)).toBe(true);
  });

  it('end_reason → Vietnamese label key; open session', () => {
    expect(endReasonKey('replaced_by_other_device')).toBe('platform.loginEvents.endReason.replaced_by_other_device');
    expect(endReasonKey(null)).toBe('platform.loginEvents.endReason.open');
  });
});
