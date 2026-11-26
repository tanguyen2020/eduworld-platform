// F080 T058, T059, T062 — "Hoạt động game" of workspace C (TC-DASH-002, TC-DASH-003, TC-ANL-005, TC-CMS-MENU-02).
// Backend = MOCK (test/liveops-mock.ts + the F052 MOCK, test only, AGENTS R17).
// F080-AC-27: getTenantGameSummary with from/to, school_ids (only for a `unit` tenant, schools of the scope — every page),
// grade_bands; one widget per group; F080-AC-28: the GAME_ANALYTICS §7.2 groups, duration only as a median, no economy,
// no student list, no total play time, phone → only Tổng quan + Chơi lành mạnh read-only; F080-AC-29: "< 5" + the
// suppressed note, never summed; F080-AC-30: grain, "Cập nhật lúc", "Tạm tính", RANGE_TOO_LARGE / BUSY (retried ≤ 2) /
// QUERY_TIMEOUT; edge cases: available_from, device telemetry off → "Không có dữ liệu"; F080-AC-33: action off → no tab.
import { screen, waitFor, within } from '@testing-library/react';
import { afterEach, describe, expect, it } from 'vitest';
import { LO, liveOpsState, renderLiveOps, tenantGameSummary } from '../../../../test/liveops-mock';
import { problem, stopPortals, TENANTS } from '../../../../test/portal-harness';

afterEach(() => stopPortals());

const URL = '/school-lms/reports/game-activity';
const summaryCalls = (backend: { calls: (m: string, p: string) => { search: string; headers: Record<string, string> }[] }) => backend.calls('GET', '/tenant/analytics/game/summary');
const WIDGETS = ['overview', 'progression', 'social', 'wellbeing', 'device'];

async function pick(user: { click: (e: Element) => Promise<void> }, label: string, option: string) {
  await user.click(screen.getByRole('combobox', { name: label }));
  await user.click((await screen.findAllByTitle(option)).at(-1)!);
}

describe('Hoạt động game (F080-AC-27, AC-28)', () => {
  it('F080-AC-27 unit tenant: getTenantGameSummary with range, schools of every page and grade bands; one widget per group', async () => {
    const { backend, user } = await renderLiveOps(URL, { account: 'ad.donvi' });
    for (const w of WIDGETS) expect(await screen.findByTestId(`game-widget-${w}`)).toBeInTheDocument();
    const first = new URLSearchParams(summaryCalls(backend)[0]!.search);
    expect(first.get('from')).toMatch(/^\d{4}-\d{2}-\d{2}$/);
    expect(first.get('to')).toMatch(/^\d{4}-\d{2}-\d{2}$/);
    expect(summaryCalls(backend)[0]!.headers['x-tenant-id']).toBe(TENANTS.pgdDemo.tenant_id);
    // the school filter walks every page of listTenantSchools
    await pick(user, 'Trường', 'TH Đơn vị 2');
    await pick(user, 'Nhóm khối', 'Khối 4–5');
    await waitFor(() => {
      const q = new URLSearchParams(summaryCalls(backend).at(-1)!.search);
      expect([q.getAll('school_ids'), q.getAll('grade_bands')]).toEqual([[LO.schoolU2], ['g4_5']]);
    });
    expect(backend.calls('GET', '/tenant/schools').some((r) => new URLSearchParams(r.search).get('cursor') === 'p2')).toBe(true);
    // one shared query for the five widgets
    expect(summaryCalls(backend).filter((r) => !new URLSearchParams(r.search).getAll('school_ids').length && !new URLSearchParams(r.search).getAll('grade_bands').length)).toHaveLength(1);
  });

  it('F080-AC-27 school tenant: no school filter (only for a unit tenant)', async () => {
    const { backend } = await renderLiveOps(URL, { account: 'ad.truong.a' });
    expect(await screen.findByTestId('game-widget-overview')).toBeInTheDocument();
    expect(screen.queryByRole('combobox', { name: 'Trường' })).toBeNull();
    expect(screen.getByRole('combobox', { name: 'Nhóm khối' })).toBeInTheDocument();
    expect(new URLSearchParams(summaryCalls(backend)[0]!.search).getAll('school_ids')).toEqual([]);
  });

  it('F080-AC-28 shows the §7.2 groups; duration only as the median; no economy number, student list or total play time', async () => {
    await renderLiveOps(URL, { account: 'ad.donvi' });
    const overview = await screen.findByTestId('game-widget-overview');
    await waitFor(() => expect(within(overview).getByTestId('gm-active-students')).toHaveTextContent('240'));
    expect(within(overview).getByTestId('gm-session-median')).toHaveTextContent('25 phút');
    expect(within(overview).getByText('Thời lượng phiên (trung vị)')).toBeInTheDocument();
    expect(within(screen.getByTestId('game-widget-wellbeing')).getByTestId('gm-over-reminder')).toHaveTextContent('12%');
    expect(within(screen.getByTestId('game-widget-social')).getByTestId('gm-social-play')).toHaveTextContent('27%');
    expect(within(screen.getByTestId('game-widget-progression')).getByTestId('gm-quest-completion')).toHaveTextContent('61%');
    const page = screen.getByTestId('game-activity-report');
    expect(page.textContent).not.toMatch(/Xu|coin|tổng thời gian chơi|Học sinh:|họ tên/i);
    expect(within(page).queryByRole('link', { name: /học sinh/i })).toBeNull();
    // duration metrics live only in Tổng quan
    for (const w of ['progression', 'social', 'wellbeing', 'device']) expect(screen.getByTestId(`game-widget-${w}`).textContent).not.toMatch(/phút|thời lượng/i);
  });

  it('F080-AC-28 on a phone only the Tổng quan and Chơi lành mạnh cards, read-only', async () => {
    await renderLiveOps(URL, { account: 'ad.donvi', width: 400 });
    expect(await screen.findByTestId('game-widget-overview')).toBeInTheDocument();
    expect(screen.getByTestId('game-widget-wellbeing')).toBeInTheDocument();
    for (const w of ['progression', 'social', 'device']) expect(screen.queryByTestId(`game-widget-${w}`)).toBeNull();
    expect(screen.queryByTestId('game-report-filters')).toBeNull();
  });
});

