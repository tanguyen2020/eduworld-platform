// F057 T011/T012/T013 — F057-AC-02, F057-AC-03, F057-AC-04 (TC-CMS-TEN-01/02/03, TC-TEN-011): the request convention
// of useCrossTenantScope. System Admin: `tenant_ids[]` = filter (none for "Tất cả tenant"), header-only lists need one
// tenant, writes carry the one target tenant (none for Platform, never "all"). Tenant user: never `tenant_ids`, even
// when a stale filter sits in sessionStorage. Permissions = MOCK (vi.mock of usePermissions).
import { renderHook } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { tenantFilterStore } from '../../shell/tenantFilterStore';
import { PLATFORM_TARGET } from '../api/withTenantScope';
import { useCrossTenantScope } from './useCrossTenantScope';

const perms = vi.hoisted(() => ({ crossTenant: true }));
// MOCK (test only): permission view of the session
vi.mock('../../permissions/usePermissions', () => ({ usePermissions: () => ({ crossTenant: perms.crossTenant }) }));

const A = '0193a4d2-0000-7000-8000-00000000000a';
const B = '0193a4d2-0000-7000-8000-00000000000b';

afterEach(() => {
  tenantFilterStore.reset();
  window.sessionStorage.clear();
});

describe('System Admin (platform.cross_tenant.read)', () => {
  it('"Tất cả tenant": no tenant_ids; selection → tenant_ids[] (F057-AC-02)', () => {
    perms.crossTenant = true;
    const { result, rerender } = renderHook(() => useCrossTenantScope());
    expect(result.current.listQuery({ resource_type: 'question' })).toEqual({ resource_type: 'question' });
    tenantFilterStore.set([A, B]);
    rerender();
    expect(result.current.listQuery({ resource_type: 'question' })).toEqual({ resource_type: 'question', tenant_ids: [A, B] });
  });

  it('header-only list needs exactly one tenant; write = X-Tenant-Id of the target, none for Platform (F057-AC-03)', () => {
    perms.crossTenant = true;
    tenantFilterStore.set([A, B]);
    const { result, rerender } = renderHook(() => useCrossTenantScope());
    expect(result.current.single).toEqual({ ready: false });
    tenantFilterStore.set([B]);
    rerender();
    expect(result.current.single).toEqual({ ready: true, headers: { 'X-Tenant-Id': B } });
    expect(result.current.writeHeaders(A)).toEqual({ 'X-Tenant-Id': A });
    expect(result.current.writeHeaders(PLATFORM_TARGET)).toEqual({});
    expect(() => result.current.writeHeaders('')).toThrow();
  });
});

describe('tenant user (F057-AC-04)', () => {
  it('never tenant_ids, filter ignored, header-only lists use the session tenant', () => {
    perms.crossTenant = false;
    tenantFilterStore.set([A]);
    const { result } = renderHook(() => useCrossTenantScope());
    expect(result.current.crossTenant).toBe(false);
    expect(result.current.filter).toEqual([]);
    expect(result.current.listQuery({ q: 'x', tenant_ids: [B] })).toEqual({ q: 'x' });
    expect(result.current.single).toEqual({ ready: true, headers: {} });
  });
});
