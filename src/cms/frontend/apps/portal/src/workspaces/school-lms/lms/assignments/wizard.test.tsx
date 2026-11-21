// F055 T009–T014 (US1) — F055-AC-01…AC-07, AC-22 (TC-LMS-001, TC-CMS-ASG-01, TC-CMS-TEN-03): content step lists only
// published content of a type enabled in the Release + quests from listGameActivities; targets by grade of a school
// (ADR-0095): school, subject, grades the school teaches, students of other grades added, students left out (exclude
// wins), or a list of students — options only from the API (scope); graded disables follow_latest and sends no release
// defaults; limits from the contract; "Giao bài" =
// createAssignment + publishAssignment with If-Match, one Idempotency-Key per call kept on retry and double click;
// server errors mapped to fields / code messages with trace id; System Admin bound to one target tenant. Backend = MOCK.
import { act, cleanup, fireEvent, screen, waitFor, within } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { tenantFilterStore } from '../../../../shell/tenantFilterStore';
import { IDS, LMS_IDS, problem, renderLms, stopPortals, TENANTS } from '../../../../test/lms-harness';
import { OTHER_SCHOOL_STUDENT } from '../../../../test/lms-mock';

// the full wizard (5 steps, several antd selects) is slow in jsdom on a loaded machine
vi.setConfig({ testTimeout: 480_000 });

afterEach(() => {
  stopPortals();
  cleanup();
  tenantFilterStore.set([]);
});

const URL = '/school-lms/assignments/new';

async function pickOption(label: string, option: string) {
  fireEvent.mouseDown(screen.getByRole('combobox', { name: label }));
  fireEvent.click(await screen.findByTitle(option));
}

function typeDate(label: string, value: string) {
  const input = screen.getByRole('textbox', { name: label }) as HTMLInputElement;
  fireEvent.mouseDown(input);
  fireEvent.change(input, { target: { value } });
  fireEvent.keyDown(input, { key: 'Enter', code: 'Enter' });
  fireEvent.blur(input);
}

const next = () => fireEvent.click(screen.getByRole('button', { name: 'Tiếp' }));

async function pickContent() {
  const picker = await screen.findByTestId('content-picker');
  fireEvent.click(within((await within(picker).findByText('Quiz phép nhân')).closest('li')!).getByRole('button', { name: 'Chọn' }));
  next();
}

/** Step "Đối tượng": the only school of the scope is picked automatically; subject Toán and grade 3. */
async function pickTargets() {
  await waitFor(() => expect(within(screen.getByRole('combobox', { name: 'Trường' }).closest('.ant-select')!).getByText('TH Demo A')).toBeInTheDocument());
  await pickOption('Môn học', 'Toán');
  await pickOption('Khối được giao', 'Khối 3');
}

/** Types in a student picker (search ≥ 2 characters) and picks the option. */
async function pickStudent(label: string, search: string, option: string) {
  const box = screen.getByRole('combobox', { name: label });
  fireEvent.mouseDown(box);
  fireEvent.change(box, { target: { value: search } });
  // the dropdown option (a chip of the same title may already show it in another picker)
  await waitFor(() => expect(screen.getAllByTitle(option).some((e) => e.classList.contains('ant-select-item-option'))).toBe(true));
  fireEvent.click(screen.getAllByTitle(option).filter((e) => e.classList.contains('ant-select-item-option')).at(-1)!);
}

/** Steps 1–4 with valid data; ends on the review step. */
async function fillWizard(opts: { graded?: boolean } = {}) {
  await pickContent();
  await pickTargets();
  next();
  fireEvent.change(await screen.findByLabelText('Tiêu đề'), { target: { value: 'Ôn tập chương 3' } });
  typeDate('Mở bài lúc', '2026-12-01 08:00:00');
  typeDate('Hạn nộp', '2026-12-05 17:00:00');
  next();
  await screen.findByText('Cách tính điểm');
  if (opts.graded) fireEvent.click(screen.getByRole('radio', { name: 'Kiểm tra (tính điểm)' }));
  next();
  await screen.findByTestId('wizard-review');
}

