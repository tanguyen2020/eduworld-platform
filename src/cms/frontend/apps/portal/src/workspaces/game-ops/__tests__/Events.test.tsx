// F079 T027–T030 — Event Builder, calendar, season track, template. Backend = MOCK (test/game-ops-mock.ts, test only, R17).
// AC-12: 8-step wizard; vi/en side by side with translation status; content references from published content only;
//        the draft is created before the preview, then submitted for review.
// AC-13: server validation errors (TRANSLATION_INCOMPLETE, festival too short…) mark and block their step; the
//        "> 3 phiên/tuần" reminder never blocks.
// AC-14: calendar of the ICT window from listGameConfigs `events` + from/to, bars colored by kind, server warnings.
// AC-15: season track: tiers × rewards, simulation by the server (no minutes), no paid-track control; template:
//        allowed_params.
import { configure, fireEvent, screen, waitFor, within } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { ictDate } from '../../../shared/release/ict';
import { GAME_ACCOUNTS, renderGameOps } from '../../../test/game-ops-mock';
import { problem, stopPortals } from '../../../test/portal-harness';

vi.setConfig({ testTimeout: 600_000, hookTimeout: 120_000 });
configure({ asyncUtilTimeout: 60_000 });
afterEach(() => stopPortals());

const LIVEOPS = GAME_ACCOUNTS['lo.vanhanh']!.sub;
const STEP_TITLES = ['Thông tin (vi/en)', 'Đối tượng', 'Nội dung', 'Phần thưởng', 'Lịch (ICT)', 'Xem trước', 'Gửi duyệt', 'Lên lịch'];

describe('Event Builder (F079-AC-12)', () => {
  it('8 steps; bilingual name with status; draft created before the preview; submit for review', async () => {
    const { backend, user } = await renderGameOps('/game-ops/events/events/new', {
      account: 'lo.vanhanh',
      setup: (m) => m.add('quests', [{ status: 'published', title: 'Quest Toán', payload: { code: 'q_math' } }]),
    });
    const wizard = await screen.findByTestId('event-wizard');
    for (const title of STEP_TITLES) expect(within(wizard).getAllByText(title).length).toBeGreaterThan(0);
    fireEvent.change(screen.getByLabelText('Tên (VI)'), { target: { value: 'Tuần lễ Toán học' } });
    const name = screen.getByTestId('field-name');
    expect(within(name).getByText(/Thiếu|Một phần|partial/i)).toBeInTheDocument();
    fireEvent.change(screen.getByLabelText('Tên (EN)'), { target: { value: 'Math week' } });
    await user.click(screen.getByRole('button', { name: 'Tiếp' }));
    await user.click(screen.getByRole('button', { name: 'Tiếp' }));
    // step 3: quests from the published list only
    expect(await screen.findByText('Chỉ chọn được nội dung đã xuất bản.')).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'Tiếp' }));
    expect(screen.getByTestId('event-healthy-hint')).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'Tiếp' }));
    await user.click(screen.getByRole('button', { name: 'Tiếp' }));
    // leaving the schedule step creates the draft (Idempotency-Key)
    await waitFor(() => expect(backend.calls('POST', '/game-admin/events')).toHaveLength(1));
    const create = backend.calls('POST', '/game-admin/events')[0]!;
    expect(create.headers['idempotency-key']).toBeTruthy();
    expect((create.body as { payload: { name: unknown } }).payload.name).toEqual({ vi: 'Tuần lễ Toán học', en: 'Math week' });
    expect(await screen.findByTestId('preview-panel')).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'Tiếp' }));
    await user.click(within(await screen.findByTestId('event-submit-step')).getByRole('button', { name: 'Gửi duyệt' }));
    await waitFor(() => expect(backend.requests.filter((r) => r.path.endsWith('/versions/1:transition'))).toHaveLength(1));
    expect(await screen.findByTestId('event-schedule-step')).toBeInTheDocument();
  });

  it('AC-13: server errors mark and block their steps and the wizard goes back to the first one', async () => {
    const { user } = await renderGameOps('/game-ops/events/events/new', {
      account: 'lo.vanhanh',
      setup: (_m, b) =>
        b.overrides.unshift((r) =>
          r.method === 'POST' && r.path === '/game-admin/events'
            ? problem(422, 'VALIDATION_FAILED', { errors: [{ field: 'name.en', code: 'TRANSLATION_INCOMPLETE' }, { field: 'schedule/end_at', code: 'EVENT_FESTIVAL_TOO_SHORT' }] })
            : undefined,
        ),
    });
    await screen.findByTestId('event-wizard');
    for (let i = 0; i < 5; i++) await user.click(screen.getByRole('button', { name: 'Tiếp' }));
    expect(await screen.findByRole('alert')).toHaveTextContent('Server báo lỗi ở bước: Thông tin (vi/en), Lịch (ICT)');
    expect(screen.getByTestId('event-step-0')).toBeInTheDocument();
    expect(screen.queryByTestId('preview-panel')).toBeNull();
  });
});

