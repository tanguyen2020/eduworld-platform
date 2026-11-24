// F079 T012–T015 — the shared versioned editor of workspace A. Backend = MOCK (test/game-ops-mock.ts, test only, R17).
// AC-02: form from the type's JSON Schema; draft saved with If-Match; 412 / 409 CONTENT_VERSION_CONFLICT stops autosave and
//        offers "Tải bản mới".
// AC-03: transition + publish with Idempotency-Key; "Duyệt" hidden for the author (the server still decides).
// AC-04: published version read only + "Tạo bản nháp từ"; diff by field; releases; rollback / unpublish need a reason.
// AC-05: 422 VALIDATION_FAILED / TRANSLATION_INCOMPLETE mapped to the field and "Gửi duyệt" blocked.
import { configure, fireEvent, screen, waitFor, within } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { GAME_ACCOUNTS, renderGameOps } from '../../../test/game-ops-mock';
import { problem, stopPortals } from '../../../test/portal-harness';

vi.setConfig({ testTimeout: 600_000, hookTimeout: 120_000 });
configure({ asyncUtilTimeout: 60_000 });
afterEach(() => stopPortals());

const DESIGNER = GAME_ACCOUNTS['gd.thietke']!.sub;
const RULE = { code: 'xp.quiz', source_code: 'quiz_completed', trigger: { event: 'activity.completed' }, grants: [{ type: 'xp', amount: 10 }], caps: { xp_per_day: 200 } };

const ruleUrl = (id: string, v = 1) => `/game-ops/rewards/reward-rules/${id}/v/${v}`;

