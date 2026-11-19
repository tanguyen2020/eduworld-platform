// menuRegistry (F051 T010, F051-AC-09/35/36, ADR-0090): pure functions over the catalog generated from
// specs/portal/menu-catalog.yaml (src/generated/menu-catalog.ts). There is NO register API: menus are never declared
// by hand. Hiding is a display convenience only — the backend blocks with authz + featuregate (AGENTS R1, R20).
import { WORKSPACE_CATALOG, type ActionDef, type ActionKey, type MenuDef, type MenuKey, type WorkspaceDef, type WorkspaceKey } from '../generated/menu-catalog';

/** The part of `PermissionSet` (getMyPermissions) the shell reads. */
export interface PermissionView {
  permissions: readonly string[];
  workspaces?: readonly string[];
  disabled_menus?: readonly string[];
  disabled_actions?: readonly string[];
}

export type MenuStatus = 'visible' | 'forbidden' | 'disabled';
export type ActionStatus = 'enabled' | 'forbidden' | 'disabled';

const MENUS = new Map<MenuKey, { menu: MenuDef; workspace: WorkspaceDef }>();
const ACTIONS = new Map<ActionKey, { action: ActionDef; menu: MenuDef; workspace: WorkspaceDef }>();
for (const workspace of WORKSPACE_CATALOG) {
  for (const menu of workspace.menus) {
    MENUS.set(menu.key, { menu, workspace });
    for (const action of menu.actions) ACTIONS.set(action.key, { action, menu, workspace });
  }
}

/** any-of: the user holds at least one of the required permissions (ADR-0090 mục 1). */
export function hasAnyPermission(required: readonly string[], granted: readonly string[]): boolean {
  return required.some((p) => granted.includes(p));
}

const workspaceGranted = (ws: WorkspaceKey, p: PermissionView) => (p.workspaces ?? []).includes(ws);

/**
 * Status of a menu: `forbidden` when its workspace is not in `workspaces[]` or no required permission is held;
 * `disabled` when allowed but in `disabled_menus[]` (→ "Tính năng đang tắt" page, F051-AC-35); else `visible`.
 */
export function menuStatus(key: MenuKey, p: PermissionView): MenuStatus {
  const entry = MENUS.get(key);
  if (!entry) return 'forbidden';
  if (!workspaceGranted(entry.workspace.key, p) || !hasAnyPermission(entry.menu.requiredPermissions, p.permissions)) return 'forbidden';
  return (p.disabled_menus ?? []).includes(key) ? 'disabled' : 'visible';
}

/** Visible menus of a workspace, in catalog order (F051-AC-09, F051-AC-35). */
export function visibleMenus(ws: WorkspaceDef, p: PermissionView): MenuDef[] {
  return ws.menus.filter((m) => menuStatus(m.key, p) === 'visible');
}

/** Workspaces in `workspaces[]` that still have at least one visible menu (an empty workspace is hidden). */
export function visibleWorkspaces(p: PermissionView, catalog: readonly WorkspaceDef[] = WORKSPACE_CATALOG): WorkspaceDef[] {
  return catalog.filter((ws) => workspaceGranted(ws.key, p) && visibleMenus(ws, p).length > 0);
}

/** Status of an action (F051-AC-36): any-of permission and not in `disabled_actions[]` (includes menu parent off). */
export function actionStatus(key: ActionKey, p: PermissionView): ActionStatus {
  const entry = ACTIONS.get(key);
  if (!entry) return 'forbidden';
  if (!workspaceGranted(entry.workspace.key, p) || !hasAnyPermission(entry.action.requiredPermissions, p.permissions)) return 'forbidden';
  const disabled = (p.disabled_actions ?? []).includes(key) || (p.disabled_menus ?? []).includes(entry.menu.key);
  return disabled ? 'disabled' : 'enabled';
}

export function workspaceByKey(key: WorkspaceKey): WorkspaceDef {
  return WORKSPACE_CATALOG.find((w) => w.key === key)!;
}

export function menuByKey(key: MenuKey): MenuDef | undefined {
  return MENUS.get(key)?.menu;
}

/** Workspace whose catalog `path` prefixes the URL path. */
export function workspaceForPath(pathname: string): WorkspaceDef | undefined {
  return WORKSPACE_CATALOG.find((w) => pathname === w.path || pathname.startsWith(`${w.path}/`));
}

/** Menu whose `path` matches the URL path or one of its sub-paths. */
export function menuForPath(pathname: string): MenuDef | undefined {
  const ws = workspaceForPath(pathname);
  return ws?.menus.find((m) => pathname === m.path || pathname.startsWith(`${m.path}/`));
}

export const ALL_MENU_KEYS: readonly MenuKey[] = [...MENUS.keys()];
export const ALL_ACTION_KEYS: readonly ActionKey[] = [...ACTIONS.keys()];
