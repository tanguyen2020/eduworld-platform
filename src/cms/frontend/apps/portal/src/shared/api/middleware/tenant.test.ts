// @vitest-environment node
// F050-AC-11 (TC-TEN-011a): Authorization from TokenProvider, X-Tenant-Id by metadata (required / optional /
// none), Accept-Language from the UI language. Gateway served by MSW (MOCK, test only).
import { http, HttpResponse } from 'msw';
import { setupServer } from 'msw/node';
import { afterAll, afterEach, beforeAll, describe, expect, it } from 'vitest';
import { findOperationById } from '../operations';
import { INTERNAL, PUBLIC, SECRET_TOKEN, TENANT_ID, testClient } from '../test-utils';

const seen: Request[] = [];
const server = setupServer(
  http.all(`${PUBLIC}/*`, ({ request }) => {
    seen.push(request.clone());
    return HttpResponse.json({ ok: true });
  }),
  http.all(`${INTERNAL}/*`, ({ request }) => {
    seen.push(request.clone());
    return HttpResponse.json({ ok: true });
  }),
);

beforeAll(() => server.listen({ onUnhandledRequest: 'error' }));
afterEach(() => {
  seen.length = 0;
  server.resetHandlers();
});
afterAll(() => server.close());

describe('tenant, auth and language headers', () => {
  it('metadata is generated for the reference operations', () => {
    expect(findOperationById('getMyPermissions')?.tenantHeader).toBe('optional');
    expect(findOperationById('listMyTenants')?.tenantHeader).toBe('none');
    expect(findOperationById('listCatalogSchools')?.tenantHeader).toBe('required');
  });

  it('required operation: Authorization, X-Tenant-Id and Accept-Language', async () => {
    const { api } = testClient();
    const { response } = await api.client.GET('/catalog/schools', {});
    expect(response.status).toBe(200);
    const h = seen[0]!.headers;
    expect(h.get('Authorization')).toBe(`Bearer ${SECRET_TOKEN}`);
    expect(h.get('X-Tenant-Id')).toBe(TENANT_ID);
    expect(h.get('Accept-Language')).toBe('vi-VN');
    expect(h.get('traceparent')).toMatch(/^00-[0-9a-f]{32}-[0-9a-f]{16}-01$/);
  });

  it('none operation (listMyTenants) never sends X-Tenant-Id', async () => {
    const { api } = testClient();
    await api.client.GET('/me/tenants', {});
    expect(seen[0]!.headers.has('X-Tenant-Id')).toBe(false);
  });

  it('none operation drops a caller-supplied X-Tenant-Id', async () => {
    const { api } = testClient();
    await api.client.GET('/me/tenants', { headers: { 'X-Tenant-Id': TENANT_ID } });
    expect(seen[0]!.headers.has('X-Tenant-Id')).toBe(false);
  });

  it('optional operation (getMyPermissions) sends the tenant when one is selected', async () => {
    const { api } = testClient();
    await api.client.GET('/me/permissions', {});
    expect(seen[0]!.headers.get('X-Tenant-Id')).toBe(TENANT_ID);
  });

  it('optional operation without working tenant (System Admin cross-tenant) sends no header', async () => {
    const { api } = testClient({ tenantId: null });
    await api.client.GET('/me/permissions', {});
    expect(seen[0]!.headers.has('X-Tenant-Id')).toBe(false);
  });

  it('Accept-Language follows the UI language', async () => {
    const { api } = testClient({ getLocale: () => 'en-US' });
    await api.client.GET('/me/tenants', {});
    expect(seen[0]!.headers.get('Accept-Language')).toBe('en-US');
  });

  it('signed out: no Authorization header', async () => {
    const { FakeTokenProvider } = await import('../test-utils');
    const { api } = testClient({ tokenProvider: new FakeTokenProvider(null) });
    await api.client.GET('/me/tenants', {});
    expect(seen[0]!.headers.has('Authorization')).toBe(false);
  });
});
