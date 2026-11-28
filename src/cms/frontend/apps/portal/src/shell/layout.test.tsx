// F051 T028/T031/T035 at unit level — F051-AC-16, F051-AC-17, F051-AC-18, F051-AC-19 (TC-I18N-006/007, TC-UI-004):
// desktop sidebar ≥ 1280 px, drawer (☰) below, tenant controls inside the drawer below 768 px; user menu switches
// language, dark theme and compact density at once without reload; skip link + landmarks. MOCK backend harness.
// Owner 2026-11-22 (ADR-0098): language and light/dark are icon buttons in the header (not in the account dropdown);
// header and sidebar stay fixed — only the content area scrolls; the sidebar (and its drawer) wears the sider tone.
import { screen, waitFor, within } from '@testing-library/react';
import { afterEach, describe, expect, it } from 'vitest';
import { MockPortalBackend, renderPortal, setViewportWidth, stopPortals, TENANTS } from '../test/portal-harness';
import { SHELL_CLASSES } from './PortalLayout';

afterEach(() => stopPortals());

async function qlcmInThDemoA() {
  window.sessionStorage.setItem('ew.portal.sessionTenantId', TENANTS.thDemoA.tenant_id);
  const r = await renderPortal({ url: '/school-lms/students', backend: new MockPortalBackend().withSession('qlcm.truong.a') });
  await screen.findByRole('heading', { level: 1, name: 'Học sinh theo khối' });
  return r;
}

describe('responsive shell (F051-AC-19)', () => {
  it('desktop ≥ 1280 px: sidebar navigation and tenant switcher in the topbar, no ☰', async () => {
    await qlcmInThDemoA();
    expect(screen.getByRole('navigation', { name: 'Menu điều hướng' })).toBeInTheDocument();
    expect(screen.getByRole('combobox', { name: 'Đơn vị làm việc' })).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Mở menu' })).toBeNull();
  });

  it('tablet (< 1280 px): sidebar becomes a drawer opened by ☰', async () => {
    setViewportWidth(1024);
    const { user } = await qlcmInThDemoA();
    expect(screen.queryByRole('navigation', { name: 'Menu điều hướng' })).toBeNull();
    expect(screen.getByRole('combobox', { name: 'Đơn vị làm việc' })).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'Mở menu' }));
    const drawer = await screen.findByRole('dialog');
    expect(within(drawer).getByRole('navigation', { name: 'Menu điều hướng' })).toBeInTheDocument();
    // the drawer is the sidebar on narrow screens: same deep-blue tone and dark-themed menu (admin tokens)
    expect((drawer.closest('.ant-drawer-content') as HTMLElement).style.background).toBe('var(--ew-ops-color-sider-bg)');
    expect(within(drawer).getByRole('menu')).toHaveClass('ant-menu-dark');
  });

  it('mobile (< 768 px): tenant controls move into the ☰ drawer; ☰ is a ≥ 44 px touch target', async () => {
    setViewportWidth(600);
    const { user } = await qlcmInThDemoA();
    expect(screen.queryByRole('combobox', { name: 'Đơn vị làm việc' })).toBeNull();
    const burger = screen.getByRole('button', { name: 'Mở menu' });
    expect(burger.style.minHeight).toBe('44px');
    await user.click(burger);
    const drawer = await screen.findByRole('dialog');
    expect(within(drawer).getByRole('combobox', { name: 'Đơn vị làm việc' })).toBeInTheDocument();
  });
});

describe('fixed shell (owner 2026-11-22, ADR-0098)', () => {
  it('desktop: the page does not scroll — header and sidebar stay, the sidebar and the content scroll on their own', async () => {
    await qlcmInThDemoA();
    const shell = screen.getByTestId('portal-shell');
    expect(shell).toHaveClass(...SHELL_CLASSES.root.split(' '));
    expect(SHELL_CLASSES.root).toContain('tw-overflow-hidden');
    const header = screen.getByRole('banner');
    expect(header.parentElement).toBe(shell);
    expect(header).toHaveClass(...SHELL_CLASSES.header.split(' '));
    const sider = screen.getByTestId('portal-sider');
    expect(sider).toHaveClass(...SHELL_CLASSES.sider.split(' '));
    expect(SHELL_CLASSES.sider).toContain('tw-overflow-y-auto');
    const main = screen.getByRole('main');
    expect(main).toHaveClass(...SHELL_CLASSES.content.split(' '));
    expect(SHELL_CLASSES.content.split(' ')).toContain('tw-overflow-auto');
    // the sidebar menu uses the dark Menu theme, coloured by the admin tokens ops.color.sider.* (no raw values)
    expect(within(screen.getByRole('navigation', { name: 'Menu điều hướng' })).getByRole('menu')).toHaveClass('ant-menu-dark');
  });

  it('phone: still no page scroll, no sidebar, header toggles stay ≥ 44 px touch targets', async () => {
    setViewportWidth(390);
    await qlcmInThDemoA();
    expect(screen.getByTestId('portal-shell')).toHaveClass('tw-h-dvh', 'tw-overflow-hidden');
    expect(screen.queryByTestId('portal-sider')).toBeNull();
    expect(screen.getByTestId('locale-toggle').style.minHeight).toBe('44px');
    expect(screen.getByTestId('theme-toggle').style.minHeight).toBe('44px');
  });
});

