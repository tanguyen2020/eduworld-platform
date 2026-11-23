// F122 T009–T012, T015, T016, T043–T045 — moderation of the Platform Safety Team (workspace E). Backend = MOCK
// (test/chat-safety-mock.ts, test only, AGENTS R17).
// F122-AC-01 queue filters (module tenant filter, severity, status, assignee, category), Tenant column, SLA overdue, cursor;
// F122-AC-02 evidence (snapshot, original, revisions, recall), search only inside the case, no free chat search;
// F122-AC-03 act with Idempotency-Key + If-Match + X-Tenant-Id of the case, note required, account_lock two steps;
// F122-AC-04 412 → reload message, 409/403 by code with trace id, no automatic retry;
// F122-AC-05 notify school: school of the case, its students only, summary limits, hidden for tenant `common`;
// F122-AC-06 platform notifications (overdue / acknowledged filter);
// F122-AC-24 Safety Moderator: only Kiểm duyệt of E (+ the read-only catalogue of D, T062), other workspaces → 403 without
// loading their chunk; F122 T064 tenant filter from listModerationCaseTenants (every page, never listTenants) and the
// Tenant column from ModerationCase.tenant_name;
// F122-AC-25 403 PERMISSION_DENIED → 403 page with trace id; internal gateway down → VPN notice;
// F122-AC-26 maxLength + counters; the same Idempotency-Key on a retry of the same intent.
import { configure, fireEvent, screen, waitFor, within } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { CS, renderSafety, T_A, T_COMMON } from '../../../../test/chat-safety-mock';
import { problem, stopPortals, TENANTS } from '../../../../test/portal-harness';

vi.setConfig({ testTimeout: 900_000 }); // whole-portal screens on a shared, throttled CPU (other agents run tests too)
configure({ asyncUtilTimeout: 90_000 });

afterEach(() => stopPortals());

const queueRows = () => screen.getAllByRole('row').slice(1);

async function pick(user: { click: (e: Element) => Promise<void> }, label: string, option: string) {
  await user.click(screen.getByRole('combobox', { name: label }));
  await user.click((await screen.findAllByTitle(option)).at(-1)!);
}

