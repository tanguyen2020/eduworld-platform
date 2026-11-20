// F056 T013 — F056-AC-03 (TC-DASH-002): presets (Hôm nay, 7 ngày, 30 ngày, Học kỳ này, Năm học này, 12 tháng), no "all
// time", custom range limited to `dashboard_max_range_days` (dates beyond it and future dates disabled), grain only the
// rule's grain or coarser (`dashboard_grain_daily_max_days` / `dashboard_grain_weekly_max_days`).
import { fireEvent, screen, within } from '@testing-library/react';
import dayjs from 'dayjs';
import { useState } from 'react';
import { describe, expect, it, vi } from 'vitest';
import { renderOps } from '../test/render';
import { allowedGrains, DashboardRangePicker, isDateDisabled, minGrainFor, presetRange, rangeDays, type DashboardRange } from './DashboardRangePicker';

const limits = { maxRangeDays: 730, grainDailyMaxDays: 31, grainWeeklyMaxDays: 366 };
const today = '2026-11-21';

describe('range rules (F056-AC-03)', () => {
  it('presets end today; term / school year only from the calendar', () => {
    expect(presetRange('today', today)).toEqual({ from: today, to: today });
    expect(presetRange('7d', today)).toEqual({ from: '2026-11-15', to: today });
    expect(rangeDays('2026-11-15', today)).toBe(7);
    expect(presetRange('30d', today)).toEqual({ from: '2026-10-23', to: today });
    expect(presetRange('12m', today)).toEqual({ from: '2025-11-22', to: today });
    expect(presetRange('term', today)).toBeNull();
    expect(presetRange('school_year', today, { school_year: { from: '2026-09-05', to: '2027-05-31' } })).toEqual({ from: '2026-09-05', to: today });
  });

  it('grain: never finer than the rule', () => {
    expect(minGrainFor(31, limits)).toBe('day');
    expect(minGrainFor(32, limits)).toBe('week');
    expect(minGrainFor(366, limits)).toBe('week');
    expect(minGrainFor(367, limits)).toBe('month');
    expect(allowedGrains(7, limits)).toEqual(['day', 'week', 'month']);
    expect(allowedGrains(90, limits)).toEqual(['week', 'month']);
    expect(allowedGrains(500, limits)).toEqual(['month']);
  });

  it('custom range: future dates and dates beyond the limit from the first picked date are disabled', () => {
    const from = dayjs('2026-01-01');
    expect(isDateDisabled(dayjs('2026-11-22'), null, today, limits)).toBe(true);
    expect(isDateDisabled(dayjs('2026-11-21'), null, today, limits)).toBe(false);
    expect(isDateDisabled(dayjs('2024-01-03'), from, today, limits)).toBe(false); // 730 days inclusive
    expect(isDateDisabled(dayjs('2024-01-02'), from, today, limits)).toBe(true); // 731 days
  });
});

function Harness({ onChange }: { onChange: (r: DashboardRange) => void }) {
  const [v, setV] = useState<DashboardRange>({ preset: '7d', from: '2026-11-15', to: today });
  return (
    <DashboardRangePicker
      value={v}
      today={today}
      limits={limits}
      onChange={(r) => {
        setV(r);
        onChange(r);
      }}
    />
  );
}

describe('DashboardRangePicker (F056-AC-03)', () => {
  it('shows the six presets and no "all time"; term/school year disabled without a school calendar', async () => {
    const onChange = vi.fn();
    await renderOps(<Harness onChange={onChange} />);
    const presets = within(screen.getByTestId('dashboard-range')).getAllByRole('radio');
    expect(presets).toHaveLength(6);
    for (const label of ['Hôm nay', '7 ngày', '30 ngày', 'Học kỳ này', 'Năm học này', '12 tháng']) expect(screen.getByText(label)).toBeInTheDocument();
    expect(screen.queryByText(/toàn bộ thời gian/i)).toBeNull();
    expect(screen.getByRole('radio', { name: 'Học kỳ này' })).toBeDisabled();
    expect(screen.getByRole('radio', { name: 'Năm học này' })).toBeDisabled();
    fireEvent.click(screen.getByText('30 ngày'));
    expect(onChange).toHaveBeenLastCalledWith({ preset: '30d', from: '2026-10-23', to: today, grain: undefined });
  });

  it('grain select offers only the rule grain or coarser', async () => {
    const onChange = vi.fn();
    await renderOps(<Harness onChange={onChange} />);
    fireEvent.click(screen.getByText('12 tháng'));
    fireEvent.mouseDown(screen.getByRole('combobox', { name: 'Độ hạt' }));
    const day = await screen.findByTitle('Theo ngày');
    expect(day.closest('.ant-select-item')).toHaveClass('ant-select-item-option-disabled');
    expect(screen.getByTitle('Theo tuần').closest('.ant-select-item')).not.toHaveClass('ant-select-item-option-disabled');
    fireEvent.click(screen.getByTitle('Theo tháng'));
    expect(onChange).toHaveBeenLastCalledWith(expect.objectContaining({ preset: '12m', grain: 'month' }));
  });

  it('the custom picker disables future days', async () => {
    await renderOps(<Harness onChange={vi.fn()} />);
    fireEvent.mouseDown(screen.getAllByRole('textbox')[0]!);
    fireEvent.click(screen.getAllByRole('textbox')[0]!);
    const future = await screen.findAllByTitle('2026-11-25');
    expect(future[0]!.closest('td')).toHaveClass('ant-picker-cell-disabled');
    expect(screen.getAllByTitle('2026-11-20')[0]!.closest('td')).not.toHaveClass('ant-picker-cell-disabled');
  });
});