describe('Lịch sự kiện (F079-AC-14)', () => {
  it('asks the server for the ICT window, draws the events by kind and shows the server overlap warnings', async () => {
    const now = new Date();
    const start = now.toISOString();
    const end = new Date(now.getTime() + 2 * 86_400_000).toISOString();
    let a = '';
    const { backend } = await renderGameOps('/game-ops/events/calendar', {
      account: 'lo.vanhanh',
      setup: (m) => {
        const other = m.add('events', [{ status: 'published', title: 'Lễ hội đèn lồng', payload: { event_kind: 'world_festival', schedule: { start_at: start, end_at: end } } }]);
        a = m.add('events', [
          {
            status: 'published',
            title: 'Tết Trung thu',
            author_id: LIVEOPS,
            payload: { event_kind: 'weekend', schedule: { start_at: start, end_at: end } },
            calendar_warnings: [{ code: 'decoration_area_overlap', other_config_id: other.config_id, other_title: 'Lễ hội đèn lồng', area_id: 'main_hub' }],
          },
        ]).config_id;
      },
    });
    expect((await screen.findAllByTestId(`calendar-event-${a}`)).length).toBeGreaterThan(0);
    const q = new URLSearchParams(backend.calls('GET', '/game-admin/events')[0]!.search);
    const today = ictDate(now);
    expect(q.get('from')! <= today && today <= q.get('to')!).toBe(true);
    expect(screen.getAllByTestId(`calendar-event-${a}`)[0]).toHaveAttribute('data-kind', 'weekend');
    expect(screen.getByTestId('calendar-warnings')).toHaveTextContent('Tết Trung thu và Lễ hội đèn lồng cùng trang trí vùng main_hub');
  });
});

describe('Season track và template (F079-AC-15)', () => {
  it('season track: tiers × rewards, server simulation without minutes, no paid-track control', async () => {
    let id = '';
    const r = await renderGameOps('/', {
      account: 'lo.vanhanh',
      setup: (m) =>
        (id = m.add('season-tracks', [
          {
            status: 'draft',
            author_id: LIVEOPS,
            payload: { schema_version: 1, paid_track: false, points: { per_tier: 100, daily_cap: 100, sources: [{ source_code: 'quest', points: 20 }] }, tiers: [{ tier: 1, rewards: [{ type: 'currency', currency_code: 'xu', amount: 50 }] }] },
          },
        ]).config_id),
    });
    await r.router.navigate(`/game-ops/events/season-tracks/${id}/v/1`);
    const tiers = await screen.findByTestId('season-tiers');
    expect(within(tiers).getByText('currency · xu · 50')).toBeInTheDocument();
    expect(screen.queryByText('Track trả phí')).toBeNull();
    expect(screen.queryByRole('switch', { name: /trả phí|paid/i })).toBeNull();
    await waitFor(() => expect(r.backend.calls('GET', `/game-admin/season-tracks/${id}/versions/1:simulate`)).toHaveLength(1));
    const q = new URLSearchParams(r.backend.calls('GET', `/game-admin/season-tracks/${id}/versions/1:simulate`)[0]!.search);
    expect(q.get('minutes_per_day')).toBeNull();
    expect(q.get('days_per_week')).toBe('5');
    expect(screen.queryByLabelText('Phút chơi/ngày')).toBeNull();
  });

  it('event template: allowed_params is declared with its explanation', async () => {
    await renderGameOps('/game-ops/events/event-templates/new', { account: 'lo.vanhanh' });
    expect(await screen.findByTestId('event-template-editor')).toHaveTextContent('allowed_params');
    expect(screen.getByRole('combobox', { name: 'Tham số nhà trường được điền' })).toBeInTheDocument();
  });
});
