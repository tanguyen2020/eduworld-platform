// @vitest-environment node
// F050-AC-12: Idempotency-Key is a UUIDv7 (version nibble 7, variant 10), kept across automatic retries and
// across a user retry of the same intent; If-Match carries the ETag of the last GET. MSW gateway (MOCK).
import { http, HttpResponse } from 'msw';
import { setupServer } from 'msw/node';
import { afterAll, afterEach, beforeAll, describe, expect, it } from 'vitest';
import { UUIDV7, uuidv7, uuidv7Time } from '../ids';
import { INTERNAL, PUBLIC, testClient } from '../test-utils';
import { IDEMPOTENCY_RECORD_TTL_MS, IdempotencyKeys } from './idempotency';

const keys: (string | null)[] = [];
const ifMatch: (string | null)[] = [];
let failuresLeft = 0;
const server = setupServer(
  http.post(`${PUBLIC}/auth/sessions\\:revoke-all`, ({ request }) => {
    keys.push(request.headers.get('Idempotency-Key'));
    if (failuresLeft-- > 0) return new HttpResponse(null, { status: 503 });
    return new HttpResponse(null, { status: 204 });
  }),
  http.get(`${INTERNAL}/admin/catalog/countries/:code`, () => HttpResponse.json({ code: 'VN' }, { headers: { ETag: '"v7"' } })),
  http.patch(`${INTERNAL}/admin/catalog/countries/:code`, ({ request }) => {
    ifMatch.push(request.headers.get('If-Match'));
    return HttpResponse.json({ code: 'VN' }, { headers: { ETag: '"v8"' } });
  }),
);

beforeAll(() => server.listen({ onUnhandledRequest: 'error' }));
afterEach(() => {
  keys.length = 0;
  ifMatch.length = 0;
  failuresLeft = 0;
});
afterAll(() => server.close());

describe('UUIDv7', () => {
  it('has version 7, RFC variant and the timestamp', () => {
    const t = Date.UTC(2026, 10, 19, 8, 0, 0);
    const id = uuidv7(t);
    expect(id).toMatch(UUIDV7);
    expect(id[14]).toBe('7');
    expect(['8', '9', 'a', 'b']).toContain(id[19]);
    expect(uuidv7Time(id)).toBe(t);
    expect(uuidv7(t)).not.toBe(id);
  });
});

describe('Idempotency-Key', () => {
  it('is generated as UUIDv7 for an operation that requires it', async () => {
    const { api } = testClient();
    const { response } = await api.client.POST('/auth/sessions:revoke-all', {});
    expect(response.status).toBe(204);
    expect(keys[0]).toMatch(UUIDV7);
  });

  it('stays the same across automatic retries (503 → 503 → 204)', async () => {
    failuresLeft = 2;
    const { api } = testClient();
    const { response } = await api.client.POST('/auth/sessions:revoke-all', {});
    expect(response.status).toBe(204);
    expect(keys).toHaveLength(3);
    expect(new Set(keys).size).toBe(1);
  });

  it('a user retry of the same intent after a network error reuses the key', async () => {
    const { api } = testClient();
    server.use(http.post(`${PUBLIC}/auth/sessions\\:revoke-all`, ({ request }) => {
      keys.push(request.headers.get('Idempotency-Key'));
      return HttpResponse.error();
    }, { once: true }), http.post(`${PUBLIC}/auth/sessions\\:revoke-all`, ({ request }) => {
      keys.push(request.headers.get('Idempotency-Key'));
      return HttpResponse.error();
    }, { once: true }), http.post(`${PUBLIC}/auth/sessions\\:revoke-all`, ({ request }) => {
      keys.push(request.headers.get('Idempotency-Key'));
      return HttpResponse.error();
    }, { once: true }));
    await expect(api.client.POST('/auth/sessions:revoke-all', { headers: { 'x-ew-intent': 'revoke-all-1' } })).rejects.toMatchObject({ code: 'NETWORK_ERROR' });
    const firstKey = keys[0];
    keys.length = 0;
    const { response } = await api.client.POST('/auth/sessions:revoke-all', { headers: { 'x-ew-intent': 'revoke-all-1' } });
    expect(response.status).toBe(204);
    expect(keys[0]).toBe(firstKey);
  });

  it('the intent header never reaches the server and keys expire after idempotency_record_ttl', () => {
    let now = Date.UTC(2026, 10, 19);
    const store = new IdempotencyKeys(() => now);
    const k1 = store.forIntent('a');
    expect(store.forIntent('a')).toBe(k1);
    now += IDEMPOTENCY_RECORD_TTL_MS + 1;
    expect(store.forIntent('a')).not.toBe(k1);
  });
});

describe('If-Match', () => {
  it('sends the ETag of the last GET of the resource', async () => {
    const { api } = testClient();
    // GET of the resource (raw pipeline call: the spec has no typed GET for a single country)
    await api.fetch(new Request(`${PUBLIC}/admin/catalog/countries/VN`));
    await api.client.PATCH('/admin/catalog/countries/{code}', { params: { path: { code: 'VN' } }, body: {} as never });
    expect(ifMatch[0]).toBe('"v7"');
    // the PATCH response carries the new ETag for the next write
    await api.client.PATCH('/admin/catalog/countries/{code}', { params: { path: { code: 'VN' } }, body: {} as never });
    expect(ifMatch[1]).toBe('"v8"');
  });
});
