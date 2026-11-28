// F136 T033 (portal part) — F136-AC-22 (TC-GAM-014): the "Học sinh theo khối" page has an "Đặt lại nickname" action
// (ActionGate action.school_lms.students.reset_nickname, display only — the backend decides with lms.student.nickname_reset)
// opening a dialog that names the student and explains the effect, a required reason (3–500) and a confirmation →
// resetStudentNickname with X-Tenant-Id and one Idempotency-Key per confirmation (reused on retry); success message;
// error shown by code. Backend = MOCK (test/school-lms-mock.ts, test only, AGENTS R17).
import { cleanup, fireEvent, screen, waitFor, within } from '@testing-library/react';
import { afterEach, describe, expect, it } from 'vitest';
import { accountWith, IDS, problem, renderSchoolLms, SCHOOL_PERMS, stopPortals, TENANTS } from '../../../test/school-lms-harness';

afterEach(() => stopPortals());

const URL = '/school-lms/students';
const resetPath = `/tenant/students/${IDS.studentAn}/game-profile/nickname:reset`;

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

async function openDialog() {
  const row = (await screen.findByText('Nguyễn An')).closest('tr')!;
  fireEvent.click(within(row).getByRole('button', { name: 'Đặt lại nickname' }));
  return screen.findByRole('dialog', { name: 'Đặt lại nickname của Nguyễn An' });
}

describe('Đặt lại nickname học sinh (F136-AC-22, TC-GAM-014)', () => {
  it('reason required (3–500); X-Tenant-Id + Idempotency-Key reused on retry; body {reason}; success message', async () => {
    const { backend } = await renderSchoolLms(URL);
    await searchStudents();
    let fail = true;
    backend.overrides.unshift((r) => (r.path === resetPath && fail ? ((fail = false), problem(409, 'CONFLICT')) : undefined));
    const dialog = await openDialog();
    // the effect is explained (the list has no nickname: the student is named, the nickname is not shown)
    expect(within(dialog).getByText(/chọn tên mới/)).toBeInTheDocument();
    const reason = within(dialog).getByLabelText('Lý do');
    expect(reason).toHaveAttribute('maxlength', '500');
    fireEvent.click(within(dialog).getByRole('button', { name: 'Đặt lại' }));
    await waitFor(() => expect(within(dialog).getByText('Trường này bắt buộc.')).toBeInTheDocument());
    fireEvent.change(reason, { target: { value: 'ab' } });
    fireEvent.click(within(dialog).getByRole('button', { name: 'Đặt lại' }));
    await waitFor(() => expect(within(dialog).getByText('Tối thiểu 3 ký tự.')).toBeInTheDocument());
    expect(backend.calls('POST', resetPath)).toHaveLength(0);
    fireEvent.change(reason, { target: { value: 'Nickname không phù hợp' } });
    fireEvent.click(within(dialog).getByRole('button', { name: 'Đặt lại' }));
    await waitFor(() => expect(backend.calls('POST', resetPath)).toHaveLength(1));
    expect(await within(dialog).findByRole('alert')).toBeInTheDocument(); // 409 → error by code (not retried automatically), dialog stays open
    fireEvent.click(within(dialog).getByRole('button', { name: 'Đặt lại' }));
    await waitFor(() => expect(screen.getByText('Đã đặt lại nickname của Nguyễn An. Học sinh sẽ chọn tên mới khi vào game.')).toBeInTheDocument());
    const calls = backend.calls('POST', resetPath);
    expect(calls).toHaveLength(2);
    expect(calls[0]!.headers['idempotency-key']).toMatch(/^[0-9a-f-]{36}$/);
    expect(calls[0]!.headers['idempotency-key']).toBe(calls[1]!.headers['idempotency-key']);
    expect(calls.every((c) => c.headers['x-tenant-id'] === TENANTS.thDemoA.tenant_id)).toBe(true);
    expect(calls[1]!.body).toEqual({ reason: 'Nickname không phù hợp' });
    await waitFor(() => expect(screen.queryByRole('dialog', { name: 'Đặt lại nickname của Nguyễn An' })).toBeNull());
  });

  it('button hidden without lms.student.nickname_reset or when the action is turned off', async () => {
    await renderSchoolLms(URL, { account: accountWith('qlcm.truong.a', TENANTS.thDemoA, SCHOOL_PERMS.filter((p) => p !== 'lms.student.nickname_reset')) });
    await searchStudents();
    expect(await screen.findByText('Nguyễn An')).toBeInTheDocument();
    expect(screen.getAllByRole('button', { name: 'Đặt lại mật khẩu' }).length).toBeGreaterThan(0);
    expect(screen.queryByRole('button', { name: 'Đặt lại nickname' })).toBeNull();
    stopPortals();
    cleanup();
    await renderSchoolLms(URL, { disabledActions: ['action.school_lms.students.reset_nickname'] });
    await searchStudents();
    expect(await screen.findByText('Nguyễn An')).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Đặt lại nickname' })).toBeNull();
  });
});
