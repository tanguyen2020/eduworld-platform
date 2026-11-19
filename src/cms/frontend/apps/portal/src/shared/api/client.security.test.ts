// F050-AC-17 (AGENTS R14): after a batch of calls (success, errors, refresh, network failure) no token is in
// localStorage / sessionStorage / document.cookie, and nothing was written to the console. MSW gateway (MOCK).
import { http, HttpResponse } from 'msw';
import { setupServer } from 'msw/node';
import { afterAll, beforeAll, describe, expect, it, vi } from 'vitest';
import { INTERNAL, PUBLIC, SECRET_TOKEN, problem, testClient } from './test-utils';

let first = true;
const server = setupServer(
  http.get(`${PUBLIC}/me/tenants`, () => HttpResponse.json({ items: [] })),
  http.get(`${PUBLIC}/me/permissions`, () => {
    if (first) {
      first = false;
      return problem(401, 'TOKEN_EXPIRED');
    }
    return problem(403, 'TENANT_MISMATCH');
  }),
  http.post(`${PUBLIC}/auth/portal/login`, () => problem(401, 'INVALID_CREDENTIALS')),
  http.all(`${INTERNAL}/*`, () => HttpResponse.error()),
);

beforeAll(() => server.listen({ onUnhandledRequest: 'error' }));
afterAll(() => server.close());

describe('apiClient security', () => {
  it('keeps tokens out of browser storage and the console', async () => {
    const spies = (['log', 'info', 'warn', 'error', 'debug'] as const).map((m) => vi.spyOn(console, m).mockImplementation(() => {}));
    const { api } = testClient();
    await api.client.GET('/me/tenants', {});
    await api.client.GET('/me/permissions', {});
    await api.client.POST('/auth/portal/login', { body: { username: 'qlcm.truong.a', password: 'P@ssw0rd-not-real' } as never });
    await api.fetch(new Request(`${INTERNAL}/admin/analytics/game/metrics?from=2026-11-01&to=2026-11-19`)).catch(() => undefined);

    const storage = [
      ...Object.keys(localStorage).map((k) => `${k}=${localStorage.getItem(k)}`),
      ...Object.keys(sessionStorage).map((k) => `${k}=${sessionStorage.getItem(k)}`),
      document.cookie,
    ].join('\n');
    expect(storage).not.toContain(SECRET_TOKEN);
    expect(storage).not.toMatch(/Bearer|Authorization/i);

    const logged = spies.flatMap((s) => s.mock.calls.flat()).map(String).join('\n');
    expect(logged).not.toContain(SECRET_TOKEN);
    expect(logged).not.toMatch(/Authorization|P@ssw0rd/);
    for (const s of spies) expect(s).not.toHaveBeenCalled();
    spies.forEach((s) => s.mockRestore());
  });
});
