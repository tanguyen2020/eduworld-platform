// F080 T041, T042, T043, T062 — Game Analytics of workspace E (TC-ANL-008, TC-DASH-003, TC-DASH-008, TC-CMS-MENU-02).
// Backend = MOCK (test/liveops-mock.ts, test only, AGENTS R17).
// F080-AC-21: 11 panels, each loading on its own, ≤ 4 requests in flight, compare period (second request with the shifted
// range), ONE "cắt theo" dimension; F080-AC-22: grain label, "Cập nhật lúc", "Tạm tính", "Đang tính số liệu…",
// "Chưa có số liệu trước ngày …", RANGE_TOO_LARGE / QUERY_TIMEOUT hints, DASHBOARD_BUSY retried at most twice;
// F080-AC-23: content row → workspace A editor (only with the menu), asynchronous export + notification, duration metrics
// only in Chơi lành mạnh (+ median in Tổng quan); F080-AC-33: export action off → hidden; FEATURE_DISABLED → notice.
import { screen, waitFor, within } from '@testing-library/react';
import { afterEach, describe, expect, it } from 'vitest';
import { LO, liveOpsState, renderLiveOps } from '../../../test/liveops-mock';
import { problem, stopPortals } from '../../../test/portal-harness';
import { tenantFilterStore } from '../../../shell/tenantFilterStore';
import { PANELS } from '../analytics/model';

afterEach(() => {
  stopPortals();
  tenantFilterStore.reset();
});

const URL = '/live-ops/game-analytics';
const metricCalls = (backend: { calls: (m: string, p: string) => { search: string }[] }, code: string) =>
  backend.calls('GET', '/admin/analytics/game/metrics').filter((r) => new URLSearchParams(r.search).getAll('metric_codes').includes(code));

async function pick(user: { click: (e: Element) => Promise<void> }, label: string, option: string) {
  await user.click(screen.getByRole('combobox', { name: label }));
  await user.click((await screen.findAllByTitle(option)).at(-1)!);
}

