// @vitest-environment node
// F051 T043 — F051-AC-23..26 (TC-CMS-TEN-01/02/03, TC-TEN-004): System Admin list → `tenant_ids[]` without
// X-Tenant-Id; tenant user → never `tenant_ids` (helper and apiClient both strip it) and always the session header;
// header-only screens need exactly one tenant; System Admin writes carry the chosen target tenant. MOCK gateway.
import { describe, expect, it, vi } from 'vitest';
import { createApiClient } from './client';
import { TEST_CONFIG } from './test-utils';
import { PLATFORM_TARGET, requireSingleTenant, targetTenantHeaders, withTenantScope } from './withTenantScope';

const A = '0193a4d2-0000-7000-8000-00000000000a';
const B = '0193a4d2-0000-7000-8000-00000000000b';

function client(sessionTenant: string | null) {
  const seen: Request[] = [];
  // MOCK gateway (test only)
  const fetch = vi.fn(async (req: Request) => {
    seen.push(req);
    return new Response(JSON.stringify({ data: [], page: {} }), { status: 200, headers: { 'Content-Type': 'application/json' } });
  });
  const api = createApiClient({ config: TEST_CONFIG, tokenProvider: { getAccessToken: async () => 'tok', refresh: async () => false }, tenant: { getTenantId: () => sessionTenant }, getLocale: () => 'vi-VN', hasTranslation: () => true, fetch: fetch as never });
  return { api, seen };
}

describe('withTenantScope', () => {
  it('System Admin: tenant_ids from the filter; empty filter = all tenants (no param)', () => {
    expect(withTenantScope({ q: 'x' }, { crossTenant: true, filter: [A, B] })).toEqual({ q: 'x', tenant_ids: [A, B] });
    expect(withTenantScope({ q: 'x', tenant_ids: [A] }, { crossTenant: true, filter: [] })).toEqual({ q: 'x' });
  });

  it('tenant user: tenant_ids never kept (F051-AC-26)', () => {
    expect(withTenantScope({ q: 'x', tenant_ids: [A] }, { crossTenant: false, filter: [A] })).toEqual({ q: 'x' });
  });

  it('header-only screens: System Admin must choose exactly one tenant (F051-AC-24)', () => {
    expect(requireSingleTenant({ crossTenant: true, filter: [] })).toEqual({ ready: false });
    expect(requireSingleTenant({ crossTenant: true, filter: [A, B] })).toEqual({ ready: false });
    expect(requireSingleTenant({ crossTenant: true, filter: [A] })).toEqual({ ready: true, headers: { 'X-Tenant-Id': A } });
    expect(requireSingleTenant({ crossTenant: false, filter: [] })).toEqual({ ready: true, headers: {} });
  });

  it('writes: one target tenant or Platform, never "all" (F051-AC-25)', () => {
    expect(targetTenantHeaders(A)).toEqual({ 'X-Tenant-Id': A });
    expect(targetTenantHeaders(PLATFORM_TARGET)).toEqual({});
    expect(() => targetTenantHeaders('')).toThrow();
  });
});

describe('apiClient tenant scope', () => {
  it('System Admin cross-tenant list: no X-Tenant-Id, tenant_ids repeated (F051-AC-23)', async () => {
    const { api, seen } = client(null);
    await api.client.GET('/admin/tenants', { params: { query: { q: 'demo', tenant_ids: [A, B] } as never } });
    expect(seen[0]!.headers.get('X-Tenant-Id')).toBeNull();
    expect(new URL(seen[0]!.url).searchParams.getAll('tenant_ids')).toEqual([A, B]);
  });

  it('System Admin write: the explicit target tenant header is kept in platform scope (F051-AC-25)', async () => {
    const { api, seen } = client(null);
    await api.client.GET('/me/permissions', { headers: targetTenantHeaders(B) });
    expect(seen[0]!.headers.get('X-Tenant-Id')).toBe(B);
  });

  it('tenant user: session tenant wins over any header and tenant_ids is stripped (F051-AC-06, F051-AC-26)', async () => {
    const { api, seen } = client(A);
    await api.client.GET('/me/permissions', { headers: { 'X-Tenant-Id': B }, params: { query: { tenant_ids: [B], keep: 'y' } as never } });
    expect(seen[0]!.headers.get('X-Tenant-Id')).toBe(A);
    const params = new URL(seen[0]!.url).searchParams;
    expect(params.getAll('tenant_ids')).toEqual([]);
    expect(params.get('keep')).toBe('y');
  });
});
