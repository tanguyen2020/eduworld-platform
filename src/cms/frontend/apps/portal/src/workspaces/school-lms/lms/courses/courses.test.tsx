// F055 T028, T029, T032, T033 (US3, US4) — F055-AC-13…AC-17: course list + hide/show only with lms.course.write
// (If-Match from the row version, delta D1); learning path builder (published versions only, template read-only,
// node count limit) and manual unlock (reason + Idempotency-Key); study groups (members of the school's classes,
// StudyGroupInput limits, delete after confirmation); schedule month/week + create/edit/delete with note limit.
// Backend = MOCK.
import { cleanup, fireEvent, screen, waitFor, within } from '@testing-library/react';
import { afterEach, describe, expect, it } from 'vitest';
import { adTruongA, IDS, LMS_IDS, LMS_PERMS, renderLms, stopPortals } from '../../../../test/lms-harness';

afterEach(() => {
  stopPortals();
  cleanup();
});

describe('Khóa học và lộ trình (US3)', () => {
  it('course list; hide sends If-Match of the row; no hide/show without lms.course.write (F055-AC-13)', async () => {
    const { backend } = await renderLms('/school-lms/courses');
    const row = (await screen.findByText('3A · Tiếng Việt')).closest('tr')!;
    fireEvent.click(within(row).getByRole('button', { name: 'Ẩn' }));
    await waitFor(() => expect(backend.calls('PATCH', `/lms/courses/${LMS_IDS.courseViet}`)).toHaveLength(1));
    const patch = backend.calls('PATCH', `/lms/courses/${LMS_IDS.courseViet}`)[0]!;
    expect(patch.headers['if-match']).toBe('W/"4"');
    expect(patch.body).toEqual({ status: 'hidden' });
    expect(await within((await screen.findByText('3A · Tiếng Việt')).closest('tr')!).findByText('Đã ẩn')).toBeInTheDocument();
    stopPortals();
    cleanup();
    await renderLms('/school-lms/courses', { account: adTruongA(LMS_PERMS.filter((p) => p !== 'lms.course.write')) });
    expect(await screen.findByText('3A · Toán')).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /^(Ẩn|Hiện)$/ })).toBeNull();
  });

  it('path builder: template read-only; new path of published versions within the node limit; unlock needs a reason (F055-AC-14, AC-15)', async () => {
    const { backend } = await renderLms(`/school-lms/courses/${LMS_IDS.courseMath}`);
    const template = (await screen.findByText('Lộ trình mẫu KNTT')).closest('li')!;
    fireEvent.click(within(template).getByRole('button', { name: 'Xem' }));
    expect(await screen.findByTestId('template-readonly')).toBeInTheDocument();
    expect(within(screen.getByTestId('path-builder')).queryByRole('button', { name: 'Lưu' })).toBeNull();
    fireEvent.click(screen.getByRole('button', { name: 'Tạo lộ trình' }));
    const builder = await screen.findByTestId('path-builder');
    expect(within(builder).getByRole('button', { name: 'Lưu' })).toBeDisabled();
    fireEvent.change(within(builder).getByLabelText('Tên lộ trình'), { target: { value: 'Ôn tập học kỳ' } });
    fireEvent.click(within(builder).getByRole('button', { name: 'Thêm bước' }));
    const picker = await screen.findByTestId('content-picker');
    fireEvent.click(within(picker).getByRole('tab', { name: 'Activity' }));
    fireEvent.click(within((await within(picker).findByText('Quiz phép nhân')).closest('li')!).getByRole('button', { name: 'Chọn' }));
    expect(within(builder).getByText('1/200 bước')).toBeInTheDocument();
    fireEvent.click(within(builder).getByRole('button', { name: 'Lưu' }));
    await waitFor(() => expect(backend.calls('POST', `/lms/courses/${LMS_IDS.courseMath}/paths`)).toHaveLength(1));
    const post = backend.calls('POST', `/lms/courses/${LMS_IDS.courseMath}/paths`)[0]!;
    expect(post.headers['idempotency-key']).toBeTruthy();
    expect(post.body).toEqual({ title: 'Ôn tập học kỳ', unlock_mode: 'sequential', nodes: [{ position: 1, content_ref: { content_type: 'activity', content_version_id: LMS_IDS.activityQuizV } }] });

    const path = (await screen.findByText('Ôn chương 3')).closest('li')!;
    fireEvent.click(within(path).getByRole('button', { name: 'Sửa' }));
    const edit = await screen.findByTestId('path-builder');
    fireEvent.click(await within(edit).findByRole('button', { name: 'Mở khóa cho học sinh' }));
    const dialog = await screen.findByRole('dialog');
    fireEvent.mouseDown(within(dialog).getByRole('combobox', { name: 'Học sinh' }));
    fireEvent.click(await screen.findByTitle('Nguyễn An'));
    fireEvent.click(within(dialog).getByRole('button', { name: 'Mở khóa cho học sinh' }));
    expect(await within(dialog).findByText('Trường này bắt buộc.')).toBeInTheDocument();
    fireEvent.change(within(dialog).getByLabelText('Lý do'), { target: { value: 'Đã ôn riêng với cô' } });
    fireEvent.click(within(dialog).getByRole('button', { name: 'Mở khóa cho học sinh' }));
    await waitFor(() => expect(backend.requests.some((r) => r.path.endsWith(`/students/${IDS.studentAn}:unlock`))).toBe(true));
    const unlock = backend.requests.find((r) => r.path.endsWith(':unlock'))!;
    expect(unlock.headers['idempotency-key']).toBeTruthy();
    expect(unlock.body).toEqual({ reason: 'Đã ôn riêng với cô' });
  });
});