describe('Game Analytics panels (F080-AC-21)', () => {
  it('F080-AC-21 shows the 11 panels, each loading independently, never more than 4 requests in flight', async () => {
    const state = liveOpsState();
    state.analyticsDelayMs = 120;
    const { backend } = await renderLiveOps(URL, { account: 'lo.vanhanh', state });
    for (const p of PANELS) expect(await screen.findByTestId(`panel-${p}`)).toBeInTheDocument();
    await waitFor(() => expect(within(screen.getByTestId('panel-performance')).getByTestId('performance-table')).toBeInTheDocument());
    await waitFor(() => expect(screen.getByTestId('content-quest')).toHaveTextContent('Quest Rừng Xanh'));
    expect(state.maxInFlight).toBeGreaterThan(1);
    expect(state.maxInFlight).toBeLessThanOrEqual(4);
    // every read goes to the internal gateway without X-Tenant-Id
    const reads = backend.requests.filter((r) => r.path.startsWith('/admin/analytics/game/'));
    expect(reads.length).toBeGreaterThanOrEqual(11);
    expect(reads.every((r) => r.gateway === 'internal' && !r.headers['x-tenant-id'])).toBe(true);
  });

  it('F080-AC-21 compare with the previous period sends a second request with the shifted range', async () => {
    const { backend, user } = await renderLiveOps(URL, { account: 'lo.vanhanh' });
    await waitFor(() => expect(within(screen.getByTestId('panel-overview')).getByTestId('metric-value-dau')).toHaveTextContent('120'));
    const first = new URLSearchParams(metricCalls(backend, 'dau')[0]!.search);
    await user.click(screen.getByRole('switch', { name: 'So sánh kỳ trước' }));
    await waitFor(() => expect(metricCalls(backend, 'dau').length).toBeGreaterThanOrEqual(2));
    const prev = new URLSearchParams(metricCalls(backend, 'dau').at(-1)!.search);
    const days = (a: string, b: string) => (Date.parse(b) - Date.parse(a)) / 86_400_000;
    expect(days(prev.get('from')!, prev.get('to')!)).toBe(days(first.get('from')!, first.get('to')!));
    expect(days(prev.get('to')!, first.get('from')!)).toBe(1);
    expect(await within(screen.getByTestId('panel-overview')).findByTestId('metric-prev-dau')).toHaveTextContent('Kỳ trước: 120');
  });

  it('F080-AC-21 one "cắt theo" dimension at a time: values go in its own parameter only', async () => {
    const { backend, user } = await renderLiveOps(URL, { account: 'lo.vanhanh' });
    await screen.findByTestId('panel-overview');
    await pick(user, 'Cắt theo', 'Nền tảng');
    await pick(user, 'Giá trị', 'Web');
    await waitFor(() => expect(metricCalls(backend, 'dau').some((r) => new URLSearchParams(r.search).getAll('platforms').includes('web'))).toBe(true));
    await pick(user, 'Cắt theo', 'Hạng thiết bị');
    await pick(user, 'Giá trị', 'Cao');
    await waitFor(() => expect(new URLSearchParams(metricCalls(backend, 'dau').at(-1)!.search).getAll('device_tiers')).toEqual(['high']));
    for (const r of backend.requests.filter((x) => x.path.startsWith('/admin/analytics/game/'))) {
      const q = new URLSearchParams(r.search);
      expect(['school_ids', 'grade_bands', 'platforms', 'locales', 'device_tiers'].filter((k) => q.getAll(k).length).length).toBeLessThanOrEqual(1);
    }
    expect(await within(screen.getByTestId('panel-overview')).findByTestId('dimension-table')).toHaveTextContent('high');
  });

  it('F080-AC-21 on a phone only Tổng quan and Chơi lành mạnh, read-only', async () => {
    await renderLiveOps(URL, { account: 'lo.vanhanh', width: 400 });
    expect(await screen.findByTestId('panel-overview')).toBeInTheDocument();
    expect(screen.getByTestId('panel-wellbeing')).toBeInTheDocument();
    expect(screen.queryByTestId('panel-economy')).toBeNull();
    expect(screen.queryByTestId('analytics-filters')).toBeNull();
    expect(screen.queryByTestId('analytics-export')).toBeNull();
  });
});