describe('Hàng đợi case (F122-AC-01, AC-06, AC-24)', () => {
  it('sm.antoan: only the Kiểm duyệt menus; queue with Tenant column, SLA overdue and cursor pages', async () => {
    const { backend, user, loaders } = await renderSafety('/live-ops/moderation-cases', { account: 'sm.antoan' });
    expect(await screen.findByText('TH Demo A')).toBeInTheDocument();
    const nav = screen.getByRole('navigation', { name: 'Menu điều hướng' });
    for (const m of ['Hàng đợi case kiểm duyệt', 'Sanction', 'Từ bổ sung của tenant']) expect(within(nav).getByText(m)).toBeInTheDocument();
    for (const m of ['Live dashboard', 'Game Analytics', 'Lịch trực on-call', 'Sự cố']) expect(within(nav).queryByText(m)).toBeNull();
    // SLA overdue of the p1 case: error token AND text
    const overdue = screen.getAllByText(/^Quá hạn/)[0]!;
    expect(overdue.closest('[data-sla]')).toHaveAttribute('data-sla', 'overdue');
    expect(screen.getByText('Nghiêm trọng (p1)')).toBeInTheDocument();
    // cursor: page size 2 → the common-tenant case comes with "Tải thêm"
    expect(queueRows()).toHaveLength(2);
    await user.click(screen.getByRole('button', { name: 'Tải thêm' }));
    await waitFor(() => expect(screen.getByText('Tenant chung')).toBeInTheDocument());
    expect(backend.calls('GET', '/admin/moderation/cases').some((r) => new URLSearchParams(r.search).get('cursor') === '2')).toBe(true);
    // no X-Tenant-Id on the cross-tenant read
    expect(backend.calls('GET', '/admin/moderation/cases')[0]!.headers['x-tenant-id']).toBeUndefined();
    // F122-AC-24: other workspaces are not loaded
    expect(loaders.platform_admin).not.toHaveBeenCalled();
    expect(loaders.school_lms).not.toHaveBeenCalled();
  });

  it('filters: module tenant filter (tenant_ids[]), severity, status, assignee, category (maxLength)', async () => {
    const { backend, user } = await renderSafety('/live-ops/moderation-cases', { account: 'sm.antoan' });
    expect(await screen.findByText('TH Demo A')).toBeInTheDocument();
    await pick(user, 'Lọc tenant', 'TH Demo A');
    await waitFor(() => expect(new URLSearchParams(backend.calls('GET', '/admin/moderation/cases').at(-1)!.search).getAll('tenant_ids')).toEqual([T_A]));
    await pick(user, 'Mức', 'Nghiêm trọng (p1)');
    await pick(user, 'Trạng thái', 'Mới');
    await pick(user, 'Người xử lý', 'Chưa nhận');
    const category = screen.getByRole('searchbox', { name: 'Nhóm vi phạm' });
    expect(category).toHaveAttribute('maxlength', '64');
    fireEvent.change(category, { target: { value: 'insult' } });
    fireEvent.keyDown(category, { key: 'Enter', code: 'Enter' });
    await waitFor(() => {
      const q = new URLSearchParams(backend.calls('GET', '/admin/moderation/cases').at(-1)!.search);
      expect([q.get('severity'), q.get('status'), q.get('assignee'), q.get('category_code')]).toEqual(['p1', 'open', 'unassigned', 'insult']);
    });
  });

  it('F122-AC-24: sm.antoan opening a C URL gets the 403 page without the chunk', async () => {
    const { loaders } = await renderSafety('/school-lms/clubs', { account: 'sm.antoan' });
    expect(await screen.findByText('Không có quyền truy cập')).toBeInTheDocument();
    expect(loaders.school_lms).not.toHaveBeenCalled();
  });

  it('F122-AC-01 (T064): the tenant filter lists the tenants having cases (every page of listModerationCaseTenants), never listTenants; Tenant column from tenant_name', async () => {
    const { backend, user } = await renderSafety('/live-ops/moderation-cases', { account: 'sm.antoan' });
    expect(await screen.findByText('TH Demo A')).toBeInTheDocument();
    expect(screen.getByText('THCS Demo B')).toBeInTheDocument();
    await user.click(screen.getByRole('combobox', { name: 'Lọc tenant' }));
    // page size 2 → the dropdown walks both pages (cursor) before showing the whole list
    expect((await screen.findAllByTitle('Tenant chung')).length).toBeGreaterThan(0);
    const calls = backend.calls('GET', '/admin/moderation/case-tenants');
    expect(calls.map((c) => new URLSearchParams(c.search).get('cursor'))).toEqual([null, '2']);
    expect(calls.every((c) => new URLSearchParams(c.search).get('limit') === '100' && c.headers['x-tenant-id'] === undefined)).toBe(true);
    expect(backend.calls('GET', '/admin/tenants')).toHaveLength(0);
    await user.click((await screen.findAllByTitle('THCS Demo B')).at(-1)!);
    await waitFor(() => expect(new URLSearchParams(backend.calls('GET', '/admin/moderation/cases').at(-1)!.search).getAll('tenant_ids')).toEqual([TENANTS.thcsDemoB.tenant_id]));
  });

  it('F122-AC-06: school notifications sent by the platform, overdue / acknowledged filters', async () => {
    const { backend, user } = await renderSafety('/live-ops/moderation-cases');
    await user.click(await screen.findByRole('tab', { name: 'Thông báo nhà trường' }));
    expect(await screen.findByText('Đã liên hệ phụ huynh')).toBeInTheDocument();
    expect(screen.getAllByText('Chờ xác nhận').length).toBeGreaterThan(0);
    await pick(user, 'Trạng thái', 'Quá hạn');
    await waitFor(() => expect(new URLSearchParams(backend.calls('GET', '/admin/safety/school-notifications').at(-1)!.search).get('overdue')).toBe('true'));
    await waitFor(() => expect(screen.queryByText('Đã liên hệ phụ huynh')).toBeNull());
    await pick(user, 'Trạng thái', 'Đã xác nhận');
    await waitFor(() => expect(new URLSearchParams(backend.calls('GET', '/admin/safety/school-notifications').at(-1)!.search).get('acknowledged')).toBe('true'));
  });
});

