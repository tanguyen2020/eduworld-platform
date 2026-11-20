// F055 T020–T024 (US2) — F055-AC-08…AC-12 (TC-LMS-001): submission table columns (no XP / rating / manual comment),
// "làm quá nhanh"; extend / excuse with reason + Idempotency-Key, row updated from the response; remind 429 → "đã nhắc
// hôm nay" + locked; release only manual + closed + lms.result.release; cancel shows 409; edit of an open assignment
// limited to due_at / instructions / late_policy with If-Match, 412 → reload, nothing overwritten. Backend = MOCK.
import { cleanup, fireEvent, screen, waitFor, within } from '@testing-library/react';
import { afterEach, describe, expect, it } from 'vitest';
import { adTruongA, LMS_IDS, LMS_PERMS, lmsAssignment, problem, renderLms, stopPortals } from '../../../../test/lms-harness';

afterEach(() => {
  stopPortals();
  cleanup();
});

const seed = (over: Record<string, unknown> = {}) => {
  const a = lmsAssignment(over);
  return { a, setup: (s: { assignments: Map<string, typeof a> }) => s.assignments.set(String(a.assignment_id), a) };
};

describe('Theo dõi nộp bài', () => {
  it('table: student, status, score, attempts, submitted at, "làm quá nhanh"; no XP/rating/comment column (F055-AC-08)', async () => {
    const { a, setup } = seed();
    await renderLms(`/school-lms/assignments/${a.assignment_id}`, { setup });
    const table = await screen.findByTestId('submission-table');
    const an = (await within(table).findByText('Nguyễn An')).closest('tr')!;
    expect(within(an).getByText('Đã nộp')).toBeInTheDocument();
    expect(within(an).getByText('8/10')).toBeInTheDocument();
    expect(within(an).getByTestId('rapid-guess')).toHaveTextContent('Làm quá nhanh');
    expect(within(table).getAllByTestId('rapid-guess')).toHaveLength(1);
    expect(within(table).getByText('Bỏ lỡ')).toBeInTheDocument();
    const headers = within(table).getAllByRole('columnheader').map((h) => h.textContent);
    expect(headers).toEqual(expect.arrayContaining(['Học sinh', 'Trạng thái', 'Điểm tự chấm', 'Số lần làm', 'Nộp lúc']));
    expect(headers.join(' ')).not.toMatch(/XP|Rating|Nhận xét/i);
  });

  it('extend and excuse: reason required, Idempotency-Key reused on retry, row updated from the response (F055-AC-09)', async () => {
    const { a, setup } = seed();
    const { backend } = await renderLms(`/school-lms/assignments/${a.assignment_id}`, { setup });
    const excusePath = `/lms/assignments/${a.assignment_id}/students/${LMS_IDS.studentBinh}:excuse`;
    let fail = true;
    backend.overrides.unshift((r) => (r.path === excusePath && fail ? ((fail = false), problem(503, 'UNAVAILABLE')) : undefined));
    const binh = (await screen.findByText('Lê Bình')).closest('tr')!;
    fireEvent.click(within(binh).getByRole('button', { name: 'Miễn' }));
    const dialog = await screen.findByRole('dialog');
    fireEvent.click(within(dialog).getByRole('button', { name: 'Miễn' }));
    expect(await within(dialog).findByText('Trường này bắt buộc.')).toBeInTheDocument();
    expect(backend.calls('POST', excusePath)).toHaveLength(0);
    fireEvent.change(within(dialog).getByLabelText('Lý do'), { target: { value: 'Ốm nằm viện' } });
    fireEvent.click(within(dialog).getByRole('button', { name: 'Miễn' }));
    await waitFor(() => expect(backend.calls('POST', excusePath).length).toBeGreaterThanOrEqual(1));
    // first answer 503 (automatic retry of the apiClient or the user clicking again): same intent, same key
    await waitFor(() => expect(screen.queryByText('Được miễn') ?? within(dialog).queryByRole('alert')).toBeTruthy());
    if (!screen.queryByText('Được miễn')) fireEvent.click(within(dialog).getByRole('button', { name: 'Miễn' }));
    await waitFor(() => expect(within(screen.getByText('Lê Bình').closest('tr')!).getByText('Được miễn')).toBeInTheDocument());
    const keys = backend.calls('POST', excusePath).map((r) => r.headers['idempotency-key']);
    expect(new Set(keys).size).toBe(1);
    expect(backend.calls('POST', excusePath).at(-1)!.body).toEqual({ reason: 'Ốm nằm viện' });

    const missed = screen.getByText('Trần Sắp Bị Dọn').closest('tr')!;
    fireEvent.click(within(missed).getByRole('button', { name: 'Gia hạn' }));
    const d2 = await screen.findByRole('dialog');
    fireEvent.change(within(d2).getByLabelText('Lý do'), { target: { value: 'ab' } });
    fireEvent.click(within(d2).getByRole('button', { name: 'Gia hạn' }));
    expect(await within(d2).findByText('Tối thiểu 3 ký tự.')).toBeInTheDocument();
    fireEvent.change(within(d2).getByLabelText('Lý do'), { target: { value: 'Gia đình có việc' } });
    fireEvent.click(within(d2).getByRole('button', { name: 'Gia hạn' }));
    await waitFor(() => expect(backend.requests.some((r) => r.path.endsWith(':extend'))).toBe(true));
    const ext = backend.requests.find((r) => r.path.endsWith(':extend'))!;
    expect(ext.headers['idempotency-key']).toBeTruthy();
    expect((ext.body as Record<string, unknown>).reason).toBe('Gia đình có việc');
    expect((ext.body as Record<string, unknown>).due_at).toMatch(/^[0-9]{4}-[0-9]{2}-[0-9]{2}T/);
  });

  it('remind: 202 once, then 429 → "Bài này đã được nhắc hôm nay" and the button locked (F055-AC-10)', async () => {
    const { a, setup } = seed();
    const { backend } = await renderLms(`/school-lms/assignments/${a.assignment_id}`, { setup });
    const btn = await screen.findByRole('button', { name: 'Nhắc học sinh chưa làm' });
    fireEvent.click(btn);
    expect(await screen.findByText('Đã gửi nhắc cho 1 học sinh.')).toBeInTheDocument();
    fireEvent.click(btn);
    expect(await screen.findByTestId('remind-limited')).toHaveTextContent('Bài này đã được nhắc hôm nay.');
    await waitFor(() => expect(screen.getByRole('button', { name: 'Nhắc học sinh chưa làm' })).toBeDisabled());
    expect(backend.calls('POST', `/lms/assignments/${a.assignment_id}:remind`).every((r) => !!r.headers['idempotency-key'])).toBe(true);
  });
});

