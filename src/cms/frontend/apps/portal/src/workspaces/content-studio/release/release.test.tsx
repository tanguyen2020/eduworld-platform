// F054 T015–T019, T021, T022, T024, T067 (integration of the whole portal with the MOCK backend; Playwright with the
// real backend is pending like F051/F052 — OQ F051-9): release management of one item.
//   AC-01 versions table + release timeline · AC-02 published version read only + "Tạo bản nháp" · AC-09 no edit /
//   delete of a published version · AC-03 diff (answers only with review) · AC-04 scheduled publish ≥ min lead, ICT →
//   UTC, cancel · AC-11 one Idempotency-Key per intent · AC-05/06 rollback (reason, 2nd confirmation, impact,
//   ROLLBACK_TARGET_INVALID, hidden without content.release.rollback) · AC-07/19 unpublish with usages · AC-08 archive ·
//   AC-31 action turned off / 403 FEATURE_DISABLED · AC-29 reason maxLength.
import { fireEvent, screen, waitFor, within } from '@testing-library/react';
import { afterEach, describe, expect, it } from 'vitest';
import { P, problem, renderPublishing, stopPortals, tenantUser, TENANT_CONTENT_PERMS, type MockPublishing } from '../../../test/publishing-harness';

afterEach(() => stopPortals());

const QS = P(1);
const url = `/content-studio/question-sets/${QS}`;
const base = `/cms/learning/content/question_set/${QS}`;

function seed(m: MockPublishing, tenant: string | null = null) {
  m.add({
    id: QS,
    type: 'question_set',
    tenant_id: tenant,
    title: { vi: 'Phép nhân 3' },
    current: 2,
    usages: [{ ref_type: 'assignment', ref_id: P(60), title: 'Bài giao 3A', used_version: 2, pinned_sessions: 4 }],
    versions: [
      { status: 'published', published_at: '2026-11-01T02:00:00Z', change_note: 'Bản đầu' },
      { status: 'published', published_at: '2026-11-10T02:00:00Z', pinned: 7, content: { definition: { kind: 'static', purpose: 'exercise', items: [] } } },
      { status: 'approved', row: 3, content: { definition: { kind: 'static', purpose: 'exercise', items: [] } } },
      { status: 'draft', content: { definition: { kind: 'static', purpose: 'exercise', items: [] } } },
    ],
  });
  m.releases.set(QS, [{ release_id: P(70), item_type: 'question_set', item_id: QS, action: 'publish', from_version: 1, to_version: 2, released_by: P(5), released_at: '2026-11-10T02:00:00Z', status: 'applied', reason: 'Sửa lỗi chính tả' }]);
}

const history = async () => {
  const h = await screen.findByTestId('version-history');
  await within(h).findByText('v4');
  return h;
};
const rowOf = (h: HTMLElement, v: number) => within(h).getByText(`v${v}`).closest('tr') as HTMLElement;

