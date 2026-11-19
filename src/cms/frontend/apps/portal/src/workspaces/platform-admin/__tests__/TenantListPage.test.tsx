// F057 T038/T043 + T010 — F057-AC-17, F057-AC-21, F057-AC-01 edge cases (TC-TEN-024, TC-TEN-018): tenant list filtered
// by `q`/`tenant_type`, cursor paging ("Tải thêm"), create form limited by the schema with counters, 409 CONFLICT
// shown on duplicate code, disable asks for a reason then updateTenant (merge patch, If-Match = `version` of the list
// row, Idempotency-Key); a stale version gets the server's 412 shown by its code (OQ F057-3). Topbar tenant
// filter pages by cursor while scrolling and marks a disabled tenant "đã tắt". Backend = MOCK.
import { configure, fireEvent, screen, waitFor, within } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { tenantFilterStore } from '../../../shell/tenantFilterStore';
import { stopPortals, TENANTS } from '../../../test/portal-harness';
import { platformState, renderPlatform } from '../../../test/platform-backend';

// whole-portal flows with several Ant Design selects and dialogs are slow on the throttled shared CPU (2 workers max)
vi.setConfig({ testTimeout: 300_000 });
configure({ asyncUtilTimeout: 30_000 });

afterEach(() => {
  stopPortals();
  tenantFilterStore.reset();
});

