// F059 T021–T025 — F059-AC-06…10, F059-AC-26 (TC-TEN-004, TC-TEN-011c, TC-CMS-AUTHZ-03, TC-CMS-RBAC-03): UserRoles of
// every tenant (filters user/role/tenant_ids[], columns), grant to platform staff (platform roles only, scope platform,
// no X-Tenant-Id) and to staff with a home tenant (tenant roles of the target tenant, X-Tenant-Id = target, scope
// school with its id), reason + Idempotency-Key kept on retry, valid_to after valid_from, server 422/409 by code,
// safety_moderator needs the training date, revoke with reason and the UserRole's tenant. Backend = MOCK.
import { configure, fireEvent, screen, waitFor, within } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { tenantFilterStore } from '../../../shell/tenantFilterStore';
import { problem, stopPortals, TENANTS } from '../../../test/portal-harness';
import { AIDS, renderAccess } from '../../../test/platform-access-backend';

// whole-portal flows with several Ant Design selects and dialogs are slow on the throttled shared CPU (2 workers max);
// valid_to > valid_from is checked by validRange (shared/authz/groupPermissions.test.ts) used as the form rule
vi.setConfig({ testTimeout: 600_000 });
configure({ asyncUtilTimeout: 30_000 });

afterEach(() => {
  stopPortals();
  tenantFilterStore.reset();
});

