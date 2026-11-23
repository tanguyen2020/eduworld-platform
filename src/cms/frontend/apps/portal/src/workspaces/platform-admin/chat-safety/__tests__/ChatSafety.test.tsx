// F122 T018–T020, T022–T024, T051 — workspace D "An toàn chat" and "Công tắc kiểm duyệt". Backend = MOCK
// (test/chat-safety-mock.ts, test only, AGENTS R17); catalogue fixtures are placeholders (`zq…`, R15).
// F122-AC-07 CRUD category (soft delete with confirmation), terms (category filter, variants ≤ 50, RE2 warning), allowlist;
// F122-AC-08 "Thử chuỗi" (decision, masked text, categories, highlight on the typed text only), publish needs a summary,
// versions with regression; F122-AC-09 without edit/test/publish actions: read only, no term in storage or console;
// F122-AC-10 platform + filtered tenants, 4 switchable steps, PII/grooming "Luôn bật", step locked by the platform;
// F122-AC-11 risk confirmation, reason ≥ 10, expires_at required, X-Tenant-Id of the target tenant,
// MODERATION_SWITCH_REASON_REQUIRED on the reason field; F122-AC-12 history; F122-AC-27 "Chờ phê duyệt", approve by
// another System Admin, hidden for the proposer, error codes.
import { cleanup, configure, fireEvent, screen, waitFor, within } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { CS, renderSafety, T_A, T_B } from '../../../../test/chat-safety-mock';
import { problem, stopPortals } from '../../../../test/portal-harness';
import { tenantFilterStore } from '../../../../shell/tenantFilterStore';

vi.setConfig({ testTimeout: 900_000 }); // whole-portal screens on a shared, throttled CPU (other agents run tests too)
configure({ asyncUtilTimeout: 90_000 });

afterEach(() => {
  stopPortals();
  tenantFilterStore.reset();
});

async function pick(user: { click: (e: Element) => Promise<void> }, label: string, option: string, within_?: HTMLElement) {
  await user.click((within_ ? within(within_) : screen).getByRole('combobox', { name: label }));
  await user.click((await screen.findAllByTitle(option)).at(-1)!);
}

