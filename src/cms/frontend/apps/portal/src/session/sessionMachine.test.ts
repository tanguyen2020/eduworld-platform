// F051 T011 — F051-AC-03 (TC-AUTH-072): post-login routing is a pure function of MyTenantsResponse; state machine of
// data-model §2 (expired, staff only, tenant change, sign-out).
import { describe, expect, it } from 'vitest';
import { initialSession, routeAfterLogin, scopeKey, sessionReducer, type MyTenant, type MyTenantsResponse } from './sessionMachine';

const t = (id: string, extra: Partial<MyTenant> = {}): MyTenant => ({ tenant_id: id, code: `c-${id}`, name: `Tenant ${id}`, tenant_type: 'school', status: 'active', ...extra });
const A = '00000000-0000-7000-8000-00000000000a';
const B = '00000000-0000-7000-8000-00000000000b';
const res = (items: MyTenant[], has_platform_role = false): MyTenantsResponse => ({ items, has_platform_role });

describe('routeAfterLogin (F051-AC-03)', () => {
  it('platform role → platform scope, even with tenants', () => {
    expect(routeAfterLogin(res([t(A)], true), null)).toEqual({ kind: 'platform' });
  });

  it('exactly one tenant → straight into it', () => {
    expect(routeAfterLogin(res([t(A)]), null)).toEqual({ kind: 'tenant', tenant: t(A) });
  });

  it('several tenants → picker, pre-selecting the remembered tenant only when still listed', () => {
    expect(routeAfterLogin(res([t(A), t(B)]), B)).toEqual({ kind: 'select', preselectedTenantId: B });
    expect(routeAfterLogin(res([t(A), t(B)]), 'gone')).toEqual({ kind: 'select', preselectedTenantId: null });
  });

  it('several tenants and a session tenant of this tab (F5) → back into it (F051-AC-28)', () => {
    expect(routeAfterLogin(res([t(A), t(B)]), null, B)).toEqual({ kind: 'tenant', tenant: t(B) });
  });

  it('empty and no platform role → no access; disabled tenants do not count', () => {
    expect(routeAfterLogin(res([]), null)).toEqual({ kind: 'no_access' });
    expect(routeAfterLogin(res([t(A, { status: 'disabled' })]), null)).toEqual({ kind: 'no_access' });
  });
});

describe('sessionReducer', () => {
  const loaded = (items: MyTenant[], decision = routeAfterLogin(res(items), null)) =>
    sessionReducer(sessionReducer(initialSession, { type: 'authenticated' }), { type: 'tenants_loaded', response: res(items), decision });

  it('restore → unauthenticated or resolving_scope', () => {
    expect(sessionReducer(initialSession, { type: 'restore_failed' }).status).toBe('unauthenticated');
    expect(sessionReducer(initialSession, { type: 'authenticated' }).status).toBe('resolving_scope');
  });

  it('one tenant → ready with tenant scope; several → selecting; select → ready', () => {
    const one = loaded([t(A)]);
    expect(one.status).toBe('ready');
    expect(one.scope).toMatchObject({ kind: 'tenant', tenantId: A });
    expect(scopeKey(one.scope)).toBe(A);
    const many = loaded([t(A), t(B)]);
    expect(many.status).toBe('selecting_tenant');
    const chosen = sessionReducer(many, { type: 'tenant_selected', tenant: t(B) });
    expect(chosen.status).toBe('ready');
    expect(scopeKey(chosen.scope)).toBe(B);
  });

  it('platform scope key; no access reason', () => {
    const p = sessionReducer(sessionReducer(initialSession, { type: 'authenticated' }), { type: 'tenants_loaded', response: res([], true), decision: { kind: 'platform' } });
    expect(scopeKey(p.scope)).toBe('platform');
    expect(loaded([]).noAccessReason).toBe('no_role');
    expect(sessionReducer(p, { type: 'staff_only' })).toMatchObject({ status: 'no_access', noAccessReason: 'student', scope: null });
  });

  it('expired keeps nothing of the session; tenant access change re-resolves; sign-out', () => {
    const ready = loaded([t(A)]);
    expect(sessionReducer(ready, { type: 'expired' })).toEqual({ ...initialSession, status: 'expired' });
    expect(sessionReducer(ready, { type: 'tenant_access_changed' })).toMatchObject({ status: 'resolving_scope', scope: null });
    expect(sessionReducer(ready, { type: 'signed_out' }).status).toBe('unauthenticated');
    expect(sessionReducer(sessionReducer(ready, { type: 'signed_out' }), { type: 'expired' }).status).toBe('unauthenticated');
  });
});
