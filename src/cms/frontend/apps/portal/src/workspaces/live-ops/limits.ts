// Parameters and field limits of the F080 screens of workspace E (AGENTS R33, ADR-0041): code-scope values of
// CONFIG_DEFAULTS under their registered names and field limits copied from specs/openapi/eduworld-api.v1.yaml with the
// location they mirror. `limits.test.ts` fails when the owner table or the contract disagrees with this file. The server
// enforces every limit anyway (422 DASHBOARD_RANGE_TOO_LARGE / VALIDATION_FAILED): a drift only changes the UI choices.
import type { DashboardRangeLimits } from '@eduworld/ops-ui/dashboard';

export const DASHBOARD_CONFIG = {
  /** Longest custom range (days, DASHBOARD_DATA §8) — presets ≤ 2 years. */
  dashboard_max_range_days: 730,
  /** ≤ this many days → daily grain. */
  dashboard_grain_daily_max_days: 31,
  /** ≤ this many days → weekly grain, more → monthly. */
  dashboard_grain_weekly_max_days: 366,
  /** Concurrent dashboard queries of one page. */
  dashboard_max_concurrent_per_user: 4,
} as const;

export const RANGE_LIMITS: DashboardRangeLimits = {
  maxRangeDays: DASHBOARD_CONFIG.dashboard_max_range_days,
  grainDailyMaxDays: DASHBOARD_CONFIG.dashboard_grain_daily_max_days,
  grainWeeklyMaxDays: DASHBOARD_CONFIG.dashboard_grain_weekly_max_days,
};

export const FIELDS = {
  /** Reason of suspend / resume / cancel of an event run (F077). */
  runReason: { source: 'schemas/EventRunOperation/properties/reason', minLength: 3, maxLength: 1000 },
  /** One client version of the performance filter. */
  clientVersion: { source: 'paths/getGamePerformance/client_versions', maxLength: 64, maxItems: 20 },
} as const;
