// F054 T029–T032 (whole portal + MOCK backend; Playwright with the real backend pending — OQ F051-9): review queue.
//   AC-12 tasks by lane / type / "giao cho tôi", submission time, overdue · AC-15 tenant reviewer: no tenant_ids, no
//   platform lane · AC-13 task page: diff with the current version + preview, approve (If-Match), reject with reason +
//   field notes · AC-14 bulk approval: one Idempotency-Key per item, result per item incl. the author's own version.
import { fireEvent, screen, waitFor, within } from '@testing-library/react';
import { afterEach, describe, expect, it } from 'vitest';
import { P, renderPublishing, stopPortals, TENANTS, tenantUser, type MockPublishing } from '../../../test/publishing-harness';

afterEach(() => stopPortals());

const A = TENANTS.thDemoA.tenant_id;
const ME = '0193a4d2-1111-7000-8000-0000000000e0'; // sub of tenantUser()

function seed(m: MockPublishing, n = 3) {
  for (let i = 1; i <= n; i++) {
    const id = P(100 + i);
    m.add({ id, type: 'question', tenant_id: A, title: { vi: `Câu ${i}` }, current: 1, versions: [{ status: 'published' }, { status: 'in_review', author_id: i === 3 ? ME : P(900), content: { answer_key: { correct_option_id: 'o_a' } } }] });
    m.reviewTasks.push({ content_type: 'question', content_id: id, version: 2, title: `Câu ${i}`, lane: 'tenant', tenant_id: A, status: 'open', author_id: i === 3 ? ME : P(900), submitted_at: i === 1 ? '2026-01-02T01:00:00Z' : new Date(Date.now() - 3600_000).toISOString() });
  }
  m.reviewTasks.push({ content_type: 'lesson', content_id: P(199), version: 1, title: 'Bài platform', lane: 'platform_learning', status: 'open', submitted_at: '2026-11-19T01:00:00Z' });
  m.previewItems = [{ type: 'single_choice', render_payload: { options: [{ id: 'o_a', content: { blocks: [{ type: 'paragraph', text: { vi: '56' } }] } }] }, correct: { option_id: 'o_a' } }];
}

