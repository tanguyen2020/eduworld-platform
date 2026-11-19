// F052 T012, T013, T014, T058 — F052-AC-04, F052-AC-05, F052-AC-07, F052-AC-27, F052-AC-31 (TC-SIS-103, TC-TEN-001,
// TC-TEN-003, TC-AUTH-087): class students read-only with the "SIS" tag and no identity/class action; unassigned
// students with guidance and no assign action; another tenant's class → "Không tìm thấy"; password reset (reason,
// Idempotency-Key reused on retry, password once + print, redacted notice, hidden without permission/when turned off,
// nothing stored); cards below 768 px. Backend = MOCK.
import { cleanup, fireEvent, screen, waitFor, within } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { accountWith, IDS, problem, READ_ONLY_PERMS, renderSchoolLms, SCHOOL_PERMS, stopPortals, TENANTS } from '../../../test/school-lms-harness';

afterEach(() => stopPortals());

const classUrl = `/school-lms/classes/${IDS.schoolA}/${IDS.class3A}`;

describe('Lớp & học sinh', () => {
  it('class students are read-only with the SIS tag, no edit/assign-class action (F052-AC-04)', async () => {
    await renderSchoolLms(classUrl);
    expect(await screen.findByText('Nguyễn An')).toBeInTheDocument();
    expect(screen.getAllByTestId('sis-tag').length).toBeGreaterThan(0);
    expect(screen.getByText('2 ngày')).toBeInTheDocument();
    expect(screen.getAllByText('Chưa vào game').length).toBeGreaterThan(0);
    expect(screen.queryByRole('button', { name: /Sửa|Gán lớp|Chuyển lớp|Xóa/ })).toBeNull();
  });

  it('class list links to the class with its head count (F052-AC-04)', async () => {
    await renderSchoolLms('/school-lms/classes');
    const link = await screen.findByText('3A', { selector: 'a' });
    expect(link.getAttribute('href')).toContain(`${IDS.schoolA}/${IDS.class3A}`);
  });

  it('unassigned students: guidance, no assign action (F052-AC-05, TC-SIS-103)', async () => {
    await renderSchoolLms('/school-lms/classes');
    fireEvent.click(await screen.findByRole('tab', { name: 'Học sinh chưa có lớp' }));
    expect(await screen.findByText('Lê Chưa Lớp')).toBeInTheDocument();
    const panel = screen.getByTestId('unassigned-students');
    expect(within(panel).getByText(/không gán lớp tạm/)).toBeInTheDocument();
    expect(within(panel).getByText('Thiếu lớp')).toBeInTheDocument();
    expect(within(panel).queryByRole('button', { name: /Gán/ })).toBeNull();
  });

  it("another tenant's class → Không tìm thấy, nothing of it shown (F052-AC-07, TC-TEN-001/003)", async () => {
    const { backend } = await renderSchoolLms(classUrl, { account: accountWith('ad.truong.b', TENANTS.thcsDemoB, SCHOOL_PERMS) });
    expect(await screen.findByTestId('school-lms-not-found')).toBeInTheDocument();
    expect(screen.queryByText('Nguyễn An')).toBeNull();
    expect(backend.calls('GET', `/tenant/schools/${IDS.schoolA}/classes/${IDS.class3A}/students`)[0]!.headers['x-tenant-id']).toBe(TENANTS.thcsDemoB.tenant_id);
  });

  it('below 768 px the list shows cards (F052-AC-27)', async () => {
    await renderSchoolLms(classUrl, { width: 390 });
    expect(await screen.findByTestId('responsive-cards')).toBeInTheDocument();
    expect(await screen.findByText('Nguyễn An')).toBeInTheDocument();
  });
});

