// F081 T007–T009, T012, T013, T014, T018, T029, T034 — "Sự kiện trường" of workspace C. Backend = MOCK
// (test/school-events-mock.ts + test/school-lms-mock.ts, test only, AGENTS R17); the server rules themselves are tested in
// module game (tests/integration/liveops_school_progress_test.go, liveops_school_test.go).
// F081-AC-01 templates of the tenant only, empty state without a free form;
// F081-AC-02 only allowed_params, options from the template lists, grades 1–9 (no class);
// F081-AC-03 Idempotency-Key once per intent, kept on retry, opens the new event;
// F081-AC-04 422 errors on their field (no matched word), 503 keeps the form, 409 limit message;
// F081-AC-05 vi required, en optional, maxLength + counters, ICT dates, template maximum shown;
// F081-AC-06 list (cursor pages, status filter), System Admin tenant_ids + Tenant column + one target tenant to create;
// F081-AC-07 If-Match, 412 → "đã thay đổi" + reload, no edit button once live/suspended;
// F081-AC-08 cancel: reason 3–1000, two-step confirmation with "giữ nguyên" rewards, Idempotency-Key;
// F081-AC-09 403 page without lms.school_event.write, another tenant's event → "Không tìm thấy";
// F081-AC-11 progress bar with server milestones, grade table sorted, no rank, "Cập nhật lúc", manual refresh;
// F081-AC-19 errors with code + trace_id, phone = view only; F081-AC-20 menu/action off, 403 FEATURE_DISABLED.
import { cleanup, configure, fireEvent, screen, waitFor, within } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { tenantFilterStore } from '../../../../shell/tenantFilterStore';
import { EV, FLAGGED, renderSchoolEvents, schoolEventsState, T_A, T_B } from '../../../../test/school-events-mock';
import { accountWith, READ_ONLY_PERMS, TENANTS } from '../../../../test/school-lms-harness';
import { problem, stopPortals } from '../../../../test/portal-harness';

vi.setConfig({ testTimeout: 900_000 }); // whole-portal screens on a shared, throttled CPU
configure({ asyncUtilTimeout: 90_000 });

afterEach(() => {
  stopPortals();
  tenantFilterStore.reset();
});

const posts = (backend: { calls: (m: string, p: string) => { headers: Record<string, string>; body: unknown }[] }) => backend.calls('POST', '/lms/school-events');

/** Fills the always-present fields of the form (name and dates in ICT). */
async function fillBasics(user: { type: (e: Element, s: string) => Promise<void>; tab: () => Promise<void> }, name = 'Tuần lễ Toán học trường A') {
  fireEvent.change(await screen.findByLabelText('Tên sự kiện (tiếng Việt)'), { target: { value: name } });
  // typed then blurred (Enter inside the form would submit it)
  await user.type(screen.getByLabelText('Bắt đầu'), '01/12/2026 08:00');
  await user.tab();
  await user.type(screen.getByLabelText('Kết thúc'), '08/12/2026 08:00');
  await user.tab();
  await waitFor(() => expect(screen.getByTestId('event-duration')).toHaveTextContent('Thời lượng: 7 ngày'));
}

