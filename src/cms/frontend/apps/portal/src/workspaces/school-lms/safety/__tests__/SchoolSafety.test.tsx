// F122 T031, T033–T036, T038, T039, T041, T053, T055 — chat safety of the school (workspace C). Backend = MOCK
// (test/chat-safety-mock.ts + test/school-lms-mock.ts, test only, AGENTS R17).
// F122-AC-18 notifications of the school: filters, detail (students with GRADE, no class, no chat area), menu badge;
// F122-AC-19 acknowledge: outcome required, note ≤ 500, Idempotency-Key; 409 → "đã được xác nhận" + reload;
// F122-AC-20 below 768 px: cards and a detail page with the confirm button; tenant isolation (school B);
// F122-AC-17 tenant categories read only (no term, no write button);
// F122-AC-21/22/28 school chat policy: per channel type, limits from above, "Kế thừa", always-on lock, no class/grade/real
// name control, preset for the whole school, If-Match + reason, errors on the channel card;
// F122-AC-23 clubs of the school: approve/archive with confirmation + Idempotency-Key, no chat screen; members of a club
// (listSchoolClubMembers, cursor pages) and "Gỡ thành viên" (removeSchoolClubMember: reason, confirmation, Idempotency-Key) — T061;
// F122-AC-18 period: the list asks for the last 30 days (created_from, filtered in SQL) by default — T066;
// F122-AC-29 menu/action turned off → hidden; 403 FEATURE_DISABLED → notice + permissions reloaded.
import { configure, fireEvent, screen, waitFor, within } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { CS, renderSafety, T_A } from '../../../../test/chat-safety-mock';
import { problem, stopPortals } from '../../../../test/portal-harness';

vi.setConfig({ testTimeout: 900_000 }); // whole-portal screens on a shared, throttled CPU (other agents run tests too)
configure({ asyncUtilTimeout: 90_000 });

afterEach(() => stopPortals());

async function pick(user: { click: (e: Element) => Promise<void> }, label: string, option: string) {
  await user.click(screen.getByRole('combobox', { name: label }));
  await user.click((await screen.findAllByTitle(option)).at(-1)!);
}

