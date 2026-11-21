// F059 T044/T045/T047/T048/T057 — F059-AC-22…25, F059-AC-28 (TC-CMS-AUTHZ-05, TC-CMS-WS-02, TC-CMS-MENU-02):
// Integrations read only (Keycloak/APISIX reference from getIntegrationOverview, route groups prefix → gateway → service — OQ
// F059-12; SIS connections with schools and
// sync runs; no input that writes, no secret/credential/token even when the response carries fields outside the
// schema); F059 screens are platform-only (403 page, chunk D not loaded for ad.truong.a / cm.noidung / sm.antoan);
// 403 PERMISSION_DENIED shows its trace id, MFA_REQUIRED opens the step-up; a turned-off menu is hidden and its
// `403 FEATURE_DISABLED` reloads permissions; protected menus stay. Backend = MOCK.
import { cleanup, configure, fireEvent, screen, waitFor, within } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { tenantFilterStore } from '../../../shell/tenantFilterStore';
import { ACCOUNTS, MockPortalBackend, permissionsOf, problem, renderPortal, stopPortals } from '../../../test/portal-harness';
import { withAccessMock, renderAccess } from '../../../test/platform-access-backend';
import { withPlatformMock } from '../../../test/platform-backend';

vi.setConfig({ testTimeout: 600_000 });
configure({ asyncUtilTimeout: 30_000 });

afterEach(() => {
  stopPortals();
  tenantFilterStore.reset();
});

const F059_URLS = ['/platform-admin/access-control', '/platform-admin/audit-logs', '/platform-admin/student-activity', '/platform-admin/feature-toggles', '/platform-admin/integrations'];

describe('Tích hợp chỉ xem (F059-AC-22/23)', () => {
  it('reference tab shows getIntegrationOverview (route groups prefix → gateway → service, OQ F059-12); SIS tab lists connections, schools, runs; no secret, no write input', async () => {
    const { backend, user } = await renderAccess('/platform-admin/integrations');
    expect(await screen.findByText('https://sso.eduworld.local/realms/eduworld')).toBeInTheDocument();
    expect(backend.calls('GET', '/admin/integrations')[0]!.headers['x-tenant-id']).toBeUndefined();
    expect(screen.getByText('eduworld-portal')).toBeInTheDocument();
    expect(screen.getByText('Admin API (module identity)')).toBeInTheDocument();
    expect(screen.getByText('Bắt buộc')).toBeInTheDocument();
    expect(screen.getByText('900 giây')).toBeInTheDocument();
    expect(screen.getByText('https://api.eduworld.local')).toBeInTheDocument();
    expect(screen.getByText('Chưa cấu hình')).toBeInTheDocument();
    const groups = within(screen.getByTestId('route-groups'));
    expect(groups.getByText('/api/v1/admin/')).toBeInTheDocument();
    expect(groups.getByText('/realtime/v1')).toBeInTheDocument();
    expect(groups.getByText('realtime')).toBeInTheDocument();
    expect(groups.getAllByText('Public').length).toBe(2);
    expect(groups.getByText('Nội bộ')).toBeInTheDocument();
    expect(screen.queryByText('Chưa có dữ liệu')).toBeNull();
    expect(screen.getByText('Lỗi xác thực: 1')).toBeInTheDocument();
    expect(document.body.innerHTML).not.toContain('kc-secret-should-not-show');
    await user.click(screen.getByRole('tab', { name: 'SIS' }));
    expect(await screen.findByText('API SIS (contract v1)')).toBeInTheDocument();
    expect(screen.getByText('Đã cấu hình (ẩn)')).toBeInTheDocument();
    expect(screen.getByText(/Lỗi một phần · .* · 3 bản ghi lỗi/)).toBeInTheDocument();
    expect(backend.calls('GET', '/sis/connections')[0]!.headers['x-tenant-id']).toBeUndefined();
    await user.click(document.querySelector('.ant-table-row-expand-icon')!);
    expect(await screen.findByText('Trường TH A')).toBeInTheDocument();
    await waitFor(() => expect(screen.getAllByText('Lỗi một phần').length).toBeGreaterThan(0));
    const page = document.body.innerHTML;
    for (const secret of ['vault://', 'tok-should-not-show', 'secret_ref', 'api_token']) expect(page).not.toContain(secret);
    expect(screen.queryAllByRole('textbox')).toHaveLength(0);
    expect(screen.queryAllByRole('spinbutton')).toHaveLength(0);
    for (const name of ['Lưu', 'Sửa', 'Xóa', 'Tạo']) expect(screen.queryByRole('button', { name })).toBeNull();
  });
});

function backendFor(account: string) {
  const backend = new MockPortalBackend().withSession(account === 'sm.antoan' ? 'cm.noidung' : account);
  if (account === 'sm.antoan') {
    // MOCK (test only): Safety Moderator — platform role of workspace E only (ADR-0072)
    backend.user = { ...ACCOUNTS['cm.noidung']!, username: 'sm.antoan', displayName: 'Safety Moderator', perms: { platform: { workspaces: ['live_ops'], permissions: permissionsOf(['live_ops'], { platform: true }) } } };
  }
  withPlatformMock(backend);
  withAccessMock(backend);
  return backend;
}