describe('Chi tiết case (F122-AC-02, AC-03, AC-04, AC-26)', () => {
  it('evidence panel: snapshot, original, revisions, recall; search inside the case only; no free chat search', async () => {
    const { backend } = await renderSafety(`/live-ops/moderation-cases/${CS.caseA}`, { account: 'sm.antoan' });
    const ev = await screen.findByTestId(`evidence-${CS.msg1}`);
    expect(within(ev).getByText('Trong snapshot')).toBeInTheDocument();
    expect(within(ev).getByText('zqgoc ban dau')).toBeInTheDocument();
    expect(within(ev).getByText('zq*** ban dau')).toBeInTheDocument();
    expect(within(ev).getByText(/Bản 1: zqgoc lan 1/)).toBeInTheDocument();
    expect(within(screen.getByTestId(`evidence-${CS.msg2}`)).getByText(/Đã thu hồi/)).toBeInTheDocument();
    const search = screen.getByRole('searchbox', { name: 'Tìm trong case này' });
    expect(search).toHaveAttribute('maxlength', '200');
    expect(screen.getAllByRole('searchbox')).toHaveLength(1); // no other (free chat) search on the screen
    fireEvent.change(search, { target: { value: 'thu hoi' } });
    fireEvent.keyDown(search, { key: 'Enter', code: 'Enter' });
    await waitFor(() => expect(new URLSearchParams(backend.calls('GET', `/admin/moderation/cases/${CS.caseA}/messages`).at(-1)!.search).get('q')).toBe('thu hoi'));
    await waitFor(() => expect(screen.queryByTestId(`evidence-${CS.msg1}`)).toBeNull());
  });

  it('act: note required, Idempotency-Key + If-Match + X-Tenant-Id of the case; account_lock needs a second confirmation; take the case', async () => {
    const { backend, user, state } = await renderSafety(`/live-ops/moderation-cases/${CS.caseA}`, { account: 'sm.antoan' });
    await screen.findByTestId(`evidence-${CS.msg1}`);
    // take the case → assign to me with the case tenant
    await user.click(screen.getByRole('button', { name: 'Nhận case' }));
    await waitFor(() => expect(backend.calls('POST', `/admin/moderation/cases/${CS.caseA}:assign`)).toHaveLength(1));
    const assign = backend.calls('POST', `/admin/moderation/cases/${CS.caseA}:assign`)[0]!;
    expect(assign.headers['x-tenant-id']).toBe(T_A);
    expect(assign.headers['idempotency-key']).toMatch(/^[0-9a-f-]{36}$/);
    expect(assign.body).toEqual({ assignee_moderator_id: CS.moderator });
    // note is required (3–1000, counter)
    const note = screen.getByLabelText('Ghi chú');
    expect(note).toHaveAttribute('maxlength', '1000');
    await user.click(screen.getByRole('button', { name: 'Thực hiện' }));
    expect(await screen.findAllByText(/bắt buộc/i)).not.toHaveLength(0);
    expect(backend.calls('POST', `/admin/moderation/cases/${CS.caseA}:act`)).toHaveLength(0);
    // restrict + account_lock → second step with a checkbox
    await pick(user, 'Thao tác', 'Hạn chế');
    await pick(user, 'Biện pháp', 'Khóa tài khoản');
    fireEvent.change(note, { target: { value: 'Dụ dỗ lặp lại nhiều lần' } });
    await user.click(screen.getByRole('button', { name: 'Thực hiện' }));
    const dialog = await screen.findByRole('dialog', { name: 'Xác nhận khóa tài khoản' });
    expect(within(dialog).getByRole('button', { name: 'Khóa tài khoản' })).toBeDisabled();
    expect(backend.calls('POST', `/admin/moderation/cases/${CS.caseA}:act`)).toHaveLength(0);
    await user.click(within(dialog).getByRole('checkbox'));
    await user.click(within(dialog).getByRole('button', { name: 'Khóa tài khoản' }));
    await waitFor(() => expect(backend.calls('POST', `/admin/moderation/cases/${CS.caseA}:act`)).toHaveLength(1));
    const act = backend.calls('POST', `/admin/moderation/cases/${CS.caseA}:act`)[0]!;
    expect(act.headers['x-tenant-id']).toBe(T_A);
    expect(act.headers['if-match']).toBe('W/"4"'); // version after the assignment
    expect(act.headers['idempotency-key']).toMatch(/^[0-9a-f-]{36}$/);
    expect(act.body).toEqual({ action: 'restrict', sanction_type: 'account_lock', note: 'Dụ dỗ lặp lại nhiều lần' });
    expect(state.cases[0]!.status).toBe('in_review');
  });

  it('F122-AC-04: 412 → reloaded message and the next try uses the new version; 409/403 by code with trace id; no automatic retry', async () => {
    const { backend, user, state } = await renderSafety(`/live-ops/moderation-cases/${CS.caseA}`, { account: 'sm.antoan' });
    await screen.findByTestId(`evidence-${CS.msg1}`);
    state.cases[0]!.version = 9; // someone else changed the case meanwhile
    fireEvent.change(screen.getByLabelText('Ghi chú'), { target: { value: 'Nhắc nhở lần đầu' } });
    await user.click(screen.getByRole('button', { name: 'Thực hiện' }));
    expect(await screen.findByText(/đã được người khác thay đổi — đã tải lại/)).toBeInTheDocument();
    expect(backend.calls('POST', `/admin/moderation/cases/${CS.caseA}:act`)).toHaveLength(1); // not retried
    await waitFor(() => expect(backend.calls('GET', `/admin/moderation/cases/${CS.caseA}`).length).toBeGreaterThan(1));
    await user.click(screen.getByRole('button', { name: 'Thực hiện' }));
    await waitFor(() => expect(backend.calls('POST', `/admin/moderation/cases/${CS.caseA}:act`)).toHaveLength(2));
    const [first, second] = backend.calls('POST', `/admin/moderation/cases/${CS.caseA}:act`);
    expect(first!.headers['if-match']).toBe('W/"3"');
    expect(second!.headers['if-match']).toBe('W/"9"');
    // 409 MODERATION_CASE_CLOSED and 403 (lead only) → message by code + trace id, no retry
    backend.overrides.unshift((r) => (r.path.endsWith(':act') ? problem(409, 'MODERATION_CASE_CLOSED') : undefined));
    fireEvent.change(screen.getByLabelText('Ghi chú'), { target: { value: 'Nhắc nhở lần hai' } }); // the form was reset after the success
    await user.click(screen.getByRole('button', { name: 'Thực hiện' }));
    expect(await screen.findByText('Case đã đóng. Mở lại case trước khi thao tác.')).toBeInTheDocument();
    expect(screen.getByText(/trace-moderation_case_closed/)).toBeInTheDocument();
    backend.overrides.shift();
    backend.overrides.unshift((r) => (r.path.endsWith(':act') ? problem(403, 'PERMISSION_DENIED') : undefined));
    await pick(user, 'Thao tác', 'Chuyển trưởng ca');
    await user.click(screen.getByRole('button', { name: 'Thực hiện' }));
    expect(await screen.findByText('Bạn không có quyền thực hiện thao tác này.')).toBeInTheDocument();
    expect(screen.getByText(/trace-permission_denied/)).toBeInTheDocument();
    expect(backend.calls('POST', `/admin/moderation/cases/${CS.caseA}:act`)).toHaveLength(4);
  });

  it('F122-AC-26: the same intent retried keeps its Idempotency-Key; a changed intent gets a new one', async () => {
    const { backend, user } = await renderSafety(`/live-ops/moderation-cases/${CS.caseA}`, { account: 'sm.antoan' });
    await screen.findByTestId(`evidence-${CS.msg1}`);
    let fail = true;
    backend.overrides.unshift((r) => (r.path.endsWith(':act') && fail ? problem(503, 'SERVICE_UNAVAILABLE') : undefined));
    fireEvent.change(screen.getByLabelText('Ghi chú'), { target: { value: 'Cảnh báo lần đầu' } });
    await user.click(screen.getByRole('button', { name: 'Thực hiện' }));
    await waitFor(() => expect(backend.calls('POST', `/admin/moderation/cases/${CS.caseA}:act`).length).toBeGreaterThan(0));
    await screen.findByRole('alert');
    fail = false;
    const before = backend.calls('POST', `/admin/moderation/cases/${CS.caseA}:act`).length;
    await user.click(screen.getByRole('button', { name: 'Thực hiện' }));
    await waitFor(() => expect(backend.calls('POST', `/admin/moderation/cases/${CS.caseA}:act`).length).toBeGreaterThan(before));
    const keys = new Set(backend.calls('POST', `/admin/moderation/cases/${CS.caseA}:act`).map((r) => r.headers['idempotency-key']));
    expect(keys.size).toBe(1);
  });
});

