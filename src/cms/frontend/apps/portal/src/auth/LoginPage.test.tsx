// F051 T055 (+T015 at unit level) — F051-AC-01, F051-AC-03, F051-AC-04, F051-AC-30 (TC-AUTH-070/071/072/093/094/095,
// TC-CMS-LOGIN-01): the portal's own /login (no Keycloak request, no tenant before sign-in), one `portalLogin` request,
// one generic error clearing password and code, 429 waiting time, OTP hint flag, routing of the demo accounts.
// Backend = MOCK harness (contract stand-in), not a live integration.
import { screen, waitFor } from '@testing-library/react';
import { afterEach, describe, expect, it } from 'vitest';
import { TEST_CONFIG } from '../shared/api/test-utils';
import { MockPortalBackend, problem, renderPortal, signIn, stopPortals } from '../test/portal-harness';

afterEach(() => stopPortals());

describe('/login (F051-AC-01, F051-AC-30)', () => {
  it('unauthenticated visit → portal login page with every field; no Keycloak, no tenant step', async () => {
    const { backend, router } = await renderPortal({ url: '/school-lms/classes' });
    expect(await screen.findByRole('heading', { name: 'Đăng nhập' })).toBeInTheDocument();
    expect(router.state.location.pathname).toBe('/login');
    expect(router.state.location.search).toBe('?returnUrl=%2Fschool-lms%2Fclasses');
    expect(screen.getByLabelText('Tên đăng nhập hoặc email')).toHaveAttribute('maxlength', '254');
    expect(screen.getByLabelText('Mật khẩu')).toHaveAttribute('type', 'password');
    expect(screen.getByLabelText('Mã xác thực 6 số')).toHaveAttribute('maxlength', '6');
    expect(screen.getByText('Bắt buộc với tài khoản vận hành platform; để trống nếu chưa cài.')).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Quên mật khẩu?' })).toBeInTheDocument();
    expect(screen.getByRole('combobox', { name: 'Ngôn ngữ' })).toBeInTheDocument();
    expect(screen.queryByText(/trường|đơn vị/i, { selector: 'label' })).toBeNull();
    // the only call at start-up is the silent refresh against the portal gateway (never Keycloak)
    expect(backend.requests.map((r) => `${r.method} ${r.path}`)).toEqual(['POST /auth/portal/token:refresh']);
  });

  it('one portalLogin request with username, password, device_id, locale (no otp when empty) → 1 tenant → straight in', async () => {
    const { backend, user, router } = await renderPortal({ url: '/login' });
    await signIn(user, 'AD.TRUONG.A@THDEMO.EDU.VN');
    expect(await screen.findByRole('heading', { level: 1, name: 'Tổng quan' })).toBeInTheDocument();
    const logins = backend.calls('POST', '/auth/portal/login');
    expect(logins).toHaveLength(1);
    expect(Object.keys(logins[0]!.body as object).sort()).toEqual(['device_id', 'locale', 'password', 'username']);
    expect(logins[0]!.body).toMatchObject({ username: 'AD.TRUONG.A@THDEMO.EDU.VN', locale: 'vi-VN' });
    expect(router.state.location.pathname).toBe('/school-lms/overview');
    expect(backend.calls('GET', '/me/tenants')[0]!.headers['x-tenant-id']).toBeUndefined();
    expect(backend.calls('GET', '/me/permissions')[0]!.headers['x-tenant-id']).toBe('0193a4d2-0000-7000-8000-00000000000a');
    expect(backend.requests.every((r) => r.gateway === 'public' || r.path.startsWith('/admin/'))).toBe(true);
    expect(TEST_CONFIG.apiBaseUrl).not.toMatch(/realms|keycloak/);
  });

  it('401 INVALID_CREDENTIALS → one generic message, password and code cleared, no hint about OTP', async () => {
    const { user } = await renderPortal({ url: '/login' });
    await signIn(user, 'sa.admin', 'Demo@12345', '000000');
    expect(await screen.findByText('Tên đăng nhập, mật khẩu hoặc mã xác thực chưa đúng.')).toBeInTheDocument();
    expect(screen.getByLabelText('Mật khẩu')).toHaveValue('');
    expect(screen.getByLabelText('Mã xác thực 6 số')).toHaveValue('');
    expect(screen.getByLabelText('Tên đăng nhập hoặc email')).toHaveValue('sa.admin');
    expect(screen.queryByText(/bắt buộc mã|cần mã/i, { selector: '[role=alert] *' })).toBeNull();
  });

  it('429 RATE_LIMITED shows the waiting time of Retry-After', async () => {
    const backend = new MockPortalBackend();
    backend.overrides.push((r) => (r.path === '/auth/portal/login' ? (() => {
      const res = problem(429, 'RATE_LIMITED');
      res.headers.set('Retry-After', '30');
      return res;
    })() : undefined));
    const { user } = await renderPortal({ url: '/login', backend });
    await signIn(user, 'ad.truong.a');
    expect(await screen.findByText('Bạn đã thử quá nhiều lần, vui lòng đợi 30 giây.')).toBeInTheDocument();
  });

  it('platform account with code → platform scope; the "code used last time" flag is the only thing remembered', async () => {
    const { backend, user, router } = await renderPortal({ url: '/login' });
    await signIn(user, 'sa.admin', 'Demo@12345', '123456');
    await waitFor(() => expect(router.state.location.pathname).toBe('/school-lms/overview'));
    expect(backend.calls('POST', '/auth/portal/login')[0]!.body).toMatchObject({ otp: '123456' });
    expect(backend.calls('GET', '/me/permissions')[0]!.headers['x-tenant-id']).toBeUndefined();
    expect(window.localStorage.getItem('ew.portal.otpHint')).toBe('true');
  });

  it('student account → student message, no workspace, no /me call (F051-AC-04)', async () => {
    const { backend, user } = await renderPortal({ url: '/login' });
    await signIn(user, 'hs.an.3a');
    expect(await screen.findByText('Tài khoản học sinh không dùng được cổng quản trị')).toBeInTheDocument();
    expect(screen.getByText('Em hãy đăng nhập trong game nhé.')).toBeInTheDocument();
    expect(backend.requests.some((r) => r.path.startsWith('/me/'))).toBe(false);
  });

  it('several tenants → "Chọn đơn vị làm việc"; returnUrl kept for after the choice (F051-AC-03)', async () => {
    const { user, router } = await renderPortal({ url: '/login?returnUrl=%2Fschool-lms%2Fclasses' });
    await signIn(user, 'qlcm.truong.a');
    expect(await screen.findByRole('heading', { name: 'Chọn đơn vị làm việc' })).toBeInTheDocument();
    expect(router.state.location.pathname).toBe('/select-tenant');
    await user.click(screen.getByRole('radio', { name: /TH Demo A/ }));
    await user.click(screen.getByRole('button', { name: 'Vào' }));
    await waitFor(() => expect(router.state.location.pathname).toBe('/school-lms/classes'));
  });

  it('no tenant and no platform role → "chưa được cấp quyền" + sign out (F051-AC-03)', async () => {
    const { user } = await renderPortal({ url: '/login' });
    await signIn(user, 'gv.moi', 'Temp@12345');
    expect(await screen.findByText('Tài khoản chưa được cấp quyền quản trị')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Đăng xuất' })).toBeInTheDocument();
  });
});
