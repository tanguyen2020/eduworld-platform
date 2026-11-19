// @vitest-environment node
// F050-AC-15 (TC-CMS-WS-02): 401 TOKEN_EXPIRED → refresh() once then replay; refresh failure → session_expired;
// 403 TENANT_MISMATCH → tenant_mismatch; 403 MFA_REQUIRED → mfa_required; 429 waits for Retry-After.
// MSW gateway and fake TokenProvider (MOCK).
import { http, HttpResponse } from 'msw';
import { setupServer } from 'msw/node';
import { afterAll, afterEach, beforeAll, describe, expect, it } from 'vitest';
import { FakeTokenProvider, PUBLIC, SECRET_TOKEN, problem, testClient } from '../test-utils';

const auths: (string | null)[] = [];
const server = setupServer();

beforeAll(() => server.listen({ onUnhandledRequest: 'error' }));
afterEach(() => {
  auths.length = 0;
  server.resetHandlers();
});
afterAll(() => server.close());

const expiredThenOk = () =>
  http.get(`${PUBLIC}/me/tenants`, ({ request }) => {
    const a = request.headers.get('Authorization');
    auths.push(a);
    return a === `Bearer ${SECRET_TOKEN}` ? problem(401, 'TOKEN_EXPIRED') : HttpResponse.json({ items: [] });
  });

describe('token expiry', () => {
  it('refreshes exactly once and replays the request with the new token', async () => {
    server.use(expiredThenOk());
    const { api, tokenProvider } = testClient();
    const { response } = await api.client.GET('/me/tenants', {});
    expect(response.status).toBe(200);
    expect(tokenProvider.refreshCalls).toBe(1);
    expect(auths).toEqual([`Bearer ${SECRET_TOKEN}`, `Bearer ${SECRET_TOKEN}-refreshed`]);
  });

  it('concurrent expired requests share one refresh', async () => {
    server.use(expiredThenOk());
    const { api, tokenProvider } = testClient();
    await Promise.all([api.client.GET('/me/tenants', {}), api.client.GET('/me/tenants', {})]);
    expect(tokenProvider.refreshCalls).toBe(1);
  });

  it('refresh failure → session_expired and the 401 is returned', async () => {
    server.use(expiredThenOk());
    const { api } = testClient({ tokenProvider: new FakeTokenProvider(SECRET_TOKEN, false) });
    const events: string[] = [];
    api.events.on('session_expired', () => events.push('session_expired'));
    const { response } = await api.client.GET('/me/tenants', {});
    expect(response.status).toBe(401);
    expect(events).toEqual(['session_expired']);
  });

  it('still expired after the refresh → session_expired, no second refresh', async () => {
    server.use(http.get(`${PUBLIC}/me/tenants`, () => problem(401, 'TOKEN_EXPIRED')));
    const { api, tokenProvider } = testClient();
    const events: string[] = [];
    api.events.on('session_expired', () => events.push('x'));
    const { response } = await api.client.GET('/me/tenants', {});
    expect(response.status).toBe(401);
    expect(tokenProvider.refreshCalls).toBe(1);
    expect(events).toHaveLength(1);
  });
});

describe('403 events', () => {
  it('TENANT_MISMATCH → tenant_mismatch', async () => {
    server.use(http.get(`${PUBLIC}/me/permissions`, () => problem(403, 'TENANT_MISMATCH')));
    const { api } = testClient();
    const events: string[] = [];
    api.events.on('tenant_mismatch', () => events.push('tenant_mismatch'));
    await api.client.GET('/me/permissions', {});
    expect(events).toEqual(['tenant_mismatch']);
  });

  it('MFA_REQUIRED → mfa_required', async () => {
    server.use(http.get(`${PUBLIC}/me/permissions`, () => problem(403, 'MFA_REQUIRED')));
    const { api } = testClient();
    const events: string[] = [];
    api.events.on('mfa_required', () => events.push('mfa_required'));
    await api.client.GET('/me/permissions', {});
    expect(events).toEqual(['mfa_required']);
  });
});

describe('429', () => {
  it('waits for Retry-After before retrying a GET', async () => {
    let calls = 0;
    server.use(
      http.get(`${PUBLIC}/me/tenants`, () => {
        calls++;
        return calls === 1 ? new HttpResponse(null, { status: 429, headers: { 'Retry-After': '3' } }) : HttpResponse.json({ items: [] });
      }),
    );
    const { api, sleeps } = testClient();
    const { response } = await api.client.GET('/me/tenants', {});
    expect(response.status).toBe(200);
    expect(sleeps).toEqual([3000]);
  });

  it('a non-idempotent write is not retried; ApiError carries retryAfterMs', async () => {
    let calls = 0;
    server.use(
      http.post(`${PUBLIC}/auth/game/logout`, () => {
        calls++;
        return problem(429, 'RATE_LIMITED', {});
      }),
    );
    const { api, sleeps } = testClient();
    const res = api.fetch(new Request(`${PUBLIC}/auth/game/logout`, { method: 'POST', body: '{}' }));
    const r = await res;
    expect(r.status).toBe(429);
    expect(calls).toBe(1);
    expect(sleeps).toEqual([]);
  });
});
