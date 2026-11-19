// F051 T028/T031/T035 at unit level — F051-AC-16, F051-AC-17, F051-AC-18, F051-AC-19 (TC-I18N-006/007, TC-UI-004):
// desktop sidebar ≥ 1280 px, drawer (☰) below, tenant controls inside the drawer below 768 px; user menu switches
// language, dark theme and compact density at once without reload; skip link + landmarks. MOCK backend harness.
import { screen, waitFor, within } from '@testing-library/react';
import { afterEach, describe, expect, it } from 'vitest';
import { MockPortalBackend, renderPortal, setViewportWidth, stopPortals, TENANTS } from '../test/portal-harness';

afterEach(() => stopPortals());

async function qlcmInThDemoA() {
  window.sessionStorage.setItem('ew.portal.sessionTenantId', TENANTS.thDemoA.tenant_id);
  const r = await renderPortal({ url: '/school-lms/classes', backend: new MockPortalBackend().withSession('qlcm.truong.a') });
  await screen.findByRole('heading', { level: 1, name: 'Lớp & học sinh' });
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

describe('user menu and accessibility (F051-AC-16, F051-AC-17, F051-AC-18)', () => {
  it('language, dark theme and compact density apply at once, without reload', async () => {
    const { user, i18n } = await qlcmInThDemoA();
    await user.click(screen.getByRole('button', { name: 'Tài khoản' }));
    await user.click(await screen.findByText('Tối'));
    await waitFor(() => expect(document.querySelector('.ew-ops-root')).toHaveAttribute('data-theme', 'dark'));
    await user.click(screen.getByRole('switch', { name: 'Mật độ gọn' }));
    await waitFor(() => expect(document.querySelector('.ew-ops-root')).toHaveAttribute('data-density', 'default'));
    await i18n.changeLanguage('en-US');
    expect(await screen.findByRole('heading', { level: 1, name: 'Classes & students' })).toBeInTheDocument();
    expect(JSON.parse(window.localStorage.getItem('ew.portal.prefs') ?? '{}')).toMatchObject({ theme: 'dark', density: 'default' });
  });

  it('skip link to the main landmark, labelled navigation, breadcrumb', async () => {
    await qlcmInThDemoA();
    const skip = screen.getByRole('link', { name: 'Bỏ qua đến nội dung chính' });
    expect(skip).toHaveAttribute('href', '#ew-main');
    expect(screen.getByRole('main')).toHaveAttribute('id', 'ew-main');
    expect(screen.getAllByText('Quản lý trường và học tập').length).toBeGreaterThan(0);
  });
});
