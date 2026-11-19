// F059 T029–T032 — F059-AC-11…14 (TC-CMS-RBAC-02, TC-CMS-LOGIN-04): listPlatformUsers (no tenant selected = platform
// staff only; tenant filter → `tenant_ids[]` without X-Tenant-Id and the "Thuộc tenant" column from `User.tenant_ids`;
// `q` within maxLength, type, status, "SIS" tag, students read only without a role button); the grant dialog opened
// from a row offers only the account's member tenants (OQ F059-5); create platform staff (createPlatformUser: no
// X-Tenant-Id, initial platform roles without safety_moderator, reason, Idempotency-Key, 409 USERNAME_TAKEN + hint) and
// school staff (createTenantUser in the target tenant, email mandatory, 409 EMAIL_TAKEN + hint); lock with reason
// (Idempotency-Key, X-Tenant-Id = home tenant), unlock/deactivate; detail drawer with UserRoles per tenant and last
// login, no password/token/IP. Backend = MOCK.
import { configure, fireEvent, screen, waitFor, within } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { tenantFilterStore } from '../../../shell/tenantFilterStore';
import { stopPortals, TENANTS } from '../../../test/portal-harness';
import { AIDS, renderAccess } from '../../../test/platform-access-backend';

vi.setConfig({ testTimeout: 600_000 });
configure({ asyncUtilTimeout: 30_000 });

afterEach(() => {
  stopPortals();
  tenantFilterStore.reset();
});

