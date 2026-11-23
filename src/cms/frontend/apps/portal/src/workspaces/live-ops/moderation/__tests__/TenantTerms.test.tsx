// F122 T029, T030 — extra terms and categories of a target tenant (Safety Team, F122-AC-15, AC-16; ADR-0072). Backend =
// MOCK (test/chat-safety-mock.ts, test only, AGENTS R17); fixtures are placeholders (`zq…`, R15).
// AC-15: list/create/delete terms in existing categories with X-Tenant-Id of the target tenant, test text with `tenant_id`,
//        publish with a change summary carrying the tenant's request code.
// AC-16: toggle (reason ≥ 10) only for `allow_tenant_toggle`; other categories show a locked switch with a note;
//        403 TENANT_TOGGLE_NOT_ALLOWED shown by code.
import { configure, fireEvent, screen, waitFor, within } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { CS, renderSafety, T_A } from '../../../../test/chat-safety-mock';
import { problem, stopPortals } from '../../../../test/portal-harness';

vi.setConfig({ testTimeout: 900_000 }); // whole-portal screens on a shared, throttled CPU (other agents run tests too)
configure({ asyncUtilTimeout: 90_000 });

afterEach(() => stopPortals());

async function pickTenant(user: { click: (e: Element) => Promise<void> }) {
  await user.click(screen.getByRole('combobox', { name: 'Lọc tenant' }));
  await user.click((await screen.findAllByTitle('TH Demo A (th-demo-a)')).at(-1)!);
}

describe('Từ bổ sung của tenant (F122-AC-15)', () => {
  it('target tenant: list, add, delete, test with tenant_id, publish with the request code', async () => {
    const { backend, user } = await renderSafety('/live-ops/tenant-terms', { account: 'sm.antoan' });
    expect(await screen.findByText(/Chọn tenant đích/)).toBeInTheDocument();
    expect(backend.calls('GET', '/admin/chat/tenant-terms')).toHaveLength(0);
    await pickTenant(user);
    expect(await screen.findByText('zqtenantonly')).toBeInTheDocument();
    expect(backend.calls('GET', '/admin/chat/tenant-terms')[0]!.headers['x-tenant-id']).toBe(T_A);
    // add into an existing category
    await user.click(screen.getByRole('button', { name: 'Thêm từ' }));
    const dialog = await screen.findByRole('dialog', { name: 'Thêm từ' });
    await user.click(within(dialog).getByRole('combobox', { name: 'Nhóm từ' }));
    await user.click((await screen.findAllByTitle('Xúc phạm')).at(-1)!);
    fireEvent.change(within(dialog).getByLabelText('Từ / cụm từ'), { target: { value: 'zqmoi' } });
    await user.click(within(dialog).getByRole('button', { name: 'Lưu' }));
    await waitFor(() => expect(backend.calls('POST', '/admin/chat/tenant-terms')).toHaveLength(1));
    const create = backend.calls('POST', '/admin/chat/tenant-terms')[0]!;
    expect(create.headers['x-tenant-id']).toBe(T_A);
    expect(create.headers['idempotency-key']).toBeTruthy();
    expect(create.body).toMatchObject({ category_id: CS.catInsult, term: 'zqmoi', match_type: 'word', locale: 'vi' });
    // delete (soft) after confirmation
    const row = (await screen.findByText('zqtenantonly')).closest('tr')!;
    fireEvent.click(within(row).getByRole('button', { name: 'Xóa' }));
    await user.click(within(await screen.findByRole('dialog', { name: /Xóa từ này/ })).getByRole('button', { name: 'Xóa' }));
    await waitFor(() => expect(backend.calls('DELETE', `/admin/chat/tenant-terms/${CS.tenantTerm1}`)).toHaveLength(1));
    expect(backend.calls('DELETE', `/admin/chat/tenant-terms/${CS.tenantTerm1}`)[0]!.headers['x-tenant-id']).toBe(T_A);
    // test with tenant_id
    fireEvent.change(screen.getByLabelText('Chuỗi cần thử'), { target: { value: 'zqmoi o day' } });
    await user.click(screen.getByRole('button', { name: 'Thử chuỗi' }));
    await waitFor(() => expect(backend.calls('POST', '/admin/chat/sensitive-terms:test')).toHaveLength(1));
    expect(backend.calls('POST', '/admin/chat/sensitive-terms:test')[0]!.body).toEqual({ text: 'zqmoi o day', tenant_id: T_A, include_drafts: true });
    // publish: the summary must carry the request code
    await user.click(screen.getByRole('button', { name: 'Publish' }));
    const pub = await screen.findByRole('dialog', { name: 'Publish danh mục' });
    fireEvent.change(within(pub).getByLabelText('Tóm tắt thay đổi'), { target: { value: 'Thêm từ theo đề nghị' } });
    await user.click(within(pub).getByRole('button', { name: 'Publish' }));
    expect(await within(pub).findByText(/mã yêu cầu của tenant/)).toBeInTheDocument();
    expect(backend.calls('POST', '/admin/chat/tenant-terms:publish')).toHaveLength(0);
    fireEvent.change(within(pub).getByLabelText('Tóm tắt thay đổi'), { target: { value: 'Thêm từ theo YC-2026-012 của Phòng GD' } });
    await user.click(within(pub).getByRole('button', { name: 'Publish' }));
    await waitFor(() => expect(backend.calls('POST', '/admin/chat/tenant-terms:publish')).toHaveLength(1));
    const publish = backend.calls('POST', '/admin/chat/tenant-terms:publish')[0]!;
    expect(publish.headers['x-tenant-id']).toBe(T_A);
    expect(publish.body).toEqual({ change_summary: 'Thêm từ theo YC-2026-012 của Phòng GD' });
  });
});

