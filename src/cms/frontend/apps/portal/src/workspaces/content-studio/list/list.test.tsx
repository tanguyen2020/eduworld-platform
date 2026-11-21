// F054 T051–T053, T055, T056 (whole portal + MOCK backend; Playwright with the real backend pending — OQ F051-9):
//   AC-24 list + search (`q` without diacritics by the server), filters → query, Tenant column + tenant_ids for the
//   System Admin · AC-25 soft delete with If-Match, reason required once published, row disappears, no "show deleted" ·
//   AC-26 tenant Thùng rác: filter by type, restore with reason, restorable = false / 409 · AC-28 Import / Thùng rác
//   hidden without permission · AC-30 cards below 768 px · OQ F054-2 "Ngôn ngữ" / "Trạng thái dịch" filters.
import { fireEvent, screen, waitFor, within } from '@testing-library/react';
import { afterEach, describe, expect, it } from 'vitest';
import { contentAccount, P, renderPublishing, stopPortals, TENANTS, tenantUser, TENANT_CONTENT_PERMS, type MockPublishing } from '../../../test/publishing-harness';

afterEach(() => stopPortals());

const A = TENANTS.thDemoA.tenant_id;

function seed(m: MockPublishing, tenant: string | null = A) {
  m.add({ id: P(301), type: 'question', tenant_id: tenant, title: { vi: 'Phân số bằng nhau' }, current: 1, versions: [{ status: 'published', content: { type: 'single_choice' } }] });
  m.add({ id: P(302), type: 'question', tenant_id: tenant, title: { vi: 'Phép cộng' }, versions: [{ status: 'draft', content: { type: 'numeric' } }] });
  m.add({ id: P(303), type: 'question', tenant_id: TENANTS.thcsDemoB.tenant_id, title: { vi: 'Câu trường B' }, versions: [{ status: 'draft' }] });
}