describe('Bảo vệ màn F059 (F059-AC-24)', () => {
  it.each(['ad.truong.a', 'cm.noidung', 'sm.antoan'])('%s → 403 on every F059 URL, chunk D not loaded, no /admin call', async (account) => {
    for (const url of F059_URLS) {
      const backend = backendFor(account);
      const { loaders } = await renderPortal({ url, backend });
      expect(await screen.findByText('Không có quyền truy cập')).toBeInTheDocument();
      expect(loaders.platform_admin).not.toHaveBeenCalled();
      expect(backend.requests.some((r) => r.path.startsWith('/admin/'))).toBe(false);
      cleanup();
      stopPortals();
    }
  });

  it('a System Admin without platform.audit.read does not see the audit menu nor its screen', async () => {
    const backend = backendFor('sa.admin');
    const sa = ACCOUNTS['sa.admin']!;
    backend.user = { ...sa, perms: { platform: { ...sa.perms.platform!, permissions: sa.perms.platform!.permissions.filter((p) => p !== 'platform.audit.read') } } };
    await renderPortal({ url: '/platform-admin/audit-logs', backend });
    expect(await screen.findByText('Không có quyền truy cập')).toBeInTheDocument();
    expect(screen.queryByRole('link', { name: 'Nhật ký toàn hệ thống' })).toBeNull();
    expect(screen.getByRole('link', { name: 'Người dùng & phân quyền' })).toBeInTheDocument();
  });
});

describe('Lỗi quyền, MFA (F059-AC-25)', () => {
  it('403 PERMISSION_DENIED → "Không có quyền" with trace id', async () => {
    const backend = backendFor('sa.admin');
    backend.overrides.unshift((r) => (r.path === '/admin/roles' ? problem(403, 'PERMISSION_DENIED') : undefined));
    await renderPortal({ url: '/platform-admin/access-control?tab=roles', backend });
    expect(await screen.findByText('Bạn không có quyền thực hiện thao tác này.')).toBeInTheDocument();
    expect(screen.getByText('Mã truy vết: trace-permission_denied')).toBeInTheDocument();
  });

  it('403 MFA_REQUIRED on an F059 list → step-up re-login, request retried', async () => {
    const backend = backendFor('sa.admin');
    let first = true;
    backend.overrides.unshift((r) => (r.path === '/admin/login-events' && first ? ((first = false), problem(403, 'MFA_REQUIRED')) : undefined));
    const { router, user } = await renderPortal({ url: '/platform-admin/student-activity', backend });
    const dialog = await screen.findByRole('dialog');
    fireEvent.change(within(dialog).getByLabelText('Tên đăng nhập hoặc email'), { target: { value: 'sa.admin' } });
    fireEvent.change(within(dialog).getByLabelText('Mật khẩu'), { target: { value: 'Demo@12345' } });
    await user.click(within(dialog).getByRole('button', { name: 'Xác nhận' }));
    expect(await screen.findByText('Đăng nhập trên thiết bị khác')).toBeInTheDocument();
    expect(backend.calls('GET', '/admin/login-events').length).toBeGreaterThanOrEqual(2);
    expect(router.state.location.pathname).toBe('/platform-admin/student-activity');
  });
});

describe('Menu/action tắt (F059-AC-28)', () => {
  it('menu student_activity turned off → menu hidden, URL shows "Tính năng đang tắt"; protected menus stay', async () => {
    await renderAccess('/platform-admin/student-activity', { disabledMenus: ['menu.platform_admin.student_activity'] });
    expect((await screen.findAllByText('Tính năng đang tắt')).length).toBeGreaterThan(0);
    expect(screen.queryByRole('link', { name: 'Hoạt động đăng nhập game' })).toBeNull();
    for (const name of ['Người dùng & phân quyền', 'Nhật ký toàn hệ thống', 'Bật/tắt menu & action']) expect(screen.getByRole('link', { name })).toBeInTheDocument();
  });

  it('turned-off assign action hides "Gán role" / "Thu hồi"; 403 FEATURE_DISABLED on login events reloads permissions', async () => {
    await renderAccess('/platform-admin/access-control?tab=user-roles', { disabledActions: ['action.platform_admin.access_control.assign_roles'] });
    expect((await screen.findAllByText('liveops_lead')).length).toBeGreaterThan(0);
    expect(screen.queryByRole('button', { name: 'Gán role' })).toBeNull();
    expect(screen.queryByRole('button', { name: 'Thu hồi' })).toBeNull();
    cleanup();
    stopPortals();
    const backend = backendFor('sa.admin');
    backend.overrides.unshift((r) => {
      if (r.path !== '/admin/login-events') return undefined;
      backend.disabledMenus = ['menu.platform_admin.student_activity']; // turned off meanwhile
      return problem(403, 'FEATURE_DISABLED', { feature_key: 'menu.platform_admin.student_activity' });
    });
    await renderPortal({ url: '/platform-admin/student-activity', backend });
    await waitFor(() => expect(backend.calls('GET', '/admin/login-events').length).toBeGreaterThan(0));
    await waitFor(() => expect(backend.calls('GET', '/me/permissions').length).toBeGreaterThan(1));
    expect((await screen.findAllByText('Tính năng đang tắt')).length).toBeGreaterThan(0);
    await waitFor(() => expect(screen.queryByRole('link', { name: 'Hoạt động đăng nhập game' })).toBeNull());
  });
});
