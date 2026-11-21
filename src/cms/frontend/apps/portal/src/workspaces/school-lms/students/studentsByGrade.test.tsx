// F052 T012, T013, T014, T058 rewritten for ADR-0095 (no class) — F052-AC-04, F052-AC-07, F052-AC-27, F052-AC-31
// (TC-TEN-001, TC-TEN-003, TC-AUTH-087): "Học sinh theo khối" = school → grade (grades the school teaches) → students
// (listTenantGradeStudents, cursor, search), read-only with the "SIS" tag and no identity/class action; no class list
// and no "học sinh chưa có lớp" tab; another tenant's school → "Không tìm thấy"; password reset (reason,
// Idempotency-Key reused on retry, password once + print, redacted notice, hidden without permission/when turned off,
// nothing stored); cards below 768 px. Backend = MOCK.
import { cleanup, fireEvent, screen, waitFor, within } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { accountWith, IDS, problem, READ_ONLY_PERMS, renderSchoolLms, SCHOOL_PERMS, stopPortals, TENANTS } from '../../../test/school-lms-harness';

afterEach(() => stopPortals());

const URL = '/school-lms/students';

/** Picks grade 3 of the (only, auto-selected) school and searches "An". */
async function searchStudents(q = 'An') {
  const page = await screen.findByTestId('students-by-grade');
  await waitFor(() => expect(within(page).getByText('TH Demo A')).toBeInTheDocument());
  fireEvent.mouseDown(within(page).getByRole('combobox', { name: 'Khối' }));
  const option = () => screen.queryAllByTitle('Khối 3').filter((e) => e.classList.contains('ant-select-item-option'));
  await waitFor(() => expect(option().length).toBeGreaterThan(0));
  fireEvent.click(option().at(-1)!);
  const box = within(page).getByRole('searchbox', { name: 'Tìm kiếm' });
  fireEvent.change(box, { target: { value: q } });
  fireEvent.keyDown(box, { key: 'Enter', code: 'Enter', keyCode: 13 });
}

const gradeCalls = (backend: { requests: { path: string; search: string; headers: Record<string, string> }[] }) =>
  backend.requests.filter((r) => /^\/tenant\/schools\/[^/]+\/grades\/\d+\/students$/.test(r.path));

describe('Học sinh theo khối (ADR-0095)', () => {
  it('school → grade → students (listTenantGradeStudents), read-only with the SIS tag, no edit/class action (F052-AC-04)', async () => {
    const { backend } = await renderSchoolLms(URL);
    const page = await screen.findByTestId('students-by-grade');
    // grades = grades the school teaches (TH Demo A: 1–5)
    fireEvent.mouseDown(within(page).getByRole('combobox', { name: 'Khối' }));
    expect(await screen.findByTitle('Khối 5')).toBeInTheDocument();
    expect(screen.queryByTitle('Khối 6')).toBeNull();
    fireEvent.mouseDown(within(page).getByRole('combobox', { name: 'Khối' }));
    await searchStudents();
    expect(await screen.findByText('Nguyễn An')).toBeInTheDocument();
    expect(screen.getByText('HS001')).toBeInTheDocument();
    expect(screen.getByText('2 ngày')).toBeInTheDocument();
    expect(screen.getAllByTestId('sis-tag').length).toBeGreaterThan(0);
    const last = gradeCalls(backend).at(-1)!;
    expect(last.path).toBe(`/tenant/schools/${IDS.schoolA}/grades/3/students`);
    expect(new URLSearchParams(last.search).get('q')).toBe('An');
    expect(screen.queryByRole('button', { name: /Sửa|Gán lớp|Chuyển lớp|Xóa/ })).toBeNull();
    expect(backend.requests.some((r) => r.path === '/tenant/users')).toBe(false);
  });

  it('no class list, class page or "học sinh chưa có lớp" tab; no class request (ADR-0095)', async () => {
    const { backend } = await renderSchoolLms(`/school-lms/classes/${IDS.schoolA}/${IDS.class3A}`);
    await waitFor(() => expect(backend.calls('GET', '/me/permissions').length).toBeGreaterThan(0));
    expect(await screen.findByText('Không tìm thấy trang.')).toBeInTheDocument();
    expect(screen.queryByRole('tab', { name: /lớp/i })).toBeNull();
    expect(backend.requests.some((r) => /\/classes|students\/unassigned/.test(r.path))).toBe(false);
  });

  it("another tenant's school → Không tìm thấy, nothing of it shown (F052-AC-07, TC-TEN-001/003)", async () => {
    const { backend } = await renderSchoolLms(URL, { account: accountWith('ad.truong.b', TENANTS.thcsDemoB, SCHOOL_PERMS) });
    const page = await screen.findByTestId('students-by-grade');
    await waitFor(() => expect(within(page).getByText('THCS Demo B')).toBeInTheDocument());
    await waitFor(() => expect(gradeCalls(backend).length).toBeGreaterThan(0));
    expect(gradeCalls(backend).every((r) => r.headers['x-tenant-id'] === TENANTS.thcsDemoB.tenant_id)).toBe(true);
    expect(screen.queryByText('Nguyễn An')).toBeNull();
  });

  it('below 768 px the list shows cards (F052-AC-27)', async () => {
    await renderSchoolLms(URL, { width: 390 });
    await searchStudents();
    expect(await screen.findByTestId('responsive-cards')).toBeInTheDocument();
    expect(await screen.findByText('Nguyễn An')).toBeInTheDocument();
  });
});

describe('Đặt lại mật khẩu học sinh (F052-AC-31, TC-AUTH-087)', () => {
  const resetPath = `/tenant/students/${IDS.studentAn}/password:reset`;

  it('reason required; retry reuses the Idempotency-Key; password shown once with print; nothing stored', async () => {
    const setItem = vi.spyOn(Storage.prototype, 'setItem');
    const { backend } = await renderSchoolLms(URL);
    await searchStudents();
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
    const { backend } = await renderSchoolLms(URL);
    await searchStudents();
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
    await renderSchoolLms(URL, { account: accountWith('qlcm.truong.a', TENANTS.thDemoA, [...READ_ONLY_PERMS, 'tenant.user.read']) });
    await searchStudents();
    expect(await screen.findByText('Nguyễn An')).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Đặt lại mật khẩu' })).toBeNull();
    stopPortals();
    cleanup();
    await renderSchoolLms(URL, { disabledActions: ['action.school_lms.students.reset_password'] });
    await searchStudents();
    expect(await screen.findByText('Nguyễn An')).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Đặt lại mật khẩu' })).toBeNull();
  });
});