describe('Danh sách, tìm kiếm, xóa mềm (F054 US5)', () => {
  it('AC-24: search without diacritics and filters build the query; no tenant_ids for a tenant user; no "show deleted"', async () => {
    const { backend } = await renderPublishing('/content-studio/question-bank', { account: tenantUser(), setup: (m) => seed(m) });
    expect(await screen.findByText('Phân số bằng nhau')).toBeInTheDocument();
    expect(screen.queryByText('Câu trường B')).toBeNull();
    const search = screen.getByLabelText('Tìm theo từ khóa (không cần dấu)');
    expect(search).toHaveAttribute('maxlength', '200');
    fireEvent.change(search, { target: { value: 'phan so' } });
    fireEvent.keyDown(search, { key: 'Enter', code: 'Enter' });
    await waitFor(() => expect(backend.calls('GET', '/cms/learning/questions').some((r) => new URLSearchParams(r.search).get('q') === 'phan so')).toBe(true));
    expect(await screen.findByText('Phân số bằng nhau')).toBeInTheDocument();
    expect(screen.queryByText('Phép cộng')).toBeNull();
    fireEvent.mouseDown(screen.getByRole('combobox', { name: 'Trạng thái' }));
    fireEvent.click(await screen.findByTitle('Đã xuất bản'));
    await waitFor(() => expect(backend.calls('GET', '/cms/learning/questions').some((r) => new URLSearchParams(r.search).get('status') === 'published' && new URLSearchParams(r.search).get('q') === 'phan so')).toBe(true));
    expect(backend.calls('GET', '/cms/learning/questions').every((r) => !r.search.includes('tenant_ids'))).toBe(true);
    expect(screen.queryByText(/đã xóa/i)).toBeNull();
  });

  it('AC-24 (OQ F054-2): "Ngôn ngữ" and "Trạng thái dịch" filters → content_language / translation_status', async () => {
    const { backend } = await renderPublishing('/content-studio/question-bank', {
      account: tenantUser(),
      setup: (m) => {
        seed(m);
        m.add({ id: P(304), type: 'question', tenant_id: A, lang: 'en', title: { en: 'Fractions' }, versions: [{ status: 'draft', translation_status: 'partial' }] });
        m.add({ id: P(305), type: 'question', tenant_id: A, lang: 'en', title: { en: 'Addition' }, versions: [{ status: 'draft', translation_status: 'complete' }] });
      },
    });
    expect(await screen.findByText('Phân số bằng nhau')).toBeInTheDocument();
    fireEvent.mouseDown(screen.getByRole('combobox', { name: 'Ngôn ngữ' }));
    fireEvent.click(await screen.findByTitle('Tiếng Anh'));
    await waitFor(() => expect(backend.calls('GET', '/cms/learning/questions').some((r) => new URLSearchParams(r.search).get('content_language') === 'en')).toBe(true));
    await waitFor(() => expect(screen.queryByText('Phân số bằng nhau')).toBeNull());
    expect(await screen.findByText('Addition')).toBeInTheDocument();
    fireEvent.mouseDown(screen.getByRole('combobox', { name: 'Trạng thái dịch' }));
    fireEvent.click(await screen.findByTitle('Thiếu bản dịch'));
    await waitFor(() => expect(backend.calls('GET', '/cms/learning/questions').some((r) => new URLSearchParams(r.search).get('translation_status') === 'partial' && new URLSearchParams(r.search).get('content_language') === 'en')).toBe(true));
    await waitFor(() => expect(screen.queryByText('Addition')).toBeNull());
    expect(screen.getByText('Fractions')).toBeInTheDocument();
  });

  it('AC-24 (OQ F054-2): the same two filters on the lesson list', async () => {
    const { backend } = await renderPublishing('/content-studio/lessons', {
      account: tenantUser(),
      setup: (m) => {
        m.add({ id: P(311), type: 'lesson', tenant_id: A, title: { vi: 'Bài nhân 2' }, versions: [{ status: 'draft', translation_status: 'outdated' }] });
        m.add({ id: P(312), type: 'lesson', tenant_id: A, title: { vi: 'Bài nhân 3' }, versions: [{ status: 'draft', translation_status: 'complete' }] });
      },
    });
    expect(await screen.findByText('Bài nhân 3')).toBeInTheDocument();
    fireEvent.mouseDown(screen.getByRole('combobox', { name: 'Trạng thái dịch' }));
    fireEvent.click(await screen.findByTitle('Bản dịch lỗi thời'));
    await waitFor(() => expect(backend.calls('GET', '/cms/learning/lessons').some((r) => new URLSearchParams(r.search).get('translation_status') === 'outdated')).toBe(true));
    await waitFor(() => expect(screen.queryByText('Bài nhân 3')).toBeNull());
    expect(screen.getByText('Bài nhân 2')).toBeInTheDocument();
  });

  it('AC-24 (TC-CMS-TEN-02): System Admin — Tenant column, tenant filter → tenant_ids[]', async () => {
    const { backend } = await renderPublishing('/content-studio/question-bank', { as: 'sa.admin', setup: (m) => seed(m) });
    expect(await screen.findByText('Câu trường B')).toBeInTheDocument();
    expect(screen.getAllByRole('columnheader').map((c) => c.textContent)).toContain('Tenant');
    expect(backend.calls('GET', '/cms/learning/questions')[0]!.headers['x-tenant-id']).toBeUndefined();
  });

  it('AC-25: delete a published item needs a reason, sends If-Match, then the row is gone (soft delete)', async () => {
    const { backend, mock } = await renderPublishing('/content-studio/question-bank', { account: tenantUser(), setup: (m) => seed(m) });
    const row = (await screen.findByText('Phân số bằng nhau')).closest('tr')!;
    fireEvent.click(within(row).getByRole('button', { name: 'Xóa' }));
    const dialog = await screen.findByRole('dialog', { name: 'Xóa nội dung' });
    expect(within(dialog).getByText(/đã từng xuất bản: bắt buộc nhập lý do/)).toBeInTheDocument();
    fireEvent.click(within(dialog).getByRole('button', { name: 'Xóa' }));
    expect(await within(dialog).findByText('Bắt buộc nhập lý do.')).toBeInTheDocument();
    fireEvent.change(within(dialog).getByLabelText('Lý do'), { target: { value: 'Trùng câu khác' } });
    fireEvent.click(within(dialog).getByRole('button', { name: 'Xóa' }));
    await waitFor(() => expect(backend.calls('DELETE', `/cms/learning/questions/${P(301)}`)).toHaveLength(1));
    const del = backend.calls('DELETE', `/cms/learning/questions/${P(301)}`)[0]!;
    expect(del.headers['if-match']).toBe('W/"1"');
    expect(del.body).toEqual({ reason: 'Trùng câu khác' });
    await waitFor(() => expect(screen.queryByText('Phân số bằng nhau')).toBeNull());
    expect(mock.trash).toHaveLength(1);
  });

  it('AC-28: no Import / Tạo mới / Xóa without learning_content.import / write; AC-30 cards < 768 px', async () => {
    const perms = TENANT_CONTENT_PERMS.filter((p) => p !== 'learning_content.import' && p !== 'learning_content.write');
    await renderPublishing('/content-studio/question-bank', { account: tenantUser(perms), width: 390, setup: (m) => seed(m) });
    expect(await screen.findByTestId('responsive-cards')).toBeInTheDocument();
    expect(await screen.findByRole('button', { name: 'Phân số bằng nhau' })).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Nhập từ Excel/CSV' })).toBeNull();
    expect(screen.queryByRole('button', { name: 'Tạo mới' })).toBeNull();
    expect(screen.queryByRole('button', { name: 'Xóa' })).toBeNull();
  });
});

