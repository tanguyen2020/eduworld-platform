// F080 T037 — "Giám sát sự kiện" of workspace E (TC-EVT-003). Backend = MOCK (test/liveops-mock.ts, test only, AGENTS R17).
// F080-AC-19: runs (live + suspended by default, status filter, cursor), getEventRunStats of the selected run, Tạm dừng /
// Tiếp tục / Hủy only with game.event.operate (ActionGate action.game_ops.events.operate), reason required (3–1000 with a
// counter), 2-step confirmation, Idempotency-Key per action, 409 → notice + the run reloaded.
import { fireEvent, screen, waitFor, within } from '@testing-library/react';
import { afterEach, describe, expect, it } from 'vitest';
import { LO, renderLiveOps } from '../../../test/liveops-mock';
import { problem, stopPortals } from '../../../test/portal-harness';

afterEach(() => stopPortals());

const URL = '/live-ops/live-dashboard';

async function openMonitor(account = 'lo.vanhanh') {
  const r = await renderLiveOps(URL, { account });
  await r.user.click(await screen.findByRole('tab', { name: 'Giám sát sự kiện' }));
  await screen.findByText('Lễ hội Mùa Thu');
  return r;
}

describe('Giám sát sự kiện (F080-AC-19)', () => {
  it('F080-AC-19 lists live + suspended runs and shows getEventRunStats of the selected run', async () => {
    const { backend, user } = await openMonitor();
    const q = new URLSearchParams(backend.calls('GET', '/game-admin/event-runs')[0]!.search);
    expect(q.getAll('status')).toEqual(['live', 'suspended']);
    expect(q.get('limit')).toBe('20');
    expect(screen.getByText('Mục tiêu chung')).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'Lễ hội Mùa Thu' }));
    const detail = await screen.findByTestId('run-detail');
    await waitFor(() => expect(within(detail).getByTestId('stat-participants')).toHaveTextContent('321'));
    expect(detail).toHaveTextContent('500 / 1.000');
    expect(detail).toHaveTextContent('Mốc m1: 40 lượt');
    expect(backend.calls('GET', `/game-admin/events/${LO.event1}/runs/${LO.run1}/stats`)).toHaveLength(1);
  });

  it('F080-AC-19 Tạm dừng needs a reason (min 3, counter, max 1000), a second confirmation and sends an Idempotency-Key', async () => {
    const { backend, user } = await openMonitor();
    await user.click(screen.getByRole('button', { name: 'Lễ hội Mùa Thu' }));
    const detail = await screen.findByTestId('run-detail');
    // live run: suspend + cancel, no resume
    expect(within(detail).queryByRole('button', { name: 'Tiếp tục' })).toBeNull();
    await user.click(within(detail).getByRole('button', { name: 'Tạm dừng khẩn cấp' }));
    const dialog = await screen.findByRole('dialog');
    await user.click(within(dialog).getByRole('button', { name: 'Tiếp tục' }));
    expect(await within(dialog).findByText('Nhập lý do.')).toBeInTheDocument();
    const reason = within(dialog).getByRole('textbox', { name: 'Lý do' });
    expect(reason).toHaveAttribute('maxlength', '1000');
    fireEvent.change(reason, { target: { value: 'ab' } });
    await user.click(within(dialog).getByRole('button', { name: 'Tiếp tục' }));
    expect(await within(dialog).findByText('Lý do tối thiểu 3 ký tự.')).toBeInTheDocument();
    fireEvent.change(reason, { target: { value: 'Lỗi trao thưởng' } });
    expect(within(dialog).getByText('15 / 1000')).toBeInTheDocument();
    await user.click(within(dialog).getByRole('button', { name: 'Tiếp tục' }));
    // step 2: nothing sent yet
    const confirm = await within(dialog).findByTestId('run-op-confirm');
    expect(confirm).toHaveTextContent('Lý do: Lỗi trao thưởng');
    expect(backend.requests.filter((r) => r.method === 'POST' && /:suspend$/.test(r.path))).toHaveLength(0);
    await user.click(within(confirm).getByRole('button', { name: 'Xác nhận tạm dừng' }));
    await waitFor(() => expect(backend.calls('POST', `/game-admin/events/${LO.event1}/runs/${LO.run1}:suspend`)).toHaveLength(1));
    const call = backend.calls('POST', `/game-admin/events/${LO.event1}/runs/${LO.run1}:suspend`)[0]!;
    expect(call.body).toEqual({ reason: 'Lỗi trao thưởng' });
    expect(call.headers['idempotency-key']).toMatch(/^[0-9a-f-]{36}$/);
    // the run reloads: now suspended → Tiếp tục offered
    await waitFor(() => expect(within(screen.getByTestId('run-detail')).getByRole('button', { name: 'Tiếp tục' })).toBeInTheDocument());
  });

  it('F080-AC-19 each user action gets its own Idempotency-Key', async () => {
    const { backend, user } = await openMonitor();
    await user.click(screen.getByRole('button', { name: 'Mục tiêu chung' }));
    const run = async (button: string, confirm: string) => {
      await user.click(within(await screen.findByTestId('run-detail')).getByRole('button', { name: button }));
      const dialog = await screen.findByRole('dialog');
      fireEvent.change(within(dialog).getByRole('textbox', { name: 'Lý do' }), { target: { value: 'Đã khắc phục' } });
      await user.click(within(dialog).getByRole('button', { name: 'Tiếp tục' }));
      await user.click(await within(dialog).findByRole('button', { name: confirm }));
      await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull());
    };
    await run('Tiếp tục', 'Xác nhận tiếp tục');
    await run('Tạm dừng khẩn cấp', 'Xác nhận tạm dừng');
    const keys = backend.requests.filter((r) => r.method === 'POST' && r.path.includes(LO.run2)).map((r) => r.headers['idempotency-key']);
    expect(keys).toHaveLength(2);
    expect(new Set(keys).size).toBe(2);
  });

  it('F080-AC-19 409 (already suspended) → notice, dialog closed and the run reloaded', async () => {
    const { backend, user, state } = await openMonitor();
    await user.click(screen.getByRole('button', { name: 'Lễ hội Mùa Thu' }));
    state.runOpAnswer = problem(409, 'INVALID_STATE_TRANSITION');
    state.runs[0]!.status = 'suspended'; // changed elsewhere meanwhile
    const before = backend.calls('GET', '/game-admin/event-runs').length;
    await user.click(within(await screen.findByTestId('run-detail')).getByRole('button', { name: 'Tạm dừng khẩn cấp' }));
    const dialog = await screen.findByRole('dialog');
    fireEvent.change(within(dialog).getByRole('textbox', { name: 'Lý do' }), { target: { value: 'Sự cố khẩn' } });
    await user.click(within(dialog).getByRole('button', { name: 'Tiếp tục' }));
    await user.click(await within(dialog).findByRole('button', { name: 'Xác nhận tạm dừng' }));
    expect(await screen.findByText('Trạng thái sự kiện vừa thay đổi. Đã tải lại trạng thái mới.')).toBeInTheDocument();
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull());
    await waitFor(() => expect(backend.calls('GET', '/game-admin/event-runs').length).toBeGreaterThan(before));
    await waitFor(() => expect(within(screen.getByTestId('run-detail')).getByRole('button', { name: 'Tiếp tục' })).toBeInTheDocument());
  });

  it('F080-AC-19 without game.event.operate the stats show but no Tạm dừng / Tiếp tục / Hủy', async () => {
    const { user } = await openMonitor('lo.xem');
    await user.click(screen.getByRole('button', { name: 'Lễ hội Mùa Thu' }));
    const detail = await screen.findByTestId('run-detail');
    await waitFor(() => expect(within(detail).getByTestId('stat-participants')).toHaveTextContent('321'));
    expect(within(detail).queryByTestId('run-actions')).toBeNull();
  });

  it('F080-AC-19 status filter asks listEventRuns with the chosen statuses', async () => {
    const { backend, user } = await openMonitor();
    await user.click(screen.getByRole('combobox', { name: 'Trạng thái lượt chạy' }));
    await user.click((await screen.findAllByTitle('Đã kết thúc')).at(-1)!);
    await waitFor(() => expect(new URLSearchParams(backend.calls('GET', '/game-admin/event-runs').at(-1)!.search).getAll('status')).toEqual(['live', 'suspended', 'ended']));
  });
});
