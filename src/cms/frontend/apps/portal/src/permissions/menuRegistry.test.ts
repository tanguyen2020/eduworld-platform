// F051 T024 — F051-AC-09, F051-AC-35, F051-AC-36 (TC-CMS-AUTHZ-06, TC-CMS-MENU-06): menus generated from the catalog
// are filtered by `workspaces[]`, `permissions[]` (any-of) and `disabled_menus[]`; an empty workspace is hidden;
// actions by permission and `disabled_actions[]` (or menu parent off).
import { describe, expect, it } from 'vitest';
import { WORKSPACE_CATALOG } from '../generated/menu-catalog';
import { actionStatus, hasAnyPermission, menuForPath, menuStatus, visibleMenus, visibleWorkspaces, workspaceByKey, workspaceForPath } from './menuRegistry';

const school = workspaceByKey('school_lms');

describe('menuRegistry', () => {
  it('any-of permissions', () => {
    expect(hasAnyPermission(['a.b', 'c.d'], ['c.d'])).toBe(true);
    expect(hasAnyPermission(['a.b'], ['x.y'])).toBe(false);
  });

  it('a menu needs its workspace in workspaces[] and one required permission (F051-AC-09)', () => {
    expect(menuStatus('menu.school_lms.schools', { permissions: ['tenant.school.read'], workspaces: ['school_lms'] })).toBe('visible');
    expect(menuStatus('menu.school_lms.schools', { permissions: ['tenant.school.read'], workspaces: [] })).toBe('forbidden');
    expect(menuStatus('menu.school_lms.schools', { permissions: ['lms.student.read'], workspaces: ['school_lms'] })).toBe('forbidden');
    // any-of: classes accepts tenant.student.read OR lms.student.read
    expect(menuStatus('menu.school_lms.classes', { permissions: ['lms.student.read'], workspaces: ['school_lms'] })).toBe('visible');
  });

  it('a turned-off menu is `disabled` (not visible, not 403) (F051-AC-35)', () => {
    const p = { permissions: ['tenant.school.read', 'lms.student.read'], workspaces: ['school_lms'], disabled_menus: ['menu.school_lms.schools'] };
    expect(menuStatus('menu.school_lms.schools', p)).toBe('disabled');
    expect(visibleMenus(school, p).some((m) => m.key === 'menu.school_lms.schools')).toBe(false);
    expect(visibleMenus(school, p).some((m) => m.key === 'menu.school_lms.classes')).toBe(true);
  });

  it('menus keep the catalog order', () => {
    const all = { permissions: WORKSPACE_CATALOG.flatMap((w) => w.menus.flatMap((m) => m.requiredPermissions)), workspaces: ['school_lms'] };
    const orders = visibleMenus(school, all).map((m) => m.order);
    expect(orders).toEqual([...orders].sort((a, b) => a - b));
    expect(visibleMenus(school, all)).toHaveLength(school.menus.length);
  });

  it('a workspace without any visible menu is hidden (F051-AC-35)', () => {
    const onlySchoolsOff = { permissions: ['tenant.school.read'], workspaces: ['school_lms', 'game_ops'], disabled_menus: ['menu.school_lms.schools'] };
    expect(visibleWorkspaces(onlySchoolsOff)).toEqual([]);
    expect(visibleWorkspaces({ permissions: ['tenant.school.read'], workspaces: ['school_lms'] }).map((w) => w.key)).toEqual(['school_lms']);
  });

  it('actions: permission any-of, disabled_actions or menu parent off (F051-AC-36)', () => {
    const p = { permissions: ['tenant.school.read', 'tenant.school.write'], workspaces: ['school_lms'] };
    expect(actionStatus('action.school_lms.schools.edit', p)).toBe('enabled');
    expect(actionStatus('action.school_lms.schools.edit', { ...p, permissions: ['tenant.school.read'] })).toBe('forbidden');
    expect(actionStatus('action.school_lms.schools.edit', { ...p, disabled_actions: ['action.school_lms.schools.edit'] })).toBe('disabled');
    expect(actionStatus('action.school_lms.schools.view', { ...p, disabled_menus: ['menu.school_lms.schools'] })).toBe('disabled');
  });

  it('path lookups follow catalog paths', () => {
    expect(workspaceForPath('/school-lms/classes/123')?.key).toBe('school_lms');
    expect(menuForPath('/school-lms/classes/123')?.key).toBe('menu.school_lms.classes');
    expect(workspaceForPath('/school-lmsx')).toBeUndefined();
  });
});
