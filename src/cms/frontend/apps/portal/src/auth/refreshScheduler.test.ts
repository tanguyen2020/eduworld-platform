// F051 T057 — F051-AC-33, F051-AC-02 (TC-AUTH-098): one `refreshPortalToken` shared by concurrent callers, sent with
// X-EW-CSRF = CSRF cookie; a tab that lost the rotation race (401 REFRESH_TOKEN_INVALID while the cookie changed)
// re-reads the cookie and retries once; proactive refresh before expiry only while active; failure clears the token.
// Also the apiClient path: several requests with 401 TOKEN_EXPIRED → one refresh, each retried once.
import { describe, expect, it, vi } from 'vitest';
import { ApiError } from '../shared/api/middleware/problem';
import { createApiClient } from '../shared/api/client';
import { TEST_CONFIG } from '../shared/api/test-utils';
import type { AuthApi, AuthResult, PortalTokenResponse } from './authApi';
import { refreshLeadMs, RefreshScheduler } from './refreshScheduler';
import { TokenStore } from './tokenStore';

const body = (token: string): PortalTokenResponse => ({ access_token: token, token_type: 'Bearer', expires_in: 300, refresh_expires_in: 1800, auth_time: '2026-11-19T08:00:00Z' });
const ok = (token: string): AuthResult<PortalTokenResponse> => ({ ok: true, status: 200, data: body(token) });
const fail = (status: number, code: string): AuthResult<PortalTokenResponse> => ({ ok: false, status, error: new ApiError({ status, code, i18nKey: `error.${code}` }) });

/** MOCK authApi.refresh (test only). */
function fakeAuth(results: AuthResult<PortalTokenResponse>[]) {
  const sent: (string | null)[] = [];
  const refresh = vi.fn(async (csrf: string | null) => {
    sent.push(csrf);
    await Promise.resolve();
    return results.shift() ?? fail(401, 'REFRESH_TOKEN_INVALID');
  });
  return { api: { refresh } as Pick<AuthApi, 'refresh'>, sent, refresh };
}

describe('RefreshScheduler', () => {
  it('single-flight: concurrent callers share one refresh with the CSRF header value', async () => {
    const { api, sent, refresh } = fakeAuth([ok('t2')]);
    const store = new TokenStore();
    const s = new RefreshScheduler({ authApi: api, store, readCsrf: () => 'csrf-a', locks: null, setTimer: () => 0, clearTimer: () => undefined });
    const results = await Promise.all([s.refresh(), s.refresh(), s.refresh()]);
    expect(results).toEqual([true, true, true]);
    expect(refresh).toHaveBeenCalledTimes(1);
    expect(sent).toEqual(['csrf-a']);
    expect(await s.getAccessToken()).toBe('t2');
  });

  it('lost rotation race (cookie changed meanwhile): re-reads the CSRF cookie and retries once', async () => {
    let cookie = 'csrf-old';
    const { api, sent } = fakeAuth([fail(401, 'REFRESH_TOKEN_INVALID'), ok('t3')]);
    const s = new RefreshScheduler({
      authApi: { refresh: async (c) => { const r = await api.refresh(c); cookie = 'csrf-new'; return r; } },
      store: new TokenStore(),
      readCsrf: () => cookie,
      locks: null,
      setTimer: () => 0,
      clearTimer: () => undefined,
    });
    expect(await s.refreshDetailed()).toEqual({ ok: true });
    expect(sent).toEqual(['csrf-old', 'csrf-new']);
  });

  it('no retry when the cookie did not change; failure clears the token', async () => {
    const { api, refresh } = fakeAuth([fail(401, 'REFRESH_TOKEN_INVALID')]);
    const store = new TokenStore();
    store.setFromResponse(body('t1'));
    const s = new RefreshScheduler({ authApi: api, store, readCsrf: () => 'same', locks: null, setTimer: () => 0, clearTimer: () => undefined });
    expect(await s.refreshDetailed()).toEqual({ ok: false, status: 401, code: 'REFRESH_TOKEN_INVALID' });
    expect(refresh).toHaveBeenCalledTimes(1);
    expect(store.get()).toBeNull();
  });

  it('serializes refreshes across tabs with the Web Locks API when present', async () => {
    const { api } = fakeAuth([ok('t2')]);
    const request = vi.fn((_name: string, cb: () => Promise<unknown>) => cb());
    const s = new RefreshScheduler({ authApi: api, store: new TokenStore(), readCsrf: () => 'c', locks: { request: request as never }, setTimer: () => 0, clearTimer: () => undefined });
    await s.refresh();
    expect(request).toHaveBeenCalledWith('ew.portal.refresh', expect.any(Function));
  });

  it('schedules the proactive refresh before expiry; skipped while idle; failure → onExpired', async () => {
    const timers: { fn: () => void; ms: number }[] = [];
    let active = true;
    const onExpired = vi.fn();
    const { api, refresh } = fakeAuth([fail(401, 'REFRESH_TOKEN_INVALID')]);
    const store = new TokenStore(() => 0);
    store.setFromResponse(body('t1'));
    const s = new RefreshScheduler({ authApi: api, store, readCsrf: () => 'c', locks: null, now: () => 0, isActive: () => active, onExpired, setTimer: (fn, ms) => timers.push({ fn, ms }), clearTimer: () => undefined });
    s.schedule();
    expect(timers[0]!.ms).toBe(300_000 - refreshLeadMs(300_000));
    active = false;
    timers[0]!.fn();
    expect(refresh).not.toHaveBeenCalled();
    s.schedule();
    active = true;
    timers[1]!.fn();
    await vi.waitFor(() => expect(onExpired).toHaveBeenCalledWith({ ok: false, status: 401, code: 'REFRESH_TOKEN_INVALID' }), { timeout: 10_000 });
  });

  it('apiClient: 3 parallel requests with 401 TOKEN_EXPIRED → one refresh, each request retried once (F051-AC-33)', async () => {
    const { api: auth, refresh } = fakeAuth([ok('fresh')]);
    const store = new TokenStore();
    store.setFromResponse(body('stale'));
    const scheduler = new RefreshScheduler({ authApi: auth, store, readCsrf: () => 'c', locks: null, setTimer: () => 0, clearTimer: () => undefined });
    const seen: string[] = [];
    // MOCK gateway (test only)
    const fetchMock = vi.fn(async (req: Request) => {
      const token = req.headers.get('Authorization');
      seen.push(token ?? '');
      if (token === 'Bearer stale') return new Response(JSON.stringify({ code: 'TOKEN_EXPIRED', status: 401, title: 'x', type: 'about:blank' }), { status: 401, headers: { 'Content-Type': 'application/problem+json' } });
      return new Response(JSON.stringify({ items: [], has_platform_role: false }), { status: 200, headers: { 'Content-Type': 'application/json' } });
    });
    const client = createApiClient({ config: TEST_CONFIG, tokenProvider: scheduler, tenant: { getTenantId: () => null }, getLocale: () => 'vi-VN', hasTranslation: () => true, fetch: fetchMock as never });
    const responses = await Promise.all([1, 2, 3].map(() => client.client.GET('/me/tenants', {})));
    expect(responses.map((r) => r.response.status)).toEqual([200, 200, 200]);
    expect(refresh).toHaveBeenCalledTimes(1);
    expect(seen.filter((s) => s === 'Bearer fresh')).toHaveLength(3);
    expect(seen.filter((s) => s === 'Bearer stale')).toHaveLength(3);
  });
});
