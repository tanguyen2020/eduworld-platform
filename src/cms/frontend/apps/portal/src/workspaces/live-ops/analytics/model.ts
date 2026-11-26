// Filters and panel catalog of Game Analytics (F080 T044, F080-AC-21, F080-AC-23, GAME_ANALYTICS §7.1). Pure functions,
// unit-tested: which metric codes each of the 11 panels asks (definitions live only in GAME_ANALYTICS §4 — the portal
// displays what module game returns), the request query of the filter bar (range, tenants, ONE "cắt theo" dimension —
// rollups store one dimension per row, two dimension filters → 422 VALIDATION_FAILED — and client versions for the
// performance panel only) and the previous period of "So sánh kỳ trước". Duration metrics appear only in Chơi lành mạnh
// (plus the median session length in Tổng quan); there is no "total play time" anywhere (AC-ANL-109, AGENTS R19).
import type { DashboardRange } from '@eduworld/ops-ui/dashboard';
import dayjs from 'dayjs';

export type Dimension = 'school' | 'grade_band' | 'platform' | 'locale' | 'device_tier';
export const DIMENSIONS: Dimension[] = ['school', 'grade_band', 'platform', 'locale', 'device_tier'];

/** Query parameter of each dimension (getGameMetrics and the other analytics reads). */
export const DIMENSION_PARAM = {
  school: 'school_ids',
  grade_band: 'grade_bands',
  platform: 'platforms',
  locale: 'locales',
  device_tier: 'device_tiers',
} as const satisfies Record<Dimension, string>;

/** Fixed values of the enumerated dimensions (contract enums GradeBand, Locale, DeviceTier; platforms of ADR-0101). */
export const DIMENSION_VALUES: Record<Exclude<Dimension, 'school'>, string[]> = {
  grade_band: ['g1_3', 'g4_5', 'g6_7', 'g8_9'],
  platform: ['web', 'android', 'ios'],
  locale: ['vi-VN', 'en-US'],
  device_tier: ['minimum', 'mid', 'high'],
};

export type PanelKey =
  | 'overview'
  | 'retention'
  | 'onboarding'
  | 'quests'
  | 'challenges'
  | 'progression'
  | 'economy'
  | 'social'
  | 'events'
  | 'wellbeing'
  | 'performance';

/** The 11 panels in the order of GAME_ANALYTICS §7.1. */
export const PANELS: PanelKey[] = ['overview', 'retention', 'onboarding', 'quests', 'challenges', 'progression', 'economy', 'social', 'events', 'wellbeing', 'performance'];

/** getGameMetrics codes of each panel (≤ 20 per request). Panels without codes read their own report. */
export const PANEL_METRICS: Record<PanelKey, string[]> = {
  overview: ['dau', 'wau', 'mau', 'stickiness', 'session_duration_p50', 'active_days_per_week'],
  retention: [],
  onboarding: ['chapter0_completion'],
  quests: ['quest_completion_rate', 'chapter_completion_rate'],
  challenges: ['challenge_success_rate', 'boss_defeat_rate'],
  progression: ['level_distribution', 'days_to_level_p50', 'region_unlock_rate', 'xp_cap_hit_rate'],
  economy: ['currency_sources', 'currency_sinks', 'currency_net_flow', 'currency_cap_hit_rate', 'shop_conversion'],
  social: ['social_play_rate', 'match_participation_rate', 'match_completion_rate', 'party_size_avg', 'chat_activity'],
  events: ['event_participation_rate'],
  wellbeing: ['sessions_over_reminder', 'sessions_over_second_reminder', 'late_night_play_rate', 'daily_playtime_p90'],
  performance: [],
};

/** Play-time metrics: only in Chơi lành mạnh, and the median in Tổng quan (F080-AC-23). */
export const DURATION_METRICS = ['session_duration_p50', 'daily_playtime_p90'] as const;

/** Export of a panel: the report kind and its own parameters (merged with the filter query). */
export function exportOf(panel: PanelKey): { report: 'metrics' | 'retention' | 'funnel' | 'content_stats' | 'performance'; params: Record<string, unknown> } {
  switch (panel) {
    case 'retention':
      return { report: 'retention', params: {} };
    case 'onboarding':
      return { report: 'funnel', params: { funnel_code: 'onboarding' } };
    case 'quests':
      return { report: 'content_stats', params: { kind: 'quest' } };
    case 'performance':
      return { report: 'performance', params: {} };
    default:
      return { report: 'metrics', params: { metric_codes: PANEL_METRICS[panel] } };
  }
}

export interface AnalyticsFilters {
  range: DashboardRange;
  /** Tenants of the topbar filter (System Admin); empty = every tenant. */
  tenantIds: readonly string[];
  dimension?: Dimension;
  values: string[];
  /** Client versions: performance panel only. */
  clientVersions: string[];
  compare: boolean;
}

export type AnalyticsQuery = Record<string, string | string[]>;

/** Query of a read: range (+ grain asked), tenants, at most ONE dimension filter, client versions when asked. */
export function analyticsQuery(f: AnalyticsFilters, opts: { clientVersions?: boolean; range?: { from: string; to: string } } = {}): AnalyticsQuery {
  const r = opts.range ?? f.range;
  const q: AnalyticsQuery = { from: r.from, to: r.to };
  if (f.range.grain) q.grain = f.range.grain;
  if (f.tenantIds.length) q.tenant_ids = [...f.tenantIds];
  if (f.dimension && f.values.length) q[DIMENSION_PARAM[f.dimension]] = [...f.values];
  if (opts.clientVersions && f.clientVersions.length) q.client_versions = [...f.clientVersions];
  return q;
}

/** The period of the same length right before the range ("So sánh kỳ trước"). */
export function previousRange(r: { from: string; to: string }): { from: string; to: string } {
  const days = dayjs(r.to).diff(dayjs(r.from), 'day') + 1;
  const to = dayjs(r.from).subtract(1, 'day');
  return { from: to.subtract(days - 1, 'day').format('YYYY-MM-DD'), to: to.format('YYYY-MM-DD') };
}

/** Display format of a server unit: ratios as percent, seconds as minutes, else a plain number. */
export type UnitFormat = 'percent' | 'minutes' | 'number';

export function unitFormat(unit: string | undefined): UnitFormat {
  switch ((unit ?? '').toLowerCase()) {
    case 'ratio':
    case 'rate':
    case 'share':
    case 'percent':
      return 'percent';
    case 's':
    case 'sec':
    case 'second':
    case 'seconds':
      return 'minutes';
    default:
      return 'number';
  }
}

/** Server value → number shown (percent values sent 0..100 become 0..1; seconds become minutes). */
export function displayValue(value: number, unit: string | undefined): number {
  const u = (unit ?? '').toLowerCase();
  if (u === 'percent') return value / 100;
  if (unitFormat(unit) === 'minutes') return value / 60;
  return value;
}
