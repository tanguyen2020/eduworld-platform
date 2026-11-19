// F059 T013–T016 — F059-AC-01…05 (TC-TEN-004, TC-AUTH-076, TC-CMS-RBAC-01, TC-CMS-RBAC-04/05): permission catalog read
// only (grouped by group_code, scope filter, catalog_version, roles holding each permission); platform/shared roles
// (no X-Tenant-Id, "Hệ thống"/"Dùng chung", tenant role read only); create with Idempotency-Key and the picker of the
// role scope only; 422 PERMISSION_NOT_ALLOWED_FOR_SCOPE / 412 / 403 shown by code without losing the form; edit with
// If-Match + reason; a role deleted meanwhile (404) closes the editor; delete with confirmation + reason (409 when in
// use); a system role only offers "Nhân bản". Backend = MOCK (src/test/platform-access-backend.ts).
import { configure, fireEvent, screen, waitFor, within } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { tenantFilterStore } from '../../../shell/tenantFilterStore';
import { problem, stopPortals } from '../../../test/portal-harness';
import { AIDS, renderAccess } from '../../../test/platform-access-backend';

// whole-portal flows with several Ant Design selects and dialogs are slow on the throttled shared CPU (2 workers max)
vi.setConfig({ testTimeout: 600_000 });
configure({ asyncUtilTimeout: 30_000 });

afterEach(() => {
  stopPortals();
  tenantFilterStore.reset();
});