describe('Tạo sự kiện từ mẫu (F081 US1)', () => {
  it('AC-01, AC-02, AC-05: templates of the tenant, only the allowed parameters, options from the template, limits and ICT', async () => {
    const { user, backend } = await renderSchoolEvents('/school-lms/school-events/new');
    const card = (await screen.findByText('Tuần lễ Toán học')).closest('.ant-card') as HTMLElement;
    expect(within(card).getByText('Tối đa 14 ngày')).toBeInTheDocument();
    expect(within(card).getByText('Sự kiện trường')).toBeInTheDocument();
    expect(within(card).getByText('Cả trường cùng giải toán')).toBeInTheDocument();
    expect(backend.calls('GET', '/lms/event-templates').every((r) => r.headers['x-tenant-id'] === T_A)).toBe(true);
    fireEvent.click(within(card).getByRole('button', { name: /Dùng mẫu Tuần lễ Toán học/ }));
    // AC-05: vi required (+ counter), en optional, description limits
    const nameVi = await screen.findByLabelText('Tên sự kiện (tiếng Việt)');
    expect(nameVi).toHaveAttribute('maxlength', '255');
    expect(screen.getByLabelText('Tên sự kiện (tiếng Anh, không bắt buộc)')).toHaveAttribute('maxlength', '255');
    expect(screen.getByLabelText('Mô tả (tiếng Việt)')).toHaveAttribute('maxlength', '4000');
    // AC-02: allowed parameters only — no school picker (not allowed), no challenge picker, no rule/score input
    expect(screen.getAllByRole('checkbox')).toHaveLength(9);
    expect(screen.getByRole('checkbox', { name: 'Khối 9' })).toBeInTheDocument();
    expect(screen.queryByText(/lớp/i)).toBeNull();
    expect(screen.getByLabelText('Mục tiêu chung')).toBeInTheDocument();
    expect(screen.queryByLabelText('Trường')).toBeNull();
    expect(screen.queryByLabelText('Thử thách trong sự kiện')).toBeNull();
    expect(screen.queryByLabelText(/luật|điểm/i)).toBeNull();
    await user.click(screen.getByRole('combobox', { name: 'Nhiệm vụ trong sự kiện' }));
    expect((await screen.findAllByText('Bài toán phiên chợ (Khối 3)')).length).toBeGreaterThan(0);
    expect(screen.getAllByText('Đếm sao (Khối 4, 5)').length).toBeGreaterThan(0);
    expect(screen.getByRole('radio', { name: 'Huy hiệu trường' })).toBeInTheDocument();
    // vi required
    await user.click(screen.getByRole('button', { name: 'Tạo sự kiện' }));
    expect(await screen.findAllByText(/bắt buộc|Nhập|Chọn thời điểm/)).not.toHaveLength(0);
    expect(posts(backend)).toHaveLength(0);
    // ICT: 01/12/2026 08:00 ICT = 01:00 UTC; the template maximum is shown
    await fillBasics(user);
    expect(screen.getByTestId('event-duration')).toHaveTextContent('Thời lượng: 7 ngày (mẫu cho phép tối đa 14 ngày)');
    await user.click(screen.getByRole('checkbox', { name: 'Khối 4' }));
    await user.click(screen.getByRole('radio', { name: 'Huy hiệu trường' }));
    await user.click(screen.getByRole('button', { name: 'Tạo sự kiện' }));
    await waitFor(() => expect(posts(backend)).toHaveLength(1));
    const body = posts(backend)[0]!.body as Record<string, unknown>;
    expect(body).toMatchObject({ template_id: EV.template, name: { vi: 'Tuần lễ Toán học trường A' }, starts_at: '2026-12-01T01:00:00.000Z', ends_at: '2026-12-08T01:00:00.000Z',
      audience: { grade_levels: [4] }, reward_option_id: 'school_badge' });
    expect(body).not.toHaveProperty('challenge_ids');
    expect((body.name as Record<string, unknown>).en).toBeUndefined();
  });

  it('AC-01: a tenant without an assigned template gets the empty state and no form', async () => {
    await renderSchoolEvents('/school-lms/school-events/new', { as: 'ad.truong.b' });
    expect(await screen.findByTestId('no-templates')).toHaveTextContent('Hãy liên hệ EduWorld');
    expect(screen.queryByLabelText('Tên sự kiện (tiếng Việt)')).toBeNull();
  });

  it('AC-03, AC-04: same Idempotency-Key on retry (503 keeps the form), flagged name on its field without the word, 409 limit', async () => {
    const state = schoolEventsState();
    const { user, backend } = await renderSchoolEvents('/school-lms/school-events/new', { state });
    fireEvent.click(await screen.findByRole('button', { name: /Dùng mẫu Tuần lễ Toán học/ }));
    // flagged text → generic message on the name field, the word never shown
    await fillBasics(user, `Sự kiện ${FLAGGED}`);
    await user.click(screen.getByRole('button', { name: 'Tạo sự kiện' }));
    expect(await screen.findByText('Nội dung chưa hợp lệ hoặc có từ ngữ không phù hợp. Hãy sửa lại.')).toBeInTheDocument();
    expect(screen.queryByText(new RegExp(FLAGGED))).toBeNull(); // only the input holds it; no message repeats a word
    // 503: the form is kept, the code and trace_id are shown; retry → the same key
    fireEvent.change(screen.getByLabelText('Tên sự kiện (tiếng Việt)'), { target: { value: 'Tuần lễ Toán học' } });
    state.screeningDown = true;
    await user.click(screen.getByRole('button', { name: 'Tạo sự kiện' }));
    expect(await screen.findByText(/Dữ liệu đã nhập được giữ nguyên/)).toBeInTheDocument();
    expect(screen.getByTestId('event-error')).toHaveTextContent('UNAVAILABLE');
    expect(screen.getByTestId('event-error')).toHaveTextContent('trace-unavailable');
    expect(screen.getByLabelText('Tên sự kiện (tiếng Việt)')).toHaveValue('Tuần lễ Toán học');
    state.screeningDown = false;
    await user.click(screen.getByRole('button', { name: 'Tạo sự kiện' }));
    await screen.findByText('Tiến độ sự kiện');
    // the attempts after the flagged one (503, automatic retries, the retry click): one key for the one intent
    const keys = posts(backend)
      .filter((r) => !JSON.stringify(r.body).includes(FLAGGED))
      .map((r) => r.headers['idempotency-key']);
    expect(keys.length).toBeGreaterThanOrEqual(2);
    expect(new Set(keys).size).toBe(1);
    expect(state.created).toBe(1);
    // 409: concurrent limit
    stopPortals();
    cleanup();
    const s2 = schoolEventsState();
    s2.tooMany = true;
    const r2 = await renderSchoolEvents('/school-lms/school-events/new', { state: s2 });
    fireEvent.click(await screen.findByRole('button', { name: /Dùng mẫu Tuần lễ Toán học/ }));
    await fillBasics(r2.user);
    await r2.user.click(screen.getByRole('button', { name: 'Tạo sự kiện' }));
    expect(await screen.findByText(/số sự kiện diễn ra đồng thời tối đa/)).toBeInTheDocument();
  });
});

