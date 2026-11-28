// F136 T036 (portal part) — F136-AC-22 (TC-GAM-014), F136-AC-19: the case detail of the Safety Team shows the server
// snapshot of a reported nickname (evidence.reported_nickname) as PLAIN TEXT labelled "Nickname bị báo cáo", and offers
// the case action "Đặt lại nickname" (actOnModerationCase action = nickname_reset, note 3–1000 required, target student
// of the case, If-Match = case version, X-Tenant-Id = tenant of the case, Idempotency-Key) where the other case actions
// are (ActionGate action.live_ops.moderation_cases.act — display only). Backend = MOCK (test/chat-safety-mock.ts).
import { configure, fireEvent, screen, waitFor, within } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { chatSafetyState, CS, renderSafety, T_A } from '../../../../test/chat-safety-mock';
import { stopPortals } from '../../../../test/portal-harness';

vi.setConfig({ testTimeout: 900_000 }); // whole-portal screens on a shared, throttled CPU
configure({ asyncUtilTimeout: 90_000 });

afterEach(() => stopPortals());

/** Placeholder nickname with markup: it must be shown as text, never rendered as HTML. */
const REPORTED = '<b>zqNick</b><img src=x>';

function stateWithNicknameReport() {
  const state = chatSafetyState();
  state.cases[0]!.target_type = 'user';
  state.cases[0]!.evidence = [{ evidence_id: '0193a4d2-d100-7000-8000-000000000001', reported_nickname: REPORTED, created_at: new Date().toISOString() }];
  return state;
}

const actPath = `/admin/moderation/cases/${CS.caseA}:act`;

describe('Nickname trong case kiểm duyệt (F136-AC-22, AC-19)', () => {
  it('shows evidence.reported_nickname as plain text labelled "Nickname bị báo cáo"', async () => {
    await renderSafety(`/live-ops/moderation-cases/${CS.caseA}`, { account: 'sm.antoan', state: stateWithNicknameReport() });
    const box = await screen.findByTestId('reported-nicknames');
    expect(within(box).getByText('Nickname bị báo cáo')).toBeInTheDocument();
    expect(within(box).getByText(REPORTED)).toBeInTheDocument();
    expect(box.querySelector('img, b')).toBeNull();
  });

  it('"Đặt lại nickname": target student + note required; posts action=nickname_reset with If-Match, X-Tenant-Id, Idempotency-Key', async () => {
    const { backend, user } = await renderSafety(`/live-ops/moderation-cases/${CS.caseA}`, { account: 'sm.antoan', state: stateWithNicknameReport() });
    const card = await screen.findByTestId('case-nickname-action');
    const note = within(card).getByLabelText('Ghi chú đặt lại nickname');
    expect(note).toHaveAttribute('maxlength', '1000');
    await user.click(within(card).getByRole('button', { name: 'Đặt lại nickname' }));
    // antd re-creates the error nodes while animating: assert inside waitFor, not on a found node
    await waitFor(() => expect(within(card).getByText('Chọn học sinh cần đặt lại nickname.')).toBeInTheDocument());
    await waitFor(() => expect(within(card).getAllByText(/bắt buộc/i).length).toBeGreaterThan(0));
    expect(backend.calls('POST', actPath)).toHaveLength(0);
    await user.click(within(card).getByRole('combobox', { name: 'Học sinh cần đặt lại nickname' }));
    await user.click((await screen.findAllByTitle(CS.studentOther)).at(-1)!);
    fireEvent.change(note, { target: { value: 'Nickname xúc phạm bạn khác' } });
    await user.click(within(card).getByRole('button', { name: 'Đặt lại nickname' }));
    await waitFor(() => expect(backend.calls('POST', actPath)).toHaveLength(1));
    const call = backend.calls('POST', actPath)[0]!;
    expect(call.body).toEqual({ action: 'nickname_reset', target_user_id: CS.studentOther, note: 'Nickname xúc phạm bạn khác' });
    expect(call.headers['if-match']).toBe('W/"3"');
    expect(call.headers['x-tenant-id']).toBe(T_A);
    expect(call.headers['idempotency-key']).toMatch(/^[0-9a-f-]{36}$/);
    await waitFor(() => expect(screen.getByText('Đã đặt lại nickname. Học sinh sẽ chọn tên mới khi vào game.')).toBeInTheDocument());
  });

  it('hidden without moderation.case.act (action.live_ops.moderation_cases.act), the snapshot still shown', async () => {
    await renderSafety(`/live-ops/moderation-cases/${CS.caseA}`, { account: 'sm.chixem', state: stateWithNicknameReport() });
    expect(await screen.findByTestId('reported-nicknames')).toBeInTheDocument();
    expect(screen.queryByTestId('case-nickname-action')).toBeNull();
    expect(screen.queryByRole('button', { name: 'Đặt lại nickname' })).toBeNull();
  });
});