const UUIDV7 = /^[0-9a-f]{8}-[0-9a-f]{4}-7[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i;
const rowOf = (text: string) => screen.getByText(text).closest('tr')!;

describe('Permission catalog (F059-AC-01)', () => {
  it('grouped by group_code with catalog_version and holding roles; scope filter; no write action', async () => {
    const { user } = await renderAccess('/platform-admin/access-control?tab=permissions');
    expect(await screen.findByText('platform.role.read')).toBeInTheDocument();
    expect(screen.getByText('Phiên bản catalog: 2026.11.19.1')).toBeInTheDocument();
    expect(screen.getByText('platform_access (2)')).toBeInTheDocument();
    expect(within(rowOf('game.quest.write')).getByText('liveops')).toBeInTheDocument();
    expect(within(rowOf('game.quest.write')).getByText('liveops_lead')).toBeInTheDocument();
    expect(screen.getByRole('searchbox', { name: 'Tìm theo mã' })).toHaveAttribute('maxlength', '64');
    await user.click(screen.getByRole('combobox', { name: 'Phạm vi' }));
    await user.click((await screen.findAllByTitle('Tenant')).at(-1)!);
    await waitFor(() => expect(screen.queryByText('platform.role.read')).toBeNull());
    expect(screen.getByText('lms.class.read')).toBeInTheDocument();
    for (const name of ['Sửa', 'Xóa', 'Thêm', 'Tạo role']) expect(screen.queryByRole('button', { name })).toBeNull();
  });
});

describe('Role platform / dùng chung (F059-AC-02…05)', () => {
  it('list without X-Tenant-Id: "Hệ thống", "Dùng chung", user count; tenant role read only; system role only "Nhân bản"', async () => {
    const { backend, user } = await renderAccess('/platform-admin/access-control?tab=roles');
    await screen.findByText('liveops_lead');
    const req = backend.calls('GET', '/admin/roles').at(-1)!;
    expect(req.headers['x-tenant-id']).toBeUndefined();
    expect(req.gateway).toBe('internal');
    const system = rowOf('system_admin');
    expect(within(system).getByText('Hệ thống')).toBeInTheDocument();
    expect(within(system).getByText('Dùng chung')).toBeInTheDocument();
    expect(within(system).getByRole('button', { name: 'Nhân bản thành role tùy biến' })).toBeInTheDocument();
    expect(within(system).queryByRole('button', { name: 'Sửa' })).toBeNull();
    expect(within(system).queryByRole('button', { name: 'Xóa' })).toBeNull();
    const custom = rowOf('gv_chu_nhiem');
    expect(within(custom).getByText('TH Demo A')).toBeInTheDocument();
    expect(within(custom).getByText(/chỉ xem/)).toBeInTheDocument();
    expect(within(rowOf('tenant_admin')).getByText('3')).toBeInTheDocument();
    await user.click(screen.getByRole('combobox', { name: 'Loại role' }));
    await user.click((await screen.findAllByTitle('Tenant')).at(-1)!);
    await waitFor(() => expect(backend.calls('GET', '/admin/roles').some((r) => new URLSearchParams(r.search).get('role_scope_kind') === 'tenant')).toBe(true));
  });

  it('create: picker of the role scope only, schema limits, Idempotency-Key, success message with principal_cache_ttl', async () => {
    const { backend, user } = await renderAccess('/platform-admin/access-control?tab=roles');
    await user.click(await screen.findByRole('button', { name: 'Tạo role' }));
    const dialog = await screen.findByRole('dialog');
    const code = within(dialog).getByLabelText('Mã role');
    expect(code).toHaveAttribute('maxlength', '64');
    expect(within(dialog).getByLabelText('Tên (tiếng Việt)')).toHaveAttribute('maxlength', '255');
    expect(within(dialog).getByLabelText('Lý do')).toHaveAttribute('maxlength', '1000');
    fireEvent.change(code, { target: { value: 'liveops_shift' } });
    expect(within(dialog).getByText('13/64')).toBeInTheDocument();
    fireEvent.change(within(dialog).getByLabelText('Tên (tiếng Việt)'), { target: { value: 'Trực ca LiveOps' } });
    // platform scope: game.* offered, tenant permissions not
    expect(await within(dialog).findByText('game.quest.write — Sửa quest')).toBeInTheDocument();
    expect(within(dialog).queryByText(/lms\.class\.read/)).toBeNull();
    await user.click(within(dialog).getByText('game.quest.write — Sửa quest'));
    await user.click(within(dialog).getByText('game.event.write — Sửa sự kiện'));
    // switch to tenant: platform/game permissions disappear from the picker
    await user.click(within(dialog).getByLabelText('Tenant'));
    expect(await within(dialog).findByText('lms.class.read — Xem lớp')).toBeInTheDocument();
    expect(within(dialog).queryByText(/game\.quest\.write/)).toBeNull();
    expect(within(dialog).queryByText(/platform\.role/)).toBeNull();
    await user.click(within(dialog).getByLabelText('Platform'));
    await user.click(await within(dialog).findByText('game.quest.write — Sửa quest'));
    await user.click(within(dialog).getByRole('button', { name: 'Lưu' }));
    await waitFor(() => expect(backend.calls('POST', '/admin/roles')).toHaveLength(1));
    const post = backend.calls('POST', '/admin/roles')[0]!;
    expect(post.body).toEqual({ code: 'liveops_shift', name: { vi: 'Trực ca LiveOps' }, role_scope_kind: 'platform', permission_codes: ['game.quest.write'] });
    expect(post.headers['idempotency-key']).toMatch(UUIDV7);
    expect(post.headers['x-tenant-id']).toBeUndefined();
    expect(await screen.findByText('Đã lưu. Thay đổi áp dụng trong ≤ 60 giây cho 0 người đang có role.')).toBeInTheDocument();
  });

  it('422 PERMISSION_NOT_ALLOWED_FOR_SCOPE lists the codes and keeps the form; retry keeps the Idempotency-Key', async () => {
    const { backend, user } = await renderAccess('/platform-admin/access-control?tab=roles');
    let first = true;
    backend.overrides.unshift((r) =>
      r.method === 'POST' && r.path === '/admin/roles' && first
        ? ((first = false), problem(422, 'VALIDATION_FAILED', { errors: [{ field: 'permission_codes[0]', code: 'PERMISSION_NOT_ALLOWED_FOR_SCOPE' }] }))
        : undefined,
    );
    await user.click(await screen.findByRole('button', { name: 'Tạo role' }));
    const dialog = await screen.findByRole('dialog');
    fireEvent.change(within(dialog).getByLabelText('Mã role'), { target: { value: 'gv_bo_mon' } });
    fireEvent.change(within(dialog).getByLabelText('Tên (tiếng Việt)'), { target: { value: 'Giáo viên bộ môn' } });
    await user.click(within(dialog).getByLabelText('Tenant'));
    await user.click(await within(dialog).findByText('lms.class.read — Xem lớp'));
    await user.click(within(dialog).getByRole('button', { name: 'Lưu' }));
    expect(await within(dialog).findByText('Permission không hợp lệ với phạm vi của role: lms.class.read')).toBeInTheDocument();
    expect(within(dialog).getByLabelText('Mã role')).toHaveValue('gv_bo_mon');
    await user.click(within(dialog).getByRole('button', { name: 'Lưu' }));
    await waitFor(() => expect(backend.calls('POST', '/admin/roles')).toHaveLength(2));
    const [a, b] = backend.calls('POST', '/admin/roles');
    expect(b!.headers['idempotency-key']).toBe(a!.headers['idempotency-key']);
    expect(b!.body).toMatchObject({ role_scope_kind: 'tenant', permission_codes: ['lms.class.read'] });
  });

  it('edit: getPlatformRole then If-Match + mandatory reason; 412 and 403 shown by code keeping the form', async () => {
    const { backend, user } = await renderAccess('/platform-admin/access-control?tab=roles');
    await screen.findByText('liveops_lead');
    await user.click(within(rowOf('liveops_lead')).getByRole('button', { name: 'Sửa' }));
    const dialog = await screen.findByRole('dialog');
    await waitFor(() => expect(within(dialog).getByLabelText('Mã role')).toHaveValue('liveops_lead'));
    await user.click(within(dialog).getByRole('button', { name: 'Lưu' }));
    expect(await within(dialog).findByText('Trường này bắt buộc.')).toBeInTheDocument();
    expect(backend.calls('PATCH', `/admin/roles/${AIDS.roleShared}`)).toHaveLength(0);
    let n = 0;
    backend.overrides.unshift((r) => (r.method === 'PATCH' && n < 2 ? (n++, n === 1 ? problem(412, 'VERSION_MISMATCH') : problem(403, 'PERMISSION_DENIED')) : undefined));
    fireEvent.change(within(dialog).getByLabelText('Lý do'), { target: { value: 'Thêm quyền sự kiện' } });
    await user.click(within(dialog).getByRole('button', { name: 'Lưu' }));
    expect(await within(dialog).findByText('Dữ liệu đã được người khác cập nhật. Tải lại rồi thử lại.')).toBeInTheDocument();
    expect(within(dialog).getByLabelText('Lý do')).toHaveValue('Thêm quyền sự kiện');
    await user.click(within(dialog).getByRole('button', { name: 'Lưu' }));
    expect(await within(dialog).findByText('Bạn không có quyền thực hiện thao tác này.')).toBeInTheDocument();
    expect(within(dialog).getByText('Mã truy vết: trace-permission_denied')).toBeInTheDocument();
    await user.click(within(dialog).getByRole('button', { name: 'Lưu' }));
    await waitFor(() => expect(backend.calls('PATCH', `/admin/roles/${AIDS.roleShared}`)).toHaveLength(3));
    const ok = backend.calls('PATCH', `/admin/roles/${AIDS.roleShared}`).at(-1)!;
    expect(ok.headers['if-match']).toBe('W/"3"');
    expect(ok.body).toMatchObject({ code: 'liveops_lead', permission_codes: ['game.quest.write'], reason: 'Thêm quyền sự kiện' });
    expect(await screen.findByText('Đã lưu. Thay đổi áp dụng trong ≤ 60 giây cho 1 người đang có role.')).toBeInTheDocument();
  });

  it('a role deleted meanwhile (404) closes the editor with a message', async () => {
    const { backend, user } = await renderAccess('/platform-admin/access-control?tab=roles');
    await screen.findByText('qa_reader');
    backend.overrides.unshift((r) => (r.method === 'GET' && r.path === `/admin/roles/${AIDS.roleUnused}` ? problem(404, 'NOT_FOUND') : undefined));
    await user.click(within(rowOf('qa_reader')).getByRole('button', { name: 'Sửa' }));
    expect(await screen.findByText('Role không còn tồn tại (đã bị xóa). Quay về danh sách.')).toBeInTheDocument();
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull());
  });

  it('delete: confirmation with the user count + reason; 409 when in use; If-Match and reason sent', async () => {
    const { backend, user } = await renderAccess('/platform-admin/access-control?tab=roles');
    await screen.findByText('liveops_lead');
    await user.click(within(rowOf('liveops_lead')).getByRole('button', { name: 'Xóa' }));
    let dialog = await screen.findByRole('dialog');
    expect(within(dialog).getByText(/Role đang được gán cho 1 người/)).toBeInTheDocument();
    fireEvent.change(within(dialog).getByLabelText('Lý do'), { target: { value: 'Không dùng nữa' } });
    await user.click(within(dialog).getByRole('button', { name: 'Xác nhận' }));
    expect(await within(dialog).findByText('Role còn người dùng: thu hồi các UserRole của role này trước khi xóa.')).toBeInTheDocument();
    await user.click(within(dialog).getByRole('button', { name: 'Hủy' }));
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull());
    await user.click(within(rowOf('qa_reader')).getByRole('button', { name: 'Xóa' }));
    dialog = await screen.findByRole('dialog');
    fireEvent.change(within(dialog).getByLabelText('Lý do'), { target: { value: 'Role thử nghiệm' } });
    await user.click(within(dialog).getByRole('button', { name: 'Xác nhận' }));
    await waitFor(() => expect(backend.calls('DELETE', `/admin/roles/${AIDS.roleUnused}`)).toHaveLength(1));
    const del = backend.calls('DELETE', `/admin/roles/${AIDS.roleUnused}`)[0]!;
    expect(del.headers['if-match']).toBe('W/"2"');
    expect(new URLSearchParams(del.search).get('reason')).toBe('Role thử nghiệm');
    await waitFor(() => expect(screen.queryByText('qa_reader')).toBeNull());
  });

  it('clone a system role into a custom role (create prefilled, scope fixed)', async () => {
    const { backend, user } = await renderAccess('/platform-admin/access-control?tab=roles');
    await screen.findByText('system_admin');
    await user.click(within(rowOf('system_admin')).getByRole('button', { name: 'Nhân bản thành role tùy biến' }));
    const dialog = await screen.findByRole('dialog', { name: 'Nhân bản role hệ thống thành role tùy biến' });
    expect(within(dialog).getByLabelText('Mã role')).toHaveValue('system_admin_custom');
    expect(within(dialog).getByLabelText('Platform')).toBeDisabled();
    await user.click(within(dialog).getByRole('button', { name: 'Lưu' }));
    await waitFor(() => expect(backend.calls('POST', '/admin/roles')).toHaveLength(1));
    expect(backend.calls('POST', '/admin/roles')[0]!.body).toMatchObject({ code: 'system_admin_custom', role_scope_kind: 'platform', permission_codes: ['platform.role.read', 'platform.role.write'] });
  });
});
