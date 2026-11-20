// Dashboard parameters used by the F056 screens (AGENTS R33): code-scope values of CONFIG_DEFAULTS mirrored here under
// their registered names; `limits.test.ts` fails when docs/00-overview/CONFIG_DEFAULTS.md and this file disagree, so the
// owner table stays the single source. The server enforces every limit anyway (422 DASHBOARD_RANGE_TOO_LARGE, page
// size capped, 429 DASHBOARD_BUSY / EXPORT_LIMIT_REACHED): a drift only changes which choices the UI offers.
import type { DashboardRangeLimits } from '@eduworld/ops-ui/dashboard';

export const DASHBOARD_CONFIG = {
  /** Longest custom range (days, DASHBOARD_DATA §8). */
  dashboard_max_range_days: 730,
  /** ≤ this many days → daily grain. */
  dashboard_grain_daily_max_days: 31,
  /** ≤ this many days → weekly grain, more → monthly. */
  dashboard_grain_weekly_max_days: 366,
  /** Concurrent dashboard queries of one page. */
  dashboard_max_concurrent_per_user: 4,
  /** Rows per page of the detail lists. */
  dashboard_page_size_max: 100,
  /** Refresh of the weekly numbers (minutes). */
  school_dashboard_refresh: 15,
} as const;

export const RANGE_LIMITS: DashboardRangeLimits = {
  maxRangeDays: DASHBOARD_CONFIG.dashboard_max_range_days,
  grainDailyMaxDays: DASHBOARD_CONFIG.dashboard_grain_daily_max_days,
  grainWeeklyMaxDays: DASHBOARD_CONFIG.dashboard_grain_weekly_max_days,
};

/** Query timing of the dashboard reads: refetch every `school_dashboard_refresh` and on focus once stale. */
export const DASHBOARD_QUERY_TIMING = {
  staleTime: DASHBOARD_CONFIG.school_dashboard_refresh * 60_000,
  refetchInterval: DASHBOARD_CONFIG.school_dashboard_refresh * 60_000,
  refetchOnWindowFocus: true,
} as const;

/** Polling of a running export job (client cadence, not a business parameter). */
export const REPORT_POLL_MS = 2000;