describe('Wizard giao bài — nội dung và đối tượng', () => {
  it('content: only published content of enabled types, quests from the game catalog, no written response (F055-AC-01, AC-22)', async () => {
    const { backend } = await renderLms(URL);
    const picker = await screen.findByTestId('content-picker');
    expect(await within(picker).findByText('Quiz phép nhân')).toBeInTheDocument();
    expect(within(picker).queryByText('Quiz nháp')).toBeNull();
    expect(within(picker).queryByText('Mô phỏng ngoài Release')).toBeNull();
    expect(backend.calls('GET', '/cms/learning/activities')[0]!.search).toContain('status=published');
    fireEvent.click(within(picker).getByRole('tab', { name: 'Hoạt động game' }));
    expect(await within(picker).findByText('Quest Rừng Số')).toBeInTheDocument();
    expect(backend.calls('GET', '/lms/catalog/game-activities')).toHaveLength(1);
    expect(screen.queryByText(/written_response|speaking|project|Tự luận/i)).toBeNull();
    expect(screen.queryByRole('button', { name: /Cấu hình|Sửa quest/ })).toBeNull();
  });

  it('targets: school of the scope, its grades (1–9 ∩ grade_min…grade_max), subjects and students from the API of the session tenant; no class, group or course (F055-AC-02, ADR-0095)', async () => {
    const { backend } = await renderLms(URL);
    await pickContent();
    await pickTargets();
    expect(screen.queryByRole('radio', { name: /Lớp|Nhóm học tập/ })).toBeNull();
    expect(screen.queryByRole('combobox', { name: 'Khóa học' })).toBeNull();
    fireEvent.mouseDown(screen.getByRole('combobox', { name: 'Khối được giao' }));
    expect(await screen.findByTitle('Khối 5')).toBeInTheDocument();
    expect(screen.queryByTitle('Khối 6')).toBeNull();
    // the student search needs 2 characters, then lists students of the school only
    const include = screen.getByRole('combobox', { name: 'Thêm học sinh khối khác (cùng trường)' });
    fireEvent.mouseDown(include);
    fireEvent.change(include, { target: { value: 'C' } });
    expect(backend.calls('GET', '/tenant/users')).toHaveLength(0);
    await pickStudent('Thêm học sinh khối khác (cùng trường)', 'Cường', 'Phạm Cường (hs.cuong.4b)');
    fireEvent.click(screen.getByRole('radio', { name: 'Học sinh cụ thể' }));
    expect(screen.queryByRole('combobox', { name: 'Khối được giao' })).toBeNull();
    await pickStudent('Học sinh', 'An', 'Nguyễn An (hs.an.3a)');
    for (const path of ['/tenant/schools', '/learning/subjects', '/tenant/users']) {
      const calls = backend.calls('GET', path);
      expect(calls.length, path).toBeGreaterThan(0);
      expect(calls.every((r) => r.headers['x-tenant-id'] === TENANTS.thDemoA.tenant_id && !r.search.includes('tenant_ids')), path).toBe(true);
    }
    const q = new URLSearchParams(backend.calls('GET', '/tenant/users').at(-1)!.search);
    expect([q.get('user_type'), q.get('school_id'), q.get('q')]).toEqual(['student', IDS.schoolA, 'An']);
    expect(backend.requests.some((r) => /\/courses|\/classes|study-groups/.test(r.path))).toBe(false);
  });

  it('grades ± students: one request per intent with grade targets, include on the first grade, exclude on every grade; exclude wins hint (ADR-0095)', async () => {
    const { backend, lms } = await renderLms(URL);
    await pickContent();
    await pickTargets();
    await pickOption('Khối được giao', 'Khối 4');
    await pickStudent('Thêm học sinh khối khác (cùng trường)', 'Cường', 'Phạm Cường (hs.cuong.4b)');
    await pickStudent('Bớt học sinh', 'Bình', 'Lê Bình (hs.binh.3a)');
    expect(screen.queryByTestId('exclude-wins')).toBeNull();
    expect(screen.getByText(/Học sinh mới vào khối sau khi giao được tự động thêm/)).toBeInTheDocument();
    await pickStudent('Bớt học sinh', 'Cường', 'Phạm Cường (hs.cuong.4b)');
    expect(screen.getByTestId('exclude-wins')).toHaveTextContent('1 học sinh vừa được thêm vừa bị bớt');
    next();
    fireEvent.change(await screen.findByLabelText('Tiêu đề'), { target: { value: 'Ôn tập khối' } });
    typeDate('Mở bài lúc', '2026-12-01 08:00:00');
    typeDate('Hạn nộp', '2026-12-05 17:00:00');
    next();
    await screen.findByText('Cách tính điểm');
    next();
    const review = await screen.findByTestId('wizard-review');
    expect(review).toHaveTextContent('Khối 3, Khối 4 · thêm 1 học sinh · bớt 2 học sinh');
    fireEvent.click(screen.getByRole('button', { name: 'Giao bài' }));
    expect(await screen.findByTestId('assignment-detail')).toBeInTheDocument();
    const creates = backend.calls('POST', '/lms/assignments');
    expect(creates).toHaveLength(1);
    expect(creates[0]!.headers['idempotency-key']).toMatch(/^[0-9a-f-]{36}:create$|^[0-9a-f-]{36}$/);
    expect(lms.assignments.size).toBe(1);
    const body = creates[0]!.body as Record<string, unknown>;
    expect(body).toMatchObject({ school_id: IDS.schoolA, subject_id: IDS.subjectMath });
    expect(body).not.toHaveProperty('course_id');
    expect(body.targets).toEqual([
      { target_type: 'grade', grade_level: 3, include_student_ids: [LMS_IDS.studentCuong4], exclude_student_ids: [LMS_IDS.studentBinh, LMS_IDS.studentCuong4] },
      { target_type: 'grade', grade_level: 4, exclude_student_ids: [LMS_IDS.studentBinh, LMS_IDS.studentCuong4] },
    ]);
  });

  it('a student of another school → 422 at the include list, back to the "Đối tượng" step (F055-AC-06, ADR-0095)', async () => {
    await renderLms(URL);
    await pickContent();
    await pickTargets();
    await pickStudent('Thêm học sinh khối khác (cùng trường)', 'Khác', 'Đỗ Khác Trường (hs.khac.truong)');
    next();
    fireEvent.change(await screen.findByLabelText('Tiêu đề'), { target: { value: 'Ôn tập' } });
    typeDate('Mở bài lúc', '2026-12-01 08:00:00');
    typeDate('Hạn nộp', '2026-12-05 17:00:00');
    next();
    await screen.findByText('Cách tính điểm');
    next();
    await screen.findByTestId('wizard-review');
    fireEvent.click(screen.getByRole('button', { name: 'Giao bài' }));
    const include = await screen.findByRole('combobox', { name: 'Thêm học sinh khối khác (cùng trường)' });
    await waitFor(() => expect(include.closest('.ant-form-item')).toHaveClass('ant-form-item-has-error'));
    expect(OTHER_SCHOOL_STUDENT).toBeTruthy();
  });

  it('prefill from the dashboard: ?school_id&subject_id&grade → grade target; ?student_ids → students target; no class parameter is read', async () => {
    await renderLms(`${URL}?mode=practice&school_id=${IDS.schoolA}&subject_id=${IDS.subjectMath}&grade=3&class_id=x`);
    await pickContent();
    expect(await screen.findByRole('radio', { name: 'Theo khối' })).toBeChecked();
    expect(await within(screen.getByTestId('grade-select')).findByText('Khối 3')).toBeInTheDocument();
    expect(await within(screen.getByRole('combobox', { name: 'Môn học' }).closest('.ant-select')!).findByText('Toán')).toBeInTheDocument();
    stopPortals();
    cleanup();
    await renderLms(`${URL}?student_ids=${IDS.studentAn},${LMS_IDS.studentBinh}`);
    await pickContent();
    expect(await screen.findByRole('radio', { name: 'Học sinh cụ thể' })).toBeChecked();
    expect(within(screen.getByTestId('pick-studentIds')).getAllByText('Học sinh đã chọn sẵn')).toHaveLength(2);
  });

  it('student search without tenant.user.read (403) → notice, grade targets still possible (interim search, ADR-0095 gap)', async () => {
    const { backend } = await renderLms(URL);
    backend.overrides.unshift((r) => (r.path === '/tenant/users' ? problem(403, 'PERMISSION_DENIED') : undefined));
    await pickContent();
    await pickTargets();
    const include = screen.getByRole('combobox', { name: 'Thêm học sinh khối khác (cùng trường)' });
    fireEvent.mouseDown(include);
    fireEvent.change(include, { target: { value: 'An' } });
    expect(await screen.findByTestId('pick-includeIds-unavailable')).toHaveTextContent('Chưa tra cứu được học sinh ở đây');
    next();
    expect(await screen.findByLabelText('Tiêu đề')).toBeInTheDocument();
  });
});