describe('Thông báo an toàn của trường (F122-AC-18, AC-19, AC-20)', () => {
  it('list of the tenant (pending by default), detail with grade and no chat area, menu badge; acknowledge with outcome + note', async () => {
    const { backend, user, state } = await renderSafety('/school-lms/safety-notifications', { account: 'ad.truong.a' });
    const nav = await screen.findByRole('navigation', { name: 'Menu điều hướng' });
    // the badge counts every notification waiting for an acknowledgement, older than 30 days included (Clarifications 2026-11-24)
    expect(await within(nav).findByTestId('safety-notifications-badge')).toHaveTextContent('2');
    const first = backend.calls('GET', '/tenant/safety/school-notifications').find((r) => new URLSearchParams(r.search).get('limit') !== '50' || true)!;
    expect(first.headers['x-tenant-id']).toBe(T_A);
    expect(await screen.findByText(/Nguyễn An \(Khối 3\)/)).toBeInTheDocument();
    expect(screen.queryByText(/lớp|3A/i)).toBeNull();
    // state filter
    await pick(user, 'Trạng thái', 'Tất cả');
    await waitFor(() => expect(new URLSearchParams(backend.calls('GET', '/tenant/safety/school-notifications').at(-1)!.search).get('acknowledged')).toBeNull());
    await pick(user, 'Trạng thái', 'Chờ xác nhận');
    // detail
    fireEvent.click((await screen.findAllByRole('link', { name: /Nghiêm trọng \(p1\)/ }))[0]!);
    const detail = await screen.findByTestId('notification-detail');
    for (const text of ['Khối 3', 'Bị ảnh hưởng', 'Đã khóa chat 24 giờ', 'Học sinh bị bạn trêu chọc lặp lại trong trận']) expect(within(detail).getAllByText(new RegExp(text)).length).toBeGreaterThan(0);
    expect(document.body.textContent).not.toMatch(/zqgoc|nội dung chat của/);
    // acknowledge: outcome required
    await user.click(screen.getByRole('button', { name: 'Xác nhận đã liên hệ phụ huynh' }));
    expect(await screen.findByText('Chọn kết quả liên hệ.')).toBeInTheDocument();
    const note = screen.getByLabelText('Ghi chú liên hệ');
    expect(note).toHaveAttribute('maxlength', '500');
    await user.click(screen.getByRole('radio', { name: 'Đã liên hệ phụ huynh' }));
    fireEvent.change(note, { target: { value: 'Đã gọi điện cho mẹ học sinh' } });
    await user.click(screen.getByRole('button', { name: 'Xác nhận đã liên hệ phụ huynh' }));
    await waitFor(() => expect(backend.calls('POST', `/tenant/safety/school-notifications/${CS.notifA}:acknowledge`)).toHaveLength(1));
    const ack = backend.calls('POST', `/tenant/safety/school-notifications/${CS.notifA}:acknowledge`)[0]!;
    expect(ack.headers['idempotency-key']).toBeTruthy();
    expect(ack.headers['x-tenant-id']).toBe(T_A);
    expect(ack.body).toEqual({ contact_outcome: 'parent_contacted', note: 'Đã gọi điện cho mẹ học sinh' });
    expect(state.schoolNotifications[0]!.school_ack_at).toBeTruthy();
    expect(await screen.findByText('Đã xác nhận.')).toBeInTheDocument();
  });

  it('F122-AC-18 (T066): the list asks the server for the last 30 days (created_from); "Mọi thời gian" drops it', async () => {
    const { backend, user } = await renderSafety('/school-lms/safety-notifications', { account: 'ad.truong.a' });
    expect(await screen.findByText(/Nguyễn An \(Khối 3\)/)).toBeInTheDocument();
    const listCalls = () => backend.calls('GET', '/tenant/safety/school-notifications').filter((r) => new URLSearchParams(r.search).get('limit') !== '50' || new URLSearchParams(r.search).has('created_from'));
    const from = new URLSearchParams(listCalls().at(-1)!.search).get('created_from');
    expect(from).toBeTruthy();
    const days = (Date.now() - Date.parse(from!)) / 86_400_000;
    expect(days).toBeGreaterThan(29.99);
    expect(days).toBeLessThan(30.01);
    expect(screen.queryByText('Vụ việc cũ hơn 30 ngày')).toBeNull();
    await pick(user, 'Khoảng thời gian', 'Mọi thời gian');
    await waitFor(() => expect(new URLSearchParams(backend.calls('GET', '/tenant/safety/school-notifications').at(-1)!.search).has('created_from')).toBe(false));
  });

  it('409 CONFLICT → "Thông báo đã được xác nhận" and the list is reloaded', async () => {
    const { backend, user } = await renderSafety(`/school-lms/safety-notifications/${CS.notifA}`, { account: 'ad.truong.a' });
    await screen.findByTestId('notification-detail');
    backend.overrides.unshift((r) => (r.path.endsWith(':acknowledge') ? problem(409, 'CONFLICT') : undefined));
    const before = backend.calls('GET', '/tenant/safety/school-notifications').length;
    await user.click(screen.getByRole('radio', { name: 'Không cần liên hệ' }));
    await user.click(screen.getByRole('button', { name: 'Xác nhận đã liên hệ phụ huynh' }));
    expect(await screen.findByText('Thông báo đã được xác nhận. Đã tải lại.')).toBeInTheDocument();
    await waitFor(() => expect(backend.calls('GET', '/tenant/safety/school-notifications').length).toBeGreaterThan(before));
  });

  it('F122-AC-20: 390 px → cards and a detail page with the confirm button', async () => {
    const { user } = await renderSafety('/school-lms/safety-notifications', { account: 'ad.truong.a', width: 390 });
    const cards = await screen.findByTestId('responsive-cards');
    expect(screen.queryByRole('table')).toBeNull();
    await user.click((await within(cards).findAllByRole('link'))[0]!);
    expect(await screen.findByTestId('notification-detail')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Xác nhận đã liên hệ phụ huynh' })).toBeInTheDocument();
  });

  it('tenant isolation: ad.truong.b sees none of TH Demo A and no badge', async () => {
    const { backend } = await renderSafety('/school-lms/safety-notifications', { account: 'ad.truong.b' });
    await screen.findByRole('navigation', { name: 'Menu điều hướng' });
    await waitFor(() => expect(backend.calls('GET', '/tenant/safety/school-notifications').length).toBeGreaterThan(0));
    expect(screen.queryByText(/Nguyễn An/)).toBeNull();
    expect(screen.queryByTestId('safety-notifications-badge')).toBeNull();
    expect(backend.calls('GET', '/tenant/safety/school-notifications').every((r) => r.headers['x-tenant-id'] !== T_A)).toBe(true);
  });
});

describe('Chính sách chat trường (F122-AC-21, AC-22, AC-28) và nhóm từ (F122-AC-17)', () => {
  it('per channel type with limits from above and "Kế thừa"; match/team/party locked; no class/grade/real-name control', async () => {
    const { backend } = await renderSafety('/school-lms/chat-safety', { account: 'ad.truong.a' });
    const direct = await screen.findByTestId('policy-card-direct');
    expect(within(direct).getByText('Kế thừa')).toBeInTheDocument();
    expect(within(direct).getByText(/Cấp trên: 06:30–21:30 \(chỉ thu hẹp được\)/)).toBeInTheDocument();
    expect(screen.getByText(/chưa có chính sách riêng/)).toBeInTheDocument();
    for (const ch of ['match', 'team', 'party']) {
      const card = screen.getByTestId(`policy-card-${ch}`);
      expect(within(card).getByText('Luôn bật')).toBeInTheDocument();
      expect(within(card).queryByRole('switch')).toBeNull();
      expect(within(card).getByRole('spinbutton')).toHaveAttribute('aria-valuemax', '10');
    }
    const text = document.body.textContent ?? '';
    expect(text).not.toMatch(/Lớp|lớp học|study_group|Nhóm học tập|tên thật|Khối \d|độ tuổi/i);
    expect(screen.queryByTestId('policy-card-class')).toBeNull();
    // the school policy (404 = none) and the effective policy of every channel type were read with the session tenant
    expect(backend.calls('GET', `/tenant/chat-policies/school/${CS.schoolA}`)[0]!.headers['x-tenant-id']).toBe(T_A);
    expect(new Set(backend.calls('GET', '/tenant/chat-policies:effective').map((r) => new URLSearchParams(r.search).get('channel_type')))).toEqual(new Set(['direct', 'world', 'area', 'team', 'match', 'party']));
  });

  it('save: If-Match "0", reason required, preset for the whole school; RELAX error lands on the channel card', async () => {
    const { backend, user, state } = await renderSafety('/school-lms/chat-safety', { account: 'ad.truong.a' });
    const direct = await screen.findByTestId('policy-card-direct');
    await user.click(within(direct).getByRole('switch', { name: 'Bật Nhắn riêng' }));
    await user.click(within(screen.getByTestId('preset-card')).getByRole('radio', { name: /Chỉ tin nhắn nhanh/ }));
    expect(screen.getByText(/mọi kênh, kể cả trận\/đội\/nhóm chơi/)).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'Lưu' }));
    expect(await screen.findAllByText(/bắt buộc/i)).not.toHaveLength(0);
    expect(backend.calls('PUT', `/tenant/chat-policies/school/${CS.schoolA}`)).toHaveLength(0);
    const reason = screen.getByLabelText('Lý do');
    expect(reason).toHaveAttribute('maxlength', '500');
    // relax refused by the server → shown on the direct card
    backend.overrides.unshift((r) => (r.method === 'PUT' ? problem(422, 'CHAT_POLICY_RELAX_NOT_ALLOWED', { channel_type: 'direct', field: 'allowed_hours' }) : undefined));
    fireEvent.change(reason, { target: { value: 'Trường tắt nhắn riêng theo họp phụ huynh' } });
    await user.click(screen.getByRole('button', { name: 'Lưu' }));
    expect(await within(screen.getByTestId('policy-card-direct')).findByText(/Cấp trường chỉ được siết chặt hơn cấp trên\. \(giờ chat\)/)).toBeInTheDocument();
    backend.overrides.shift();
    await user.click(screen.getByRole('button', { name: 'Lưu' }));
    expect(await screen.findByText('Đã lưu chính sách chat của trường.')).toBeInTheDocument();
    const put = backend.calls('PUT', `/tenant/chat-policies/school/${CS.schoolA}`).at(-1)!;
    expect(put.headers['if-match']).toBe('"0"');
    expect(put.headers['x-tenant-id']).toBe(T_A);
    expect(put.body).toEqual({ rules: [{ channel_type: 'direct', enabled: false, allowed_kinds: [] }], chat_policy_preset: 'quick_message_emote_only', reason: 'Trường tắt nhắn riêng theo họp phụ huynh' });
    expect(state.schoolPolicy?.version).toBe(1);
    // after saving the card is the school's own rule and the next save sends If-Match "1"
    expect(await within(screen.getByTestId('policy-card-direct')).findByText('Trường đặt')).toBeInTheDocument();
  });

  it('F122-AC-22: CHAT_POLICY_ALWAYS_ON and VALIDATION_FAILED by code', async () => {
    const { backend, user } = await renderSafety('/school-lms/chat-safety', { account: 'ad.truong.a' });
    const match = await screen.findByTestId('policy-card-match');
    fireEvent.change(within(match).getByRole('spinbutton'), { target: { value: '8' } });
    fireEvent.blur(within(match).getByRole('spinbutton'));
    backend.overrides.unshift((r) => (r.method === 'PUT' ? problem(422, 'CHAT_POLICY_ALWAYS_ON', { channel_type: 'match', field: 'enabled' }) : undefined));
    fireEvent.change(screen.getByLabelText('Lý do'), { target: { value: 'Giảm tốc độ chat trong trận' } });
    await user.click(screen.getByRole('button', { name: 'Lưu' }));
    expect(await within(screen.getByTestId('policy-card-match')).findByText(/luôn bật, không tắt hay giới hạn giờ được/)).toBeInTheDocument();
    backend.overrides.shift();
    backend.overrides.unshift((r) => (r.method === 'PUT' ? problem(422, 'VALIDATION_FAILED', { errors: [{ field: 'reason', code: 'TOO_SHORT' }] }) : undefined));
    await user.click(screen.getByRole('button', { name: 'Lưu' }));
    expect(await screen.findByText('Dữ liệu nhập chưa hợp lệ. Kiểm tra các trường được đánh dấu.')).toBeInTheDocument();
  });

  it('F122-AC-17: tenant categories read only — names, severity, state; no term and no write button', async () => {
    const { user } = await renderSafety('/school-lms/chat-safety', { account: 'ad.truong.a' });
    await user.click(await screen.findByRole('tab', { name: 'Nhóm từ' }));
    expect(await screen.findByText('Chính trị, tôn giáo')).toBeInTheDocument();
    expect(screen.getByText('Xúc phạm')).toBeInTheDocument();
    expect(document.body.textContent).not.toMatch(/zqplaceholder|zqtenantonly/);
    const panel = screen.getByRole('tabpanel');
    expect(within(panel).queryByRole('button')).toBeNull();
    expect(within(panel).queryByRole('switch')).toBeNull();
  });
});

