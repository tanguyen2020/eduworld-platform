// F028-AC-15 (T030): the Word import of the import screen with the MOCK backend of the contract — upload of the .docx
// as a document asset, the lessons_docx analysis job, the server-paged report filtered by status, drafts created one
// question at a time with the MathJax SVG of exactly its formulas (one Idempotency-Key per question), a formula that
// does not render is reported without calling the API, and the action gate.
import { fireEvent, screen, waitFor, within } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { renderPublishing, stopPortals, tenantUser, TENANT_CONTENT_PERMS } from '../../../../test/publishing-harness';

const DOCX = 'application/vnd.openxmlformats-officedocument.wordprocessingml.document';

/** A ContentImportItem of the mock (TEST FIXTURE, placeholder content). */
function item(no: number, status: string, formulas: { latex: string; display: boolean }[] = [{ latex: '\\frac{3}{4}', display: false }]) {
  return {
    import_id: 'x',
    item_no: no,
    source_label: `Câu ${no}`,
    status,
    question_type: status === 'error' ? undefined : 'single_choice',
    stem_preview: `Tính giá trị câu mẫu ${no}`,
    issues: status === 'error' ? [{ code: 'ANSWER_MISSING', severity: 'error' }] : status === 'warning' ? [{ code: 'EXTERNAL_LINK_REMOVED', severity: 'warning' }] : [],
    formulas,
    images: [],
    question: { content_language: 'vi' },
    updated_at: '2026-11-27T03:00:00Z',
  };
}

async function chooseDocx() {
  fireEvent.click(await screen.findByText('Word (.docx)', {}, { timeout: 120_000 })); // first render loads the lazy route chunk
  await screen.findByText('Kéo thả hoặc bấm để chọn tệp Word .docx');
  const input = document.querySelector('input[type="file"][accept=".docx"]') as HTMLInputElement;
  fireEvent.change(input, { target: { files: [new File(['PK docx'], 'de-mau.docx', { type: DOCX })] } });
  await screen.findByRole('textbox', { name: 'Giấy phép / nguồn' });
}

afterEach(() => {
  stopPortals();
  vi.unstubAllGlobals();
});