const UUIDV7 = /^[0-9a-f]{8}-[0-9a-f]{4}-7[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i;
const URL_USERS = '/platform-admin/access-control?tab=users';

async function openDrawer(title: string): Promise<HTMLElement> {
  return (await screen.findByText(title)).closest('.ant-drawer-content') as HTMLElement;
}

async function openModal(title: string): Promise<HTMLElement> {
  return (await screen.findByText(title, { selector: '.ant-modal-title' })).closest('.ant-modal-content') as HTMLElement;
}

async function pick(user: { click: (e: Element) => Promise<void> }, scope: HTMLElement | Document, label: string, title: string) {
  await user.click(within(scope as HTMLElement).getByRole('combobox', { name: label }));
  // option picked with a plain click: under CPU load the closing dropdown can still carry pointer-events: none
  const option = await waitFor(() => {
    const found = screen.getAllByTitle(title).filter((e) => e.classList.contains('ant-select-item-option')).at(-1);
    if (!found) throw new Error(`no option ${title}`);
    return found;
  });
  fireEvent.click(option);
}

describe('Người dùng (F059-AC-11/14)', () => {
  it('platform staff without tenant filter; tenant filter → tenant_ids[] + member tenants; search capped, SIS tag; students read only', async () => {
    const { backend, user } = await renderAccess(URL_USERS);
    const platformRow = (await screen.findByText('cm.duyet')).closest('tr')!;
    expect(within(platformRow).getByText('Nhân sự platform')).toBeInTheDocument();
    expect(screen.getByText(/Chưa chọn tenant: danh sách chỉ gồm nhân sự platform/)).toBeInTheDocument();
    expect(screen.queryByText('qlcm.truong.a')).toBeNull();
    const first = backend.calls('GET', '/admin/users')[0]!;
    expect(new URLSearchParams(first.search).getAll('tenant_ids')).toEqual([]);
    expect(first.headers['x-tenant-id']).toBeUndefined();

    tenantFilterStore.set([TENANTS.thDemoA.tenant_id, TENANTS.pgdDemo.tenant_id]);
    const staff = (await screen.findByText('qlcm.truong.a')).closest('tr')!;
    const req = backend.calls('GET', '/admin/users').at(-1)!;
    expect(new URLSearchParams(req.search).getAll('tenant_ids')).toEqual([TENANTS.thDemoA.tenant_id, TENANTS.pgdDemo.tenant_id]);
    expect(req.headers['x-tenant-id']).toBeUndefined();
    expect(screen.getByRole('columnheader', { name: 'Thuộc tenant' })).toBeInTheDocument();
    expect(within(staff).getByText('Phòng GD Demo')).toBeInTheDocument();
    expect(screen.queryByText('cm.duyet')).toBeNull();

    const search = screen.getByRole('searchbox', { name: 'Tìm theo tên đăng nhập, email, họ tên' });
    expect(search).toHaveAttribute('maxlength', '100');
    fireEvent.change(search, { target: { value: 'hs' } });
    fireEvent.keyDown(search, { key: 'Enter', code: 'Enter', keyCode: 13 });
    await waitFor(() => expect(new URLSearchParams(backend.calls('GET', '/admin/users').at(-1)!.search).get('q')).toBe('hs'));
    await pick(user, document, 'Loại tài khoản', 'Học sinh');
    await waitFor(() => expect(new URLSearchParams(backend.calls('GET', '/admin/users').at(-1)!.search).get('user_type')).toBe('student'));
    const student = (await screen.findByText('hs.an.3a')).closest('tr')!;
    expect(within(student).getByText('SIS')).toBeInTheDocument();
    expect(within(student).queryByRole('button', { name: 'Gán role' })).toBeNull();
  });

  it('detail drawer: UserRoles of every tenant, last login, no password/token/IP', async () => {
    tenantFilterStore.set([TENANTS.thDemoA.tenant_id]);
    const { backend } = await renderAccess(URL_USERS);
    const staffAgain = (await screen.findByText('qlcm.truong.a')).closest('tr')!;
    expect(within(staffAgain).getByRole('button', { name: 'Gán role' })).toBeInTheDocument();
    fireEvent.click(within(staffAgain).getByRole('button', { name: 'Chi tiết' })); // plain click: see pick()
    const drawer = await openDrawer('Chi tiết tài khoản');
    expect(within(drawer).getByText('qlcm.truong.a@thdemo.edu.vn')).toBeInTheDocument();
    expect(within(drawer).getByText(/^18\/11\/2026/)).toBeInTheDocument();
    await waitFor(() => expect(within(drawer).getAllByText('tenant_admin')).toHaveLength(2));
    const urq = backend.calls('GET', '/admin/user-roles').at(-1)!;
    expect(new URLSearchParams(urq.search).get('user_id')).toBe(AIDS.qlcm);
    const text = drawer.textContent ?? '';
    for (const banned of [/mật khẩu/i, /password/i, /token/i, /\bIP\b/]) expect(text).not.toMatch(banned);
  });

  it('status filter is sent as `status`', async () => {
    const { backend, user } = await renderAccess(URL_USERS);
    await screen.findByText('cm.duyet');
    await pick(user, document, 'Trạng thái', 'Đã khóa');
    await waitFor(() => expect(new URLSearchParams(backend.calls('GET', '/admin/users').at(-1)!.search).get('status')).toBe('locked'));
    await waitFor(() => expect(screen.queryByText('cm.duyet')).toBeNull());
  });

  it('grant from a row: target tenant limited to the account member tenants (OQ F059-5), X-Tenant-Id = chosen tenant', async () => {
    tenantFilterStore.set([TENANTS.thDemoA.tenant_id, TENANTS.pgdDemo.tenant_id]);
    const { backend, user } = await renderAccess(URL_USERS);
    const row = (await screen.findByText('qlcm.truong.a')).closest('tr')!;
    fireEvent.click(within(row).getByRole('button', { name: 'Gán role' }));
    const dialog = await openModal('Gán role cho người dùng');
    expect(within(dialog).getByText('Chỉ các tenant mà tài khoản đang thuộc.')).toBeInTheDocument();
    await user.click(within(dialog).getByRole('combobox', { name: 'Tenant' }));
    await waitFor(() => expect(screen.getAllByTitle('Phòng GD Demo (pgd-demo)').length).toBeGreaterThan(0));
    const offered = [...document.querySelectorAll('.ant-select-dropdown:not(.ant-select-dropdown-hidden) .ant-select-item-option')].map((o) => o.getAttribute('title'));
    expect(offered.sort()).toEqual(['Phòng GD Demo (pgd-demo)', 'TH Demo A (th-demo-a)']);
    fireEvent.click(screen.getAllByTitle('Phòng GD Demo (pgd-demo)').filter((e) => e.classList.contains('ant-select-item-option')).at(-1)!);
    await pick(user, dialog, 'Role', 'tenant_admin — Role tenant_admin');
    fireEvent.change(within(dialog).getByLabelText('Lý do'), { target: { value: 'Phụ trách đơn vị' } });
    await user.click(within(dialog).getByRole('button', { name: 'Gán role' }));
    await waitFor(() => expect(backend.calls('POST', '/admin/user-roles')).toHaveLength(1));
    const post = backend.calls('POST', '/admin/user-roles')[0]!;
    expect(post.headers['x-tenant-id']).toBe(TENANTS.pgdDemo.tenant_id);
    expect(post.body).toMatchObject({ user_id: AIDS.qlcm, role_id: AIDS.roleTenantAdmin, scope_type: 'tenant', reason: 'Phụ trách đơn vị' });
  });

  it('lookup by user id opens the drawer; lock asks a reason (Idempotency-Key); unlock / deactivate', async () => {
    const { backend, user } = await renderAccess(URL_USERS);
    fireEvent.change(await screen.findByLabelText('Tra cứu theo mã người dùng'), { target: { value: AIDS.cmDuyet } });
    await user.click(screen.getByRole('button', { name: 'Mở' }));
    const drawer = await openDrawer('Chi tiết tài khoản');
    expect(await within(drawer).findByText('liveops_lead')).toBeInTheDocument();
    await user.click(within(drawer).getByRole('button', { name: 'Khóa' }));
    const dialog = await openModal('Khóa tài khoản');
    expect(within(dialog).getByLabelText('Lý do')).toHaveAttribute('maxlength', '500');
    fireEvent.change(within(dialog).getByLabelText('Lý do'), { target: { value: 'Lộ mật khẩu' } });
    await user.click(within(dialog).getByRole('button', { name: 'Xác nhận' }));
    await waitFor(() => expect(backend.calls('POST', `/admin/users/${AIDS.cmDuyet}:lock`)).toHaveLength(1));
    const lock = backend.calls('POST', `/admin/users/${AIDS.cmDuyet}:lock`)[0]!;
    expect(lock.body).toEqual({ reason: 'Lộ mật khẩu' });
    expect(lock.headers['idempotency-key']).toMatch(UUIDV7);
    expect(lock.headers['x-tenant-id']).toBeUndefined();
    expect(await screen.findByText('Đã khóa tài khoản.')).toBeInTheDocument();
    await user.click(within(drawer).getByRole('button', { name: 'Mở khóa' }));
    const unlock = await openModal('Mở khóa tài khoản');
    fireEvent.change(within(unlock).getByLabelText('Lý do'), { target: { value: 'ab' } });
    await user.click(within(unlock).getByRole('button', { name: 'Xác nhận' }));
    expect(await within(unlock).findByText('Tối thiểu 3 ký tự.')).toBeInTheDocument();
    fireEvent.change(within(unlock).getByLabelText('Lý do'), { target: { value: 'Đã đổi mật khẩu' } });
    await user.click(within(unlock).getByRole('button', { name: 'Xác nhận' }));
    await waitFor(() => expect(backend.calls('POST', `/admin/users/${AIDS.cmDuyet}:unlock`)).toHaveLength(1));
  });

  it('lock of a tenant account sends X-Tenant-Id = its home tenant', async () => {
    tenantFilterStore.set([TENANTS.pgdDemo.tenant_id]);
    const { backend, user } = await renderAccess(URL_USERS);
    fireEvent.click(within((await screen.findByText('cv.pgd')).closest('tr')!).getByRole('button', { name: 'Chi tiết' }));
    const drawer = await openDrawer('Chi tiết tài khoản');
    await user.click(within(drawer).getByRole('button', { name: 'Vô hiệu hóa' }));
    const dialog = await openModal('Vô hiệu hóa tài khoản');
    fireEvent.change(within(dialog).getByLabelText('Lý do'), { target: { value: 'Nghỉ việc' } });
    await user.click(within(dialog).getByRole('button', { name: 'Xác nhận' }));
    await waitFor(() => expect(backend.calls('POST', `/admin/users/${AIDS.pgdStaff}:deactivate`)).toHaveLength(1));
    expect(backend.calls('POST', `/admin/users/${AIDS.pgdStaff}:deactivate`)[0]!.headers['x-tenant-id']).toBe(TENANTS.pgdDemo.tenant_id);
  });
});

describe('Tạo nhân sự (F059-AC-12)', () => {
  it('school staff in the target tenant; email mandatory; 409 EMAIL_TAKEN + hint', async () => {
    const { backend, user } = await renderAccess(URL_USERS);
    await user.click(await screen.findByRole('button', { name: 'Tạo nhân sự' }));
    const dialog = await screen.findByRole('dialog', { name: 'Tạo tài khoản nhân sự' });
    expect(within(dialog).getByLabelText('Nhân sự platform (không có tenant gốc)')).toBeEnabled();
    await pick(user, dialog, 'Tenant', 'TH Demo A (th-demo-a)');
    const username = within(dialog).getByLabelText('Tên đăng nhập');
    expect(username).toHaveAttribute('maxlength', '128');
    expect(within(dialog).getByLabelText('Email')).toHaveAttribute('maxlength', '320');
    fireEvent.change(username, { target: { value: 'gv.moi.a' } });
    fireEvent.change(within(dialog).getByLabelText('Họ tên'), { target: { value: 'Giáo viên mới' } });
    await pick(user, dialog, 'Role ban đầu', 'tenant_admin — Role tenant_admin');
    await user.click(within(dialog).getByRole('button', { name: 'Lưu' }));
    expect(await within(dialog).findByText('Trường này bắt buộc.')).toBeInTheDocument();
    expect(backend.calls('POST', '/tenant/users')).toHaveLength(0);
    fireEvent.change(within(dialog).getByLabelText('Email'), { target: { value: 'qlcm.truong.a@thdemo.edu.vn' } });
    await user.click(within(dialog).getByRole('button', { name: 'Lưu' }));
    expect(await within(dialog).findByText('Email đã thuộc một tài khoản khác.')).toBeInTheDocument();
    expect(within(dialog).getByText(/gán role cho tài khoản sẵn có/)).toBeInTheDocument();
    fireEvent.change(within(dialog).getByLabelText('Email'), { target: { value: 'gv.moi.a@thdemo.edu.vn' } });
    await user.click(within(dialog).getByRole('button', { name: 'Lưu' }));
    await waitFor(() => expect(backend.calls('POST', '/tenant/users')).toHaveLength(2));
    const [dup, ok] = backend.calls('POST', '/tenant/users');
    expect(ok!.headers['x-tenant-id']).toBe(TENANTS.thDemoA.tenant_id);
    expect(ok!.headers['idempotency-key']).toMatch(UUIDV7);
    expect(ok!.headers['idempotency-key']).not.toBe(dup!.headers['idempotency-key']);
    expect(ok!.body).toEqual({
      username: 'gv.moi.a',
      email: 'gv.moi.a@thdemo.edu.vn',
      display_name: 'Giáo viên mới',
      user_type: 'staff',
      locale: 'vi-VN',
      initial_user_roles: [{ role_id: AIDS.roleTenantAdmin, scope_type: 'tenant' }],
    });
    expect(await screen.findByText('Đã tạo tài khoản.')).toBeInTheDocument();
  });

  it('platform staff: createPlatformUser without X-Tenant-Id, platform roles (no safety_moderator), reason; 409 USERNAME_TAKEN + hint', async () => {
    const { backend, user } = await renderAccess(URL_USERS);
    await user.click(await screen.findByRole('button', { name: 'Tạo nhân sự' }));
    const dialog = await screen.findByRole('dialog', { name: 'Tạo tài khoản nhân sự' });
    await user.click(within(dialog).getByLabelText('Nhân sự platform (không có tenant gốc)'));
    expect(within(dialog).queryByRole('combobox', { name: 'Tenant' })).toBeNull();
    fireEvent.change(within(dialog).getByLabelText('Tên đăng nhập'), { target: { value: 'cm.duyet' } });
    fireEvent.change(within(dialog).getByLabelText('Email'), { target: { value: 'liveops.moi@eduworld.vn' } });
    fireEvent.change(within(dialog).getByLabelText('Họ tên'), { target: { value: 'Vận hành mới' } });
    await user.click(within(dialog).getByRole('button', { name: 'Lưu' }));
    expect(await within(dialog).findByText('Chọn role.')).toBeInTheDocument();
    await user.click(within(dialog).getByRole('combobox', { name: 'Role platform ban đầu' }));
    await waitFor(() => expect(screen.getAllByTitle('liveops_lead — Role liveops_lead').length).toBeGreaterThan(0));
    const offered = [...document.querySelectorAll('.ant-select-dropdown:not(.ant-select-dropdown-hidden) .ant-select-item-option')].map((o) => o.getAttribute('title'));
    expect(offered).not.toContain('safety_moderator — Role safety_moderator');
    expect(offered).not.toContain('tenant_admin — Role tenant_admin');
    fireEvent.click(screen.getAllByTitle('liveops_lead — Role liveops_lead').filter((e) => e.classList.contains('ant-select-item-option')).at(-1)!);
    const reason = within(dialog).getByLabelText('Lý do');
    expect(reason).toHaveAttribute('maxlength', '1000');
    fireEvent.change(reason, { target: { value: 'Mở rộng đội vận hành' } });
    await user.click(within(dialog).getByRole('button', { name: 'Lưu' }));
    expect(await within(dialog).findByText('Tên đăng nhập đã thuộc một tài khoản khác.')).toBeInTheDocument();
    expect(within(dialog).getByText(/gán role cho tài khoản sẵn có/)).toBeInTheDocument();
    fireEvent.change(within(dialog).getByLabelText('Tên đăng nhập'), { target: { value: 'liveops.moi' } });
    await user.click(within(dialog).getByRole('button', { name: 'Lưu' }));
    await waitFor(() => expect(backend.calls('POST', '/admin/users')).toHaveLength(2));
    const [dup, ok] = backend.calls('POST', '/admin/users');
    for (const r of [dup!, ok!]) {
      expect(r.headers['x-tenant-id']).toBeUndefined();
      expect(r.headers['idempotency-key']).toMatch(UUIDV7);
    }
    expect(ok!.headers['idempotency-key']).not.toBe(dup!.headers['idempotency-key']);
    expect(ok!.body).toEqual({
      username: 'liveops.moi',
      email: 'liveops.moi@eduworld.vn',
      display_name: 'Vận hành mới',
      locale: 'vi-VN',
      initial_role_ids: [AIDS.roleShared],
      reason: 'Mở rộng đội vận hành',
    });
    expect(backend.calls('POST', '/tenant/users')).toHaveLength(0);
    expect(await screen.findByText('Đã tạo tài khoản.')).toBeInTheDocument();
  });
});
