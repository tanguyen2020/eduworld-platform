// F057 T017/T018/T042 — F057-AC-05, F057-AC-06, F057-AC-20 (TC-TEN-019, TC-CMS-ASG-01): tab "Tenant được gán" of a
// platform record (listTenantAssignments), dialog "Gán cho tenant" (several tenants, Idempotency-Key, no X-Tenant-Id),
// retry after an error keeps the same Idempotency-Key and shows code + trace_id; "Gỡ gán" confirmation says learning
// history is kept, then unassignFromTenants. Backend = MOCK (src/test/platform-backend.ts).
import { configure, fireEvent, screen, waitFor, within } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { problem, stopPortals, TENANTS } from '../../../test/portal-harness';
import { IDS, renderPlatform } from '../../../test/platform-backend';

// whole-portal flows with several Ant Design selects and dialogs are slow on the throttled shared CPU (2 workers max)
vi.setConfig({ testTimeout: 300_000 });
configure({ asyncUtilTimeout: 30_000 });

afterEach(() => stopPortals());

const URL_ = `/platform-admin/tenant-data?resource_type=question_set&resource_id=${IDS.questionSet}`;
const UUIDV7 = /^[0-9a-f]{8}-[0-9a-f]{4}-7[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i;

async function pickTenants(user: Awaited<ReturnType<typeof renderPlatform>>['user'], dialog: HTMLElement, titles: string[]) {
  await user.click(within(dialog).getByRole('combobox', { name: 'Tenant' }));
  for (const title of titles) await user.click((await screen.findAllByTitle(title)).at(-1)!);
}

describe('Gán cho tenant (F057-AC-05)', () => {
  it('assigns a platform record to two tenants with one Idempotency-Key, no X-Tenant-Id; tab lists them', async () => {
    const { backend, user } = await renderPlatform(URL_);
    await screen.findByRole('heading', { level: 1, name: 'Gán và clone dữ liệu tenant' });
    await waitFor(() => expect(backend.calls('GET', '/admin/tenant-assignments')).toHaveLength(1));
    const list = new URLSearchParams(backend.calls('GET', '/admin/tenant-assignments')[0]!.search);
    expect([list.get('resource_type'), list.get('resource_id')]).toEqual(['question_set', IDS.questionSet]);

    await user.click(screen.getByRole('button', { name: 'Gán cho tenant' }));
    const dialog = await screen.findByRole('dialog');
    await user.click(within(dialog).getByRole('button', { name: 'Xác nhận' }));
    expect(await within(dialog).findByText('Chọn ít nhất một tenant hoặc chọn "Mọi tenant hiện tại".')).toBeInTheDocument();
    expect(backend.calls('POST', '/admin/tenant-assignments')).toHaveLength(0);

    await pickTenants(user, dialog, ['TH Demo A (th-demo-a)', 'Phòng GD Demo (pgd-demo)']);
    await user.click(within(dialog).getByRole('button', { name: 'Xác nhận' }));
    await waitFor(() => expect(backend.calls('POST', '/admin/tenant-assignments')).toHaveLength(1));
    const req = backend.calls('POST', '/admin/tenant-assignments')[0]!;
    expect(req.gateway).toBe('internal');
    expect(req.headers['idempotency-key']).toMatch(UUIDV7);
    expect(req.headers['x-tenant-id']).toBeUndefined();
    expect(req.body).toEqual({ resource_type: 'question_set', resource_id: IDS.questionSet, tenant_ids: [TENANTS.thDemoA.tenant_id, TENANTS.pgdDemo.tenant_id], apply_to_all_tenants: false, apply_to_new_tenants: false });
    const tab = screen.getByRole('region', { name: 'Tenant được gán' });
    expect(await within(tab).findByText('TH Demo A')).toBeInTheDocument();
    expect(within(tab).getByText('Phòng GD Demo')).toBeInTheDocument();
  });

  it('[Proposed] "Mọi tenant hiện tại" sends apply_to_all_tenants without tenant_ids', async () => {
    const { backend, user } = await renderPlatform(URL_);
    await user.click(await screen.findByRole('button', { name: 'Gán cho tenant' }));
    const dialog = await screen.findByRole('dialog');
    await user.click(within(dialog).getByRole('checkbox', { name: 'Mọi tenant hiện tại' }));
    await user.click(within(dialog).getByRole('button', { name: 'Xác nhận' }));
    await waitFor(() => expect(backend.calls('POST', '/admin/tenant-assignments')).toHaveLength(1));
    expect(backend.calls('POST', '/admin/tenant-assignments')[0]!.body).toEqual({ resource_type: 'question_set', resource_id: IDS.questionSet, apply_to_all_tenants: true, apply_to_new_tenants: false });
  });

  it('retry after an error keeps the same Idempotency-Key; error shows its code message and trace_id (F057-AC-20)', async () => {
    const { backend, user } = await renderPlatform(URL_);
    let failed = false;
    backend.overrides.unshift((r) => (r.method === 'POST' && r.path === '/admin/tenant-assignments' && !failed ? ((failed = true), problem(500, 'INTERNAL')) : undefined));
    await user.click(await screen.findByRole('button', { name: 'Gán cho tenant' }));
    const dialog = await screen.findByRole('dialog');
    await pickTenants(user, dialog, ['TH Demo A (th-demo-a)']);
    await user.click(within(dialog).getByRole('button', { name: 'Xác nhận' }));
    expect(await within(dialog).findByText('Mã truy vết: trace-internal')).toBeInTheDocument();
    await user.click(within(dialog).getByRole('button', { name: 'Xác nhận' }));
    await waitFor(() => expect(backend.calls('POST', '/admin/tenant-assignments')).toHaveLength(2));
    const [first, second] = backend.calls('POST', '/admin/tenant-assignments');
    expect(first!.headers['idempotency-key']).toMatch(UUIDV7);
    expect(second!.headers['idempotency-key']).toBe(first!.headers['idempotency-key']);
  });
});

describe('Gỡ gán (F057-AC-06)', () => {
  it('confirmation states learning history is kept, then unassignFromTenants with reason + Idempotency-Key', async () => {
    const state = (await import('../../../test/platform-backend')).platformState();
    state.assignments.set(`question_set:${IDS.questionSet}`, [
      { tenant_id: TENANTS.thDemoA.tenant_id, tenant_name: 'TH Demo A', assigned_at: '2026-11-19T08:00:00Z' },
      { tenant_id: TENANTS.pgdDemo.tenant_id, tenant_name: 'Phòng GD Demo', assigned_at: '2026-11-19T08:00:00Z' },
    ]);
    const { backend, user } = await renderPlatform(URL_, { state });
    const row = (await screen.findByText('Phòng GD Demo')).closest('tr')!;
    await user.click(within(row).getByRole('button', { name: 'Gỡ gán' }));
    const dialog = await screen.findByRole('dialog');
    expect(within(dialog).getByText('Tenant sẽ không còn thấy dữ liệu này. Lịch sử học tập và bài đã giao giữ nguyên.')).toBeInTheDocument();
    fireEvent.change(within(dialog).getByLabelText('Lý do'), { target: { value: 'Hết hợp đồng nội dung' } });
    await user.click(within(dialog).getByRole('button', { name: 'Xác nhận' }));
    await waitFor(() => expect(backend.calls('DELETE', '/admin/tenant-assignments')).toHaveLength(1));
    const req = backend.calls('DELETE', '/admin/tenant-assignments')[0]!;
    expect(req.body).toEqual({ resource_type: 'question_set', resource_id: IDS.questionSet, tenant_ids: [TENANTS.pgdDemo.tenant_id], reason: 'Hết hợp đồng nội dung' });
    expect(req.headers['idempotency-key']).toMatch(UUIDV7);
    await waitFor(() => expect(screen.queryByText('Phòng GD Demo')).toBeNull());
    expect(screen.getByText('TH Demo A')).toBeInTheDocument();
  });
});