describe('Danh mục từ nhạy cảm (F122-AC-07, AC-08)', () => {
  it('categories: create, edit, soft delete after confirmation; terms with variants and RE2 warning; allowlist', async () => {
    const { backend, user, state } = await renderSafety('/platform-admin/chat-safety');
    expect(await screen.findByText('Xúc phạm')).toBeInTheDocument();
    // create a category (code pattern + limits)
    await user.click(screen.getByRole('button', { name: 'Thêm nhóm từ' }));
    let dialog = await screen.findByRole('dialog', { name: 'Thêm nhóm từ' });
    expect(within(dialog).getByLabelText('Mã')).toHaveAttribute('maxlength', '64');
    expect(within(dialog).getByLabelText('Tên nhóm từ')).toHaveAttribute('maxlength', '120');
    fireEvent.change(within(dialog).getByLabelText('Mã'), { target: { value: 'Bad Code' } });
    fireEvent.change(within(dialog).getByLabelText('Tên nhóm từ'), { target: { value: 'Nhóm thử' } });
    await user.click(within(dialog).getByRole('button', { name: 'Lưu' }));
    expect(backend.calls('POST', '/admin/chat/sensitive-categories')).toHaveLength(0);
    fireEvent.change(within(dialog).getByLabelText('Mã'), { target: { value: 'zq_group' } });
    await user.click(within(dialog).getByRole('button', { name: 'Lưu' }));
    await waitFor(() => expect(backend.calls('POST', '/admin/chat/sensitive-categories')).toHaveLength(1));
    expect(backend.calls('POST', '/admin/chat/sensitive-categories')[0]!.body).toMatchObject({ code: 'zq_group', name: 'Nhóm thử', severity: 'medium', default_action: 'mask' });
    expect(backend.calls('POST', '/admin/chat/sensitive-categories')[0]!.headers['idempotency-key']).toBeTruthy();
    // soft delete with confirmation
    const row = (await screen.findByText('Nhóm thử')).closest('tr')!;
    fireEvent.click(within(row).getByRole('button', { name: 'Xóa' }));
    const confirm = await screen.findByRole('dialog', { name: /Xóa nhóm từ "Nhóm thử"/ });
    expect(within(confirm).getByText(/Xóa mềm/)).toBeInTheDocument();
    await user.click(within(confirm).getByRole('button', { name: 'Xóa' }));
    await waitFor(() => expect(backend.calls('DELETE', '/admin/chat/sensitive-categories/0193a4d2-a100-7000-8000-0000000000aa')).toHaveLength(1));
    // terms: category filter, add with variants and the RE2 warning for a pattern
    await user.click(screen.getByRole('tab', { name: 'Từ' }));
    expect(await screen.findByText('zqplaceholder')).toBeInTheDocument();
    await pick(user, 'Nhóm từ', 'Xúc phạm');
    await waitFor(() => expect(new URLSearchParams(backend.calls('GET', '/admin/chat/sensitive-terms').at(-1)!.search).get('category_id')).toBe(CS.catInsult));
    await user.click(screen.getByRole('button', { name: 'Thêm từ' }));
    dialog = await screen.findByRole('dialog', { name: 'Thêm từ' });
    await pick(user, 'Nhóm từ', 'Xúc phạm', dialog);
    expect(within(dialog).getByLabelText('Từ / cụm từ')).toHaveAttribute('maxlength', '200');
    fireEvent.change(within(dialog).getByLabelText('Từ / cụm từ'), { target: { value: 'zq[0-9]+' } });
    await pick(user, 'Kiểu khớp', 'Mẫu (RE2)', dialog);
    expect(within(dialog).getByText(/Biểu thức RE2/)).toBeInTheDocument();
    expect(within(dialog).getByText(/tối đa 50 biến thể/)).toBeInTheDocument();
    await user.click(within(dialog).getByRole('button', { name: 'Lưu' }));
    await waitFor(() => expect(backend.calls('POST', '/admin/chat/sensitive-terms')).toHaveLength(1));
    expect(backend.calls('POST', '/admin/chat/sensitive-terms')[0]!.body).toMatchObject({ category_id: CS.catInsult, term: 'zq[0-9]+', match_type: 'pattern', locale: 'vi', enabled: true });
    // allowlist
    await user.click(screen.getByRole('tab', { name: 'Danh sách cho phép' }));
    expect(await screen.findByText('zqok')).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'Thêm cụm từ cho phép' }));
    dialog = await screen.findByRole('dialog', { name: 'Thêm cụm từ cho phép' });
    fireEvent.change(within(dialog).getByLabelText('Cụm từ'), { target: { value: 'zqcho' } });
    fireEvent.change(within(dialog).getByLabelText('Lý do'), { target: { value: 'Tên riêng' } });
    await user.click(within(dialog).getByRole('button', { name: 'Lưu' }));
    await waitFor(() => expect(backend.calls('POST', '/admin/chat/sensitive-allowlist')).toHaveLength(1));
    expect(state.allowlist).toHaveLength(2);
  });

  it('test text: decision, masked text, categories, highlight on the typed text; publish needs a summary; versions with regression', async () => {
    const { backend, user } = await renderSafety('/platform-admin/chat-safety');
    await screen.findByText('Xúc phạm');
    await user.click(screen.getByRole('tab', { name: 'Thử chuỗi' }));
    const input = await screen.findByLabelText('Chuỗi cần thử');
    expect(input).toHaveAttribute('maxlength', '500');
    fireEvent.change(input, { target: { value: 'ban la zqabc nhe' } });
    await user.click(screen.getByRole('button', { name: 'Thử chuỗi' }));
    expect(await screen.findByTestId('tester-masked')).toHaveTextContent('ban la ***** nhe');
    expect(within(screen.getByTestId('tester-highlight')).getByText('zqabc').tagName).toBe('MARK');
    expect(screen.getByText(/insult · mức medium · Che/)).toBeInTheDocument();
    expect(backend.calls('POST', '/admin/chat/sensitive-terms:test')[0]!.body).toEqual({ text: 'ban la zqabc nhe', include_drafts: true });
    // publish: change summary required (5–1000)
    await user.click(screen.getByRole('tab', { name: 'Phiên bản' }));
    expect(await screen.findByText('97.0%')).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'Publish' }));
    const dialog = await screen.findByRole('dialog', { name: 'Publish danh mục' });
    const summary = within(dialog).getByLabelText('Tóm tắt thay đổi');
    expect(summary).toHaveAttribute('maxlength', '1000');
    await user.click(within(dialog).getByRole('button', { name: 'Publish' }));
    expect(backend.calls('POST', '/admin/chat/sensitive-catalog:publish')).toHaveLength(0);
    fireEvent.change(summary, { target: { value: 'Thêm nhóm thử' } });
    await user.click(within(dialog).getByRole('button', { name: 'Publish' }));
    await waitFor(() => expect(backend.calls('POST', '/admin/chat/sensitive-catalog:publish')).toHaveLength(1));
    expect(backend.calls('POST', '/admin/chat/sensitive-catalog:publish')[0]!.body).toEqual({ change_summary: 'Thêm nhóm thử' });
    expect(await screen.findByText('98.0%')).toBeInTheDocument();
  });

  it('F122-AC-09: without the edit/test/publish actions the catalogue is read only; no term in storage or console', async () => {
    const log = vi.spyOn(console, 'log');
    const info = vi.spyOn(console, 'info');
    await renderSafety('/platform-admin/chat-safety', {
      disabledActions: ['action.platform_admin.chat_safety.edit', 'action.platform_admin.chat_safety.test', 'action.platform_admin.chat_safety.publish'],
    });
    expect(await screen.findByText('Xúc phạm')).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Thêm nhóm từ' })).toBeNull();
    expect(screen.queryByRole('button', { name: 'Sửa' })).toBeNull();
    fireEvent.click(screen.getByRole('tab', { name: 'Từ' }));
    expect(await screen.findByText('zqplaceholder')).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Thêm từ' })).toBeNull();
    expect(screen.queryByRole('button', { name: 'Xóa' })).toBeNull();
    fireEvent.click(screen.getByRole('tab', { name: 'Thử chuỗi' }));
    expect(screen.queryByLabelText('Chuỗi cần thử')).toBeNull();
    fireEvent.click(screen.getByRole('tab', { name: 'Phiên bản' }));
    expect(screen.queryByRole('button', { name: 'Publish' })).toBeNull();
    const stored = JSON.stringify({ ...window.localStorage }) + JSON.stringify({ ...window.sessionStorage });
    expect(stored).not.toMatch(/zqplaceholder|zqp1aceholder/);
    const logged = JSON.stringify([...log.mock.calls, ...info.mock.calls]);
    expect(logged).not.toMatch(/zqplaceholder/);
  });
});