describe('Wizard giao bài — chính sách, giới hạn, gửi', () => {
  it('limits before sending: counter + maxLength on title, due must be after open (F055-AC-04)', async () => {
    await renderLms(URL);
    await pickContent();
    // the targets step does not let the user go on without subject and grade
    await waitFor(() => expect(within(screen.getByRole('combobox', { name: 'Trường' }).closest('.ant-select')!).getByText('TH Demo A')).toBeInTheDocument());
    next();
    expect(await screen.findAllByText('Trường này bắt buộc.')).toHaveLength(2);
    await pickTargets();
    next();
    const title = (await screen.findByLabelText('Tiêu đề')) as HTMLInputElement;
    expect(title.maxLength).toBe(120);
    fireEvent.change(title, { target: { value: 'Ôn tập' } });
    expect(screen.getByText('6 / 120')).toBeInTheDocument();
    typeDate('Mở bài lúc', '2026-12-05 08:00:00');
    typeDate('Hạn nộp', '2026-12-01 08:00:00');
    next();
    expect(await screen.findByText('Hạn nộp phải sau thời điểm mở bài.')).toBeInTheDocument();
    expect(screen.queryByText('Cách tính điểm')).toBeNull();
  });

  it('graded: follow_latest disabled, no feedback_timing/result_release sent; create + publish with If-Match and one key each, double click = one assignment (F055-AC-03, AC-05)', async () => {
    const { backend, lms } = await renderLms(URL);
    await fillWizard({ graded: true });
    expect(screen.queryByText('Có')).toBeNull();
    const assign = screen.getByRole('button', { name: 'Giao bài' });
    act(() => {
      fireEvent.click(assign);
      fireEvent.click(assign);
    });
    expect(await screen.findByTestId('assignment-detail')).toBeInTheDocument();
    const creates = backend.calls('POST', '/lms/assignments');
    expect(new Set(creates.map((r) => r.headers['idempotency-key'])).size).toBe(1);
    expect(lms.assignments.size).toBe(1);
    const body = creates[0]!.body as Record<string, unknown>;
    expect(body).not.toHaveProperty('feedback_timing');
    expect(body).not.toHaveProperty('result_release');
    expect(body).not.toHaveProperty('tenant_id');
    expect(body).toMatchObject({ mode: 'graded', follow_latest: false, school_id: IDS.schoolA, subject_id: IDS.subjectMath, content_ref: { content_type: 'activity', content_id: LMS_IDS.activityQuiz, pinned_version_id: LMS_IDS.activityQuizV } });
    expect(body).not.toHaveProperty('course_id');
    expect(body.targets).toEqual([{ target_type: 'grade', grade_level: 3 }]);
    const id = [...lms.assignments.keys()][0]!;
    const publish = backend.calls('POST', `/lms/assignments/${id}:publish`);
    expect(publish.length).toBeGreaterThan(0);
    expect(publish[0]!.headers['if-match']).toBe('W/"1"');
    expect(publish[0]!.headers['idempotency-key']).toMatch(/^[0-9a-f-]{36}$/);
    expect(publish[0]!.headers['idempotency-key']).not.toBe(creates[0]!.headers['idempotency-key']);
  });

  it('graded mode disables the follow_latest option (F055-AC-03)', async () => {
    await renderLms(URL);
    await pickContent();
    await pickTargets();
    next();
    fireEvent.change(await screen.findByLabelText('Tiêu đề'), { target: { value: 'Bài' } });
    typeDate('Mở bài lúc', '2026-12-01 08:00:00');
    typeDate('Hạn nộp', '2026-12-05 17:00:00');
    next();
    const follow = await screen.findByRole('checkbox', { name: /Luôn dùng phiên bản mới nhất/ });
    expect(follow).not.toBeDisabled();
    fireEvent.click(screen.getByRole('radio', { name: 'Kiểm tra (tính điểm)' }));
    expect(follow).toBeDisabled();
  });

  it('network error after create: retry reuses both keys and creates nothing twice (F055-AC-05, edge case)', async () => {
    const { backend, lms } = await renderLms(URL);
    // the network fails on publish until the user retries (automatic retries of the apiClient keep failing too)
    let fail = true;
    backend.overrides.unshift((r) => (r.path.endsWith(':publish') && fail ? Promise.reject(new TypeError('Failed to fetch')) : undefined));
    await fillWizard();
    fireEvent.click(screen.getByRole('button', { name: 'Giao bài' }));
    expect(await screen.findByTestId('wizard-error')).toBeInTheDocument();
    fail = false;
    fireEvent.click(screen.getByRole('button', { name: 'Giao bài' }));
    expect(await screen.findByTestId('assignment-detail')).toBeInTheDocument();
    expect(lms.assignments.size).toBe(1);
    expect(new Set(backend.calls('POST', '/lms/assignments').map((r) => r.headers['idempotency-key'])).size).toBe(1);
    const id = [...lms.assignments.keys()][0]!;
    const pubKeys = backend.calls('POST', `/lms/assignments/${id}:publish`).map((r) => r.headers['idempotency-key']);
    expect(pubKeys.length).toBeGreaterThanOrEqual(2);
    expect(new Set(pubKeys).size).toBe(1);
  });

  it('422 VALIDATION_FAILED errors[] → the field and its step; CONTENT_NOT_PUBLISHED → back to content with code message + trace id (F055-AC-06)', async () => {
    const { backend } = await renderLms(URL);
    let first = true;
    backend.overrides.unshift((r) => {
      if (r.method !== 'POST' || r.path !== '/lms/assignments') return undefined;
      if (first) {
        first = false;
        return problem(422, 'VALIDATION_FAILED', { detail: 'raw server detail', errors: [{ field: '/title', code: 'TOO_LONG' }] });
      }
      return problem(422, 'CONTENT_NOT_PUBLISHED', { detail: 'raw server detail' });
    });
    await fillWizard();
    fireEvent.click(screen.getByRole('button', { name: 'Giao bài' }));
    expect(await screen.findByLabelText('Tiêu đề')).toBeInTheDocument();
    expect(screen.getByText(/Dữ liệu (nhập )?chưa hợp lệ/)).toBeInTheDocument();
    next();
    next();
    await screen.findByTestId('wizard-review');
    fireEvent.click(screen.getByRole('button', { name: 'Giao bài' }));
    const err = await screen.findByTestId('wizard-error');
    expect(err).toHaveTextContent('Nội dung đã chọn chưa được xuất bản hoặc vừa bị gỡ');
    expect(err).toHaveTextContent('trace-content_not_published');
    expect(screen.getByTestId('content-picker')).toBeInTheDocument();
    expect(screen.queryByText('raw server detail')).toBeNull();
  });

  it('System Admin: must pick one target tenant; body tenant_id and X-Tenant-Id = that tenant (F055-AC-07)', async () => {
    const { backend } = await renderLms(URL, { as: 'sa.admin', account: null });
    expect(await screen.findByText('Chọn đúng một tenant để làm việc')).toBeInTheDocument();
    expect(backend.calls('GET', '/cms/learning/activities')).toHaveLength(0);
    act(() => tenantFilterStore.set([TENANTS.thDemoA.tenant_id]));
    await fillWizard();
    fireEvent.click(screen.getByRole('button', { name: 'Giao bài' }));
    expect(await screen.findByTestId('assignment-detail')).toBeInTheDocument();
    const create = backend.calls('POST', '/lms/assignments')[0]!;
    expect(create.headers['x-tenant-id']).toBe(TENANTS.thDemoA.tenant_id);
    expect((create.body as Record<string, unknown>).tenant_id).toBe(TENANTS.thDemoA.tenant_id);
  });
});
