// @vitest-environment node
// F050-AC-16: only GET or requests with Idempotency-Key are retried, on network error or 502/503/504, at most
// MAX_RETRIES times with bounded backoff; 4xx (except 429) never. MSW gateway (MOCK).
import { http, HttpResponse } from 'msw';
import { setupServer } from 'msw/node';
import { afterAll, afterEach, beforeAll, describe, expect, it } from 'vitest';
import { PUBLIC, problem, testClient } from '../test-utils';
import { backoffMs, MAX_RETRIES, parseRetryAfter } from './retry';

let calls = 0;
const server = setupServer();
beforeAll(() => server.listen({ onUnhandledRequest: 'error' }));
afterEach(() => {
  calls = 0;
  server.resetHandlers();
});
afterAll(() => server.close());

const always = (method: 'get' | 'post', path: string, make: () => Response) =>
  server.use(
    http[method](`${PUBLIC}${path}`, () => {
      calls++;
      return make();
    }),
  );

describe('retry policy', () => {
  it('GET 503 → retried MAX_RETRIES times with growing backoff', async () => {
    always('get', '/me/tenants', () => new HttpResponse(null, { status: 503 }));
    const { api, sleeps } = testClient();
    const { response } = await api.client.GET('/me/tenants', {});
    expect(response.status).toBe(503);
    expect(calls).toBe(1 + MAX_RETRIES);
    expect(sleeps).toEqual([150, 300]); // 100·2ⁿ + jitter (random 0.5)
  });

  it.each([502, 504])('GET %i is retried', async (status) => {
    always('get', '/me/tenants', () => new HttpResponse(null, { status }));
    const { api } = testClient();
    await api.client.GET('/me/tenants', {});
    expect(calls).toBe(3);
  });

  it('POST without Idempotency-Key is not retried on 503', async () => {
    always('post', '/auth/game/logout', () => new HttpResponse(null, { status: 503 }));
    const { api, sleeps } = testClient();
    const res = await api.fetch(new Request(`${PUBLIC}/auth/game/logout`, { method: 'POST', body: '{}' }));
    expect(res.status).toBe(503);
    expect(calls).toBe(1);
    expect(sleeps).toEqual([]);
  });

  it('POST with Idempotency-Key is retried on 503', async () => {
    always('post', '/auth/sessions\\:revoke-all', () => new HttpResponse(null, { status: 503 }));
    const { api } = testClient();
    await api.client.POST('/auth/sessions:revoke-all', {});
    expect(calls).toBe(3);
  });

  it('POST without key is not retried on a network error', async () => {
    always('post', '/auth/game/logout', () => HttpResponse.error());
    const { api } = testClient();
    await expect(api.fetch(new Request(`${PUBLIC}/auth/game/logout`, { method: 'POST', body: '{}' }))).rejects.toMatchObject({ code: 'NETWORK_ERROR' });
    expect(calls).toBe(1);
  });

  it.each([400, 404, 409, 422])('GET %i is never retried', async (status) => {
    always('get', '/me/tenants', () => problem(status, 'BAD_REQUEST'));
    const { api } = testClient();
    await api.client.GET('/me/tenants', {});
    expect(calls).toBe(1);
  });

  it('backoff is bounded and Retry-After parses seconds and dates', () => {
    expect(backoffMs(0, () => 0)).toBe(100);
    expect(backoffMs(1, () => 0.999)).toBeLessThan(400);
    expect(parseRetryAfter('2')).toBe(2000);
    expect(parseRetryAfter('Thu, 19 Nov 2026 08:00:05 GMT', Date.UTC(2026, 10, 19, 8, 0, 0))).toBe(5000);
    expect(parseRetryAfter('soon')).toBeNull();
  });
});