describe('Thùng rác của tenant (F054-AC-26)', () => {
  function trash(m: MockPublishing) {
    m.trash.push(
      { resource_type: 'question', resource_id: P(401), tenant_id: A, title: 'Câu đã xóa', deleted_at: '2026-11-19T01:00:00Z', deleted_by: P(9), delete_reason: 'Nhầm', restorable: true },
      { resource_type: 'question', resource_id: P(402), tenant_id: A, title: 'Câu trùng mã', deleted_at: '2026-11-19T01:00:00Z', deleted_by: P(9), restorable: false },
      { resource_type: 'question', resource_id: P(403), tenant_id: A, title: 'Câu xung đột', deleted_at: '2026-11-19T01:00:00Z', deleted_by: P(9), restorable: true, conflict: true },
      { resource_type: 'lesson', resource_id: P(404), tenant_id: A, title: 'Bài đã xóa', deleted_at: '2026-11-19T01:00:00Z', deleted_by: P(9), restorable: true },
    );
  }

  it('lists by resource type, restores with a reason (3–1000) and Idempotency-Key; restorable=false and 409 explained', async () => {
    const { backend } = await renderPublishing('/school-lms/trash', { account: tenantUser(), setup: trash });
    expect(await screen.findByText('Câu đã xóa')).toBeInTheDocument();
    expect(screen.queryByText('Bài đã xóa')).toBeNull();
    expect(backend.calls('GET', '/tenant/trash')[0]!.search).toContain('resource_type=question');
    const blocked = screen.getByText('Câu trùng mã').closest('tr')!;
    expect(within(blocked).getByRole('button', { name: 'Khôi phục' })).toBeDisabled();
    expect(within(blocked).getByText(/Đã có bản ghi mới cùng mã/)).toBeInTheDocument();
    fireEvent.click(within(screen.getByText('Câu đã xóa').closest('tr')!).getByRole('button', { name: 'Khôi phục' }));
    const dialog = await screen.findByRole('dialog');
    const reason = within(dialog).getByRole('textbox');
    expect(reason).toHaveAttribute('maxlength', '1000');
    fireEvent.change(reason, { target: { value: 'ab' } });
    fireEvent.click(within(dialog).getByRole('button', { name: /Xác nhận|OK|Đồng ý/ }));
    expect(await within(dialog).findByText(/Tối thiểu 3/)).toBeInTheDocument();
    fireEvent.change(reason, { target: { value: 'Xóa nhầm' } });
    fireEvent.click(within(dialog).getByRole('button', { name: /Xác nhận|OK|Đồng ý/ }));
    await waitFor(() => expect(backend.calls('POST', `/tenant/trash/question/${P(401)}:restore`)).toHaveLength(1));
    const call = backend.calls('POST', `/tenant/trash/question/${P(401)}:restore`)[0]!;
    expect(call.body).toEqual({ reason: 'Xóa nhầm' });
    expect(call.headers['idempotency-key']).toBeTruthy();
    await waitFor(() => expect(screen.queryByText('Câu đã xóa')).toBeNull());
    fireEvent.click(within(screen.getByText('Câu xung đột').closest('tr')!).getByRole('button', { name: 'Khôi phục' }));
    const d2 = await screen.findByRole('dialog');
    fireEvent.change(within(d2).getByRole('textbox'), { target: { value: 'Khôi phục lại' } });
    fireEvent.click(within(d2).getByRole('button', { name: /Xác nhận|OK|Đồng ý/ }));
    expect(await within(d2).findByText(/Đã có bản ghi mới cùng mã/)).toBeInTheDocument();
  });

  it('AC-28: no Thùng rác menu without tenant.trash.read; restore hidden without tenant.trash.restore', async () => {
    const noRead = TENANT_CONTENT_PERMS.filter((p) => !p.startsWith('tenant.trash.'));
    await renderPublishing('/school-lms/students', { account: tenantUser(noRead) });
    await screen.findAllByText(/Học sinh theo khối/);
    expect(screen.queryByRole('menuitem', { name: /Thùng rác/ })).toBeNull();
    stopPortals();
    document.body.innerHTML = '';
    const readOnly = TENANT_CONTENT_PERMS.filter((p) => p !== 'tenant.trash.restore');
    await renderPublishing('/school-lms/trash', { account: contentAccount('qlcm.a', TENANTS.thDemoA, readOnly), setup: trash });
    expect(await screen.findByText('Câu đã xóa')).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Khôi phục' })).toBeNull();
  });
});
