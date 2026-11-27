// F134 T043, T044 — "Tải bộ cài" menu and feature toggle (F134-AC-17, F134-AC-18; TC-CMS-WS-01, TC-CMS-MENU-02). Backend =
// MOCK (test/installers-mock.ts + the F052 MOCK, test only, AGENTS R17). The menu comes from the generated catalog
// (`menu.school_lms.installers`, ADR-0090) and is attached to the screen by key, never registered by hand; it shows only
// with `school.installer.read`; a turned-off menu is hidden (its URL shows "Tính năng đang tắt"), a turned-off action
// hides the links and calls nothing, and `403 FEATURE_DISABLED` shows the notice and reloads getMyPermissions.
import { cleanup, screen, waitFor, within } from '@testing-library/react';
import { afterEach, describe, expect, it } from 'vitest';
import { WORKSPACE_CATALOG } from '../../../../generated/menu-catalog';
import { installersState, renderInstallers } from '../../../../test/installers-mock';
import { problem, stopPortals } from '../../../../test/portal-harness';
import { screens } from '../../index';

afterEach(() => {
  stopPortals();
  cleanup();
});

const MENU = 'Bộ cài game cho trường';
const URL = '/school-lms/installers';
const installerCalls = (backend: { calls: (m: string, p: string) => unknown[] }) => backend.calls('GET', '/tenant/client-installers');

describe('Menu "Tải bộ cài" (F134-AC-17, T044)', () => {
  it('catalog: menu.school_lms.installers requires school.installer.read, action .view → listSchoolClientInstallers; screen attached by key', () => {
    const menu = WORKSPACE_CATALOG.find((w) => w.key === 'school_lms')!.menus.find((m) => m.key === 'menu.school_lms.installers')!;
    expect(menu.path).toBe('/school-lms/installers');
    expect(menu.requiredPermissions).toEqual(['school.installer.read']);
    expect(menu.actions.map((a) => a.key)).toEqual(['action.school_lms.installers.view']);
    expect(screens['menu.school_lms.installers']).toBeDefined();
  });

  it('School Admin (school.installer.read) sees the menu and opens the screen', async () => {
    const { user } = await renderInstallers('/school-lms/schools');
    const nav = await screen.findByRole('navigation', { name: 'Menu điều hướng' });
    await user.click(await within(nav).findByRole('link', { name: MENU }));
    expect(await screen.findByTestId('installers-page')).toBeInTheDocument();
  });

  it('staff without school.installer.read: no menu; direct URL → 403 page, no installer call', async () => {
    const { backend } = await renderInstallers('/school-lms/schools', { as: 'qlcm.khong.cai' });
    const nav = await screen.findByRole('navigation', { name: 'Menu điều hướng' });
    await waitFor(() => expect(within(nav).getAllByRole('link').length).toBeGreaterThan(0));
    expect(within(nav).queryByRole('link', { name: MENU })).toBeNull();
    stopPortals();
    cleanup();
    const r2 = await renderInstallers(URL, { as: 'qlcm.khong.cai' });
    expect(await screen.findByText('Không có quyền truy cập')).toBeInTheDocument();
    expect(installerCalls(r2.backend)).toHaveLength(0);
    expect(installerCalls(backend)).toHaveLength(0);
  });
});

describe('Tắt menu/action (F134-AC-18)', () => {
  it('menu turned off: hidden; its URL shows "Tính năng đang tắt" and nothing is called', async () => {
    await renderInstallers('/school-lms/schools', { disabledMenus: ['menu.school_lms.installers'] });
    const nav = await screen.findByRole('navigation', { name: 'Menu điều hướng' });
    await waitFor(() => expect(within(nav).getAllByRole('link').length).toBeGreaterThan(0));
    expect(within(nav).queryByRole('link', { name: MENU })).toBeNull();
    stopPortals();
    cleanup();
    const r2 = await renderInstallers(URL, { disabledMenus: ['menu.school_lms.installers'] });
    expect(await screen.findByText('Tính năng đang tắt')).toBeInTheDocument();
    expect(installerCalls(r2.backend)).toHaveLength(0);
  });

  it('view action turned off: "tính năng tạm tắt", no download/Web link, no installer call', async () => {
    const { backend } = await renderInstallers(URL, { disabledActions: ['action.school_lms.installers.view'] });
    expect(await screen.findByTestId('installers-disabled')).toHaveTextContent('Tính năng đang tạm tắt.');
    expect(screen.getByTestId('installers-page').querySelectorAll('a[href]')).toHaveLength(0);
    expect(installerCalls(backend)).toHaveLength(0);
  });

  it('403 FEATURE_DISABLED from listSchoolClientInstallers → "tính năng tạm tắt", no link, permissions reloaded', async () => {
    const state = installersState();
    state.answer = () => problem(403, 'FEATURE_DISABLED', { feature_key: 'menu.school_lms.installers' });
    const { backend } = await renderInstallers('/school-lms/schools', { state });
    await screen.findByRole('navigation', { name: 'Menu điều hướng' });
    await waitFor(() => expect(backend.calls('GET', '/me/permissions').length).toBeGreaterThan(0));
    const before = backend.calls('GET', '/me/permissions').length;
    const nav = screen.getByRole('navigation', { name: 'Menu điều hướng' });
    (await within(nav).findByRole('link', { name: MENU })).click();
    expect(await screen.findByTestId('installers-error')).toHaveTextContent('Tính năng đang tạm tắt.');
    expect(screen.queryByTestId('web-client-card')).toBeNull();
    await waitFor(() => expect(backend.calls('GET', '/me/permissions').length).toBeGreaterThan(before));
  });
});