describe('Công bố, hủy, sửa', () => {
  it('release results only for manual + closed + lms.result.release (F055-AC-11)', async () => {
    const { a, setup } = seed({ status: 'closed', result_release: 'manual' });
    const { backend } = await renderLms(`/school-lms/assignments/${a.assignment_id}`, { setup });
    fireEvent.click(await screen.findByRole('button', { name: 'Công bố kết quả' }));
    await waitFor(() => expect(screen.getAllByRole('button', { name: 'Công bố kết quả' })).toHaveLength(2));
    fireEvent.click(screen.getAllByRole('button', { name: 'Công bố kết quả' }).at(-1)!);
    await waitFor(() => expect(backend.calls('POST', `/lms/assignments/${a.assignment_id}:release-results`)).toHaveLength(1));
    expect(backend.calls('POST', `/lms/assignments/${a.assignment_id}:release-results`)[0]!.headers['idempotency-key']).toBeTruthy();
    stopPortals();
    cleanup();
    const other = seed({ status: 'closed', result_release: 'after_close' });
    await renderLms(`/school-lms/assignments/${other.a.assignment_id}`, { setup: other.setup });
    await screen.findByTestId('assignment-detail');
    expect(screen.queryByRole('button', { name: 'Công bố kết quả' })).toBeNull();
    stopPortals();
    cleanup();
    const noPerm = seed({ status: 'closed', result_release: 'manual' });
    await renderLms(`/school-lms/assignments/${noPerm.a.assignment_id}`, { setup: noPerm.setup, account: adTruongA(LMS_PERMS.filter((p) => p !== 'lms.result.release')) });
    await screen.findByTestId('assignment-detail');
    expect(screen.queryByRole('button', { name: 'Công bố kết quả' })).toBeNull();
  });

  it('cancel with a reason; 409 CONFLICT when students already submitted is shown (F055-AC-11)', async () => {
    const { a, setup } = seed();
    const { backend } = await renderLms(`/school-lms/assignments/${a.assignment_id}`, { setup });
    fireEvent.click(await screen.findByRole('button', { name: 'Hủy bài' }));
    const dialog = await screen.findByRole('dialog');
    fireEvent.change(within(dialog).getByLabelText('Lý do'), { target: { value: 'Giao nhầm lớp' } });
    fireEvent.click(within(dialog).getByRole('button', { name: 'Hủy bài' }));
    expect(await screen.findByTestId('action-error')).toHaveTextContent('Không thể thực hiện vì dữ liệu đã thay đổi');
    expect(backend.calls('POST', `/lms/assignments/${a.assignment_id}:cancel`)[0]!.body).toEqual({ reason: 'Giao nhầm lớp' });
  });

  it('edit an open assignment: only due/instructions/late policy, If-Match sent; 412 → message + reload (F055-AC-12)', async () => {
    const { a, setup } = seed();
    const { backend, lms } = await renderLms(`/school-lms/assignments/${a.assignment_id}`, { setup });
    fireEvent.click(await screen.findByRole('button', { name: 'Sửa' }));
    const dialog = await screen.findByRole('dialog');
    expect(within(dialog).getByLabelText('Tiêu đề')).toBeDisabled();
    expect(within(dialog).getByLabelText('Hướng dẫn')).not.toBeDisabled();
    fireEvent.change(within(dialog).getByLabelText('Hướng dẫn'), { target: { value: 'Làm cẩn thận' } });
    lms.assignments.get(String(a.assignment_id))!.version = 7; // another tab saved meanwhile
    fireEvent.click(within(dialog).getByRole('button', { name: 'Lưu' }));
    expect(await within(dialog).findByTestId('edit-stale')).toHaveTextContent('Bài giao đã được người khác cập nhật');
    const patch = backend.calls('PATCH', `/lms/assignments/${a.assignment_id}`)[0]!;
    expect(patch.headers['if-match']).toBe('W/"1"');
    expect(patch.body).toEqual({ instructions: 'Làm cẩn thận' });
    await waitFor(() => expect(backend.calls('GET', `/lms/assignments/${a.assignment_id}`).length).toBeGreaterThan(1));
    expect(lms.assignments.get(String(a.assignment_id))!.instructions).toBeUndefined();
  });
});
