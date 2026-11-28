// F051 T055 (+T015 at unit level), T062 — F051-AC-01, F051-AC-03, F051-AC-04, F051-AC-30 (TC-AUTH-070/071/072/093/094/095,
// TC-CMS-LOGIN-01): the portal's own /login (no Keycloak request, no tenant before sign-in), one `portalLogin` request
// without any code (portal MFA removed, owner 2026-11-21), one generic error clearing the password, 429 waiting time,
// routing of the demo accounts; layout of direction A "Sân trường" (landmarks, labels, one column on phones).
// Backend = MOCK harness (contract stand-in), not a live integration.
import { screen, waitFor, within } from '@testing-library/react';
import { afterEach, describe, expect, it } from 'vitest';
import { TEST_CONFIG } from '../shared/api/test-utils';
import { DEFAULT_PREFS, PREFS_KEY, readPreferences } from '../app/preferences';
import { MockPortalBackend, problem, renderPortal, setViewportWidth, signIn, stopPortals } from '../test/portal-harness';

afterEach(() => stopPortals());

describe('/login (F051-AC-01, F051-AC-30)', () => {
  it('unauthenticated visit → portal login page with every field; no Keycloak, no tenant step', async () => {
    const { backend, router } = await renderPortal({ url: '/school-lms/students' });
    expect(await screen.findByRole('heading', { level: 1, name: 'Welcome back, teachers' })).toBeInTheDocument();
    expect(router.state.location.pathname).toBe('/login');
    expect(router.state.location.search).toBe('?returnUrl=%2Fschool-lms%2Fstudents');
    expect(screen.getByLabelText('Username or email')).toHaveAttribute('maxlength', '254');
    expect(screen.getByLabelText('Password')).toHaveAttribute('type', 'password');
    // no 6-digit code anywhere (owner 2026-11-21)
    expect(screen.queryByLabelText(/mã xác thực/i)).toBeNull();
    expect(screen.queryByText(/mã xác thực/i)).toBeNull();
    expect(document.querySelector('input[autocomplete="one-time-code"]')).toBeNull();
    expect(screen.getByRole('link', { name: 'Forgot password?' })).toBeInTheDocument();
    // no language selector on the login page (owner 2026-11-23: the login screen is always English)
    expect(screen.queryByRole('combobox')).toBeNull();
    expect(screen.queryByTestId('locale-toggle')).toBeNull();
    expect(screen.queryByText(/trường|đơn vị/i, { selector: 'label' })).toBeNull();
    // the only call at start-up is the silent refresh against the portal gateway (never Keycloak)
    expect(backend.requests.map((r) => `${r.method} ${r.path}`)).toEqual(['POST /auth/portal/token:refresh']);
  });

  it('one portalLogin request with username, password, device_id, locale (no otp when empty) → 1 tenant → straight in', async () => {
    const { backend, user, router } = await renderPortal({ url: '/login' });
    await signIn(user, 'AD.TRUONG.A@THDEMO.EDU.VN');
    // the first menu of workspace C since the Overview menu left Release 1 (owner 2026-11-22, catalog 2026.11.22.1)
    await waitFor(() => expect(router.state.location.pathname).toBe('/school-lms/schools'), { timeout: 30_000 });
    const logins = backend.calls('POST', '/auth/portal/login');
    expect(logins).toHaveLength(1);
    expect(Object.keys(logins[0]!.body as object).sort()).toEqual(['device_id', 'locale', 'password', 'username']);
    expect(logins[0]!.body).toMatchObject({ username: 'AD.TRUONG.A@THDEMO.EDU.VN', locale: 'en-US' });
    expect(logins[0]!.headers['accept-language']).toBe('en-US');
    expect(router.state.location.pathname).toBe('/school-lms/schools');
    expect(backend.calls('GET', '/me/tenants')[0]!.headers['x-tenant-id']).toBeUndefined();
    expect(backend.calls('GET', '/me/permissions')[0]!.headers['x-tenant-id']).toBe('0193a4d2-0000-7000-8000-00000000000a');
    expect(backend.requests.every((r) => r.gateway === 'public' || r.path.startsWith('/admin/'))).toBe(true);
    expect(TEST_CONFIG.apiBaseUrl).not.toMatch(/realms|keycloak/);
  });

  it('401 INVALID_CREDENTIALS → one generic message, password cleared, no hint about the account', async () => {
    const { user } = await renderPortal({ url: '/login' });
    await signIn(user, 'sa.admin', 'Sai@12345');
    expect(await screen.findByText('The username or password is not correct.')).toBeInTheDocument();
    expect(screen.getByLabelText('Password')).toHaveValue('');
    expect(screen.getByLabelText('Username or email')).toHaveValue('sa.admin');
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
    expect(await screen.findByText('Too many attempts, please wait 30 seconds.')).toBeInTheDocument();
  });

  it('platform account with username + password only → platform scope; one request without otp; stale code flag cleared', async () => {
    window.localStorage.setItem('ew.portal.otpHint', 'true');
    const { backend, user, router } = await renderPortal({ url: '/login' });
    await signIn(user, 'sa.admin');
    await waitFor(() => expect(router.state.location.pathname).toBe('/school-lms/schools'));
    const logins = backend.calls('POST', '/auth/portal/login');
    expect(logins).toHaveLength(1);
    expect(logins[0]!.body).not.toHaveProperty('otp');
    expect(backend.calls('GET', '/me/permissions')[0]!.headers['x-tenant-id']).toBeUndefined();
    expect(window.localStorage.getItem('ew.portal.otpHint')).toBeNull();
  });

  it('student account → student message, no workspace, no /me call (F051-AC-04)', async () => {
    const { backend, user } = await renderPortal({ url: '/login' });
    await signIn(user, 'hs.an.3a');
    expect(await screen.findByText('Student accounts cannot use the admin portal')).toBeInTheDocument();
    expect(screen.getByText('Please sign in inside the game.')).toBeInTheDocument();
    expect(backend.requests.some((r) => r.path.startsWith('/me/'))).toBe(false);
  });

  it('several tenants → "Chọn đơn vị làm việc"; returnUrl kept for after the choice (F051-AC-03)', async () => {
    const { user, router } = await renderPortal({ url: '/login?returnUrl=%2Fschool-lms%2Fstudents' });
    await signIn(user, 'qlcm.truong.a');
    expect(await screen.findByRole('heading', { name: 'Chọn đơn vị làm việc' })).toBeInTheDocument();
    expect(router.state.location.pathname).toBe('/select-tenant');
    await user.click(screen.getByRole('radio', { name: /TH Demo A/ }));
    await user.click(screen.getByRole('button', { name: 'Vào' }));
    await waitFor(() => expect(router.state.location.pathname).toBe('/school-lms/students'));
  });

  it('no tenant and no platform role → "chưa được cấp quyền" + sign out (F051-AC-03)', async () => {
    const { user } = await renderPortal({ url: '/login' });
    await signIn(user, 'gv.moi', 'Temp@12345');
    expect(await screen.findByText('Tài khoản chưa được cấp quyền quản trị')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Đăng xuất' })).toBeInTheDocument();
  });
});

describe('/login layout — direction A "Sân trường" (ADMIN_PORTAL §1.3b, owner 2026-11-21)', () => {
  it('desktop: platform banner (complementary: headline, capability list, hidden illustration) + main with labelled form, hints', async () => {
    await renderPortal({ url: '/login' });
    await screen.findByRole('heading', { level: 1, name: 'Welcome back, teachers' });
    expect(screen.getByTestId('auth-layout')).toHaveAttribute('data-layout', 'two-columns');
    const brand = screen.getByRole('complementary', { name: 'About EduWorld' });
    // the banner says the portal runs the whole platform, not only school teaching (owner 2026-11-21)
    expect(within(brand).getByText('EduWorld administration portal')).toBeInTheDocument();
    expect(within(brand).getByText('Author learning content, run the game world and follow students — all in one place.')).toBeInTheDocument();
    const caps = within(brand).getByRole('list', { name: 'What the portal covers' });
    expect(within(caps).getAllByRole('listitem').map((li) => li.textContent)).toEqual([
      'Question bank & exams',
      'Lessons & interactive activities',
      'Game world: quests, items, events',
      'Assign work by grade & follow progress',
      'Safety & community moderation',
    ]);
    // grade wording only (ADR-0095), every picture decorative
    expect(brand).not.toHaveTextContent(/\bclass/i);
    expect(within(brand).getByTestId('banner-illustration')).toHaveAttribute('aria-hidden', 'true');
    for (const svg of brand.querySelectorAll('svg')) expect(svg.closest('[aria-hidden="true"]')).not.toBeNull();
    const main = screen.getByRole('main');
    expect(within(main).getByRole('form', { name: 'Sign in' })).toBeInTheDocument();
    expect(within(main).getByText('Sign in to continue your work.')).toBeInTheDocument();
    expect(within(main).getByText('Need help? Contact your school administrator')).toBeInTheDocument();
    expect(within(main).getByTestId('student-hint')).toHaveTextContent('Are you a student? Open the EduWorld game to start learning.');
    expect(within(main).getByText(/Decree 13\/2023/)).toBeInTheDocument();
    // no URL is hardcoded for the privacy policy or the game (AGENTS R3)
    expect(within(main).getAllByRole('link').map((a) => a.getAttribute('href'))).toEqual(['/forgot-password']);
    for (const input of within(main).getAllByRole('textbox')) expect(input).toHaveAccessibleName();
    expect(within(main).getByLabelText('Password')).toHaveAttribute('type', 'password');
    expect(within(main).getByRole('button', { name: 'Sign in' })).toHaveAttribute('type', 'submit');
  });

  it('phone width: one column, compact banner header with the headline only', async () => {
    setViewportWidth(390);
    await renderPortal({ url: '/login' });
    await screen.findByRole('heading', { level: 1, name: 'Welcome back, teachers' });
    expect(screen.getByTestId('auth-layout')).toHaveAttribute('data-layout', 'one-column');
    const brand = screen.getByRole('complementary', { name: 'About EduWorld' });
    expect(within(brand).getByText('EduWorld administration portal')).toBeInTheDocument();
    expect(within(brand).queryByText('Author learning content, run the game world and follow students — all in one place.')).toBeNull();
    expect(within(brand).queryByRole('list')).toBeNull();
    expect(within(brand).queryByTestId('banner-illustration')).toBeNull();
    expect(screen.getByLabelText('Username or email')).toBeInTheDocument();
  });

  it('forgot-password page uses the same frame', async () => {
    await renderPortal({ url: '/forgot-password' });
    expect(await screen.findByRole('heading', { level: 1, name: 'Quên mật khẩu' })).toBeInTheDocument();
    expect(screen.getByRole('complementary', { name: 'Giới thiệu EduWorld' })).toBeInTheDocument();
    // the other pre-sign-in pages keep the saved language and its toggle (only /login is pinned to English)
    expect(screen.getByTestId('locale-toggle')).toHaveTextContent('VI');
  });
});

describe('/login is always English (owner 2026-11-23)', () => {
  const navigatorLanguage = Object.getOwnPropertyDescriptor(window.navigator, 'language');
  const navigatorLanguages = Object.getOwnPropertyDescriptor(window.navigator, 'languages');
  afterEach(() => {
    if (navigatorLanguage) Object.defineProperty(window.navigator, 'language', navigatorLanguage);
    else delete (window.navigator as { language?: string }).language;
    if (navigatorLanguages) Object.defineProperty(window.navigator, 'languages', navigatorLanguages);
    else delete (window.navigator as { languages?: readonly string[] }).languages;
    document.documentElement.lang = '';
  });

  it('browser set to Vietnamese → English login page, no language selector, <html lang="en-US">', async () => {
    Object.defineProperty(window.navigator, 'language', { configurable: true, value: 'vi-VN' });
    Object.defineProperty(window.navigator, 'languages', { configurable: true, value: ['vi-VN', 'vi'] });
    await renderPortal({ url: '/login' });
    expect(await screen.findByRole('heading', { level: 1, name: 'Welcome back, teachers' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Sign in' })).toBeInTheDocument();
    expect(screen.queryByRole('combobox')).toBeNull();
    expect(screen.queryByText('Tên đăng nhập hoặc email')).toBeNull();
    expect(document.documentElement.lang).toBe('en-US');
  });

  it('saved Vietnamese preference → login page still English; a Problem without its own message is English too', async () => {
    window.localStorage.setItem(PREFS_KEY, JSON.stringify({ ...DEFAULT_PREFS, locale: 'vi-VN' }));
    const backend = new MockPortalBackend();
    backend.overrides.push((r) => (r.path === '/auth/portal/login' ? problem(403, 'ACCOUNT_LOCKED') : undefined));
    const { user, i18n } = await renderPortal({ url: '/login', backend, locale: readPreferences().locale });
    expect(i18n.language).toBe('vi-VN');
    expect(await screen.findByRole('heading', { level: 1, name: 'Welcome back, teachers' })).toBeInTheDocument();
    await signIn(user, 'ad.truong.a');
    const alert = await screen.findByRole('alert');
    expect(alert).toHaveTextContent(i18n.getFixedT('en-US')('error.ACCOUNT_LOCKED'));
    expect(alert).not.toHaveTextContent(i18n.getFixedT('vi-VN')('error.ACCOUNT_LOCKED'));
    expect(backend.calls('POST', '/auth/portal/login')[0]!.headers['accept-language']).toBe('en-US');
  });

  it('after sign-in the saved Vietnamese preference applies again (header VI/EN toggle kept)', async () => {
    window.localStorage.setItem(PREFS_KEY, JSON.stringify({ ...DEFAULT_PREFS, locale: 'vi-VN' }));
    const { user, router, i18n } = await renderPortal({ url: '/login', locale: readPreferences().locale });
    await screen.findByRole('heading', { level: 1, name: 'Welcome back, teachers' });
    await signIn(user, 'ad.truong.a');
    await waitFor(() => expect(router.state.location.pathname).toBe('/school-lms/schools'), { timeout: 30_000 });
    expect(await screen.findByTestId('locale-toggle')).toHaveTextContent('VI');
    expect(screen.getByRole('button', { name: new RegExp(`^${i18n.getFixedT('vi-VN')('shell.userMenu.label')}`) })).toBeInTheDocument();
    expect(document.documentElement.lang).toBe('vi-VN');
  });
});