describe('Dashboard rules (F080-AC-22)', () => {
  it('F080-AC-22 grain label, "Cập nhật lúc", "Tạm tính", "Đang tính số liệu…" and "Chưa có số liệu trước ngày …"', async () => {
    const state = liveOpsState();
    state.analyticsAnswers.push((r) => {
      const codes = new URLSearchParams(r.search).getAll('metric_codes');
      if (!codes.includes('sessions_over_reminder')) return undefined;
      return new Response(
        JSON.stringify({ grain: 'week', data_as_of: '2026-11-26T08:05:00+07:00', is_partial: true, available_from: '2099-01-01', data: [{ metric_code: 'sessions_over_reminder', unit: 'ratio', granularity: 'week', points: [{ period_start: '2026-11-23', value: null, suppressed: false }] }] }),
        { status: 200, headers: { 'Content-Type': 'application/json' } },
      );
    });
    await renderLiveOps(URL, { account: 'lo.vanhanh', state });
    const w = await screen.findByTestId('panel-wellbeing');
    await waitFor(() => expect(within(w).getByTestId('dashboard-meta')).toBeInTheDocument());
    const metaLine = within(w).getByTestId('dashboard-meta');
    expect(metaLine).toHaveTextContent('Cập nhật lúc 08:05');
    expect(metaLine).toHaveTextContent('Theo tuần');
    expect(metaLine).toHaveTextContent('Tạm tính');
    expect(metaLine).toHaveTextContent('Chưa có số liệu trước ngày 01/01/2099');
    expect(within(w).getByTestId('metric-value-sessions_over_reminder')).toHaveTextContent('Đang tính số liệu…');
    expect(within(w).getByTestId('metric-value-sessions_over_reminder')).not.toHaveTextContent('0');
  });

  it('F080-AC-22 DASHBOARD_RANGE_TOO_LARGE and DASHBOARD_QUERY_TIMEOUT show their hints; QUERY_TIMEOUT is not repeated', async () => {
    const state = liveOpsState();
    state.analyticsAnswers.push((r) => (r.path.endsWith('/retention') ? problem(422, 'DASHBOARD_RANGE_TOO_LARGE') : undefined));
    state.analyticsAnswers.push((r) => (r.path.endsWith('/performance') ? problem(504, 'DASHBOARD_QUERY_TIMEOUT') : undefined));
    const { backend } = await renderLiveOps(URL, { account: 'lo.vanhanh', state });
    expect(await within(await screen.findByTestId('panel-retention')).findByText(/Khoảng thời gian quá dài/)).toBeInTheDocument();
    expect(await within(screen.getByTestId('panel-performance')).findByText(/Truy vấn mất quá nhiều thời gian/)).toBeInTheDocument();
    expect(backend.calls('GET', '/admin/analytics/game/performance')).toHaveLength(1);
    // the other panels still load
    await waitFor(() => expect(within(screen.getByTestId('panel-overview')).getByTestId('metric-value-dau')).toHaveTextContent('120'));
  });

  it('F080-AC-22 DASHBOARD_BUSY is retried after 2–5 s at most twice, then the busy notice', async () => {
    const state = liveOpsState();
    state.analyticsAnswers.push((r) => (new URLSearchParams(r.search).getAll('metric_codes').includes('dau') ? problem(429, 'DASHBOARD_BUSY') : undefined));
    const { backend } = await renderLiveOps(URL, { account: 'lo.vanhanh', state });
    const w = await screen.findByTestId('panel-overview');
    expect(await within(w).findByText(/Hệ thống đang bận/, undefined, { timeout: 120_000 })).toBeInTheDocument();
    expect(metricCalls(backend, 'dau')).toHaveLength(3);
  });
});