describe('Danh sách, sửa, hủy (F081 US2)', () => {
  it('AC-06, AC-07, AC-08: list of the session tenant; edit only scheduled (If-Match, 412 → reload); cancel in two steps', async () => {
    const { user, backend } = await renderSchoolEvents('/school-lms/school-events');
    expect(await screen.findByText('Tuần lễ Toán học trường A')).toBeInTheDocument();
    expect(screen.getByText('Sự kiện bị tạm dừng')).toBeInTheDocument();
    expect(screen.queryByText('Sự kiện THCS Demo B')).toBeNull();
    const list = backend.calls('GET', '/lms/school-events');
    expect(list.every((r) => r.headers['x-tenant-id'] === T_A && !r.search.includes('tenant_ids'))).toBe(true);
    expect(screen.queryByRole('columnheader', { name: 'Tenant' })).toBeNull();
    expect(screen.getByText('Tạm dừng bởi EduWorld')).toBeInTheDocument();
    // status filter in the query
    await user.click(screen.getByRole('combobox', { name: 'Trạng thái' }));
    await user.click((await screen.findAllByTitle('Đang diễn ra')).at(-1)!);
    await waitFor(() => expect(new URLSearchParams(backend.calls('GET', '/lms/school-events').at(-1)!.search).get('status')).toBe('live'));
    // a live event: no edit, cancel offered; a suspended event: neither
    fireEvent.click(await screen.findByRole('link', { name: 'Ngày hội đọc sách' }));
    await screen.findByText('Tiến độ sự kiện');
    expect(screen.queryByRole('button', { name: 'Sửa' })).toBeNull();
    expect(screen.getByRole('button', { name: 'Hủy sự kiện' })).toBeInTheDocument();
    stopPortals();
    cleanup();
    await renderSchoolEvents(`/school-lms/school-events/${EV.eventSuspended}`);
    expect(await screen.findByText(/Sự kiện đang tạm dừng bởi EduWorld/)).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Sửa' })).toBeNull();
    expect(screen.queryByRole('button', { name: 'Hủy sự kiện' })).toBeNull();
    stopPortals();
    cleanup();
    // edit the scheduled event: If-Match "1"; a stale version → 412 → message + reload
    const r3 = await renderSchoolEvents(`/school-lms/school-events/${EV.eventScheduled}/edit`);
    const nameVi = await screen.findByLabelText('Tên sự kiện (tiếng Việt)');
    await waitFor(() => expect(nameVi).toHaveValue('Tuần lễ Toán học trường A'));
    r3.events.events.find((e) => e.event_id === EV.eventScheduled)!.version = 2; // changed in another tab
    fireEvent.change(nameVi, { target: { value: 'Tuần lễ Toán học (sửa)' } });
    await r3.user.click(screen.getByRole('button', { name: 'Lưu thay đổi' }));
    expect(await screen.findByText('Sự kiện đã được thay đổi ở nơi khác. Tải lại để xem dữ liệu mới.')).toBeInTheDocument();
    const patch = r3.backend.calls('PATCH', `/lms/school-events/${EV.eventScheduled}`);
    expect(patch[0]!.headers['if-match']).toBe('"1"');
    expect(patch[0]!.headers['idempotency-key']).toBeTruthy();
    await r3.user.click(screen.getByRole('button', { name: 'Tải lại' }));
    await waitFor(() => expect(screen.getByLabelText('Tên sự kiện (tiếng Việt)')).toHaveValue('Tuần lễ Toán học trường A')); // reloaded (version 2)
    await r3.user.click(screen.getByRole('button', { name: 'Lưu thay đổi' }));
    await screen.findByText('Tiến độ sự kiện');
    expect(r3.backend.calls('PATCH', `/lms/school-events/${EV.eventScheduled}`).at(-1)!.headers['if-match']).toBe('"2"');
    // cancel: reason required (3–1000), second confirmation about rewards, Idempotency-Key
    await r3.user.click(await screen.findByRole('button', { name: 'Hủy sự kiện' }));
    const dialog = await screen.findByRole('dialog');
    const reason = within(dialog).getByLabelText('Lý do hủy');
    expect(reason).toHaveAttribute('maxlength', '1000');
    fireEvent.change(reason, { target: { value: 'ab' } });
    await r3.user.click(within(dialog).getByRole('button', { name: 'Tiếp tục' }));
    expect(await within(dialog).findByText(/3/)).toBeInTheDocument();
    expect(within(dialog).queryByTestId('cancel-confirm')).toBeNull();
    fireEvent.change(reason, { target: { value: 'Trùng lịch thi học kỳ' } });
    await r3.user.click(within(dialog).getByRole('button', { name: 'Tiếp tục' }));
    expect(await within(dialog).findByText('Phần thưởng học sinh đã đạt được giữ nguyên.')).toBeInTheDocument();
    await r3.user.click(within(dialog).getByRole('button', { name: 'Hủy sự kiện' }));
    await waitFor(() => expect(r3.backend.calls('POST', `/lms/school-events/${EV.eventScheduled}:cancel`)).toHaveLength(1));
    const c = r3.backend.calls('POST', `/lms/school-events/${EV.eventScheduled}:cancel`)[0]!;
    expect(c.body).toEqual({ reason: 'Trùng lịch thi học kỳ' });
    expect(c.headers['idempotency-key']).toBeTruthy();
    expect(await screen.findAllByText('Đã hủy')).not.toHaveLength(0);
  });

  it('AC-06: System Admin lists several tenants (tenant_ids, no X-Tenant-Id, Tenant column) and creates in one target tenant', async () => {
    tenantFilterStore.set([T_A, T_B]);
    const { backend } = await renderSchoolEvents('/school-lms/school-events', { as: 'sa.admin' });
    expect(await screen.findByText('Sự kiện THCS Demo B')).toBeInTheDocument();
    expect(screen.getByText('Tuần lễ Toán học trường A')).toBeInTheDocument();
    const call = backend.calls('GET', '/lms/school-events').at(-1)!;
    expect(new URLSearchParams(call.search).getAll('tenant_ids').sort()).toEqual([T_A, T_B].sort());
    expect(call.headers['x-tenant-id']).toBeUndefined();
    expect(screen.getByRole('columnheader', { name: 'Tenant' })).toBeInTheDocument();
    // a row opens with its tenant
    fireEvent.click(screen.getByRole('link', { name: 'Sự kiện THCS Demo B' }));
    await screen.findByText('Tiến độ sự kiện');
    expect(backend.calls('GET', `/lms/school-events/${EV.eventB}`).at(-1)!.headers['x-tenant-id']).toBe(T_B);
    stopPortals();
    cleanup();
    tenantFilterStore.set([T_A, T_B]);
    await renderSchoolEvents('/school-lms/school-events/new', { as: 'sa.admin' });
    expect(await screen.findByText('Chọn tenant đích trước khi chọn mẫu.')).toBeInTheDocument();
    expect(screen.queryByText('Tuần lễ Toán học')).toBeNull();
    expect(screen.getByLabelText('Tenant')).toBeInTheDocument();
  });

  it('AC-09: without lms.school_event.write → 403 page, no menu; another tenant’s event → Không tìm thấy', async () => {
    const perms = READ_ONLY_PERMS.filter((p) => p !== 'lms.school_event.write');
    const { renderSchoolLms } = await import('../../../../test/school-lms-harness');
    await renderSchoolLms('/school-lms/school-events', { account: accountWith('qlcm.truong.a', TENANTS.thDemoA, perms) });
    expect(await screen.findByText('Không có quyền truy cập')).toBeInTheDocument();
    const nav = await screen.findByRole('navigation', { name: 'Menu điều hướng' });
    expect(within(nav).queryByRole('link', { name: 'Sự kiện trường' })).toBeNull();
    stopPortals();
    cleanup();
    await renderSchoolEvents(`/school-lms/school-events/${EV.eventScheduled}`, { as: 'ad.truong.b' });
    expect(await screen.findByTestId('school-lms-not-found')).toBeInTheDocument();
    expect(screen.queryByText('Tuần lễ Toán học trường A')).toBeNull();
  });
});

