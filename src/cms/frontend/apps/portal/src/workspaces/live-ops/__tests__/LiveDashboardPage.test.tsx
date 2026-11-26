// F080 T038, T045, T046, T048 — Live dashboard of workspace E. Backend = MOCK (test/liveops-mock.ts, test only, AGENTS R17).
// F080-AC-20: auto refresh 15–30 s, "Cập nhật lúc HH:mm:ss" from `as_of`, status by token colour + icon + label, link to
// Kiểm duyệt only when "báo cáo chờ" is over its threshold AND the user sees that menu, 503 → error without numbers;
// F080-AC-24: users without game.liveops.read / game.analytics.read (Safety Moderator) get the 403 page; tenant roles never
// load the workspace E chunk; F080-AC-25: API 403 → 403 page with trace_id; internal gateway down → VPN notice;
// F080-AC-33: menu turned off → hidden menu + "Tính năng đang tắt" page.
import { screen, waitFor, within } from '@testing-library/react';
import { afterEach, describe, expect, it } from 'vitest';
import { LIVE_REFRESH_MS } from '../api/liveops';
import { liveOpsState, renderLiveOps } from '../../../test/liveops-mock';
import { problem, stopPortals } from '../../../test/portal-harness';
import { tenantFilterStore } from '../../../shell/tenantFilterStore';

afterEach(() => {
  stopPortals();
  tenantFilterStore.reset();
});

const URL = '/live-ops/live-dashboard';

describe('Live dashboard (F080-AC-20)', () => {
  it('F080-AC-20 shows the KPI cards with "Cập nhật lúc HH:mm:ss", status label + icon, refreshes every 15–30 s', async () => {
    const { runtime, backend } = await renderLiveOps(URL, { account: 'lo.vanhanh' });
    expect(await screen.findByTestId('kpi-value-ccu')).toHaveTextContent('1.234');
    expect(screen.getByTestId('live-as-of')).toHaveTextContent(/^Cập nhật lúc \d{2}:\d{2}:\d{2}$/);
    expect(screen.getByText('Người chơi đồng thời (CCU)')).toBeInTheDocument();
    // status is never colour alone: label + icon + data-status
    const incident = within(screen.getByTestId('kpi-http_5xx_rate'));
    expect(incident.getByText('Sự cố').closest('[data-status]')).toHaveAttribute('data-status', 'incident');
    expect(incident.getByRole('img', { hidden: true })).toBeInTheDocument();
    expect(screen.getByTestId('kpi-http_5xx_rate')).toHaveAttribute('data-accent', 'error');
    // unknown code rendered generically, null value is not a zero
    expect(screen.getByTestId('kpi-new_kpi_code')).toHaveTextContent('new_kpi_code');
    expect(screen.getByTestId('kpi-value-new_kpi_code')).toHaveTextContent('Không có dữ liệu');
    // refresh cadence of the overview query
    expect(LIVE_REFRESH_MS).toBeGreaterThanOrEqual(15_000);
    expect(LIVE_REFRESH_MS).toBeLessThanOrEqual(30_000);
    const q = runtime.queryClient.getQueryCache().findAll({ queryKey: ['live-ops', 'overview'] })[0]!;
    expect((q.options as { refetchInterval?: number }).refetchInterval).toBe(LIVE_REFRESH_MS);
    expect(backend.calls('GET', '/admin/liveops/overview')[0]!.gateway).toBe('internal');
  });

  it('F080-AC-20 links to Kiểm duyệt when reports_pending is over its threshold and the user sees the moderation menu', async () => {
    await renderLiveOps(URL, { account: 'sa.admin' });
    const card = await screen.findByTestId('kpi-reports_pending');
    expect(within(card).getByTestId('kpi-moderation-link')).toHaveAttribute('href', '/live-ops/moderation-cases');
  });

  it('F080-AC-20 no moderation link without the Kiểm duyệt menu (Live Ops role)', async () => {
    await renderLiveOps(URL, { account: 'lo.vanhanh' });
    const card = await screen.findByTestId('kpi-reports_pending');
    expect(within(card).queryByTestId('kpi-moderation-link')).toBeNull();
  });

  it('F080-AC-20 503 (Prometheus down) shows an error and never zeros', async () => {
    const state = liveOpsState();
    state.overviewError = problem(503, 'SERVICE_UNAVAILABLE');
    const r = await renderLiveOps(URL, { account: 'lo.vanhanh', state });
    expect(await screen.findByRole('alert')).toBeInTheDocument();
    expect(screen.queryByTestId('kpi-cards')).toBeNull();
    expect(screen.queryByTestId('live-as-of')).toBeNull();
    expect(r.backend.calls('GET', '/admin/liveops/overview').length).toBeGreaterThan(0);
  });
});