describe('Hàng đợi duyệt (F054 US2)', () => {
  it('AC-12/AC-15: tenant reviewer sees only tenant tasks (no tenant_ids, no platform lane), overdue warning', async () => {
    const { backend } = await renderPublishing('/content-studio/reviews', { account: tenantUser(), setup: (m) => seed(m) });
    expect(await screen.findByText('Câu 1')).toBeInTheDocument();
    expect(screen.queryByText('Bài platform')).toBeNull();
    const row1 = screen.getByText('Câu 1').closest('tr')!;
    expect(within(row1).getByText('Quá hạn')).toBeInTheDocument();
    expect(within(screen.getByText('Câu 2').closest('tr')!).queryByText('Quá hạn')).toBeNull();
    const req = backend.calls('GET', '/cms/learning/review-tasks')[0]!;
    expect(req.search).not.toContain('tenant_ids');
    expect(req.headers['x-tenant-id']).toBe(A);
    fireEvent.mouseDown(screen.getByRole('combobox', { name: 'Luồng duyệt' }));
    expect(await screen.findByTitle('Của trường / đơn vị')).toBeInTheDocument();
    expect(screen.queryByTitle('Nội dung EduWorld')).toBeNull();
    fireEvent.click(screen.getByLabelText('Giao cho tôi'));
    await waitFor(() => expect(backend.calls('GET', '/cms/learning/review-tasks').some((r) => r.search.includes('assigned_to_me=true'))).toBe(true));
  });

  it('AC-13: task page shows the diff with the current version and the preview; approve with If-Match; reject needs a reason and keeps field notes', async () => {
    const { backend } = await renderPublishing(`/content-studio/reviews/question/${P(101)}/2`, { account: tenantUser(), setup: (m) => seed(m) });
    expect(await screen.findByTestId('version-diff')).toHaveTextContent('So sánh v1 → v2');
    expect(screen.getByTestId('preview-panel')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Từ chối' }));
    const dialog = await screen.findByRole('dialog', { name: 'Từ chối phiên bản v2' });
    fireEvent.click(within(dialog).getByRole('button', { name: 'Từ chối' }));
    expect(await within(dialog).findByText('Bắt buộc nhập lý do.')).toBeInTheDocument();
    fireEvent.change(within(dialog).getByLabelText('Lý do từ chối'), { target: { value: 'Đáp án chưa đúng' } });
    fireEvent.click(within(dialog).getByRole('button', { name: 'Thêm ghi chú theo trường' }));
    fireEvent.change(await within(dialog).findByLabelText('Trường 1'), { target: { value: 'stem' } });
    fireEvent.change(within(dialog).getByLabelText('Ghi chú 1'), { target: { value: 'Thiếu đơn vị' } });
    fireEvent.click(within(dialog).getByRole('button', { name: 'Từ chối' }));
    const path = `/cms/learning/content/question/${P(101)}/versions/2:transition`;
    await waitFor(() => expect(backend.calls('POST', path)).toHaveLength(1));
    const call = backend.calls('POST', path)[0]!;
    expect(call.body).toEqual({ action: 'reject', note: 'Đáp án chưa đúng\n[stem] Thiếu đơn vị' });
    expect(call.headers['if-match']).toBe('W/"1"');
    expect(call.headers['idempotency-key']).toBeTruthy();
    expect(await screen.findByText('Câu 2')).toBeInTheDocument(); // back to the queue
  });

  it('AC-13: the author of the version gets no decision buttons', async () => {
    await renderPublishing(`/content-studio/reviews/question/${P(103)}/2`, { account: tenantUser(), setup: (m) => seed(m) });
    expect(await screen.findByText('Bạn là tác giả của phiên bản này nên không thể duyệt.')).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Duyệt' })).toBeNull();
  });

  it('AC-14: bulk approval — one Idempotency-Key per item, result per item, own version refused', async () => {
    const { backend } = await renderPublishing('/content-studio/reviews', { account: tenantUser(), setup: (m) => seed(m) });
    await screen.findByText('Câu 3');
    const table = screen.getByText('Câu 1').closest('table')!;
    fireEvent.click(within(table).getAllByRole('checkbox')[0]!); // select all
    fireEvent.click(screen.getByRole('button', { name: 'Duyệt hàng loạt (3)' }));
    const dialog = await screen.findByRole('dialog', { name: 'Duyệt 3 nội dung' });
    fireEvent.click(within(dialog).getByRole('button', { name: 'Duyệt 3 nội dung' }));
    const results = within(dialog).getByTestId('bulk-results');
    await waitFor(() => expect(results.querySelectorAll('[data-result="ok"]')).toHaveLength(2));
    await waitFor(() => expect(results.querySelectorAll('[data-result="error"]')).toHaveLength(1));
    expect(within(results).getByText('Bạn là tác giả của phiên bản này nên không thể duyệt.')).toBeInTheDocument();
    const calls = backend.requests.filter((r) => r.method === 'POST' && r.path.endsWith('/versions/2:transition'));
    expect(calls).toHaveLength(3);
    expect(new Set(calls.map((c) => c.headers['idempotency-key'])).size).toBe(3);
    expect(calls.every((c) => (c.body as { action: string }).action === 'approve' && !!c.headers['if-match'])).toBe(true);
    // retry: only the failed item is sent again, with the same key
    await waitFor(() => expect(within(dialog).getByRole('button', { name: 'Thử lại mục chưa duyệt' })).toBeEnabled());
    fireEvent.click(within(dialog).getByRole('button', { name: 'Thử lại mục chưa duyệt' }));
    await waitFor(() => expect(backend.requests.filter((r) => r.method === 'POST' && r.path.endsWith('/versions/2:transition'))).toHaveLength(4));
    const again = backend.requests.filter((r) => r.method === 'POST' && r.path.endsWith('/versions/2:transition'));
    expect(again[3]!.headers['idempotency-key']).toBe(calls.find((c) => c.path.includes(P(103)))!.headers['idempotency-key']);
  });

  it('AC-14: more than content_review_bulk_max (50) selected → refused before any call', async () => {
    const { backend } = await renderPublishing('/content-studio/reviews', { account: tenantUser(), setup: (m) => seed(m, 51) });
    await screen.findByText('Câu 51');
    const table = screen.getByText('Câu 1').closest('table')!;
    fireEvent.click(within(table).getAllByRole('checkbox')[0]!);
    fireEvent.click(screen.getByRole('button', { name: 'Duyệt hàng loạt (51)' }));
    const dialog = await screen.findByRole('dialog', { name: 'Duyệt 51 nội dung' });
    expect(within(dialog).getByText('Mỗi lần duyệt tối đa 50 nội dung.')).toBeInTheDocument();
    expect(within(dialog).getByRole('button', { name: 'Duyệt 51 nội dung' })).toBeDisabled();
    expect(backend.requests.filter((r) => r.path.endsWith(':transition'))).toHaveLength(0);
  });

  it('AC-30: below 768 px the queue is a list of cards and a task can still be decided', async () => {
    await renderPublishing('/content-studio/reviews', { account: tenantUser(), width: 390, setup: (m) => seed(m) });
    expect(await screen.findByTestId('responsive-cards')).toBeInTheDocument();
    fireEvent.click(await screen.findByRole('button', { name: 'Câu 2' }));
    expect(await screen.findByRole('button', { name: 'Duyệt' })).toBeInTheDocument();
    expect(screen.getByText('Tác vụ này cần màn hình lớn hơn')).toBeInTheDocument(); // the diff only
  });
});
