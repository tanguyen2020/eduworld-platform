// F057 T026/T027 — F057-AC-09, F057-AC-10, F057-AC-02, F057-AC-03 (TC-CMS-DEL-01, TC-CMS-DEL-02, TC-CMS-TEN-02/03):
// sa.admin opens the trash of every tenant + platform: listTrash by resource_type and tenant_ids[] (no X-Tenant-Id),
// Tenant column (name + type, "Platform"), deleted by/at/reason; restore asks a reason then restoreTrashItem with
// Idempotency-Key and X-Tenant-Id = tenant of the item (none for platform data); restorable=false / 409 → "Đã có bản
// ghi mới cùng mã"; no "Xóa vĩnh viễn" anywhere. Backend = MOCK (src/test/platform-backend.ts).
import { configure, fireEvent, screen, waitFor, within } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { tenantFilterStore } from '../../../shell/tenantFilterStore';
import { stopPortals, TENANTS } from '../../../test/portal-harness';
import { IDS, renderPlatform } from '../../../test/platform-backend';

// whole-portal flows with several Ant Design selects and dialogs are slow on the throttled shared CPU (2 workers max)
vi.setConfig({ testTimeout: 300_000 });
configure({ asyncUtilTimeout: 30_000 });

afterEach(() => {
  stopPortals();
  tenantFilterStore.reset();
});

