// F051 T060, T025 — F051-AC-12, F051-AC-34, F051-AC-35, F051-AC-36, F051-AC-37 (TC-CMS-MENU-02/05/06,
// TC-CMS-RBAC-03): generated catalog matches specs/portal/menu-catalog.yaml and every menu key has a screen; menus
// follow permissions/disabled_menus; a turned-off menu URL shows "Tính năng đang tắt" (not 403) without its screen;
// ActionGate hides/disables; 403 FEATURE_DISABLED → notice + permissions reloaded; permissions refresh every
// principal_cache_ttl, on focus and after 403 PERMISSION_DENIED. Backend = MOCK harness.
import { OpsThemeProvider } from '@eduworld/ops-ui';
import { QueryClientProvider } from '@tanstack/react-query';
import { render, screen, waitFor, within } from '@testing-library/react';
import { Button } from 'antd';
import fs from 'node:fs';
import path from 'node:path';
import { I18nextProvider } from 'react-i18next';
import { afterEach, describe, expect, it } from 'vitest';
import { createPortalRuntime, PortalRuntimeContext } from '../app/runtime';
import { WORKSPACE_CATALOG } from '../generated/menu-catalog';
import { createI18n } from '../i18n';
import { permissionsQueryTiming } from '../shared/api/hooks/useMyPermissions';
import { TEST_CONFIG } from '../shared/api/test-utils';
import { MockPortalBackend, problem, renderPortal, stopPortals } from '../test/portal-harness';
import { ActionGate } from './ActionGate';
import { generate } from '../../scripts/gen-menu-catalog.mjs';

afterEach(() => stopPortals());

const WORKSPACES_DIR = path.resolve(__dirname, '../workspaces');

