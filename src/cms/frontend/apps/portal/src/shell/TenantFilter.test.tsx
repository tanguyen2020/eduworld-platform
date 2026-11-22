// F051 T042 (+T044 at unit level) — F051-AC-22, F051-AC-26, F051-AC-27 (TC-TEN-018, TC-CMS-TEN-01, TC-TEN-024):
// System Admin sees the tenant filter (listTenants on the internal gateway, default "Tất cả tenant", multi-select,
// kept in sessionStorage, every page loaded and searched client-side — owner rule 2026-11-22); tenant users and platform staff without
// `platform.cross_tenant.read` never see it and never send `tenant_ids`. Backend = MOCK harness.
import { screen, waitFor } from '@testing-library/react';
import { afterEach, describe, expect, it } from 'vitest';
import { clampTenantSearch, TENANT_SEARCH_MAX_LENGTH } from '../shared/api/hooks/useTenantOptions';
import { MockPortalBackend, renderPortal, stopPortals, TENANTS } from '../test/portal-harness';
import { tenantFilterStore } from './tenantFilterStore';

afterEach(() => {
  stopPortals();
  tenantFilterStore.reset();
});

describe('System Admin tenant filter', () => {
  it('visible for sa.admin: default all tenants, options from listTenants (internal gateway), kept per browser session', async () => {
    const backend = new MockPortalBackend().withSession('sa.admin');
    const { user } = await renderPortal({ url: '/platform-admin/tenants', backend });
    const filter = await screen.findByRole('combobox', { name: 'Lọc tenant' });
    expect(screen.getByText('Tất cả tenant')).toBeInTheDocument();
    await user.click(filter);
    await user.click(await screen.findByTitle('TH Demo A (th-demo-a)'));
    await user.click(await screen.findByTitle('Phòng GD Demo (pgd-demo)'));
    await waitFor(() => expect(JSON.parse(window.sessionStorage.getItem('ew.portal.tenantFilter') ?? '[]')).toEqual([TENANTS.thDemoA.tenant_id, TENANTS.pgdDemo.tenant_id]));
    const list = backend.calls('GET', '/admin/tenants');
    expect(list.length).toBeGreaterThan(0);
    expect(list.every((r) => r.gateway === 'internal' && r.headers['x-tenant-id'] === undefined)).toBe(true);
  });

  it('options = every listTenants page (limit 100), search filtered client-side without diacritics, cut at 100 (F051-AC-27; ADMIN_PORTAL §1.6)', async () => {
    expect(TENANT_SEARCH_MAX_LENGTH).toBe(100);
    expect(clampTenantSearch('x'.repeat(150))).toHaveLength(100);
    const backend = new MockPortalBackend().withSession('sa.admin');
    const { user } = await renderPortal({ url: '/platform-admin/tenants', backend });
    const filter = await screen.findByRole('combobox', { name: 'Lọc tenant' });
    await user.click(filter);
    await waitFor(() => expect(backend.calls('GET', '/admin/tenants').some((r) => new URLSearchParams(r.search).get('limit') === '100')).toBe(true));
    await user.paste('th demo');
    expect(await screen.findByTitle('TH Demo A (th-demo-a)')).toBeInTheDocument();
    await waitFor(() => expect(screen.queryByTitle('Phòng GD Demo (pgd-demo)')).toBeNull());
    expect(backend.calls('GET', '/admin/tenants').every((r) => !new URLSearchParams(r.search).get('q'))).toBe(true);
  });

  it.each(['ad.truong.a', 'cm.noidung'])('%s has no tenant filter, no listTenants call, no tenant_ids (F051-AC-26)', async (account) => {
    const backend = new MockPortalBackend().withSession(account);
    await renderPortal({ url: '/', backend });
    await screen.findByRole('navigation', { name: 'Menu điều hướng' });
    expect(screen.queryByRole('combobox', { name: 'Lọc tenant' })).toBeNull();
    expect(backend.calls('GET', '/admin/tenants')).toHaveLength(0);
    expect(backend.requests.some((r) => r.search.includes('tenant_ids'))).toBe(false);
  });
});
