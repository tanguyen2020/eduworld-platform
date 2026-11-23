// F051 T026 — F051-AC-13, F051-AC-14, F051-AC-15 (TC-CMS-WS-02, TC-AUTH-101): no step-up re-login (ADR-0094): a 403 on a
// platform route never opens a dialog and is not retried; an ended session (refresh 401) goes to /login keeping the URL;
// network failure of the internal gateway in game_ops/live_ops/platform_admin → "Cần kết nối mạng quản trị (VPN)";
// Problem Details → translated message by `code` + trace_id, unknown code → generic message. MOCK backend.
import { OpsThemeProvider } from '@eduworld/ops-ui';
import { render, screen, waitFor, within } from '@testing-library/react';
import { I18nextProvider } from 'react-i18next';
import { afterEach, describe, expect, it } from 'vitest';
import { createI18n } from '../i18n';
import { ApiError } from '../shared/api/middleware/problem';
import { MockPortalBackend, problem, renderPortal, stopPortals } from '../test/portal-harness';
import { ProblemMessage } from './ProblemMessage';

afterEach(() => stopPortals());

describe('no step-up re-login (F051-AC-13, ADR-0094)', () => {
  it('a 403 on a platform route opens no dialog and is not retried; the URL stays', async () => {
    const backend = new MockPortalBackend().withSession('sa.admin');
    const { router, runtime } = await renderPortal({ url: '/platform-admin/tenants', backend });
    await screen.findByRole('heading', { level: 1, name: 'Tenant, kết nối SIS' });
    const before = backend.calls('GET', '/admin/tenants').length;
    backend.overrides.push((r) => (r.path === '/admin/tenants' ? problem(403, 'PERMISSION_DENIED') : undefined));
    const err = await runtime.api.call(runtime.api.client.GET('/admin/tenants', {})).catch((e: unknown) => e);
    expect((err as ApiError).code).toBe('PERMISSION_DENIED');
    expect(backend.calls('GET', '/admin/tenants')).toHaveLength(before + 1);
    expect(screen.queryByRole('dialog')).toBeNull();
    expect(backend.calls('POST', '/auth/portal/login')).toHaveLength(0);
    expect(router.state.location.pathname).toBe('/platform-admin/tenants');
  });

  it('session ended (refresh 401 REFRESH_TOKEN_INVALID) → /login keeping the URL, no dialog', async () => {
    const backend = new MockPortalBackend();
    const { router } = await renderPortal({ url: '/platform-admin/tenants', backend });
    expect(await screen.findByRole('heading', { level: 1, name: 'Welcome back, teachers' })).toBeInTheDocument();
    expect(screen.queryByRole('dialog')).toBeNull();
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
