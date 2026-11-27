// F134 T023 — F134-AC-11 (TC-CMS-WS-01): "Tenant QA" switch of the tenant form (workspace D, System Admin only —
// platform.tenant.manage) with a confirmation that states the consequences (excluded from Game Analytics and the platform
// learning summary, no backfill when unflagged); `is_qa` goes in the updateTenant merge patch (If-Match, Idempotency-Key,
// reason) only when it changed; disabled for the `common` tenant; the "QA" tag shows next to the tenant name in the list.
// Nothing is computed in the client. Backend = MOCK (test/platform-backend.ts, test only, AGENTS R17).
import { configure, fireEvent, screen, waitFor, within } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { tenantFilterStore } from '../../../shell/tenantFilterStore';
import { ACCOUNTS, MockPortalBackend, renderPortal, stopPortals, TENANTS, type MockAccount } from '../../../test/portal-harness';
import { platformState, renderPlatform, withPlatformMock } from '../../../test/platform-backend';

// whole-portal flows with dialogs are slow on the throttled shared CPU
vi.setConfig({ testTimeout: 300_000 });
configure({ asyncUtilTimeout: 30_000 });

afterEach(() => {
  stopPortals();
  tenantFilterStore.reset();
});

const UUIDV7 = /^[0-9a-f]{8}-[0-9a-f]{4}-7[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i;
const COMMON = { tenant_id: '0193a4d2-0000-7000-8000-0000000000c0', code: 'chung', name: 'Tenant chung', tenant_type: 'common', status: 'active', school_count: 0 };

async function openEdit(user: { click: (e: Element) => Promise<void> }, code: string) {
  const row = (await screen.findByText(code)).closest('tr')!;
  await user.click(within(row).getByRole('button', { name: 'Sửa' }));
  return screen.findByRole('dialog');
}

describe('Công tắc Tenant QA (F134-AC-11)', () => {
  it('System Admin: toggle → confirmation with the consequences → Xác nhận → save sends is_qa with If-Match + Idempotency-Key + reason', async () => {
    const { backend, user } = await renderPlatform('/platform-admin/tenants');
    const dialog = await openEdit(user, 'pgd-demo');
    const qa = within(dialog).getByRole('switch', { name: 'Tenant QA' });
    expect(qa).not.toBeChecked();
    expect(within(dialog).getByTestId('tenant-qa-help')).toHaveTextContent('không tính vào Game Analytics');
    await user.click(qa);
    const confirm = await screen.findByTestId('tenant-qa-confirm');
    expect(confirm).toHaveTextContent('loại khỏi Game Analytics và tổng hợp học tập platform');
    expect(confirm).toHaveTextContent('không backfill');
    expect(within(confirm.closest('[role="dialog"]') as HTMLElement).getAllByText('Đánh dấu Phòng GD Demo là tenant QA?').length).toBeGreaterThan(0);
    await user.click(within(confirm.closest('[role="dialog"]') as HTMLElement).getByRole('button', { name: 'Xác nhận' }));
    await waitFor(() => expect(qa).toBeChecked());
    fireEvent.change(within(dialog).getByLabelText('Lý do'), { target: { value: 'Tenant cho probe synthetic' } });
    await user.click(within(dialog).getByRole('button', { name: 'Lưu' }));
    const path = `/admin/tenants/${TENANTS.pgdDemo.tenant_id}`;
    await waitFor(() => expect(backend.calls('PATCH', path)).toHaveLength(1));
    const req = backend.calls('PATCH', path)[0]!;
    expect(req.body).toEqual({ name: 'Phòng GD Demo', reason: 'Tenant cho probe synthetic', is_qa: true });
    expect(req.headers['if-match']).toBe('W/"1"');
    expect(req.headers['idempotency-key']).toMatch(UUIDV7);
    expect(req.headers['content-type']).toContain('application/merge-patch+json');
    // the reloaded list carries is_qa = true from the server → "QA" tag next to the name
    const row = (await screen.findByText('pgd-demo')).closest('tr')!;
    expect(await within(row).findByTestId('tenant-qa-tag')).toHaveTextContent('QA');
  });

  it('cancelled confirmation keeps the switch; unflagging explains that the flagged period is not backfilled; unchanged flag is not sent', async () => {
    const state = platformState();
    state.tenants.find((x) => x.tenant_id === TENANTS.thcsDemoB.tenant_id)!.is_qa = true;
    const { backend, user } = await renderPlatform('/platform-admin/tenants', { state });
    const dialog = await openEdit(user, 'thcs-demo-b');
    const qa = within(dialog).getByRole('switch', { name: 'Tenant QA' });
    expect(qa).toBeChecked();
    await user.click(qa);
    const confirm = await screen.findByTestId('tenant-qa-confirm');
    expect(confirm).toHaveTextContent('Dữ liệu trong thời gian đã đánh dấu không có và không được bổ sung lại (không backfill).');
    await user.click(within(confirm.closest('[role="dialog"]') as HTMLElement).getByRole('button', { name: 'Hủy' }));
    await waitFor(() => expect(screen.queryByTestId('tenant-qa-confirm')).toBeNull());
    expect(qa).toBeChecked();
    fireEvent.change(within(dialog).getByLabelText('Lý do'), { target: { value: 'Chỉ đổi tên' } });
    await user.click(within(dialog).getByRole('button', { name: 'Lưu' }));
    const path = `/admin/tenants/${TENANTS.thcsDemoB.tenant_id}`;
    await waitFor(() => expect(backend.calls('PATCH', path)).toHaveLength(1));
    expect(backend.calls('PATCH', path)[0]!.body).toEqual({ name: 'THCS Demo B', reason: 'Chỉ đổi tên' });
  });

  it('common tenant: switch disabled with the reason; is_qa never sent', async () => {
    const state = platformState();
    state.tenants.push({ ...COMMON });
    const { backend, user } = await renderPlatform('/platform-admin/tenants', { state });
    const dialog = await openEdit(user, 'chung');
    expect(within(dialog).getByRole('switch', { name: 'Tenant QA' })).toBeDisabled();
    expect(within(dialog).getByTestId('tenant-qa-help')).toHaveTextContent('Tenant chung không thể đánh dấu QA.');
    fireEvent.change(within(dialog).getByLabelText('Lý do'), { target: { value: 'Đổi tên tenant chung' } });
    await user.click(within(dialog).getByRole('button', { name: 'Lưu' }));
    const path = `/admin/tenants/${COMMON.tenant_id}`;
    await waitFor(() => expect(backend.calls('PATCH', path)).toHaveLength(1));
    expect(backend.calls('PATCH', path)[0]!.body).not.toHaveProperty('is_qa');
  });

  it('platform staff without platform.tenant.manage: list with the QA tag, no edit button, no switch', async () => {
    const sa = ACCOUNTS['sa.admin']!;
    const reader: MockAccount = {
      ...sa,
      username: 'sa.xem',
      sub: '0193a4d2-1111-7000-8000-0000000000e9',
      perms: { platform: { workspaces: ['platform_admin'], permissions: ['platform.cross_tenant.read'] } },
    };
    const backend = new MockPortalBackend().withSession('sa.admin');
    backend.user = reader;
    const state = platformState();
    state.tenants.find((x) => x.tenant_id === TENANTS.thcsDemoB.tenant_id)!.is_qa = true;
    withPlatformMock(backend, state);
    await renderPortal({ url: '/platform-admin/tenants', backend });
    const row = (await screen.findByText('thcs-demo-b')).closest('tr')!;
    expect(within(row).getByTestId('tenant-qa-tag')).toHaveTextContent('QA');
    expect(within((await screen.findByText('th-demo-a')).closest('tr')!).queryByTestId('tenant-qa-tag')).toBeNull();
    expect(screen.queryByRole('button', { name: 'Sửa' })).toBeNull();
    expect(screen.queryByRole('switch', { name: 'Tenant QA' })).toBeNull();
  });
});