describe('Suppressed cells and dashboard rules (F080-AC-29, AC-30)', () => {
  it('F080-AC-29 shows "< 5" with the suppressed note and never sums the cells', async () => {
    await renderLiveOps(URL, { account: 'ad.donvi' });
    const note = await screen.findByTestId('game-report-suppressed');
    expect(note).toHaveTextContent('2 ô có dưới 5 học sinh đã được ẩn');
    const wellbeing = screen.getByTestId('game-widget-wellbeing');
    expect(within(within(wellbeing).getByTestId('gm-late-night')).getByTestId('suppressed-cell')).toHaveTextContent('< 5');
    const bySchool = within(screen.getByTestId('game-widget-progression')).getByTestId('gm-by-school');
    expect(within(bySchool).getAllByTestId('suppressed-cell').length).toBe(2);
    // the visible school (200) is not turned into a total, the hidden one is not derived from active_students (240)
    expect(bySchool.textContent).not.toMatch(/\b40\b/);
    expect(within(screen.getByTestId('game-widget-device')).getByTestId('gm-platforms')).toHaveTextContent('< 5');
  });

  it('F080-AC-30 grain label, "Cập nhật lúc", "Tạm tính", "Đang tính số liệu…" and "Chưa có số liệu trước ngày …"', async () => {
    const state = liveOpsState();
    state.summary = tenantGameSummary({ grain: 'week', is_partial: true, from: '2026-11-01', available_from: '2026-11-15', dau_avg: undefined });
    await renderLiveOps(URL, { account: 'ad.donvi', state });
    const overview = await screen.findByTestId('game-widget-overview');
    const metaLine = await within(overview).findByTestId('dashboard-meta');
    expect(metaLine).toHaveTextContent('Cập nhật lúc 09:41');
    expect(metaLine).toHaveTextContent('Theo tuần');
    expect(metaLine).toHaveTextContent('Tạm tính');
    expect(metaLine).toHaveTextContent('Chưa có số liệu trước ngày 15/11/2026');
    expect(within(overview).getByTestId('gm-dau-avg')).toHaveTextContent('Đang tính số liệu…');
  });

  it('F080-AC-30 DASHBOARD_RANGE_TOO_LARGE → narrow the range', async () => {
    const state = liveOpsState();
    state.summaryAnswers.push(problem(422, 'DASHBOARD_RANGE_TOO_LARGE'));
    const r = await renderLiveOps(URL, { account: 'ad.donvi', state });
    expect((await within(await screen.findByTestId('game-widget-overview')).findAllByText(/Khoảng thời gian quá dài/)).length).toBeGreaterThan(0);
    expect(summaryCalls(r.backend)).toHaveLength(1);
  });

  it('F080-AC-30 DASHBOARD_QUERY_TIMEOUT → narrow the scope (not repeated)', async () => {
    const state2 = liveOpsState();
    state2.summaryAnswers.push(problem(504, 'DASHBOARD_QUERY_TIMEOUT'), problem(504, 'DASHBOARD_QUERY_TIMEOUT'));
    const r2 = await renderLiveOps(URL, { account: 'ad.donvi', state: state2 });
    expect(await within(await screen.findByTestId('game-widget-wellbeing')).findByText(/Truy vấn mất quá nhiều thời gian/)).toBeInTheDocument();
    expect(summaryCalls(r2.backend)).toHaveLength(1);
  });

  it('F080-AC-30 DASHBOARD_BUSY is retried after 2–5 s at most twice', async () => {
    const state = liveOpsState();
    state.summaryAnswers.push(problem(429, 'DASHBOARD_BUSY'), problem(429, 'DASHBOARD_BUSY'), problem(429, 'DASHBOARD_BUSY'));
    const { backend } = await renderLiveOps(URL, { account: 'ad.donvi', state });
    expect(await within(await screen.findByTestId('game-widget-overview')).findByText(/Hệ thống đang bận/, undefined, { timeout: 120_000 })).toBeInTheDocument();
    expect(summaryCalls(backend)).toHaveLength(3);
  });

  it('F080-AC-28 edge: product telemetry off → device widget "Không có dữ liệu", no error', async () => {
    const state = liveOpsState();
    state.summary = tenantGameSummary({ platform_distribution: [], device_tier_distribution: [] });
    await renderLiveOps(URL, { account: 'ad.donvi', state });
    const device = await screen.findByTestId('game-widget-device');
    expect(await within(device).findByText('Không có dữ liệu')).toBeInTheDocument();
    expect(within(device).queryByRole('alert')).toBeNull();
  });
});