describe('Tiến độ (F081 US3) và ràng buộc chung (US4)', () => {
  it('AC-11: participants, bar with the server milestones, grade table sorted without rank, "Cập nhật lúc", manual refresh', async () => {
    const { backend, user } = await renderSchoolEvents(`/school-lms/school-events/${EV.eventLive}`);
    const panel = await screen.findByTestId('event-progress');
    expect(within(panel).getByText('42')).toBeInTheDocument();
    expect(within(panel).getByText('180 / 300')).toBeInTheDocument();
    const ms = within(panel).getByTestId('goal-milestones');
    expect(within(ms).getAllByText(/Mốc \d+%/).map((x) => x.textContent)).toEqual(['Mốc 25% · Đã đạt', 'Mốc 50% · Đã đạt', 'Mốc 75% · Chưa đạt', 'Mốc 100% · Chưa đạt']);
    const table = within(panel).getByRole('table');
    expect(within(table).getAllByRole('columnheader').map((h) => h.textContent)).toEqual(['Khối', 'Tổng đóng góp', 'Số học sinh đóng góp']);
    expect(within(table).getAllByRole('row').slice(1).map((r) => r.querySelector('td')?.textContent)).toEqual(['Khối 4', 'Khối 5']);
    expect(within(panel).queryByText(/Hạng|Xếp hạng|#1/)).toBeNull();
    expect(within(panel).getByText(/Cập nhật lúc 25\/11\/2026 10:00/)).toBeInTheDocument();
    const before = backend.calls('GET', `/lms/school-events/${EV.eventLive}/progress`).length;
    await user.click(within(panel).getByRole('button', { name: 'Làm mới' }));
    await waitFor(() => expect(backend.calls('GET', `/lms/school-events/${EV.eventLive}/progress`).length).toBeGreaterThan(before));
  });

  it('AC-19, AC-20: phone = view only; action off hides "Tạo sự kiện"; menu off hides it; 403 FEATURE_DISABLED → notice + permissions reloaded', async () => {
    await renderSchoolEvents('/school-lms/school-events/new', { width: 390 });
    expect(await screen.findByText('Tác vụ này cần màn hình lớn hơn')).toBeInTheDocument();
    expect(screen.queryByText('Tuần lễ Toán học')).toBeNull();
    stopPortals();
    cleanup();
    await renderSchoolEvents('/school-lms/school-events', { width: 390 });
    expect(await screen.findByTestId('responsive-cards')).toBeInTheDocument();
    stopPortals();
    cleanup();
    await renderSchoolEvents('/school-lms/school-events', { disabledActions: ['action.school_lms.school_events.edit'] });
    expect(await screen.findByText('Tuần lễ Toán học trường A')).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Tạo sự kiện' })).toBeNull();
    stopPortals();
    cleanup();
    await renderSchoolEvents('/school-lms/schools', { disabledMenus: ['menu.school_lms.school_events'] });
    const nav = await screen.findByRole('navigation', { name: 'Menu điều hướng' });
    await within(nav).findByRole('link', { name: 'Trường' });
    expect(within(nav).queryByRole('link', { name: 'Sự kiện trường' })).toBeNull();
    stopPortals();
    cleanup();
    const { backend, user } = await renderSchoolEvents('/school-lms/school-events/new');
    const before = backend.calls('GET', '/me/permissions').length;
    backend.overrides.unshift((r) => (r.method === 'POST' && r.path === '/lms/school-events' ? problem(403, 'FEATURE_DISABLED', { feature_key: 'action.school_lms.school_events.edit' }) : undefined));
    fireEvent.click(await screen.findByRole('button', { name: /Dùng mẫu Tuần lễ Toán học/ }));
    await fillBasics(user);
    await user.click(screen.getByRole('button', { name: 'Tạo sự kiện' }));
    expect((await screen.findAllByText(/tạm tắt/)).length).toBeGreaterThan(0);
    expect(screen.getByTestId('event-error')).toHaveTextContent('FEATURE_DISABLED');
    await waitFor(() => expect(backend.calls('GET', '/me/permissions').length).toBeGreaterThan(before));
  });
});
