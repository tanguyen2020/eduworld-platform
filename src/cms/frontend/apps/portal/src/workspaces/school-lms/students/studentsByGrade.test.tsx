// F052 T012, T013, T014, T058 rewritten for ADR-0095 (no class) — F052-AC-04, F052-AC-07, F052-AC-27, F052-AC-31
// (TC-TEN-001, TC-TEN-003, TC-AUTH-087): "Học sinh theo khối" searches the students of one school (interim
// listTenantUsers user_type=student until the school → grade → students list of WP-I1), read-only with the "SIS" tag
// and no identity/class action; no class list and no "học sinh chưa có lớp" tab; another tenant's data never shown;
// password reset (reason, Idempotency-Key reused on retry, password once + print, redacted notice, hidden without
// permission/when turned off, nothing stored); cards below 768 px. Backend = MOCK.
import { cleanup, fireEvent, screen, waitFor, within } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { accountWith, IDS, problem, READ_ONLY_PERMS, renderSchoolLms, SCHOOL_PERMS, stopPortals, TENANTS } from '../../../test/school-lms-harness';

afterEach(() => stopPortals());

const URL = '/school-lms/classes';

/** Searches the students of the (only, auto-selected) school by "An". */
async function searchStudents(q = 'An') {
  const box = await screen.findByRole('searchbox', { name: 'Tìm kiếm' });
  await waitFor(() => expect(within(screen.getByTestId('students-by-grade')).getByText('TH Demo A')).toBeInTheDocument());
  fireEvent.change(box, { target: { value: q } });
  fireEvent.keyDown(box, { key: 'Enter', code: 'Enter', keyCode: 13 });
}

describe('Học sinh theo khối (ADR-0095)', () => {
  it('students of the school: search ≥ 2 characters, read-only with the SIS tag, no edit/class action (F052-AC-04)', async () => {
    const { backend } = await renderSchoolLms(URL);
    expect(await screen.findByText('Gõ tên hoặc tên đăng nhập (ít nhất 2 ký tự) để tìm học sinh của trường.')).toBeInTheDocument();
    expect(backend.calls('GET', '/tenant/users')).toHaveLength(0);
    await searchStudents();
    expect(await screen.findByText('Nguyễn An')).toBeInTheDocument();
    expect(screen.queryByText('Cán bộ B')).toBeNull();
    expect(screen.getAllByTestId('sis-tag').length).toBeGreaterThan(0);
    const q = new URLSearchParams(backend.calls('GET', '/tenant/users').at(-1)!.search);
    expect([q.get('user_type'), q.get('school_id'), q.get('q')]).toEqual(['student', IDS.schoolA, 'An']);
    expect(screen.queryByRole('button', { name: /Sửa|Gán lớp|Chuyển lớp|Xóa/ })).toBeNull();
  });

  it('no class list, class page or "học sinh chưa có lớp" tab; no class request (ADR-0095)', async () => {
    const { backend } = await renderSchoolLms(`${URL}/${IDS.schoolA}/${IDS.class3A}`);
    await screen.findByTestId('students-by-grade');
    expect(screen.queryByRole('tab', { name: /lớp/i })).toBeNull();
    expect(document.body.textContent).not.toMatch(/\bLớp\b|3A|chưa có lớp/);
    expect(backend.requests.some((r) => /\/classes|students\/unassigned/.test(r.path))).toBe(false);
  });

  it("another tenant's session never shows the data of tenant A (F052-AC-07, TC-TEN-001/003)", async () => {
    const { backend } = await renderSchoolLms(URL, { account: accountWith('ad.truong.b', TENANTS.thcsDemoB, SCHOOL_PERMS) });
    await waitFor(() => expect(within(screen.getByTestId('students-by-grade')).getByText('THCS Demo B')).toBeInTheDocument());
    const box = screen.getByRole('searchbox', { name: 'Tìm kiếm' });
    fireEvent.change(box, { target: { value: 'An' } });
    fireEvent.keyDown(box, { key: 'Enter', code: 'Enter', keyCode: 13 });
    await waitFor(() => expect(backend.calls('GET', '/tenant/users').length).toBeGreaterThan(0));
    expect(backend.calls('GET', '/tenant/users').every((r) => r.headers['x-tenant-id'] === TENANTS.thcsDemoB.tenant_id)).toBe(true);
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
    await renderSchoolLms(URL, { disabledActions: ['action.school_lms.classes.reset_student_password'] });
    await searchStudents();
    expect(await screen.findByText('Nguyễn An')).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Đặt lại mật khẩu' })).toBeNull();
  });
});