describe('Content, export and durations (F080-AC-23)', () => {
  it('F080-AC-23 a quest row opens its current version in workspace A when the user sees that menu', async () => {
    const { router, user } = await renderLiveOps(URL, { account: 'lo.vanhanh' });
    const table = await screen.findByTestId('content-quest');
    await user.click(await within(table).findByRole('button', { name: 'Mở Quest Rừng Xanh trong Game Operations Console' }));
    await waitFor(() => expect(router.state.location.pathname).toBe(`/game-ops/quests/quests/${LO.quest1}/v/3`));
  });

  it('F080-AC-23 without the workspace A menu the title is plain text; no duration column in the content table', async () => {
    await renderLiveOps(URL, { account: 'lo.xem' });
    const table = await screen.findByTestId('content-quest');
    await within(table).findByText('Quest Rừng Xanh');
    expect(within(table).queryByRole('button', { name: /Quest Rừng Xanh/ })).toBeNull();
    expect(table).not.toHaveTextContent(/thời lượng|thời gian|phút/i);
  });

  it('F080-AC-23 duration metrics only in Chơi lành mạnh and the median in Tổng quan; never a total play time', async () => {
    await renderLiveOps(URL, { account: 'lo.vanhanh' });
    const overview = await screen.findByTestId('panel-overview');
    await waitFor(() => expect(within(overview).getByTestId('metric-value-session_duration_p50')).toHaveTextContent('11 phút'));
    const wellbeing = screen.getByTestId('panel-wellbeing');
    await waitFor(() => expect(within(wellbeing).getByTestId('metric-daily_playtime_p90')).toBeInTheDocument());
    for (const p of PANELS.filter((x) => x !== 'overview' && x !== 'wellbeing')) {
      const panel = screen.getByTestId(`panel-${p}`);
      expect(within(panel).queryByTestId('metric-session_duration_p50')).toBeNull();
      expect(within(panel).queryByTestId('metric-daily_playtime_p90')).toBeNull();
    }
    expect(document.body.textContent).not.toMatch(/tổng thời gian chơi/i);
  });

  it('F080-AC-23 Xuất CSV creates an asynchronous job (Idempotency-Key), polls it and notifies when ready', async () => {
    const state = liveOpsState();
    state.exportHold = true;
    const { backend, user } = await renderLiveOps(URL, { account: 'lo.vanhanh', state });
    const exp = await screen.findByTestId('analytics-export');
    await user.click(within(exp).getByRole('button', { name: 'Xuất CSV' }));
    await waitFor(() => expect(backend.calls('POST', '/admin/analytics/game/exports')).toHaveLength(1));
    const post = backend.calls('POST', '/admin/analytics/game/exports')[0]!;
    expect(post.headers['idempotency-key']).toBeTruthy();
    expect(post.body).toMatchObject({ report: 'metrics', format: 'csv', params: { metric_codes: ['dau', 'wau', 'mau', 'stickiness', 'session_duration_p50', 'active_days_per_week'] } });
    expect(await within(exp).findByText('Đang chuẩn bị báo cáo…')).toBeInTheDocument();
    expect(within(exp).queryByTestId('export-link')).toBeNull();
    state.exportHold = false;
    expect(await within(exp).findByTestId('export-link', undefined, { timeout: 60_000 })).toHaveAttribute('href', 'https://exports.example.test/analytics.csv');
    expect(await screen.findByText('Báo cáo analytics đã sẵn sàng để tải.')).toBeInTheDocument();
    expect(backend.calls('GET', `/admin/analytics/game/exports/${LO.exportJob}`).length).toBeGreaterThanOrEqual(2);
  });
});

describe('Menu/action toggles (F080-AC-33)', () => {
  it('F080-AC-33 export action turned off → no export button; the panels still show', async () => {
    await renderLiveOps(URL, { account: 'lo.vanhanh', disabledActions: ['action.live_ops.game_analytics.export'] });
    expect(await screen.findByTestId('panel-overview')).toBeInTheDocument();
    expect(screen.queryByTestId('analytics-export')).toBeNull();
  });

  it('F080-AC-33 403 FEATURE_DISABLED on export → "tạm tắt" message and getMyPermissions reloaded', async () => {
    const { backend, user } = await renderLiveOps(URL, { account: 'lo.vanhanh' });
    const exp = await screen.findByTestId('analytics-export');
    backend.overrides.unshift((r) => (r.method === 'POST' && r.path === '/admin/analytics/game/exports' ? problem(403, 'FEATURE_DISABLED', { feature_key: 'action.live_ops.game_analytics.export' }) : undefined));
    const before = backend.calls('GET', '/me/permissions').length;
    await user.click(within(exp).getByRole('button', { name: 'Xuất CSV' }));
    expect(await within(exp).findByText('Tính năng này đang tạm tắt.')).toBeInTheDocument();
    await waitFor(() => expect(backend.calls('GET', '/me/permissions').length).toBeGreaterThan(before));
  });

  it('F080-AC-23 EXPORT_LIMIT_REACHED → its message', async () => {
    const { backend, user } = await renderLiveOps(URL, { account: 'lo.vanhanh' });
    const exp = await screen.findByTestId('analytics-export');
    backend.overrides.unshift((r) => (r.method === 'POST' && r.path === '/admin/analytics/game/exports' ? problem(429, 'EXPORT_LIMIT_REACHED') : undefined));
    await user.click(within(exp).getByRole('button', { name: 'Xuất CSV' }));
    expect(await within(exp).findByText('Đã đạt giới hạn số báo cáo đang xuất. Vui lòng thử lại sau.', undefined, { timeout: 20_000 })).toBeInTheDocument();
  });
});