describe('Club của trường (F122-AC-23)', () => {
  it('clubs of the school waiting for approval; approve / archive with confirmation and Idempotency-Key; no chat screen', async () => {
    const { backend, user, state } = await renderSafety('/school-lms/clubs', { account: 'ad.truong.a' });
    expect(await screen.findByText('Nhà thám hiểm')).toBeInTheDocument();
    const q = new URLSearchParams(backend.calls('GET', '/social/clubs')[0]!.search);
    expect([q.get('school_id'), q.get('status')]).toEqual([CS.schoolA, 'pending_approval']);
    expect(backend.calls('GET', '/social/clubs')[0]!.headers['x-tenant-id']).toBe(T_A);
    await user.click(screen.getByRole('button', { name: 'Duyệt' }));
    const dialog = await screen.findByRole('dialog');
    await user.click(within(dialog).getByRole('button', { name: 'Duyệt' }));
    await waitFor(() => expect(backend.calls('POST', `/social/clubs/${CS.clubPending}:approve`)).toHaveLength(1));
    expect(backend.calls('POST', `/social/clubs/${CS.clubPending}:approve`)[0]!.headers['idempotency-key']).toBeTruthy();
    expect(state.clubs[0]!.status).toBe('active');
    await pick(user, 'Trạng thái', 'Đang hoạt động');
    const row = (await screen.findByText('CLB Cờ vua')).closest('tr')!;
    await user.click(within(row).getByRole('button', { name: 'Lưu trữ' }));
    await user.click(within(await screen.findByRole('dialog', { name: /Lưu trữ club "CLB Cờ vua"/ })).getByRole('button', { name: 'Lưu trữ' }));
    await waitFor(() => expect(backend.calls('POST', `/social/clubs/${CS.clubActive}:archive`)).toHaveLength(1));
    expect(screen.queryByText(/tin nhắn|Xem chat/i)).toBeNull();
  });
});

