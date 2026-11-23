// F051 T056 — F051-AC-31, F051-AC-32 (TC-AUTH-096, TC-AUTH-097): first sign-in setup (new password with policy
// errors per field, "check your email" step, expired ticket → /login), forgot password (same message for every
// identifier), reset link (token removed from the URL at once, never stored, invalid link → request again).
// Backend = MOCK harness.
import { fireEvent, screen, waitFor } from '@testing-library/react';
import fs from 'node:fs';
import path from 'node:path';
import { afterEach, describe, expect, it } from 'vitest';
import { ACCOUNTS, MockPortalBackend, problem, renderPortal, signIn, stopPortals } from '../test/portal-harness';
import { takeResetTokenFromUrl } from './AccountRecoveryPages';
import { resetTokenStore, setupTicketStore } from './memoryStores';

afterEach(() => {
  stopPortals();
  delete ACCOUNTS['gv.moi']!.setupRequired;
  setupTicketStore.clear();
  resetTokenStore.clear();
});

const RESET = 'reset-token-0123456789abcdef0123456789abcdef';

describe('account setup (F051-AC-31)', () => {
  it('update_password: policy error per field, then success → /login with notice; ticket only in memory', async () => {
    ACCOUNTS['gv.moi']!.setupRequired = { steps: ['update_password'] };
    const { user, router, backend } = await renderPortal({ url: '/login' });
    await signIn(user, 'gv.moi', 'Temp@12345');
    expect(await screen.findByRole('heading', { name: 'Đặt mật khẩu mới' })).toBeInTheDocument();
    expect(router.state.location.state).toBeNull();
    expect(JSON.stringify({ ...window.localStorage, ...window.sessionStorage })).not.toContain('setup-ticket');
    fireEvent.change(screen.getByLabelText('Mật khẩu mới'), { target: { value: 'short' } });
    fireEvent.change(screen.getByLabelText('Nhập lại mật khẩu mới'), { target: { value: 'short' } });
    await user.click(screen.getByRole('button', { name: 'Lưu và đăng nhập lại' }));
    expect(await screen.findByText('Mật khẩu chưa đạt chính sách mật khẩu.')).toBeInTheDocument();
    fireEvent.change(screen.getByLabelText('Mật khẩu mới'), { target: { value: 'LongEnough@2026' } });
    fireEvent.change(screen.getByLabelText('Nhập lại mật khẩu mới'), { target: { value: 'LongEnough@2026' } });
    await user.click(screen.getByRole('button', { name: 'Lưu và đăng nhập lại' }));
    expect(await screen.findByText('Password changed. Sign in with your new password.')).toBeInTheDocument();
    expect(router.state.location.pathname).toBe('/login');
    expect(backend.calls('POST', '/auth/portal/setup:password')).toHaveLength(2);
    expect(setupTicketStore.get()).toBeNull();
  });

  it('mismatching confirmation is caught before calling the API', async () => {
    ACCOUNTS['gv.moi']!.setupRequired = { steps: ['update_password'] };
    const { user, backend } = await renderPortal({ url: '/login' });
    await signIn(user, 'gv.moi', 'Temp@12345');
    await screen.findByRole('heading', { name: 'Đặt mật khẩu mới' });
    fireEvent.change(screen.getByLabelText('Mật khẩu mới'), { target: { value: 'LongEnough@2026' } });
    fireEvent.change(screen.getByLabelText('Nhập lại mật khẩu mới'), { target: { value: 'Different@2026' } });
    await user.click(screen.getByRole('button', { name: 'Lưu và đăng nhập lại' }));
    expect(await screen.findByText('Hai mật khẩu chưa khớp.')).toBeInTheDocument();
    expect(backend.calls('POST', '/auth/portal/setup:password')).toHaveLength(0);
  });

  it('401 SETUP_TICKET_INVALID → /login with the expired notice', async () => {
    ACCOUNTS['gv.moi']!.setupRequired = { steps: ['update_password'] };
    const backend = new MockPortalBackend();
    backend.overrides.push((r) => (r.path === '/auth/portal/setup:password' ? problem(401, 'SETUP_TICKET_INVALID') : undefined));
    const { user, router } = await renderPortal({ url: '/login', backend });
    await signIn(user, 'gv.moi', 'Temp@12345');
    await screen.findByRole('heading', { name: 'Đặt mật khẩu mới' });
    fireEvent.change(screen.getByLabelText('Mật khẩu mới'), { target: { value: 'LongEnough@2026' } });
    fireEvent.change(screen.getByLabelText('Nhập lại mật khẩu mới'), { target: { value: 'LongEnough@2026' } });
    await user.click(screen.getByRole('button', { name: 'Lưu và đăng nhập lại' }));
    expect(await screen.findByText('The account setup has expired. Please sign in again.')).toBeInTheDocument();
    expect(router.state.location.pathname).toBe('/login');
  });

  it('opening /account-setup without a ticket → /login', async () => {
    const { router } = await renderPortal({ url: '/account-setup' });
    await waitFor(() => expect(router.state.location.pathname).toBe('/login'));
  });
});