describe('user menu and accessibility (F051-AC-16, F051-AC-17, F051-AC-18)', () => {
  it('header icon buttons switch language and light/dark at once, without reload, and persist them', async () => {
    const { user } = await qlcmInThDemoA();
    await user.click(screen.getByRole('button', { name: 'Chuyển sang giao diện tối' }));
    await waitFor(() => expect(document.querySelector('.ew-ops-root')).toHaveAttribute('data-theme', 'dark'));
    expect(screen.getByRole('button', { name: 'Chuyển sang giao diện sáng' })).toBeInTheDocument();
    const lang = screen.getByRole('button', { name: 'Ngôn ngữ VI — chuyển sang English' });
    expect(lang).toHaveTextContent('VI');
    await user.click(lang);
    expect(await screen.findByRole('heading', { level: 1, name: 'Students by grade' })).toBeInTheDocument();
    const back = screen.getByRole('button', { name: 'Language EN — switch to Tiếng Việt' });
    expect(back).toHaveTextContent('EN');
    await waitFor(() => expect(document.documentElement.lang).toBe('en-US'));
    expect(JSON.parse(window.localStorage.getItem('ew.portal.prefs') ?? '{}')).toMatchObject({ theme: 'dark', locale: 'en-US' });
    // keyboard: Enter on the focused toggle switches back
    back.focus();
    await user.keyboard('{Enter}');
    expect(await screen.findByRole('heading', { level: 1, name: 'Học sinh theo khối' })).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'Chuyển sang giao diện sáng' }));
    await waitFor(() => expect(document.querySelector('.ew-ops-root')).toHaveAttribute('data-theme', 'light'));
    expect(JSON.parse(window.localStorage.getItem('ew.portal.prefs') ?? '{}')).toMatchObject({ theme: 'light', locale: 'vi-VN' });
  });

  it('account dropdown keeps the user name, compact density and sign out — no language or theme control', async () => {
    const { user } = await qlcmInThDemoA();
    await user.click(screen.getByRole('button', { name: /^Tài khoản/ }));
    const compact = await screen.findByRole('switch', { name: 'Mật độ gọn' });
    const popup = compact.closest('.ant-popover') as HTMLElement;
    expect(within(popup).getByRole('button', { name: 'Đăng xuất' })).toBeInTheDocument();
    expect(within(popup).queryByRole('combobox')).toBeNull();
    expect(within(popup).queryByRole('radiogroup')).toBeNull();
    expect(within(popup).queryByText('Tối')).toBeNull();
    expect(within(popup).queryByText('English')).toBeNull();
    // comfortable density is the default (DESIGN_SYSTEM §4.1); the switch turns compact on as an explicit choice
    expect(document.querySelector('.ew-ops-root')).toHaveAttribute('data-density', 'default');
    await user.click(compact);
    await waitFor(() => expect(document.querySelector('.ew-ops-root')).toHaveAttribute('data-density', 'compact'));
    expect(JSON.parse(window.localStorage.getItem('ew.portal.prefs') ?? '{}')).toMatchObject({ density: 'compact', densityChosen: true });
  });

  it('skip link to the main landmark, labelled navigation, breadcrumb', async () => {
    await qlcmInThDemoA();
    const skip = screen.getByRole('link', { name: 'Bỏ qua đến nội dung chính' });
    expect(skip).toHaveAttribute('href', '#ew-main');
    expect(screen.getByRole('main')).toHaveAttribute('id', 'ew-main');
    expect(screen.getAllByText('Quản lý trường và học tập').length).toBeGreaterThan(0);
  });
});
