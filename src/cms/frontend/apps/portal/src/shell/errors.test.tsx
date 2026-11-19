// F051 T026 — F051-AC-13, F051-AC-14, F051-AC-15 (TC-CMS-WS-02, TC-AUTH-101): `403 MFA_REQUIRED` opens the re-login
// dialog on the current page (portalLogin with code), the request is retried once and the URL kept, cancel → /login;
// network failure of the internal gateway in game_ops/live_ops/platform_admin → "Cần kết nối mạng quản trị (VPN)";
// Problem Details → translated message by `code` + trace_id, unknown code → generic message. MOCK backend.
import { OpsThemeProvider } from '@eduworld/ops-ui';
import { fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { I18nextProvider } from 'react-i18next';
import { afterEach, describe, expect, it } from 'vitest';
import { createI18n } from '../i18n';
import { ApiError } from '../shared/api/middleware/problem';
import { MockPortalBackend, problem, renderPortal, stopPortals } from '../test/portal-harness';
import { ProblemMessage } from './ProblemMessage';

afterEach(() => stopPortals());

describe('MFA re-login (F051-AC-13)', () => {
  it('opens on the current page, retries the request with the new token, keeps the URL', async () => {
    const backend = new MockPortalBackend().withSession('sa.admin');
    const { user, router, runtime } = await renderPortal({ url: '/platform-admin/tenants', backend });
    await screen.findByRole('heading', { level: 1, name: 'Tenant, kết nối SIS' });
    const oldToken = runtime.tokens.get()!.token;
    backend.overrides.push((r) => (r.path === '/admin/tenants' && r.headers.authorization === `Bearer ${oldToken}` ? problem(403, 'MFA_REQUIRED') : undefined));
    const pending = runtime.api.call<{ data: unknown[] }>(runtime.api.client.GET('/admin/tenants', {}));
    const dialog = await screen.findByRole('dialog');
    expect(within(dialog).getByText('Thao tác này cần đăng nhập lại với mã xác thực.')).toBeInTheDocument();
    fireEvent.change(within(dialog).getByLabelText('Tên đăng nhập hoặc email'), { target: { value: 'sa.admin' } });
    fireEvent.change(within(dialog).getByLabelText('Mật khẩu'), { target: { value: 'Demo@12345' } });
    fireEvent.change(within(dialog).getByLabelText('Mã xác thực 6 số'), { target: { value: '123456' } });
    await user.click(within(dialog).getByRole('button', { name: 'Xác nhận' }));
    const res = await pending;
    expect(res.data.length).toBeGreaterThan(0);
    expect(runtime.tokens.get()!.token).not.toBe(oldToken);
    const auths = backend.calls('GET', '/admin/tenants').map((r) => r.headers.authorization);
    expect(auths).toContain(`Bearer ${oldToken}`);
    expect(auths.at(-1)).toBe(`Bearer ${runtime.tokens.get()!.token}`);
    expect(router.state.location.pathname).toBe('/platform-admin/tenants');
  });

  it('cancel → back to /login keeping the URL', async () => {
    const backend = new MockPortalBackend().withSession('sa.admin');
    const { user, router, runtime } = await renderPortal({ url: '/platform-admin/tenants', backend });
    await screen.findByRole('heading', { level: 1, name: 'Tenant, kết nối SIS' });
    backend.overrides.push((r) => (r.path === '/admin/tenants' ? problem(403, 'MFA_REQUIRED') : undefined));
    const pending = runtime.api.call(runtime.api.client.GET('/admin/tenants', {})).catch((e: unknown) => e);
    const dialog = await screen.findByRole('dialog');
    await user.click(within(dialog).getByRole('button', { name: 'Hủy' }));
    expect(await pending).toBeInstanceOf(ApiError);
    await waitFor(() => expect(router.state.location.pathname).toBe('/login'));
    expect(router.state.location.search).toBe('?returnUrl=%2Fplatform-admin%2Ftenants');
  });
});

describe('VPN notice (F051-AC-14)', () => {
  it('network failure of the internal gateway in an internal workspace → VPN notice instead of a generic error', async () => {
    const backend = new MockPortalBackend().withSession('sa.admin');
    const { runtime } = await renderPortal({ url: '/platform-admin/tenants', backend });
    await screen.findByRole('heading', { level: 1, name: 'Tenant, kết nối SIS' });
    backend.overrides.push((r) => (r.gateway === 'internal' ? Promise.reject(new TypeError('Failed to fetch')) : undefined));
    const err = await runtime.api.call(runtime.api.client.GET('/admin/tenants', {})).catch((e: unknown) => e);
    expect((err as ApiError).code).toBe('VPN_REQUIRED');
    const banner = await screen.findByTestId('banner-slot');
    expect(within(banner).getByText('Cần kết nối mạng quản trị (VPN)')).toBeInTheDocument();
  });
});

describe('ProblemMessage (F051-AC-15)', () => {
  it('translated message by code with trace_id; unknown code → generic message', async () => {
    const i18n = await createI18n('vi-VN');
    const known = new ApiError({ status: 403, code: 'PERMISSION_DENIED', i18nKey: 'error.PERMISSION_DENIED', traceId: 'trace-abc' });
    const unknown = new ApiError({ status: 418, code: 'SOMETHING_NEW', i18nKey: 'error.generic', traceId: 'trace-def' });
    render(
      <I18nextProvider i18n={i18n}>
        <OpsThemeProvider>
          <ProblemMessage error={known} />
          <ProblemMessage error={unknown} />
          <ProblemMessage error={new Error('boom')} />
        </OpsThemeProvider>
      </I18nextProvider>,
    );
    expect(screen.getByText('Bạn không có quyền thực hiện thao tác này.')).toBeInTheDocument();
    expect(screen.getByText('Mã truy vết: trace-abc')).toBeInTheDocument();
    expect(screen.getAllByText('Đã có lỗi xảy ra. Vui lòng thử lại.')).toHaveLength(2);
    expect(screen.getByText('Mã truy vết: trace-def')).toBeInTheDocument();
    expect(screen.queryByText('boom')).toBeNull();
  });
});
