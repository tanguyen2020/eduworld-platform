// F080 T041, T043 (unit) — filter query and panel catalog of Game Analytics.
// F080-AC-21: ONE dimension filter per request, tenants, client versions only where asked, previous period;
// F080-AC-23: duration metrics only in Chơi lành mạnh + the median in Tổng quan, no "total play time" metric.
import { describe, expect, it } from 'vitest';
import { analyticsQuery, DURATION_METRICS, exportOf, PANEL_METRICS, PANELS, previousRange, type AnalyticsFilters } from './model';

const base: AnalyticsFilters = { range: { preset: '7d', from: '2026-11-20', to: '2026-11-26' }, tenantIds: [], values: [], clientVersions: [], compare: false };

describe('Game Analytics model', () => {
  it('F080-AC-21 has the 11 panels of GAME_ANALYTICS §7.1 with ≤ 20 metric codes each', () => {
    expect(PANELS).toHaveLength(11);
    for (const p of PANELS) expect(PANEL_METRICS[p].length).toBeLessThanOrEqual(20);
  });

  it('F080-AC-21 sends at most one dimension filter, tenants and the asked grain', () => {
    const q = analyticsQuery({ ...base, range: { ...base.range, grain: 'week' }, tenantIds: ['t1', 't2'], dimension: 'platform', values: ['web', 'ios'] });
    expect(q).toEqual({ from: '2026-11-20', to: '2026-11-26', grain: 'week', tenant_ids: ['t1', 't2'], platforms: ['web', 'ios'] });
    const keys = ['school_ids', 'grade_bands', 'platforms', 'locales', 'device_tiers'].filter((k) => k in q);
    expect(keys).toHaveLength(1);
    expect(analyticsQuery({ ...base, dimension: 'school', values: [] })).toEqual({ from: '2026-11-20', to: '2026-11-26' });
  });

  it('F080-AC-21 client versions only for the performance read', () => {
    const f = { ...base, clientVersions: ['1.2.0'] };
    expect(analyticsQuery(f)).not.toHaveProperty('client_versions');
    expect(analyticsQuery(f, { clientVersions: true }).client_versions).toEqual(['1.2.0']);
  });

  it('F080-AC-21 previous period has the same length and ends the day before', () => {
    expect(previousRange({ from: '2026-11-20', to: '2026-11-26' })).toEqual({ from: '2026-11-13', to: '2026-11-19' });
    expect(previousRange({ from: '2026-11-01', to: '2026-11-01' })).toEqual({ from: '2026-10-31', to: '2026-10-31' });
  });

  it('F080-AC-23 duration metrics only in Chơi lành mạnh, the median also in Tổng quan; no total play time', () => {
    for (const p of PANELS) {
      const durations = PANEL_METRICS[p].filter((c) => (DURATION_METRICS as readonly string[]).includes(c));
      if (p === 'overview') expect(durations).toEqual(['session_duration_p50']);
      else if (p === 'wellbeing') expect(durations).toEqual(['daily_playtime_p90']);
      else expect(durations).toEqual([]);
      expect(PANEL_METRICS[p].some((c) => /total_play|playtime_total|total_time/.test(c))).toBe(false);
    }
  });

  it('F080-AC-23 export of a panel maps to its report', () => {
    expect(exportOf('overview')).toEqual({ report: 'metrics', params: { metric_codes: PANEL_METRICS.overview } });
    expect(exportOf('retention').report).toBe('retention');
    expect(exportOf('onboarding')).toEqual({ report: 'funnel', params: { funnel_code: 'onboarding' } });
    expect(exportOf('quests')).toEqual({ report: 'content_stats', params: { kind: 'quest' } });
    expect(exportOf('performance').report).toBe('performance');
  });
});