describe('Soạn và tự động lưu (F079-AC-02)', () => {
  it('saves the draft with If-Match of the last ETag; a 412 stops autosave and offers "Tải bản mới"', async () => {
    let id = '';
    const { backend, user, router } = await renderGameOps('/', {
      setup: (m) => (id = m.add('reward-rules', [{ status: 'draft', author_id: DESIGNER, payload: RULE }]).config_id),
    });
    await router.navigate(ruleUrl(id));
    const code = await screen.findByLabelText('Mã');
    expect(code).toHaveAttribute('maxlength', '64');
    fireEvent.change(code, { target: { value: 'xp.quiz2' } });
    await user.click(screen.getByRole('button', { name: 'Lưu' }));
    await waitFor(() => expect(backend.calls('PATCH', `/game-admin/reward-rules/${id}/versions/1`)).toHaveLength(1));
    const first = backend.calls('PATCH', `/game-admin/reward-rules/${id}/versions/1`)[0]!;
    expect(first.headers['if-match']).toBe('"1"');
    expect((first.body as { payload: { code: string } }).payload.code).toBe('xp.quiz2');
    // someone else saved meanwhile → 412
    backend.overrides.unshift((r) => (r.method === 'PATCH' ? problem(412, 'PRECONDITION_FAILED') : undefined));
    fireEvent.change(screen.getByLabelText('Mã'), { target: { value: 'xp.quiz3' } });
    await user.click(screen.getByRole('button', { name: 'Lưu' }));
    expect(await screen.findByText('Người khác đã sửa bản nháp này')).toBeInTheDocument();
    expect(backend.calls('PATCH', `/game-admin/reward-rules/${id}/versions/1`)[1]!.headers['if-match']).toBe('"2"');
    expect(screen.getByRole('button', { name: 'Tải bản mới' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Lưu' })).toBeDisabled();
  });
});

describe('Workflow (F079-AC-03)', () => {
  it('author: "Gửi duyệt" sends transition with Idempotency-Key + If-Match; no "Duyệt" on own version', async () => {
    let id = '';
    const { backend, user, router } = await renderGameOps('/', { setup: (m) => (id = m.add('reward-rules', [{ status: 'draft', author_id: DESIGNER, payload: RULE }]).config_id) });
    await router.navigate(ruleUrl(id));
    await user.click(await screen.findByRole('button', { name: 'Gửi duyệt' }));
    await waitFor(() => expect(backend.calls('POST', `/game-admin/reward-rules/${id}/versions/1:transition`)).toHaveLength(1));
    const call = backend.calls('POST', `/game-admin/reward-rules/${id}/versions/1:transition`)[0]!;
    expect(call.body).toEqual({ action: 'submit_review' });
    expect(call.headers['idempotency-key']).toBeTruthy();
    expect(call.headers['if-match']).toBeTruthy();
    expect(await screen.findByText('Đang duyệt')).toBeInTheDocument();
    // the designer holds game.content.review, yet "Duyệt" is hidden on their own version
    expect(screen.queryByRole('button', { name: 'Duyệt' })).toBeNull();
  });

  it('reviewer: approve, reject with a mandatory note', async () => {
    let id = '';
    const r = await renderGameOps('/', { account: 'cm.duyet', setup: (m) => (id = m.add('reward-rules', [{ status: 'in_review', author_id: DESIGNER, payload: RULE }]).config_id) });
    await r.router.navigate(ruleUrl(id));
    await r.user.click(await screen.findByRole('button', { name: 'Từ chối' }));
    const dialog = await screen.findByRole('dialog');
    await r.user.click(within(dialog).getByRole('button', { name: /OK|Xác nhận/ }));
    expect(r.backend.calls('POST', `/game-admin/reward-rules/${id}/versions/1:transition`)).toHaveLength(0);
    fireEvent.change(within(dialog).getByRole('textbox'), { target: { value: 'Thiếu cap tuần' } });
    await r.user.click(within(dialog).getByRole('button', { name: /OK|Xác nhận/ }));
    await waitFor(() => expect(r.backend.calls('POST', `/game-admin/reward-rules/${id}/versions/1:transition`)).toHaveLength(1));
    expect(r.backend.calls('POST', `/game-admin/reward-rules/${id}/versions/1:transition`)[0]!.body).toEqual({ action: 'reject', note: 'Thiếu cap tuần' });
  });

  it('Live-ops publishes an approved version now (Idempotency-Key, If-Match)', async () => {
    let id = '';
    const r = await renderGameOps('/', { account: 'lo.vanhanh', setup: (m) => (id = m.add('events', [{ status: 'approved', author_id: DESIGNER, payload: { code: 'math_week', event_kind: 'learning_campaign' } }]).config_id) });
    await r.router.navigate(`/game-ops/events/events/${id}/v/1`);
    await r.user.click(await screen.findByRole('button', { name: 'Xuất bản ngay' }));
    const dialog = await screen.findByRole('dialog', { name: 'Xuất bản v1' });
    await r.user.click(within(dialog).getByRole('button', { name: 'Xuất bản ngay' }));
    await waitFor(() => expect(r.backend.calls('POST', `/game-admin/events/${id}/versions/1:publish`)).toHaveLength(1));
    const call = r.backend.calls('POST', `/game-admin/events/${id}/versions/1:publish`)[0]!;
    expect(call.headers['idempotency-key']).toBeTruthy();
    expect(call.headers['if-match']).toBe('"1"');
    expect(call.body).toEqual({});
  });
});

describe('Phiên bản, diff, rollback, gỡ (F079-AC-04)', () => {
  it('published version is read only with "Tạo bản nháp từ"; compare; rollback and unpublish need a reason', async () => {
    let id = '';
    const r = await renderGameOps('/', {
      account: 'sa.admin',
      setup: (m) => {
        const c = m.add('reward-rules', [
          { status: 'published', author_id: DESIGNER, payload: RULE, published_at: '2026-11-01T01:00:00Z' },
          { status: 'published', author_id: DESIGNER, payload: { ...RULE, code: 'xp.quiz.v2' }, published_at: '2026-11-10T01:00:00Z' },
        ]);
        c.current = 2;
        id = c.config_id;
      },
    });
    await r.router.navigate(ruleUrl(id, 2));
    expect(await screen.findByRole('button', { name: 'Tạo bản nháp từ v2' })).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Lưu' })).toBeNull();
    expect(screen.getByLabelText('Mã')).toBeDisabled();
    await r.user.click(screen.getByRole('tab', { name: 'Phiên bản' }));
    const panel = await screen.findByTestId('versions-panel');
    // reward rules have no rollback (F078): only unpublish
    expect(within(panel).queryByRole('button', { name: /Rollback về/ })).toBeNull();
    await r.user.click(within(panel).getByRole('checkbox', { name: 'Chọn v1' }));
    await r.user.click(within(panel).getByRole('checkbox', { name: 'Chọn v2' }));
    await r.user.click(within(panel).getByRole('button', { name: 'So sánh 2 phiên bản' }));
    expect(await screen.findByText('/name/vi')).toBeInTheDocument();
    expect(r.backend.calls('GET', `/game-admin/reward-rules/${id}/versions/2:diff`)[0]!.search).toContain('with=1');
    await r.user.click(within(panel).getByRole('button', { name: 'Gỡ xuất bản' }));
    const dialog = await screen.findByRole('dialog', { name: 'Gỡ xuất bản' });
    await r.user.click(within(dialog).getByRole('button', { name: /OK|Xác nhận/ }));
    expect(r.backend.calls('POST', `/game-admin/reward-rules/${id}:unpublish`)).toHaveLength(0);
    fireEvent.change(within(dialog).getByRole('textbox'), { target: { value: 'Sai cap' } });
    await r.user.click(within(dialog).getByRole('button', { name: /OK|Xác nhận/ }));
    await waitFor(() => expect(r.backend.calls('POST', `/game-admin/reward-rules/${id}:unpublish`)).toHaveLength(1));
    const call = r.backend.calls('POST', `/game-admin/reward-rules/${id}:unpublish`)[0]!;
    expect(call.body).toEqual({ reason: 'Sai cap' });
    expect(call.headers['idempotency-key']).toBeTruthy();
  });

  it('rollback to an older published version (quests): reason required, rollbackGameConfig, release timeline', async () => {
    let id = '';
    const r = await renderGameOps('/', {
      account: 'sa.admin',
      setup: (m) => {
        const c = m.add('quests', [
          { status: 'published', author_id: DESIGNER, payload: { code: 'q1' } },
          { status: 'published', author_id: DESIGNER, payload: { code: 'q1' } },
        ]);
        c.current = 2;
        c.releases.push({ release_id: m.newId(), item_type: 'quests', item_id: c.config_id, action: 'publish', to_version: 2, status: 'applied', released_by: DESIGNER, released_at: '2026-11-10T01:00:00Z' });
        id = c.config_id;
      },
    });
    await r.router.navigate(`/game-ops/quests/quests/${id}/v/2`);
    await r.user.click(await screen.findByRole('tab', { name: 'Phiên bản' }));
    await r.user.click(await screen.findByRole('button', { name: 'Rollback về v1' }));
    const dialog = await screen.findByRole('dialog', { name: 'Rollback v2 → v1' });
    fireEvent.change(within(dialog).getByRole('textbox'), { target: { value: 'Lỗi bước 2' } });
    await r.user.click(within(dialog).getByRole('button', { name: /OK|Xác nhận/ }));
    await waitFor(() => expect(r.backend.calls('POST', `/game-admin/quests/${id}:rollback`)).toHaveLength(1));
    expect(r.backend.calls('POST', `/game-admin/quests/${id}:rollback`)[0]!.body).toEqual({ to_version: 1, reason: 'Lỗi bước 2' });
    await r.user.click(screen.getByRole('tab', { name: 'Lịch sử phát hành' }));
    expect(await screen.findByText('Rollback v2 → v1')).toBeInTheDocument();
    expect(screen.getByText('Xuất bản v2')).toBeInTheDocument();
  });
});

describe('Lỗi validate của server (F079-AC-05)', () => {
  it('422 VALIDATION_FAILED on save → message on the field; "Gửi duyệt" blocked; TRANSLATION_INCOMPLETE on submit → field', async () => {
    let id = '';
    const r = await renderGameOps('/', { setup: (m) => (id = m.add('reward-rules', [{ status: 'draft', author_id: DESIGNER, payload: RULE }]).config_id) });
    await r.router.navigate(ruleUrl(id));
    r.backend.overrides.unshift((q) =>
      q.method === 'PATCH' ? problem(422, 'VALIDATION_FAILED', { errors: [{ field: 'payload/caps/xp_per_day', code: 'OUT_OF_RANGE', message: 'cap > 0' }] }) : undefined,
    );
    fireEvent.change(await screen.findByLabelText('Mã'), { target: { value: 'xp.bad' } });
    await r.user.click(screen.getByRole('button', { name: 'Lưu' }));
    expect(await screen.findAllByText(/Giá trị chưa hợp lệ \(OUT_OF_RANGE\)/)).not.toHaveLength(0);
    expect(screen.getByRole('button', { name: 'Gửi duyệt' })).toBeDisabled();
    expect(screen.getByText('Chưa thể gửi duyệt hoặc xuất bản, còn thiếu:')).toBeInTheDocument();
    expect(r.backend.calls('POST', `/game-admin/reward-rules/${id}/versions/1:transition`)).toHaveLength(0);
  });

  it('422 TRANSLATION_INCOMPLETE on submit_review → error on name/en', async () => {
    let id = '';
    const r = await renderGameOps('/', {
      setup: (m) => (id = m.add('achievements', [{ status: 'draft', author_id: DESIGNER, payload: { code: 'a1', name: { vi: 'Nhà thám hiểm' } } }]).config_id),
    });
    await r.router.navigate(`/game-ops/achievements/achievements/${id}/v/1`);
    r.backend.overrides.unshift((q) => (q.path.endsWith(':transition') ? problem(422, 'TRANSLATION_INCOMPLETE', { errors: [{ field: 'name.en', code: 'TRANSLATION_INCOMPLETE' }] }) : undefined));
    await r.user.click(await screen.findByRole('button', { name: 'Gửi duyệt' }));
    expect(await screen.findAllByText(/TRANSLATION_INCOMPLETE|Thiếu bản ngôn ngữ/)).not.toHaveLength(0);
    expect(screen.getByRole('button', { name: 'Gửi duyệt' })).toBeDisabled();
  });
});