const UUIDV7 = /^[0-9a-f]{8}-[0-9a-f]{4}-7[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i;

describe('Thùng rác mọi tenant (F057-AC-09)', () => {
  it('lists deleted records of every tenant + platform with Tenant, deleted by/at and reason; no tenant header', async () => {
    const { backend } = await renderPlatform('/platform-admin/trash');
    await screen.findByRole('heading', { level: 1, name: 'Thùng rác' });
    const row = (await screen.findByText('Câu hỏi bảng nhân 7')).closest('tr')!;
    expect(within(row).getByText('TH Demo A')).toBeInTheDocument();
    expect(within(row).getByText('Trường')).toBeInTheDocument();
    expect(within(row).getByText('Xóa nhầm')).toBeInTheDocument();
    expect(within(row).getByText(IDS.deletedBy)).toBeInTheDocument();
    const platformRow = screen.getByText('Câu hỏi platform').closest('tr')!;
    expect(within(platformRow).getByText('Platform')).toBeInTheDocument();
    const calls = backend.calls('GET', '/admin/trash');
    expect(calls[0]!.gateway).toBe('internal');
    expect(new URLSearchParams(calls[0]!.search).get('resource_type')).toBe('question');
    expect(calls.every((r) => r.headers['x-tenant-id'] === undefined && !r.search.includes('tenant_ids'))).toBe(true);
  });

  it('tenant filter → tenant_ids[] on listTrash, still no X-Tenant-Id (F057-AC-02)', async () => {
    tenantFilterStore.set([TENANTS.thDemoA.tenant_id, TENANTS.pgdDemo.tenant_id]);
    const { backend } = await renderPlatform('/platform-admin/trash');
    await screen.findByText('Câu hỏi bảng nhân 7');
    expect(screen.queryByText('Câu hỏi platform')).toBeNull();
    const last = backend.calls('GET', '/admin/trash').at(-1)!;
    expect(new URLSearchParams(last.search).getAll('tenant_ids')).toEqual([TENANTS.thDemoA.tenant_id, TENANTS.pgdDemo.tenant_id]);
    expect(last.headers['x-tenant-id']).toBeUndefined();
  });

  it('restore: reason required (min 3, counter), then restoreTrashItem with Idempotency-Key + X-Tenant-Id of the item', async () => {
    const { backend, user } = await renderPlatform('/platform-admin/trash');
    const row = (await screen.findByText('Câu hỏi bảng nhân 7')).closest('tr')!;
    await user.click(within(row).getByRole('button', { name: 'Khôi phục' }));
    const dialog = await screen.findByRole('dialog');
    await user.click(within(dialog).getByRole('button', { name: 'Xác nhận' }));
    expect(await within(dialog).findByText('Vui lòng nhập lý do.')).toBeInTheDocument();
    expect(backend.calls('POST', `/admin/trash/question/${IDS.trashQuestion}:restore`)).toHaveLength(0);
    fireEvent.change(within(dialog).getByLabelText('Lý do'), { target: { value: 'ab' } });
    await user.click(within(dialog).getByRole('button', { name: 'Xác nhận' }));
    expect(await within(dialog).findByText('Tối thiểu 3 ký tự.')).toBeInTheDocument();
    fireEvent.change(within(dialog).getByLabelText('Lý do'), { target: { value: 'Giáo viên xóa nhầm' } });
    expect(within(dialog).getByText('18 / 1000')).toBeInTheDocument();
    await user.click(within(dialog).getByRole('button', { name: 'Xác nhận' }));
    await waitFor(() => expect(backend.calls('POST', `/admin/trash/question/${IDS.trashQuestion}:restore`)).toHaveLength(1));
    const req = backend.calls('POST', `/admin/trash/question/${IDS.trashQuestion}:restore`)[0]!;
    expect(req.body).toEqual({ reason: 'Giáo viên xóa nhầm' });
    expect(req.headers['idempotency-key']).toMatch(UUIDV7);
    expect(req.headers['x-tenant-id']).toBe(TENANTS.thDemoA.tenant_id);
    await waitFor(() => expect(screen.queryByText('Câu hỏi bảng nhân 7')).toBeNull());
  });

  it('platform record: restore without X-Tenant-Id', async () => {
    const { backend, user } = await renderPlatform('/platform-admin/trash');
    const row = (await screen.findByText('Câu hỏi platform')).closest('tr')!;
    await user.click(within(row).getByRole('button', { name: 'Khôi phục' }));
    const dialog = await screen.findByRole('dialog');
    fireEvent.change(within(dialog).getByLabelText('Lý do'), { target: { value: 'Khôi phục dữ liệu chung' } });
    await user.click(within(dialog).getByRole('button', { name: 'Xác nhận' }));
    await waitFor(() => expect(backend.calls('POST', `/admin/trash/question/${IDS.trashPlatform}:restore`)).toHaveLength(1));
    expect(backend.calls('POST', `/admin/trash/question/${IDS.trashPlatform}:restore`)[0]!.headers['x-tenant-id']).toBeUndefined();
  });
});

describe('Không khôi phục được (F057-AC-10)', () => {
  it('restorable=false → button disabled with "Đã có bản ghi mới cùng mã"; no "Xóa vĩnh viễn" in the DOM', async () => {
    await renderPlatform('/platform-admin/trash');
    const row = (await screen.findByText('Câu hỏi trùng mã')).closest('tr')!;
    expect(within(row).getByRole('button', { name: 'Khôi phục' })).toBeDisabled();
    expect(within(row).getByText('Đã có bản ghi mới cùng mã')).toBeInTheDocument();
    expect(screen.queryByText(/Xóa vĩnh viễn/i)).toBeNull();
    expect(document.body.textContent).not.toMatch(/vĩnh viễn/i);
  });

  it('409 CONFLICT on restore → "Đã có bản ghi mới cùng mã" + code message + trace_id', async () => {
    const { backend, user, state } = await renderPlatform('/platform-admin/trash');
    const row = (await screen.findByText('Câu hỏi bảng nhân 7')).closest('tr')!;
    // the server knows better than the loaded list: the code was reused after the page loaded
    state.trash[0]!.restorable = false;
    await user.click(within(row).getByRole('button', { name: 'Khôi phục' }));
    const dialog = await screen.findByRole('dialog');
    fireEvent.change(within(dialog).getByLabelText('Lý do'), { target: { value: 'Giáo viên xóa nhầm' } });
    await user.click(within(dialog).getByRole('button', { name: 'Xác nhận' }));
    expect(await within(dialog).findByText('Đã có bản ghi mới cùng mã')).toBeInTheDocument();
    expect(within(dialog).getByText('Dữ liệu đã thay đổi hoặc bị trùng.')).toBeInTheDocument();
    expect(within(dialog).getByText('Mã truy vết: trace-conflict')).toBeInTheDocument();
    expect(backend.calls('POST', `/admin/trash/question/${IDS.trashQuestion}:restore`)).toHaveLength(1);
  });
});
