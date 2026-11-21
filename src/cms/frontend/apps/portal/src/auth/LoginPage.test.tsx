// F051 T055 (+T015 at unit level), T062 — F051-AC-01, F051-AC-03, F051-AC-04, F051-AC-30 (TC-AUTH-070/071/072/093/094/095,
// TC-CMS-LOGIN-01): the portal's own /login (no Keycloak request, no tenant before sign-in), one `portalLogin` request
// without any code (portal MFA removed, owner 2026-11-21), one generic error clearing the password, 429 waiting time,
// routing of the demo accounts; layout of direction A "Sân trường" (landmarks, labels, one column on phones).
// Backend = MOCK harness (contract stand-in), not a live integration.
import { screen, waitFor, within } from '@testing-library/react';
import { afterEach, describe, expect, it } from 'vitest';
import { TEST_CONFIG } from '../shared/api/test-utils';
import { MockPortalBackend, problem, renderPortal, setViewportWidth, signIn, stopPortals } from '../test/portal-harness';

afterEach(() => stopPortals());

describe('/login (F051-AC-01, F051-AC-30)', () => {
  it('unauthenticated visit → portal login page with every field; no Keycloak, no tenant step', async () => {
    const { backend, router } = await renderPortal({ url: '/school-lms/classes' });
    expect(await screen.findByRole('heading', { level: 1, name: 'Chào mừng thầy cô trở lại' })).toBeInTheDocument();
    expect(router.state.location.pathname).toBe('/login');
    expect(router.state.location.search).toBe('?returnUrl=%2Fschool-lms%2Fclasses');
    expect(screen.getByLabelText('Tên đăng nhập hoặc email')).toHaveAttribute('maxlength', '254');
    expect(screen.getByLabelText('Mật khẩu')).toHaveAttribute('type', 'password');
    // no 6-digit code anywhere (owner 2026-11-21)
    expect(screen.queryByLabelText(/mã xác thực/i)).toBeNull();
    expect(screen.queryByText(/mã xác thực/i)).toBeNull();
    expect(document.querySelector('input[autocomplete="one-time-code"]')).toBeNull();
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

  it('401 INVALID_CREDENTIALS → one generic message, password cleared, no hint about the account', async () => {
    const { user } = await renderPortal({ url: '/login' });
    await signIn(user, 'sa.admin', 'Sai@12345');
    expect(await screen.findByText('Tên đăng nhập hoặc mật khẩu chưa đúng.')).toBeInTheDocument();
    expect(screen.getByLabelText('Mật khẩu')).toHaveValue('');
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

  it('platform account with username + password only → platform scope; one request without otp; stale code flag cleared', async () => {
    window.localStorage.setItem('ew.portal.otpHint', 'true');
    const { backend, user, router } = await renderPortal({ url: '/login' });
    await signIn(user, 'sa.admin');
    await waitFor(() => expect(router.state.location.pathname).toBe('/school-lms/overview'));
    const logins = backend.calls('POST', '/auth/portal/login');
    expect(logins).toHaveLength(1);
    expect(logins[0]!.body).not.toHaveProperty('otp');
    expect(backend.calls('GET', '/me/permissions')[0]!.headers['x-tenant-id']).toBeUndefined();
    expect(window.localStorage.getItem('ew.portal.otpHint')).toBeNull();
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

describe('/login layout — direction A "Sân trường" (ADMIN_PORTAL §1.3b, owner 2026-11-21)', () => {
  it('desktop: platform banner (complementary: headline, capability list, hidden illustration) + main with labelled form, hints', async () => {
    await renderPortal({ url: '/login' });
    await screen.findByRole('heading', { level: 1, name: 'Chào mừng thầy cô trở lại' });
    expect(screen.getByTestId('auth-layout')).toHaveAttribute('data-layout', 'two-columns');
    const brand = screen.getByRole('complementary', { name: 'Giới thiệu EduWorld' });
    // the banner says the portal runs the whole platform, not only school teaching (owner 2026-11-21)
    expect(within(brand).getByText('Cổng quản trị EduWorld')).toBeInTheDocument();
    expect(within(brand).getByText('Soạn nội dung học tập, quản lý thế giới game và theo dõi học sinh — trong một nơi.')).toBeInTheDocument();
    const caps = within(brand).getByRole('list', { name: 'Những việc cổng quản trị hỗ trợ' });
    expect(within(caps).getAllByRole('listitem').map((li) => li.textContent)).toEqual([
      'Ngân hàng câu hỏi & đề thi',
      'Bài học & hoạt động tương tác',
      'Thế giới game: nhiệm vụ, vật phẩm, sự kiện',
      'Giao bài theo khối & theo dõi tiến bộ',
      'An toàn & kiểm duyệt cộng đồng',
    ]);
    // grade wording only (ADR-0095), every picture decorative
    expect(brand).not.toHaveTextContent(/lớp/i);
    expect(within(brand).getByTestId('banner-illustration')).toHaveAttribute('aria-hidden', 'true');
    for (const svg of brand.querySelectorAll('svg')) expect(svg.closest('[aria-hidden="true"]')).not.toBeNull();
    const main = screen.getByRole('main');
    expect(within(main).getByRole('form', { name: 'Đăng nhập' })).toBeInTheDocument();
    expect(within(main).getByText('Đăng nhập để tiếp tục công việc của bạn.')).toBeInTheDocument();
    expect(within(main).getByText('Cần hỗ trợ? Liên hệ quản trị trường')).toBeInTheDocument();
    expect(within(main).getByTestId('student-hint')).toHaveTextContent('Em là học sinh? Hãy mở game EduWorld để vào học nhé.');
    expect(within(main).getByText(/Nghị định 13\/2023/)).toBeInTheDocument();
    // no URL is hardcoded for the privacy policy or the game (AGENTS R3)
    expect(within(main).getAllByRole('link').map((a) => a.getAttribute('href'))).toEqual(['/forgot-password']);
    for (const input of within(main).getAllByRole('textbox')) expect(input).toHaveAccessibleName();
    expect(within(main).getByLabelText('Mật khẩu')).toHaveAttribute('type', 'password');
    expect(within(main).getByRole('button', { name: 'Đăng nhập' })).toHaveAttribute('type', 'submit');
  });

  it('phone width: one column, compact banner header with the headline only', async () => {
    setViewportWidth(390);
    await renderPortal({ url: '/login' });
    await screen.findByRole('heading', { level: 1, name: 'Chào mừng thầy cô trở lại' });
    expect(screen.getByTestId('auth-layout')).toHaveAttribute('data-layout', 'one-column');
    const brand = screen.getByRole('complementary', { name: 'Giới thiệu EduWorld' });
    expect(within(brand).getByText('Cổng quản trị EduWorld')).toBeInTheDocument();
    expect(within(brand).queryByText('Soạn nội dung học tập, quản lý thế giới game và theo dõi học sinh — trong một nơi.')).toBeNull();
    expect(within(brand).queryByRole('list')).toBeNull();
    expect(within(brand).queryByTestId('banner-illustration')).toBeNull();
    expect(screen.getByLabelText('Tên đăng nhập hoặc email')).toBeInTheDocument();
  });

  it('forgot-password page uses the same frame', async () => {
    await renderPortal({ url: '/forgot-password' });
    expect(await screen.findByRole('heading', { level: 1, name: 'Quên mật khẩu' })).toBeInTheDocument();
    expect(screen.getByRole('complementary', { name: 'Giới thiệu EduWorld' })).toBeInTheDocument();
  });
});