describe('Thông báo nhà trường (F122-AC-05)', () => {
  it('school of the case, only its students, summary 10–1000 with a counter, Idempotency-Key + X-Tenant-Id', async () => {
    const { backend, user } = await renderSafety(`/live-ops/moderation-cases/${CS.caseA}`, { account: 'sm.antoan' });
    await screen.findByTestId(`evidence-${CS.msg1}`);
    await user.click(screen.getByRole('button', { name: 'Gửi thông báo trường' }));
    const drawer = await screen.findByRole('dialog', { name: 'Gửi thông báo nhà trường' });
    expect(within(drawer).getByText('Chọn trường trước.')).toBeInTheDocument();
    await pick(user, 'Trường nhận thông báo', CS.schoolA);
    // only the student of school A (CaVang), not the one of the other school (MeoCon)
    expect(await within(drawer).findByRole('checkbox', { name: 'CaVang' })).toBeInTheDocument();
    expect(within(drawer).queryByRole('checkbox', { name: 'MeoCon' })).toBeNull();
    await user.click(within(drawer).getByRole('checkbox', { name: 'CaVang' }));
    const summary = within(drawer).getByLabelText('Tóm tắt cho nhà trường');
    expect(summary).toHaveAttribute('maxlength', '1000');
    expect(within(drawer).getByText('Không chép nội dung chat vào tóm tắt.')).toBeInTheDocument();
    fireEvent.change(summary, { target: { value: 'ngắn' } });
    await user.click(within(drawer).getByRole('button', { name: 'Gửi thông báo' }));
    expect(await within(drawer).findByText(/10/)).toBeInTheDocument();
    expect(backend.calls('POST', `/admin/moderation/cases/${CS.caseA}:notify-school`)).toHaveLength(0);
    fireEvent.change(summary, { target: { value: 'Học sinh bị trêu chọc lặp lại; đã khóa chat 24 giờ.' } });
    await user.click(within(drawer).getByRole('button', { name: 'Gửi thông báo' }));
    await waitFor(() => expect(backend.calls('POST', `/admin/moderation/cases/${CS.caseA}:notify-school`)).toHaveLength(1));
    const call = backend.calls('POST', `/admin/moderation/cases/${CS.caseA}:notify-school`)[0]!;
    expect(call.headers['x-tenant-id']).toBe(T_A);
    expect(call.headers['idempotency-key']).toMatch(/^[0-9a-f-]{36}$/);
    expect(call.body).toEqual({ school_id: CS.schoolA, student_ids: [CS.studentA], summary: 'Học sinh bị trêu chọc lặp lại; đã khóa chat 24 giờ.' });
  });

  it('a case of the common tenant has no notify button but "Liên hệ trực tiếp"', async () => {
    await renderSafety(`/live-ops/moderation-cases/${CS.caseCommon}`, { account: 'sm.antoan' });
    expect(await screen.findByText(/Safety Team liên hệ trực tiếp/)).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Gửi thông báo trường' })).toBeNull();
    expect(T_COMMON).toBeTruthy();
  });
});

