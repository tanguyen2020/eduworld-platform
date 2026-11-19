// F059 T029–T032 — F059-AC-11…14 (TC-CMS-RBAC-02, TC-CMS-LOGIN-04): accounts of one tenant (listTenantUsers with
// X-Tenant-Id = that tenant, q within maxLength, type, "SIS" tag, students read only without a role button) while
// `listPlatformUsers` is missing (said on screen); create school staff (createTenantUser in the target tenant, email
// mandatory, Idempotency-Key, 409 EMAIL_TAKEN + hint; "Nhân sự platform" disabled until createPlatformUser); lock with
// reason (Idempotency-Key, X-Tenant-Id = home tenant), unlock/deactivate; detail drawer with UserRoles per tenant and
// last login, no password/token/IP. Backend = MOCK.
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
  it('tenant accounts with X-Tenant-Id, search capped, SIS tag; students read only; detail drawer without secrets', async () => {
    const { backend, user } = await renderAccess(URL_USERS);
    expect(await screen.findByText(/listPlatformUsers/)).toBeInTheDocument();
    await pick(user, document, 'Tenant', 'TH Demo A (th-demo-a)');
    expect(await screen.findByText('qlcm.truong.a')).toBeInTheDocument();
    const req = backend.calls('GET', '/tenant/users').at(-1)!;
    expect(req.headers['x-tenant-id']).toBe(TENANTS.thDemoA.tenant_id);
    expect(new URLSearchParams(req.search).get('user_type')).toBe('staff');
    const search = screen.getByRole('searchbox', { name: 'Tìm theo tên đăng nhập, email, họ tên' });
    expect(search).toHaveAttribute('maxlength', '100');
    await pick(user, document, 'Loại tài khoản', 'Học sinh');
    const student = (await screen.findByText('hs.an.3a')).closest('tr')!;
    expect(within(student).getByText('SIS')).toBeInTheDocument();
    expect(within(student).queryByRole('button', { name: 'Gán role' })).toBeNull();
    await pick(user, document, 'Loại tài khoản', 'Nhân sự');
    const staff = (await screen.findByText('qlcm.truong.a')).closest('tr')!;
    expect(within(staff).getByRole('button', { name: 'Gán role' })).toBeInTheDocument();
    fireEvent.click(within(staff).getByRole('button', { name: 'Chi tiết' })); // plain click: see pick()
    const drawer = await openDrawer('Chi tiết tài khoản');
    expect(within(drawer).getByText('qlcm.truong.a@thdemo.edu.vn')).toBeInTheDocument();
    expect(within(drawer).getByText(/^18\/11\/2026/)).toBeInTheDocument();
    await waitFor(() => expect(within(drawer).getAllByText('tenant_admin')).toHaveLength(2));
    expect(within(drawer).getByText('Phòng GD Demo')).toBeInTheDocument();
    const urq = backend.calls('GET', '/admin/user-roles').at(-1)!;
    expect(new URLSearchParams(urq.search).get('user_id')).toBe(AIDS.qlcm);
    const text = drawer.textContent ?? '';
    for (const banned of [/mật khẩu/i, /password/i, /token/i, /\bIP\b/]) expect(text).not.toMatch(banned);
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
    const { backend, user } = await renderAccess(URL_USERS);
    await screen.findByText(/listPlatformUsers/);
    await pick(user, document, 'Tenant', 'Phòng GD Demo (pgd-demo)');
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
  it('school staff in the target tenant; email mandatory; 409 EMAIL_TAKEN + hint; platform staff disabled', async () => {
    const { backend, user } = await renderAccess(URL_USERS);
    await user.click(await screen.findByRole('button', { name: 'Tạo nhân sự' }));
    const dialog = await screen.findByRole('dialog', { name: 'Tạo tài khoản nhân sự' });
    expect(within(dialog).getByLabelText('Nhân sự platform (không có tenant gốc)')).toBeDisabled();
    expect(within(dialog).getByText(/createPlatformUser/)).toBeInTheDocument();
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
});
