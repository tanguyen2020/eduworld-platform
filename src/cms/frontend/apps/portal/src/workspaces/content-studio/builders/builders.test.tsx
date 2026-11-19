// F054 T043–T047, T060 (whole portal + MOCK backend; Playwright with the real backend pending — OQ F051-9):
//   AC-20 question set rule editor with pool size per rule · AC-21 Lesson Builder blocks, completion rule, math_assets
//   · AC-22 blueprint matrix, total points, sample exam → QUESTION_POOL_INSUFFICIENT highlights the cells ·
//   AC-23 draft saved with If-Match; CONTENT_NOT_EDITABLE → read only + "Tạo bản nháp" · AC-28 reviewer cannot create
//   · AC-27 import: formula errors by row before sending, dry run with row errors, real run.
import { fireEvent, screen, waitFor, within } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { P, problem, renderPublishing, stopPortals, tenantUser, TENANT_CONTENT_PERMS, type MockPublishing } from '../../../test/publishing-harness';

afterEach(() => {
  stopPortals();
  vi.unstubAllGlobals();
});

const LO1 = P(31);
const LO2 = P(32);

describe('Question Set / Lesson / Blueprint builders (F054 US4)', () => {
  it('AC-20: rule-based set shows the pool of each rule; AC-23 autosave PATCH with If-Match', async () => {
    const QS = P(501);
    const { backend } = await renderPublishing(`/content-studio/question-sets/${QS}`, {
      setup: (m: MockPublishing) => {
        m.add({ id: QS, type: 'question_set', tenant_id: null, title: { vi: 'Luyện nhân' }, versions: [{ status: 'draft', content: { title: { vi: 'Luyện nhân' }, definition: { kind: 'rule_based', purpose: 'exercise', selection_rules: [{ filters: { objective_ids: [LO1], difficulty: { min: 2, max: 2 } }, count: 5 }, { filters: {}, count: 10 }] } } }] });
        m.items.get(QS)!.pool = [
          { rule_index: 0, required: 5, available: 12, insufficient: false, warning: true },
          { rule_index: 1, required: 10, available: 3, insufficient: true, warning: true },
        ];
      },
    });
    expect(await screen.findByTestId('pool-0')).toHaveTextContent('Pool 12/5');
    expect(screen.getByTestId('pool-1')).toHaveTextContent('Pool 3/10');
    expect(screen.getByTestId('pool-1').className).toContain('error');
    fireEvent.change(screen.getByLabelText('Số câu của luật 2'), { target: { value: '3' } });
    await waitFor(() => expect(backend.calls('PATCH', `/cms/learning/question-sets/${QS}/versions/1`)).toHaveLength(1), { timeout: 60_000 });
    const patch = backend.calls('PATCH', `/cms/learning/question-sets/${QS}/versions/1`)[0]!;
    expect(patch.headers['if-match']).toBe('W/"1"');
    expect((patch.body as { definition: { selection_rules: { count: number }[] } }).definition.selection_rules[1]!.count).toBe(3);
  });

  it('AC-21: Lesson Builder — add blocks, mark required, completion rule; create sends blocks + math_assets', async () => {
    const { backend } = await renderPublishing('/content-studio/lessons/new', { account: tenantUser() });
    fireEvent.change(await screen.findByLabelText('Tiêu đề (Tiếng Việt)'), { target: { value: 'Bài 1: Nhân 2' } });
    const blocks = screen.getByTestId('lesson-blocks');
    fireEvent.click(within(blocks).getByRole('button', { name: '+ Ví dụ mẫu' }));
    fireEvent.click(within(blocks).getByRole('button', { name: '+ Hoạt động' }));
    expect(within(blocks).getByText('3/100 khối')).toBeInTheDocument();
    fireEvent.change(within(blocks).getByLabelText('Mã hoạt động của khối 3'), { target: { value: P(77) } });
    fireEvent.click(within(blocks).getByRole('button', { name: 'Đưa khối 3 lên' }));
    expect(within(blocks).getByText('Khối 2 · Hoạt động')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Lưu' }));
    await waitFor(() => expect(backend.calls('POST', '/cms/learning/lessons')).toHaveLength(1));
    const body = backend.calls('POST', '/cms/learning/lessons')[0]!.body as { blocks: { kind: string; required?: boolean }[]; math_assets: unknown[]; completion_rule: string; title: unknown };
    expect(body.blocks.map((b) => b.kind)).toEqual(['rich_text', 'activity', 'worked_example']);
    expect(body.blocks[1]!.required).toBe(true);
    expect(body.completion_rule).toBe('all_required_activities');
    expect(Array.isArray(body.math_assets)).toBe(true);
    expect(backend.calls('POST', '/cms/learning/lessons')[0]!.headers['idempotency-key']).toBeTruthy();
  });

  it('AC-22: blueprint matrix totals and points check; sample exam → QUESTION_POOL_INSUFFICIENT highlights the missing cell', async () => {
    const BP = P(601);
    const rules = [
      { objective_ids: [LO1], difficulty: { min: 1, max: 1 }, count: 4, points_each: 1 },
      { objective_ids: [LO2], difficulty: { min: 2, max: 2 }, count: 3, points_each: 2 },
    ];
    await renderPublishing(`/content-studio/assessments/${BP}`, {
      setup: (m) => {
        m.add({ id: BP, type: 'assessment_blueprint', tenant_id: null, title: { vi: 'Kiểm tra chương' }, versions: [{ status: 'draft', content: { title: { vi: 'Kiểm tra chương' }, subject_id: P(10), grade_levels: [3], objective_ids: [LO1, LO2], definition: { duration_sec: 2700, total_points: 10, sections: [{ id: 'sec_main', title: { vi: 'P1' }, rules }], attempt_policy: { max_attempts: 1, score_policy: 'best' }, result_release: { mode: 'after_close' } } } }] });
        m.items.get(BP)!.pool = [{ section_id: 'sec_main', rule_index: 1, required: 3, available: 1, insufficient: true, warning: true }];
      },
    });
    expect(await screen.findByTestId('blueprint-totals')).toHaveTextContent('Tổng: 7 câu · 10 điểm (cần 10)');
    fireEvent.change(screen.getByLabelText('Tổng điểm'), { target: { value: '12' } });
    expect(await screen.findAllByText('Tổng điểm của ma trận (10) khác tổng điểm đề (12).')).not.toHaveLength(0);
    fireEvent.change(screen.getByLabelText('Tổng điểm'), { target: { value: '10' } });
    fireEvent.click(screen.getByRole('button', { name: 'Sinh đề mẫu' }));
    expect(await screen.findByText('Ngân hàng câu hỏi không đủ câu theo ma trận.')).toBeInTheDocument();
    await waitFor(() => expect(document.querySelector(`[data-cell="${LO2}:2"]`)?.getAttribute('data-insufficient')).toBe('true'));
    expect(screen.getByText('Ngân hàng không đủ câu cho các ô tô đỏ; giảm số câu hoặc bổ sung câu hỏi.')).toBeInTheDocument();
    expect(document.querySelector(`[data-cell="${LO1}:1"]`)?.getAttribute('data-insufficient')).toBeNull();
  });

  it('AC-23: CONTENT_NOT_EDITABLE on save → read only with "Tạo bản nháp từ phiên bản"; AC-28 a reviewer cannot create', async () => {
    const L = P(701);
    const { backend } = await renderPublishing(`/content-studio/lessons/${L}`, {
      setup: (m) => {
        m.add({ id: L, type: 'lesson', tenant_id: null, title: { vi: 'Bài 2' }, versions: [{ status: 'draft', content: { title: { vi: 'Bài 2' }, completion_rule: 'view_only', blocks: [{ kind: 'rich_text', content: { blocks: [{ type: 'paragraph', text: { vi: 'Xin chào' } }] } }] } }] });
        m.fail.push({ method: 'PATCH', path: /lessons\/.+\/versions\/1$/, response: () => problem(409, 'CONTENT_NOT_EDITABLE') });
      },
    });
    fireEvent.change(await screen.findByLabelText('Thời lượng dự kiến (phút)'), { target: { value: '20' } });
    fireEvent.click(await screen.findByRole('button', { name: 'Lưu' }));
    expect((await screen.findAllByText('Phiên bản này không còn sửa được. Hãy tạo bản nháp mới.')).length).toBeGreaterThan(0);
    fireEvent.click(await screen.findByRole('button', { name: 'Tạo bản nháp từ phiên bản v1' }));
    await waitFor(() => expect(backend.calls('POST', `/cms/learning/lessons/${L}/versions`)).toHaveLength(1));
    stopPortals();
    document.body.innerHTML = '';
    const reviewer = TENANT_CONTENT_PERMS.filter((p) => p !== 'learning_content.write');
    await renderPublishing('/content-studio/lessons', { account: tenantUser(reviewer) });
    expect(await screen.findByRole('heading', { level: 1 })).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Tạo mới' })).toBeNull();
  });
});

describe('Nhập câu hỏi (F054-AC-27)', () => {
  const header = 'type,subject_code,grade_levels,curriculum_code,topic_code,objective_codes,difficulty,stem,render_payload,answer_key,scoring_rule';
  const q = (s: string) => `"${s.replace(/"/g, '""')}"`;
  const stem = (text: string) => q(JSON.stringify({ blocks: [{ type: 'paragraph', text: { vi: text } }] }));
  const line = (s: string) => ['single_choice', 'MATH', '3', 'CT', 'T1', 'LO1', '2', s, q('{"options":[]}'), q('{"correct_option_id":"o_a"}'), q('{"mode":"all_or_nothing"}')].join(',');

  it('formula errors are listed by row before anything is sent', async () => {
    const { backend } = await renderPublishing('/content-studio/question-bank/import', { account: tenantUser() });
    const csv = [header, line(stem('Tính \\(\\frac{1}{2}\\)')), line(stem('Sai \\(\\href{a}{b}\\)'))].join('\n');
    await screen.findByText('Kéo thả hoặc bấm để chọn tệp .xlsx hoặc .csv');
    const input = document.querySelector('input[type="file"]') as HTMLInputElement;
    fireEvent.change(input, { target: { files: [new File([csv], 'cau-hoi.csv', { type: 'text/csv' })] } });
    const issues = await screen.findByTestId('import-math-issues');
    expect(within(issues).getByText('Công thức dùng lệnh bị cấm')).toBeInTheDocument();
    expect(within(issues.querySelector('tbody')!).getByText('3')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Tải lên và chạy thử' })).toBeDisabled();
    expect(backend.calls('POST', '/cms/media:presign-upload')).toHaveLength(0);
  });

  it('dry run → row errors from the server, nothing created; real run creates drafts (one import per run)', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => new Response(null, { status: 200 }))); // MOCK object storage PUT
    const { backend } = await renderPublishing('/content-studio/question-bank/import', {
      account: tenantUser(),
      setup: (m) => {
        m.importErrors = [{ row: 3, field: 'objective_codes', code: 'NOT_FOUND', message: 'LO9 không tồn tại' }];
      },
    });
    const csv = [header, line(stem('Tính \\(2 \\times 3\\)')), line(stem('Câu 2'))].join('\n');
    await screen.findByText('Kéo thả hoặc bấm để chọn tệp .xlsx hoặc .csv');
    const input = document.querySelector('input[type="file"]') as HTMLInputElement;
    fireEvent.change(input, { target: { files: [new File([csv], 'cau-hoi.csv', { type: 'text/csv' })] } });
    expect(await screen.findByTestId('import-math-summary')).toHaveTextContent('Đã đọc 2 dòng, 1 công thức, 0 lỗi');
    fireEvent.click(screen.getByRole('button', { name: 'Tải lên và chạy thử' }));
    const report = await screen.findByTestId('import-report', {}, { timeout: 60_000 });
    expect(report).toHaveTextContent('Chưa tạo bản nháp nào');
    expect(within(screen.getByTestId('import-server-errors')).getByText('LO9 không tồn tại')).toBeInTheDocument();
    const presign = backend.calls('POST', '/cms/media:presign-upload')[0]!;
    expect(presign.body).toMatchObject({ kind: 'document', filename: 'cau-hoi.csv', mime_type: 'text/csv' });
    const uploaded = (vi.mocked(fetch).mock.calls[0]![1] as RequestInit).body as File;
    const text = await new Promise<string>((resolve) => {
      const r = new FileReader();
      r.onload = () => resolve(String(r.result));
      r.readAsText(uploaded);
    });
    expect(text.slice(0, text.indexOf(String.fromCharCode(10)))).toContain('math_assets');
    expect(text).toContain('<svg');
    expect(backend.calls('POST', '/cms/learning/imports')[0]!.body).toMatchObject({ kind: 'questions_csv', dry_run: true });
    fireEvent.click(screen.getByRole('button', { name: 'Chạy thật (tạo bản nháp)' }));
    expect(await screen.findByText(/đã tạo 18 bản nháp/, {}, { timeout: 60_000 })).toBeInTheDocument();
    const runs = backend.calls('POST', '/cms/learning/imports');
    expect(runs).toHaveLength(2);
    expect(runs[1]!.body).toMatchObject({ dry_run: false });
    expect(runs[0]!.headers['idempotency-key']).not.toBe(runs[1]!.headers['idempotency-key']);
    expect(screen.getByRole('button', { name: 'Chạy thật (tạo bản nháp)' })).toBeDisabled();
  });

  it('import needs ≥ 1024 px (AC-30)', async () => {
    await renderPublishing('/content-studio/question-bank/import', { account: tenantUser(), width: 900 });
    expect(await screen.findByText('Tác vụ này cần màn hình lớn hơn')).toBeInTheDocument();
  });
});
