// F052 T027–T031 — F052-AC-12..F052-AC-18, F052-AC-25, F052-AC-26 (TC-CMS-RBAC-01/02/03/05/06, TC-AUTH-073, TC-AUTH-076,
// TC-AUTH-077, TC-AUTH-078, TC-CMS-LOGIN-04, TC-TEN-024): users list (students read-only "SIS", no role action),
// create staff (limits from OpenAPI, 409 at the field + hint), permission picker (tenant permissions only, refused
// permission marked), system role read-only + clone, delete conflict hint, grant (no Lớp scope, reason), revoke with
// the principal_cache_ttl notice, permission log, read-only user without write buttons. Backend = MOCK.
import { fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { I18nextProvider } from 'react-i18next';
import { afterEach, describe, expect, it } from 'vitest';
import { createI18n } from '../../../i18n';
import { accountWith, IDS, READ_ONLY_PERMS, renderSchoolLms, stopPortals, TENANTS } from '../../../test/school-lms-harness';
import { PermissionPicker, refusedPermissions } from '../roles/PermissionPicker';
import { cloneDraft } from '../roles/RoleEditor';
import { GRANT_SCOPES, grantableRoles, grantMode, groupPermissions, scopesFor, type Permission, type Role } from './canGrant';

afterEach(() => stopPortals());

const perm = (code: string, scope_kind: 'tenant' | 'platform', group_code = 'g'): Permission => ({ code, scope_kind, group_code, name: { vi: code, en: code } });

describe('grant helpers (pure)', () => {
  it('no class scope; a School Admin gets Trường/Môn only (ADR-0072)', () => {
    expect(GRANT_SCOPES).not.toContain('class' as never);
    expect(scopesFor({ kind: 'tenant' })).toEqual(['tenant', 'school', 'subject']);
    expect(scopesFor({ kind: 'school', schoolIds: ['s'] })).toEqual(['school', 'subject']);
  });
  it('grant mode from the effective roles', () => {
    expect(grantMode([{ role: 'tenant_admin', scope_type: 'tenant' }])).toEqual({ kind: 'tenant' });
    expect(grantMode([{ role: 'school_admin', scope_type: 'school', scope_id: 's1' }])).toEqual({ kind: 'school', schoolIds: ['s1'] });
  });
  it('only roles whose permissions the granter holds', () => {
    const roles = [
      { role_id: 'a', code: 'a', role_scope_kind: 'tenant', permission_codes: ['x.read'] },
      { role_id: 'b', code: 'b', role_scope_kind: 'tenant', permission_codes: ['x.read', 'tenant.user_role.write'] },
      { role_id: 'c', code: 'c', role_scope_kind: 'platform', permission_codes: ['x.read'] },
    ] as Role[];
    expect(grantableRoles(roles, ['x.read']).map((r) => r.code)).toEqual(['a']);
  });
  it('picker groups only tenant permissions; empty groups disappear (F052-AC-14)', () => {
    expect(groupPermissions([perm('a.read', 'tenant', 'lms'), perm('game.quest.write', 'platform', 'game')])).toEqual([{ group: 'lms', items: [perm('a.read', 'tenant', 'lms')] }]);
  });
  it('refused permissions from errors[] (PERMISSION_NOT_ALLOWED_FOR_SCOPE)', () => {
    expect([...refusedPermissions({ 'permission_codes.1': { code: 'PERMISSION_NOT_ALLOWED_FOR_SCOPE' }, name: { code: 'REQUIRED' } }, ['a.read', 'game.quest.write'])]).toEqual(['game.quest.write']);
  });
  it('clone of a system role is a custom draft with the same permissions (F052-AC-15)', () => {
    expect(cloneDraft({ code: 'school_admin', name: { vi: 'Quản trị trường' }, permission_codes: ['x.read'] })).toEqual({ code: 'school_admin_custom', name_vi: 'Quản trị trường', name_en: undefined, permission_codes: ['x.read'] });
  });
  it('picker marks a refused permission in error', async () => {
    const i18n = await createI18n('vi-VN');
    render(
      <I18nextProvider i18n={i18n}>
        <PermissionPicker permissions={[perm('tenant.class.read', 'tenant', 'lms')]} value={['tenant.class.read']} invalid={new Set(['tenant.class.read'])} />
      </I18nextProvider>,
    );
    expect(screen.getByRole('alert')).toHaveTextContent('Permission này không được dùng cho role của tenant.');
  });
});

describe('Người dùng & phân quyền', () => {
  const unitAdmin = () => accountWith('ad.donvi', TENANTS.thDemoA, [...READ_ONLY_PERMS, 'tenant.user.write', 'tenant.role.write', 'tenant.user_role.write', 'tenant.class.read', 'lms.report.read']);

  it('students are read-only with the SIS tag and no role action (F052-AC-12, TC-CMS-RBAC-02)', async () => {
    await renderSchoolLms('/school-lms/access-control', { account: unitAdmin() });
    fireEvent.mouseDown(await screen.findByRole('combobox', { name: 'Loại' }));
    expect(await screen.findByText('Nguyễn An')).toBeInTheDocument();
    const student = screen.getByText('Nguyễn An').closest('tr')!;
    expect(within(student).getByTestId('sis-tag')).toBeInTheDocument();
    expect(within(student).queryByRole('button')).toBeNull();
    const staff = screen.getByText('Cán bộ B').closest('tr')!;
    expect(within(staff).getByRole('button', { name: 'Phân quyền' })).toBeInTheDocument();
  });

  it('create staff: limits of OpenAPI, 409 EMAIL_TAKEN at the field + hint (F052-AC-13, F052-AC-26, TC-AUTH-073)', async () => {
    const { backend } = await renderSchoolLms('/school-lms/access-control', { account: unitAdmin() });
    fireEvent.click(await screen.findByRole('button', { name: 'Thêm nhân sự' }));
    const form = await screen.findByTestId('create-staff-form');
    expect(within(form).getByLabelText('Tên đăng nhập')).toHaveAttribute('maxlength', '128');
    expect(within(form).getByLabelText('Email')).toHaveAttribute('maxlength', '320');
    fireEvent.change(within(form).getByLabelText('Họ tên'), { target: { value: 'Cán bộ C' } });
    fireEvent.change(within(form).getByLabelText('Tên đăng nhập'), { target: { value: 'ad truong' } });
    fireEvent.change(within(form).getByLabelText('Email'), { target: { value: 'AD.A@thdemo.edu.vn' } });
    fireEvent.mouseDown(within(form).getByLabelText('Role ban đầu'));
    fireEvent.click(await screen.findByText('Tổ trưởng Toán', { selector: '.ant-select-item-option-content' }));
    fireEvent.click(screen.getByRole('button', { name: 'Lưu' }));
    expect(await within(form).findByText('Giá trị không đúng định dạng.')).toBeInTheDocument();
    expect(backend.calls('POST', '/tenant/users')).toHaveLength(0);
    fireEvent.change(within(form).getByLabelText('Tên đăng nhập'), { target: { value: 'cb.c.truong.a' } });
    fireEvent.click(screen.getByRole('button', { name: 'Lưu' }));
    expect(await within(form).findByText('Email đã được dùng.')).toBeInTheDocument();
    expect(screen.getByText(/gán role cho tài khoản sẵn có/)).toBeInTheDocument();
    const call = backend.calls('POST', '/tenant/users')[0]!;
    expect(call.headers['idempotency-key']).toBeTruthy();
    expect(call.body).toMatchObject({ user_type: 'staff', initial_user_roles: [{ role_id: IDS.roleCustom, scope_type: 'tenant' }] });
  });

  it('change email: If-Match of the row, 409 EMAIL_TAKEN at the field, unverified note, then saved (OQ F012-1, F052 T063)', async () => {
    const { backend } = await renderSchoolLms('/school-lms/access-control', { account: unitAdmin() });
    const staff = (await screen.findByText('Cán bộ B')).closest('tr')!;
    fireEvent.click(within(staff).getByRole('button', { name: 'Đổi email' }));
    const form = await screen.findByTestId('change-email-form');
    const email = within(form).getByLabelText('Email');
    expect(email).toHaveValue('qlcm.a@thdemo.edu.vn');
    expect(email).toHaveAttribute('maxlength', '320');
    expect(screen.getByText(/chưa xác minh trên hệ thống đăng nhập/)).toBeInTheDocument();
    expect(within(form).queryByLabelText('Tên đăng nhập')).toBeNull();
    fireEvent.change(email, { target: { value: 'AD.A@thdemo.edu.vn' } });
    fireEvent.click(screen.getByRole('button', { name: 'Lưu' }));
    expect(await within(form).findByText('Email đã được dùng.')).toBeInTheDocument();
    fireEvent.change(email, { target: { value: 'qlcm.moi@thdemo.edu.vn' } });
    fireEvent.click(screen.getByRole('button', { name: 'Lưu' }));
    await waitFor(() => expect(backend.calls('PATCH', `/tenant/users/${IDS.userQlcm}`)).toHaveLength(2));
    const call = backend.calls('PATCH', `/tenant/users/${IDS.userQlcm}`)[1]!;
    expect(call.body).toEqual({ email: 'qlcm.moi@thdemo.edu.vn' });
    expect(call.headers['if-match']).toBe('W/"1"');
    await waitFor(() => expect(screen.queryByTestId('change-email-form')).toBeNull());
  });

  it('system role read-only with "Nhân bản"; custom role delete 409 → revoke hint (F052-AC-15, TC-CMS-RBAC-05)', async () => {
    const { backend } = await renderSchoolLms('/school-lms/access-control', { account: unitAdmin() });
    fireEvent.click(await screen.findByRole('tab', { name: 'Role' }));
    fireEvent.click(await screen.findByRole('button', { name: 'school_admin' }));
    let editor = await screen.findByTestId('role-editor');
    expect(await within(editor).findByRole('button', { name: 'Nhân bản thành role tùy biến' })).toBeInTheDocument();
    expect(within(editor).queryByRole('button', { name: 'Xóa' })).toBeNull();
    fireEvent.click(within(editor).getByRole('button', { name: 'Nhân bản thành role tùy biến' }));
    await waitFor(() => expect(screen.getByDisplayValue('school_admin_custom')).toBeInTheDocument());
    fireEvent.click(document.querySelector('.ant-drawer-close') as HTMLElement);
    fireEvent.click(await screen.findByRole('button', { name: 'to_truong_toan' }));
    editor = await screen.findByTestId('role-editor');
    fireEvent.click(await within(editor).findByRole('button', { name: 'Xóa' }));
    const dialog = (await screen.findAllByRole('dialog')).at(-1)!;
    fireEvent.change(within(dialog).getByLabelText('Lý do'), { target: { value: 'Không còn dùng' } });
    fireEvent.click(within(dialog).getByRole('button', { name: 'Xóa' }));
    expect(await screen.findByText(/thu hồi các UserRole của role này trước/)).toBeInTheDocument();
    const del = backend.calls('DELETE', `/tenant/roles/${IDS.roleCustom}`)[0]!;
    expect(del.headers['if-match']).toBe('"v2"');
    expect(del.search).toContain('reason=');
  });

  it('grant: no Lớp scope, reason required, Idempotency-Key (F052-AC-16, TC-AUTH-077)', async () => {
    const { backend } = await renderSchoolLms('/school-lms/access-control', { account: unitAdmin() });
    const staff = (await screen.findByText('Cán bộ B')).closest('tr')!;
    fireEvent.click(within(staff).getByRole('button', { name: 'Phân quyền' }));
    fireEvent.click(await screen.findByRole('button', { name: 'Gán role' }));
    const form = await screen.findByTestId('grant-form');
    expect(within(form).getByRole('radio', { name: 'Đơn vị' })).toBeInTheDocument();
    expect(within(form).getByRole('radio', { name: 'Trường' })).toBeInTheDocument();
    expect(within(form).queryByRole('radio', { name: 'Lớp' })).toBeNull();
    fireEvent.click(within(form).getByRole('radio', { name: 'Môn' }));
    fireEvent.mouseDown(within(form).getByRole('combobox', { name: 'Role' }));
    fireEvent.click(await screen.findByText('Quản lý chuyên môn', { selector: '.ant-select-item-option-content' }));
    fireEvent.mouseDown(await within(form).findByRole('combobox', { name: 'Môn' }));
    fireEvent.click(await screen.findByText('Toán', { selector: '.ant-select-item-option-content' }));
    const dialog = form.closest('.ant-modal')! as HTMLElement;
    fireEvent.click(within(dialog).getByRole('button', { name: 'Gán role' }));
    expect(await within(form).findByText('Trường này bắt buộc.')).toBeInTheDocument();
    fireEvent.change(within(form).getByLabelText('Lý do'), { target: { value: 'Phân công tổ Toán' } });
    fireEvent.click(within(dialog).getByRole('button', { name: 'Gán role' }));
    await waitFor(() => expect(backend.calls('POST', '/tenant/user-roles')).toHaveLength(1));
    expect(backend.calls('POST', '/tenant/user-roles')[0]!.body).toMatchObject({ user_id: IDS.userQlcm, role_id: IDS.roleAcademic, scope_type: 'subject', scope_id: IDS.subjectMath, reason: 'Phân công tổ Toán' });
    expect(backend.calls('POST', '/tenant/user-roles')[0]!.headers['idempotency-key']).toBeTruthy();
  }, 240_000);

  it('revoke: reason + effect within principal_cache_ttl (F052-AC-17, TC-AUTH-078)', async () => {
    const { backend } = await renderSchoolLms('/school-lms/access-control', { account: unitAdmin() });
    const staff = (await screen.findByText('Cán bộ B')).closest('tr')!;
    fireEvent.click(within(staff).getByRole('button', { name: 'Phân quyền' }));
    fireEvent.click(await screen.findByRole('button', { name: 'Thu hồi' }));
    const revoke = (await screen.findAllByRole('dialog')).at(-1)!;
    expect(within(revoke).getByText('Thay đổi có hiệu lực với người dùng trong vòng 60 giây.')).toBeInTheDocument();
    fireEvent.change(within(revoke).getByLabelText('Lý do'), { target: { value: 'Hết phân công' } });
    fireEvent.click(within(revoke).getByRole('button', { name: 'Thu hồi' }));
    await waitFor(() => expect(backend.calls('DELETE', `/tenant/user-roles/${IDS.userRole1}`)).toHaveLength(1));
    expect(backend.calls('DELETE', `/tenant/user-roles/${IDS.userRole1}`)[0]!.search).toContain('reason=');
  });

  it('permission log: one query action_prefix=role&action_prefix=user_role, optional action narrows (F052-AC-18, OQ F052-2)', async () => {
    const { backend } = await renderSchoolLms('/school-lms/access-control', { account: unitAdmin() });
    fireEvent.click(await screen.findByRole('tab', { name: 'Nhật ký phân quyền' }));
    expect(await screen.findByText('Phân công tổ Toán')).toBeInTheDocument();
    expect(screen.getByText('Tạo role tổ trưởng')).toBeInTheDocument(); // role.* and user_role.* together
    expect(screen.queryByText('Đổi địa chỉ')).toBeNull(); // other actions are not in the log
    expect(screen.getByText('trace-audit-1')).toBeInTheDocument();
    expect(screen.getByTestId('audit-changes')).toHaveTextContent('academic_manager');
    const first = new URLSearchParams(backend.calls('GET', '/tenant/audit-logs')[0]!.search);
    expect(first.getAll('action_prefix')).toEqual(['role', 'user_role']);
    expect(first.get('action')).toBeNull();
    fireEvent.mouseDown(screen.getByRole('combobox', { name: 'Hành động' }));
    fireEvent.click(await screen.findByTitle('role.created'));
    await waitFor(() => expect(screen.queryByText('Phân công tổ Toán')).toBeNull());
    const narrowed = new URLSearchParams(backend.calls('GET', '/tenant/audit-logs').at(-1)!.search);
    expect(narrowed.get('action')).toBe('role.created');
    expect(narrowed.getAll('action_prefix')).toEqual(['role', 'user_role']);
    expect(await screen.findByText('Tạo role tổ trưởng')).toBeInTheDocument();
  });

  it('a read-only user has no write button (F052-AC-25, TC-CMS-RBAC-03)', async () => {
    await renderSchoolLms('/school-lms/access-control', { account: accountWith('qlcm.truong.a', TENANTS.thDemoA, READ_ONLY_PERMS) });
    expect(await screen.findByText('Cán bộ B')).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Thêm nhân sự' })).toBeNull();
    fireEvent.click(screen.getByRole('tab', { name: 'Role' }));
    expect(await screen.findByRole('button', { name: 'to_truong_toan' })).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Tạo role' })).toBeNull();
  });
});