describe('Phiên bản và phát hành (F054 US1)', () => {
  it('AC-01/AC-02/AC-09: versions with status tags and current pointer; timeline; published = read only with "Tạo bản nháp"', async () => {
    const { backend } = await renderPublishing(url, { setup: (m) => seed(m) });
    const h = await history();
    expect(within(rowOf(h, 2)).getByText('Học sinh đang nhận')).toBeInTheDocument();
    expect(within(rowOf(h, 1)).getByText('Bản đầu')).toBeInTheDocument();
    expect(within(rowOf(h, 3)).getAllByText('Đã duyệt').length).toBeGreaterThan(0);
    // no edit / delete action on a version row
    expect(within(h).queryByRole('button', { name: /^Sửa|^Xóa/ })).toBeNull();
    fireEvent.click(within(rowOf(h, 1)).getByRole('button', { name: 'Xem' }));
    const view = await screen.findByTestId('version-view');
    expect(within(view).getByText(/chỉ đọc: không sửa, không xóa/)).toBeInTheDocument();
    expect(within(view).queryByRole('button', { name: /Sửa|Xóa/ })).toBeNull();
    fireEvent.click(within(view).getByRole('button', { name: 'Tạo bản nháp từ phiên bản v1' }));
    await waitFor(() => expect(backend.calls('POST', `/cms/learning/question-sets/${QS}/versions`)).toHaveLength(1));
    expect(backend.calls('POST', `/cms/learning/question-sets/${QS}/versions`)[0]!.body).toEqual({ from_version: 1 });
    // release timeline with who / reason
    fireEvent.click(screen.getByRole('tab', { name: 'Phát hành' }));
    const tl = await screen.findByTestId('release-timeline');
    expect(within(tl).getByText('Xuất bản v2')).toBeInTheDocument();
    expect(within(tl).getByText('Lý do: Sửa lỗi chính tả')).toBeInTheDocument();
  });

  it('AC-03: diff by group; answer changes only with learning_content.review', async () => {
    await renderPublishing(url, { setup: (m) => seed(m) });
    const h = await history();
    fireEvent.click(within(h).getByLabelText('Chọn phiên bản v2'));
    fireEvent.click(within(h).getByLabelText('Chọn phiên bản v1'));
    fireEvent.click(within(h).getByRole('button', { name: 'So sánh 2 phiên bản đã chọn' }));
    const diff = await screen.findByTestId('version-diff');
    expect(within(diff).getByText('Đáp án (1)')).toBeInTheDocument();
    expect(within(diff).getByText('Văn bản (1)')).toBeInTheDocument();
    expect(within(diff).getByText('/answer_key/correct_option_id')).toBeInTheDocument();
    stopPortals();
    document.body.innerHTML = '';
    const noReview = TENANT_CONTENT_PERMS.filter((p) => p !== 'learning_content.review');
    await renderPublishing(url, { account: tenantUser(noReview), setup: (m) => seed(m, '0193a4d2-0000-7000-8000-00000000000a') });
    const h2 = await history();
    fireEvent.click(within(h2).getByLabelText('Chọn phiên bản v2'));
    fireEvent.click(within(h2).getByLabelText('Chọn phiên bản v1'));
    fireEvent.click(within(h2).getByRole('button', { name: 'So sánh 2 phiên bản đã chọn' }));
    const diff2 = await screen.findByTestId('version-diff');
    expect(within(diff2).getByText(/chỉ hiển thị cho người có quyền duyệt/)).toBeInTheDocument();
    expect(within(diff2).queryByText(/^Đáp án/)).toBeNull();
    expect(within(diff2).queryByText('/answer_key/correct_option_id')).toBeNull();
  });

  it('AC-04/AC-11: schedule ≥ now + min lead (ICT shown, UTC sent), summary diff, cancel; publish twice → one release', async () => {
    const { backend, mock } = await renderPublishing(url, { setup: (m) => seed(m) });
    const h = await history();
    fireEvent.click(within(rowOf(h, 3)).getByRole('button', { name: 'Xuất bản' }));
    const dialog = await screen.findByRole('dialog', { name: 'Xuất bản phiên bản v3' });
    expect(await within(dialog).findByText(/So với v2: 1 thay đổi nội dung, 1 thay đổi đáp án, 1 thay đổi media, 1 thay đổi thông tin/)).toBeInTheDocument();
    fireEvent.click(within(dialog).getByLabelText('Hẹn giờ'));
    const input = await within(dialog).findByLabelText('Thời điểm xuất bản (giờ Việt Nam)');
    // too early: 1 minute from now (ICT wall clock)
    const ict = (d: Date) => {
      const x = new Date(d.getTime() + 7 * 3600_000);
      const p = (n: number) => String(n).padStart(2, '0');
      return `${p(x.getUTCDate())}/${p(x.getUTCMonth() + 1)}/${x.getUTCFullYear()} ${p(x.getUTCHours())}:${p(x.getUTCMinutes())}`;
    };
    const typeTime = (d: Date) => {
      fireEvent.mouseDown(input);
      fireEvent.change(input, { target: { value: ict(d) } });
      fireEvent.keyDown(input, { key: 'Enter', code: 'Enter' });
      fireEvent.blur(input);
    };
    typeTime(new Date(Date.now() + 60_000));
    fireEvent.click(within(dialog).getByRole('button', { name: 'Hẹn giờ xuất bản' }));
    expect(await within(dialog).findByRole('alert')).toHaveTextContent(/Chọn thời điểm từ/);
    expect(backend.calls('POST', `${base}/versions/3:publish`)).toHaveLength(0);
    const later = new Date(Math.ceil((Date.now() + 2 * 86_400_000) / 60_000) * 60_000);
    typeTime(later);
    fireEvent.click(within(dialog).getByRole('button', { name: 'Hẹn giờ xuất bản' }));
    await waitFor(() => expect(backend.calls('POST', `${base}/versions/3:publish`)).toHaveLength(1));
    const call = backend.calls('POST', `${base}/versions/3:publish`)[0]!;
    expect((call.body as { publish_at: string }).publish_at).toBe(later.toISOString());
    expect(call.headers['if-match']).toBe('W/"3"');
    expect(call.headers['idempotency-key']).toMatch(/^[0-9a-f-]{36}$/);
    // timeline: scheduled → cancel
    fireEvent.click(screen.getByRole('tab', { name: 'Phát hành' }));
    const tl = await screen.findByTestId('release-timeline');
    fireEvent.click(await within(tl).findByRole('button', { name: 'Hủy hẹn giờ' }));
    await waitFor(() => expect(document.querySelector('.ant-popconfirm')).not.toBeNull());
    fireEvent.click(within(document.querySelector('.ant-popconfirm') as HTMLElement).getByRole('button', { name: 'Hủy hẹn giờ' }));
    await waitFor(() => expect(backend.requests.filter((r) => r.method === 'POST' && /releases\/[^/]+:cancel$/.test(r.path))).toHaveLength(1));
    expect(mock.created).toBe(1);
  });

  it('AC-11: "Xuất bản ngay" retried after a network failure reuses the Idempotency-Key → one release', async () => {
    const { backend, mock } = await renderPublishing(url, {
      setup: (m) => {
        seed(m);
        m.fail.push({ method: 'POST', path: /versions\/3:publish$/, response: () => problem(503, 'UNAVAILABLE'), once: true });
      },
    });
    const h = await history();
    fireEvent.click(within(rowOf(h, 3)).getByRole('button', { name: 'Xuất bản' }));
    const dialog = await screen.findByRole('dialog', { name: 'Xuất bản phiên bản v3' });
    fireEvent.click(within(dialog).getByRole('button', { name: 'Xuất bản ngay' }));
    await waitFor(() => expect(backend.calls('POST', `${base}/versions/3:publish`).length).toBeGreaterThanOrEqual(2), { timeout: 60_000 });
    const keys = new Set(backend.calls('POST', `${base}/versions/3:publish`).map((c) => c.headers['idempotency-key']));
    expect(keys.size).toBe(1);
    await waitFor(() => expect(mock.created).toBe(1));
  });

  it('AC-05/AC-06/AC-29: rollback — published targets only, reason ≤ 500, impact + 2nd confirmation, ROLLBACK_TARGET_INVALID shown', async () => {
    const { backend } = await renderPublishing(url, {
      setup: (m) => {
        seed(m);
        m.fail.push({ method: 'POST', path: /:rollback$/, response: () => problem(422, 'ROLLBACK_TARGET_INVALID'), once: true });
      },
    });
    const h = await history();
    fireEvent.click(within(h).getByRole('button', { name: 'Rollback…' }));
    const dialog = await screen.findByRole('dialog', { name: 'Dùng lại một phiên bản đã xuất bản' });
    const reason = within(dialog).getByLabelText('Lý do');
    expect(reason).toHaveAttribute('maxlength', '500');
    fireEvent.mouseDown(within(dialog).getByRole('combobox'));
    expect(await screen.findByText('v2 (đang dùng)')).toBeInTheDocument();
    expect(screen.getByText('v2 (đang dùng)').closest('.ant-select-item')).toHaveClass('ant-select-item-option-disabled');
    fireEvent.click(screen.getByText(/^v1 — xuất bản/));
    fireEvent.click(within(dialog).getByRole('button', { name: 'Tiếp tục' }));
    expect(await within(dialog).findByText('Bắt buộc nhập lý do.')).toBeInTheDocument();
    fireEvent.change(reason, { target: { value: 'v2 sai đáp án câu 3' } });
    fireEvent.click(within(dialog).getByRole('button', { name: 'Tiếp tục' }));
    const impact = await within(dialog).findByTestId('rollback-impact');
    expect(impact).toHaveTextContent('Lượt đang làm (khoảng 7) giữ phiên bản v2');
    expect(impact).toHaveTextContent('Lượt bắt đầu sau khi xác nhận nhận phiên bản v1');
    fireEvent.click(within(dialog).getByRole('button', { name: 'Xác nhận rollback' }));
    expect(await within(dialog).findByText(/không dùng lại được/)).toBeInTheDocument();
    fireEvent.click(within(dialog).getByRole('button', { name: 'Xác nhận rollback' }));
    await waitFor(() => expect(backend.calls('POST', `${base}:rollback`)).toHaveLength(2));
    const calls = backend.calls('POST', `${base}:rollback`);
    expect(calls[1]!.body).toEqual({ to_version: 1, reason: 'v2 sai đáp án câu 3' });
    expect(calls[1]!.headers['idempotency-key']).toBe(calls[0]!.headers['idempotency-key']);
    await waitFor(() => expect(screen.queryByRole('dialog', { name: 'Dùng lại một phiên bản đã xuất bản' })).toBeNull());
  });

  it('AC-06/AC-28: no Rollback / Unpublish without content.release.rollback / learning_content.publish', async () => {
    const perms = TENANT_CONTENT_PERMS.filter((p) => p !== 'content.release.rollback' && p !== 'learning_content.publish');
    await renderPublishing(url, { account: tenantUser(perms), setup: (m) => seed(m, '0193a4d2-0000-7000-8000-00000000000a') });
    const h = await history();
    expect(within(h).queryByRole('button', { name: 'Rollback…' })).toBeNull();
    expect(within(h).queryByRole('button', { name: 'Gỡ xuất bản' })).toBeNull();
    expect(within(h).queryByRole('button', { name: /^Rollback về/ })).toBeNull();
    expect(within(h).queryByRole('button', { name: 'Xuất bản' })).toBeNull();
  });

  it('AC-07/AC-19: unpublish lists where the item is used and needs a reason; AC-08 archive a non-current version', async () => {
    const { backend } = await renderPublishing(url, { setup: (m) => seed(m) });
    const h = await history();
    fireEvent.click(within(h).getByRole('button', { name: 'Gỡ xuất bản' }));
    const dialog = await screen.findByRole('dialog', { name: 'Gỡ xuất bản nội dung' });
    expect(await within(dialog).findByText(/Nội dung đang được 1 nơi tham chiếu, 4 lượt đang dùng/)).toBeInTheDocument();
    expect(within(dialog).getByText(/Bài giao 3A/)).toBeInTheDocument();
    fireEvent.click(within(dialog).getByRole('button', { name: 'Gỡ xuất bản' }));
    expect(await within(dialog).findByText('Bắt buộc nhập lý do.')).toBeInTheDocument();
    fireEvent.change(within(dialog).getByLabelText('Lý do'), { target: { value: 'Ngừng dùng' } });
    fireEvent.click(within(dialog).getByRole('button', { name: 'Gỡ xuất bản' }));
    await waitFor(() => expect(backend.calls('POST', `${base}:unpublish`)).toHaveLength(1));
    expect(backend.calls('POST', `${base}:unpublish`)[0]!.body).toEqual({ reason: 'Ngừng dùng' });
    // archive v1 (not the current pointer)
    const h2 = await history();
    await waitFor(() => expect(within(rowOf(h2, 1)).getByRole('button', { name: 'Lưu trữ' })).toBeInTheDocument());
    fireEvent.click(within(rowOf(h2, 1)).getByRole('button', { name: 'Lưu trữ' }));
    const ad = await screen.findByRole('dialog', { name: 'Lưu trữ phiên bản v1' });
    fireEvent.change(within(ad).getByLabelText('Lý do'), { target: { value: 'Cũ' } });
    fireEvent.click(within(ad).getByRole('button', { name: 'Lưu trữ' }));
    await waitFor(() => expect(backend.calls('POST', `${base}/versions/1:transition`)).toHaveLength(1));
    const tr = backend.calls('POST', `${base}/versions/1:transition`)[0]!;
    expect(tr.body).toEqual({ action: 'archive', note: 'Cũ' });
    expect(tr.headers['if-match']).toBe('W/"1"');
  });

  it('AC-31 (TC-CMS-MENU-02): rollback action turned off → buttons disabled; 403 FEATURE_DISABLED → message + permissions reloaded', async () => {
    const { backend } = await renderPublishing(url, {
      disabledActions: ['action.content_studio.question_sets.rollback'],
      setup: (m) => {
        seed(m);
        m.fail.push({ method: 'POST', path: /versions\/3:publish$/, response: () => problem(403, 'FEATURE_DISABLED', { feature_key: 'action.content_studio.question_sets.publish' }) });
      },
    });
    const h = await history();
    expect(within(h).getByRole('button', { name: 'Rollback…' })).toBeDisabled();
    expect(within(h).getByRole('button', { name: 'Gỡ xuất bản' })).toBeDisabled();
    const before = backend.calls('GET', '/me/permissions').length;
    fireEvent.click(within(rowOf(h, 3)).getByRole('button', { name: 'Xuất bản' }));
    const dialog = await screen.findByRole('dialog', { name: 'Xuất bản phiên bản v3' });
    fireEvent.click(within(dialog).getByRole('button', { name: 'Xuất bản ngay' }));
    expect(await within(dialog).findByText('Tính năng này đang tạm tắt.')).toBeInTheDocument();
    await waitFor(() => expect(backend.calls('GET', '/me/permissions').length).toBeGreaterThan(before));
  });

  it("AC-15/TC-TEN-001: another tenant's item → Không tìm thấy (no version data)", async () => {
    await renderPublishing(url, { account: tenantUser(), setup: (m) => seed(m, '0193a4d2-0000-7000-8000-00000000000c') });
    expect(await screen.findByText('Không tìm thấy nội dung')).toBeInTheDocument();
    expect(screen.queryByTestId('version-history')).toBeNull();
  });
});