describe('Nhóm từ của tenant (F122-AC-16)', () => {
  it('toggle only when allow_tenant_toggle (reason ≥ 10); locked switch with a note otherwise; TENANT_TOGGLE_NOT_ALLOWED by code', async () => {
    const { backend, user } = await renderSafety('/live-ops/tenant-terms', { account: 'sm.antoan' });
    await screen.findByText(/Chọn tenant đích/);
    await pickTenant(user);
    await user.click(await screen.findByRole('tab', { name: 'Nhóm từ' }));
    const insult = (await screen.findByText('Xúc phạm')).closest('tr')!;
    expect(within(insult).getByRole('switch')).toBeDisabled();
    expect(within(insult).getByText(/không cho tenant bật\/tắt/)).toBeInTheDocument();
    const politics = screen.getByText('Chính trị, tôn giáo').closest('tr')!;
    await user.click(within(politics).getByRole('switch', { name: 'Bật/tắt Chính trị, tôn giáo' }));
    let dialog = await screen.findByRole('dialog', { name: 'Tắt nhóm từ cho tenant' });
    const reason = within(dialog).getByLabelText('Lý do');
    expect(reason).toHaveAttribute('maxlength', '500');
    fireEvent.change(reason, { target: { value: 'ngắn' } });
    await user.click(within(dialog).getByRole('button', { name: /OK|Xác nhận|Đồng ý/ }));
    expect(backend.calls('POST', '/admin/chat/tenant-categories/politics_religion:toggle')).toHaveLength(0);
    fireEvent.change(reason, { target: { value: 'Trường đề nghị theo công văn 12/2026' } });
    await user.click(within(dialog).getByRole('button', { name: /OK|Xác nhận|Đồng ý/ }));
    await waitFor(() => expect(backend.calls('POST', '/admin/chat/tenant-categories/politics_religion:toggle')).toHaveLength(1));
    const call = backend.calls('POST', '/admin/chat/tenant-categories/politics_religion:toggle')[0]!;
    expect(call.headers['x-tenant-id']).toBe(T_A);
    expect(call.headers['idempotency-key']).toBeTruthy();
    expect(call.body).toEqual({ enabled: false, reason: 'Trường đề nghị theo công văn 12/2026' });
    // server refuses → message by code
    backend.overrides.unshift((r) => (r.path.endsWith(':toggle') ? problem(403, 'TENANT_TOGGLE_NOT_ALLOWED') : undefined));
    const again = (await screen.findByText('Chính trị, tôn giáo')).closest('tr')!;
    await user.click(within(again).getByRole('switch', { name: 'Bật/tắt Chính trị, tôn giáo' }));
    dialog = await screen.findByRole('dialog');
    fireEvent.change(within(dialog).getByLabelText('Lý do'), { target: { value: 'Bật lại theo đề nghị của trường' } });
    await user.click(within(dialog).getByRole('button', { name: /OK|Xác nhận|Đồng ý/ }));
    expect(await screen.findByText('Platform không cho tenant bật/tắt nhóm từ này.')).toBeInTheDocument();
  });
});
