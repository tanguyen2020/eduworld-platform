// F051 T013, T016, T050/T051, T023 (unit level) — F051-AC-04, F051-AC-07, F051-AC-08, F051-AC-28, F051-AC-29
// (TC-AUTH-089, TC-AUTH-095, TC-AUTH-098): F5 restores the session silently (URL and tab tenant kept, token never in
// storage); idle longer than the limit → sign-out and /login keeping the URL; refresh failure → "Phiên làm việc đã
// hết" → /login → back to the same URL; staff-only answer after sign-in → student page + sign-out; sign-out clears
// token, cache and tab tenant even when the network fails; blocked storage does not break sign-in. MOCK backend.
import { screen, waitFor } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { MockPortalBackend, problem, renderPortal, signIn, stopPortals, TENANTS } from '../test/portal-harness';

afterEach(() => {
  stopPortals();
  vi.restoreAllMocks();
});

function storageDump(): string {
  const parts: string[] = [document.cookie];
  for (const s of [window.localStorage, window.sessionStorage]) for (let i = 0; i < s.length; i++) parts.push(`${s.key(i)}=${s.getItem(s.key(i)!)}`);
  return parts.join('\n');
}

describe('reload while the session is valid (F051-AC-28)', () => {
  it('refreshPortalToken with X-EW-CSRF restores URL and the tab tenant without the login page; no token stored', async () => {
    const backend = new MockPortalBackend().withSession('qlcm.truong.a');
    const csrf = backend.csrf;
    window.sessionStorage.setItem('ew.portal.sessionTenantId', TENANTS.pgdDemo.tenant_id);
    const { router, runtime } = await renderPortal({ url: '/school-lms/students?page=2', backend });
    expect(await screen.findByRole('heading', { level: 1, name: 'Học sinh theo khối' })).toBeInTheDocument();
    expect(router.state.location.pathname + router.state.location.search).toBe('/school-lms/students?page=2');
    expect(backend.calls('POST', '/auth/portal/token:refresh')[0]!.headers['x-ew-csrf']).toBe(csrf);
    expect(backend.calls('GET', '/me/permissions')[0]!.headers['x-tenant-id']).toBe(TENANTS.pgdDemo.tenant_id);
    expect(screen.queryByRole('heading', { name: 'Welcome back, teachers' })).toBeNull();
    expect(storageDump()).not.toContain(runtime.tokens.get()!.token);
  });

  it('session gone (401 REFRESH_TOKEN_INVALID / 403 CSRF_TOKEN_INVALID) → /login with returnUrl, no error page', async () => {
    const backend = new MockPortalBackend();
    backend.csrf = 'stale-csrf-value'.padEnd(43, 'x');
    const { router } = await renderPortal({ url: '/school-lms/students', backend });
    expect(await screen.findByRole('heading', { name: 'Welcome back, teachers' })).toBeInTheDocument();
    expect(router.state.location.search).toBe('?returnUrl=%2Fschool-lms%2Fstudents');
  });
});

describe('idle limit (F051-AC-29)', () => {
  it('last interaction older than the idle limit → no refresh, portalLogout, /login keeping the URL', async () => {
    const backend = new MockPortalBackend().withSession('ad.truong.a');
    window.localStorage.setItem('ew.portal.lastActivityAt', new Date(Date.now() - 31 * 60 * 1000).toISOString());
    const { router } = await renderPortal({ url: '/school-lms/students', backend });
    expect(await screen.findByRole('heading', { name: 'Welcome back, teachers' })).toBeInTheDocument();
    expect(backend.calls('POST', '/auth/portal/token:refresh')).toHaveLength(0);
    expect(backend.calls('POST', '/auth/portal/logout')).toHaveLength(1);
    expect(router.state.location.search).toBe('?returnUrl=%2Fschool-lms%2Fstudents');
  });

  it('idle detected while working → sign-out, "session ended" notice on /login', async () => {
    const backend = new MockPortalBackend().withSession('ad.truong.a');
    const { router, runtime } = await renderPortal({ url: '/school-lms/students', backend });
    await screen.findByRole('heading', { level: 1, name: 'Học sinh theo khối' });
    window.localStorage.setItem('ew.portal.lastActivityAt', new Date(Date.now() - 31 * 60 * 1000).toISOString());
    // run the idle check now instead of waiting for the interval
    (runtime.idle as unknown as { memory: number | null }).memory = null;
    expect(runtime.idle.isExpired()).toBe(true);
    (runtime.idle as unknown as { onIdle: () => void }).onIdle();
    expect(await screen.findByText('Your session has ended, please sign in again.')).toBeInTheDocument();
    expect(router.state.location.pathname).toBe('/login');
    expect(backend.calls('POST', '/auth/portal/logout')).toHaveLength(1);
    expect(runtime.tokens.get()).toBeNull();
  });

  it('blocked localStorage: sign-in still works (spec edge case)', async () => {
    vi.spyOn(Storage.prototype, 'getItem').mockImplementation(() => {
      throw new Error('blocked');
    });
    vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => {
      throw new Error('blocked');
    });
    const { user } = await renderPortal({ url: '/login' });
    await signIn(user, 'ad.truong.a');
    expect(await screen.findByRole('heading', { level: 1, name: 'Trường' })).toBeInTheDocument();
  });
});

