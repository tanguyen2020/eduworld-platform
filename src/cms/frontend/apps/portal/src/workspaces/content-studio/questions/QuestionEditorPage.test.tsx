// F053 T023 (+ T024–T026, T041, T064 at unit/integration level with the MOCK backends; real-backend Playwright is
// pending like F051 — OQ F051-9): the question editor page in the whole portal.
//   AC-06/AC-07 autosave with If-Match, "Đã lưu lúc", conflict → banner, no overwrite (TC-CNT-003)
//   AC-08 published read only + "Tạo bản nháp từ v1" → createQuestionVersion (TC-CMS-001)
//   AC-09/AC-10 approve (If-Match + Idempotency-Key), publish retried with the SAME key → one release
//   AC-04 grade 3 without spoken stem → Gửi duyệt locked with the field listed (TC-CNT-010)
//   AC-05 upload image (license + alt) → status until ready (TC-CNT-008)
//   AC-11 assigned platform content → "Từ EduWorld", read only (TC-CMS-ASG-02)
//   AC-12 System Admin must choose Platform or a tenant (TC-CMS-TEN-03); AC-15 < 1024 px → read only
//   AC-13 reviewer without write: no Lưu / Gửi duyệt; 403 PERMISSION_DENIED message; AC-33 action turned off → disabled
//   AC-25 out of Release 1 (owner 2026-11-22): no translation export/import buttons
import { cleanup, configure, fireEvent, screen, waitFor, within } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { clearUnsavedChangesForTest } from '../../../session/unsavedChangesGuard';
import { MockPortalBackend, problem, renderPortal, setViewportWidth, stopPortals, TENANTS } from '../../../test/portal-harness';
import { MockContentBackend, U } from '../../../test/mock-content';

// the first test loads the whole Content Studio chunk (F053 + F054 screens) on a throttled CPU: 30 s for async queries
configure({ asyncUtilTimeout: 30_000 });

afterEach(() => {
  stopPortals();
  clearUnsavedChangesForTest();
  vi.unstubAllGlobals();
});

const Q = U(1);
const p = (s: string) => ({ blocks: [{ type: 'paragraph', text: s }] });
const opts = [
  { id: 'o_aaaa', content: p('56') },
  { id: 'o_bbbb', content: p('54') },
];
const body = (extra: Record<string, unknown> = {}) => ({
  type: 'single_choice',
  stem: p('7 × 8 = ?'),
  render_payload: { options: opts, shuffle_options: true },
  answer_key: { correct_option_id: 'o_aaaa' },
  scoring_rule: { mode: 'all_or_nothing' },
  grade_levels: [4],
  hints: [],
  ...extra,
});

function setup(account: string, versions: Parameters<MockContentBackend['addQuestion']>[0]['versions'], extra: { assigned?: boolean } = {}) {
  const backend = new MockPortalBackend().withSession(account);
  const content = new MockContentBackend();
  content.addQuestion({ id: Q, tenant_id: null, owner_scope: 'platform', ...extra, versions });
  backend.overrides.push(content.handler);
  return { backend, content };
}

const btn = (name: string) => screen.queryAllByText(name).map((e) => e.closest('button')).find(Boolean) ?? null;
const findBtn = async (name: string) => {
  await screen.findAllByText(name);
  return btn(name)!;
};