describe('Lỗi quyền và mạng quản trị (F122-AC-25)', () => {
  it('403 PERMISSION_DENIED → 403 page with the trace id (no MFA step-up); internal gateway down → VPN notice', async () => {
    const { backend } = await renderSafety('/live-ops/sanctions', { account: 'sm.antoan' });
    backend.overrides.unshift((r) => (r.path === '/admin/moderation/sanctions' ? problem(403, 'PERMISSION_DENIED') : undefined));
    // first load may already be done: reload by switching screens
    fireEvent.click(await screen.findByRole('link', { name: 'Hàng đợi case kiểm duyệt' }));
    fireEvent.click(await screen.findByRole('link', { name: 'Sanction' }));
    expect(await screen.findByText(/trace-permission_denied/)).toBeInTheDocument();
    expect(screen.getByText('Không có quyền truy cập')).toBeInTheDocument();
    expect(screen.queryByText(/MFA/)).toBeNull();
    backend.overrides.unshift((r) => (r.gateway === 'internal' && r.path === '/admin/moderation/cases' ? Promise.reject(new TypeError('Failed to fetch')) : undefined));
    fireEvent.click(screen.getByRole('link', { name: 'Hàng đợi case kiểm duyệt' }));
    expect((await screen.findAllByText(/Cần kết nối mạng quản trị \(VPN\)/)).length).toBeGreaterThan(0);
    expect(TENANTS.thDemoA.tenant_id).toBe(T_A);
  });
});
