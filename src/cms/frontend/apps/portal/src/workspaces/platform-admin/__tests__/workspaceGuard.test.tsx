// F057 T040/T041/T049 — F057-AC-18, F057-AC-19, F057-AC-22 (TC-CMS-AUTHZ-05, TC-CMS-WS-01/02, TC-CMS-MENU-02):
// users without system_admin (per getMyPermissions) opening a workspace D URL get the 403 page and the D chunk is never
// imported; `403 MFA_REQUIRED` on a D call opens the step-up re-login and keeps the URL; a network failure of the
// internal gateway shows "Cần kết nối mạng quản trị (VPN)"; a turned-off clone action hides its tab and
// `403 FEATURE_DISABLED` shows the notice and reloads permissions. Display gates only — the backend authorizes.
import { configure, fireEvent, screen, waitFor, within } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { ACCOUNTS, MockPortalBackend, permissionsOf, problem, renderPortal, stopPortals } from '../../../test/portal-harness';
import { renderPlatform, withPlatformMock } from '../../../test/platform-backend';

// whole-portal flows with several Ant Design selects and dialogs are slow on the throttled shared CPU (2 workers max)
vi.setConfig({ testTimeout: 300_000 });
configure({ asyncUtilTimeout: 30_000 });

afterEach(() => stopPortals());

function backendFor(account: string) {
  const backend = new MockPortalBackend().withSession(account === 'sm.antoan' ? 'cm.noidung' : account);
  if (account === 'sm.antoan') {
    // MOCK (test only): Safety Moderator — platform role of workspace E only (ADR-0072)
    backend.user = { ...ACCOUNTS['cm.noidung']!, username: 'sm.antoan', displayName: 'Safety Moderator', perms: { platform: { workspaces: ['live_ops'], permissions: permissionsOf(['live_ops'], { platform: true }) } } };
  }
  withPlatformMock(backend);
  return backend;
}

describe('Bảo vệ workspace D (F057-AC-18)', () => {
  it.each(['ad.truong.a', 'cm.noidung', 'sm.antoan'])('%s → 403 page, D chunk not loaded, no D API call', async (account) => {
    const backend = backendFor(account);
    const { loaders } = await renderPortal({ url: '/platform-admin/trash', backend });
    expect(await screen.findByText('Không có quyền truy cập')).toBeInTheDocument();
    expect(loaders.platform_admin).not.toHaveBeenCalled();
    expect(backend.requests.some((r) => r.path.startsWith('/admin/'))).toBe(false);
  });

  it('sa.admin loads the D chunk and the screen', async () => {
    const { loaders } = await renderPlatform('/platform-admin/trash');
    await screen.findByRole('heading', { level: 1, name: 'Thùng rác' });
    expect(loaders.platform_admin).toHaveBeenCalled();
  });
});

describe('MFA step-up và VPN (F057-AC-19)', () => {
  it('403 MFA_REQUIRED on a D list → re-login (username + password), request retried, URL kept', async () => {
    const backend = backendFor('sa.admin');
    let first = true;
    backend.overrides.unshift((r) => (r.path === '/admin/trash' && first ? ((first = false), problem(403, 'MFA_REQUIRED')) : undefined));
    const { router, user } = await renderPortal({ url: '/platform-admin/trash', backend });
    const dialog = await screen.findByRole('dialog');
    expect(within(dialog).getByText('Thao tác này cần đăng nhập lại.')).toBeInTheDocument();
    fireEvent.change(within(dialog).getByLabelText('Tên đăng nhập hoặc email'), { target: { value: 'sa.admin' } });
    fireEvent.change(within(dialog).getByLabelText('Mật khẩu'), { target: { value: 'Demo@12345' } });
    await user.click(within(dialog).getByRole('button', { name: 'Xác nhận' }));
    expect(await screen.findByText('Câu hỏi bảng nhân 7')).toBeInTheDocument();
    expect(backend.calls('GET', '/admin/trash').length).toBeGreaterThanOrEqual(2);
    expect(router.state.location.pathname).toBe('/platform-admin/trash');
  });

  it('network failure of the internal gateway → "Cần kết nối mạng quản trị (VPN)"', async () => {
    const backend = backendFor('sa.admin');
    backend.overrides.unshift((r) => (r.path === '/admin/trash' ? Promise.reject(new TypeError('Failed to fetch')) : undefined));
    await renderPortal({ url: '/platform-admin/trash', backend });
    const banner = await screen.findByTestId('banner-slot');
    await waitFor(() => expect(within(banner).getByText('Cần kết nối mạng quản trị (VPN)')).toBeInTheDocument());
  });
});