describe('Đặt lại mật khẩu học sinh (F052-AC-31, TC-AUTH-087)', () => {
  const resetPath = `/tenant/students/${IDS.studentAn}/password:reset`;

  it('reason required; retry reuses the Idempotency-Key; password shown once with print; nothing stored', async () => {
    const setItem = vi.spyOn(Storage.prototype, 'setItem');
    const { backend } = await renderSchoolLms(classUrl);
    let fail = true;
    backend.overrides.unshift((r) => (r.path === resetPath && fail ? ((fail = false), problem(422, 'VALIDATION_FAILED')) : undefined));
    const row = (await screen.findByText('Nguyễn An')).closest('tr')!;
    fireEvent.click(within(row).getByRole('button', { name: 'Đặt lại mật khẩu' }));
    const dialog = await screen.findByRole('dialog');
    fireEvent.click(within(dialog).getByRole('button', { name: 'Đặt lại' }));
    expect(await within(dialog).findByText('Trường này bắt buộc.')).toBeInTheDocument();
    expect(backend.calls('POST', resetPath)).toHaveLength(0);
    fireEvent.change(within(dialog).getByLabelText('Lý do'), { target: { value: 'Học sinh quên mật khẩu' } });
    fireEvent.click(within(dialog).getByRole('button', { name: 'Đặt lại' }));
    await waitFor(() => expect(backend.calls('POST', resetPath)).toHaveLength(1));
    fireEvent.click(within(dialog).getByRole('button', { name: 'Đặt lại' }));
    expect(await within(dialog).findByTestId('initial-password')).toHaveTextContent('Mock-Pass-0001');
    const keys = backend.calls('POST', resetPath).map((r) => r.headers['idempotency-key']);
    expect(keys).toHaveLength(2);
    expect(keys[0]).toBe(keys[1]);
    expect(backend.calls('POST', resetPath)[1]!.body).toEqual({ reason: 'Học sinh quên mật khẩu' });
    expect(within(dialog).getByRole('button', { name: 'In thẻ đăng nhập' })).toBeInTheDocument();
    for (const [, v] of setItem.mock.calls) expect(String(v)).not.toContain('Mock-Pass-0001');
    fireEvent.click(within(dialog).getByRole('button', { name: 'Đóng' }));
    await waitFor(() => expect(screen.queryByTestId('initial-password')).toBeNull());
    setItem.mockRestore();
  });

  it('password_redacted → cannot be shown again, suggest another reset', async () => {
    const { backend } = await renderSchoolLms(classUrl);
    backend.overrides.unshift((r) =>
      r.path === resetPath ? new Response(JSON.stringify({ student_id: IDS.studentAn, username: 'hs.an.3a', password_temporary: true, password_redacted: true, reset_at: '2026-11-19T08:00:00Z' }), { status: 200, headers: { 'Content-Type': 'application/json' } }) : undefined,
    );
    const row = (await screen.findByText('Nguyễn An')).closest('tr')!;
    fireEvent.click(within(row).getByRole('button', { name: 'Đặt lại mật khẩu' }));
    const dialog = await screen.findByRole('dialog');
    fireEvent.change(within(dialog).getByLabelText('Lý do'), { target: { value: 'Thử lại lần hai' } });
    fireEvent.click(within(dialog).getByRole('button', { name: 'Đặt lại' }));
    expect(await within(dialog).findByText(/không xem lại được/)).toBeInTheDocument();
    expect(within(dialog).queryByTestId('initial-password')).toBeNull();
  });

  it('button hidden without lms.student.password_reset or when the action is turned off', async () => {
    await renderSchoolLms(classUrl, { account: accountWith('qlcm.truong.a', TENANTS.thDemoA, READ_ONLY_PERMS) });
    expect(await screen.findByText('Nguyễn An')).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Đặt lại mật khẩu' })).toBeNull();
    stopPortals();
    cleanup();
    await renderSchoolLms(classUrl, { disabledActions: ['action.school_lms.classes.reset_student_password'] });
    expect(await screen.findByText('Nguyễn An')).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Đặt lại mật khẩu' })).toBeNull();
  });
});