describe('Nhập câu hỏi từ Word (F028-AC-15)', () => {
  it('analyses the .docx: report summary, table paged by the server and filtered by status; nothing created', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => new Response(null, { status: 200 }))); // MOCK object storage PUT
    const { backend } = await renderPublishing('/content-studio/question-bank/import', {
      account: tenantUser(),
      setup: (m) => {
        m.importItems = [...Array.from({ length: 23 }, (_, i) => item(i + 1, 'ready')), item(24, 'warning'), item(25, 'error')];
      },
    });
    await chooseDocx();
    fireEvent.click(screen.getByRole('button', { name: 'Tải lên và phân tích' }));
    const report = await screen.findByTestId('docx-report', {}, { timeout: 60_000 });
    expect(report).toHaveTextContent('Đã tách 25 câu: 24 câu nhập được, 1 câu lỗi. Chưa tạo bản nháp nào.');
    expect(backend.calls('POST', '/cms/media:presign-upload')[0]!.body).toMatchObject({ kind: 'document', filename: 'de-mau.docx', mime_type: DOCX });
    expect(backend.calls('POST', '/cms/learning/imports')[0]!.body).toMatchObject({ kind: 'lessons_docx', dry_run: true });
    const table = await screen.findByTestId('docx-items');
    await waitFor(() => expect(within(table).getAllByRole('row')).toHaveLength(21)); // header + one page of 20
    expect(new URLSearchParams(backend.calls('GET', /\/items$/)[0]!.search).get('limit')).toBe('20');
    fireEvent.click(screen.getByRole('button', { name: 'Tải thêm' }));
    await waitFor(() => expect(within(table).getAllByRole('row')).toHaveLength(26));
    expect(within(table).getByText('Chưa đánh dấu đáp án (in đậm chữ cái hoặc dòng "Đáp án:")')).toBeInTheDocument();
    // status filter → server query
    fireEvent.mouseDown(screen.getByRole('combobox', { name: 'Lọc theo trạng thái' }));
    fireEvent.click(await screen.findByTitle('Lỗi'));
    await waitFor(() => expect(backend.calls('GET', /\/items$/).some((c) => new URLSearchParams(c.search).get('status') === 'error')).toBe(true));
    expect(backend.calls('POST', /:create-draft$/)).toHaveLength(0);
  });

  it('creates the drafts one question at a time with the SVG of its formulas; a formula that does not render is reported', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => new Response(null, { status: 200 })));
    const { backend } = await renderPublishing('/content-studio/question-bank/import', {
      account: tenantUser(),
      setup: (m) => {
        m.importItems = [item(1, 'ready'), item(2, 'warning', []), item(3, 'ready', [{ latex: '\\href{x}{y}', display: false }]), item(4, 'error')];
      },
    });
    await chooseDocx();
    fireEvent.click(screen.getByRole('button', { name: 'Tải lên và phân tích' }));
    await screen.findByTestId('docx-report', {}, { timeout: 60_000 });
    fireEvent.click(screen.getByRole('button', { name: 'Tạo bản nháp cho các câu hợp lệ' }));
    const results = await screen.findByTestId('docx-results', {}, { timeout: 60_000 });
    expect(results).toHaveTextContent('Đã tạo 2 bản nháp, 1 câu không tạo được.');
    expect(results).toHaveTextContent('Câu 3: Công thức dùng lệnh bị cấm');
    const drafts = backend.calls('POST', /:create-draft$/);
    expect(drafts.map((c) => c.path)).toEqual([expect.stringMatching(/items\/1:create-draft$/), expect.stringMatching(/items\/2:create-draft$/)]);
    const assets = (drafts[0]!.body as { math_assets: { latex: string; svg: string; speech_text: string }[] }).math_assets;
    expect(assets).toHaveLength(1);
    expect(assets[0]!.latex).toBe('\\frac{3}{4}');
    expect(assets[0]!.svg.startsWith('<svg')).toBe(true);
    expect((drafts[1]!.body as { math_assets: unknown[] }).math_assets).toEqual([]);
    expect(drafts[0]!.headers['idempotency-key']).not.toBe(drafts[1]!.headers['idempotency-key']);
  });

  it('a file larger than csv_import_max is refused before upload; the action needs learning_content.import', async () => {
    const { backend } = await renderPublishing('/content-studio/question-bank/import', { account: tenantUser() });
    fireEvent.click(await screen.findByText('Word (.docx)', {}, { timeout: 120_000 })); // first render loads the lazy route chunk
    const input = (await waitFor(() => document.querySelector('input[type="file"][accept=".docx"]'))) as HTMLInputElement;
    const big = new File(['x'], 'lon.docx', { type: DOCX });
    Object.defineProperty(big, 'size', { value: 11 * 1024 * 1024 });
    fireEvent.change(input, { target: { files: [big] } });
    expect(await screen.findByText('Tệp vượt quá 10 MB.')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Tải lên và phân tích' })).toBeDisabled();
    expect(backend.calls('POST', '/cms/media:presign-upload')).toHaveLength(0);
    stopPortals();
    document.body.innerHTML = '';
    await renderPublishing('/content-studio/question-bank/import', { account: tenantUser(TENANT_CONTENT_PERMS.filter((p) => p !== 'learning_content.import')) });
    fireEvent.click(await screen.findByText('Word (.docx)', {}, { timeout: 120_000 })); // first render loads the lazy route chunk
    await screen.findByText('Kéo thả hoặc bấm để chọn tệp Word .docx');
    expect(screen.queryByRole('button', { name: 'Tải lên và phân tích' })).toBeNull();
  });
});