describe('Menu/action tắt (F057-AC-22)', () => {
  it('turned-off clone action hides the "Clone giữa tenant" tab; assign stays', async () => {
    await renderPlatform('/platform-admin/tenant-data', { disabledActions: ['action.platform_admin.tenant_data.clone'] });
    expect(await screen.findByRole('tab', { name: 'Gán cho tenant' })).toBeInTheDocument();
    expect(screen.queryByRole('tab', { name: 'Clone giữa tenant' })).toBeNull();
  });

  it('turned-off catalog import hides its tab; system settings edit off → read only, no Save', async () => {
    await renderPlatform('/platform-admin/catalog', { disabledActions: ['action.platform_admin.catalog.import_schools'] });
    expect(await screen.findByRole('tab', { name: 'Quốc gia' })).toBeInTheDocument();
    expect(screen.queryByRole('tab', { name: 'Import trường (CSV)' })).toBeNull();
    stopPortals();
    await renderPlatform('/platform-admin/system-settings', { disabledActions: ['action.platform_admin.system_settings.edit'] });
    expect(await screen.findByText('Bạn chỉ có quyền xem cấu hình.')).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Lưu' })).toBeNull();
  });

  it('403 FEATURE_DISABLED from createTenantClone → "Tính năng đang tắt" notice and permissions reloaded', async () => {
    const { backend, user } = await renderPlatform('/platform-admin/tenant-data');
    backend.overrides.unshift((r) => (r.method === 'POST' && r.path === '/admin/tenant-clones' ? problem(403, 'FEATURE_DISABLED', { feature_key: 'action.platform_admin.tenant_data.clone' }) : undefined));
    await user.click(await screen.findByRole('tab', { name: 'Clone giữa tenant' }));
    const pick = async (label: string, title: string) => {
      await user.click(screen.getByRole('combobox', { name: label }));
      await user.click((await screen.findAllByTitle(title)).at(-1)!);
    };
    await pick('Tenant nguồn', 'TH Demo A (th-demo-a)');
    await pick('Loại dữ liệu', 'Câu hỏi');
    const ids = screen.getByRole('combobox', { name: 'Mã bản ghi (UUID)' });
    fireEvent.change(ids, { target: { value: '0193a4d2-2222-7000-8000-000000000001' } });
    fireEvent.keyDown(ids, { key: 'Enter', code: 'Enter', keyCode: 13 });
    await user.click(screen.getByRole('button', { name: 'Tiếp' }));
    await pick('Tenant đích', 'Phòng GD Demo (pgd-demo)');
    await user.click(screen.getByRole('button', { name: 'Tiếp' }));
    await user.click(await screen.findByRole('button', { name: 'Tiếp' }));
    const permsBefore = backend.calls('GET', '/me/permissions').length;
    backend.disabledActions = ['action.platform_admin.tenant_data.clone'];
    await user.click(await screen.findByRole('button', { name: 'Chạy clone' }));
    expect((await screen.findAllByText('Tính năng đang tắt')).length).toBeGreaterThan(0);
    await waitFor(() => expect(backend.calls('GET', '/me/permissions').length).toBeGreaterThan(permsBefore));
    await waitFor(() => expect(screen.queryByRole('tab', { name: 'Clone giữa tenant' })).toBeNull());
  });
});