describe('session expired while working (F051-AC-07)', () => {
  it('refresh fails on TOKEN_EXPIRED → dialog → /login → back to the same URL', async () => {
    const backend = new MockPortalBackend().withSession('ad.truong.a');
    const { router, runtime, user } = await renderPortal({ url: '/school-lms/students', backend });
    await screen.findByRole('heading', { level: 1, name: 'Học sinh theo khối' });
    backend.expiredTokens.add(runtime.tokens.get()!.token);
    backend.refreshAlive = false;
    await runtime.queryClient.refetchQueries({ queryKey: ['me', 'permissions'] });
    // the dialog belongs to the signed-in app: the saved language (vi)
    expect(await screen.findByText('Phiên làm việc đã hết, vui lòng đăng nhập lại.')).toBeInTheDocument();
    expect(runtime.tokens.get()).toBeNull();
    expect(runtime.queryClient.getQueryCache().getAll()).toHaveLength(0);
    await user.click(screen.getByRole('button', { name: 'Đăng nhập lại' }));
    await waitFor(() => expect(router.state.location.pathname).toBe('/login'));
    await signIn(user, 'ad.truong.a');
    await waitFor(() => expect(router.state.location.pathname).toBe('/school-lms/students'));
  });
});

describe('staff only (F051-AC-04)', () => {
  it('403 PORTAL_ACCESS_STAFF_ONLY from an API → student page, sign-out, no workspace', async () => {
    const backend = new MockPortalBackend().withSession('ad.truong.a');
    backend.overrides.push((r) => (r.path === '/me/tenants' ? problem(403, 'PORTAL_ACCESS_STAFF_ONLY') : undefined));
    const { loaders, runtime } = await renderPortal({ url: '/school-lms/schools', backend });
    expect(await screen.findByText('Tài khoản học sinh không dùng được cổng quản trị')).toBeInTheDocument();
    await waitFor(() => expect(backend.calls('POST', '/auth/portal/logout')).toHaveLength(1));
    expect(runtime.tokens.get()).toBeNull();
    expect(loaders.school_lms).not.toHaveBeenCalled();
  });
});

describe('sign-out (F051-AC-08)', () => {
  it.each([
    ['online', false],
    ['network failure', true],
  ])('%s: portalLogout with X-EW-CSRF, token + cache + tab tenant cleared, /login', async (_label, offline) => {
    const backend = new MockPortalBackend().withSession('ad.truong.a');
    const { router, runtime, user } = await renderPortal({ url: '/school-lms/schools', backend });
    await screen.findByRole('heading', { level: 1, name: 'Trường' });
    const csrf = backend.csrf;
    if (offline) backend.overrides.push((r) => (r.path === '/auth/portal/logout' ? Promise.reject(new TypeError('offline')) : undefined));
    await user.click(screen.getByRole('button', { name: /^Tài khoản/ }));
    await user.click(await screen.findByRole('button', { name: 'Đăng xuất' }));
    await waitFor(() => expect(router.state.location.pathname).toBe('/login'));
    const logout = backend.calls('POST', '/auth/portal/logout');
    expect(logout).toHaveLength(1);
    expect(logout[0]!.headers['x-ew-csrf']).toBe(csrf);
    expect(runtime.tokens.get()).toBeNull();
    expect(runtime.tenant.getTenantId()).toBeNull();
    expect(runtime.queryClient.getQueryCache().getAll()).toHaveLength(0);
    expect(window.sessionStorage.getItem('ew.portal.sessionTenantId')).toBeNull();
  });
});
