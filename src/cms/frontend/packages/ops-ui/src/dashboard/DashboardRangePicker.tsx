// DashboardRangePicker (F056 T009, F056-AC-03, ADMIN_PORTAL §3.14.1, DASHBOARD_DATA §8): presets Hôm nay, 7 ngày, 30 ngày,
// Học kỳ này, Năm học này, 12 tháng and a custom range of at most `dashboard_max_range_days` — dates outside the limit
// (and future dates) are disabled in the picker; there is no "all time". The grain select only offers the grain of the
// range or a coarser one (`dashboard_grain_daily_max_days` / `dashboard_grain_weekly_max_days`); the server decides the
// grain it really uses and returns it. "Học kỳ này" / "Năm học này" need the school calendar dates from the caller;
// without them the preset is shown disabled. Limits are props (the app owns the configuration, AGENTS R33).
import { DatePicker, Flex, Segmented, Select, Tooltip } from 'antd';
import dayjs, { type Dayjs } from 'dayjs';
import { useTranslation } from 'react-i18next';
import { OPS_UI_NS } from '../i18n';
import type { DashboardGrain } from './DashboardWidget';

export const RANGE_PRESETS = ['today', '7d', '30d', 'term', 'school_year', '12m'] as const;
export type RangePreset = (typeof RANGE_PRESETS)[number] | 'custom';

export interface DashboardRange {
  preset: RangePreset;
  /** YYYY-MM-DD, inclusive. */
  from: string;
  /** YYYY-MM-DD, inclusive. */
  to: string;
  /** Coarser grain asked by the user; undefined = the server's rule. */
  grain?: DashboardGrain;
}

export interface DashboardRangeLimits {
  maxRangeDays: number;
  grainDailyMaxDays: number;
  grainWeeklyMaxDays: number;
}

export interface CalendarPeriod {
  from: string;
  to: string;
}

export interface DashboardRangePickerProps {
  value: DashboardRange;
  onChange: (next: DashboardRange) => void;
  limits: DashboardRangeLimits;
  /** Dates of the current term / school year (school calendar); a missing one disables its preset. */
  calendar?: { term?: CalendarPeriod; school_year?: CalendarPeriod };
  /** Today (YYYY-MM-DD) — injectable for tests. */
  today?: string;
}

const FMT = 'YYYY-MM-DD';
const GRAINS: DashboardGrain[] = ['day', 'week', 'month'];

/** Inclusive number of days of a range. */
export function rangeDays(from: string, to: string): number {
  return dayjs(to).diff(dayjs(from), 'day') + 1;
}

/** Finest grain allowed for a range (DASHBOARD_DATA §8): ≤ daily max → day, ≤ weekly max → week, else month. */
export function minGrainFor(days: number, limits: DashboardRangeLimits): DashboardGrain {
  if (days <= limits.grainDailyMaxDays) return 'day';
  if (days <= limits.grainWeeklyMaxDays) return 'week';
  return 'month';
}

/** Grains the user may pick: the rule's grain or coarser (never finer). */
export function allowedGrains(days: number, limits: DashboardRangeLimits): DashboardGrain[] {
  return GRAINS.slice(GRAINS.indexOf(minGrainFor(days, limits)));
}

/** Range of a preset ending today; `term` / `school_year` come from the calendar (end capped at today). */
export function presetRange(preset: Exclude<RangePreset, 'custom'>, today: string, calendar?: DashboardRangePickerProps['calendar']): CalendarPeriod | null {
  const t = dayjs(today);
  switch (preset) {
    case 'today':
      return { from: today, to: today };
    case '7d':
      return { from: t.subtract(6, 'day').format(FMT), to: today };
    case '30d':
      return { from: t.subtract(29, 'day').format(FMT), to: today };
    case '12m':
      return { from: t.subtract(1, 'year').add(1, 'day').format(FMT), to: today };
    default: {
      const p = calendar?.[preset];
      if (!p) return null;
      return { from: p.from, to: dayjs(p.to).isAfter(t) ? today : p.to };
    }
  }
}

/** Disabled dates of the custom picker: future days, and days making the range longer than `maxRangeDays`. */
export function isDateDisabled(date: Dayjs, anchor: Dayjs | null | undefined, today: string, limits: DashboardRangeLimits): boolean {
  if (date.isAfter(dayjs(today), 'day')) return true;
  if (anchor && Math.abs(date.startOf('day').diff(anchor.startOf('day'), 'day')) + 1 > limits.maxRangeDays) return true;
  return false;
}

export function DashboardRangePicker({ value, onChange, limits, calendar, today = dayjs().format(FMT) }: DashboardRangePickerProps) {
  const { t } = useTranslation(OPS_UI_NS);
  const days = rangeDays(value.from, value.to);
  const grains = allowedGrains(days, limits);
  const pick = (preset: Exclude<RangePreset, 'custom'>) => {
    const r = presetRange(preset, today, calendar);
    if (r) onChange({ preset, ...r, grain: undefined });
  };
  const options = RANGE_PRESETS.map((p) => {
    const missing = (p === 'term' || p === 'school_year') && !calendar?.[p];
    const label = t(`dashboard.range.preset.${p}`);
    return { value: p, disabled: missing, label: missing ? <Tooltip title={t('dashboard.range.noCalendar')}>{label}</Tooltip> : label };
  });
  return (
    <Flex wrap gap="small" align="center" data-testid="dashboard-range">
      <Segmented<RangePreset>
        aria-label={t('dashboard.range.label')}
        options={options}
        value={value.preset === 'custom' ? (undefined as never) : value.preset}
        onChange={(p) => pick(p as Exclude<RangePreset, 'custom'>)}
      />
      <DatePicker.RangePicker
        aria-label={t('dashboard.range.custom')}
        allowClear={false}
        format="DD/MM/YYYY"
        value={[dayjs(value.from), dayjs(value.to)]}
        disabledDate={(d, info) => isDateDisabled(d, info?.from, today, limits)}
        onChange={(v) => {
          if (v?.[0] && v[1]) onChange({ preset: 'custom', from: v[0].format(FMT), to: v[1].format(FMT), grain: undefined });
        }}
      />
      <Select<DashboardGrain>
        aria-label={t('dashboard.range.grain')}
        allowClear
        placeholder={t('dashboard.range.grainAuto')}
        value={value.grain && grains.includes(value.grain) ? value.grain : undefined}
        options={GRAINS.map((g) => ({ value: g, label: t(`dashboard.grain.${g}`), disabled: !grains.includes(g) }))}
        onChange={(g) => onChange({ ...value, grain: g ?? undefined })}
      />
    </Flex>
  );
}