describe('forgot and reset password (F051-AC-32)', () => {
  it.each(['ad.truong.a', 'nobody@example.test'])('forgot password for %s → the same confirmation', async (id) => {
    const { user, backend } = await renderPortal({ url: '/forgot-password' });
    await user.type(await screen.findByLabelText('Tên đăng nhập hoặc email'), id);
    await user.click(screen.getByRole('button', { name: 'Gửi hướng dẫn' }));
    expect(await screen.findByText('Nếu tài khoản tồn tại, email hướng dẫn đã được gửi.')).toBeInTheDocument();
    expect(backend.calls('POST', '/auth/portal/password:forgot')[0]!.body).toMatchObject({ identifier: id, locale: 'vi-VN' });
  });

  it('reads the token then removes it from the URL at once (history.replaceState), keeps it in memory only', () => {
    window.history.replaceState(null, '', `/reset-password?token=${RESET}&x=1`);
    expect(takeResetTokenFromUrl()).toBe(RESET);
    expect(window.location.search).toBe('?x=1');
    expect(JSON.stringify(window.history.state)).not.toContain(RESET);
    expect(JSON.stringify({ ...window.localStorage, ...window.sessionStorage })).not.toContain(RESET);
    window.history.replaceState(null, '', '/');
  });

  it('reset with a valid token → /login with notice; invalid link → "no longer valid" + request again', async () => {
    resetTokenStore.set(RESET);
    const first = await renderPortal({ url: '/reset-password' });
    fireEvent.change(await screen.findByLabelText('Mật khẩu mới'), { target: { value: 'LongEnough@2026' } });
    fireEvent.change(screen.getByLabelText('Nhập lại mật khẩu mới'), { target: { value: 'LongEnough@2026' } });
    await first.user.click(screen.getByRole('button', { name: 'Đặt lại mật khẩu' }));
    expect(await screen.findByText('Password reset. Sign in with your new password.')).toBeInTheDocument();
    expect(first.backend.calls('POST', '/auth/portal/password:reset')[0]!.body).toMatchObject({ token: RESET });
  });

  it('401 PASSWORD_RESET_TOKEN_INVALID → invalid link page with "Gửi lại email"', async () => {
    resetTokenStore.set('reset-token-already-used-0123456789abcdef');
    const { user, router } = await renderPortal({ url: '/reset-password' });
    fireEvent.change(await screen.findByLabelText('Mật khẩu mới'), { target: { value: 'LongEnough@2026' } });
    fireEvent.change(screen.getByLabelText('Nhập lại mật khẩu mới'), { target: { value: 'LongEnough@2026' } });
    await user.click(screen.getByRole('button', { name: 'Đặt lại mật khẩu' }));
    expect(await screen.findByText('Link đặt lại mật khẩu không còn hiệu lực')).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'Gửi lại email' }));
    await waitFor(() => expect(router.state.location.pathname).toBe('/forgot-password'));
  });

  it('Referrer-Policy: no-referrer for the whole portal (index.html)', () => {
    const html = fs.readFileSync(path.resolve(__dirname, '../../index.html'), 'utf8');
    expect(html).toMatch(/<meta name="referrer" content="no-referrer"/);
  });
});