const UUIDV7 = /^[0-9a-f]{8}-[0-9a-f]{4}-7[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i;

describe('Quản lý tenant (F057-AC-17)', () => {
  it('filters by q and tenant_type (q capped at maxLength 100 with a counter)', async () => {
    const { backend, user } = await renderPlatform('/platform-admin/tenants');
    await screen.findByRole('heading', { level: 1, name: 'Tenant, kết nối SIS' });
    expect(await screen.findByText('th-demo-a')).toBeInTheDocument();
    const search = screen.getByRole('searchbox', { name: 'Tìm theo mã hoặc tên' });
    expect(search).toHaveAttribute('maxlength', '100');
    await user.type(search, 'pgd{Enter}');
    await waitFor(() => expect(backend.calls('GET', '/admin/tenants').some((r) => new URLSearchParams(r.search).get('q') === 'pgd' && new URLSearchParams(r.search).get('limit') === '50')).toBe(true));
    expect(await screen.findByText('pgd-demo')).toBeInTheDocument();
    await waitFor(() => expect(screen.queryByText('th-demo-a')).toBeNull());
    await user.click(screen.getByRole('combobox', { name: 'Loại' }));
    await user.click((await screen.findAllByTitle('Đơn vị')).at(-1)!);
    await waitFor(() => expect(backend.calls('GET', '/admin/tenants').some((r) => new URLSearchParams(r.search).get('tenant_type') === 'unit')).toBe(true));
  });

  it('cursor paging: "Tải thêm" loads the next page', async () => {
    const state = platformState();
    state.tenantPageSize = 2;
    const { backend, user } = await renderPlatform('/platform-admin/tenants', { state });
    await screen.findByText('pgd-demo');
    expect(screen.queryByText('thcs-demo-b')).toBeNull();
    await user.click(screen.getByRole('button', { name: 'Tải thêm' }));
    expect(await screen.findByText('thcs-demo-b')).toBeInTheDocument();
    expect(backend.calls('GET', '/admin/tenants').some((r) => new URLSearchParams(r.search).get('cursor') === '2')).toBe(true);
  });

  it('create: schema limits with counters, 409 CONFLICT on a duplicate code, then success with Idempotency-Key', async () => {
    const { backend, user } = await renderPlatform('/platform-admin/tenants');
    await user.click(await screen.findByRole('button', { name: 'Tạo tenant' }));
    const dialog = await screen.findByRole('dialog');
    const code = within(dialog).getByLabelText('Mã');
    expect(code).toHaveAttribute('maxlength', '64');
    expect(within(dialog).getByLabelText('Tên')).toHaveAttribute('maxlength', '255');
    expect(within(dialog).getByLabelText('Lý do')).toHaveAttribute('maxlength', '500');
    fireEvent.change(code, { target: { value: 'th-demo-a' } });
    expect(within(dialog).getByText('9/64')).toBeInTheDocument();
    fireEvent.change(within(dialog).getByLabelText('Tên'), { target: { value: 'Trường trùng mã' } });
    fireEvent.change(within(dialog).getByLabelText('Alias Keycloak Organization'), { target: { value: 'Bad Alias' } });
    await user.click(within(dialog).getByRole('button', { name: 'Lưu' }));
    expect(await within(dialog).findByText('Giá trị không đúng định dạng.')).toBeInTheDocument();
    fireEvent.change(within(dialog).getByLabelText('Alias Keycloak Organization'), { target: { value: 'th-trung-ma' } });
    await user.click(within(dialog).getByRole('button', { name: 'Lưu' }));
    expect(await within(dialog).findByText('Mã tenant hoặc alias đã tồn tại.')).toBeInTheDocument();
    expect(within(dialog).getByText('Mã truy vết: trace-conflict')).toBeInTheDocument();
    fireEvent.change(code, { target: { value: 'th-moi' } });
    await user.click(within(dialog).getByRole('button', { name: 'Lưu' }));
    await waitFor(() => expect(backend.calls('POST', '/admin/tenants')).toHaveLength(2));
    const [dup, ok] = backend.calls('POST', '/admin/tenants');
    expect(ok!.body).toEqual({ code: 'th-moi', name: 'Trường trùng mã', tenant_type: 'school', keycloak_org_alias: 'th-trung-ma', default_locale: 'vi-VN' });
    expect(ok!.headers['idempotency-key']).toMatch(UUIDV7);
    expect(ok!.headers['idempotency-key']).not.toBe(dup!.headers['idempotency-key']);
    expect(await screen.findByText('th-moi')).toBeInTheDocument();
  });

  it('disable asks for a reason; updateTenant sends status + reason with If-Match = version of the row', async () => {
    const { backend, user } = await renderPlatform('/platform-admin/tenants');
    await user.click(await screen.findByRole('button', { name: 'Tạo tenant' }));
    const create = await screen.findByRole('dialog');
    fireEvent.change(within(create).getByLabelText('Mã'), { target: { value: 'th-tam' } });
    fireEvent.change(within(create).getByLabelText('Tên'), { target: { value: 'TH Tạm' } });
    fireEvent.change(within(create).getByLabelText('Alias Keycloak Organization'), { target: { value: 'th-tam' } });
    await user.click(within(create).getByRole('button', { name: 'Lưu' }));
    const row = (await screen.findByText('th-tam')).closest('tr')!;
    await user.click(within(row).getByRole('button', { name: 'Tắt' }));
    const dialog = (await screen.findAllByRole('dialog')).at(-1)!;
    expect(within(dialog).getByText('Người dùng của tenant sẽ không đăng nhập được cho tới khi kích hoạt lại.')).toBeInTheDocument();
    fireEvent.change(within(dialog).getByLabelText('Lý do'), { target: { value: 'Ngừng hợp đồng' } });
    await user.click(within(dialog).getByRole('button', { name: 'Xác nhận' }));
    await waitFor(() => expect(backend.requests.filter((r) => r.method === 'PATCH')).toHaveLength(1));
    const req = backend.requests.find((r) => r.method === 'PATCH')!;
    expect(req.headers['if-match']).toBe('W/"1"');
    expect(req.headers['content-type']).toContain('application/merge-patch+json');
    expect(req.headers['idempotency-key']).toMatch(UUIDV7);
    expect(req.body).toEqual({ status: 'disabled', reason: 'Ngừng hợp đồng' });
    await waitFor(() => expect(within(screen.getByText('th-tam').closest('tr')!).getByText('Đã tắt')).toBeInTheDocument());
  });

  it('edit of a tenant not written in this session: If-Match = row version (OQ F057-3); stale version → 412 by code', async () => {
    const state = platformState();
    const path = `/admin/tenants/${TENANTS.pgdDemo.tenant_id}`;
    state.versions.set(path, 3);
    const { backend, user } = await renderPlatform('/platform-admin/tenants', { state });
    const row = (await screen.findByText('pgd-demo')).closest('tr')!;
    await user.click(within(row).getByRole('button', { name: 'Sửa' }));
    const dialog = await screen.findByRole('dialog');
    fireEvent.change(within(dialog).getByLabelText('Tên'), { target: { value: 'Phòng GD Demo mới' } });
    fireEvent.change(within(dialog).getByLabelText('Lý do'), { target: { value: 'Đổi tên đơn vị' } });
    state.versions.set(path, 4); // someone else saved meanwhile
    await user.click(within(dialog).getByRole('button', { name: 'Lưu' }));
    expect(await within(dialog).findByText('Dữ liệu đã được người khác cập nhật. Tải lại rồi thử lại.')).toBeInTheDocument();
    expect(backend.calls('PATCH', path)[0]!.headers['if-match']).toBe('W/"3"');
    await user.click(within(dialog).getByRole('button', { name: 'Hủy' }));
    // reloaded list carries version 4: the next edit succeeds
    await waitFor(() => expect(backend.calls('GET', '/admin/tenants').length).toBeGreaterThan(1));
    await user.click(within((await screen.findByText('pgd-demo')).closest('tr')!).getByRole('button', { name: 'Sửa' }));
    const again = (await screen.findAllByRole('dialog')).at(-1)!;
    fireEvent.change(within(again).getByLabelText('Lý do'), { target: { value: 'Đổi tên đơn vị' } });
    await user.click(within(again).getByRole('button', { name: 'Lưu' }));
    await waitFor(() => expect(backend.calls('PATCH', path)).toHaveLength(2));
    expect(backend.calls('PATCH', path)[1]!.headers['if-match']).toBe('W/"4"');
    expect(await screen.findByText('Đã cập nhật tenant.')).toBeInTheDocument();
  });
});

describe('Bộ lọc tenant: cursor và tenant đã tắt (F057-AC-01, edge cases)', () => {
  it('loads the next page when the dropdown is scrolled to its end; a disabled tenant is labelled "đã tắt"', async () => {
    const state = platformState();
    state.tenantPageSize = 2;
    const { backend, user } = await renderPlatform('/platform-admin/catalog', { state });
    const filter = (await screen.findAllByRole('combobox', { name: 'Lọc tenant' }))[0]!;
    await user.click(filter);
    await screen.findAllByTitle('TH Demo A (th-demo-a)');
    const holder = document.querySelector('.ant-select-dropdown:not(.ant-select-dropdown-hidden) .rc-virtual-list-holder') as HTMLElement;
    fireEvent.scroll(holder, { target: { scrollTop: 1000 } });
    await waitFor(() => expect(backend.calls('GET', '/admin/tenants').some((r) => new URLSearchParams(r.search).get('cursor') === '2')).toBe(true));
    fireEvent.scroll(holder, { target: { scrollTop: 2000 } });
    expect(await screen.findByTitle('TH Ngừng HĐ (th-ngung) · đã tắt')).toBeInTheDocument();
  });
});