describe('QuestionEditorPage', () => {
  it('AC-08: published version is read only; "Tạo bản nháp từ v1" creates v2 (draft, editable)', async () => {
    const { backend } = setup('cm.noidung', [{ status: 'published', body: body() }]);
    await renderPortal({ url: `/content-studio/question-bank/${Q}`, backend });
    expect(await screen.findByText(/Phiên bản v1 đã xuất bản và không sửa được/)).toBeInTheDocument();
    expect(btn('Lưu')).toBeNull();
    expect(btn('Gửi duyệt')).toBeNull();
    fireEvent.click(await findBtn('Tạo bản nháp từ v1'));
    await waitFor(() => expect(backend.calls('POST', `/cms/learning/questions/${Q}/versions`)).toHaveLength(1));
    expect(backend.calls('POST', `/cms/learning/questions/${Q}/versions`)[0]!.body).toEqual({ from_version: 1 });
    expect(backend.calls('POST', `/cms/learning/questions/${Q}/versions`)[0]!.headers['idempotency-key']).toBeTruthy();
    expect(await findBtn('Gửi duyệt')).toBeInTheDocument();
  });

  it('AC-06/AC-07: autosave PATCH with If-Match, "Đã lưu lúc"; a conflict stops autosave and shows the banner', async () => {
    const { backend, content } = setup('cm.noidung', [{ status: 'draft', body: body(), author_id: '0193a4d2-1111-7000-8000-000000000004' }]);
    await renderPortal({ url: `/content-studio/question-bank/${Q}`, backend });
    await screen.findByDisplayValue('7 × 8 = ?');
    // the editor settles after its reads (metadata lists); type into the live stem field
    await waitFor(() => expect(backend.calls('GET', '/cms/learning/subjects').length).toBeGreaterThan(0));
    await new Promise((r) => setTimeout(r, 500));
    fireEvent.change(document.getElementById('stem.blocks.0.text.vi')!, { target: { value: '7 × 8 bằng mấy?' } });
    expect(await screen.findByDisplayValue('7 × 8 bằng mấy?')).toBeInTheDocument();
    await waitFor(() => expect(backend.calls('PATCH', `/cms/learning/questions/${Q}/versions/1`)).toHaveLength(1), { timeout: 30_000 });
    const patch = backend.calls('PATCH', `/cms/learning/questions/${Q}/versions/1`)[0]!;
    expect(patch.headers['if-match']).toBe('W/"1"');
    expect(patch.headers['content-type']).toContain('application/merge-patch+json');
    expect(patch.body).toMatchObject({ stem: p('7 × 8 bằng mấy?'), answer_key: { correct_option_id: 'o_aaaa' }, math_assets: [] });
    expect(await screen.findByText(/Đã lưu lúc \d\d:\d\d/)).toBeInTheDocument();
    // someone else saves meanwhile → 409: banner, local text kept, no further PATCH
    content.fail.push({ method: 'PATCH', path: /versions\/1$/, response: () => problem(409, 'CONTENT_VERSION_CONFLICT') });
    fireEvent.change(screen.getByDisplayValue('7 × 8 bằng mấy?'), { target: { value: 'Bản của tôi' } });
    expect(await screen.findByText('Người khác đã sửa bản nháp này', {}, { timeout: 30_000 })).toBeInTheDocument();
    const n = backend.calls('PATCH', `/cms/learning/questions/${Q}/versions/1`).length;
    fireEvent.change(screen.getByDisplayValue('Bản của tôi'), { target: { value: 'Bản của tôi 2' } });
    await new Promise((r) => setTimeout(r, 2500));
    expect(backend.calls('PATCH', `/cms/learning/questions/${Q}/versions/1`)).toHaveLength(n);
    expect(screen.getByDisplayValue('Bản của tôi 2')).toBeInTheDocument();
  });

  it('AC-09/AC-10: reviewer approves (If-Match + key), publish retried after a failure reuses the key → one release', async () => {
    const { backend, content } = setup('cm.noidung', [{ status: 'in_review', row: 4, body: body() }]);
    await renderPortal({ url: `/content-studio/question-bank/${Q}`, backend });
    fireEvent.click(await findBtn('Duyệt'));
    await waitFor(() => expect(backend.requests.filter((r) => r.path.endsWith(':transition'))).toHaveLength(1));
    const tr = backend.requests.find((r) => r.path.endsWith(':transition'))!;
    expect(tr.body).toEqual({ action: 'approve' });
    expect(tr.headers['if-match']).toBe('W/"4"');
    expect(tr.headers['idempotency-key']).toBeTruthy();
    content.fail.push({ method: 'POST', path: /:publish$/, response: () => problem(500, 'INTERNAL'), once: true });
    fireEvent.click(await findBtn('Xuất bản ngay'));
    const dialog = await screen.findByRole('dialog');
    fireEvent.click(within(dialog).getByText('Xuất bản').closest('button')!);
    await waitFor(() => expect(backend.requests.filter((r) => r.path.endsWith(':publish'))).toHaveLength(1));
    fireEvent.click(within(dialog).getByText('Xuất bản').closest('button')!);
    await waitFor(() => expect(backend.requests.filter((r) => r.path.endsWith(':publish'))).toHaveLength(2));
    const [a, b] = backend.requests.filter((r) => r.path.endsWith(':publish'));
    expect(a!.headers['idempotency-key']).toBe(b!.headers['idempotency-key']);
    expect(content.releases).toHaveLength(1);
    expect(await screen.findByText('Đã xuất bản v1')).toBeInTheDocument();
  });

  it('AC-04: grade 3 without spoken stem → Gửi duyệt locked, field listed; exemption reason unlocks', async () => {
    const { backend } = setup('cm.noidung', [{ status: 'draft', body: body({ grade_levels: [3] }) }]);
    await renderPortal({ url: `/content-studio/question-bank/${Q}`, backend });
    expect(await screen.findByText('Câu hỏi lớp 1–3 thiếu âm thanh đọc đề (hoặc lý do miễn).')).toBeInTheDocument();
    expect(await findBtn('Gửi duyệt')).toBeDisabled();
    fireEvent.change(screen.getAllByLabelText('Lý do miễn âm thanh đọc đề').find((e) => e.tagName === 'TEXTAREA')!, { target: { value: 'Câu đánh vần' } });
    await waitFor(() => expect(btn('Gửi duyệt')).toBeEnabled());
  });

  it('AC-11: assigned platform question → "Từ EduWorld", read only, no action', async () => {
    const { backend } = setup('qlcm.truong.a', [{ status: 'published', body: body() }], { assigned: true });
    window.sessionStorage.setItem('ew.portal.sessionTenantId', TENANTS.thDemoA.tenant_id);
    await renderPortal({ url: `/content-studio/question-bank/${Q}`, backend });
    expect(await screen.findByTestId('assigned-badge')).toHaveTextContent('Từ EduWorld');
    expect(btn('Tạo bản nháp từ v1')).toBeNull();
    expect(btn('Lưu')).toBeNull();
  });

  it('AC-12: System Admin must pick Platform or a target tenant before the first save; AC-15: narrow → read only', async () => {
    const backend = new MockPortalBackend().withSession('sa.admin');
    const content = new MockContentBackend();
    backend.overrides.push(content.handler);
    await renderPortal({ url: '/content-studio/question-bank/new', backend });
    fireEvent.click(await findBtn('Lưu'));
    expect(await screen.findByText('Chọn một tenant đích.')).toBeInTheDocument();
    expect(backend.calls('POST', '/cms/learning/questions')).toHaveLength(0);
    stopPortals();
    cleanup();
    setViewportWidth(800);
    await renderPortal({ url: '/content-studio/question-bank/new', backend: new MockPortalBackend().withSession('cm.noidung') });
    expect(await screen.findByText('Tác vụ này cần màn hình lớn hơn')).toBeInTheDocument();
    expect(btn('Lưu')).toBeNull();
  });

  it('AC-13/AC-33: reviewer without write has no Lưu / Gửi duyệt; publish turned off → disabled; 403 PERMISSION_DENIED shown', async () => {
    const { backend, content } = setup('cm.noidung', [{ status: 'draft', body: body() }]);
    backend.overrides.unshift((r) =>
      r.path === '/me/permissions'
        ? new Response(JSON.stringify({ roles: [], permissions: ['learning_content.read', 'learning_content.review', 'learning_content.publish'], workspaces: ['content_studio'], disabled_menus: [], disabled_actions: ['action.content_studio.question_bank.publish'] }), { status: 200, headers: { 'Content-Type': 'application/json' } })
        : undefined,
    );
    await renderPortal({ url: `/content-studio/question-bank/${Q}`, backend });
    await screen.findByDisplayValue('7 × 8 = ?');
    expect(btn('Lưu')).toBeNull();
    expect(btn('Gửi duyệt')).toBeNull();
    content.questions.get(Q)!.versions[0]!.status = 'approved';
    stopPortals();
    cleanup();
    await renderPortal({ url: `/content-studio/question-bank/${Q}`, backend });
    expect(await findBtn('Xuất bản ngay')).toBeDisabled();
    content.questions.get(Q)!.versions[0]!.status = 'in_review';
    content.fail.push({ method: 'POST', path: /:transition$/, response: () => problem(403, 'PERMISSION_DENIED') });
    stopPortals();
    cleanup();
    await renderPortal({ url: `/content-studio/question-bank/${Q}`, backend });
    fireEvent.click(await findBtn('Duyệt'));
    expect(await screen.findByText('Bạn không có quyền thực hiện thao tác này.')).toBeInTheDocument();
  });

  it('AC-05: image upload needs license + alt, then follows the pipeline to "Sẵn sàng"', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => new Response(null, { status: 200 }))); // MOCK object storage PUT
    const { backend } = setup('cm.noidung', [{ status: 'draft', body: body({ stem: { blocks: [{ type: 'image', alt: 'Hình' }] } }) }]);
    await renderPortal({ url: `/content-studio/question-bank/${Q}`, backend });
    fireEvent.click((await findBtn('Tải lên')));
    const dialog = await screen.findByRole('dialog');
    const file = new File([new Uint8Array([1, 2, 3])], 'tam-giac.png', { type: 'image/png' });
    fireEvent.change(within(dialog).getByLabelText('Tệp'), { target: { files: [file] } });
    fireEvent.click(within(dialog).getByText('Tải lên').closest('button')!);
    expect(await within(dialog).findByText('Bắt buộc ghi giấy phép hoặc nguồn.')).toBeInTheDocument();
    expect(await within(dialog).findByText('Ảnh phải có mô tả.')).toBeInTheDocument();
    fireEvent.change(within(dialog).getByLabelText('Giấy phép / nguồn'), { target: { value: 'CC BY 4.0' } });
    fireEvent.change(within(dialog).getByLabelText('Mô tả ảnh (alt)'), { target: { value: 'Tam giác ABC' } });
    fireEvent.click(within(dialog).getByText('Tải lên').closest('button')!);
    await waitFor(() => expect(backend.calls('POST', '/cms/media:presign-upload')).toHaveLength(1));
    expect(backend.calls('POST', '/cms/media:presign-upload')[0]!.body).toMatchObject({ kind: 'image', license: 'CC BY 4.0', alt_text: 'Tam giác ABC', filename: 'tam-giac.png' });
    expect(await screen.findByText('Sẵn sàng', {}, { timeout: 30_000 })).toBeInTheDocument();
  });

  it('OQ F054-1: a rejected version shows the reviewer field comments of its latest rejection to the author', async () => {
    const { backend } = setup('cm.noidung', [{ status: 'rejected', body: body(), author_id: '0193a4d2-1111-7000-8000-000000000004' }]);
    // MOCK (test only): listContentVersions answers the summary with field_comments (F025 T046)
    backend.overrides.push((r) =>
      r.method === 'GET' && r.path === `/cms/learning/content/question/${Q}/versions`
        ? new Response(JSON.stringify({ item_id: Q, data: [{ version: 1, status: 'rejected', field_comments: [{ path: '/stem', comment: 'Thiếu đơn vị đo' }, { path: '', comment: 'Xem lại cả câu' }] }], page: { has_more: false } }), { status: 200, headers: { 'Content-Type': 'application/json' } })
        : undefined,
    );
    await renderPortal({ url: `/content-studio/question-bank/${Q}`, backend });
    const box = await screen.findByTestId('review-field-comments');
    expect(box).toHaveTextContent('Ghi chú của người duyệt khi từ chối v1');
    expect(within(box).getByText('/stem')).toBeInTheDocument();
    expect(box).toHaveTextContent('Thiếu đơn vị đo');
    expect(within(box).getByText('Cả phiên bản')).toBeInTheDocument();
    expect(await findBtn('Lưu')).toBeInTheDocument(); // still editable
  });

  it('AC-25 out of Release 1: no translation export/import on the editor', async () => {
    const { backend } = setup('cm.noidung', [{ status: 'draft', body: body() }]);
    await renderPortal({ url: `/content-studio/question-bank/${Q}`, backend });
    expect(await findBtn('Lưu')).toBeInTheDocument();
    expect(screen.queryByText('Xuất file dịch')).toBeNull();
    expect(screen.queryByText('Nhập file dịch')).toBeNull();
  });
});