describe('Nhóm học tập và lịch (US4)', () => {
  it('study group: members from the class of the school, name limit, create with a key, delete after confirmation (F055-AC-16)', async () => {
    const { backend } = await renderLms('/school-lms/study-groups');
    expect(await screen.findByText('Ôn Nhân 3A')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Tạo nhóm' }));
    const dialog = await screen.findByRole('dialog');
    const name = within(dialog).getByLabelText('Tên nhóm') as HTMLInputElement;
    expect(name.maxLength).toBe(64);
    fireEvent.change(name, { target: { value: 'Nhóm Toán 3A' } });
    fireEvent.mouseDown(within(dialog).getByRole('combobox', { name: 'Lớp' }));
    fireEvent.click(await screen.findByTitle('3A'));
    fireEvent.click(within(dialog).getByRole('button', { name: 'Lưu' }));
    expect(await within(dialog).findByText('Chọn từ 1 đến 100 học sinh.')).toBeInTheDocument();
    fireEvent.mouseDown(within(dialog).getByRole('combobox', { name: 'Thành viên' }));
    fireEvent.click(await screen.findByTitle('Nguyễn An'));
    fireEvent.click(within(dialog).getByRole('button', { name: 'Lưu' }));
    await waitFor(() => expect(backend.calls('POST', `/lms/schools/${IDS.schoolA}/study-groups`)).toHaveLength(1));
    const post = backend.calls('POST', `/lms/schools/${IDS.schoolA}/study-groups`)[0]!;
    expect(post.body).toEqual({ name: 'Nhóm Toán 3A', class_id: IDS.class3A, member_ids: [IDS.studentAn] });
    expect(post.headers['idempotency-key']).toBeTruthy();
    expect(backend.requests.filter((r) => r.path.includes('/classes/')).every((r) => r.path.startsWith(`/tenant/schools/${IDS.schoolA}/`))).toBe(true);
    const row = (await screen.findByText('Ôn Nhân 3A')).closest('tr')!;
    fireEvent.click(within(row).getByRole('button', { name: 'Xóa' }));
    expect(backend.calls('DELETE', `/lms/schools/${IDS.schoolA}/study-groups/${LMS_IDS.group}`)).toHaveLength(0);
    await waitFor(() => expect(screen.getAllByRole('button', { name: 'Xóa' }).length).toBeGreaterThan(2));
    fireEvent.click(screen.getAllByRole('button', { name: 'Xóa' }).at(-1)!);
    await waitFor(() => expect(backend.calls('DELETE', `/lms/schools/${IDS.schoolA}/study-groups/${LMS_IDS.group}`)).toHaveLength(1));
  });

  it('schedule: month and week views from listScheduleItems; create, edit (If-Match of the row) and delete; note limit (F055-AC-17)', async () => {
    const { backend } = await renderLms('/school-lms/schedules');
    expect(await screen.findByText('Kiểm tra · Kiểm tra 15 phút')).toBeInTheDocument();
    const q = new URLSearchParams(backend.calls('GET', '/lms/schedules')[0]!.search);
    expect(q.get('from')).toMatch(/^\d{4}-\d\d-01$/);
    fireEvent.click(screen.getByText('Tuần'));
    expect(await screen.findByTestId('schedule-week')).toBeInTheDocument();
    fireEvent.click(await screen.findByText('Kiểm tra · Kiểm tra 15 phút'));
    const dialog = await screen.findByRole('dialog');
    const note = within(dialog).getByLabelText('Ghi chú') as HTMLTextAreaElement;
    expect(note.maxLength).toBe(500);
    fireEvent.change(note, { target: { value: 'Kiểm tra 15 phút chương 3' } });
    fireEvent.click(within(dialog).getByRole('button', { name: 'Lưu' }));
    await waitFor(() => expect(backend.calls('PATCH', `/lms/schedules/${LMS_IDS.scheduleItem}`)).toHaveLength(1));
    const patch = backend.calls('PATCH', `/lms/schedules/${LMS_IDS.scheduleItem}`)[0]!;
    expect(patch.headers['if-match']).toBe('W/"2"');
    expect((patch.body as Record<string, unknown>).note).toBe('Kiểm tra 15 phút chương 3');
    fireEvent.click(await screen.findByText('Kiểm tra · Kiểm tra 15 phút chương 3'));
    const d2 = await screen.findByRole('dialog');
    fireEvent.click(within(d2).getByRole('button', { name: 'Xóa' }));
    await waitFor(() => expect(screen.getAllByRole('button', { name: 'Xóa' }).length).toBeGreaterThan(1));
    fireEvent.click(screen.getAllByRole('button', { name: 'Xóa' }).at(-1)!);
    await waitFor(() => expect(backend.calls('DELETE', `/lms/schedules/${LMS_IDS.scheduleItem}`)).toHaveLength(1));
    fireEvent.click(screen.getByRole('button', { name: 'Thêm mục lịch' }));
    const d3 = await screen.findByRole('dialog');
    fireEvent.mouseDown(within(d3).getByRole('combobox', { name: 'Khóa học' }));
    fireEvent.click(await screen.findByTitle('3A · Toán'));
    fireEvent.click(within(d3).getByRole('button', { name: 'Lưu' }));
    await waitFor(() => expect(backend.calls('POST', '/lms/schedules')).toHaveLength(1));
    expect(backend.calls('POST', '/lms/schedules')[0]!.headers['idempotency-key']).toBeTruthy();
    expect(backend.calls('POST', '/lms/schedules')[0]!.body).toMatchObject({ course_id: LMS_IDS.courseMath, kind: 'lesson' });
  });
});
