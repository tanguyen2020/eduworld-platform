// F055 T009–T014 (US1) — F055-AC-01…AC-07, AC-22 (TC-LMS-001, TC-CMS-ASG-01, TC-CMS-TEN-03): content step lists only
// published content of a type enabled in the Release + quests from listGameActivities; targets only from the API
// (scope); graded disables follow_latest and sends no release defaults; limits from the contract; "Giao bài" =
// createAssignment + publishAssignment with If-Match, one Idempotency-Key per call kept on retry and double click;
// server errors mapped to fields / code messages with trace id; System Admin bound to one target tenant. Backend = MOCK.
import { act, cleanup, fireEvent, screen, waitFor, within } from '@testing-library/react';
import { afterEach, describe, expect, it } from 'vitest';
import { tenantFilterStore } from '../../../../shell/tenantFilterStore';
import { IDS, LMS_IDS, problem, renderLms, stopPortals, TENANTS } from '../../../../test/lms-harness';

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

/** Steps 1–4 with valid data; ends on the review step. */
async function fillWizard(opts: { graded?: boolean } = {}) {
  const picker = await screen.findByTestId('content-picker');
  const row = (await within(picker).findByText('Quiz phép nhân')).closest('li')!;
  fireEvent.click(within(row).getByRole('button', { name: 'Chọn' }));
  next();
  await pickOption('Khóa học', '3A · Toán');
  await waitFor(() => expect(screen.getByTestId('target-select')).toHaveTextContent('3A'));
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

  it('targets: courses, classes, groups and students come from the API of the session tenant (F055-AC-02)', async () => {
    const { backend } = await renderLms(URL);
    const picker = await screen.findByTestId('content-picker');
    fireEvent.click(within((await within(picker).findByText('Quiz phép nhân')).closest('li')!).getByRole('button', { name: 'Chọn' }));
    next();
    await pickOption('Khóa học', '3A · Toán');
    fireEvent.click(screen.getByRole('radio', { name: 'Nhóm học tập' }));
    fireEvent.mouseDown(within(screen.getByTestId('target-select')).getByRole('combobox'));
    expect(await screen.findByTitle('Ôn Nhân 3A')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('radio', { name: 'Học sinh cụ thể' }));
    fireEvent.mouseDown(within(screen.getByTestId('target-select')).getByRole('combobox'));
    expect(await screen.findByTitle('Nguyễn An')).toBeInTheDocument();
    for (const path of ['/lms/courses', `/lms/schools/${IDS.schoolA}/study-groups`, `/tenant/schools/${IDS.schoolA}/classes/${IDS.class3A}/students`]) {
      const calls = backend.calls('GET', path);
      expect(calls.length, path).toBeGreaterThan(0);
      expect(calls.every((r) => r.headers['x-tenant-id'] === TENANTS.thDemoA.tenant_id && !r.search.includes('tenant_ids')), path).toBe(true);
    }
  });
});

describe('Wizard giao bài — chính sách, giới hạn, gửi', () => {
  it('limits before sending: counter + maxLength on title, due must be after open (F055-AC-04)', async () => {
    await renderLms(URL);
    const picker = await screen.findByTestId('content-picker');
    fireEvent.click(within((await within(picker).findByText('Quiz phép nhân')).closest('li')!).getByRole('button', { name: 'Chọn' }));
    next();
    await pickOption('Khóa học', '3A · Toán');
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
    expect(body).toMatchObject({ mode: 'graded', follow_latest: false, course_id: LMS_IDS.courseMath, content_ref: { content_type: 'activity', content_id: LMS_IDS.activityQuiz, pinned_version_id: LMS_IDS.activityQuizV } });
    expect(body.targets).toEqual([{ target_type: 'class', target_id: IDS.class3A }]);
    const id = [...lms.assignments.keys()][0]!;
    const publish = backend.calls('POST', `/lms/assignments/${id}:publish`);
    expect(publish.length).toBeGreaterThan(0);
    expect(publish[0]!.headers['if-match']).toBe('W/"1"');
    expect(publish[0]!.headers['idempotency-key']).toMatch(/^[0-9a-f-]{36}$/);
    expect(publish[0]!.headers['idempotency-key']).not.toBe(creates[0]!.headers['idempotency-key']);
  });

  it('graded mode disables the follow_latest option (F055-AC-03)', async () => {
    await renderLms(URL);
    const picker = await screen.findByTestId('content-picker');
    fireEvent.click(within((await within(picker).findByText('Quiz phép nhân')).closest('li')!).getByRole('button', { name: 'Chọn' }));
    next();
    await pickOption('Khóa học', '3A · Toán');
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