describe('catalog generation (F051-AC-34)', () => {
  it('src/generated/menu-catalog.ts is exactly what pnpm gen:catalog produces from the YAML', () => {
    const committed = fs.readFileSync(path.resolve(__dirname, '../generated/menu-catalog.ts'), 'utf8').replace(/\r\n/g, '\n');
    expect(committed).toBe(generate());
  });

  it('every menu key of the catalog has a screen in its workspace (and nothing else)', async () => {
    for (const ws of WORKSPACE_CATALOG) {
      const mod = (await import(/* @vite-ignore */ path.join(WORKSPACES_DIR, ws.path.slice(1), 'index.tsx'))) as { screens: Record<string, unknown> };
      expect(Object.keys(mod.screens).sort(), ws.key).toEqual(ws.menus.map((m) => m.key).sort());
    }
  });

  it('no menu path, label or permission is declared by hand in TSX', () => {
    const offenders: string[] = [];
    const walk = (dir: string) => {
      for (const e of fs.readdirSync(dir, { withFileTypes: true })) {
        const p = path.join(dir, e.name);
        if (e.isDirectory()) {
          if (!['generated', 'test'].includes(e.name)) walk(p);
        } else if (/\.tsx?$/.test(e.name) && !/\.test\.tsx?$/.test(e.name) && !e.name.endsWith('.stories.tsx')) {
          const src = fs.readFileSync(p, 'utf8');
          if (/['"`]\/(school-lms|content-studio|game-ops|live-ops|platform-admin)\/[a-z]/.test(src) || /['"]portal\.menu\./.test(src)) offenders.push(path.relative(dir, p));
        }
      }
    };
    walk(path.resolve(__dirname, '..'));
    expect(offenders).toEqual([]);
  });
});

describe('menus from getMyPermissions (F051-AC-09, F051-AC-35)', () => {
  it('turned-off menu: hidden in the sidebar, its URL shows "Tính năng đang tắt" (not 403) and no screen', async () => {
    const backend = new MockPortalBackend().withSession('ad.truong.a');
    backend.disabledMenus = ['menu.school_lms.assignments'];
    const { router } = await renderPortal({ url: '/school-lms/overview', backend });
    const nav = await screen.findByRole('navigation', { name: 'Menu điều hướng' });
    expect(within(nav).queryByText('Bài tập, đề, bài chẩn đoán')).toBeNull();
    expect(within(nav).getByText('Lớp & học sinh')).toBeInTheDocument();
    await router.navigate('/school-lms/assignments');
    expect(await screen.findByTestId('feature-disabled')).toBeInTheDocument();
    expect(screen.queryByText('Không có quyền truy cập')).toBeNull();
    expect(screen.queryByRole('heading', { level: 1, name: 'Bài tập, đề, bài chẩn đoán' })).toBeNull();
  });

  it('403 FEATURE_DISABLED → notice with feature key and trace id, no sign-out, permissions reloaded (F051-AC-37)', async () => {
    const backend = new MockPortalBackend().withSession('ad.truong.a');
    const { runtime } = await renderPortal({ url: '/school-lms/classes', backend });
    await screen.findByRole('heading', { level: 1, name: 'Lớp & học sinh' });
    const before = backend.calls('GET', '/me/permissions').length;
    backend.overrides.push((r) => (r.path === '/auth/me' ? problem(403, 'FEATURE_DISABLED', { feature_key: 'menu.school_lms.classes' }) : undefined));
    backend.disabledMenus = ['menu.school_lms.classes'];
    await runtime.api.call(runtime.api.client.GET('/auth/me', {})).catch(() => undefined);
    expect(await screen.findByText(/Khóa tính năng: menu\.school_lms\.classes/)).toBeInTheDocument();
    expect(screen.getByText(/Mã truy vết: trace-feature_disabled/)).toBeInTheDocument();
    await waitFor(() => expect(backend.calls('GET', '/me/permissions').length).toBeGreaterThan(before));
    expect(await screen.findByTestId('feature-disabled')).toBeInTheDocument();
    expect(runtime.tokens.get()).not.toBeNull();
  });

  it('403 PERMISSION_DENIED → permissions reloaded at once (F051-AC-12)', async () => {
    const backend = new MockPortalBackend().withSession('ad.truong.a');
    const { runtime } = await renderPortal({ url: '/school-lms/classes', backend });
    await screen.findByRole('heading', { level: 1, name: 'Lớp & học sinh' });
    const before = backend.calls('GET', '/me/permissions').length;
    backend.overrides.push((r) => (r.path === '/auth/me' ? problem(403, 'PERMISSION_DENIED') : undefined));
    await runtime.api.call(runtime.api.client.GET('/auth/me', {})).catch(() => undefined);
    await waitFor(() => expect(backend.calls('GET', '/me/permissions').length).toBeGreaterThan(before));
  });

  it('refresh period = principal_cache_ttl from the runtime config, always on window focus (F051-AC-12)', () => {
    expect(permissionsQueryTiming(TEST_CONFIG.principalCacheTtlSeconds)).toEqual({ staleTime: 60_000, refetchInterval: 60_000, refetchOnWindowFocus: 'always' });
    expect(permissionsQueryTiming(30).refetchInterval).toBe(30_000);
  });
});

describe('ActionGate (F051-AC-36)', () => {
  async function renderGate(perms: { permissions: string[]; disabled_actions?: string[] }, mode?: 'hide' | 'disable') {
    const i18n = await createI18n('vi-VN');
    const backend = new MockPortalBackend();
    const runtime = createPortalRuntime(TEST_CONFIG, i18n, { fetch: backend.fetch, locks: null, channel: null });
    runtime.queryClient.setQueryData(['me', 'permissions', 'none'], { roles: [], workspaces: ['school_lms'], ...perms });
    const { SessionProvider } = await import('../session/SessionProvider');
    const { MemoryRouter } = await import('react-router-dom');
    render(
      <I18nextProvider i18n={i18n}>
        <OpsThemeProvider>
          <PortalRuntimeContext.Provider value={runtime}>
            <QueryClientProvider client={runtime.queryClient}>
              <MemoryRouter>
                <SessionProvider>
                  <ActionGate actionKey="action.school_lms.schools.edit" mode={mode}>
                    <Button>Sửa</Button>
                  </ActionGate>
                </SessionProvider>
              </MemoryRouter>
            </QueryClientProvider>
          </PortalRuntimeContext.Provider>
        </OpsThemeProvider>
      </I18nextProvider>,
    );
    return runtime;
  }

  it('shown with the permission; hidden without it; hidden or disabled+tooltip when turned off', async () => {
    let rt = await renderGate({ permissions: ['tenant.school.write'] });
    expect(await screen.findByRole('button', { name: 'Sửa' })).toBeEnabled();
    rt.scheduler.stop();
    document.body.innerHTML = '';
    rt = await renderGate({ permissions: ['tenant.school.read'] });
    expect(screen.queryByRole('button', { name: 'Sửa' })).toBeNull();
    document.body.innerHTML = '';
    rt = await renderGate({ permissions: ['tenant.school.write'], disabled_actions: ['action.school_lms.schools.edit'] });
    expect(screen.queryByRole('button', { name: 'Sửa' })).toBeNull();
    document.body.innerHTML = '';
    await renderGate({ permissions: ['tenant.school.write'], disabled_actions: ['action.school_lms.schools.edit'] }, 'disable');
    expect(await screen.findByRole('button', { name: 'Sửa' })).toBeDisabled();
  });
});
