// F079 T023–T025 — Level & XP Configuration. Backend = MOCK (test/game-ops-mock.ts, test only, AGENTS R17).
// AC-09: curve form (formula / overrides / titles / rewards) from the schema; threshold table + chart and XP sources with
//        caps shown from server data.
// AC-10: "Mô phỏng" calls simulateGameConfig (minutes/day, days/week, weeks) and shows the level per week it returns —
//        the numbers on screen are the server's (the MOCK returns values no client formula would give).
// AC-11: server field errors (non-monotonic thresholds, cap ≤ 0) shown and blocking; the publish confirmation states
//        "không giảm level, không thu hồi XP đã cấp".
import { configure, fireEvent, screen, waitFor, within } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { GAME_ACCOUNTS, renderGameOps } from '../../../test/game-ops-mock';
import { problem, stopPortals } from '../../../test/portal-harness';

vi.setConfig({ testTimeout: 600_000, hookTimeout: 120_000 });
configure({ asyncUtilTimeout: 60_000 });
afterEach(() => stopPortals());

const DESIGNER = GAME_ACCOUNTS['gd.thietke']!.sub;
const CURVE = { schema_version: 1, code: 'default', max_level: 60, formula: { kind: 'power_linear', base: 100, exponent: 1.35, linear: 20 }, titles: [{ code: 'beginner', from_level: 1, name: { vi: 'Tân binh', en: 'Beginner' } }] };

describe('Level & XP (F079-AC-09, AC-10)', () => {
  it('formula fields from the schema; thresholds, chart and weekly levels come from simulateGameConfig; XP sources with caps', async () => {
    let id = '';
    const r = await renderGameOps('/', {
      setup: (m) => {
        id = m.add('progression-curves', [{ status: 'draft', author_id: DESIGNER, payload: CURVE }]).config_id;
        m.add('reward-rules', [{ status: 'published', payload: { code: 'xp.quiz', source_code: 'quiz_completed', caps: { xp_per_day: 200 } } }]);
      },
    });
    await r.router.navigate(`/game-ops/progression/progression-curves/${id}/v/1`);
    expect(await screen.findByRole('spinbutton', { name: 'exponent' })).toHaveValue('1.35');
    expect(screen.getByRole('spinbutton', { name: 'Level tối đa' })).toHaveValue('60');
    // first simulation with the default inputs, done by the server
    await waitFor(() => expect(r.backend.calls('GET', `/game-admin/progression-curves/${id}/versions/1:simulate`)).toHaveLength(1));
    const q = new URLSearchParams(r.backend.calls('GET', `/game-admin/progression-curves/${id}/versions/1:simulate`)[0]!.search);
    expect([q.get('minutes_per_day'), q.get('days_per_week'), q.get('weeks')]).toEqual(['25', '5', '8']);
    const thresholds = await screen.findByTestId('threshold-table');
    expect(within(thresholds).getByText('433')).toBeInTheDocument();
    expect(screen.getByTestId('threshold-chart')).toHaveAttribute('aria-label', expect.stringContaining('3: 433'));
    // XP sources and their caps (published reward rules)
    const sources = screen.getByTestId('xp-sources');
    expect(await within(sources).findByText('xp.quiz')).toBeInTheDocument();
    expect(within(sources).getByText('200')).toBeInTheDocument();
    // "Mô phỏng" with other inputs
    fireEvent.change(screen.getByLabelText('Phút chơi/ngày'), { target: { value: '40' } });
    fireEvent.change(screen.getByLabelText('Số tuần'), { target: { value: '4' } });
    await r.user.click(screen.getByRole('button', { name: 'Mô phỏng' }));
    await waitFor(() => expect(r.backend.calls('GET', `/game-admin/progression-curves/${id}/versions/1:simulate`)).toHaveLength(2));
    const q2 = new URLSearchParams(r.backend.calls('GET', `/game-admin/progression-curves/${id}/versions/1:simulate`)[1]!.search);
    expect([q2.get('minutes_per_day'), q2.get('weeks')]).toEqual(['40', '4']);
    const weeks = await screen.findByTestId('simulation-weeks');
    await waitFor(() => expect(within(weeks).getAllByRole('row')).toHaveLength(5));
    // MOCK level per week = 2 + i: the screen shows what the server sent
    expect(within(weeks).getAllByRole('row')[4]).toHaveTextContent('5');
  });
});

describe('Lỗi và publish đường cong (F079-AC-11)', () => {
  it('server errors on thresholds / caps are shown and block "Gửi duyệt"', async () => {
    let id = '';
    const r = await renderGameOps('/', { setup: (m) => (id = m.add('progression-curves', [{ status: 'draft', author_id: DESIGNER, payload: CURVE }]).config_id) });
    await r.router.navigate(`/game-ops/progression/progression-curves/${id}/v/1`);
    r.backend.overrides.unshift((q) =>
      q.method === 'PATCH' ? problem(422, 'VALIDATION_FAILED', { errors: [{ field: 'level_overrides/1/xp_to_next', code: 'NOT_MONOTONIC' }] }) : undefined,
    );
    fireEvent.change(await screen.findByRole('spinbutton', { name: 'exponent' }), { target: { value: '1.2' } });
    await r.user.click(screen.getByRole('button', { name: 'Lưu' }));
    expect(await screen.findByText(/level_overrides\/1\/xp_to_next/)).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Gửi duyệt' })).toBeDisabled();
  });

  it('the publish confirmation of a curve says no level is lowered and no XP is taken back', async () => {
    let id = '';
    const r = await renderGameOps('/', { account: 'sa.admin', setup: (m) => (id = m.add('progression-curves', [{ status: 'approved', author_id: DESIGNER, payload: CURVE }]).config_id) });
    await r.router.navigate(`/game-ops/progression/progression-curves/${id}/v/1`);
    await r.user.click(await screen.findByRole('button', { name: 'Xuất bản ngay' }));
    const dialog = await screen.findByRole('dialog', { name: 'Xuất bản v1' });
    expect(within(dialog).getByTestId('publish-curve-notice')).toHaveTextContent('không giảm level của ai, không thu hồi XP đã cấp');
  });
});
