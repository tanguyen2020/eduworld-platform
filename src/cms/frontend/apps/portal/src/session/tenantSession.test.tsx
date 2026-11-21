// F051 T014, T037, T038 (+T039 at unit level) — F051-AC-05, F051-AC-06, F051-AC-20, F051-AC-21 (TC-CMS-LOGIN-02/03,
// TC-AUTH-101, TC-TEN-008, TC-TEN-011): choosing a tenant keeps the same access token (no new login/refresh) and sends
// X-Tenant-Id; remembered choice; topbar switch confirms unsaved drafts, clears the old tenant cache, reloads
// permissions and goes to the workspace overview, with no request carrying the old header afterwards;
// 403 TENANT_MISMATCH / TENANT_ACCESS_DISABLED → message + back to the picker. Backend = MOCK harness.
import { screen, waitFor, within } from '@testing-library/react';
import { afterEach, describe, expect, it } from 'vitest';
import { problem, renderPortal, signIn, stopPortals, TENANTS } from '../test/portal-harness';
import { clearUnsavedChangesForTest, registerUnsavedChanges } from './unsavedChangesGuard';

afterEach(() => {
  stopPortals();
  clearUnsavedChangesForTest();
});

async function signedInQlcm(url = '/login') {
  const r = await renderPortal({ url });
  await signIn(r.user, 'qlcm.truong.a');
  await screen.findByRole('heading', { name: 'Chọn đơn vị làm việc' });
  return r;
}

describe('tenant selection after sign-in (F051-AC-05, F051-AC-06)', () => {
  it('same access token, X-Tenant-Id on tenant-scoped calls, no new token, choice remembered', async () => {
    const { backend, user, router } = await signedInQlcm();
    await user.click(screen.getByRole('radio', { name: /Phòng GD Demo/ }));
    await user.click(screen.getByRole('checkbox', { name: 'Nhớ lựa chọn cho lần sau' }));
    await user.click(screen.getByRole('button', { name: 'Vào' }));
    await waitFor(() => expect(router.state.location.pathname).toBe('/school-lms/overview'));
    expect(backend.calls('POST', '/auth/portal/login')).toHaveLength(1);
    expect(backend.calls('POST', '/auth/portal/token:refresh')).toHaveLength(1); // the start-up restore only
    const perms = backend.calls('GET', '/me/permissions');
    expect(perms.at(-1)!.headers['x-tenant-id']).toBe(TENANTS.pgdDemo.tenant_id);
    const tenantsCall = backend.calls('GET', '/me/tenants')[0]!;
    expect(perms.at(-1)!.headers.authorization).toBe(tenantsCall.headers.authorization);
    expect(window.localStorage.getItem('ew.portal.0193a4d2-1111-7000-8000-000000000002.rememberedTenantId')).toBe(TENANTS.pgdDemo.tenant_id);
    expect(window.sessionStorage.getItem('ew.portal.sessionTenantId')).toBe(TENANTS.pgdDemo.tenant_id);
  });

  it('remembered tenant is pre-selected at the next sign-in', async () => {
    window.localStorage.setItem('ew.portal.0193a4d2-1111-7000-8000-000000000002.rememberedTenantId', TENANTS.pgdDemo.tenant_id);
    await signedInQlcm();
    expect(screen.getByRole('radio', { name: /Phòng GD Demo/ })).toBeChecked();
    expect(screen.getByRole('checkbox', { name: 'Nhớ lựa chọn cho lần sau' })).toBeChecked();
  });

  it('search box filters by name/code and is capped (F051-AC-27)', async () => {
    const { user } = await signedInQlcm();
    const box = screen.getByLabelText('Tìm theo tên hoặc mã');
    expect(box).toHaveAttribute('maxlength', '100');
    await user.type(box, 'pgd');
    expect(screen.queryByRole('radio', { name: /TH Demo A/ })).toBeNull();
    expect(screen.getByRole('radio', { name: /Phòng GD Demo/ })).toBeInTheDocument();
  });
});

describe('switching tenant on the topbar (F051-AC-20)', () => {
  it('confirms unsaved drafts, keeps the token, clears old data, no request with the old header afterwards', async () => {
    const { backend, user, router, runtime } = await signedInQlcm();
    await user.click(screen.getByRole('radio', { name: /TH Demo A/ }));
    await user.click(screen.getByRole('button', { name: 'Vào' }));
    await screen.findByRole('heading', { level: 1, name: 'Tổng quan' });
    await router.navigate('/school-lms/classes');
    await screen.findByRole('heading', { level: 1, name: 'Học sinh theo khối' });
    const oldKeys = runtime.queryClient.getQueryCache().getAll().map((q) => JSON.stringify(q.queryKey));
    expect(oldKeys.some((k) => k.includes(TENANTS.thDemoA.tenant_id))).toBe(true);
    const unregister = registerUnsavedChanges('editor', () => true);

    const switcher = screen.getByRole('combobox', { name: 'Đơn vị làm việc' });
    await user.click(switcher);
    await user.click(await screen.findByTitle('Phòng GD Demo (pgd-demo)'));
    const dialog = await screen.findByRole('dialog');
    expect(within(dialog).getByText('Đổi đơn vị sẽ bỏ các thay đổi chưa lưu. Tiếp tục?')).toBeInTheDocument();
    const before = backend.requests.length;
    unregister();
    await user.click(within(dialog).getByRole('button', { name: 'Đổi đơn vị' }));

    // back to the overview of the workspace (F051-AC-20)
    await waitFor(() => expect(router.state.location.pathname).toBe('/school-lms/overview'));
    const after = backend.requests.slice(before);
    expect(after.some((r) => r.path === '/me/permissions' && r.headers['x-tenant-id'] === TENANTS.pgdDemo.tenant_id)).toBe(true);
    expect(after.filter((r) => r.headers['x-tenant-id']).every((r) => r.headers['x-tenant-id'] === TENANTS.pgdDemo.tenant_id)).toBe(true);
    expect(after.some((r) => r.path.startsWith('/auth/portal/'))).toBe(false);
    const keys = runtime.queryClient.getQueryCache().getAll().map((q) => JSON.stringify(q.queryKey));
    expect(keys.some((k) => k.includes(TENANTS.thDemoA.tenant_id))).toBe(false);
  }, 180_000);
});

describe('tenant access changed (F051-AC-21)', () => {
  it.each(['TENANT_MISMATCH', 'TENANT_ACCESS_DISABLED'])('403 %s → message, cache cleared, back to the tenant picker', async (code) => {
    const { backend, user, router, runtime } = await signedInQlcm();
    await user.click(screen.getByRole('radio', { name: /TH Demo A/ }));
    await user.click(screen.getByRole('button', { name: 'Vào' }));
    await screen.findByRole('heading', { level: 1, name: 'Tổng quan' });
    backend.overrides.push((r) => (r.path === '/me/permissions' ? problem(403, code) : undefined));
    await runtime.queryClient.refetchQueries({ queryKey: ['me', 'permissions'] });
    expect(await screen.findByText('Đơn vị làm việc đã thay đổi quyền truy cập.')).toBeInTheDocument();
    await waitFor(() => expect(router.state.location.pathname).toBe('/select-tenant'));
    expect(runtime.tenant.getTenantId()).toBeNull();
    expect(window.sessionStorage.getItem('ew.portal.sessionTenantId')).toBeNull();
  });
});