describe('Thành viên club (F122-AC-23, T061)', () => {
  it('members of a club (cursor pages, nicknames only) and "Gỡ thành viên" with reason, confirmation and Idempotency-Key; never the owner', async () => {
    const { backend, user, state } = await renderSafety('/school-lms/clubs', { account: 'ad.truong.a' });
    expect(await screen.findByText('Nhà thám hiểm')).toBeInTheDocument();
    await pick(user, 'Trạng thái', 'Đang hoạt động');
    const row = (await screen.findByText('CLB Cờ vua')).closest('tr')!;
    await user.click(within(row).getByRole('button', { name: 'Thành viên' }));
    const panel = await screen.findByRole('dialog', { name: /Thành viên club "CLB Cờ vua"/ });
    expect(await within(panel).findByText('CaVang')).toBeInTheDocument();
    const members = `/tenant/social/clubs/${CS.clubActive}/members`;
    expect(backend.calls('GET', members)[0]!.headers['x-tenant-id']).toBe(T_A);
    // page size 2 → "Tải thêm" brings the third member with the cursor
    expect(within(panel).queryByText('ThoNau')).toBeNull();
    await user.click(within(panel).getByRole('button', { name: 'Tải thêm' }));
    expect(await within(panel).findByText('ThoNau')).toBeInTheDocument();
    expect(new URLSearchParams(backend.calls('GET', members).at(-1)!.search).get('cursor')).toBe('2');
    // the owner cannot be removed here
    const ownerRow = within(panel).getByText('CaVang').closest('tr')!;
    expect(within(ownerRow).queryByRole('button', { name: 'Gỡ thành viên' })).toBeNull();
    const binhRow = within(panel).getByText('MeoCon').closest('tr')!;
    await user.click(within(binhRow).getByRole('button', { name: 'Gỡ thành viên' }));
    const confirm = await within(panel).findByRole('group', { name: /Gỡ "MeoCon" khỏi club/ });
    const reason = within(confirm).getByLabelText('Lý do gỡ');
    expect(reason).toHaveAttribute('maxlength', '500');
    await user.click(within(confirm).getByRole('button', { name: 'Gỡ thành viên' }));
    expect(await within(confirm).findByText(/Nhập lý do/)).toBeInTheDocument();
    const path = `/tenant/social/clubs/${CS.clubActive}/members/${CS.memberBinh}:remove`;
    expect(backend.calls('POST', path)).toHaveLength(0);
    fireEvent.change(reason, { target: { value: 'Vi phạm nội quy club nhiều lần' } });
    await user.click(within(confirm).getByRole('button', { name: 'Gỡ thành viên' }));
    await waitFor(() => expect(backend.calls('POST', path)).toHaveLength(1));
    const call = backend.calls('POST', path)[0]!;
    expect(call.headers['idempotency-key']).toBeTruthy();
    expect(call.headers['x-tenant-id']).toBe(T_A);
    expect(call.body).toEqual({ reason: 'Vi phạm nội quy club nhiều lần' });
    expect(state.clubMembers[CS.clubActive]!.map((m) => m.nickname)).toEqual(['CaVang', 'ThoNau']);
    await waitFor(() => expect(within(panel).queryByText('MeoCon')).toBeNull());
    expect(document.body.textContent).not.toMatch(/Nguyễn|tin nhắn|Xem chat/);
  });

  it('club approve action off → the members are listed but there is no "Gỡ thành viên"', async () => {
    const { user } = await renderSafety('/school-lms/clubs', { account: 'ad.truong.a', disabledActions: ['action.school_lms.clubs.approve'] });
    expect(await screen.findByText('Nhà thám hiểm')).toBeInTheDocument();
    await pick(user, 'Trạng thái', 'Đang hoạt động');
    const row = (await screen.findByText('CLB Cờ vua')).closest('tr')!;
    await user.click(within(row).getByRole('button', { name: 'Thành viên' }));
    const panel = await screen.findByRole('dialog', { name: /Thành viên club "CLB Cờ vua"/ });
    expect(await within(panel).findByText('MeoCon')).toBeInTheDocument();
    expect(within(panel).queryByRole('button', { name: 'Gỡ thành viên' })).toBeNull();
  });
});

