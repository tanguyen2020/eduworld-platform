// F051 T054 — F051-AC-34 (TC-CMS-MENU-05): the generator turns the catalog YAML into typed unions and the menu tree
// sorted by `order`, drops server-only fields, and the committed file matches the YAML (gen:check fails otherwise).
import fs from 'node:fs';
import { describe, expect, it } from 'vitest';
import { buildCatalogSource, generate, OUT } from './gen-menu-catalog.mjs';

const tiny = {
  catalog_version: '2026.11.19.1',
  workspaces: [
    {
      key: 'live_ops',
      label_key: 'portal.workspace.live_ops',
      title: 'E',
      order: 5,
      path: '/live-ops',
      menus: [
        { key: 'menu.live_ops.b', label_key: 'portal.menu.live_ops.b', title: 'B', path: '/live-ops/b', order: 20, required_permissions: ['x.y'], can_disable: true, actions: [{ key: 'action.live_ops.b.view', label_key: 'portal.action.live_ops.b.view', title: 'v', required_permissions: ['x.y'], operation_ids: ['opB'], can_disable: true }] },
        { key: 'menu.live_ops.a', label_key: 'portal.menu.live_ops.a', title: 'A', path: '/live-ops/a', order: 10, required_permissions: ['x.z', 'x.y'], can_disable: false, actions: [{ key: 'action.live_ops.a.view', label_key: 'portal.action.live_ops.a.view', title: 'v', required_permissions: ['x.z'], operation_ids: [], can_disable: false }] },
      ],
    },
  ],
};

describe('gen-menu-catalog', () => {
  it('emits unions and the tree in catalog order without server-only fields', () => {
    const src = buildCatalogSource(tiny);
    expect(src).toContain('export type MenuKey =\n  | "menu.live_ops.a"\n  | "menu.live_ops.b";');
    expect(src).toContain('export type ActionKey =\n  | "action.live_ops.a.view"\n  | "action.live_ops.b.view";');
    expect(src).toContain('requiredPermissions: ["x.z", "x.y"]');
    expect(src).not.toContain('opB');
    expect(src).not.toMatch(/canDisable|can_disable:|operationIds/);
    expect(src).toContain('DO NOT EDIT');
  });

  it('committed src/generated/menu-catalog.ts matches specs/portal/menu-catalog.yaml', () => {
    expect(fs.readFileSync(OUT, 'utf8').replace(/\r\n/g, '\n')).toBe(generate());
  });
});
