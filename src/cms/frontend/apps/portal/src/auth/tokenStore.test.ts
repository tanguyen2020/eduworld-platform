// F051 T012 — F051-AC-02 (TC-AUTH-098): the access token lives in memory only; login/refresh/logout never write it,
// the password, the code, a setup ticket or a reset token to localStorage, sessionStorage, IndexedDB or a cookie;
// the CSRF cookie is only read; `device_id` is a random per-browser UUID (not a secret).
import { afterEach, describe, expect, it, vi } from 'vitest';
import { TEST_CONFIG } from '../shared/api/test-utils';
import { createAuthApi } from './authApi';
import { CSRF_COOKIE, readCsrfToken } from './csrf';
import { deviceId, resetDeviceIdForTest } from './deviceId';
import { setupTicketStore } from './memoryStores';
import { tokenSubject, TokenStore } from './tokenStore';
import { fakeJwt, MockPortalBackend } from '../test/portal-harness';

const SECRET = 'S3cret!pass';

function storageDump(): string {
  const parts: string[] = [document.cookie];
  for (const s of [window.localStorage, window.sessionStorage]) for (let i = 0; i < s.length; i++) parts.push(`${s.key(i)}=${s.getItem(s.key(i)!)}`);
  return parts.join('\n');
}

afterEach(() => {
  window.localStorage.clear();
  window.sessionStorage.clear();
  setupTicketStore.clear();
  resetDeviceIdForTest();
});

describe('token store and portal session API (F051-AC-02)', () => {
  it('keeps the token in memory with its expiry; clear forgets it', () => {
    const store = new TokenStore(() => 1_000);
    const t = store.setFromResponse({ access_token: 'tok', expires_in: 300, refresh_expires_in: 1800, auth_time: '2026-11-19T08:00:00Z', mfa: true });
    expect(t).toMatchObject({ token: 'tok', expiresAt: 301_000, sessionExpiresAt: 1_801_000, mfa: true });
    store.clear();
    expect(store.get()).toBeNull();
  });

  it('login → refresh → logout leave no token, password, code or ticket in browser storage', async () => {
    const backend = new MockPortalBackend();
    const setItem = vi.spyOn(Storage.prototype, 'setItem');
    const api = createAuthApi({ config: TEST_CONFIG, getLocale: () => 'vi-VN', hasTranslation: () => true, fetch: backend.fetch });
    const store = new TokenStore();
    const login = await api.login({ username: 'sa.admin', password: 'Demo@12345', otp: '123456', device_id: deviceId() });
    expect(login.ok).toBe(true);
    if (login.ok) store.setFromResponse(login.data);
    const refreshed = await api.refresh(backend.csrf);
    expect(refreshed.ok).toBe(true);
    await api.logout(backend.csrf);
    const dump = storageDump();
    for (const secret of [store.get()!.token, 'Demo@12345', '123456', 'mock-signature']) expect(dump).not.toContain(secret);
    expect(setItem.mock.calls.map(([k]) => k)).toEqual(['ew.portal.deviceId']);
    setItem.mockRestore();
  });

  it('every portal session call is same-origin public gateway, credentials include, no Authorization/X-Tenant-Id', async () => {
    const backend = new MockPortalBackend();
    const seen: RequestInit[] = [];
    const api = createAuthApi({
      config: TEST_CONFIG,
      getLocale: () => 'en-US',
      hasTranslation: () => true,
      fetch: (async (url: string, init: RequestInit) => {
        seen.push(init);
        return backend.fetch(url, init);
      }) as never,
    });
    await api.login({ username: 'ad.truong.a', password: SECRET, device_id: deviceId() });
    await api.refresh('csrf-value-not-a-secret');
    expect(seen.every((i) => i.credentials === 'include' && i.referrerPolicy === 'no-referrer')).toBe(true);
    expect(backend.requests.map((r) => r.gateway)).toEqual(['public', 'public']);
    for (const r of backend.requests) {
      expect(r.headers.authorization).toBeUndefined();
      expect(r.headers['x-tenant-id']).toBeUndefined();
      expect(r.headers['idempotency-key']).toBeUndefined();
    }
    expect(backend.requests[1]!.headers['x-ew-csrf']).toBe('csrf-value-not-a-secret');
    expect(backend.requests[0]!.headers['accept-language']).toBe('en-US');
  });

  it('403 ACCOUNT_SETUP_REQUIRED exposes the setup ticket and steps (memory only)', async () => {
    const backend = new MockPortalBackend();
    backend.overrides.push(() =>
      new Response(JSON.stringify({ code: 'ACCOUNT_SETUP_REQUIRED', status: 403, title: 'x', type: 'about:blank', setup_ticket: 't'.repeat(40), required_steps: ['update_password', 'configure_otp'], otp_setup_email_sent: false }), {
        status: 403,
        headers: { 'Content-Type': 'application/problem+json' },
      }),
    );
    const api = createAuthApi({ config: TEST_CONFIG, getLocale: () => 'vi-VN', hasTranslation: () => true, fetch: backend.fetch });
    const r = await api.login({ username: 'gv.moi', password: 'x', device_id: deviceId() });
    expect(r.ok).toBe(false);
    if (!r.ok) expect(r.setup).toEqual({ setupTicket: 't'.repeat(40), requiredSteps: ['update_password', 'configure_otp'], otpSetupEmailSent: false });
  });

  it('reads (never writes) the CSRF cookie; rejects malformed values', () => {
    expect(readCsrfToken(`a=b; ${CSRF_COOKIE}=${'A'.repeat(43)}`)).toBe('A'.repeat(43));
    expect(readCsrfToken(`${CSRF_COOKIE}=short`)).toBeNull();
    expect(readCsrfToken('')).toBeNull();
  });

  it('device_id: one random UUID per browser; a new one per load when storage is blocked', () => {
    const first = deviceId();
    expect(first).toMatch(/^[0-9a-f-]{36}$/);
    expect(deviceId()).toBe(first);
    window.localStorage.clear();
    resetDeviceIdForTest();
    const get = vi.spyOn(Storage.prototype, 'getItem').mockImplementation(() => {
      throw new Error('blocked');
    });
    const blocked = deviceId();
    expect(blocked).not.toBe(first);
    get.mockRestore();
  });

  it('subject of the token keys per-viewer storage; garbage → null', () => {
    expect(tokenSubject(fakeJwt('0193a4d2-1111-7000-8000-000000000001', 1))).toBe('0193a4d2-1111-7000-8000-000000000001');
    expect(tokenSubject('not-a-jwt')).toBeNull();
  });
});