const UUIDV7 = /^[0-9a-f]{8}-[0-9a-f]{4}-7[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i;
const URL_UR = '/platform-admin/access-control?tab=user-roles';

async function pick(user: { click: (e: Element) => Promise<void> }, scope: HTMLElement, label: string, title: string) {
  await user.click(within(scope).getByRole('combobox', { name: label }));
  await user.click((await screen.findAllByTitle(title)).at(-1)!);
}

describe('UserRole list (F059-AC-06/09)', () => {
  it('columns, filters user_id / role_id / tenant_ids[] without X-Tenant-Id', async () => {
    const { backend, user } = await renderAccess(URL_UR);
    expect((await screen.findAllByText('tenant_admin')).length).toBe(2);
    const sis = screen.getByText('SIS').closest('tr')!;
    expect(within(sis).getByText('TH Demo A')).toBeInTheDocument();
    expect(within(sis).getByText(/30\/06\/2027/)).toBeInTheDocument();
    const school = screen.getByText(AIDS.school).closest('tr')!;
    expect(within(school).getByText('Trường')).toBeInTheDocument();
    expect(within(school).getByText('Phòng GD Demo')).toBeInTheDocument();
    const platform = screen.getByText('liveops_lead').closest('tr')!;
    expect(within(platform).getAllByText('Platform').length).toBe(2); // scope + tenant cell
    expect(within(platform).getByText(/không thời hạn/)).toBeInTheDocument();
    fireEvent.change(screen.getByLabelText('Mã người dùng (UUID)'), { target: { value: AIDS.qlcm } });
    await user.click(screen.getByRole('button', { name: 'Tìm' }));
    await waitFor(() => expect(backend.calls('GET', '/admin/user-roles').some((r) => new URLSearchParams(r.search).get('user_id') === AIDS.qlcm)).toBe(true));
    await waitFor(() => expect(screen.queryByText('liveops_lead')).toBeNull());
    tenantFilterStore.set([TENANTS.pgdDemo.tenant_id]);
    await waitFor(() => expect(backend.calls('GET', '/admin/user-roles').some((r) => new URLSearchParams(r.search).getAll('tenant_ids').includes(TENANTS.pgdDemo.tenant_id))).toBe(true));
    for (const r of backend.calls('GET', '/admin/user-roles')) expect(r.headers['x-tenant-id']).toBeUndefined();
  });

  it('revoke: confirmation + reason, X-Tenant-Id = tenant of the UserRole; platform UserRole without header', async () => {
    const { backend, user } = await renderAccess(URL_UR);
    await screen.findByText(AIDS.school);
    await user.click(within(screen.getByText(AIDS.school).closest('tr')!).getByRole('button', { name: 'Thu hồi' }));
    let dialog = await screen.findByRole('dialog');
    expect(within(dialog).getByText('Người dùng mất quyền của role này trong ≤ 60 giây.')).toBeInTheDocument();
    await user.click(within(dialog).getByRole('button', { name: 'Xác nhận' }));
    expect(await within(dialog).findByText('Vui lòng nhập lý do.')).toBeInTheDocument();
    fireEvent.change(within(dialog).getByLabelText('Lý do'), { target: { value: 'Chuyển công tác' } });
    await user.click(within(dialog).getByRole('button', { name: 'Xác nhận' }));
    await waitFor(() => expect(backend.calls('DELETE', `/admin/user-roles/${AIDS.urPgd}`)).toHaveLength(1));
    const del = backend.calls('DELETE', `/admin/user-roles/${AIDS.urPgd}`)[0]!;
    expect(del.headers['x-tenant-id']).toBe(TENANTS.pgdDemo.tenant_id);
    expect(new URLSearchParams(del.search).get('reason')).toBe('Chuyển công tác');
    await waitFor(() => expect(screen.queryByText(AIDS.school)).toBeNull());
    await user.click(within(screen.getByText('liveops_lead').closest('tr')!).getByRole('button', { name: 'Thu hồi' }));
    dialog = await screen.findByRole('dialog');
    fireEvent.change(within(dialog).getByLabelText('Lý do'), { target: { value: 'Hết ca trực' } });
    await user.click(within(dialog).getByRole('button', { name: 'Xác nhận' }));
    await waitFor(() => expect(backend.calls('DELETE', `/admin/user-roles/${AIDS.urPlatform}`)).toHaveLength(1));
    expect(backend.calls('DELETE', `/admin/user-roles/${AIDS.urPlatform}`)[0]!.headers['x-tenant-id']).toBeUndefined();
  });
});

describe('Gán UserRole (F059-AC-07/08/10/26)', () => {
  it('platform staff: platform roles only, scope platform, no X-Tenant-Id; reason + Idempotency-Key kept on retry; 409 by code', async () => {
    const { backend, user } = await renderAccess(URL_UR);
    let first = true;
    backend.overrides.unshift((r) => (r.method === 'POST' && r.path === '/admin/user-roles' && first ? ((first = false), Promise.reject(new TypeError('Failed to fetch'))) : undefined));
    await user.click(await screen.findByRole('button', { name: 'Gán role' }));
    const dialog = await screen.findByRole('dialog');
    fireEvent.change(within(dialog).getByLabelText('Mã người dùng (UUID)'), { target: { value: AIDS.cmDuyet } });
    await user.click(within(dialog).getByRole('combobox', { name: 'Role' }));
    expect(await screen.findByTitle('qa_reader — Role qa_reader')).toBeInTheDocument();
    expect(screen.queryByTitle(/^tenant_admin/)).toBeNull();
    expect(screen.queryByTitle(/^gv_chu_nhiem/)).toBeNull();
    await user.click(screen.getByTitle('qa_reader — Role qa_reader'));
    expect(within(dialog).getByRole('combobox', { name: 'Phạm vi' })).toBeDisabled();
    const reason = within(dialog).getByLabelText('Lý do');
    expect(reason).toHaveAttribute('maxlength', '1000');
    await user.click(within(dialog).getByRole('button', { name: 'Gán role' }));
    expect(await within(dialog).findByText('Trường này bắt buộc.')).toBeInTheDocument();
    fireEvent.change(reason, { target: { value: 'Đội QA mới' } });
    await user.click(within(dialog).getByRole('button', { name: 'Gán role' }));
    await waitFor(() => expect(backend.calls('POST', '/admin/user-roles').length).toBeGreaterThanOrEqual(2));
    const [lost, ok] = backend.calls('POST', '/admin/user-roles');
    expect(ok!.headers['idempotency-key']).toMatch(UUIDV7);
    expect(ok!.headers['idempotency-key']).toBe(lost!.headers['idempotency-key']);
    expect(ok!.headers['x-tenant-id']).toBeUndefined();
    expect(ok!.body).toEqual({ user_id: AIDS.cmDuyet, role_id: AIDS.roleUnused, scope_type: 'platform', reason: 'Đội QA mới' });
    expect(await screen.findByText('Đã gán. Quyền mới có hiệu lực trong ≤ 60 giây.')).toBeInTheDocument();
    // same grant again → 409 shown by code
    await user.click(screen.getByRole('button', { name: 'Gán role' }));
    const again = await screen.findByRole('dialog');
    fireEvent.change(within(again).getByLabelText('Mã người dùng (UUID)'), { target: { value: AIDS.cmDuyet } });
    await pick(user, again, 'Role', 'qa_reader — Role qa_reader');
    fireEvent.change(within(again).getByLabelText('Lý do'), { target: { value: 'Gán lại' } });
    await user.click(within(again).getByRole('button', { name: 'Gán role' }));
    expect(await within(again).findByText('Dữ liệu đã thay đổi hoặc bị trùng.')).toBeInTheDocument();
    expect(within(again).getByText('Mã truy vết: trace-conflict')).toBeInTheDocument();
  });

  it('staff with a home tenant: target tenant, tenant roles of that tenant, scope school + id, X-Tenant-Id = target', async () => {
    const { backend, user } = await renderAccess(URL_UR);
    await user.click(await screen.findByRole('button', { name: 'Gán role' }));
    const dialog = await screen.findByRole('dialog');
    await user.click(within(dialog).getByLabelText('Nhân sự trường (có tenant gốc)'));
    await pick(user, dialog, 'Tenant', 'Phòng GD Demo (pgd-demo)');
    await pick(user, dialog, 'Nhân sự của tenant đích', 'Chuyên viên PGD (cv.pgd)');
    await user.click(within(dialog).getByRole('combobox', { name: 'Role' }));
    expect(await screen.findByTitle('tenant_admin — Role tenant_admin')).toBeInTheDocument();
    expect(screen.queryByTitle(/^liveops/)).toBeNull();
    expect(screen.queryByTitle(/^gv_chu_nhiem/)).toBeNull(); // custom role of another tenant
    await user.click(screen.getByTitle('tenant_admin — Role tenant_admin'));
    await pick(user, dialog, 'Phạm vi', 'Trường');
    await pick(user, dialog, 'Trường', 'Trường TH A (th-a)');
    fireEvent.change(within(dialog).getByLabelText('Lý do'), { target: { value: 'Phụ trách trường A' } });
    await user.click(within(dialog).getByRole('button', { name: 'Gán role' }));
    await waitFor(() => expect(backend.calls('POST', '/admin/user-roles')).toHaveLength(1));
    const post = backend.calls('POST', '/admin/user-roles')[0]!;
    expect(post.headers['x-tenant-id']).toBe(TENANTS.pgdDemo.tenant_id);
    expect(post.body).toMatchObject({ user_id: AIDS.pgdStaff, role_id: AIDS.roleTenantAdmin, scope_type: 'school', scope_id: AIDS.school, reason: 'Phụ trách trường A' });
    const staff = backend.calls('GET', '/tenant/users').at(-1)!;
    expect(staff.headers['x-tenant-id']).toBe(TENANTS.pgdDemo.tenant_id);
  });

  it('safety_moderator needs the training date (sent as training_completed_at); 422 ROLE_NOT_FOUND by code', async () => {
    const { backend, user } = await renderAccess(URL_UR);
    await user.click(await screen.findByRole('button', { name: 'Gán role' }));
    const dialog = await screen.findByRole('dialog');
    fireEvent.change(within(dialog).getByLabelText('Mã người dùng (UUID)'), { target: { value: AIDS.cmDuyet } });
    await pick(user, dialog, 'Role', 'safety_moderator — Role safety_moderator');
    expect(await within(dialog).findByText('Role safety_moderator cần ngày hoàn thành đào tạo an toàn trẻ em.')).toBeInTheDocument();
    fireEvent.change(within(dialog).getByLabelText('Lý do'), { target: { value: 'Kiểm duyệt viên mới' } });
    await user.click(within(dialog).getByRole('button', { name: 'Gán role' }));
    expect(await within(dialog).findByText('Nhập ngày hoàn thành đào tạo.')).toBeInTheDocument();
    expect(backend.calls('POST', '/admin/user-roles')).toHaveLength(0);
    const date = within(dialog).getByLabelText('Ngày hoàn thành đào tạo');
    await user.click(date);
    fireEvent.change(date, { target: { value: '15/11/2026' } });
    fireEvent.keyDown(date, { key: 'Enter', code: 'Enter', keyCode: 13 });
    let first = true;
    backend.overrides.unshift((r) => (r.method === 'POST' && r.path === '/admin/user-roles' && first ? ((first = false), problem(422, 'ROLE_NOT_FOUND')) : undefined));
    await user.click(within(dialog).getByRole('button', { name: 'Gán role' }));
    expect(await within(dialog).findByText('Role không tồn tại hoặc đã bị xóa.')).toBeInTheDocument();
    await user.click(within(dialog).getByRole('button', { name: 'Gán role' }));
    await waitFor(() => expect(backend.calls('POST', '/admin/user-roles')).toHaveLength(2));
    const body = backend.calls('POST', '/admin/user-roles')[1]!.body as Record<string, string>;
    expect(body.role_id).toBe(AIDS.roleSafety);
    expect(body.training_completed_at).toMatch(/^2026-11-1[45]T/);
  });
});