describe('Guards (F080-AC-24, AC-25, AC-33)', () => {
  it('F080-AC-24 Safety Moderator opening the Live dashboard gets the 403 page and no F080 request', async () => {
    const { backend } = await renderLiveOps(URL, { account: 'sm.antoan' });
    expect(await screen.findByText('Không có quyền truy cập')).toBeInTheDocument();
    expect(backend.calls('GET', '/admin/liveops/overview')).toHaveLength(0);
  });

  it('F080-AC-24 Safety Moderator opening Game Analytics gets the 403 page and no analytics request', async () => {
    const { backend } = await renderLiveOps('/live-ops/game-analytics', { account: 'sm.antoan' });
    expect(await screen.findByText('Không có quyền truy cập')).toBeInTheDocument();
    expect(backend.requests.filter((r) => r.path.startsWith('/admin/analytics/'))).toHaveLength(0);
  });

  it('F080-AC-24 a tenant role never loads the workspace E chunk', async () => {
    const { loaders } = await renderLiveOps(URL, { account: 'ad.truong.a' });
    expect(await screen.findByText('Không có quyền truy cập')).toBeInTheDocument();
    expect(loaders.live_ops).not.toHaveBeenCalled();
  });

  it('F080-AC-25 API 403 PERMISSION_DENIED → 403 page with the trace_id; gateway unreachable → VPN notice', async () => {
    const state = liveOpsState();
    state.overviewError = problem(403, 'PERMISSION_DENIED');
    await renderLiveOps(URL, { account: 'lo.vanhanh', state });
    const forbidden = await screen.findByTestId('game-ops-forbidden');
    expect(forbidden).toHaveTextContent('trace-permission_denied');
    stopPortals();
    state.overviewError = null;
    const r = await renderLiveOps(URL, { account: 'lo.vanhanh', state });
    r.backend.overrides.unshift((x) => (x.path === '/admin/liveops/overview' ? Promise.reject(new TypeError('Failed to fetch')) : undefined));
    await r.runtime.queryClient.refetchQueries({ queryKey: ['live-ops', 'overview'] });
    await waitFor(() => expect(screen.getAllByText('Cần kết nối mạng quản trị (VPN)').length).toBeGreaterThan(0));
    expect(screen.queryByTestId('kpi-cards')).toBeNull();
  });

  it('F080-AC-33 menu turned off → hidden in the menu and "Tính năng đang tắt" page; FEATURE_DISABLED → notice + permissions reload', async () => {
    const r = await renderLiveOps(URL, { account: 'lo.vanhanh', disabledMenus: ['menu.live_ops.live_dashboard'] });
    expect(await screen.findByTestId('feature-disabled')).toBeInTheDocument();
    const nav = screen.getByRole('navigation', { name: 'Menu điều hướng' });
    expect(within(nav).queryByText('Live dashboard')).toBeNull();
    expect(r.backend.calls('GET', '/admin/liveops/overview')).toHaveLength(0);
    stopPortals();
    const r2 = await renderLiveOps(URL, { account: 'lo.vanhanh' });
    await screen.findByTestId('kpi-value-ccu');
    const before = r2.backend.calls('GET', '/me/permissions').length;
    r2.state.overviewError = problem(403, 'FEATURE_DISABLED', { feature_key: 'menu.live_ops.live_dashboard' });
    await r2.runtime.queryClient.refetchQueries({ queryKey: ['live-ops', 'overview'] });
    expect(await screen.findByTestId('game-ops-feature-disabled')).toBeInTheDocument();
    await waitFor(() => expect(r2.backend.calls('GET', '/me/permissions').length).toBeGreaterThan(before));
  });
});
