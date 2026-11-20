// F055 T036, T047 (US5) — F055-AC-18, AC-23 (TC-LRN-015): "Hỗ trợ thời gian" only for an assignment with an exam session
// and lms.assignment.write — students of the assignment, percent within ExamAccommodation, saved with
// putExamAccommodations (reason, Idempotency-Key, If-Match); "Chấm lại đề" only with learning_content.re_evaluate on a
// graded exam: shared RegradeDialog with scope exam_session, a question of the locked papers, reason 10–1000,
// affected attempts from the preview, Idempotency-Key, status followed; no manual score field. Backend = MOCK.
import { cleanup, fireEvent, screen, waitFor, within } from '@testing-library/react';
import { afterEach, describe, expect, it } from 'vitest';
import { adTruongA, LMS_IDS, LMS_PERMS, lmsAssignment, renderLms, stopPortals } from '../../../../test/lms-harness';

afterEach(() => {
  stopPortals();
  cleanup();
});

const seed = (over: Record<string, unknown>) => {
  const a = lmsAssignment({ mode: 'graded', exam_session_id: LMS_IDS.examSession, ...over });
  return { a, setup: (s: { assignments: Map<string, typeof a> }) => s.assignments.set(String(a.assignment_id), a) };
};

describe('Hỗ trợ thời gian (F055-AC-18)', () => {
  it('tab only with an exam session; +25 % for a student of the assignment, saved with reason, key and If-Match', async () => {
    const { a, setup } = seed({ status: 'open' });
    const { backend, lms } = await renderLms(`/school-lms/assignments/${a.assignment_id}`, { setup });
    fireEvent.click(await screen.findByRole('tab', { name: 'Hỗ trợ thời gian' }));
    const tab = await screen.findByTestId('accommodations');
    fireEvent.mouseDown(within(tab).getByRole('combobox', { name: 'Học sinh' }));
    fireEvent.click(await screen.findByTitle('Lê Bình'));
    expect((within(tab).getByRole('spinbutton', { name: 'Thời gian thêm (%)' }) as HTMLInputElement).getAttribute('aria-valuemax')).toBe('100');
    fireEvent.click(within(tab).getByRole('button', { name: 'Thêm' }));
    expect(within(tab).getByText('+25%')).toBeInTheDocument();
    fireEvent.click(within(tab).getByRole('button', { name: 'Lưu' }));
    const dialog = await screen.findByRole('dialog');
    fireEvent.change(within(dialog).getByLabelText('Lý do'), { target: { value: 'Học sinh cần hỗ trợ đọc' } });
    fireEvent.click(within(dialog).getByRole('button', { name: 'Xác nhận' }));
    await waitFor(() => expect(backend.calls('PUT', `/exams/sessions/${LMS_IDS.examSession}/accommodations`)).toHaveLength(1));
    const put = backend.calls('PUT', `/exams/sessions/${LMS_IDS.examSession}/accommodations`)[0]!;
    expect(put.headers['idempotency-key']).toBeTruthy();
    expect(put.headers['if-match']).toBe('W/"1"');
    expect(put.body).toEqual({ accommodations: [{ student_id: LMS_IDS.studentBinh, extra_time_percent: 25 }], reason: 'Học sinh cần hỗ trợ đọc' });
    await waitFor(() => expect(lms.session.version).toBe(2));
  });

  it('no tab without an exam session or without lms.assignment.write', async () => {
    const plain = lmsAssignment({});
    await renderLms(`/school-lms/assignments/${plain.assignment_id}`, { setup: (s) => s.assignments.set(String(plain.assignment_id), plain) });
    await screen.findByTestId('assignment-detail');
    expect(screen.queryByRole('tab', { name: 'Hỗ trợ thời gian' })).toBeNull();
    stopPortals();
    cleanup();
    const { a, setup } = seed({ status: 'open' });
    await renderLms(`/school-lms/assignments/${a.assignment_id}`, { setup, account: adTruongA(LMS_PERMS.filter((p) => p !== 'lms.assignment.write')) });
    await screen.findByTestId('assignment-detail');
    expect(screen.queryByRole('tab', { name: 'Hỗ trợ thời gian' })).toBeNull();
  });
});

describe('Chấm lại đề (F055-AC-23, TC-LRN-015)', () => {
  it('hidden without learning_content.re_evaluate; with it: scope exam_session, question of the papers, preview count, key, status', async () => {
    const { a, setup } = seed({ status: 'closed' });
    await renderLms(`/school-lms/assignments/${a.assignment_id}`, { setup });
    await screen.findByTestId('assignment-detail');
    expect(screen.queryByTestId('exam-regrade')).toBeNull();
    stopPortals();
    cleanup();
    // academic manager: workspaces C and B (the regrade action belongs to content_studio, ADMIN_PORTAL §2.3)
    const manager = adTruongA([...LMS_PERMS, 'learning_content.re_evaluate']);
    for (const fx of Object.values(manager.perms)) fx.workspaces = ['school_lms', 'content_studio'];
    const { backend } = await renderLms(`/school-lms/assignments/${a.assignment_id}`, { setup, account: manager });
    fireEvent.click(await screen.findByTestId('exam-regrade'));
    const dialog = await screen.findByTestId('regrade-dialog');
    expect(within(dialog).queryByRole('spinbutton')).toBeNull();
    fireEvent.mouseDown(within(dialog).getByRole('combobox', { name: 'Câu hỏi của đề cần chấm lại' }));
    fireEvent.click(await screen.findByTitle('Đề 1 · câu 1'));
    expect(within(dialog).queryByRole('radio', { name: 'Dùng đáp án mới' })).toBeNull();
    fireEvent.click(within(dialog).getByRole('radio', { name: 'Bỏ câu này' }));
    fireEvent.change(within(dialog).getByRole('textbox', { name: 'Lý do (10–1.000 ký tự)' }), { target: { value: 'Câu 1 có hai đáp án đúng' } });
    expect(await within(dialog).findByTestId('regrade-impact')).toHaveTextContent('12');
    const preview = backend.calls('POST', '/cms/learning/regrade-requests:preview').at(-1)!;
    expect(preview.body).toMatchObject({ scope: 'exam_session', exam_session_id: LMS_IDS.examSession, question_version_id: LMS_IDS.questionV1, action: 'drop_item' });
    fireEvent.click(screen.getByRole('button', { name: 'Xác nhận chấm lại' }));
    expect(await within(dialog).findByTestId('regrade-status')).toBeInTheDocument();
    const create = backend.calls('POST', '/cms/learning/regrade-requests');
    expect(create).toHaveLength(1);
    expect(create[0]!.headers['idempotency-key']).toBeTruthy();
    await waitFor(() => expect(backend.requests.some((r) => r.method === 'GET' && r.path === `/cms/learning/regrade-requests/${LMS_IDS.questionV1}`)).toBe(true));
  });
});