describe('Menu/action toggles (F080-AC-33)', () => {
  it('F080-AC-33 the "Hoạt động game" tab is in Báo cáo & analytics', async () => {
    const { router, user } = await renderLiveOps('/school-lms/reports', { account: 'ad.donvi' });
    await user.click(await screen.findByRole('tab', { name: 'Hoạt động game' }));
    await waitFor(() => expect(router.state.location.pathname).toBe(URL));
    expect(await screen.findByTestId('game-widget-overview')).toBeInTheDocument();
  });

  it('F080-AC-33 action turned off → no "Hoạt động game" tab, the URL shows "Tính năng đang tắt"', async () => {
    const r2 = await renderLiveOps(URL, { account: 'ad.donvi', disabledActions: ['action.school_lms.reports.view'] });
    expect(await screen.findByTestId('feature-disabled')).toBeInTheDocument();
    expect(screen.queryByRole('tab', { name: 'Hoạt động game' })).toBeNull();
    expect(summaryCalls(r2.backend)).toHaveLength(0);
  });

  it('F080-AC-33 403 FEATURE_DISABLED from getTenantGameSummary → "tạm tắt" message and permissions reloaded', async () => {
    const state = liveOpsState();
    state.summaryAnswers.push(problem(403, 'FEATURE_DISABLED', { feature_key: 'action.school_lms.reports.view' }));
    const { backend } = await renderLiveOps(URL, { account: 'ad.donvi', state });
    expect((await within(await screen.findByTestId('game-widget-overview')).findAllByText(/tạm tắt/i)).length).toBeGreaterThan(0);
    await waitFor(() => expect(backend.calls('GET', '/me/permissions').length).toBeGreaterThan(1));
  });
});