describe('Menu/action đang tắt (F122-AC-29)', () => {
  it('chat-safety menu off → hidden and its URL shows "Tính năng đang tắt"; club approve action off → no button', async () => {
    const { backend } = await renderSafety('/school-lms/chat-safety', { account: 'ad.truong.a', disabledMenus: ['menu.school_lms.chat_safety'], disabledActions: ['action.school_lms.clubs.approve'] });
    expect(await screen.findByTestId('feature-disabled')).toBeInTheDocument();
    const nav = screen.getByRole('navigation', { name: 'Menu điều hướng' });
    expect(within(nav).queryByText('Chat policy trường')).toBeNull();
    expect(within(nav).getByText('Thông báo an toàn của trường')).toBeInTheDocument(); // protected menu, cannot be turned off
    expect(backend.calls('GET', '/tenant/chat/categories')).toHaveLength(0);
    fireEvent.click(within(nav).getByText('Club của học sinh'));
    expect(await screen.findByText('Nhà thám hiểm')).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Duyệt' })).toBeNull();
  });

  it('403 FEATURE_DISABLED from a chat-safety API → notice and permissions reloaded', async () => {
    const { backend } = await renderSafety('/school-lms/clubs', { account: 'ad.truong.a' });
    await screen.findByText('Nhà thám hiểm');
    const before = backend.calls('GET', '/me/permissions').length;
    backend.overrides.unshift((r) => (r.path.endsWith(':approve') ? problem(403, 'FEATURE_DISABLED', { feature_key: 'action.school_lms.clubs.approve' }) : undefined));
    fireEvent.click(screen.getByRole('button', { name: 'Duyệt' }));
    fireEvent.click(within(await screen.findByRole('dialog')).getByRole('button', { name: 'Duyệt' }));
    expect((await screen.findAllByText(/tạm tắt|Tính năng đang tắt/)).length).toBeGreaterThan(0);
    await waitFor(() => expect(backend.calls('GET', '/me/permissions').length).toBeGreaterThan(before));
  });
});