describe('Công tắc kiểm duyệt (F122-AC-10, AC-11, AC-12, AC-27)', () => {
  it('tenant rows are cursor pages of the query (owner rule 2026-11-23): page 1, then "Tải thêm" appends page 2 with the cursor', async () => {
    const { state, backend, user } = await renderSafety('/platform-admin/moderation-switches');
    state.switchPageSize = 1;
    tenantFilterStore.set([T_A, T_B]);
    expect(await screen.findByRole('switch', { name: 'Lọc spam — TH Demo A' })).toBeInTheDocument();
    expect(screen.queryByRole('switch', { name: 'Lọc spam — THCS Demo B' })).toBeNull();
    await user.click(screen.getByRole('button', { name: 'Tải thêm' }));
    expect(await screen.findByRole('switch', { name: 'Lọc spam — THCS Demo B' })).toBeInTheDocument();
    expect(screen.getAllByRole('switch', { name: 'Lọc spam — TH Demo A' })).toHaveLength(1);
    const calls = backend.calls('GET', '/admin/chat/moderation-switches').map((r) => new URLSearchParams(r.search).get('cursor'));
    expect(calls).toContain('1');
    expect(screen.queryByRole('button', { name: 'Tải thêm' })).toBeNull();
  });

  it('platform row + filtered tenants; PII and grooming "Luôn bật"; a step off at platform level is locked on tenant rows', async () => {
    const { state, user } = await renderSafety('/platform-admin/moderation-switches');
    tenantFilterStore.set([T_A, T_B]);
    const rowA = (await screen.findByRole('switch', { name: 'Lọc spam — TH Demo A' })).closest('tr')!;
    expect(screen.getByRole('switch', { name: 'Lọc spam — THCS Demo B' })).toBeInTheDocument();
    expect(within(rowA).getByRole('switch', { name: 'Lọc spam — TH Demo A' })).not.toBeChecked();
    expect(within(rowA).getAllByText('Luôn bật')).toHaveLength(2);
    expect(within(rowA).queryByRole('switch', { name: /Thông tin cá nhân/ })).toBeNull();
    expect(screen.getByText(/Bước PII và phát hiện dụ dỗ luôn chạy/)).toBeInTheDocument();
    // the platform turns bullying off → locked on the tenant rows
    state.switches.platform.effective.steps.bullying = false;
    state.switches.platform.disabled_steps = ['bullying'];
    fireEvent.click(screen.getByRole('tab', { name: 'Lịch sử' }));
    fireEvent.click(screen.getByRole('tab', { name: 'Công tắc' }));
    const rowB = (await screen.findAllByText('THCS Demo B'))[0]!.closest('tr')!;
    await waitFor(() => expect(within(rowB).getAllByText('Platform đang tắt').length).toBeGreaterThan(0));
    expect(within(rowB).getByRole('switch', { name: 'Phát hiện bắt nạt — THCS Demo B' })).toBeDisabled();
    expect(user).toBeTruthy();
  });

  it('turning a step off: risk text, reason ≥ 10 with counter, expiry required, X-Tenant-Id; 422 reason code on the field; then "Chờ phê duyệt"', async () => {
    const { backend, user } = await renderSafety('/platform-admin/moderation-switches');
    tenantFilterStore.set([T_B]);
    const rowB = (await screen.findByText('THCS Demo B')).closest('tr')!;
    await user.click(within(rowB).getByRole('switch', { name: 'Lọc spam — THCS Demo B' }));
    const dialog = await screen.findByRole('dialog', { name: 'Tắt Lọc spam' });
    expect(within(dialog).getByText(/học sinh 6–15 tuổi/)).toBeInTheDocument();
    const reason = within(dialog).getByLabelText('Lý do');
    expect(reason).toHaveAttribute('maxlength', '1000');
    fireEvent.change(reason, { target: { value: 'ngắn' } });
    await user.click(within(dialog).getByRole('button', { name: 'Gửi yêu cầu tắt' }));
    expect(await within(dialog).findByText('Chọn thời hạn.')).toBeInTheDocument();
    expect(backend.calls('PUT', '/admin/chat/moderation-switches')).toHaveLength(0);
    await pick(user, 'Tự bật lại sau (tối đa 24 giờ)', '2 giờ', dialog);
    // the client lets "ngắn" through only if the form passes: min 10 blocks it first
    await user.click(within(dialog).getByRole('button', { name: 'Gửi yêu cầu tắt' }));
    expect(backend.calls('PUT', '/admin/chat/moderation-switches')).toHaveLength(0);
    // the server's own check is mapped to the field too
    backend.overrides.unshift((r) => (r.method === 'PUT' && r.path === '/admin/chat/moderation-switches' ? problem(422, 'MODERATION_SWITCH_REASON_REQUIRED') : undefined));
    fireEvent.change(reason, { target: { value: 'Điều tra lọc nhầm tin thi đấu' } });
    await user.click(within(dialog).getByRole('button', { name: 'Gửi yêu cầu tắt' }));
    expect(await within(dialog).findAllByText('Cần nhập lý do (ít nhất 10 ký tự).')).not.toHaveLength(0);
    backend.overrides.shift();
    await user.click(within(dialog).getByRole('button', { name: 'Gửi yêu cầu tắt' }));
    expect(await within(dialog).findByText('Chờ phê duyệt')).toBeInTheDocument();
    const put = backend.calls('PUT', '/admin/chat/moderation-switches').at(-1)!;
    expect(put.headers['x-tenant-id']).toBe(T_B);
    expect(put.headers['idempotency-key']).toBeTruthy();
    const body = put.body as { scope: string; tenant_id: string; steps: Record<string, boolean>; reason: string; expires_at: string };
    expect(body).toMatchObject({ scope: 'tenant', tenant_id: T_B, enabled_all: true, reason: 'Điều tra lọc nhầm tin thi đấu' });
    expect(body.steps).toEqual({ sensitive_terms: true, spam: false, bullying: true, async_analysis: true });
    const hours = (Date.parse(body.expires_at) - Date.now()) / 3_600_000;
    expect(hours).toBeGreaterThan(1.9);
    expect(hours).toBeLessThanOrEqual(2);
  });

  it('F122-AC-27: pending request — no Approve button for the proposer; another System Admin approves; error codes by code', async () => {
    let r = await renderSafety('/platform-admin/moderation-switches'); // sa.admin proposed request1
    tenantFilterStore.set([T_A]);
    const pending = await screen.findByRole('region', { name: 'Yêu cầu chờ phê duyệt' });
    expect(within(pending).getByText('Chờ phê duyệt')).toBeInTheDocument();
    expect(within(pending).queryByRole('button', { name: 'Phê duyệt' })).toBeNull();
    expect(within(pending).getByText(/Yêu cầu của bạn/)).toBeInTheDocument();
    cleanup();
    stopPortals();
    tenantFilterStore.reset();
    r = await renderSafety('/platform-admin/moderation-switches', { account: 'sa.admin2' });
    tenantFilterStore.set([T_A]);
    const region = await screen.findByRole('region', { name: 'Yêu cầu chờ phê duyệt' });
    // self-approval refused by the server is shown by code
    r.backend.overrides.unshift((x) => (x.path.endsWith(':approve') ? problem(403, 'MODERATION_SWITCH_SELF_APPROVAL') : undefined));
    await r.user.click(within(region).getByRole('button', { name: 'Phê duyệt' }));
    expect(await screen.findByText(/Người đề xuất không tự phê duyệt được/)).toBeInTheDocument();
    r.backend.overrides.shift();
    await r.user.click(within(region).getByRole('button', { name: 'Phê duyệt' }));
    await waitFor(() => expect(r.backend.calls('POST', `/admin/chat/moderation-switches/requests/${CS.request1}:approve`)).toHaveLength(2));
    const approve = r.backend.calls('POST', `/admin/chat/moderation-switches/requests/${CS.request1}:approve`)[1]!;
    expect(approve.headers['x-tenant-id']).toBe(T_A);
    await waitFor(() => expect(screen.queryByRole('region', { name: 'Yêu cầu chờ phê duyệt' })).toBeNull());
  });

  it('F122-AC-27: EXPIRY_REQUIRED and STEP_NOT_ALLOWED are shown by code', async () => {
    const { backend, user } = await renderSafety('/platform-admin/moderation-switches');
    const row = (await screen.findByText('Platform', { selector: '.ant-tag' })).closest('tr')!;
    backend.overrides.unshift((x) => (x.method === 'PUT' ? problem(422, 'MODERATION_SWITCH_STEP_NOT_ALLOWED') : undefined));
    await user.click(within(row).getByRole('switch', { name: 'Danh mục từ nhạy cảm — Platform' }));
    const dialog = await screen.findByRole('dialog');
    fireEvent.change(within(dialog).getByLabelText('Lý do'), { target: { value: 'Điều tra danh mục mới phát hành' } });
    await pick(user, 'Tự bật lại sau (tối đa 24 giờ)', '1 giờ', dialog);
    await user.click(within(dialog).getByRole('button', { name: 'Gửi yêu cầu tắt' }));
    expect(await within(dialog).findByText('Bước PII và phát hiện dụ dỗ luôn chạy, không tắt được.')).toBeInTheDocument();
    backend.overrides.shift();
    backend.overrides.unshift((x) => (x.method === 'PUT' ? problem(422, 'MODERATION_SWITCH_EXPIRY_REQUIRED') : undefined));
    await user.click(within(dialog).getByRole('button', { name: 'Gửi yêu cầu tắt' }));
    expect(await within(dialog).findAllByText('Cần chọn thời hạn tự bật lại (tối đa 24 giờ).')).not.toHaveLength(0);
  });

  it('F122-AC-12: history with before/after, reason, expiry, actor, approver and source', async () => {
    await renderSafety('/platform-admin/moderation-switches');
    fireEvent.click(await screen.findByRole('tab', { name: 'Lịch sử' }));
    expect(await screen.findByText('Sự kiện thi đấu cấp trường')).toBeInTheDocument();
    const row = screen.getByText('Sự kiện thi đấu cấp trường').closest('tr')!;
    expect(within(row).getByText('Mọi bước bật')).toBeInTheDocument();
    expect(within(row).getByText('Lọc spam')).toBeInTheDocument();
    expect(within(row).getByText('Đã duyệt')).toBeInTheDocument();
    const auto = screen.getByText('Hết hạn tắt').closest('tr')!;
    expect(within(auto).getByText('Hết hạn tự bật')).toBeInTheDocument();
    expect(within(auto).getByText('Hệ thống')).toBeInTheDocument();
  });
});
